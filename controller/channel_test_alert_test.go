package controller

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupChannelAlertTaskTest(t *testing.T, handler http.HandlerFunc) (*gorm.DB, string) {
	t.Helper()
	database := setupCodexGovernanceControllerTest(t)
	require.NoError(t, database.AutoMigrate(&model.Option{}, &model.AuditLog{}, &model.ChannelTestAlertCooldown{}))
	monitor := operation_setting.GetMonitorSetting()
	previousMonitor, previousOptions := *monitor, common.OptionMap
	previousFetch := *system_setting.GetFetchSetting()
	previousDisable, previousEnable, previousInterval := common.AutomaticDisableChannelEnabled, common.AutomaticEnableChannelEnabled, common.RequestInterval
	*monitor = operation_setting.MonitorSetting{AutoTestChannelMinutes: 10, ChannelTestMode: operation_setting.ChannelTestModeScheduledAll, DingTalkAlertCooldownMinutes: 60, ChannelTestConcurrency: 3}
	common.OptionMap = make(map[string]string)
	common.AutomaticDisableChannelEnabled, common.AutomaticEnableChannelEnabled, common.RequestInterval = false, false, 0
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	endpoint, err := url.Parse(server.URL)
	require.NoError(t, err)
	*system_setting.GetFetchSetting() = system_setting.FetchSetting{EnableSSRFProtection: true, AllowPrivateIp: true, AllowedPorts: []string{endpoint.Port()}}
	if service.GetSSRFProtectedHTTPClient() == nil {
		service.InitHttpClient()
	}
	t.Cleanup(func() {
		*monitor, common.OptionMap = previousMonitor, previousOptions
		*system_setting.GetFetchSetting() = previousFetch
		common.AutomaticDisableChannelEnabled, common.AutomaticEnableChannelEnabled, common.RequestInterval = previousDisable, previousEnable, previousInterval
	})
	return database, server.URL
}

