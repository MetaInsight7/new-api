package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/require"
)

func TestProcessChannelErrorDoesNotCoolDownFuzzyMatchByDefault(t *testing.T) {
	resetChannelConcurrencyForTest()
	previousRDB := common.RDB
	previousRedisEnabled := common.RedisEnabled
	previousErrorLogEnabled := constant.ErrorLogEnabled
	previousSetting := operation_setting.GetChannelConcurrencySetting()
	common.RDB = nil
	common.RedisEnabled = false
	constant.ErrorLogEnabled = false
	setting := previousSetting
	setting.CooldownEnabled = true
	setting.CooldownOnStatus429 = true
	setting.CooldownOnMessageMatch = false
	operation_setting.SetChannelConcurrencySettingForTest(setting)
	t.Cleanup(func() {
		common.RDB = previousRDB
		common.RedisEnabled = previousRedisEnabled
		constant.ErrorLogEnabled = previousErrorLogEnabled
		operation_setting.SetChannelConcurrencySettingForTest(previousSetting)
		resetChannelConcurrencyForTest()
	})

	channel := &model.Channel{Id: 1301, MaxConcurrency: 1}
	gin.SetMode(gin.TestMode)
	requestContext, _ := gin.CreateTestContext(httptest.NewRecorder())
	ProcessChannelError(
		requestContext,
		types.ChannelError{ChannelId: channel.Id},
		types.NewOpenAIError(errors.New("upstream overloaded, retry later"), types.ErrorCodeBadResponseStatusCode, http.StatusServiceUnavailable),
		nil,
	)

	lease, acquired, err := TryAcquireChannelConcurrency(context.Background(), channel)
	require.NoError(t, err)
	require.True(t, acquired)
	require.NotNil(t, lease)
	require.NoError(t, ReleaseChannelConcurrency(context.Background(), lease))
}

func TestProcessChannelErrorExcludesQuotaFailuresBeforeCooldownTriggers(t *testing.T) {
	testCases := []struct {
		name       string
		message    string
		statusCode int
	}{
		{
			name:       "english quota exhaustion before HTTP 429",
			message:    "quota exhausted after rate limit",
			statusCode: http.StatusTooManyRequests,
		},
		{
			name:       "chinese quota exhaustion before fuzzy capacity match",
			message:    "配额耗尽：upstream capacity unavailable",
			statusCode: http.StatusServiceUnavailable,
		},
		{
			name:       "english insufficient balance before HTTP 429",
			message:    "insufficient balance while rate limited",
			statusCode: http.StatusTooManyRequests,
		},
		{
			name:       "chinese insufficient credit before HTTP 429",
			message:    "额度不足：请求过多",
			statusCode: http.StatusTooManyRequests,
		},
	}

	for index, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			resetChannelConcurrencyForTest()
			previousRDB := common.RDB
			previousRedisEnabled := common.RedisEnabled
			previousErrorLogEnabled := constant.ErrorLogEnabled
			previousSetting := operation_setting.GetChannelConcurrencySetting()
			common.RDB = nil
			common.RedisEnabled = false
			constant.ErrorLogEnabled = false
			setting := previousSetting
			setting.CooldownEnabled = true
			setting.CooldownOnStatus429 = true
			setting.CooldownOnMessageMatch = true
			operation_setting.SetChannelConcurrencySettingForTest(setting)
			t.Cleanup(func() {
				common.RDB = previousRDB
				common.RedisEnabled = previousRedisEnabled
				constant.ErrorLogEnabled = previousErrorLogEnabled
				operation_setting.SetChannelConcurrencySettingForTest(previousSetting)
				resetChannelConcurrencyForTest()
			})

			channel := &model.Channel{Id: 1310 + index, MaxConcurrency: 1}
			gin.SetMode(gin.TestMode)
			requestContext, _ := gin.CreateTestContext(httptest.NewRecorder())
			ProcessChannelError(
				requestContext,
				types.ChannelError{ChannelId: channel.Id},
				types.NewOpenAIError(errors.New(testCase.message), types.ErrorCodeBadResponseStatusCode, testCase.statusCode),
				nil,
			)

			lease, acquired, err := TryAcquireChannelConcurrency(context.Background(), channel)
			require.NoError(t, err)
			require.True(t, acquired)
			require.NotNil(t, lease)
			require.NoError(t, ReleaseChannelConcurrency(context.Background(), lease))
		})
	}
}

