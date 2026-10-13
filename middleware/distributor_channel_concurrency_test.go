package middleware

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	appI18n "github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestDistributeReleasesOrdinaryChannelConcurrencyOnEveryExit(t *testing.T) {
	useMiddlewareMemoryChannelConcurrency(t)
	useMiddlewareChannelSelectionDB(t)

	channel := &model.Channel{
		Id:             930001,
		Type:           constant.ChannelTypeOpenAI,
		Key:            "sk-ordinary",
		Status:         common.ChannelStatusEnabled,
		Name:           "ordinary",
		Group:          "default",
		Models:         "gpt-ordinary-lifecycle",
		MaxConcurrency: 1,
	}
	require.NoError(t, model.DB.Create(channel).Error)
	require.NoError(t, channel.AddAbilities(nil))
	model.InitChannelCache()

	testCases := []struct {
		name       string
		statusCode int
		context    func() (context.Context, context.CancelFunc)
	}{
		{
			name:       "normal completion",
			statusCode: http.StatusOK,
			context: func() (context.Context, context.CancelFunc) {
				return context.WithCancel(context.Background())
			},
		},
		{
			name:       "upstream failure",
			statusCode: http.StatusBadGateway,
			context: func() (context.Context, context.CancelFunc) {
				return context.WithCancel(context.Background())
			},
		},
		{
			name:       "request cancellation",
			statusCode: 499,
			context: func() (context.Context, context.CancelFunc) {
				requestContext, cancel := context.WithCancel(context.Background())
				cancel()
				return requestContext, func() {}
			},
		},
		{
			name:       "request timeout",
			statusCode: http.StatusGatewayTimeout,
			context: func() (context.Context, context.CancelFunc) {
				return context.WithTimeout(context.Background(), 0)
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			router := gin.New()
			router.Use(func(c *gin.Context) {
				common.SetContextKey(c, constant.ContextKeyUsingGroup, "default")
				common.SetContextKey(c, constant.ContextKeyUserGroup, "default")
				c.Next()
			})
			router.Use(Distribute())
			router.POST("/v1/chat/completions", func(c *gin.Context) {
				defer common.CleanupBodyStorage(c)
				require.Equal(t, channel.Id, common.GetContextKeyInt(c, constant.ContextKeyChannelId))
				loads, loadErr := service.GetChannelConcurrencyLoads(c.Request.Context(), []*model.Channel{channel})
				require.NoError(t, loadErr)
				require.Equal(t, 1, loads[channel.Id].Active)
				c.Status(testCase.statusCode)
			})

			requestContext, cancel := testCase.context()
			defer cancel()
			request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-ordinary-lifecycle"}`)).WithContext(requestContext)
			request.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)
			require.Equal(t, testCase.statusCode, recorder.Code)

			loads, err := service.GetChannelConcurrencyLoads(context.Background(), []*model.Channel{channel})
			require.NoError(t, err)
			require.Equal(t, 0, loads[channel.Id].Active)
		})
	}
}

func TestDistributeSpecificChannelWaitsForConcurrencyRelease(t *testing.T) {
	useMiddlewareMemoryChannelConcurrency(t)
	useMiddlewareChannelConcurrencyWaitSetting(t, 500*time.Millisecond, 5*time.Millisecond, 1)
	useMiddlewareChannelSelectionDB(t)

	channel := &model.Channel{
		Id:             930002,
		Type:           constant.ChannelTypeOpenAI,
		Key:            "sk-specific",
		Status:         common.ChannelStatusEnabled,
		Name:           "specific",
		Group:          "default",
		Models:         "gpt-specific-wait",
		MaxConcurrency: 1,
	}
	require.NoError(t, model.DB.Create(channel).Error)
	model.InitChannelCache()

	heldLease, acquired, err := service.TryAcquireChannelConcurrency(context.Background(), channel)
	require.NoError(t, err)
	require.True(t, acquired)
	t.Cleanup(func() {
		require.NoError(t, service.ReleaseChannelConcurrency(context.Background(), heldLease))
	})

	router := gin.New()
	router.Use(func(c *gin.Context) {
		service.GetChannelConstraints(c).AddPin(dto.ChannelPin{
			ChannelId: channel.Id,
			Source:    dto.PinSourceToken,
			Rank:      dto.PinRankToken,
			RetryMode: dto.PinRetrySingleAttempt,
		})
		c.Next()
	})
	router.Use(Distribute())
	router.POST("/v1/chat/completions", func(c *gin.Context) {
		defer common.CleanupBodyStorage(c)
		c.Status(http.StatusOK)
	})

	done := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-specific-wait"}`))
		request.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(recorder, request)
		done <- recorder
	}()

	select {
	case recorder := <-done:
		t.Fatalf("request completed before held lease was released with status %d", recorder.Code)
	case <-time.After(30 * time.Millisecond):
	}
	require.NoError(t, service.ReleaseChannelConcurrency(context.Background(), heldLease))

	select {
	case recorder := <-done:
		require.Equal(t, http.StatusOK, recorder.Code)
	case <-time.After(time.Second):
		t.Fatal("request did not acquire channel concurrency after release")
	}
}

