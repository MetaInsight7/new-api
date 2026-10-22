package service

import (
	"context"
	"errors"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/require"
)

func TestAcquireChannelConcurrencyWithWaitNeverExceedsQueueLimit(t *testing.T) {
	resetChannelConcurrencyForTest()
	miniRedis := miniredis.RunT(t)
	incrHook := newBlockingRedisIncrementHook(3)
	previousRDB := common.RDB
	previousRedisEnabled := common.RedisEnabled
	common.RDB = redis.NewClient(&redis.Options{Addr: miniRedis.Addr()})
	common.RDB.AddHook(incrHook)
	common.RedisEnabled = true
	t.Cleanup(func() {
		incrHook.Unblock()
		_ = common.RDB.Close()
		common.RDB = previousRDB
		common.RedisEnabled = previousRedisEnabled
		resetChannelConcurrencyForTest()
	})

	previousSetting := operation_setting.GetChannelConcurrencySetting()
	setting := previousSetting
	setting.SlotTTLMinutes = 1
	setting.WaitEnabled = true
	setting.WaitTimeoutMS = 2000
	setting.WaitIntervalMS = 100
	setting.MaxWaitingPerChannel = 3
	setting.CooldownEnabled = false
	operation_setting.SetChannelConcurrencySettingForTest(setting)
	t.Cleanup(func() {
		operation_setting.SetChannelConcurrencySettingForTest(previousSetting)
	})

	channel := &model.Channel{Id: 1201, MaxConcurrency: 1}
	heldLease, acquired, err := TryAcquireChannelConcurrency(context.Background(), channel)
	require.NoError(t, err)
	require.True(t, acquired)
	t.Cleanup(func() {
		require.NoError(t, ReleaseChannelConcurrency(context.Background(), heldLease))
	})

	const waiterCount = 20
	results := make(chan error, waiterCount)
	cancels := make([]context.CancelFunc, 0, waiterCount)
	for range waiterCount {
		waitContext, cancel := context.WithCancel(context.Background())
		cancels = append(cancels, cancel)
		go func() {
			lease, _, acquireErr := AcquireChannelConcurrencyWithWait(waitContext, channel)
			if lease != nil {
				_ = ReleaseChannelConcurrency(context.Background(), lease)
			}
			results <- acquireErr
		}()
	}
	defer func() {
		incrHook.Unblock()
		for _, cancel := range cancels {
			cancel()
		}
	}()

	require.Eventually(t, func() bool {
		return incrHook.Count() > setting.MaxWaitingPerChannel || len(results) >= waiterCount-setting.MaxWaitingPerChannel
	}, time.Second, 5*time.Millisecond)

	waitingValue, err := miniRedis.Get(channelConcurrencyWaitingRedisKey(channel.Id))
	require.NoError(t, err)
	waiting, err := strconv.Atoi(waitingValue)
	require.NoError(t, err)
	require.LessOrEqual(t, waiting, setting.MaxWaitingPerChannel, "atomic registration must never expose more than the configured queue bound")

	incrHook.Unblock()
	for _, cancel := range cancels {
		cancel()
	}
	for range waiterCount {
		select {
		case <-results:
		case <-time.After(time.Second):
			t.Fatal("waiting request did not exit after cancellation")
		}
	}
	require.Eventually(t, func() bool {
		return !miniRedis.Exists(channelConcurrencyWaitingRedisKey(channel.Id))
	}, time.Second, 5*time.Millisecond)
}

func TestAcquireChannelConcurrencyWithWaitBacksOffBetweenAuthoritativeAttempts(t *testing.T) {
	resetChannelConcurrencyForTest()
	miniRedis := miniredis.RunT(t)
	channel := &model.Channel{Id: 1202, MaxConcurrency: 1}
	timelineHook := &redisAcquireTimelineHook{key: channelConcurrencyRedisKey(channel.Id)}
	previousRDB := common.RDB
	previousRedisEnabled := common.RedisEnabled
	common.RDB = redis.NewClient(&redis.Options{Addr: miniRedis.Addr()})
	common.RDB.AddHook(timelineHook)
	common.RedisEnabled = true
	t.Cleanup(func() {
		_ = common.RDB.Close()
		common.RDB = previousRDB
		common.RedisEnabled = previousRedisEnabled
		resetChannelConcurrencyForTest()
	})

	previousSetting := operation_setting.GetChannelConcurrencySetting()
	setting := previousSetting
	setting.SlotTTLMinutes = 1
	setting.WaitEnabled = true
	setting.WaitTimeoutMS = 520
	setting.WaitIntervalMS = 100
	setting.MaxWaitingPerChannel = 1
	setting.CooldownEnabled = false
	operation_setting.SetChannelConcurrencySettingForTest(setting)
	t.Cleanup(func() {
		operation_setting.SetChannelConcurrencySettingForTest(previousSetting)
	})

	warmupLease, acquired, err := TryAcquireChannelConcurrency(context.Background(), &model.Channel{Id: 1299, MaxConcurrency: 1})
	require.NoError(t, err)
	require.True(t, acquired)
	require.NoError(t, ReleaseChannelConcurrency(context.Background(), warmupLease))

	heldLease, acquired, err := TryAcquireChannelConcurrency(context.Background(), channel)
	require.NoError(t, err)
	require.True(t, acquired)
	t.Cleanup(func() {
		require.NoError(t, ReleaseChannelConcurrency(context.Background(), heldLease))
	})
	timelineHook.Reset()

	lease, acquired, err := AcquireChannelConcurrencyWithWait(context.Background(), channel)
	require.ErrorIs(t, err, ErrChannelConcurrencyLimit)
	require.False(t, acquired)
	require.Nil(t, lease)

	attempts := timelineHook.Timestamps()
	require.LessOrEqual(t, len(attempts), 4, "exponential backoff must bound authoritative retries within the wait timeout")
	if len(attempts) == 4 {
		require.GreaterOrEqual(t, attempts[3].Sub(attempts[2]), 180*time.Millisecond)
	}
}

