package controller

import (
	"bytes"
	"context"
	"errors"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	perfmetrics "github.com/QuantumNous/new-api/pkg/perf_metrics"
	"github.com/QuantumNous/new-api/relay"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type streamingTestTransport func(*http.Request) (*http.Response, error)

type streamingRelayScenario struct {
	name           string
	ping           bool
	retries        int
	succeedOnRetry bool
	preludeFailure bool
	changePolicy   bool
	delayHeaders   bool
	cancelRequest  bool
	wantStatus     int
	wantAttempts   int
}

func (transport streamingTestTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return transport(request)
}

func TestRelayFirstEventTimeoutReturnsGatewayError(test *testing.T) {
	for _, scenario := range []streamingRelayScenario{
		{name: "uncommitted silence", wantStatus: 504, wantAttempts: 1},
		{name: "committed ping forbids retry", ping: true, retries: 1, wantStatus: 200, wantAttempts: 1},
		{name: "ordinary provider failure still retries successfully", retries: 1, preludeFailure: true, succeedOnRetry: true, wantStatus: 200, wantAttempts: 2},
		{name: "request policy survives changes across retries", retries: 1, preludeFailure: true, changePolicy: true, wantStatus: 504, wantAttempts: 2},
		{name: "existing hard 504 exclusion remains authoritative", retries: 1, wantStatus: 504, wantAttempts: 1},
		{name: "header wait does not spend first event budget", delayHeaders: true, wantStatus: 504, wantAttempts: 1},
		{name: "cancelled client releases the concurrency slot", cancelRequest: true, wantStatus: 200, wantAttempts: 1},
	} {
		test.Run(scenario.name, func(test *testing.T) { testStreamingRelayTimeout(test, scenario) })
	}
}

func testStreamingRelayTimeout(test *testing.T, scenario streamingRelayScenario) {
	test.Helper()
	database := setupRelayRetryRouting(test, &model.User{})
	require.NoError(test, database.Create(&model.User{Id: 1, Username: "streaming-relay-user", Group: "default"}).Error)
	previousRatios := ratio_setting.ModelRatio2JSONString()
	previousCountToken, previousLogConsume, previousBatch := constant.CountToken, common.LogConsumeEnabled, common.BatchUpdateEnabled
	previousFreePreConsume := operation_setting.GetQuotaSetting().EnableFreeModelPreConsume
	previousPing := *operation_setting.GetGeneralSetting()
	previousIdleTimeout := constant.StreamingTimeout
	previousRetryStatuses := operation_setting.AutomaticRetryStatusCodesToString()
	require.NoError(test, operation_setting.AutomaticRetryStatusCodesFromString("503-504"))
	constant.StreamingTimeout = 30
	settings := config.GlobalConfig.Get("streaming_setting")
	previousSettings, err := config.ConfigToMap(settings)
	require.NoError(test, err)
	require.NoError(test, ratio_setting.UpdateModelRatioByJSONString(`{"streaming-timeout-model":0}`))
	require.NoError(test, config.UpdateConfigFromMap(settings, map[string]string{
		"first_response_timeout_enabled": "true", "first_response_timeout_seconds": "1",
	}))
	constant.CountToken, common.LogConsumeEnabled, common.BatchUpdateEnabled = false, false, false
	operation_setting.GetQuotaSetting().EnableFreeModelPreConsume = false
	operation_setting.GetGeneralSetting().PingIntervalEnabled = false
	if scenario.ping {
		operation_setting.GetGeneralSetting().PingIntervalEnabled = true
		operation_setting.GetGeneralSetting().PingIntervalSeconds = 1
		require.NoError(test, config.UpdateConfigFromMap(settings, map[string]string{"first_response_timeout_seconds": "3"}))
	}
	common.RetryTimes = scenario.retries
	service.InitHttpClient()
	client := service.GetHttpClient()
	previousTransport := client.Transport
	test.Cleanup(func() {
		client.Transport = previousTransport
		require.NoError(test, ratio_setting.UpdateModelRatioByJSONString(previousRatios))
		require.NoError(test, config.UpdateConfigFromMap(settings, previousSettings))
		constant.CountToken, common.LogConsumeEnabled, common.BatchUpdateEnabled = previousCountToken, previousLogConsume, previousBatch
		operation_setting.GetQuotaSetting().EnableFreeModelPreConsume = previousFreePreConsume
		*operation_setting.GetGeneralSetting() = previousPing
		constant.StreamingTimeout = previousIdleTimeout
		require.NoError(test, operation_setting.AutomaticRetryStatusCodesFromString(previousRetryStatuses))
	})
	withTieredBillingConfig(test, map[string]string{}, map[string]string{})
	channel := &model.Channel{Name: "timeout-channel", Type: constant.ChannelTypeOpenAI, Key: "upstream-secret", Status: common.ChannelStatusEnabled, Group: "default", Models: "streaming-timeout-model", BaseURL: common.GetPointer("http://streaming.example"), MaxConcurrency: 1, Priority: common.GetPointer(int64(2))}
	require.NoError(test, database.Create(channel).Error)
	require.NoError(test, channel.AddAbilities(nil))
	channels := []*model.Channel{channel}
	if scenario.retries > 0 {
		fallback := &model.Channel{Name: "fallback-channel", Type: constant.ChannelTypeOpenAI, Key: "fallback-secret", Status: common.ChannelStatusEnabled, Group: "default", Models: "streaming-timeout-model", BaseURL: common.GetPointer("http://streaming.example"), MaxConcurrency: 1, Priority: common.GetPointer(int64(1))}
		require.NoError(test, database.Create(fallback).Error)
		require.NoError(test, fallback.AddAbilities(nil))
		channels = append(channels, fallback)
	}
	model.InitChannelCache()

	synctest.Test(test, func(test *testing.T) {
		var writers []*io.PipeWriter
		defer func() {
			for _, writer := range writers {
				_ = writer.Close()
			}
		}()
		calls := 0
		headerGate := make(chan struct{})
		client.Transport = streamingTestTransport(func(request *http.Request) (*http.Response, error) {
			calls++
			if scenario.delayHeaders {
				<-headerGate
			}
			if scenario.changePolicy && calls == 1 {
				require.NoError(test, config.UpdateConfigFromMap(settings, map[string]string{"first_response_timeout_enabled": "false", "first_response_timeout_seconds": "600"}))
			}
			if scenario.preludeFailure && calls == 1 {
				return &http.Response{StatusCode: 503, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"error":{"message":"provider unavailable","type":"server_error","code":"provider_unavailable"}}`)), Request: request}, nil
			}
			if scenario.succeedOnRetry && calls > 1 {
				return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"text/event-stream"}}, Body: io.NopCloser(strings.NewReader("data: {\"id\":\"chatcmpl-retry-success\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"ok\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")), Request: request}, nil
			}
			reader, writer := io.Pipe()
			writers = append(writers, writer)
			return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"text/event-stream"}}, Body: reader, Request: request}, nil
		})
		var policy *service.RequestPolicyState
		router := gin.New()
		router.POST("/v1/chat/completions", func(requestContext *gin.Context) {
			common.SetContextKey(requestContext, constant.ContextKeyUsingGroup, "default")
			common.SetContextKey(requestContext, constant.ContextKeyUserGroup, "default")
			common.SetContextKey(requestContext, constant.ContextKeyUserId, 1)
			common.SetContextKey(requestContext, constant.ContextKeyOriginalModel, "streaming-timeout-model")
			require.Nil(test, middleware.SetupContextForSelectedChannel(requestContext, channel, "streaming-timeout-model"))
			Relay(requestContext, types.RelayFormatOpenAI)
			policy = service.RequestPolicy(requestContext)
		})
		request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"streaming-timeout-model","stream":true,"messages":[{"role":"user","content":"hello"}]}`))
		request.Header.Set("Content-Type", "application/json")
		requestContext, cancel := context.WithCancel(request.Context())
		defer cancel()
		request = request.WithContext(requestContext)
		recorder := httptest.NewRecorder()
		if scenario.delayHeaders || scenario.cancelRequest {
			finished := make(chan struct{})
			go func() { router.ServeHTTP(recorder, request); close(finished) }()
			synctest.Wait()
			if scenario.delayHeaders {
				<-time.NewTimer(2 * time.Second).C
				synctest.Wait()
				select {
				case <-finished:
					test.Fatal("first-event budget ran before upstream response headers")
				default:
				}
				close(headerGate)
			} else {
				cancel()
			}
			<-finished
		} else {
			router.ServeHTTP(recorder, request)
		}

		assert.Equal(test, scenario.wantStatus, recorder.Code, recorder.Body.String())
		if scenario.wantStatus == http.StatusGatewayTimeout {
			assert.Contains(test, recorder.Header().Get("Content-Type"), "application/json")
			assert.Contains(test, recorder.Body.String(), "upstream_first_response_timeout")
			assert.Empty(test, recorder.Header().Get("Transfer-Encoding"))
		} else if scenario.cancelRequest {
			assert.Empty(test, recorder.Body.String())
		} else if scenario.ping {
			assert.Contains(test, recorder.Body.String(), ": PING")
			assert.NotContains(test, recorder.Body.String(), `"error"`)
		} else {
			assert.Contains(test, recorder.Body.String(), "chatcmpl-retry-success")
		}
		if !scenario.succeedOnRetry {
			assert.NotContains(test, recorder.Body.String(), "[DONE]")
		}
		require.NotNil(test, policy)
		assert.Equal(test, scenario.succeedOnRetry, policy.Successful)
		for _, event := range policy.Events() {
			if event.ErrorCode == "upstream_first_response_timeout" {
				assert.Equal(test, "upstream", event.ErrorSource)
				assert.NotEqual(test, "local_rejection", event.Decision.Reason)
			}
		}
		for _, writer := range writers {
			_, err := writer.Write([]byte("data: late\n\n"))
			assert.Error(test, err)
		}
		assert.Equal(test, scenario.wantAttempts, policy.Attempts, "%+v", policy.Events())
		assert.Equal(test, scenario.wantAttempts, calls)
		loads, err := service.GetChannelConcurrencyLoadsFresh(context.Background(), channels)
		require.NoError(test, err)
		for _, channel := range channels {
			assert.Zero(test, loads[channel.Id].Active)
		}
	})
}

func streamingOptionRequest(test *testing.T, key string, value any) bool {
	test.Helper()
	payload, err := common.Marshal(OptionUpdateRequest{Key: key, Value: value})
	require.NoError(test, err)
	recorder := httptest.NewRecorder()
	requestContext, _ := gin.CreateTestContext(recorder)
	requestContext.Set("id", 1)
	requestContext.Set("role", common.RoleRootUser)
	requestContext.Request = httptest.NewRequest(http.MethodPut, "/api/option/", bytes.NewReader(payload))
	UpdateOption(requestContext)
	var response struct {
		Success bool `json:"success"`
	}
	require.NoError(test, common.Unmarshal(recorder.Body.Bytes(), &response), recorder.Body.String())
	return response.Success
}

func setupStreamingOptionsDatabase(test *testing.T, dialect, dsn string) *gorm.DB {
	test.Helper()
	database, _ := newAuditTestDatabase(test, dialect, dsn)
	require.NoError(test, database.AutoMigrate(&model.Option{}, &model.User{}, &model.AuditLog{}))
	require.NoError(test, database.Create(&model.User{Id: 1, Username: "streaming-settings-root", Role: common.RoleRootUser}).Error)
	previousDB, previousLogDB := model.DB, model.LOG_DB
	previousMainType, previousLogType := common.MainDatabaseType(), common.LogDatabaseType()
	common.SetDatabaseTypes(common.DatabaseType(dialect), common.DatabaseType(dialect))
	previousRedis := common.RedisEnabled
	common.RedisEnabled = false
	common.OptionMapRWMutex.Lock()
	previousOptions := maps.Clone(common.OptionMap)
	common.OptionMap = map[string]string{}
	common.OptionMapRWMutex.Unlock()
	settings := config.GlobalConfig.Get("streaming_setting")
	previousSettings, err := config.ConfigToMap(settings)
	require.NoError(test, err)
	model.DB = database
	model.LOG_DB = database
	test.Cleanup(func() {
		model.DB, model.LOG_DB = previousDB, previousLogDB
		common.SetDatabaseTypes(previousMainType, previousLogType)
		common.RedisEnabled = previousRedis
		common.OptionMapRWMutex.Lock()
		common.OptionMap = previousOptions
		common.OptionMapRWMutex.Unlock()
		if settings != nil {
			require.NoError(test, config.UpdateConfigFromMap(settings, previousSettings))
		}
	})
	return database
}

func TestStreamingOptionsRejectInvalidUpdates(test *testing.T) {
	setupStreamingOptionsDatabase(test, "sqlite", "")
	const secondsKey = "streaming_setting.first_response_timeout_seconds"
	require.True(test, streamingOptionRequest(test, secondsKey, "600"))
	for _, invalid := range []any{"0", "-1", "1.5", "not a number", "9223372037", "9223372036854775808", 0, -1, 1.5, true, nil} {
		assert.False(test, streamingOptionRequest(test, secondsKey, invalid), "invalid seconds: %v", invalid)
	}
	for _, invalid := range []any{"yes", "1", 1, nil, map[string]string{"enabled": "true"}} {
		assert.False(test, streamingOptionRequest(test, "streaming_setting.first_response_timeout_enabled", invalid), "invalid boolean: %v", invalid)
	}
	common.OptionMapRWMutex.RLock()
	assert.Equal(test, "600", common.OptionMap[secondsKey])
	common.OptionMapRWMutex.RUnlock()
}

func streamingOptionsReadback(test *testing.T) map[string]string {
	test.Helper()
	recorder := httptest.NewRecorder()
	requestContext, _ := gin.CreateTestContext(recorder)
	requestContext.Request = httptest.NewRequest(http.MethodGet, "/api/option/", nil)
	GetOptions(requestContext)
	var response struct {
		Success bool           `json:"success"`
		Data    []model.Option `json:"data"`
	}
	require.NoError(test, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(test, response.Success)
	values := make(map[string]string)
	for _, option := range response.Data {
		if strings.HasPrefix(option.Key, "streaming_setting.") {
			values[option.Key] = option.Value
		}
	}
	return values
}

func TestStreamingOptionsDatabaseMatrix(test *testing.T) {
	for _, dialect := range []struct{ name, env string }{{"sqlite", ""}, {"mysql", "TEST_MYSQL_DSN"}, {"postgres", "TEST_POSTGRES_DSN"}} {
		test.Run(dialect.name, func(test *testing.T) {
			if dialect.env != "" && os.Getenv(dialect.env) == "" {
				test.Skip("set " + dialect.env + " for the database matrix")
			}
			database := setupStreamingOptionsDatabase(test, dialect.name, os.Getenv(dialect.env))
			query := "SELECT version()"
			if dialect.name == "sqlite" {
				query = "SELECT sqlite_version()"
			}
			var version string
			require.NoError(test, database.Raw(query).Scan(&version).Error)
			test.Logf("database=%s version=%s", dialect.name, version)
			settings := config.GlobalConfig.Get("streaming_setting")
			require.NoError(test, config.UpdateConfigFromMap(settings, map[string]string{"first_response_timeout_enabled": "false", "first_response_timeout_seconds": "600"}))
			model.InitOptionMap()
			assert.Equal(test, map[string]string{"streaming_setting.first_response_timeout_enabled": "false", "streaming_setting.first_response_timeout_seconds": "600"}, streamingOptionsReadback(test))
			require.True(test, streamingOptionRequest(test, "streaming_setting.first_response_timeout_seconds", 1200))
			require.True(test, streamingOptionRequest(test, "streaming_setting.first_response_timeout_enabled", true))
			require.True(test, streamingOptionRequest(test, "streaming_setting.first_response_timeout_enabled", false))
			assert.Equal(test, "1200", streamingOptionsReadback(test)["streaming_setting.first_response_timeout_seconds"])
			require.True(test, streamingOptionRequest(test, "streaming_setting.first_response_timeout_enabled", true))
			assert.False(test, streamingOptionRequest(test, "streaming_setting.first_response_timeout_seconds", "0"))
			require.NoError(test, config.UpdateConfigFromMap(settings, map[string]string{"first_response_timeout_enabled": "false", "first_response_timeout_seconds": "600"}))
			model.InitOptionMap()
			model.InitOptionMap()
			assert.Equal(test, map[string]string{"streaming_setting.first_response_timeout_enabled": "true", "streaming_setting.first_response_timeout_seconds": "1200"}, streamingOptionsReadback(test))
			assert.EqualValues(test, 1200, operation_setting.GetStreamingSetting().FirstResponseTimeoutSeconds)
			assert.True(test, operation_setting.GetStreamingSetting().FirstResponseTimeoutEnabled)
			test.Run("save survives stale reload", func(test *testing.T) {
				readOldOptions := make(chan struct{})
				releaseReload := make(chan struct{}, 1)
				reloadFinished := make(chan struct{})
				require.NoError(test, database.Callback().Query().After("gorm:query").Register("test:streaming-stale-reload", func(transaction *gorm.DB) {
					if _, ok := transaction.Statement.Dest.(*[]*model.Option); ok {
						close(readOldOptions)
						<-releaseReload
					}
				}))
				test.Cleanup(func() {
					close(releaseReload)
					<-reloadFinished
					require.NoError(test, database.Callback().Query().Remove("test:streaming-stale-reload"))
				})
				go func() {
					model.InitOptionMap()
					close(reloadFinished)
				}()
				<-readOldOptions
				saved := streamingOptionRequest(test, "streaming_setting.first_response_timeout_seconds", 1800)
				releaseReload <- struct{}{}
				<-reloadFinished
				require.True(test, saved)
				assert.Equal(test, "1800", streamingOptionsReadback(test)["streaming_setting.first_response_timeout_seconds"])
				assert.Equal(test, 1800*time.Second, operation_setting.GetStreamingSetting().FirstResponseTimeout())
			})
			require.NoError(test, config.UpdateConfigFromMap(settings, map[string]string{"first_response_timeout_enabled": "false"}))
			require.NoError(test, database.Model(&model.Option{}).Where(&model.Option{Key: "streaming_setting.first_response_timeout_seconds"}).Update("value", "0").Error)
			model.InitOptionMap()
			assert.False(test, operation_setting.GetStreamingSetting().FirstResponseTimeoutEnabled, "invalid synchronized policy must not partially enable protection")
			assert.Equal(test, "false", streamingOptionsReadback(test)["streaming_setting.first_response_timeout_enabled"])
		})
	}
}

func TestStreamingOptionsSaveFailureKeepsLastPolicy(test *testing.T) {
	database := setupStreamingOptionsDatabase(test, "sqlite", "")
	require.True(test, streamingOptionRequest(test, "streaming_setting.first_response_timeout_seconds", 600))
	require.NoError(test, database.Callback().Update().Before("gorm:update").Register("test:streaming-save-failure", func(transaction *gorm.DB) {
		transaction.AddError(errors.New("settings storage unavailable"))
	}))
	assert.False(test, streamingOptionRequest(test, "streaming_setting.first_response_timeout_seconds", 1200))
	assert.Equal(test, "600", streamingOptionsReadback(test)["streaming_setting.first_response_timeout_seconds"])
	assert.EqualValues(test, 600, operation_setting.GetStreamingSetting().FirstResponseTimeoutSeconds)
	require.NoError(test, database.Callback().Update().Remove("test:streaming-save-failure"))
	model.InitOptionMap()
	assert.Equal(test, "600", streamingOptionsReadback(test)["streaming_setting.first_response_timeout_seconds"])
	require.NoError(test, config.UpdateConfigFromMap(config.GlobalConfig.Get("streaming_setting"), map[string]string{"first_response_timeout_enabled": "false"}))
	require.NoError(test, database.Create(&model.Option{Key: "streaming_setting.first_response_timeout_enabled", Value: "true"}).Error)
	require.NoError(test, database.Callback().Query().After("gorm:query").Register("test:streaming-sync-failure", func(transaction *gorm.DB) {
		if transaction.Statement.Table == "options" {
			transaction.AddError(errors.New("incomplete option read"))
		}
	}))
	model.InitOptionMap()
	assert.False(test, operation_setting.GetStreamingSetting().FirstResponseTimeoutEnabled)
	require.NoError(test, database.Callback().Query().Remove("test:streaming-sync-failure"))
}

func TestStreamingOptionsRequireRootAuthorization(test *testing.T) {
	_, token := setupAccessTokenAudit(test)
	router := gin.New()
	router.GET("/api/option/", middleware.RootAuth(), GetOptions)
	router.PUT("/api/option/", middleware.RootAuth(), UpdateOption)
	for _, method := range []string{http.MethodGet, http.MethodPut} {
		assert.Equal(test, http.StatusForbidden, auditRequest(router, method, "/api/option/", token).Code)
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(method, "/api/option/", nil))
		assert.Equal(test, http.StatusUnauthorized, recorder.Code)
	}
}

func TestStreamingAdaptorsPropagateFirstEventTimeout(test *testing.T) {
	previousTimeout := constant.StreamingTimeout
	previousPing := *operation_setting.GetGeneralSetting()
	operation_setting.GetGeneralSetting().PingIntervalEnabled = true
	operation_setting.GetGeneralSetting().PingIntervalSeconds = 1
	constant.StreamingTimeout = 30
	test.Cleanup(func() {
		constant.StreamingTimeout = previousTimeout
		*operation_setting.GetGeneralSetting() = previousPing
	})
	for _, scenario := range []struct {
		name        string
		channelType int
		format      types.RelayFormat
		mode        int
		committed   bool
		bridge      func(*gin.Context, *relaycommon.RelayInfo, *http.Response) (*dto.Usage, *types.NewAPIError)
	}{
		{name: "OpenAI chat", channelType: constant.ChannelTypeOpenAI},
		{name: "OpenAI Responses", channelType: constant.ChannelTypeOpenAI, mode: relayconstant.RelayModeResponses},
		{name: "OpenAI images", channelType: constant.ChannelTypeOpenAI, mode: relayconstant.RelayModeImagesGenerations},
		{name: "OpenAI speech", channelType: constant.ChannelTypeOpenAI, mode: relayconstant.RelayModeAudioSpeech},
		{name: "Claude native", channelType: constant.ChannelTypeAnthropic, format: types.RelayFormatClaude},
		{name: "Claude to chat", channelType: constant.ChannelTypeAnthropic},
		{name: "Claude to Responses", channelType: constant.ChannelTypeAnthropic, format: types.RelayFormatOpenAIResponses},
		{name: "Gemini chat", channelType: constant.ChannelTypeGemini},
		{name: "Gemini Responses", channelType: constant.ChannelTypeGemini, format: types.RelayFormatOpenAIResponses, mode: relayconstant.RelayModeResponses},
		{name: "Baidu", channelType: constant.ChannelTypeBaidu},
		{name: "Dify", channelType: constant.ChannelTypeDify},
		{name: "xAI", channelType: constant.ChannelTypeXai},
		{name: "chat to Responses", bridge: openai.OaiChatToResponsesStreamHandler},
		{name: "Responses to chat", bridge: openai.OaiResponsesToChatStreamHandler},
		{name: "committed native Responses error", channelType: constant.ChannelTypeOpenAI, format: types.RelayFormatOpenAIResponses, mode: relayconstant.RelayModeResponses, committed: true},
		{name: "committed Claude error frame", channelType: constant.ChannelTypeAnthropic, format: types.RelayFormatClaude, committed: true},
		{name: "committed Claude to Responses failure", channelType: constant.ChannelTypeAnthropic, format: types.RelayFormatOpenAIResponses, committed: true},
		{name: "committed Gemini to Responses failure", channelType: constant.ChannelTypeGemini, format: types.RelayFormatOpenAIResponses, mode: relayconstant.RelayModeResponses, committed: true},
		{name: "committed chat to Responses failure", bridge: openai.OaiChatToResponsesStreamHandler, committed: true},
	} {
		test.Run(scenario.name, func(test *testing.T) {
			synctest.Test(test, func(test *testing.T) {
				reader, writer := io.Pipe()
				defer writer.Close()
				recorder := httptest.NewRecorder()
				requestContext, _ := gin.CreateTestContext(recorder)
				requestContext.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
				format := scenario.format
				if format == "" {
					format = types.RelayFormatOpenAI
				}
				info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{ChannelType: scenario.channelType, UpstreamModelName: "timeout-model"}, RelayFormat: format, RelayMode: scenario.mode, IsStream: true, DisablePing: true, FirstResponseTimeout: time.Second}
				if scenario.committed {
					info.DisablePing = false
					info.FirstResponseTimeout = 3 * time.Second
				}
				response := &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"text/event-stream"}}, Body: reader}
				var apiErr *types.NewAPIError
				if scenario.bridge != nil {
					_, apiErr = scenario.bridge(requestContext, info, response)
				} else {
					apiType, ok := common.ChannelType2APIType(scenario.channelType)
					require.True(test, ok)
					adaptor := relay.GetAdaptor(apiType)
					require.NotNil(test, adaptor)
					adaptor.Init(info)
					_, apiErr = adaptor.DoResponse(requestContext, response, info)
				}
				require.NotNil(test, apiErr)
				assert.Equal(test, http.StatusGatewayTimeout, apiErr.StatusCode)
				assert.Equal(test, "upstream_first_response_timeout", string(apiErr.GetErrorCode()))
				if scenario.committed {
					assert.Contains(test, recorder.Body.String(), ": PING")
					if scenario.format == types.RelayFormatClaude || scenario.channelType == constant.ChannelTypeOpenAI {
						assert.Contains(test, recorder.Body.String(), "event: error")
						assert.Contains(test, recorder.Body.String(), "upstream_first_response_timeout")
					} else {
						assert.Contains(test, recorder.Body.String(), "response.failed")
					}
					assert.NotContains(test, recorder.Body.String(), "response.completed")
					assert.NotContains(test, recorder.Body.String(), "[DONE]")
				} else {
					assert.Empty(test, recorder.Body.String(), "timeout must bypass normal finalizers")
					assert.False(test, requestContext.Writer.Written())
				}
				assert.Equal(test, perfmetrics.OutcomeFailure, perfmetrics.ClassifyRelayOutcome(context.Background(), info, apiErr))
			})
		})
	}
}

