package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type channelConcurrencyStatusResponse struct {
	Success bool                       `json:"success"`
	Message string                     `json:"message"`
	Data    []channelConcurrencyStatus `json:"data"`
}

func requestChannelConcurrencyStatus(t *testing.T, ids string) (*httptest.ResponseRecorder, channelConcurrencyStatusResponse) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/channel/concurrency?ids="+url.QueryEscape(ids), nil)

	GetChannelConcurrencyStatus(context)

	var body channelConcurrencyStatusResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &body))
	return recorder, body
}

func TestChannelConcurrencyStatusEnforcesRawFragmentLimit(t *testing.T) {
	twoHundredMalformedFragments := strings.Join(make([]string, 200), ",")
	recorder, body := requestChannelConcurrencyStatus(t, twoHundredMalformedFragments)
	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.True(t, body.Success)
	assert.Empty(t, body.Data)

	twoHundredOneMalformedFragments := twoHundredMalformedFragments + ","
	recorder, body = requestChannelConcurrencyStatus(t, twoHundredOneMalformedFragments)
	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.False(t, body.Success)
}

func TestChannelConcurrencyStatusReturnsOnlyExistingLimitedChannels(t *testing.T) {
	previousDB := model.DB
	previousRedisEnabled := common.RedisEnabled
	previousSetting := operation_setting.GetChannelConcurrencySetting()
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, database.AutoMigrate(&model.Channel{}))
	model.DB = database
	common.RedisEnabled = false
	setting := previousSetting
	setting.CooldownEnabled = true
	operation_setting.SetChannelConcurrencySettingForTest(setting)
	t.Cleanup(func() {
		model.DB = previousDB
		common.RedisEnabled = previousRedisEnabled
		operation_setting.SetChannelConcurrencySettingForTest(previousSetting)
	})

	unlimited := &model.Channel{Id: 7500, Name: "unlimited", MaxConcurrency: 0}
	limited := &model.Channel{Id: 7501, Name: "limited", MaxConcurrency: 3}
	require.NoError(t, database.Create(unlimited).Error)
	require.NoError(t, database.Create(limited).Error)
	lease, acquired, err := service.TryAcquireChannelConcurrency(context.Background(), limited)
	require.NoError(t, err)
	require.True(t, acquired)
	t.Cleanup(func() {
		require.NoError(t, service.ReleaseChannelConcurrency(context.Background(), lease))
	})
	require.NoError(t, service.MarkChannelConcurrencyCooldown(context.Background(), limited.Id, time.Minute, "test"))

	recorder, body := requestChannelConcurrencyStatus(t, " ,nope,0,-2,7500,7501,7501,9999")

	require.Equal(t, http.StatusOK, recorder.Code)
	require.True(t, body.Success)
	require.Len(t, body.Data, 1)
	assert.Equal(t, channelConcurrencyStatus{
		ChannelID:      limited.Id,
		MaxConcurrency: 3,
		Active:         1,
		Waiting:        0,
		CoolingDown:    true,
	}, body.Data[0])
}

func TestChannelConcurrencyStatusSharesBoundedLoadSnapshotWithoutAcquiringSlots(t *testing.T) {
	previousDB := model.DB
	previousRDB := common.RDB
	previousRedisEnabled := common.RedisEnabled
	previousSetting := operation_setting.GetChannelConcurrencySetting()
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, database.AutoMigrate(&model.Channel{}))
	miniRedis := miniredis.RunT(t)
	commandHook := &channelConcurrencyRedisCommandHook{}
	redisClient := redis.NewClient(&redis.Options{Addr: miniRedis.Addr()})
	redisClient.AddHook(commandHook)
	model.DB = database
	common.RDB = redisClient
	common.RedisEnabled = true
	setting := previousSetting
	setting.LoadCacheEnabled = true
	setting.LoadCacheTTLMS = 5000
	operation_setting.SetChannelConcurrencySettingForTest(setting)
	t.Cleanup(func() {
		model.DB = previousDB
		common.RDB = previousRDB
		common.RedisEnabled = previousRedisEnabled
		operation_setting.SetChannelConcurrencySettingForTest(previousSetting)
		require.NoError(t, redisClient.Close())
	})

	channel := &model.Channel{Id: 7601, Name: "shared snapshot", MaxConcurrency: 4}
	require.NoError(t, database.Create(channel).Error)
	warmed, err := service.GetChannelConcurrencyLoads(context.Background(), []*model.Channel{channel})
	require.NoError(t, err)
	require.Contains(t, warmed, channel.Id)
	commandsAfterWarm := commandHook.Commands()
	require.NotEmpty(t, commandsAfterWarm)

	recorder, body := requestChannelConcurrencyStatus(t, "7601")
	require.Equal(t, http.StatusOK, recorder.Code)
	require.True(t, body.Success)
	require.Equal(t, []channelConcurrencyStatus{{
		ChannelID:      channel.Id,
		MaxConcurrency: channel.MaxConcurrency,
	}}, body.Data)
	commandsAfterStatus := commandHook.Commands()
	assert.Equal(t, commandsAfterWarm, commandsAfterStatus, "the management endpoint must reuse the shared warm snapshot")
	for _, command := range commandsAfterStatus {
		assert.NotEqual(t, "eval", command)
		assert.NotEqual(t, "evalsha", command)
		assert.NotEqual(t, "zadd", command)
	}
}

type channelConcurrencyRedisCommandHook struct {
	mu       sync.Mutex
	commands []string
}

func (hook *channelConcurrencyRedisCommandHook) BeforeProcess(ctx context.Context, command redis.Cmder) (context.Context, error) {
	hook.record(command.Name())
	return ctx, nil
}

func (hook *channelConcurrencyRedisCommandHook) AfterProcess(context.Context, redis.Cmder) error {
	return nil
}

func (hook *channelConcurrencyRedisCommandHook) BeforeProcessPipeline(ctx context.Context, commands []redis.Cmder) (context.Context, error) {
	for _, command := range commands {
		hook.record(command.Name())
	}
	return ctx, nil
}

func (hook *channelConcurrencyRedisCommandHook) AfterProcessPipeline(context.Context, []redis.Cmder) error {
	return nil
}

func (hook *channelConcurrencyRedisCommandHook) record(command string) {
	hook.mu.Lock()
	defer hook.mu.Unlock()
	hook.commands = append(hook.commands, strings.ToLower(command))
}

func (hook *channelConcurrencyRedisCommandHook) Commands() []string {
	hook.mu.Lock()
	defer hook.mu.Unlock()
	return append([]string(nil), hook.commands...)
}
