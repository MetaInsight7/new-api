package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
)

const CodexLimitReportTimeout = 60 * time.Second

var ErrCodexLimitReportRange = errors.New("Invalid Codex report time range")
var errCodexLimitReportRefresh = errors.New("Failed to refresh Codex credential")

type CodexLimitWindow struct {
	UsedPercent        *float64 `json:"used_percent,omitempty"`
	ResetAt            *int64   `json:"reset_at,omitempty"`
	ResetAfterSeconds  *int64   `json:"reset_after_seconds,omitempty"`
	LimitWindowSeconds *int64   `json:"limit_window_seconds,omitempty"`
}

type CodexAdditionalLimit struct {
	Name           string              `json:"name"`
	MeteredFeature string              `json:"metered_feature,omitempty"`
	FiveHourWindow *CodexLimitWindow   `json:"five_hour_window,omitempty"`
	WeeklyWindow   *CodexLimitWindow   `json:"weekly_window,omitempty"`
	UnknownWindows []*CodexLimitWindow `json:"unknown_windows,omitempty"`
}

type CodexLimitReportRow struct {
	ChannelID          int                    `json:"channel_id"`
	ChannelName        string                 `json:"channel_name"`
	ChannelStatus      int                    `json:"channel_status"`
	RangeTokenUsed     int64                  `json:"range_token_used"`
	RangeQuota         int64                  `json:"range_quota"`
	Success            bool                   `json:"success"`
	Message            string                 `json:"message,omitempty"`
	UpstreamStatus     int                    `json:"upstream_status,omitempty"`
	PlanType           string                 `json:"plan_type,omitempty"`
	Email              string                 `json:"email,omitempty"`
	AccountID          string                 `json:"account_id,omitempty"`
	UserID             string                 `json:"user_id,omitempty"`
	Allowed            *bool                  `json:"allowed,omitempty"`
	LimitReached       *bool                  `json:"limit_reached,omitempty"`
	BaseFiveHourWindow *CodexLimitWindow      `json:"base_five_hour_window,omitempty"`
	BaseWeeklyWindow   *CodexLimitWindow      `json:"base_weekly_window,omitempty"`
	UnknownWindows     []*CodexLimitWindow    `json:"unknown_windows,omitempty"`
	AdditionalLimits   []CodexAdditionalLimit `json:"additional_limits,omitempty"`
}

type CodexLimitReport struct {
	GeneratedAt    int64                 `json:"generated_at"`
	StartTimestamp int64                 `json:"start_timestamp"`
	EndTimestamp   int64                 `json:"end_timestamp"`
	TotalChannels  int                   `json:"total_channels"`
	SuccessCount   int                   `json:"success_count"`
	FailureCount   int                   `json:"failure_count"`
	TotalTokenUsed int64                 `json:"total_token_used"`
	TotalQuota     int64                 `json:"total_quota"`
	Rows           []CodexLimitReportRow `json:"rows"`
}

type CodexUsageFetcher func(context.Context, *model.Channel) (int, []byte, error)

type codexReportRateLimit struct {
	PlanType        string            `json:"plan_type"`
	Allowed         *bool             `json:"allowed"`
	LimitReached    *bool             `json:"limit_reached"`
	PrimaryWindow   *CodexLimitWindow `json:"primary_window"`
	SecondaryWindow *CodexLimitWindow `json:"secondary_window"`
}

type codexReportAdditionalLimit struct {
	LimitName       string               `json:"limit_name"`
	MeteredFeature  string               `json:"metered_feature"`
	PlanType        string               `json:"plan_type"`
	RateLimit       codexReportRateLimit `json:"rate_limit"`
	PrimaryWindow   *CodexLimitWindow    `json:"primary_window"`
	SecondaryWindow *CodexLimitWindow    `json:"secondary_window"`
}

type codexReportUsage struct {
	PlanType             string                       `json:"plan_type"`
	Email                string                       `json:"email"`
	AccountID            string                       `json:"account_id"`
	UserID               string                       `json:"user_id"`
	RateLimit            *codexReportRateLimit        `json:"rate_limit"`
	AdditionalRateLimits []codexReportAdditionalLimit `json:"additional_rate_limits"`
}