func TestClaudeApplicationHeartbeatDoesNotSatisfyFirstEvent(test *testing.T) {
	previousTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	test.Cleanup(func() { constant.StreamingTimeout = previousTimeout })
	for _, format := range []types.RelayFormat{types.RelayFormatClaude, types.RelayFormatOpenAI, types.RelayFormatOpenAIResponses, types.RelayFormatGemini} {
		test.Run(string(format), func(test *testing.T) {
			synctest.Test(test, func(test *testing.T) {
				reader, writer := io.Pipe()
				defer writer.Close()
				recorder := httptest.NewRecorder()
				requestContext, _ := gin.CreateTestContext(recorder)
				requestContext.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
				info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{ChannelType: constant.ChannelTypeAnthropic, UpstreamModelName: "claude-timeout"}, RelayFormat: format, IsStream: true, DisablePing: true, FirstResponseTimeout: time.Second}
				response := &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"text/event-stream"}}, Body: reader}
				apiType, ok := common.ChannelType2APIType(constant.ChannelTypeAnthropic)
				require.True(test, ok)
				adaptor := relay.GetAdaptor(apiType)
				adaptor.Init(info)
				go func() { _, _ = io.WriteString(writer, "event: ping\ndata: {\"type\":\"ping\"}\n\n") }()
				_, apiErr := adaptor.DoResponse(requestContext, response, info)
				require.NotNil(test, apiErr)
				assert.Equal(test, "upstream_first_response_timeout", string(apiErr.GetErrorCode()))
				assert.Equal(test, relaycommon.StreamEndReasonFirstResponseTimeout, info.StreamStatus.EndReason)
				assert.NotContains(test, recorder.Body.String(), "message_stop")
				assert.NotContains(test, recorder.Body.String(), "response.completed")
				assert.NotContains(test, recorder.Body.String(), "[DONE]")
				if format == types.RelayFormatClaude {
					assert.Contains(test, recorder.Body.String(), `"type":"ping"`)
					assert.Contains(test, recorder.Body.String(), "event: error")
				}
			})
		})
	}
}