func TestDistributeAffinityChannelWaitsForConcurrencyRelease(t *testing.T) {
	useMiddlewareMemoryChannelConcurrency(t)
	useMiddlewareChannelConcurrencyWaitSetting(t, 500*time.Millisecond, 5*time.Millisecond, 1)
	useMiddlewareChannelSelectionDB(t)
	useMiddlewareChannelAffinityRule(t)

	priority := int64(0)
	preferredWeight := uint(1_000_000)
	fallbackWeight := uint(1)
	preferred := &model.Channel{
		Id:             930003,
		Type:           constant.ChannelTypeOpenAI,
		Key:            "sk-preferred",
		Status:         common.ChannelStatusEnabled,
		Name:           "preferred",
		Group:          "default",
		Models:         "gpt-affinity-wait",
		Priority:       &priority,
		Weight:         &preferredWeight,
		MaxConcurrency: 1,
	}
	fallback := &model.Channel{
		Id:             930004,
		Type:           constant.ChannelTypeOpenAI,
		Key:            "sk-fallback",
		Status:         common.ChannelStatusEnabled,
		Name:           "fallback",
		Group:          "default",
		Models:         "gpt-affinity-wait",
		Priority:       &priority,
		Weight:         &fallbackWeight,
		MaxConcurrency: 1,
	}
	require.NoError(t, model.DB.Create(preferred).Error)
	require.NoError(t, preferred.AddAbilities(nil))
	require.NoError(t, model.DB.Create(fallback).Error)
	require.NoError(t, fallback.AddAbilities(nil))
	model.InitChannelCache()

	affinityValue := fmt.Sprintf("affinity-%d", time.Now().UnixNano())
	seedContext, _ := gin.CreateTestContext(httptest.NewRecorder())
	seedContext.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-affinity-wait"}`))
	seedContext.Request.Header.Set("Content-Type", "application/json")
	seedContext.Request.Header.Set("X-Test-Affinity", affinityValue)
	_, found := service.GetPreferredChannelByAffinity(seedContext, "gpt-affinity-wait", "default")
	require.False(t, found)
	service.RecordChannelAffinity(seedContext, preferred.Id)

	heldLease, acquired, err := service.TryAcquireChannelConcurrency(context.Background(), preferred)
	require.NoError(t, err)
	require.True(t, acquired)
	t.Cleanup(func() {
		require.NoError(t, service.ReleaseChannelConcurrency(context.Background(), heldLease))
	})

	router := gin.New()
	router.Use(func(c *gin.Context) {
		common.SetContextKey(c, constant.ContextKeyUsingGroup, "default")
		common.SetContextKey(c, constant.ContextKeyUserGroup, "default")
		c.Next()
	})
	router.Use(Distribute())
	router.POST("/v1/chat/completions", func(c *gin.Context) {
		defer common.CleanupBodyStorage(c)
		if channelID := common.GetContextKeyInt(c, constant.ContextKeyChannelId); channelID != preferred.Id {
			c.String(http.StatusInternalServerError, "selected channel = %d, want %d", channelID, preferred.Id)
			return
		}
		c.Status(http.StatusOK)
	})

	done := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-affinity-wait"}`))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("X-Test-Affinity", affinityValue)
		router.ServeHTTP(recorder, request)
		done <- recorder
	}()

	select {
	case recorder := <-done:
		t.Fatalf("request completed before affinity lease was released with status %d", recorder.Code)
	case <-time.After(30 * time.Millisecond):
	}
	require.NoError(t, service.ReleaseChannelConcurrency(context.Background(), heldLease))

	select {
	case recorder := <-done:
		require.Equal(t, http.StatusOK, recorder.Code)
	case <-time.After(time.Second):
		t.Fatal("affinity request did not acquire channel concurrency after release")
	}
}

