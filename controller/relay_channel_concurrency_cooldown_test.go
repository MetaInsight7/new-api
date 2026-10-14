package controller

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestRelayHTTP429CooldownChangesSubsequentDistributionAndExpires(t *testing.T) {
	gin.SetMode(gin.TestMode)
	upstreamCalls := atomic.Int32{}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		upstreamCalls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"message":"rate limit exceeded","type":"rate_limit_error","code":"rate_limit"}}`))
	}))
	t.Cleanup(upstream.Close)

	miniRedis := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: miniRedis.Addr()})
	previousDB := model.DB
	previousMemoryCacheEnabled := common.MemoryCacheEnabled
	previousDatabaseType := common.MainDatabaseType()
	previousRDB := common.RDB
	previousRedisEnabled := common.RedisEnabled
	previousErrorLogEnabled := constant.ErrorLogEnabled
	previousSetting := operation_setting.GetChannelConcurrencySetting()
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, database.AutoMigrate(&model.Channel{}, &model.Ability{}))
	model.DB = database
	common.MemoryCacheEnabled = true
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	common.RDB = redisClient
	common.RedisEnabled = true
	constant.ErrorLogEnabled = false
	setting := previousSetting
	setting.WaitEnabled = false
	setting.CooldownEnabled = true
	setting.CooldownSeconds = 1
	setting.CooldownOnStatus429 = true
	setting.CooldownOnMessageMatch = false
	operation_setting.SetChannelConcurrencySettingForTest(setting)
	t.Cleanup(func() {
		require.NoError(t, redisClient.Close())
		model.DB = previousDB
		common.MemoryCacheEnabled = previousMemoryCacheEnabled
		common.SetMainDatabaseType(previousDatabaseType)
		common.RDB = previousRDB
		common.RedisEnabled = previousRedisEnabled
		constant.ErrorLogEnabled = previousErrorLogEnabled
		operation_setting.SetChannelConcurrencySettingForTest(previousSetting)
		if previousMemoryCacheEnabled && previousDB != nil {
			model.InitChannelCache()
		}
	})

	priority := int64(0)
	preferredWeight := uint(1000)
	fallbackWeight := uint(1)
	preferred := &model.Channel{
		Id:             950001,
		Type:           constant.ChannelTypeOpenAI,
		Key:            "sk-preferred",
		Status:         common.ChannelStatusEnabled,
		Name:           "preferred",
		Group:          "default",
		Models:         "gpt-cooldown-relay",
		BaseURL:        &upstream.URL,
		Priority:       &priority,
		Weight:         &preferredWeight,
		MaxConcurrency: 1,
	}
	fallback := &model.Channel{
		Id:             950002,
		Type:           constant.ChannelTypeOpenAI,
		Key:            "sk-fallback",
		Status:         common.ChannelStatusEnabled,
		Name:           "fallback",
		Group:          "default",
		Models:         "gpt-cooldown-relay",
		Priority:       &priority,
		Weight:         &fallbackWeight,
		MaxConcurrency: 1,
	}
	require.NoError(t, model.DB.Create(preferred).Error)
	require.NoError(t, preferred.AddAbilities(nil))
	require.NoError(t, model.DB.Create(fallback).Error)
	require.NoError(t, fallback.AddAbilities(nil))
	model.InitChannelCache()

	selectedProbeChannel := atomic.Int32{}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		common.SetContextKey(c, constant.ContextKeyUsingGroup, "default")
		common.SetContextKey(c, constant.ContextKeyUserGroup, "default")
		if c.GetHeader("X-Test-Pin") == "preferred" {
			service.GetChannelConstraints(c).AddPin(dto.ChannelPin{
				ChannelId: preferred.Id,
				Source:    dto.PinSourceToken,
				Rank:      dto.PinRankToken,
				RetryMode: dto.PinRetrySingleAttempt,
			})
		}
		c.Next()
	})
	router.Use(middleware.Distribute())
	router.POST("/v1/chat/completions", func(c *gin.Context) {
		if c.GetHeader("X-Test-Probe") == "true" {
			selectedProbeChannel.Store(int32(common.GetContextKeyInt(c, constant.ContextKeyChannelId)))
			c.Status(http.StatusNoContent)
			return
		}

		request, requestErr := helper.GetAndValidateRequest(c, types.RelayFormatOpenAI)
		if requestErr != nil {
			c.String(http.StatusBadRequest, requestErr.Error())
			return
		}
		relayInfo, relayInfoErr := relaycommon.GenRelayInfo(c, types.RelayFormatOpenAI, request, nil)
		if relayInfoErr != nil {
			c.String(http.StatusInternalServerError, relayInfoErr.Error())
			return
		}
		apiErr := executeRelayAttemptWithConcurrency(c, types.RelayFormatOpenAI, relayInfo, nil)
		if apiErr == nil {
			c.Status(http.StatusOK)
			return
		}
		service.ProcessChannelError(c, types.ChannelError{ChannelId: preferred.Id}, apiErr, relayInfo)
		c.Status(apiErr.StatusCode)
	})

	requestBody := `{"model":"gpt-cooldown-relay","messages":[{"role":"user","content":"hello"}]}`
	failedRequest := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(requestBody))
	failedRequest.Header.Set("Content-Type", "application/json")
	failedRequest.Header.Set("X-Test-Pin", "preferred")
	failedRecorder := httptest.NewRecorder()
	router.ServeHTTP(failedRecorder, failedRequest)
	require.Equal(t, http.StatusTooManyRequests, failedRecorder.Code, failedRecorder.Body.String())
	require.Equal(t, int32(1), upstreamCalls.Load())
	var persistedPreferred model.Channel
	require.NoError(t, model.DB.First(&persistedPreferred, preferred.Id).Error)
	require.Equal(t, common.ChannelStatusEnabled, persistedPreferred.Status)

	probe := func(pinPreferred bool) *httptest.ResponseRecorder {
		selectedProbeChannel.Store(0)
		probeRequest := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(requestBody))
		probeRequest.Header.Set("Content-Type", "application/json")
		probeRequest.Header.Set("X-Test-Probe", "true")
		if pinPreferred {
			probeRequest.Header.Set("X-Test-Pin", "preferred")
		}
		probeRecorder := httptest.NewRecorder()
		router.ServeHTTP(probeRecorder, probeRequest)
		return probeRecorder
	}

	fallbackProbe := probe(false)
	require.Equal(t, http.StatusNoContent, fallbackProbe.Code, fallbackProbe.Body.String())
	require.Equal(t, int32(fallback.Id), selectedProbeChannel.Load())

	miniRedis.FastForward(2 * time.Second)
	recoveredProbe := probe(true)
	require.Equal(t, http.StatusNoContent, recoveredProbe.Code, recoveredProbe.Body.String())
	require.Equal(t, int32(preferred.Id), selectedProbeChannel.Load())

	loads, err := service.GetChannelConcurrencyLoads(context.Background(), []*model.Channel{preferred})
	require.NoError(t, err)
	require.False(t, loads[preferred.Id].CoolingDown)
}

func setupRelayRetryRouting(test *testing.T, extraModels ...any) *gorm.DB {
	test.Helper()
	gin.SetMode(gin.TestMode)
	models := append([]any{&model.Channel{}, &model.Ability{}, &model.ModelAvailabilityState{}}, extraModels...)
	database, dialect := openTaskDialectDatabase(test, models...)
	previousDB := model.DB
	previousDatabaseType := common.MainDatabaseType()
	previousMemoryCacheEnabled := common.MemoryCacheEnabled
	previousRedisEnabled := common.RedisEnabled
	previousRetryTimes := common.RetryTimes
	previousErrorLogEnabled := constant.ErrorLogEnabled
	previousSetting := operation_setting.GetChannelConcurrencySetting()
	model.DB = database
	common.SetMainDatabaseType(dialect)
	common.MemoryCacheEnabled = true
	common.RedisEnabled = false
	common.RetryTimes = 3
	constant.ErrorLogEnabled = false
	setting := previousSetting
	setting.WaitEnabled = false
	setting.CooldownEnabled = false
	operation_setting.SetChannelConcurrencySettingForTest(setting)
	test.Cleanup(func() {
		model.DB = previousDB
		common.SetMainDatabaseType(previousDatabaseType)
		common.MemoryCacheEnabled = previousMemoryCacheEnabled
		common.RedisEnabled = previousRedisEnabled
		common.RetryTimes = previousRetryTimes
		constant.ErrorLogEnabled = previousErrorLogEnabled
		operation_setting.SetChannelConcurrencySettingForTest(previousSetting)
		if previousMemoryCacheEnabled && previousDB != nil {
			model.InitChannelCache()
		}
	})
	return database
}

func TestRelayHTTPCandidateExhaustionPreservesLastFailure(test *testing.T) {
	for _, testCase := range []struct {
		name            string
		statuses        []int
		wantStatus      int
		wantCode        string
		wantType        string
		wantMessage     string
		databaseFailure bool
	}{
		{"upstream unavailable", []int{503}, 503, "provider_overloaded", "server_error", "上游服务异常，请稍后再试", false},
		{"upstream rate limit", []int{429}, 429, "rate_limit_exceeded", "rate_limit_error", "请求频率过高，请稍后再试", false},
		{"latest failure wins", []int{429, 503}, 503, "provider_overloaded", "server_error", "上游服务异常，请稍后再试", false},
		{"fallback succeeds", []int{503, 200}, 200, "", "", "", false},
		{"new database failure wins", []int{503}, 500, "get_channel_failed", "new_api_error", "请求处理失败，请稍后再试", true},
	} {
		test.Run(testCase.name, func(test *testing.T) {
			database := setupRelayRetryRouting(test, &model.User{})
			require.NoError(test, database.Create(&model.User{Id: 1, Username: "relay-retry-user", Group: "default"}).Error)
			previousModelRatios := ratio_setting.ModelRatio2JSONString()
			previousCountToken := constant.CountToken
			previousFreePreConsume := operation_setting.GetQuotaSetting().EnableFreeModelPreConsume
			previousLogConsumeEnabled := common.LogConsumeEnabled
			previousBatchUpdateEnabled := common.BatchUpdateEnabled
			require.NoError(test, ratio_setting.UpdateModelRatioByJSONString(`{"retry-preservation-model":0}`))
			constant.CountToken = false
			common.LogConsumeEnabled = false
			common.BatchUpdateEnabled = false
			operation_setting.GetQuotaSetting().EnableFreeModelPreConsume = false
			test.Cleanup(func() {
				require.NoError(test, ratio_setting.UpdateModelRatioByJSONString(previousModelRatios))
				constant.CountToken = previousCountToken
				common.LogConsumeEnabled = previousLogConsumeEnabled
				common.BatchUpdateEnabled = previousBatchUpdateEnabled
				operation_setting.GetQuotaSetting().EnableFreeModelPreConsume = previousFreePreConsume
			})
			withTieredBillingConfig(test, map[string]string{}, map[string]string{})

			upstreamCalls := atomic.Int32{}
			routingFailed := atomic.Bool{}
			upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				attempt := int(upstreamCalls.Add(1)) - 1
				if attempt >= len(testCase.statuses) {
					writer.WriteHeader(http.StatusInternalServerError)
					return
				}
				status := testCase.statuses[attempt]
				writer.Header().Set("Content-Type", "application/json")
				writer.WriteHeader(status)
				if testCase.databaseFailure {
					routingFailed.Store(true)
				}
				switch status {
				case http.StatusOK:
					_, _ = writer.Write([]byte(`{"id":"chatcmpl-success","model":"retry-preservation-model","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`))
				case http.StatusTooManyRequests:
					_, _ = writer.Write([]byte(`{"error":{"message":"private provider diagnostic sk-upstream-secret","type":"rate_limit_error","code":"rate_limit_exceeded"}}`))
				default:
					_, _ = writer.Write([]byte(`{"error":{"message":"private provider diagnostic sk-upstream-secret","type":"server_error","code":"provider_overloaded"}}`))
				}
			}))
			test.Cleanup(upstream.Close)
			var firstChannel *model.Channel
			for index := range len(testCase.statuses) {
				priority := int64(len(testCase.statuses) - index)
				channel := &model.Channel{
					Name:     fmt.Sprintf("private-channel-%d", index),
					Type:     constant.ChannelTypeOpenAI,
					Key:      "sk-upstream-secret",
					Status:   common.ChannelStatusEnabled,
					Group:    "default",
					Models:   "retry-preservation-model",
					BaseURL:  &upstream.URL,
					Priority: &priority,
				}
				require.NoError(test, database.Create(channel).Error)
				require.NoError(test, channel.AddAbilities(nil))
				if firstChannel == nil {
					firstChannel = channel
				}
			}
			model.InitChannelCache()
			if testCase.databaseFailure {
				common.MemoryCacheEnabled = false
				require.NoError(test, database.Callback().Query().Before("gorm:query").Register("test:routing-failure", func(query *gorm.DB) {
					if routingFailed.Load() {
						query.AddError(errors.New("routing database unavailable"))
					}
				}))
			}

			var policy *service.RequestPolicyState
			router := gin.New()
			router.POST("/v1/chat/completions", func(requestContext *gin.Context) {
				common.SetContextKey(requestContext, constant.ContextKeyUsingGroup, "default")
				common.SetContextKey(requestContext, constant.ContextKeyUserGroup, "default")
				common.SetContextKey(requestContext, constant.ContextKeyUserId, 1)
				common.SetContextKey(requestContext, constant.ContextKeyOriginalModel, "retry-preservation-model")
				require.Nil(test, middleware.SetupContextForSelectedChannel(requestContext, firstChannel, "retry-preservation-model"))
				Relay(requestContext, types.RelayFormatOpenAI)
				policy = service.RequestPolicy(requestContext)
			})
			request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"retry-preservation-model","messages":[{"role":"user","content":"hello"}]}`))
			request.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)

			assert.Equal(test, testCase.wantStatus, recorder.Code, recorder.Body.String())
			if testCase.wantStatus == http.StatusOK {
				assert.Contains(test, recorder.Body.String(), "chatcmpl-success")
				assert.NotContains(test, recorder.Body.String(), `"error":`)
			} else {
				var response struct {
					Error types.OpenAIError `json:"error"`
				}
				require.NoError(test, common.Unmarshal(recorder.Body.Bytes(), &response))
				assert.Equal(test, testCase.wantCode, fmt.Sprint(response.Error.Code))
				assert.Equal(test, testCase.wantType, response.Error.Type)
				assert.Contains(test, response.Error.Message, testCase.wantMessage)
			}
			assert.NotContains(test, recorder.Body.String(), "sk-upstream-secret")
			assert.NotContains(test, recorder.Body.String(), "private provider diagnostic")
			assert.NotContains(test, recorder.Body.String(), upstream.URL)
			assert.Equal(test, int32(len(testCase.statuses)), upstreamCalls.Load())
			require.NotNil(test, policy)
			assert.Equal(test, len(testCase.statuses), policy.Attempts)
			assert.Equal(test, testCase.wantStatus == http.StatusOK, policy.Successful)
		})
	}
}
