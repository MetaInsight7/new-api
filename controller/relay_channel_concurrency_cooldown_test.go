package controller

import (
	"context"
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
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/go-redis/redis/v8"
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
