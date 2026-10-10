package service

import (
	"context"
	"errors"
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

func TestTryAcquireChannelConcurrencyCleansUncertainRedisResultOnce(t *testing.T) {
	resetChannelConcurrencyForTest()
	miniRedis := miniredis.RunT(t)
	channel := &model.Channel{Id: 1203, MaxConcurrency: 1}
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

	warmupLease, acquired, err := TryAcquireChannelConcurrency(context.Background(), &model.Channel{Id: 1298, MaxConcurrency: 1})
	require.NoError(t, err)
	require.True(t, acquired)
	require.NoError(t, ReleaseChannelConcurrency(context.Background(), warmupLease))
	uncertainHook.Enable()

	lease, acquired, err := TryAcquireChannelConcurrency(requestContext, channel)
	require.ErrorContains(t, err, uncertainHook.resultErr.Error())
	require.False(t, acquired)
	require.Nil(t, lease)
	require.Error(t, requestContext.Err())

	require.Eventually(t, func() bool {
		return !miniRedis.Exists(channelConcurrencyRedisKey(channel.Id))
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