func GetCodexLimitReport(ctx context.Context, startTimestamp, endTimestamp int64) (CodexLimitReport, error) {
	if startTimestamp <= 0 || endTimestamp < startTimestamp || endTimestamp-startTimestamp > 90*24*3600 {
		return CodexLimitReport{}, ErrCodexLimitReportRange
	}
	ctx, cancel := context.WithTimeout(ctx, CodexLimitReportTimeout)
	defer cancel()
	channels, err := model.GetCodexLimitReportChannels(ctx)
	if err != nil {
		return CodexLimitReport{}, fmt.Errorf("list Codex channels: %w", err)
	}
	channelIDs := make([]int, 0, len(channels))
	for _, channel := range channels {
		channelIDs = append(channelIDs, channel.Id)
	}
	stats, err := model.GetCodexChannelUsageStats(ctx, channelIDs, startTimestamp, endTimestamp)
	if err != nil {
		return CodexLimitReport{}, fmt.Errorf("query Codex consumption logs: %w", err)
	}
	var refreshed atomic.Bool
	fetcher := func(ctx context.Context, channel *model.Channel) (int, []byte, error) {
		status, body, rotated, fetchErr := fetchCodexLimitReportUsage(ctx, channel)
		if rotated {
			refreshed.Store(true)
		}
		return status, body, fetchErr
	}
	report := BuildCodexLimitReportWithUsage(ctx, channels, fetcher, stats, startTimestamp, endTimestamp)
	if refreshed.Load() {
		model.InitChannelCache()
	}
	return report, nil
}

func BuildCodexLimitReportWithUsage(ctx context.Context, channels []*model.Channel, fetcher CodexUsageFetcher, stats map[int]model.CodexChannelUsageStat, startTimestamp, endTimestamp int64) CodexLimitReport {
	ctx, cancel := context.WithTimeout(ctx, CodexLimitReportTimeout)
	defer cancel()
	report := CodexLimitReport{
		StartTimestamp: startTimestamp, EndTimestamp: endTimestamp,
		TotalChannels: len(channels), Rows: make([]CodexLimitReportRow, len(channels)),
	}
	jobs := make(chan int)
	var workers sync.WaitGroup
	for range min(10, len(channels)) {
		workers.Go(func() {
			for index := range jobs {
				report.Rows[index] = buildCodexLimitReportRow(ctx, channels[index], fetcher)
			}
		})
	}
	for index := range channels {
		jobs <- index
	}
	close(jobs)
	workers.Wait()
	for index := range report.Rows {
		row := &report.Rows[index]
		stat := stats[row.ChannelID]
		row.RangeTokenUsed, row.RangeQuota = stat.TokenUsed, stat.Quota
		report.TotalTokenUsed += stat.TokenUsed
		report.TotalQuota += stat.Quota
		if row.Success {
			report.SuccessCount++
		} else {
			report.FailureCount++
		}
	}
	report.GeneratedAt = common.GetTimestamp()
	return report
}