func TestAcquireChannelConcurrencyWithWaitNormalizesCanceledRedisRetry(t *testing.T) {
	resetChannelConcurrencyForTest()
	miniRedis := miniredis.RunT(t)
	channel := &model.Channel{Id: 1211, MaxConcurrency: 1}
	waitContext, cancelWait := context.WithCancel(context.Background())
	cancelHook := &cancelRedisAcquireRetryHook{
		key:    channelConcurrencyRedisKey(channel.Id),
		cancel: cancelWait,
	}
	previousRDB, previousRedisEnabled := common.RDB, common.RedisEnabled
	common.RDB = redis.NewClient(&redis.Options{Addr: miniRedis.Addr()})
	common.RDB.AddHook(cancelHook)
	common.RedisEnabled = true
	t.Cleanup(func() {
		cancelWait()
		require.NoError(t, common.RDB.Close())
		common.RDB, common.RedisEnabled = previousRDB, previousRedisEnabled
		resetChannelConcurrencyForTest()
	})

	previousSetting := operation_setting.GetChannelConcurrencySetting()
	setting := previousSetting
	setting.SlotTTLMinutes = 1
	setting.WaitEnabled = true
	setting.WaitTimeoutMS = 1000
	setting.WaitIntervalMS = 20
	setting.MaxWaitingPerChannel = 1
	setting.CooldownEnabled = false
	operation_setting.SetChannelConcurrencySettingForTest(setting)
	t.Cleanup(func() { operation_setting.SetChannelConcurrencySettingForTest(previousSetting) })

	heldLease, acquired, err := TryAcquireChannelConcurrency(context.Background(), channel)
	require.NoError(t, err)
	require.True(t, acquired)
	t.Cleanup(func() { require.NoError(t, ReleaseChannelConcurrency(context.Background(), heldLease)) })
	cancelHook.armed.Store(true)

	lease, acquired, err := AcquireChannelConcurrencyWithWait(waitContext, channel)
	require.ErrorIs(t, err, ErrChannelConcurrencyLimit)
	require.False(t, acquired)
	require.Nil(t, lease)
	require.ErrorIs(t, waitContext.Err(), context.Canceled)
	require.False(t, miniRedis.Exists(channelConcurrencyWaitingRedisKey(channel.Id)))
}

func TestTryAcquireChannelConcurrencyCleansUncertainRedisResultOnce(t *testing.T) {
	resetChannelConcurrencyForTest()
	miniRedis := miniredis.RunT(t)
	channel := &model.Channel{Id: 1203, MaxConcurrency: 2}
	requestContext, cancelRequest := context.WithCancel(context.Background())
	uncertainHook := &uncertainRedisAcquireHook{
		key:       channelConcurrencyRedisKey(channel.Id),
		cancel:    cancelRequest,
		resultErr: errors.New("redis result lost after commit"),
	}
	previousRDB := common.RDB
	previousRedisEnabled := common.RedisEnabled
	common.RDB = redis.NewClient(&redis.Options{Addr: miniRedis.Addr()})
	common.RDB.AddHook(uncertainHook)
	common.RedisEnabled = true
	t.Cleanup(func() {
		cancelRequest()
		_ = common.RDB.Close()
		common.RDB = previousRDB
		common.RedisEnabled = previousRedisEnabled
		resetChannelConcurrencyForTest()
	})

	previousSetting := operation_setting.GetChannelConcurrencySetting()
	setting := previousSetting
	setting.SlotTTLMinutes = 1
	setting.CooldownEnabled = false
	operation_setting.SetChannelConcurrencySettingForTest(setting)
	t.Cleanup(func() {
		operation_setting.SetChannelConcurrencySettingForTest(previousSetting)
	})

	heldLease, acquired, err := TryAcquireChannelConcurrency(context.Background(), channel)
	require.NoError(t, err)
	require.True(t, acquired)
	t.Cleanup(func() { require.NoError(t, ReleaseChannelConcurrency(context.Background(), heldLease)) })
	uncertainHook.Enable()

	lease, acquired, err := TryAcquireChannelConcurrency(requestContext, channel)
	require.ErrorContains(t, err, uncertainHook.resultErr.Error())
	require.False(t, acquired)
	require.Nil(t, lease)
	require.Error(t, requestContext.Err())

	require.Eventually(t, func() bool {
		members, membersErr := miniRedis.ZMembers(channelConcurrencyRedisKey(channel.Id))
		return membersErr == nil && len(members) == 1 && members[0] == heldLease.token
	}, time.Second, 5*time.Millisecond)
	require.Equal(t, int32(1), uncertainHook.ZRemCount())
}

