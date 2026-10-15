package middleware

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var prometheusAuthTestSequence atomic.Uint64

func useRateLimitMiniRedis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()

	previousRedisEnabled := common.RedisEnabled
	previousRedisClient := common.RDB
	redisServer := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	require.NoError(t, redisClient.Ping(context.Background()).Err())

	common.RedisEnabled = true
	common.RDB = redisClient
	t.Cleanup(func() {
		_ = redisClient.Close()
		common.RedisEnabled = previousRedisEnabled
		common.RDB = previousRedisClient
	})

	return redisServer, redisClient
}

func performRateLimitRequest(router http.Handler, path string, remoteAddr string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, path, nil)
	request.RemoteAddr = remoteAddr
	router.ServeHTTP(recorder, request)
	return recorder
}

func TestPrometheusMetricsAuthentication(testContext *testing.T) {
	gin.SetMode(gin.TestMode)
	invocation := prometheusAuthTestSequence.Add(1)
	for scenarioIndex, scenario := range []struct {
		name          string
		redis         bool
		uninitialized bool
	}{
		{name: "memory"},
		{name: "redis", redis: true},
		{name: "uninitialized redis", redis: true, uninitialized: true},
	} {
		testContext.Run(scenario.name, func(testContext *testing.T) {
			address := fmt.Sprintf("[2001:db8:%x::%x]:1234", invocation, scenarioIndex+1)
			previousEnabled, previousClient := common.RedisEnabled, common.RDB
			testContext.Cleanup(func() { common.RedisEnabled, common.RDB = previousEnabled, previousClient })
			common.RedisEnabled, common.RDB = scenario.redis, nil
			if scenario.redis && !scenario.uninitialized {
				useRateLimitMiniRedis(testContext)
			}
			testContext.Setenv(PrometheusMetricsTokenEnv, "monitor-secret")
			testContext.Setenv("PROMETHEUS_METRICS_RATE_LIMIT", "1")
			testContext.Setenv("PROMETHEUS_METRICS_RATE_LIMIT_DURATION", "60")
			router := gin.New()
			require.NoError(testContext, router.SetTrustedProxies(nil))
			router.GET("/metrics", PrometheusMetricsAuth(), func(c *gin.Context) { c.Status(http.StatusNoContent) })
			request := func(address, token string) *httptest.ResponseRecorder {
				recorder := httptest.NewRecorder()
				incoming := httptest.NewRequest(http.MethodGet, "/metrics", nil)
				incoming.RemoteAddr = address
				incoming.Header.Set("X-Forwarded-For", "127.0.0.1")
				incoming.Header.Set("Authorization", token)
				router.ServeHTTP(recorder, incoming)
				assert.NotContains(testContext, recorder.Body.String(), "monitor-secret")
				return recorder
			}
			assert.Equal(testContext, http.StatusNoContent, request("127.0.0.1:1234", "").Code)
			assert.Equal(testContext, http.StatusNoContent, request("[::1]:1234", "").Code)
			assert.Equal(testContext, http.StatusNoContent, request(address, "Bearer monitor-secret").Code)
			assert.Equal(testContext, http.StatusNoContent, request(address, "bearer monitor-secret").Code)
			for tokenIndex, token := range []string{"", "Basic monitor-secret", "Bearer monitor-secret extra"} {
				invalidAddress := fmt.Sprintf("[2001:db8:%x:%x::%x]:1234", invocation, scenarioIndex+1, tokenIndex+1)
				assert.Equal(testContext, http.StatusUnauthorized, request(invalidAddress, token).Code)
			}
			assert.Equal(testContext, http.StatusUnauthorized, request(address, "Bearer wrong").Code)
			assert.Equal(testContext, http.StatusTooManyRequests, request(address, "malformed").Code)
			assert.Equal(testContext, http.StatusNoContent, request(address, "Bearer monitor-secret").Code)
			testContext.Setenv(PrometheusMetricsTokenEnv, " ")
			assert.Equal(testContext, http.StatusServiceUnavailable, request(address, "Bearer monitor-secret").Code)
			assert.Equal(testContext, http.StatusNoContent, request("127.0.0.1:1234", "").Code)
		})
	}
}