func TestProcessChannelErrorAppliesHotUpdatedCooldownTriggers(t *testing.T) {
	resetChannelConcurrencyForTest()
	previousRDB := common.RDB
	previousRedisEnabled := common.RedisEnabled
	previousErrorLogEnabled := constant.ErrorLogEnabled
	previousSetting := operation_setting.GetChannelConcurrencySetting()
	common.RDB = nil
	common.RedisEnabled = false
	constant.ErrorLogEnabled = false
	t.Cleanup(func() {
		common.RDB = previousRDB
		common.RedisEnabled = previousRedisEnabled
		constant.ErrorLogEnabled = previousErrorLogEnabled
		operation_setting.SetChannelConcurrencySettingForTest(previousSetting)
		resetChannelConcurrencyForTest()
	})

	configValue := config.GlobalConfig.Get("channel_concurrency_setting")
	gin.SetMode(gin.TestMode)
	requestContext, _ := gin.CreateTestContext(httptest.NewRecorder())
	process := func(channelID int, message string, statusCode int) {
		ProcessChannelError(
			requestContext,
			types.ChannelError{ChannelId: channelID},
			types.NewOpenAIError(errors.New(message), types.ErrorCodeBadResponseStatusCode, statusCode),
			nil,
		)
	}

	require.NoError(t, config.UpdateConfigFromMap(configValue, map[string]string{
		"cooldown_enabled":          "true",
		"cooldown_seconds":          "30",
		"cooldown_on_status_429":    "true",
		"cooldown_on_message_match": "false",
	}))
	process(1331, "upstream rejected request", http.StatusTooManyRequests)
	process(1332, "upstream overloaded", http.StatusServiceUnavailable)

	require.NoError(t, config.UpdateConfigFromMap(configValue, map[string]string{
		"cooldown_on_status_429":    "false",
		"cooldown_on_message_match": "true",
	}))
	process(1333, "rate limit exceeded", http.StatusTooManyRequests)
	process(1334, "upstream capacity unavailable", http.StatusServiceUnavailable)

	require.NoError(t, config.UpdateConfigFromMap(configValue, map[string]string{
		"cooldown_enabled":       "false",
		"cooldown_on_status_429": "true",
	}))
	process(1335, "rate limit exceeded", http.StatusTooManyRequests)

	channels := []*model.Channel{
		{Id: 1331, MaxConcurrency: 1},
		{Id: 1332, MaxConcurrency: 1},
		{Id: 1333, MaxConcurrency: 1},
		{Id: 1334, MaxConcurrency: 1},
		{Id: 1335, MaxConcurrency: 1},
	}
	loads, err := GetChannelConcurrencyLoads(context.Background(), channels)
	require.NoError(t, err)
	require.True(t, loads[1331].CoolingDown)
	require.False(t, loads[1332].CoolingDown)
	require.False(t, loads[1333].CoolingDown)
	require.True(t, loads[1334].CoolingDown)
	require.False(t, loads[1335].CoolingDown)
}

func TestRedisAuthoritativeAcquireRejectsCooldownStartedAfterObservation(t *testing.T) {
	resetChannelConcurrencyForTest()
	miniRedis := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: miniRedis.Addr()})
	cooldownWriter := redis.NewClient(&redis.Options{Addr: miniRedis.Addr()})
	previousRDB := common.RDB
	previousRedisEnabled := common.RedisEnabled
	previousSetting := operation_setting.GetChannelConcurrencySetting()
	common.RDB = redisClient
	common.RedisEnabled = true
	setting := previousSetting
	setting.CooldownEnabled = true
	operation_setting.SetChannelConcurrencySettingForTest(setting)
	t.Cleanup(func() {
		require.NoError(t, redisClient.Close())
		require.NoError(t, cooldownWriter.Close())
		common.RDB = previousRDB
		common.RedisEnabled = previousRedisEnabled
		operation_setting.SetChannelConcurrencySettingForTest(previousSetting)
		resetChannelConcurrencyForTest()
	})

	channel := &model.Channel{Id: 1320, MaxConcurrency: 1}
	loads, err := GetChannelConcurrencyLoads(context.Background(), []*model.Channel{channel})
	require.NoError(t, err)
	require.False(t, loads[channel.Id].CoolingDown)

	hook := &cooldownBeforeAcquireScriptHook{
		channelKey:  channelConcurrencyRedisKey(channel.Id),
		cooldownKey: channelConcurrencyCooldownRedisKey(channel.Id),
		writer:      cooldownWriter,
	}
	redisClient.AddHook(hook)
	lease, acquired, err := TryAcquireChannelConcurrency(context.Background(), channel)
	require.NoError(t, err)
	require.False(t, acquired)
	require.Nil(t, lease)
	require.Zero(t, hook.standaloneCooldownChecks.Load())
	active, err := redisClient.ZCard(context.Background(), channelConcurrencyRedisKey(channel.Id)).Result()
	require.NoError(t, err)
	require.Zero(t, active)
}

type cooldownBeforeAcquireScriptHook struct {
	channelKey               string
	cooldownKey              string
	writer                   *redis.Client
	once                     sync.Once
	standaloneCooldownChecks atomic.Int32
}

func (h *cooldownBeforeAcquireScriptHook) BeforeProcess(ctx context.Context, cmd redis.Cmder) (context.Context, error) {
	if strings.EqualFold(cmd.Name(), "exists") && redisCommandContainsKey(cmd, h.cooldownKey) {
		h.standaloneCooldownChecks.Add(1)
	}
	if (strings.EqualFold(cmd.Name(), "eval") || strings.EqualFold(cmd.Name(), "evalsha")) && redisCommandContainsKey(cmd, h.channelKey) {
		h.once.Do(func() {
			_ = h.writer.Set(context.Background(), h.cooldownKey, "late cooldown", time.Minute).Err()
		})
	}
	return ctx, nil
}

func (h *cooldownBeforeAcquireScriptHook) AfterProcess(context.Context, redis.Cmder) error {
	return nil
}

func (h *cooldownBeforeAcquireScriptHook) BeforeProcessPipeline(ctx context.Context, _ []redis.Cmder) (context.Context, error) {
	return ctx, nil
}

func (h *cooldownBeforeAcquireScriptHook) AfterProcessPipeline(context.Context, []redis.Cmder) error {
	return nil
}