func TestTryAcquireChannelConcurrencyRedisNeverExceedsLimit(t *testing.T) {
	miniRedis := useRedisChannelConcurrencyLifecycle(t, nil)
	channel := &model.Channel{Id: 1204, MaxConcurrency: 3}

	const contenderCount = 32
	type acquireResult struct {
		lease    *ChannelConcurrencyLease
		acquired bool
		err      error
	}
	results := make(chan acquireResult, contenderCount)
	var contenders sync.WaitGroup
	for range contenderCount {
		contenders.Add(1)
		go func() {
			defer contenders.Done()
			lease, acquired, err := TryAcquireChannelConcurrency(context.Background(), channel)
			results <- acquireResult{lease: lease, acquired: acquired, err: err}
		}()
	}
	contenders.Wait()
	close(results)

	leases := make([]*ChannelConcurrencyLease, 0, channel.MaxConcurrency)
	for result := range results {
		require.NoError(t, result.err)
		if result.acquired {
			require.NotNil(t, result.lease)
			leases = append(leases, result.lease)
		} else {
			require.Nil(t, result.lease)
		}
	}
	require.Len(t, leases, channel.MaxConcurrency)
	members, err := miniRedis.ZMembers(channelConcurrencyRedisKey(channel.Id))
	require.NoError(t, err)
	require.Len(t, members, channel.MaxConcurrency)

	for _, lease := range leases {
		require.NoError(t, ReleaseChannelConcurrency(context.Background(), lease))
	}
	require.False(t, miniRedis.Exists(channelConcurrencyRedisKey(channel.Id)))
}

func TestTryAcquireChannelConcurrencyReclaimsExpiredRedisLease(t *testing.T) {
	miniRedis := useRedisChannelConcurrencyLifecycle(t, nil)
	channel := &model.Channel{Id: 1205, MaxConcurrency: 1}

	redisTime, err := common.RDB.Time(context.Background()).Result()
	require.NoError(t, err)
	require.NoError(t, common.RDB.ZAdd(context.Background(), channelConcurrencyRedisKey(channel.Id), &redis.Z{
		Score:  float64(redisTime.Add(-time.Second).UnixMilli()),
		Member: "expired-request",
	}).Err())

	lease, acquired, err := TryAcquireChannelConcurrency(context.Background(), channel)
	require.NoError(t, err)
	require.True(t, acquired)
	require.NotNil(t, lease)
	members, err := miniRedis.ZMembers(channelConcurrencyRedisKey(channel.Id))
	require.NoError(t, err)
	require.Equal(t, []string{lease.token}, members)
	require.NoError(t, ReleaseChannelConcurrency(context.Background(), lease))
}

func TestRedisChannelConcurrencyLeaseRenewsUntilRelease(t *testing.T) {
	miniRedis := useRedisChannelConcurrencyLifecycle(t, nil)
	previousRenewInterval := channelConcurrencyRenewInterval
	channelConcurrencyRenewInterval = func(time.Duration) time.Duration {
		return 5 * time.Millisecond
	}
	t.Cleanup(func() {
		channelConcurrencyRenewInterval = previousRenewInterval
	})

	channel := &model.Channel{Id: 1206, MaxConcurrency: 1}
	requestContext, cancelRequest := context.WithCancel(context.Background())
	lease, acquired, err := TryAcquireChannelConcurrency(requestContext, channel)
	require.NoError(t, err)
	require.True(t, acquired)
	require.NotNil(t, lease)

	key := channelConcurrencyRedisKey(channel.Id)
	initialScore, err := common.RDB.ZScore(context.Background(), key, lease.token).Result()
	require.NoError(t, err)
	cancelRequest()
	miniRedis.FastForward(30 * time.Second)
	require.Eventually(t, func() bool {
		currentScore, scoreErr := common.RDB.ZScore(context.Background(), key, lease.token).Result()
		return scoreErr == nil && currentScore > initialScore
	}, time.Second, 5*time.Millisecond)
	members, err := miniRedis.ZMembers(key)
	require.NoError(t, err)
	require.Equal(t, []string{lease.token}, members)

	require.NoError(t, ReleaseChannelConcurrency(context.Background(), lease))
	require.False(t, miniRedis.Exists(key))
}

