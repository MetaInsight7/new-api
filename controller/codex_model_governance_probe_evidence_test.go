package controller

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestCodexGovernanceProbeRetainsUnsupportedSSEEvidence(test *testing.T) {
	const message = "The 'gpt-5.4' model is not supported when using Codex with a ChatGPT account. access_token=probe-token-secret account_id=probe-account-sensitive request=probe-request-sensitive"
	upstreamError := map[string]any{"type": "invalid_request_error", "code": "invalid_request_error", "message": message}
	for _, scenario := range []struct {
		name  string
		event map[string]any
	}{
		{
			name: "response failure",
			event: map[string]any{
				"type":     "response.failed",
				"response": map[string]any{"model": "gpt-5.4", "status": "failed", "error": upstreamError},
			},
		},
		{name: "flat error", event: map[string]any{"type": "error", "code": "invalid_request_error", "message": message}},
		{name: "nested error", event: map[string]any{"type": "error", "error": upstreamError}},
	} {
		test.Run(scenario.name, func(test *testing.T) {
			database := setupCodexGovernanceControllerTest(test)
			payload, err := common.Marshal(scenario.event)
			require.NoError(test, err)
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				requests.Add(1)
				writer.Header().Set("Content-Type", "text/event-stream")
				_, err := io.WriteString(writer, "data: "+string(payload)+"\n\n")
				assert.NoError(test, err)
			}))
			test.Cleanup(server.Close)
			if service.GetHttpClient() == nil {
				service.InitHttpClient()
			}
			channel := model.Channel{
				Id: 61, Type: constant.ChannelTypeCodex, Status: common.ChannelStatusEnabled,
				Name: "SSE probe", BaseURL: common.GetPointer(server.URL),
				Key:    `{"access_token":"probe-token-secret","account_id":"probe-account-sensitive"}`,
				Models: "gpt-5.4-mini", ModelMapping: common.GetPointer(`{"gpt-5.4-mini":"gpt-5.4"}`), Group: "default",
			}
			require.NoError(test, database.Create(&channel).Error)
			require.NoError(test, channel.AddAbilities(nil))
			processed, err := runCodexGovernanceProbes(context.Background(), nil)
			require.NoError(test, err)
			assert.Zero(test, processed)
			assert.Zero(test, requests.Load())
			setting := operation_setting.GetCodexModelGovernanceSetting()
			setting.Enabled, setting.ProbeEnabled = true, true
			processed, err = runCodexGovernanceProbes(context.Background(), nil)
			require.NoError(test, err)
			assert.Equal(test, 1, processed)
			_, err = model.GetCodexModelGovernanceRecordByModelName("gpt-5.4-mini")
			require.ErrorIs(test, err, gorm.ErrRecordNotFound)
			processed, err = runCodexGovernanceProbes(context.Background(), nil)
			require.NoError(test, err)
			assert.Equal(test, 1, processed)
			assert.EqualValues(test, 2, requests.Load())
			record, err := model.GetCodexModelGovernanceRecordByModelName("gpt-5.4-mini")
			require.NoError(test, err)
			assert.Equal(test, model.CodexModelGovernanceSourceProbe, record.Source)
			assert.Equal(test, operation_setting.DefaultCodexUnsupportedPattern, record.MatchedRule)
			assert.Equal(test, []int{61}, model.CodexModelGovernanceDisabledChannelIDs(*record))
			assert.NotContains(test, record.LastError, "probe-token-secret")
			assert.NotContains(test, record.LastError, "probe-account-sensitive")
			assert.NotContains(test, record.LastError, "probe-request-sensitive")
			var ability model.Ability
			require.NoError(test, database.First(&ability, "channel_id = ? AND model = ?", channel.Id, "gpt-5.4-mini").Error)
			assert.False(test, ability.Enabled)
		})
	}
}

func TestCodexGovernanceProbeSSEEvidenceRequiresConsecutiveConfiguredRules(test *testing.T) {
	const unsupported = "The 'gpt-5.4' model is not supported when using Codex with a ChatGPT account."
	for _, scenario := range []struct {
		name     string
		patterns []string
		messages []string
	}{
		{
			name:     "temporary failure resets the unsupported threshold",
			patterns: []string{operation_setting.DefaultCodexUnsupportedPattern},
			messages: []string{unsupported, "rate limit exceeded", unsupported, unsupported},
		},
		{
			name:     "only the configured unsupported rule counts",
			patterns: []string{`^Unsupported Codex model: ([A-Za-z0-9.-]+)$`},
			messages: []string{unsupported, unsupported, "Unsupported Codex model: gpt-5.4", "Unsupported Codex model: gpt-5.4"},
		},
	} {
		test.Run(scenario.name, func(test *testing.T) {
			database := setupCodexGovernanceControllerTest(test)
			var currentMessage atomic.Value
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				message := currentMessage.Load().(string)
				errorType := "invalid_request_error"
				if message == "rate limit exceeded" {
					errorType = "rate_limit_error"
				}
				payload, err := common.Marshal(map[string]any{
					"type": "response.failed",
					"response": map[string]any{
						"status": "failed",
						"error":  map[string]any{"type": errorType, "code": errorType, "message": message},
					},
				})
				if !assert.NoError(test, err) {
					return
				}
				writer.Header().Set("Content-Type", "text/event-stream")
				_, err = io.WriteString(writer, "data: "+string(payload)+"\n\n")
				assert.NoError(test, err)
			}))
			test.Cleanup(server.Close)
			if service.GetHttpClient() == nil {
				service.InitHttpClient()
			}
			channel := model.Channel{
				Id: 62, Type: constant.ChannelTypeCodex, Status: common.ChannelStatusEnabled,
				Name: "strict SSE probe", BaseURL: common.GetPointer(server.URL),
				Key:    `{"access_token":"fake-probe-token","account_id":"fake-probe-account"}`,
				Models: "gpt-5.4", Group: "default",
			}
			require.NoError(test, database.Create(&channel).Error)
			require.NoError(test, channel.AddAbilities(nil))
			setting := operation_setting.GetCodexModelGovernanceSetting()
			setting.Enabled, setting.ProbeEnabled = true, true
			setting.UnsupportedMessagePatterns = scenario.patterns
			for index, message := range scenario.messages {
				currentMessage.Store(message)
				processed, err := runCodexGovernanceProbes(context.Background(), nil)
				require.NoError(test, err)
				assert.Equal(test, 1, processed)
				record, err := model.GetCodexModelGovernanceRecordByModelName("gpt-5.4")
				if index < len(scenario.messages)-1 {
					require.ErrorIs(test, err, gorm.ErrRecordNotFound)
					continue
				}
				require.NoError(test, err)
				assert.Equal(test, scenario.patterns[0], record.MatchedRule)
				assert.Equal(test, []int{62}, model.CodexModelGovernanceDisabledChannelIDs(*record))
				assert.Contains(test, record.LastError, "gpt-5.4")
			}
		})
	}
}