func buildCodexLimitReportRow(ctx context.Context, channel *model.Channel, fetcher CodexUsageFetcher) CodexLimitReportRow {
	row := CodexLimitReportRow{}
	if channel == nil {
		row.Message = "Channel not found"
		return row
	}
	row.ChannelID, row.ChannelName, row.ChannelStatus = channel.Id, channel.Name, channel.Status
	if ctx.Err() != nil {
		row.Message = "Codex usage query cancelled or timed out"
		return row
	}
	if fetcher == nil {
		row.Message = "Failed to fetch Codex usage"
		return row
	}
	status, body, err := fetcher(ctx, channel)
	row.UpstreamStatus = status
	if err != nil {
		row.Message = "Failed to fetch Codex usage"
		if errors.Is(err, errCodexLimitReportRefresh) {
			row.Message = errCodexLimitReportRefresh.Error()
		} else if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			row.Message = "Codex usage query cancelled or timed out"
		}
		return row
	}
	if status < 200 || status >= 300 {
		row.Message = "Codex upstream returned an unsuccessful status"
		return row
	}
	var payload codexReportUsage
	if common.Unmarshal(body, &payload) != nil || payload.RateLimit == nil {
		row.Message = "Invalid Codex usage response"
		return row
	}
	row.Success = true
	row.PlanType = firstCodexReportValue(payload.PlanType, payload.RateLimit.PlanType)
	row.Email, row.AccountID, row.UserID = strings.TrimSpace(payload.Email), strings.TrimSpace(payload.AccountID), strings.TrimSpace(payload.UserID)
	row.Allowed, row.LimitReached = payload.RateLimit.Allowed, payload.RateLimit.LimitReached
	row.BaseFiveHourWindow, row.BaseWeeklyWindow, row.UnknownWindows = resolveCodexReportWindows(row.PlanType, payload.RateLimit.PrimaryWindow, payload.RateLimit.SecondaryWindow)
	for _, item := range payload.AdditionalRateLimits {
		primary, secondary := item.RateLimit.PrimaryWindow, item.RateLimit.SecondaryWindow
		if primary == nil {
			primary = item.PrimaryWindow
		}
		if secondary == nil {
			secondary = item.SecondaryWindow
		}
		fiveHour, weekly, unknown := resolveCodexReportWindows(firstCodexReportValue(item.PlanType, item.RateLimit.PlanType), primary, secondary)
		row.AdditionalLimits = append(row.AdditionalLimits, CodexAdditionalLimit{
			Name: firstCodexReportValue(item.LimitName, item.MeteredFeature), MeteredFeature: strings.TrimSpace(item.MeteredFeature),
			FiveHourWindow: fiveHour, WeeklyWindow: weekly, UnknownWindows: unknown,
		})
	}
	return row
}

func resolveCodexReportWindows(planType string, primary, secondary *CodexLimitWindow) (fiveHour, weekly *CodexLimitWindow, unknown []*CodexLimitWindow) {
	for _, window := range []*CodexLimitWindow{primary, secondary} {
		if window == nil || (window.UsedPercent == nil && window.ResetAt == nil && window.ResetAfterSeconds == nil && window.LimitWindowSeconds == nil) {
			continue
		}
		var windowSeconds int64
		if window.LimitWindowSeconds != nil {
			windowSeconds = *window.LimitWindowSeconds
		}
		if windowSeconds == 5*3600 {
			if !strings.EqualFold(strings.TrimSpace(planType), "free") {
				if fiveHour == nil {
					fiveHour = window
				} else {
					unknown = append(unknown, window)
				}
			}
		} else if windowSeconds >= 6*24*3600 && windowSeconds <= 8*24*3600 && weekly == nil {
			weekly = window
		} else {
			unknown = append(unknown, window)
		}
	}
	return
}

func firstCodexReportValue(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func fetchCodexLimitReportUsage(ctx context.Context, channel *model.Channel) (int, []byte, bool, error) {
	if channel.Type != constant.ChannelTypeCodex || channel.ChannelInfo.IsMultiKey {
		return 0, nil, false, errors.New("unsupported Codex channel")
	}
	key, err := parseCodexOAuthKey(channel.Key)
	if err != nil {
		return 0, nil, false, err
	}
	fetch := func(currentChannel *model.Channel, key *CodexOAuthKey) (int, []byte, error) {
		client, err := GetHttpClientWithProxy(currentChannel.GetSetting().Proxy)
		if err != nil {
			return 0, nil, err
		}
		requestCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
		return FetchCodexWhamUsage(requestCtx, client, currentChannel.GetBaseURL(), key.AccessToken, key.AccountID)
	}
	status, body, err := fetch(channel, key)
	if err != nil || (status != http.StatusUnauthorized && status != http.StatusForbidden) || strings.TrimSpace(key.RefreshToken) == "" {
		return status, body, false, err
	}
	key, refreshedChannel, err := RefreshCodexChannelCredential(ctx, channel.Id, CodexCredentialRefreshOptions{ResetCaches: false, ExpectedKey: channel.Key})
	if err != nil {
		return status, nil, false, fmt.Errorf("%w: %w", errCodexLimitReportRefresh, err)
	}
	status, body, err = fetch(refreshedChannel, key)
	return status, body, true, err
}