func TestTryAcquireChannelConcurrencyRedisFailureStaysClosed(t *testing.T) {
	failureHook := &redisAcquireFailureHook{err: errors.New("redis acquire unavailable")}
	useRedisChannelConcurrencyLifecycle(t, failureHook)
	channel := &model.Channel{Id: 1207, MaxConcurrency: 1}

	lease, acquired, err := TryAcquireChannelConcurrency(context.Background(), channel)
	require.ErrorContains(t, err, failureHook.err.Error())
	require.False(t, acquired)
	require.Nil(t, lease)

	common.RedisEnabled = false
	lease, acquired, err = TryAcquireChannelConcurrency(context.Background(), channel)
	require.NoError(t, err)
	require.True(t, acquired, "a Redis error must not consume a hidden process-local slot")
	require.NotNil(t, lease)
	require.NoError(t, ReleaseChannelConcurrency(context.Background(), lease))
}

func TestUnlimitedChannelConcurrencyDoesNotUseRedis(t *testing.T) {
	commandHook := &redisCommandCountHook{}
	useRedisChannelConcurrencyLifecycle(t, commandHook)
	channel := &model.Channel{Id: 1208, MaxConcurrency: 0}

	lease, acquired, err := TryAcquireChannelConcurrency(context.Background(), channel)
	require.NoError(t, err)
	require.True(t, acquired)
	require.Nil(t, lease)

	loads, err := GetChannelConcurrencyLoads(context.Background(), []*model.Channel{channel})
	require.NoError(t, err)
	require.Equal(t, 0, loads[channel.Id].MaxConcurrency)
	require.Equal(t, int32(0), commandHook.Count())
}

func TestAcquireChannelConcurrencyWithWaitReleasesWaitingAndActiveLeases(t *testing.T) {
	miniRedis := useRedisChannelConcurrencyLifecycle(t, nil)
	previousSetting := operation_setting.GetChannelConcurrencySetting()
	setting := previousSetting
	setting.WaitEnabled = true
	setting.WaitTimeoutMS = 200
	setting.WaitIntervalMS = 10
	setting.MaxWaitingPerChannel = 1
	setting.CooldownEnabled = false
	operation_setting.SetChannelConcurrencySettingForTest(setting)
	t.Cleanup(func() {
		operation_setting.SetChannelConcurrencySettingForTest(previousSetting)
	})

	channel := &model.Channel{Id: 1209, MaxConcurrency: 1}
	heldLease, acquired, err := TryAcquireChannelConcurrency(context.Background(), channel)
	require.NoError(t, err)
	require.True(t, acquired)

	type waitResult struct {
		lease    *ChannelConcurrencyLease
		acquired bool
		err      error
	}
	resultChannel := make(chan waitResult, 1)
	go func() {
		lease, acquired, acquireErr := AcquireChannelConcurrencyWithWait(context.Background(), channel)
		resultChannel <- waitResult{lease: lease, acquired: acquired, err: acquireErr}
	}()
	require.Eventually(t, func() bool {
		return miniRedis.Exists(channelConcurrencyWaitingRedisKey(channel.Id))
	}, time.Second, 5*time.Millisecond)
	require.NoError(t, ReleaseChannelConcurrency(context.Background(), heldLease))

	result := <-resultChannel
	require.NoError(t, result.err)
	require.True(t, result.acquired)
	require.NotNil(t, result.lease)
	require.Eventually(t, func() bool {
		return !miniRedis.Exists(channelConcurrencyWaitingRedisKey(channel.Id))
	}, time.Second, 5*time.Millisecond)
	require.NoError(t, ReleaseChannelConcurrency(context.Background(), result.lease))
	require.False(t, miniRedis.Exists(channelConcurrencyRedisKey(channel.Id)))

	heldLease, acquired, err = TryAcquireChannelConcurrency(context.Background(), channel)
	require.NoError(t, err)
	require.True(t, acquired)
	lease, acquired, err := AcquireChannelConcurrencyWithWait(context.Background(), channel)
	require.ErrorIs(t, err, ErrChannelConcurrencyLimit)
	require.False(t, acquired)
	require.Nil(t, lease)
	require.False(t, miniRedis.Exists(channelConcurrencyWaitingRedisKey(channel.Id)))
	require.NoError(t, ReleaseChannelConcurrency(context.Background(), heldLease))
}