func TestPrometheusAuthenticationNonpositiveLimitsUseDefaultsAndFailClosed(testContext *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, value := range []string{"0", "-1"} {
		testContext.Run(value, func(testContext *testing.T) {
			server, _ := useRateLimitMiniRedis(testContext)
			testContext.Setenv(PrometheusMetricsTokenEnv, "monitor-secret")
			testContext.Setenv("PROMETHEUS_METRICS_RATE_LIMIT", value)
			testContext.Setenv("PROMETHEUS_METRICS_RATE_LIMIT_DURATION", value)
			router := gin.New()
			require.NoError(testContext, router.SetTrustedProxies(nil))
			router.GET("/metrics", PrometheusMetricsAuth(), func(c *gin.Context) { c.Status(http.StatusNoContent) })
			for range 60 {
				require.Equal(testContext, http.StatusUnauthorized, performRateLimitRequest(router, "/metrics", "192.0.2.211:1234").Code)
			}
			require.Equal(testContext, http.StatusTooManyRequests, performRateLimitRequest(router, "/metrics", "192.0.2.211:1234").Code)
			server.FastForward(59 * time.Second)
			require.Equal(testContext, http.StatusTooManyRequests, performRateLimitRequest(router, "/metrics", "192.0.2.211:1234").Code)
			server.FastForward(time.Second)
			require.Equal(testContext, http.StatusUnauthorized, performRateLimitRequest(router, "/metrics", "192.0.2.211:1234").Code)
			server.Close()
			require.Equal(testContext, http.StatusInternalServerError, performRateLimitRequest(router, "/metrics", "192.0.2.211:1234").Code)
			request := httptest.NewRequest(http.MethodGet, "/metrics", nil)
			request.RemoteAddr = "192.0.2.211:1234"
			request.Header.Set("Authorization", "Bearer monitor-secret")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			require.Equal(testContext, http.StatusNoContent, response.Code)
		})
	}
}

func TestRedisIPRateLimiterThresholdTTLAndNamespace(t *testing.T) {
	gin.SetMode(gin.TestMode)
	redisServer, _ := useRateLimitMiniRedis(t)

	router := gin.New()
	require.NoError(t, router.SetTrustedProxies(nil))
	router.GET("/limited", rateLimitFactory(2, 37, "TEST"), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	remoteAddr := "192.0.2.10:12345"
	legacyKey := "rateLimit:TEST192.0.2.10"
	_, err := redisServer.Push(legacyKey, "legacy-list-entry")
	require.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, performRateLimitRequest(router, "/limited", remoteAddr).Code)
	assert.Equal(t, http.StatusNoContent, performRateLimitRequest(router, "/limited", remoteAddr).Code)
	limitedResponse := performRateLimitRequest(router, "/limited", remoteAddr)
	assert.Equal(t, http.StatusTooManyRequests, limitedResponse.Code)
	assert.Equal(t, "37", limitedResponse.Header().Get("Retry-After"))

	key := redisIPRateLimitKey("TEST", "192.0.2.10")
	count, err := redisServer.Get(key)
	require.NoError(t, err)
	assert.Equal(t, "3", count)
	assert.Equal(t, 37*time.Second, redisServer.TTL(key))
	assert.True(t, redisServer.Exists(legacyKey), "the v2 counter must not touch an old list key")
}

func TestRedisUserRateLimiterUsesSharedFixedWindow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	redisServer, _ := useRateLimitMiniRedis(t)

	router := gin.New()
	router.GET(
		"/limited",
		func(c *gin.Context) { c.Set("id", 42) },
		userRateLimitFactory(1, 23, "USER"),
		func(c *gin.Context) { c.Status(http.StatusNoContent) },
	)

	assert.Equal(t, http.StatusNoContent, performRateLimitRequest(router, "/limited", "192.0.2.20:12345").Code)
	assert.Equal(t, http.StatusTooManyRequests, performRateLimitRequest(router, "/limited", "198.51.100.20:12345").Code)

	key := redisUserRateLimitKey("USER", 42)
	assert.True(t, redisServer.Exists(key))
	assert.Equal(t, 23*time.Second, redisServer.TTL(key))
}

func TestRedisEmailVerificationRateLimiterPreservesResponseAndTTL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	redisServer, _ := useRateLimitMiniRedis(t)

	router := gin.New()
	require.NoError(t, router.SetTrustedProxies(nil))
	router.GET("/verify", EmailVerificationRateLimit(), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	remoteAddr := "192.0.2.30:12345"
	assert.Equal(t, http.StatusNoContent, performRateLimitRequest(router, "/verify", remoteAddr).Code)
	assert.Equal(t, http.StatusNoContent, performRateLimitRequest(router, "/verify", remoteAddr).Code)
	response := performRateLimitRequest(router, "/verify", remoteAddr)
	assert.Equal(t, http.StatusTooManyRequests, response.Code)
	assert.JSONEq(t, `{"success":false,"message":"发送过于频繁，请等待 30 秒后再试"}`, response.Body.String())

	key := redisIPRateLimitKey(EmailVerificationRateLimitMark, "192.0.2.30")
	assert.True(t, redisServer.Exists(key))
	assert.Equal(t, time.Duration(EmailVerificationDuration)*time.Second, redisServer.TTL(key))
}