func TestStreamingProtocolProgressAndErrorsKeepTheirMeaning(test *testing.T) {
	previousTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	test.Cleanup(func() { constant.StreamingTimeout = previousTimeout })
	for _, scenario := range []struct {
		name        string
		channelType int
		data        string
		wantError   string
	}{
		{name: "Claude metadata starts response", data: `{"type":"message_start","message":{"id":"msg_meta","role":"assistant","model":"claude-test","content":[],"usage":{"input_tokens":2,"output_tokens":0}}}`},
		{name: "Claude thinking is progress", data: `{"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"reason about ping"}}`},
		{name: "Claude tool call is progress", data: `{"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"tool_1","name":"lookup","input":{}}}`},
		{name: "Claude accepts unfamiliar events", data: `{"type":"future_event","note":"ping"}`},
		{name: "OpenAI upstream is not classified by Claude downstream", channelType: constant.ChannelTypeOpenAI, data: `{"type":"ping"}`},
		{name: "actual Claude error", data: `{"type":"error","error":{"type":"overloaded_error","message":"provider overloaded"}}`, wantError: "overloaded_error"},
		{name: "error overrides ping-shaped envelope", data: `{"type":"ping","error":{"type":"overloaded_error","message":"provider overloaded"}}`, wantError: "overloaded_error"},
		{name: "malformed Claude payload keeps parsing error", data: `{"type":"ping"`, wantError: "bad_response_body"},
	} {
		test.Run(scenario.name, func(test *testing.T) {
			synctest.Test(test, func(test *testing.T) {
				reader, writer := io.Pipe()
				defer writer.Close()
				recorder := httptest.NewRecorder()
				requestContext, _ := gin.CreateTestContext(recorder)
				requestContext.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
				channelType := scenario.channelType
				if channelType == 0 {
					channelType = constant.ChannelTypeAnthropic
				}
				info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{ChannelType: channelType, UpstreamModelName: "claude-test"}, RelayFormat: types.RelayFormatClaude, IsStream: true, DisablePing: true, FirstResponseTimeout: time.Second}
				info.ClaudeConvertInfo = &relaycommon.ClaudeConvertInfo{}
				response := &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"text/event-stream"}}, Body: reader}
				apiType, ok := common.ChannelType2APIType(channelType)
				require.True(test, ok)
				adaptor := relay.GetAdaptor(apiType)
				adaptor.Init(info)
				finished := make(chan *types.NewAPIError, 1)
				go func() { _, apiErr := adaptor.DoResponse(requestContext, response, info); finished <- apiErr }()
				synctest.Wait()
				_, err := io.WriteString(writer, "data: "+scenario.data+"\n\n")
				require.NoError(test, err)
				synctest.Wait()
				if scenario.wantError != "" {
					apiErr := <-finished
					require.NotNil(test, apiErr)
					assert.Equal(test, scenario.wantError, string(apiErr.GetErrorCode()))
					assert.NotEqual(test, relaycommon.StreamEndReasonFirstResponseTimeout, info.StreamStatus.EndReason)
					return
				}
				<-time.NewTimer(2 * time.Second).C
				synctest.Wait()
				select {
				case apiErr := <-finished:
					test.Fatalf("non-heartbeat progress ended stream prematurely: %v", apiErr)
				default:
				}
				require.NoError(test, writer.Close())
				require.Nil(test, <-finished)
			})
		})
	}
}