func TestAcquireChannelConcurrencyWithWaitDisabledDoesNotRegisterWaiter(t *testing.T) {
	miniRedis := useRedisChannelConcurrencyLifecycle(t, nil)
	previousSetting := operation_setting.GetChannelConcurrencySetting()
	setting := previousSetting
	setting.WaitEnabled = false
	setting.CooldownEnabled = false
	operation_setting.SetChannelConcurrencySettingForTest(setting)
	t.Cleanup(func() {
		operation_setting.SetChannelConcurrencySettingForTest(previousSetting)
	})

	channel := &model.Channel{Id: 1210, MaxConcurrency: 1}
	heldLease, acquired, err := TryAcquireChannelConcurrency(context.Background(), channel)
	require.NoError(t, err)
	require.True(t, acquired)
	t.Cleanup(func() {
		require.NoError(t, ReleaseChannelConcurrency(context.Background(), heldLease))
	})

	lease, acquired, err := AcquireChannelConcurrencyWithWait(context.Background(), channel)
	require.NoError(t, err)
	require.False(t, acquired)
	require.Nil(t, lease)
	require.False(t, miniRedis.Exists(channelConcurrencyWaitingRedisKey(channel.Id)))
}

func TestChannelConcurrencyRealRedisIntegration(t *testing.T) {
	address := os.Getenv("TEST_REDIS_ADDR")
	if address == "" {
		t.Skip("TEST_REDIS_ADDR is required for real Redis verification")
	}
	host, _, err := net.SplitHostPort(address)
	require.NoError(t, err)
	require.True(t, net.ParseIP(host).IsLoopback(), "Redis tests require a loopback instance")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)
	local := redis.NewClient(&redis.Options{Addr: address, DB: 12})
	peer := redis.NewClient(&redis.Options{Addr: address, DB: 12})
	require.NoError(t, local.Ping(ctx).Err())
	require.NoError(t, peer.Ping(ctx).Err())

	channelIDs := []int{91201, 91202, 91203, 91204}
	keys := make([]string, 0, len(channelIDs)*3)
	for _, channelID := range channelIDs {
		keys = append(keys,
			channelConcurrencyRedisKey(channelID),
			channelConcurrencyWaitingRedisKey(channelID),
			channelConcurrencyCooldownRedisKey(channelID),
		)
	}
	require.NoError(t, local.Del(ctx, keys...).Err())

	previousRDB, previousRedisEnabled := common.RDB, common.RedisEnabled
	previousSetting := operation_setting.GetChannelConcurrencySetting()
	setting := previousSetting
	setting.SlotTTLMinutes = 1
	setting.WaitEnabled = true
	setting.WaitTimeoutMS = 1500
	setting.WaitIntervalMS = 20
	setting.MaxWaitingPerChannel = 4
	setting.CooldownEnabled = true
	setting.CooldownSeconds = 1
	setting.LoadCacheEnabled = true
	setting.LoadCacheTTLMS = 400
	operation_setting.SetChannelConcurrencySettingForTest(setting)
	common.RDB, common.RedisEnabled = local, true
	resetChannelConcurrencyForTest()
	t.Cleanup(func() {
		common.RDB, common.RedisEnabled = local, true
		_ = local.Del(context.Background(), keys...).Err()
		resetChannelConcurrencyForTest()
		operation_setting.SetChannelConcurrencySettingForTest(previousSetting)
		common.RDB, common.RedisEnabled = previousRDB, previousRedisEnabled
		require.NoError(t, peer.Close())
		require.NoError(t, local.Close())
	})

	t.Run("two clients share the atomic limit", func(t *testing.T) {
		channel := &model.Channel{Id: channelIDs[0], MaxConcurrency: 3}
		redisTime, timeErr := peer.Time(ctx).Result()
		require.NoError(t, timeErr)
		require.NoError(t, peer.ZAdd(ctx, channelConcurrencyRedisKey(channel.Id), &redis.Z{
			Score:  float64(redisTime.Add(-time.Second).UnixMilli()),
			Member: "expired-real-redis-request",
		}).Err())

		first, acquired, acquireErr := TryAcquireChannelConcurrency(ctx, channel)
		require.NoError(t, acquireErr)
		require.True(t, acquired)
		require.NotNil(t, first)
		t.Cleanup(func() { _ = ReleaseChannelConcurrency(context.Background(), first) })

		common.RDB = peer
		const contenderCount = 31
		type acquireResult struct {
			lease    *ChannelConcurrencyLease
			acquired bool
			err      error
		}
		results := make(chan acquireResult, contenderCount)
		var contenders sync.WaitGroup
		for range contenderCount {
			contenders.Go(func() {
				lease, contenderAcquired, contenderErr := TryAcquireChannelConcurrency(context.Background(), channel)
				results <- acquireResult{lease: lease, acquired: contenderAcquired, err: contenderErr}
			})
		}
		contenders.Wait()
		close(results)

		leases := []*ChannelConcurrencyLease{first}
		for result := range results {
			require.NoError(t, result.err)
			if result.acquired {
				require.NotNil(t, result.lease)
				leases = append(leases, result.lease)
			} else {
				require.Nil(t, result.lease)
			}
		}
		require.Len(t, leases, channel.MaxConcurrency)
		for _, lease := range leases[1:] {
			t.Cleanup(func() { _ = ReleaseChannelConcurrency(context.Background(), lease) })
		}
		active, activeErr := peer.ZCard(ctx, channelConcurrencyRedisKey(channel.Id)).Result()
		require.NoError(t, activeErr)
		require.EqualValues(t, channel.MaxConcurrency, active)

		loads, loadErr := GetChannelConcurrencyLoadsFresh(ctx, []*model.Channel{channel})
		require.NoError(t, loadErr)
		require.Equal(t, channel.MaxConcurrency, loads[channel.Id].Active)
		require.Equal(t, 1.0, loads[channel.Id].LoadRate)

		for _, lease := range leases {
			require.NoError(t, ReleaseChannelConcurrency(ctx, lease))
		}
		require.Eventually(t, func() bool {
			count, countErr := peer.ZCard(context.Background(), channelConcurrencyRedisKey(channel.Id)).Result()
			return countErr == nil && count == 0
		}, time.Second, 10*time.Millisecond)
	})

	t.Run("snapshot cache expires into the real pipeline", func(t *testing.T) {
		common.RDB = local
		resetChannelConcurrencyForTest()
		channel := &model.Channel{Id: channelIDs[1], MaxConcurrency: 2}
		first, acquired, acquireErr := TryAcquireChannelConcurrency(ctx, channel)
		require.NoError(t, acquireErr)
		require.True(t, acquired)
		t.Cleanup(func() { _ = ReleaseChannelConcurrency(context.Background(), first) })

		loads, loadErr := GetChannelConcurrencyLoads(ctx, []*model.Channel{channel})
		require.NoError(t, loadErr)
		require.Equal(t, 1, loads[channel.Id].Active)

		second, acquired, acquireErr := TryAcquireChannelConcurrency(ctx, channel)
		require.NoError(t, acquireErr)
		require.True(t, acquired)
		t.Cleanup(func() { _ = ReleaseChannelConcurrency(context.Background(), second) })

		cached, cacheErr := GetChannelConcurrencyLoads(ctx, []*model.Channel{channel})
		require.NoError(t, cacheErr)
		require.Equal(t, 1, cached[channel.Id].Active)
		require.Eventually(t, func() bool {
			refreshed, refreshErr := GetChannelConcurrencyLoads(context.Background(), []*model.Channel{channel})
			return refreshErr == nil && refreshed[channel.Id].Active == 2
		}, 2*time.Second, 20*time.Millisecond)

		require.NoError(t, ReleaseChannelConcurrency(ctx, first))
		require.NoError(t, ReleaseChannelConcurrency(ctx, second))
		fresh, freshErr := GetChannelConcurrencyLoadsFresh(ctx, []*model.Channel{channel})
		require.NoError(t, freshErr)
		require.Zero(t, fresh[channel.Id].Active)
	})

	t.Run("wait cancellation and cooldown clean shared state", func(t *testing.T) {
		common.RDB = local
		resetChannelConcurrencyForTest()
		channel := &model.Channel{Id: channelIDs[2], MaxConcurrency: 1}
		held, acquired, acquireErr := TryAcquireChannelConcurrency(ctx, channel)
		require.NoError(t, acquireErr)
		require.True(t, acquired)
		t.Cleanup(func() { _ = ReleaseChannelConcurrency(context.Background(), held) })

		waitCtx, cancelWait := context.WithCancel(context.Background())
		t.Cleanup(cancelWait)
		result := make(chan error, 1)
		go func() {
			lease, _, waitErr := AcquireChannelConcurrencyWithWait(waitCtx, channel)
			if lease != nil {
				_ = ReleaseChannelConcurrency(context.Background(), lease)
			}
			result <- waitErr
		}()
		require.Eventually(t, func() bool {
			waiting, waitingErr := peer.Get(context.Background(), channelConcurrencyWaitingRedisKey(channel.Id)).Int()
			return waitingErr == nil && waiting == 1
		}, time.Second, 10*time.Millisecond)
		loads, loadErr := GetChannelConcurrencyLoadsFresh(ctx, []*model.Channel{channel})
		require.NoError(t, loadErr)
		require.Equal(t, 1, loads[channel.Id].Active)
		require.Equal(t, 1, loads[channel.Id].Waiting)
		cancelWait()
		require.ErrorIs(t, <-result, ErrChannelConcurrencyLimit)
		require.Eventually(t, func() bool {
			exists, existsErr := peer.Exists(context.Background(), channelConcurrencyWaitingRedisKey(channel.Id)).Result()
			return existsErr == nil && exists == 0
		}, time.Second, 10*time.Millisecond)
		require.NoError(t, ReleaseChannelConcurrency(ctx, held))

		cooldownChannel := &model.Channel{Id: channelIDs[3], MaxConcurrency: 1}
		require.NoError(t, MarkChannelConcurrencyCooldown(ctx, cooldownChannel.Id, 250*time.Millisecond, "real Redis integration"))
		lease, cooldownAcquired, cooldownErr := TryAcquireChannelConcurrency(ctx, cooldownChannel)
		require.NoError(t, cooldownErr)
		require.False(t, cooldownAcquired)
		require.Nil(t, lease)
		cooldownLoads, cooldownLoadErr := GetChannelConcurrencyLoadsFresh(ctx, []*model.Channel{cooldownChannel})
		require.NoError(t, cooldownLoadErr)
		require.True(t, cooldownLoads[cooldownChannel.Id].CoolingDown)

		var recovered *ChannelConcurrencyLease
		require.Eventually(t, func() bool {
			var recoveredErr error
			recovered, cooldownAcquired, recoveredErr = TryAcquireChannelConcurrency(context.Background(), cooldownChannel)
			return recoveredErr == nil && cooldownAcquired
		}, 2*time.Second, 20*time.Millisecond)
		require.NotNil(t, recovered)
		t.Cleanup(func() { _ = ReleaseChannelConcurrency(context.Background(), recovered) })
		require.NoError(t, ReleaseChannelConcurrency(ctx, recovered))
	})
}

