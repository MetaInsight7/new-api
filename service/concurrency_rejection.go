package service

import (
	"errors"
	"net/http"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	perfmetrics "github.com/QuantumNous/new-api/pkg/perf_metrics"

	"github.com/gin-gonic/gin"
)

type ConcurrencyScope string

const (
	ConcurrencyScopeUser    ConcurrencyScope = "user"
	ConcurrencyScopeChannel ConcurrencyScope = "channel"
)

const concurrencyLimitLogRecordedContextKey = "concurrency_limit_log_recorded"
const concurrencyLimitMetricRecordedContextKey = "concurrency_limit_metric_recorded"

type ChannelConcurrencyCandidate struct {
	ChannelID int `json:"channel_id"`
	Limit     int `json:"limit"`
}

type ChannelConcurrencyLimitError struct {
	candidates []ChannelConcurrencyCandidate
}

func (e *ChannelConcurrencyLimitError) Error() string {
	return ErrChannelConcurrencyLimit.Error()
}

func (e *ChannelConcurrencyLimitError) Unwrap() error {
	return ErrChannelConcurrencyLimit
}

func NewChannelConcurrencyLimitError(candidates []ChannelConcurrencyCandidate) error {
	return &ChannelConcurrencyLimitError{candidates: mergeChannelConcurrencyCandidates(candidates)}
}

func ChannelConcurrencyCandidates(err error) []ChannelConcurrencyCandidate {
	var concurrencyErr *ChannelConcurrencyLimitError
	if !errors.As(err, &concurrencyErr) || concurrencyErr == nil {
		return nil
	}
	return append([]ChannelConcurrencyCandidate(nil), concurrencyErr.candidates...)
}

func mergeChannelConcurrencyCandidates(groups ...[]ChannelConcurrencyCandidate) []ChannelConcurrencyCandidate {
	seen := make(map[int]struct{})
	var merged []ChannelConcurrencyCandidate
	for _, candidates := range groups {
		for _, candidate := range candidates {
			if candidate.ChannelID <= 0 {
				continue
			}
			if _, exists := seen[candidate.ChannelID]; exists {
				continue
			}
			seen[candidate.ChannelID] = struct{}{}
			merged = append(merged, candidate)
		}
	}
	return merged
}

type ConcurrencyLimitRejection struct {
	Scope      ConcurrencyScope
	ModelName  string
	Limit      int
	Candidates []ChannelConcurrencyCandidate
	PreRelay   bool
}

func RecordConcurrencyLimitRejection(c *gin.Context, rejection ConcurrencyLimitRejection) {
	if c == nil {
		return
	}
	if rejection.Scope != ConcurrencyScopeUser && rejection.Scope != ConcurrencyScopeChannel {
		return
	}

	modelName := rejection.ModelName
	if modelName == "" {
		modelName = common.GetContextKeyString(c, constant.ContextKeyOriginalModel)
	}
	group := common.GetContextKeyString(c, constant.ContextKeyUsingGroup)
	if group == "" {
		group = common.GetContextKeyString(c, constant.ContextKeyTokenGroup)
	}
	startedAt := common.GetContextKeyTime(c, constant.ContextKeyRequestStartTime)
	if startedAt.IsZero() {
		startedAt = time.Now()
	}
	if rejection.PreRelay && !c.GetBool(concurrencyLimitMetricRecordedContextKey) {
		c.Set(concurrencyLimitMetricRecordedContextKey, true)
		perfmetrics.RecordPreRelayIgnored(modelName, group)
	}
	if !constant.ErrorLogEnabled || c.GetBool(concurrencyLimitLogRecordedContextKey) || !RequestPolicy(c).ClaimErrorLog() {
		return
	}
	c.Set(concurrencyLimitLogRecordedContextKey, true)

	content := "Request rejected because the user concurrency limit was reached."
	errorCode := "user_concurrency_limit"
	if rejection.Scope == ConcurrencyScopeChannel {
		content = "Request rejected because all eligible channels reached their concurrency limits."
		errorCode = "channel_concurrency_limit"
	}
	other := model.NewLogOther()
	if c.Request != nil && c.Request.URL != nil {
		other.SetPublic("request_path", c.Request.URL.Path)
	}
	other.SetPublic("content_parts", []*common.Message{common.NewMessage(content)})
	other.SetPublic("error_type", "rate_limit_error")
	other.SetPublic("error_code", errorCode)
	other.SetPublic("status_code", http.StatusTooManyRequests)
	other.SetPublic("error_source", "local_gateway")
	other.SetPublic("concurrency_scope", string(rejection.Scope))

	adminDetails := map[string]any{
		"scope":              string(rejection.Scope),
		"upstream_attempted": false,
	}
	if rejection.Limit > 0 {
		adminDetails["configured_limit"] = rejection.Limit
	}
	if len(rejection.Candidates) > 0 {
		adminDetails["candidates"] = rejection.Candidates
	}
	other.SetAdmin("concurrency", adminDetails)

	useTimeSeconds := max(int(time.Since(startedAt).Seconds()), 0)
	model.RecordErrorLog(
		c,
		common.GetContextKeyInt(c, constant.ContextKeyUserId),
		0,
		modelName,
		c.GetString("token_name"),
		content,
		common.GetContextKeyInt(c, constant.ContextKeyTokenId),
		useTimeSeconds,
		common.GetContextKeyBool(c, constant.ContextKeyIsStream),
		group,
		other,
	)
}
