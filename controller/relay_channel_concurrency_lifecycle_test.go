package controller

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relaytypes "github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestRelayRetryRebindsAndReleasesChannelConcurrency(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previousDB := model.DB
	previousMemoryCacheEnabled := common.MemoryCacheEnabled
	previousRDB := common.RDB
	previousRedisEnabled := common.RedisEnabled
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, database.AutoMigrate(&model.Channel{}))
	model.DB = database
	common.MemoryCacheEnabled = false
	common.RDB = nil
	common.RedisEnabled = false
	t.Cleanup(func() {
		model.DB = previousDB
		common.MemoryCacheEnabled = previousMemoryCacheEnabled
		common.RDB = previousRDB
		common.RedisEnabled = previousRedisEnabled
	})

	firstChannel := &model.Channel{
		Id:             940001,
		Type:           constant.ChannelTypeOpenAI,
		Key:            "sk-first",
		Status:         common.ChannelStatusEnabled,
		Name:           "first-attempt",
		Group:          "default",
		Models:         "gpt-retry-lifecycle",
		MaxConcurrency: 1,
	}
	retryChannel := &model.Channel{
		Id:             940002,
		Type:           constant.ChannelTypeOpenAI,
		Key:            "sk-retry",
		Status:         common.ChannelStatusEnabled,
		Name:           "retry-attempt",
		Group:          "default",
		Models:         "gpt-retry-lifecycle",
		MaxConcurrency: 1,
	}
	require.NoError(t, model.DB.Create(firstChannel).Error)
	require.NoError(t, model.DB.Create(retryChannel).Error)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	c, _ := gin.CreateTestContext(recorder)
	c.Request = request

	acquired, err := service.AcquireChannelConcurrencyForContext(c, firstChannel)
	require.NoError(t, err)
	require.True(t, acquired)
	t.Cleanup(func() {
		require.NoError(t, service.ReleaseChannelConcurrencyForContext(c))
	})

	common.SetContextKey(c, constant.ContextKeyChannelId, retryChannel.Id)
	bodyErr := errors.New("request body became unavailable")
	body := &observingErrorReadCloser{
		err: bodyErr,
		observe: func() {
			loads, loadErr := service.GetChannelConcurrencyLoads(context.Background(), []*model.Channel{firstChannel, retryChannel})
			require.NoError(t, loadErr)
			require.Equal(t, 0, loads[firstChannel.Id].Active)
			require.Equal(t, 1, loads[retryChannel.Id].Active)
		},
	}
	c.Request.Body = body

	relayErr := executeRelayAttemptWithConcurrency(c, relaytypes.RelayFormatOpenAI, &relaycommon.RelayInfo{}, nil)
	require.NotNil(t, relayErr)
	require.ErrorContains(t, relayErr, bodyErr.Error())
	require.Equal(t, http.StatusBadRequest, relayErr.StatusCode)
	require.True(t, body.Observed())

	loads, err := service.GetChannelConcurrencyLoads(context.Background(), []*model.Channel{firstChannel, retryChannel})
	require.NoError(t, err)
	require.Equal(t, 0, loads[firstChannel.Id].Active)
	require.Equal(t, 0, loads[retryChannel.Id].Active)
}

func TestGetChannelMapsConcurrencyExhaustionToTooManyRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previousDB := model.DB
	previousMemoryCacheEnabled := common.MemoryCacheEnabled
	previousRDB := common.RDB
	previousRedisEnabled := common.RedisEnabled
	previousMainDatabaseType := common.MainDatabaseType()
	previousConcurrencySetting := operation_setting.GetChannelConcurrencySetting()
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, database.AutoMigrate(&model.Channel{}, &model.Ability{}))
	model.DB = database
	common.MemoryCacheEnabled = true
	common.RDB = nil
	common.RedisEnabled = false
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	concurrencySetting := previousConcurrencySetting
	concurrencySetting.WaitEnabled = false
	operation_setting.SetChannelConcurrencySettingForTest(concurrencySetting)
	t.Cleanup(func() {
		model.DB = previousDB
		common.MemoryCacheEnabled = previousMemoryCacheEnabled
		common.RDB = previousRDB
		common.RedisEnabled = previousRedisEnabled
		common.SetMainDatabaseType(previousMainDatabaseType)
		operation_setting.SetChannelConcurrencySettingForTest(previousConcurrencySetting)
		if previousMemoryCacheEnabled && previousDB != nil {
			model.InitChannelCache()
		}
	})

	channel := &model.Channel{
		Id:             940003,
		Type:           constant.ChannelTypeOpenAI,
		Key:            "sk-full-retry",
		Status:         common.ChannelStatusEnabled,
		Name:           "full-retry",
		Group:          "default",
		Models:         "gpt-retry-full",
		MaxConcurrency: 1,
	}
	require.NoError(t, model.DB.Create(channel).Error)
	require.NoError(t, channel.AddAbilities(nil))
	model.InitChannelCache()

	heldLease, acquired, err := service.TryAcquireChannelConcurrency(context.Background(), channel)
	require.NoError(t, err)
	require.True(t, acquired)
	t.Cleanup(func() {
		require.NoError(t, service.ReleaseChannelConcurrency(context.Background(), heldLease))
	})

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	relayInfo := &relaycommon.RelayInfo{
		TokenGroup:      "default",
		OriginModelName: "gpt-retry-full",
		ChannelMeta:     &relaycommon.ChannelMeta{},
	}
	retryParam := &service.RetryParam{
		Ctx:        c,
		TokenGroup: "default",
		ModelName:  "gpt-retry-full",
		Retry:      common.GetPointer(0),
	}

	selected, apiErr := getChannel(c, relayInfo, retryParam)
	require.Nil(t, selected)
	require.NotNil(t, apiErr)
	require.Equal(t, http.StatusTooManyRequests, apiErr.StatusCode)
	require.Equal(t, relaytypes.ErrorCodeGetChannelFailed, apiErr.GetErrorCode())
	require.True(t, relaytypes.IsSkipRetryError(apiErr))
}

type observingErrorReadCloser struct {
	err     error
	observe func()
	once    sync.Once
	seen    atomic.Bool
}

func (r *observingErrorReadCloser) Read([]byte) (int, error) {
	r.once.Do(func() {
		r.seen.Store(true)
		r.observe()
	})
	return 0, r.err
}

func (r *observingErrorReadCloser) Close() error {
	return nil
}

func (r *observingErrorReadCloser) Observed() bool {
	return r.seen.Load()
}