type blockingRedisIncrementHook struct {
	count      atomic.Int32
	blockAfter int32
	unblock    chan struct{}
	unblockMu  sync.Once
}

func newBlockingRedisIncrementHook(blockAfter int32) *blockingRedisIncrementHook {
	return &blockingRedisIncrementHook{blockAfter: blockAfter, unblock: make(chan struct{})}
}

func (h *blockingRedisIncrementHook) BeforeProcess(ctx context.Context, cmd redis.Cmder) (context.Context, error) {
	return ctx, nil
}

func (h *blockingRedisIncrementHook) AfterProcess(_ context.Context, cmd redis.Cmder) error {
	if strings.EqualFold(cmd.Name(), "incr") {
		if h.count.Add(1) > h.blockAfter {
			<-h.unblock
		}
	}
	return nil
}

func (h *blockingRedisIncrementHook) BeforeProcessPipeline(ctx context.Context, _ []redis.Cmder) (context.Context, error) {
	return ctx, nil
}

func (h *blockingRedisIncrementHook) AfterProcessPipeline(context.Context, []redis.Cmder) error {
	return nil
}

func (h *blockingRedisIncrementHook) Count() int {
	return int(h.count.Load())
}

func (h *blockingRedisIncrementHook) Unblock() {
	h.unblockMu.Do(func() {
		close(h.unblock)
	})
}