func TestRedisFixedWindowIsAtomicUnderConcurrency(t *testing.T) {
	redisServer, _ := useRateLimitMiniRedis(t)
	const (
		requestCount = 20
		maximumCount = 7
		duration     = int64(41)
	)
	key := redisIPRateLimitKey("CONCURRENT", "192.0.2.40")

	var allowedCount atomic.Int64
	errorsFound := make(chan error, requestCount)
	var waitGroup sync.WaitGroup
	waitGroup.Add(requestCount)
	for range requestCount {
		go func() {
			defer waitGroup.Done()
			allowed, _, _, err := redisFixedWindowTake(context.Background(), key, maximumCount, duration)
			if err != nil {
				errorsFound <- err
				return
			}
			if allowed {
				allowedCount.Add(1)
			}
		}()
	}
	waitGroup.Wait()
	close(errorsFound)
	for err := range errorsFound {
		require.NoError(t, err)
	}

	assert.Equal(t, int64(maximumCount), allowedCount.Load())
	count, err := redisServer.Get(key)
	require.NoError(t, err)
	assert.Equal(t, "20", count)
	assert.Equal(t, time.Duration(duration)*time.Second, redisServer.TTL(key))
}

func TestRedisFixedWindowResetsAtBoundary(t *testing.T) {
	redisServer, _ := useRateLimitMiniRedis(t)
	const duration = int64(10)
	key := redisIPRateLimitKey("BOUNDARY", "192.0.2.50")

	for range 2 {
		allowed, _, _, err := redisFixedWindowTake(context.Background(), key, 2, duration)
		require.NoError(t, err)
		assert.True(t, allowed)
	}
	allowed, _, _, err := redisFixedWindowTake(context.Background(), key, 2, duration)
	require.NoError(t, err)
	assert.False(t, allowed)

	// This reset is intentional fixed-window behavior. A client can consume one
	// full allowance immediately before and another immediately after a boundary.
	redisServer.FastForward(time.Duration(duration) * time.Second)
	for range 2 {
		allowed, _, _, err = redisFixedWindowTake(context.Background(), key, 2, duration)
		require.NoError(t, err)
		assert.True(t, allowed)
	}
}

func TestRedisFixedWindowRepairsCounterWithoutTTL(t *testing.T) {
	redisServer, _ := useRateLimitMiniRedis(t)
	const duration = int64(29)
	key := redisIPRateLimitKey("MISSING-TTL", "192.0.2.51")
	redisServer.Set(key, "5")

	allowed, count, ttl, err := redisFixedWindowTake(context.Background(), key, 3, duration)
	require.NoError(t, err)
	assert.False(t, allowed)
	assert.Equal(t, int64(6), count)
	assert.Equal(t, duration, ttl)
	assert.Equal(t, time.Duration(duration)*time.Second, redisServer.TTL(key))

	redisServer.FastForward(time.Duration(duration) * time.Second)
	assert.False(t, redisServer.Exists(key), "a recovered counter must not remain permanently rate-limited")
}

func TestRedisFailurePolicies(t *testing.T) {
	gin.SetMode(gin.TestMode)
	_, redisClient := useRateLimitMiniRedis(t)
	require.NoError(t, redisClient.Close())

	router := gin.New()
	require.NoError(t, router.SetTrustedProxies(nil))
	router.GET("/ip", rateLimitFactory(1, 30, "FAIL-IP"), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	router.GET(
		"/user",
		func(c *gin.Context) { c.Set("id", 7) },
		userRateLimitFactory(1, 30, "FAIL-USER"),
		func(c *gin.Context) { c.Status(http.StatusNoContent) },
	)
	router.GET("/email", EmailVerificationRateLimit(), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	ipResponse := performRateLimitRequest(router, "/ip", "192.0.2.60:12345")
	assert.Equal(t, http.StatusInternalServerError, ipResponse.Code)
	assert.Empty(t, ipResponse.Body.String())
	userResponse := performRateLimitRequest(router, "/user", "192.0.2.61:12345")
	assert.Equal(t, http.StatusInternalServerError, userResponse.Code)
	assert.Empty(t, userResponse.Body.String())
	assert.Equal(t, http.StatusNoContent, performRateLimitRequest(router, "/email", "192.0.2.62:12345").Code)
}