func putChannelAlertOption(t *testing.T, key string, value any) bool {
	t.Helper()
	body, err := common.Marshal(map[string]any{"key": key, "value": value})
	require.NoError(t, err)
	recorder := httptest.NewRecorder()
	requestContext, _ := gin.CreateTestContext(recorder)
	requestContext.Request = httptest.NewRequest(http.MethodPut, "/api/option/", bytes.NewReader(body))
	requestContext.Request.Header.Set("Content-Type", "application/json")
	requestContext.Set("id", 9001)
	requestContext.Set("role", common.RoleRootUser)
	UpdateOption(requestContext)
	var response struct {
		Success bool `json:"success"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	return response.Success
}

func TestChannelAlertSharedConfigurationAPI(t *testing.T) {
	database, endpoint := setupChannelAlertTaskTest(t, func(writer http.ResponseWriter, request *http.Request) {})
	t.Setenv("MONITOR_AI_ANALYSIS_BASE_URL", "https://env.example/v1")
	t.Setenv("MONITOR_AI_ANALYSIS_MODEL", "env-model")
	require.NoError(t, database.Create(&model.Option{Key: "codex_model_governance_setting.ai_analysis_api_key", Value: "obsolete-analysis-key"}).Error)
	require.NoError(t, database.Create(&model.Option{Key: "codex_model_governance_setting.dingtalk_enabled", Value: "true"}).Error)
	model.InitOptionMap()
	assert.Empty(t, operation_setting.GetMonitorAIAnalysisAPIKey())
	assert.False(t, operation_setting.GetMonitorAlertSetting().DingTalkAlertEnabled)
	assert.Equal(t, "https://env.example/v1", operation_setting.GetMonitorAIAnalysisBaseURL())
	assert.Equal(t, "env-model", operation_setting.GetMonitorAIAnalysisModel())
	assert.False(t, putChannelAlertOption(t, "codex_model_governance_setting.ai_analysis_api_key", "rejected"))
	assert.False(t, putChannelAlertOption(t, "monitor_setting.dingtalk_alert_enabled", true))
	for _, invalid := range []any{0, -1, "nan", "1.5", "153722868"} {
		assert.False(t, putChannelAlertOption(t, "monitor_setting.dingtalk_alert_cooldown_minutes", invalid))
	}
	for key, value := range map[string]string{
		"dingtalk_alert_webhook_url": endpoint + "/robot?access_token=robot-token",
		"dingtalk_alert_secret":      "signed-secret", "ai_analysis_api_key": "analysis-secret",
		"ai_analysis_base_url": operation_setting.DefaultMonitorAIAnalysisBaseURL,
		"ai_analysis_model":    operation_setting.DefaultMonitorAIAnalysisModelName,
	} {
		require.True(t, putChannelAlertOption(t, "monitor_setting."+key, value))
	}
	require.True(t, putChannelAlertOption(t, "monitor_setting.dingtalk_alert_enabled", true))
	assert.False(t, putChannelAlertOption(t, "monitor_setting.dingtalk_alert_webhook_url", ""))
	for range 2 {
		model.InitOptionMap()
		assert.Equal(t, operation_setting.DefaultMonitorAIAnalysisBaseURL, operation_setting.GetMonitorAIAnalysisBaseURL())
		assert.Equal(t, operation_setting.DefaultMonitorAIAnalysisModelName, operation_setting.GetMonitorAIAnalysisModel())
		assert.Equal(t, "analysis-secret", operation_setting.GetMonitorAIAnalysisAPIKey())
	}
	recorder := httptest.NewRecorder()
	requestContext, _ := gin.CreateTestContext(recorder)
	requestContext.Request = httptest.NewRequest(http.MethodGet, "/api/option/", nil)
	GetOptions(requestContext)
	for _, secret := range []string{"signed-secret", "analysis-secret", "obsolete-analysis-key"} {
		assert.NotContains(t, recorder.Body.String(), secret)
	}
	assert.NotContains(t, recorder.Body.String(), "codex_model_governance_setting.dingtalk_enabled")
	require.True(t, putChannelAlertOption(t, "monitor_setting.ai_analysis_api_key", ""))
	assert.Empty(t, operation_setting.GetMonitorAIAnalysisAPIKey())
}

func TestChannelAlertTaskBatchesAndCooldownWithoutRetesting(t *testing.T) {
	var upstream atomic.Int32
	var mutex sync.Mutex
	var messages []string
	database, endpoint := setupChannelAlertTaskTest(t, func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/robot" {
			body, err := io.ReadAll(request.Body)
			require.NoError(t, err)
			mutex.Lock()
			messages = append(messages, string(body))
			mutex.Unlock()
			_, _ = io.WriteString(writer, `{"errcode":0}`)
			return
		}
		upstream.Add(1)
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusTooManyRequests)
		_, _ = io.WriteString(writer, `{"error":{"message":"quota access_token=private-token api_key=sk-private-token at https://upstream.example/secret via bare.upstream.example 192.0.2.1","type":"rate_limit_error","code":"rate_limit"}}`)
	})
	for channelID := range 6 {
		channel := model.Channel{Id: channelID + 100, Name: fmt.Sprintf("alert-%d", channelID), Type: constant.ChannelTypeOpenAI, Status: common.ChannelStatusEnabled, BaseURL: &endpoint, Key: "fake-upstream-key", Models: "gpt-4o-mini", Group: "default"}
		require.NoError(t, database.Create(&channel).Error)
	}
	require.True(t, putChannelAlertOption(t, "monitor_setting.dingtalk_alert_webhook_url", endpoint+"/robot"))
	require.True(t, putChannelAlertOption(t, "monitor_setting.dingtalk_alert_enabled", true))
	for range 2 {
		summary, err := runChannelTestTask(context.Background(), operation_setting.ChannelTestModeScheduledAll, false, nil)
		require.NoError(t, err)
		assert.Equal(t, 6, summary.Tested)
		assert.Equal(t, 6, summary.Failed)
		assert.Zero(t, summary.Disabled)
	}
	assert.EqualValues(t, 12, upstream.Load())
	mutex.Lock()
	defer mutex.Unlock()
	require.Len(t, messages, 2)
	assert.Equal(t, 5, strings.Count(messages[0], "Channel ID:"))
	assert.Equal(t, 1, strings.Count(messages[1], "Channel ID:"))
	assert.NotContains(t, strings.Join(messages, ""), "private-token")
	assert.NotContains(t, strings.Join(messages, ""), "upstream.example")
}

func TestChannelAlertTaskOptionalSummaryAndFallback(t *testing.T) {
	for _, response := range []string{
		`{"choices":[{"message":{"content":"- 认证失败 api_key=sk-summary-secret"}}]}`,
		`{"choices":[]}`, `not-json`, strings.Repeat("x", 65537),
	} {
		t.Run(fmt.Sprintf("response_%d", len(response)), func(t *testing.T) {
			var upstream, analyses, deliveries atomic.Int32
			var delivered string
			database, endpoint := setupChannelAlertTaskTest(t, func(writer http.ResponseWriter, request *http.Request) {
				switch request.URL.Path {
				case "/analysis/chat/completions":
					analyses.Add(1)
					assert.Equal(t, "Bearer monitor-analysis-key", request.Header.Get("Authorization"))
					body, err := io.ReadAll(request.Body)
					require.NoError(t, err)
					var payload struct {
						Model     string `json:"model"`
						Stream    bool   `json:"stream"`
						MaxTokens int    `json:"max_completion_tokens"`
						Messages  []struct {
							Content string `json:"content"`
						} `json:"messages"`
					}
					require.NoError(t, common.Unmarshal(body, &payload))
					assert.Equal(t, "configured-analysis-model", payload.Model)
					assert.False(t, payload.Stream)
					assert.Equal(t, 600, payload.MaxTokens)
					require.Len(t, payload.Messages, 2)
					assert.LessOrEqual(t, utf8.RuneCountInString(payload.Messages[1].Content), 12000)
					for _, secret := range []string{"upstream-key", "private-token", "monitor-analysis-key", "robot-secret", "upstream.example", "AKIAIOSFODNN7EXAMPLE", "AIza" + strings.Repeat("G", 35)} {
						assert.NotContains(t, string(body), secret)
					}
					_, _ = io.WriteString(writer, response)
				case "/robot":
					deliveries.Add(1)
					body, err := io.ReadAll(request.Body)
					require.NoError(t, err)
					delivered = string(body)
					_, _ = io.WriteString(writer, `{"errcode":0}`)
				default:
					upstream.Add(1)
					writer.Header().Set("Content-Type", "application/json")
					writer.WriteHeader(http.StatusTooManyRequests)
					_, _ = fmt.Fprintf(writer, `{"error":{"message":"access_token=private-token AKIAIOSFODNN7EXAMPLE AIza%s https://upstream.example/secret %s","type":"rate_limit_error"}}`, strings.Repeat("G", 35), strings.Repeat("故", 13000))
				}
			})
			channel := model.Channel{Id: 201, Type: constant.ChannelTypeCodex, Status: common.ChannelStatusEnabled, Name: "Codex alert", BaseURL: &endpoint, Key: `{"access_token":"upstream-key","account_id":"fake-account"}`, Models: "gpt-5.4-mini", Group: "default"}
			require.NoError(t, database.Create(&channel).Error)
			for key, value := range map[string]string{
				"dingtalk_alert_webhook_url": endpoint + "/robot", "dingtalk_alert_secret": "robot-secret",
				"ai_analysis_api_key": "monitor-analysis-key", "ai_analysis_base_url": endpoint + "/analysis", "ai_analysis_model": "configured-analysis-model",
			} {
				require.True(t, putChannelAlertOption(t, "monitor_setting."+key, value))
			}
			require.True(t, putChannelAlertOption(t, "monitor_setting.dingtalk_alert_enabled", true))
			for range 2 {
				summary, err := runChannelTestTask(context.Background(), operation_setting.ChannelTestModeScheduledAll, false, nil)
				require.NoError(t, err)
				assert.Equal(t, 1, summary.Failed)
			}
			assert.EqualValues(t, 2, upstream.Load())
			assert.EqualValues(t, 1, analyses.Load(), "cooldown filtering must precede analysis")
			assert.EqualValues(t, 1, deliveries.Load())
			assert.Contains(t, delivered, "Channel ID: 201")
			assert.NotContains(t, delivered, "summary-secret")
			if strings.Contains(response, "认证失败") {
				assert.Contains(t, delivered, "AI 中文总结")
			} else {
				assert.NotContains(t, delivered, "AI 中文总结")
			}
		})
	}
}

func TestChannelAlertTaskFailedDeliveryReleasesCooldown(t *testing.T) {
	for _, failure := range []struct {
		name, body string
		status     int
	}{
		{"http", `{"errcode":0}`, http.StatusBadGateway},
		{"rejected", `{"errcode":500}`, http.StatusOK},
		{"missing", `{}`, http.StatusOK},
		{"null", `{"errcode":null}`, http.StatusOK},
		{"malformed", `invalid`, http.StatusOK},
		{"oversized", strings.Repeat("x", 65537), http.StatusOK},
	} {
		t.Run(failure.name, func(t *testing.T) {
			var upstream, deliveries atomic.Int32
			database, endpoint := setupChannelAlertTaskTest(t, func(writer http.ResponseWriter, request *http.Request) {
				writer.Header().Set("Content-Type", "application/json")
				if request.URL.Path == "/robot" {
					if deliveries.Add(1) == 1 {
						writer.WriteHeader(failure.status)
						_, _ = io.WriteString(writer, failure.body)
					} else {
						_, _ = io.WriteString(writer, `{"errcode":0}`)
					}
					return
				}
				upstream.Add(1)
				writer.WriteHeader(http.StatusBadGateway)
				_, _ = io.WriteString(writer, `{"error":{"message":"upstream unavailable"}}`)
			})
			require.NoError(t, database.Create(&model.Channel{Id: 301, Name: "failure", Type: constant.ChannelTypeOpenAI, Status: common.ChannelStatusEnabled, BaseURL: &endpoint, Key: "test-key", Models: "gpt-4o-mini", Group: "default"}).Error)
			require.True(t, putChannelAlertOption(t, "monitor_setting.dingtalk_alert_webhook_url", endpoint+"/robot"))
			require.True(t, putChannelAlertOption(t, "monitor_setting.dingtalk_alert_enabled", true))
			for attempt := range 3 {
				summary, err := runChannelTestTask(context.Background(), operation_setting.ChannelTestModeScheduledAll, false, nil)
				require.NoError(t, err)
				assert.Equal(t, 1, summary.Failed)
				if attempt == 0 {
					var record model.ChannelTestAlertCooldown
					require.NoError(t, database.First(&record, "channel_id = ?", 301).Error)
					assert.Zero(t, record.LastAt)
					assert.Zero(t, record.PendingAt)
				}
			}
			assert.EqualValues(t, 3, upstream.Load())
			assert.EqualValues(t, 2, deliveries.Load(), "no application retry; next test creates the next alert")
		})
	}
}

func TestChannelAlertTaskCancellationAbortsHTTPAndDropsTail(t *testing.T) {
	for _, stage := range []string{"analysis", "robot"} {
		t.Run(stage, func(t *testing.T) {
			started, aborted := make(chan struct{}), make(chan struct{})
			release := make(chan struct{})
			defer close(release)
			var deliveries atomic.Int32
			database, endpoint := setupChannelAlertTaskTest(t, func(writer http.ResponseWriter, request *http.Request) {
				if request.URL.Path == "/robot" || request.URL.Path == "/analysis/chat/completions" {
					_, _ = io.Copy(io.Discard, request.Body)
					if request.URL.Path == "/robot" {
						deliveries.Add(1)
					}
					close(started)
					select {
					case <-request.Context().Done():
						close(aborted)
					case <-release:
					}
					return
				}
				writer.Header().Set("Content-Type", "application/json")
				writer.WriteHeader(http.StatusBadGateway)
				_, _ = io.WriteString(writer, `{"error":{"message":"upstream unavailable"}}`)
			})
			for channelID := range 6 {
				require.NoError(t, database.Create(&model.Channel{Id: channelID + 401, Name: "cancel", Type: constant.ChannelTypeOpenAI, Status: common.ChannelStatusEnabled, BaseURL: &endpoint, Key: "test-key", Models: "gpt-4o-mini", Group: "default"}).Error)
			}
			require.True(t, putChannelAlertOption(t, "monitor_setting.dingtalk_alert_webhook_url", endpoint+"/robot"))
			if stage == "analysis" {
				require.True(t, putChannelAlertOption(t, "monitor_setting.ai_analysis_base_url", endpoint+"/analysis"))
				require.True(t, putChannelAlertOption(t, "monitor_setting.ai_analysis_api_key", "test-analysis-key"))
			}
			require.True(t, putChannelAlertOption(t, "monitor_setting.dingtalk_alert_enabled", true))
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan error, 1)
			go func() {
				_, err := runChannelTestTask(ctx, operation_setting.ChannelTestModeScheduledAll, false, nil)
				done <- err
			}()
			select {
			case <-started:
			case <-time.After(5 * time.Second):
				t.Fatal("alert HTTP did not start")
			}
			cancel()
			select {
			case err := <-done:
				require.NoError(t, err)
			case <-time.After(5 * time.Second):
				t.Fatal("task did not honor cancellation")
			}
			select {
			case <-aborted:
			case <-time.After(5 * time.Second):
				t.Fatal("external HTTP did not observe cancellation")
			}
			var pending, committed int64
			require.NoError(t, database.Model(&model.ChannelTestAlertCooldown{}).Where("pending_at <> 0").Count(&pending).Error)
			require.NoError(t, database.Model(&model.ChannelTestAlertCooldown{}).Where("last_at <> 0").Count(&committed).Error)
			assert.Zero(t, pending)
			assert.Zero(t, committed)
			if stage == "analysis" {
				assert.Zero(t, deliveries.Load())
			} else {
				assert.EqualValues(t, 1, deliveries.Load())
			}
		})
	}
}

func TestChannelAlertTaskPreservesModesAndManualBehavior(t *testing.T) {
	for _, scenario := range []struct {
		mode            string
		notify, enabled bool
		tested, alerted int
	}{
		{operation_setting.ChannelTestModeScheduledAll, false, true, 3, 3},
		{operation_setting.ChannelTestModeAutoBanOnly, false, true, 2, 2},
		{operation_setting.ChannelTestModePassiveRecovery, false, true, 1, 1},
		{operation_setting.ChannelTestModeScheduledAll, true, true, 3, 0},
		{operation_setting.ChannelTestModeScheduledAll, false, false, 3, 0},
	} {
		t.Run(fmt.Sprintf("%s_manual_%t_enabled_%t", scenario.mode, scenario.notify, scenario.enabled), func(t *testing.T) {
			var upstream, alerted atomic.Int32
			database, endpoint := setupChannelAlertTaskTest(t, func(writer http.ResponseWriter, request *http.Request) {
				if request.URL.Path == "/robot" {
					body, err := io.ReadAll(request.Body)
					require.NoError(t, err)
					alerted.Add(int32(strings.Count(string(body), "Channel ID:")))
					_, _ = io.WriteString(writer, `{"errcode":0}`)
					return
				}
				upstream.Add(1)
				writer.Header().Set("Content-Type", "application/json")
				writer.WriteHeader(http.StatusTooManyRequests)
				_, _ = io.WriteString(writer, `{"error":{"message":"rate limit"}}`)
			})
			for channelID, channel := range []model.Channel{
				{Status: common.ChannelStatusEnabled, AutoBan: common.GetPointer(1)},
				{Status: common.ChannelStatusEnabled, AutoBan: common.GetPointer(0)},
				{Status: common.ChannelStatusAutoDisabled, AutoBan: common.GetPointer(1)},
				{Status: common.ChannelStatusManuallyDisabled, AutoBan: common.GetPointer(1)},
			} {
				channel.Id, channel.Type, channel.BaseURL, channel.Key, channel.Models, channel.Group = channelID+501, constant.ChannelTypeOpenAI, &endpoint, "test-key", "gpt-4o-mini", "default"
				require.NoError(t, database.Create(&channel).Error)
			}
			require.True(t, putChannelAlertOption(t, "monitor_setting.dingtalk_alert_webhook_url", endpoint+"/robot"))
			require.True(t, putChannelAlertOption(t, "monitor_setting.dingtalk_alert_enabled", scenario.enabled))
			summary, err := runChannelTestTask(context.Background(), scenario.mode, scenario.notify, nil)
			require.NoError(t, err)
			assert.Equal(t, scenario.tested, summary.Tested)
			assert.EqualValues(t, scenario.tested, upstream.Load())
			assert.EqualValues(t, scenario.alerted, alerted.Load())
			assert.Zero(t, summary.Disabled)
		})
	}
}

func TestChannelAlertOptionsRemainRootOnly(t *testing.T) {
	user, _ := setupAccessTokenAudit(t)
	router := gin.New()
	router.PUT("/api/option/", middleware.RootAuth(), UpdateOption)
	for _, role := range []int{common.RoleCommonUser, common.RoleAdminUser} {
		require.NoError(t, model.DB.Model(user).Update("role", role).Error)
		_, token := createAccessTokenTestSession(t, user.Id, fmt.Sprintf("monitor-role-%d", role))
		response := accessTokenRequest(router, http.MethodPut, "/api/option/", token, "", `{"key":"monitor_setting.ai_analysis_api_key","value":"forbidden-key"}`)
		assert.Equal(t, http.StatusForbidden, response.Code)
	}
	var saved int64
	require.NoError(t, model.DB.Model(&model.Option{}).Where(&model.Option{Key: "monitor_setting.ai_analysis_api_key"}).Count(&saved).Error)
	assert.Zero(t, saved)
}