type redisAcquireTimelineHook struct {
	key        string
	mu         sync.Mutex
	timestamps []time.Time
}

type cancelRedisAcquireRetryHook struct {
	key      string
	cancel   context.CancelFunc
	armed    atomic.Bool
	attempts atomic.Int32
}

func (h *cancelRedisAcquireRetryHook) BeforeProcess(ctx context.Context, cmd redis.Cmder) (context.Context, error) {
	if h.armed.Load() && redisCommandContainsKey(cmd, h.key) && h.attempts.Add(1) == 2 {
		h.cancel()
		return ctx, context.Canceled
	}
	return ctx, nil
}

func (h *cancelRedisAcquireRetryHook) AfterProcess(context.Context, redis.Cmder) error {
	return nil
}

func (h *cancelRedisAcquireRetryHook) BeforeProcessPipeline(ctx context.Context, _ []redis.Cmder) (context.Context, error) {
	return ctx, nil
}

func (h *cancelRedisAcquireRetryHook) AfterProcessPipeline(context.Context, []redis.Cmder) error {
	return nil
}

func (h *redisAcquireTimelineHook) BeforeProcess(ctx context.Context, cmd redis.Cmder) (context.Context, error) {
	if redisCommandContainsKey(cmd, h.key) {
		h.mu.Lock()
		h.timestamps = append(h.timestamps, time.Now())
		h.mu.Unlock()
	}
	return ctx, nil
}