func TestDistributeAffinityQueueFullHonorsSessionMode(t *testing.T) {
	require.NoError(t, appI18n.Init())
	testCases := []struct {
		name              string
		sessionMode       string
		expectedStatus    int
		expectedChannelID int
	}{
		{
			name:              "prefer falls back",
			sessionMode:       "prefer",
			expectedStatus:    http.StatusOK,
			expectedChannelID: 930006,
		},
		{
			name:              "strict stops",
			sessionMode:       "strict",
			expectedStatus:    http.StatusTooManyRequests,
			expectedChannelID: 0,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			useMiddlewareMemoryChannelConcurrency(t)
			useMiddlewareChannelConcurrencyWaitSetting(t, 2*time.Second, 5*time.Millisecond, 1)
			useMiddlewareChannelSelectionDB(t)
			useMiddlewareChannelAffinityRule(t)
			affinitySetting := operation_setting.GetChannelAffinitySetting()
			affinitySetting.SessionMode = testCase.sessionMode
			affinitySetting.Rules[0].SessionMode = "inherit"

			priority := int64(0)
			preferredWeight := uint(1_000_000)
			fallbackWeight := uint(1)
			preferred := &model.Channel{
				Id:             930005,
				Type:           constant.ChannelTypeOpenAI,
				Key:            "sk-preferred-full",
				Status:         common.ChannelStatusEnabled,
				Name:           "preferred-full",
				Group:          "default",
				Models:         "gpt-affinity-wait",
				Priority:       &priority,
				Weight:         &preferredWeight,
				MaxConcurrency: 1,
			}
			fallback := &model.Channel{
				Id:             930006,
				Type:           constant.ChannelTypeOpenAI,
				Key:            "sk-fallback-free",
				Status:         common.ChannelStatusEnabled,
				Name:           "fallback-free",
				Group:          "default",
				Models:         "gpt-affinity-wait",
				Priority:       &priority,
				Weight:         &fallbackWeight,
				MaxConcurrency: 1,
			}
			for _, channel := range []*model.Channel{preferred, fallback} {
				require.NoError(t, model.DB.Create(channel).Error)
				require.NoError(t, channel.AddAbilities(nil))
			}
			model.InitChannelCache()

			affinityValue := fmt.Sprintf("affinity-full-%d", time.Now().UnixNano())
			seedContext, _ := gin.CreateTestContext(httptest.NewRecorder())
			seedContext.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-affinity-wait"}`))
			seedContext.Request.Header.Set("Content-Type", "application/json")
			seedContext.Request.Header.Set("X-Test-Affinity", affinityValue)
			_, found := service.GetPreferredChannelByAffinity(seedContext, "gpt-affinity-wait", "default")
			require.False(t, found)
			service.RecordChannelAffinity(seedContext, preferred.Id)

			occupyMiddlewareChannelConcurrencyQueue(t, preferred)

			selectedChannelID := 0
			router := gin.New()
			router.Use(func(c *gin.Context) {
				common.SetContextKey(c, constant.ContextKeyUsingGroup, "default")
				common.SetContextKey(c, constant.ContextKeyUserGroup, "default")
				c.Next()
			})
			router.Use(Distribute())
			router.POST("/v1/chat/completions", func(c *gin.Context) {
				defer common.CleanupBodyStorage(c)
				selectedChannelID = common.GetContextKeyInt(c, constant.ContextKeyChannelId)
				c.Status(http.StatusOK)
			})

			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-affinity-wait"}`))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-Test-Affinity", affinityValue)
			router.ServeHTTP(recorder, request)

			require.Equal(t, testCase.expectedStatus, recorder.Code, recorder.Body.String())
			require.Equal(t, testCase.expectedChannelID, selectedChannelID)
		})
	}
}

