package service

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

const ChannelTestAlertBatchSize = 5
const channelTestAlertPendingTTL = 80 * time.Second

type ChannelTestFailure struct {
	ChannelID        int
	ChannelName      string
	ChannelTypeName  string
	StatusCode       int
	ErrorCode        string
	Message          string
	AutoBanEnabled   bool
	DisableTriggered bool
	TestedAt         time.Time
}

var (
	alertCredentialPattern       = regexp.MustCompile(`(?i)(\b(?:access_token|refresh_token|id_token|api[_-]?key|authorization)\b\s*(?::|=)?\s*)(?:"[^"]*"|'[^']*'|bearer\s+[^\s,;}]+|[^\s,;}]+)`)
	alertQuotedCredentialPattern = regexp.MustCompile(`(?i)(["'](?:access_token|refresh_token|id_token|api[_-]?key|authorization)["']\s*:\s*)(?:"[^"]*"|'[^']*'|[^,\s}]+)`)
	alertSKPattern               = regexp.MustCompile(`sk-[A-Za-z0-9_-]+`)
	alertAWSKeyPattern           = regexp.MustCompile(`\b(?:AKIA|ASIA)[A-Z0-9]{16}\b`)
	alertGoogleKeyPattern        = regexp.MustCompile(`\bAIza[0-9A-Za-z_-]{35}\b`)
	alertURLPattern              = regexp.MustCompile(`(?i)(?:https?|wss?)://[^\s"<>{}]+`)
	alertDomainPattern           = regexp.MustCompile(`(?i)\b(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,}\b`)
	channelAlertLocalCooldown    = struct {
		sync.Mutex
		records map[int]localChannelAlertCooldown
	}{records: make(map[int]localChannelAlertCooldown)}
)

type localChannelAlertCooldown struct {
	lastAt    time.Time
	pendingAt time.Time
	token     string
}

type channelAlertReservation struct {
	channelID int
	token     string
	durable   *model.ChannelTestAlertReservation
}

func sanitizeChannelAlertText(value string) string {
	value = common.MaskSensitiveInfo(value)
	value = alertURLPattern.ReplaceAllString(value, "***")
	value = alertDomainPattern.ReplaceAllString(value, "***")
	value = alertQuotedCredentialPattern.ReplaceAllString(value, `${1}"***"`)
	value = alertCredentialPattern.ReplaceAllString(value, `${1}***`)
	value = alertSKPattern.ReplaceAllString(value, "sk-***")
	value = alertAWSKeyPattern.ReplaceAllString(value, "aws-***")
	return alertGoogleKeyPattern.ReplaceAllString(value, "AIza***")
}

func buildChannelTestAlertContent(alerts []ChannelTestFailure) string {
	var content strings.Builder
	fmt.Fprintf(&content, "New API channel test failures\nTotal Failures: %d\n", len(alerts))
	for _, alert := range alerts {
		fmt.Fprintf(&content, "\nChannel ID: %d\nChannel Name: %s\nChannel Type: %s\nStatus Code: %d\nError Code: %s\nError: %s\nAuto Ban Enabled: %t\nDisable Flow Triggered (not confirmation): %t\nTime: %s\n", alert.ChannelID, sanitizeChannelAlertText(alert.ChannelName), sanitizeChannelAlertText(alert.ChannelTypeName), alert.StatusCode, sanitizeChannelAlertText(alert.ErrorCode), sanitizeChannelAlertText(alert.Message), alert.AutoBanEnabled, alert.DisableTriggered, alert.TestedAt.Format("2006-01-02 15:04:05"))
	}
	return sanitizeChannelAlertText(content.String())
}

func reserveLocalChannelAlert(channelID int, token string, cooldown time.Duration) bool {
	channelAlertLocalCooldown.Lock()
	defer channelAlertLocalCooldown.Unlock()
	now := time.Now()
	record := channelAlertLocalCooldown.records[channelID]
	if (!record.lastAt.IsZero() && now.Sub(record.lastAt) < cooldown) || (!record.pendingAt.IsZero() && now.Sub(record.pendingAt) < channelTestAlertPendingTTL) {
		return false
	}
	record.pendingAt, record.token = now, token
	channelAlertLocalCooldown.records[channelID] = record
	return true
}

func (reservation channelAlertReservation) finish(ctx context.Context, sent bool) {
	if reservation.durable != nil {
		if sent {
			if err := model.CommitChannelTestAlertCooldown(ctx, reservation.durable); err != nil {
				common.SysError("channel-test alert delivered but durable cooldown commit failed")
			}
		} else if err := model.RollbackChannelTestAlertCooldown(ctx, reservation.durable); err != nil {
			common.SysError("channel-test alert pending reservation cleanup failed")
		}
	}
	channelAlertLocalCooldown.Lock()
	defer channelAlertLocalCooldown.Unlock()
	record := channelAlertLocalCooldown.records[reservation.channelID]
	if reservation.durable == nil && record.token != reservation.token {
		return
	}
	if sent {
		record.lastAt = time.Now()
	}
	if record.token == reservation.token {
		record.pendingAt, record.token = time.Time{}, ""
	}
	if record.lastAt.IsZero() && record.token == "" {
		delete(channelAlertLocalCooldown.records, reservation.channelID)
	} else {
		channelAlertLocalCooldown.records[reservation.channelID] = record
	}
}

func NotifyDingTalkChannelTestFailures(ctx context.Context, alerts []ChannelTestFailure) error {
	setting := operation_setting.GetMonitorAlertSetting()
	analysis := setting.AIAnalysisConfig()
	if !setting.DingTalkAlertEnabled || len(alerts) == 0 || ctx.Err() != nil {
		return nil
	}
	minutes := setting.DingTalkAlertCooldownMinutes
	if minutes <= 0 || int64(minutes) > int64((1<<63-1)/time.Minute) {
		minutes = 60
	}
	cooldown := time.Duration(minutes) * time.Minute
	var eligible []ChannelTestFailure
	var reservations []channelAlertReservation
	sent := false
	defer func() {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
		defer cancel()
		for _, reservation := range reservations {
			reservation.finish(cleanup, sent)
		}
	}()
	for _, alert := range alerts {
		if err := ctx.Err(); err != nil {
			return err
		}
		token, err := common.GenerateRandomCharsKey(32)
		if err != nil {
			return fmt.Errorf("cannot generate channel alert reservation token")
		}
		durable, allowed, err := model.ReserveChannelTestAlertCooldown(ctx, alert.ChannelID, cooldown, channelTestAlertPendingTTL, token)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			common.SysError("channel-test alert durable reservation unavailable; using process-local cooldown without cross-instance deduplication")
			allowed = reserveLocalChannelAlert(alert.ChannelID, token, cooldown)
		}
		if !allowed {
			continue
		}
		eligible = append(eligible, alert)
		reservations = append(reservations, channelAlertReservation{channelID: alert.ChannelID, token: token, durable: durable})
	}
	if len(eligible) == 0 || ctx.Err() != nil {
		return nil
	}
	content := buildChannelTestAlertContent(eligible)
	if analysis.APIKey != "" {
		summary, err := channelTestAlertAISummary(ctx, analysis, content)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			common.SysError("channel-test alert AI summary skipped: " + err.Error())
		} else if summary != "" {
			content = "AI 中文总结：\n" + summary + "\n\n" + content
		}
	}
	if err := sendDingTalkText(ctx, setting.DingTalkAlertWebhookURL, setting.DingTalkAlertSecret, content); err != nil {
		return err
	}
	sent = true
	return nil
}