func (h *redisAcquireTimelineHook) AfterProcess(context.Context, redis.Cmder) error {
	return nil
}

func (h *redisAcquireTimelineHook) BeforeProcessPipeline(ctx context.Context, _ []redis.Cmder) (context.Context, error) {
	return ctx, nil
}

func (h *redisAcquireTimelineHook) AfterProcessPipeline(context.Context, []redis.Cmder) error {
	return nil
}

func (h *redisAcquireTimelineHook) Reset() {
	h.mu.Lock()
	h.timestamps = nil
	h.mu.Unlock()
}

func (h *redisAcquireTimelineHook) Timestamps() []time.Time {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]time.Time(nil), h.timestamps...)
}

func redisCommandContainsKey(cmd redis.Cmder, key string) bool {
	name := strings.ToLower(cmd.Name())
	if name != "eval" && name != "evalsha" {
		return false
	}
	for _, argument := range cmd.Args() {
		if value, ok := argument.(string); ok && value == key {
			return true
		}
	}
	return false
}

type uncertainRedisAcquireHook struct {
	key       string
	cancel    context.CancelFunc
	resultErr error
	enabled   atomic.Bool
	injected  atomic.Bool
	zremCount atomic.Int32
}

func (h *uncertainRedisAcquireHook) BeforeProcess(ctx context.Context, cmd redis.Cmder) (context.Context, error) {
	if strings.EqualFold(cmd.Name(), "zrem") {
		for _, argument := range cmd.Args() {
			if value, ok := argument.(string); ok && value == h.key {
				h.zremCount.Add(1)
				break
			}
		}
	}
	return ctx, nil
}

func (h *uncertainRedisAcquireHook) AfterProcess(_ context.Context, cmd redis.Cmder) error {
	if h.enabled.Load() && redisCommandContainsKey(cmd, h.key) && h.injected.CompareAndSwap(false, true) {
		h.cancel()
		return h.resultErr
	}
	return nil
}

func (h *uncertainRedisAcquireHook) BeforeProcessPipeline(ctx context.Context, _ []redis.Cmder) (context.Context, error) {
	return ctx, nil
}

func (h *uncertainRedisAcquireHook) AfterProcessPipeline(context.Context, []redis.Cmder) error {
	return nil
}

func (h *uncertainRedisAcquireHook) Enable() {
	h.enabled.Store(true)
}

func (h *uncertainRedisAcquireHook) ZRemCount() int32 {
	return h.zremCount.Load()
}

type redisAcquireFailureHook struct {
	err error
}

func (h *redisAcquireFailureHook) BeforeProcess(ctx context.Context, cmd redis.Cmder) (context.Context, error) {
	name := strings.ToLower(cmd.Name())
	if name == "eval" || name == "evalsha" {
		return ctx, h.err
	}
	return ctx, nil
}

func (h *redisAcquireFailureHook) AfterProcess(context.Context, redis.Cmder) error {
	return nil
}

func (h *redisAcquireFailureHook) BeforeProcessPipeline(ctx context.Context, _ []redis.Cmder) (context.Context, error) {
	return ctx, nil
}

func (h *redisAcquireFailureHook) AfterProcessPipeline(context.Context, []redis.Cmder) error {
	return nil
}

type redisCommandCountHook struct {
	count atomic.Int32
}

func (h *redisCommandCountHook) BeforeProcess(ctx context.Context, _ redis.Cmder) (context.Context, error) {
	h.count.Add(1)
	return ctx, nil
}

func (h *redisCommandCountHook) AfterProcess(context.Context, redis.Cmder) error {
	return nil
}

func (h *redisCommandCountHook) BeforeProcessPipeline(ctx context.Context, commands []redis.Cmder) (context.Context, error) {
	h.count.Add(int32(len(commands)))
	return ctx, nil
}

func (h *redisCommandCountHook) AfterProcessPipeline(context.Context, []redis.Cmder) error {
	return nil
}

func (h *redisCommandCountHook) Count() int32 {
	return h.count.Load()
}

func useRedisChannelConcurrencyLifecycle(t *testing.T, hook redis.Hook) *miniredis.Miniredis {
	t.Helper()
	resetChannelConcurrencyForTest()
	miniRedis := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: miniRedis.Addr()})
	if hook != nil {
		client.AddHook(hook)
	}
	previousRDB := common.RDB
	previousRedisEnabled := common.RedisEnabled
	common.RDB = client
	common.RedisEnabled = true
	t.Cleanup(func() {
		_ = client.Close()
		common.RDB = previousRDB
		common.RedisEnabled = previousRedisEnabled
		resetChannelConcurrencyForTest()
	})
	return miniRedis
}