func TestDistributeConcurrencyQueueFullReturnsTooManyRequests(t *testing.T) {
	require.NoError(t, appI18n.Init())
	useMiddlewareMemoryChannelConcurrency(t)
	useMiddlewareChannelConcurrencyWaitSetting(t, 2*time.Second, 5*time.Millisecond, 1)
	useMiddlewareChannelSelectionDB(t)

	channel := &model.Channel{
		Id:             930007,
		Type:           constant.ChannelTypeOpenAI,
		Key:            "sk-full",
		Status:         common.ChannelStatusEnabled,
		Name:           "full",
		Group:          "default",
		Models:         "gpt-concurrency-full",
		MaxConcurrency: 1,
	}
	require.NoError(t, model.DB.Create(channel).Error)
	require.NoError(t, channel.AddAbilities(nil))
	model.InitChannelCache()
	occupyMiddlewareChannelConcurrencyQueue(t, channel)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		common.SetContextKey(c, constant.ContextKeyUsingGroup, "default")
		common.SetContextKey(c, constant.ContextKeyUserGroup, "default")
		c.Next()
	})
	router.Use(Distribute())
	router.POST("/v1/chat/completions", func(c *gin.Context) {
		defer common.CleanupBodyStorage(c)
		c.Status(http.StatusOK)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-concurrency-full"}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusTooManyRequests, recorder.Code, recorder.Body.String())
}

type middlewareConcurrencyWaitResult struct {
	lease *service.ChannelConcurrencyLease
	err   error
}

func occupyMiddlewareChannelConcurrencyQueue(t *testing.T, channel *model.Channel) {
	t.Helper()
	heldLease, acquired, err := service.TryAcquireChannelConcurrency(context.Background(), channel)
	require.NoError(t, err)
	require.True(t, acquired)

	waitContext, cancelWait := context.WithCancel(context.Background())
	waitDone := make(chan middlewareConcurrencyWaitResult, 1)
	go func() {
		lease, _, waitErr := service.AcquireChannelConcurrencyWithWait(waitContext, channel)
		waitDone <- middlewareConcurrencyWaitResult{lease: lease, err: waitErr}
	}()
	require.Eventually(t, func() bool {
		loads, loadErr := service.GetChannelConcurrencyLoads(context.Background(), []*model.Channel{channel})
		return loadErr == nil && loads[channel.Id].Waiting == 1
	}, time.Second, 5*time.Millisecond)

	t.Cleanup(func() {
		cancelWait()
		require.NoError(t, service.ReleaseChannelConcurrency(context.Background(), heldLease))
		result := <-waitDone
		if result.lease != nil {
			require.NoError(t, service.ReleaseChannelConcurrency(context.Background(), result.lease))
		} else {
			require.ErrorIs(t, result.err, service.ErrChannelConcurrencyLimit)
		}
	})
}

func useMiddlewareMemoryChannelConcurrency(t *testing.T) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	previousRDB := common.RDB
	previousRedisEnabled := common.RedisEnabled
	common.RDB = nil
	common.RedisEnabled = false
	t.Cleanup(func() {
		common.RDB = previousRDB
		common.RedisEnabled = previousRedisEnabled
	})
}

func useMiddlewareChannelConcurrencyWaitSetting(t *testing.T, timeout time.Duration, interval time.Duration, maxWaiting int) {
	t.Helper()
	setting := operation_setting.GetChannelConcurrencySetting()
	original := setting
	setting.WaitEnabled = true
	setting.WaitTimeoutMS = int(timeout / time.Millisecond)
	setting.WaitIntervalMS = int(interval / time.Millisecond)
	setting.MaxWaitingPerChannel = maxWaiting
	setting.CooldownEnabled = false
	operation_setting.SetChannelConcurrencySettingForTest(setting)
	t.Cleanup(func() {
		operation_setting.SetChannelConcurrencySettingForTest(original)
	})
}

func useMiddlewareChannelSelectionDB(t *testing.T) {
	t.Helper()
	previousDB := model.DB
	previousMemoryCacheEnabled := common.MemoryCacheEnabled
	previousMainDatabaseType := common.MainDatabaseType()

	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, database.AutoMigrate(&model.Channel{}, &model.Ability{}))
	model.DB = database
	common.MemoryCacheEnabled = true
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)

	t.Cleanup(func() {
		model.DB = previousDB
		common.MemoryCacheEnabled = previousMemoryCacheEnabled
		common.SetMainDatabaseType(previousMainDatabaseType)
		if previousMemoryCacheEnabled && previousDB != nil {
			model.InitChannelCache()
		}
	})
}

func useMiddlewareChannelAffinityRule(t *testing.T) {
	t.Helper()
	setting := operation_setting.GetChannelAffinitySetting()
	original := *setting
	setting.Enabled = true
	setting.DefaultTTLSeconds = 60
	setting.Rules = []operation_setting.ChannelAffinityRule{
		{
			Name:       fmt.Sprintf("middleware-affinity-%d", time.Now().UnixNano()),
			ModelRegex: []string{"^gpt-affinity-wait$"},
			PathRegex:  []string{"/v1/chat/completions"},
			KeySources: []operation_setting.ChannelAffinityKeySource{
				{Type: "request_header", Key: "X-Test-Affinity"},
			},
			IncludeRuleName:   true,
			IncludeModelName:  true,
			IncludeUsingGroup: true,
		},
	}
	t.Cleanup(func() {
		*setting = original
	})
}
