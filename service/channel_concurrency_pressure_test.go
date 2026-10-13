package service

import (
	"context"
	"errors"
	"fmt"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type redisLoadCommandCounterHook struct {
	mu            sync.Mutex
	commands      []string
	pipelineSizes []int
	blockCommand  string
	block         <-chan struct{}
	started       chan struct{}
	startedOnce   sync.Once
	failCommand   string
	failErr       error
}

func (h *redisLoadCommandCounterHook) BeforeProcess(ctx context.Context, command redis.Cmder) (context.Context, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.commands = append(h.commands, strings.ToLower(command.Name()))
	if h.failErr != nil && strings.EqualFold(command.Name(), h.failCommand) {
		return ctx, h.failErr
	}
	shouldBlock := h.block != nil && strings.EqualFold(command.Name(), h.blockCommand)
	block := h.block
	started := h.started
	if shouldBlock && started != nil {
		h.startedOnce.Do(func() { close(started) })
	}
	if shouldBlock {
		h.mu.Unlock()
		select {
		case <-block:
		case <-ctx.Done():
			h.mu.Lock()
			return ctx, ctx.Err()
		}
		h.mu.Lock()
	}
	return ctx, nil
}

func (h *redisLoadCommandCounterHook) AfterProcess(context.Context, redis.Cmder) error {
	return nil
}

func (h *redisLoadCommandCounterHook) BeforeProcessPipeline(ctx context.Context, commands []redis.Cmder) (context.Context, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.pipelineSizes = append(h.pipelineSizes, len(commands))
	for _, command := range commands {
		h.commands = append(h.commands, strings.ToLower(command.Name()))
	}
	return ctx, nil
}

func (h *redisLoadCommandCounterHook) PipelineSizes() []int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]int(nil), h.pipelineSizes...)
}

func (h *redisLoadCommandCounterHook) AfterProcessPipeline(context.Context, []redis.Cmder) error {
	return nil
}

func (h *redisLoadCommandCounterHook) Commands() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]string(nil), h.commands...)
}

func countRedisLoadCommands(commands []string, name string) int {
	count := 0
	for _, command := range commands {
		if command == name {
			count++
		}
	}
	return count
}

func useCountingRedisChannelConcurrencyLoads(t *testing.T) *redisLoadCommandCounterHook {
	t.Helper()
	miniRedis := miniredis.RunT(t)
	hook := &redisLoadCommandCounterHook{}
	client := redis.NewClient(&redis.Options{Addr: miniRedis.Addr()})
	client.AddHook(hook)
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
	return hook
}

func useChannelConcurrencyLoadSetting(t *testing.T, setting operation_setting.ChannelConcurrencySetting) {
	t.Helper()
	previous := operation_setting.GetChannelConcurrencySetting()
	operation_setting.SetChannelConcurrencySettingForTest(setting)
	t.Cleanup(func() {
		operation_setting.SetChannelConcurrencySettingForTest(previous)
	})
}

func useChannelConcurrencySelectionDatabase(t *testing.T) *gorm.DB {
	t.Helper()
	previousDB := model.DB
	previousMemoryCacheEnabled := common.MemoryCacheEnabled
	database, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, database.AutoMigrate(&model.Channel{}, &model.Ability{}))
	model.DB = database
	common.MemoryCacheEnabled = true
	t.Cleanup(func() {
		model.DB = previousDB
		common.MemoryCacheEnabled = previousMemoryCacheEnabled
		if previousMemoryCacheEnabled && previousDB != nil && previousDB.Migrator().HasTable(&model.Channel{}) && previousDB.Migrator().HasTable(&model.Ability{}) {
			model.InitChannelCache()
		}
		sqlDatabase, err := database.DB()
		if err == nil {
			require.NoError(t, sqlDatabase.Close())
		}
	})
	return database
}

func TestChannelConcurrencyLoadCacheServesRepeatReadsWithinTTL(t *testing.T) {
	resetChannelConcurrencyForTest()
	hook := useCountingRedisChannelConcurrencyLoads(t)
	useChannelConcurrencyLoadSetting(t, operation_setting.ChannelConcurrencySetting{
		SlotTTLMinutes:     1,
		WaitEnabled:        true,
		WaitTimeoutMS:      5000,
		WaitIntervalMS:     100,
		CooldownEnabled:    true,
		CooldownSeconds:    30,
		LoadCacheEnabled:   true,
		LoadCacheTTLMS:     1000,
		MaxAcquireAttempts: 8,
	})
	channels := []*model.Channel{
		{Id: 401, MaxConcurrency: 4},
		{Id: 402, MaxConcurrency: 2},
	}

	first, err := GetChannelConcurrencyLoads(context.Background(), channels)
	require.NoError(t, err)
	require.Len(t, first, 2)
	commandsAfterFirst := len(hook.Commands())
	require.Positive(t, commandsAfterFirst)

	second, err := GetChannelConcurrencyLoads(context.Background(), channels)
	require.NoError(t, err)
	require.Equal(t, first, second)
	require.Equal(t, commandsAfterFirst, len(hook.Commands()))
}

func TestChannelConcurrencyLoadCacheDisabledAndExpiredReadsRedisAgain(t *testing.T) {
	t.Run("disabled", func(t *testing.T) {
		resetChannelConcurrencyForTest()
		hook := useCountingRedisChannelConcurrencyLoads(t)
		useChannelConcurrencyLoadSetting(t, operation_setting.ChannelConcurrencySetting{
			SlotTTLMinutes: 1, WaitEnabled: true, WaitTimeoutMS: 5000, WaitIntervalMS: 100,
			CooldownEnabled: true, CooldownSeconds: 30, LoadCacheEnabled: false, LoadCacheTTLMS: 200, MaxAcquireAttempts: 8,
		})
		channels := []*model.Channel{{Id: 403, MaxConcurrency: 4}}
		_, err := GetChannelConcurrencyLoads(context.Background(), channels)
		require.NoError(t, err)
		commandsAfterFirst := len(hook.Commands())
		_, err = GetChannelConcurrencyLoads(context.Background(), channels)
		require.NoError(t, err)
		require.Greater(t, len(hook.Commands()), commandsAfterFirst)
	})

	t.Run("expired", func(t *testing.T) {
		resetChannelConcurrencyForTest()
		hook := useCountingRedisChannelConcurrencyLoads(t)
		useChannelConcurrencyLoadSetting(t, operation_setting.ChannelConcurrencySetting{
			SlotTTLMinutes: 1, WaitEnabled: true, WaitTimeoutMS: 5000, WaitIntervalMS: 100,
			CooldownEnabled: true, CooldownSeconds: 30, LoadCacheEnabled: true, LoadCacheTTLMS: 20, MaxAcquireAttempts: 8,
		})
		channels := []*model.Channel{{Id: 405, MaxConcurrency: 4}}
		_, err := GetChannelConcurrencyLoads(context.Background(), channels)
		require.NoError(t, err)
		commandsAfterFirst := len(hook.Commands())
		time.Sleep(40 * time.Millisecond)
		_, err = GetChannelConcurrencyLoads(context.Background(), channels)
		require.NoError(t, err)
		require.Greater(t, len(hook.Commands()), commandsAfterFirst)
	})
}

func TestChannelConcurrencyLoadCacheTTLHotUpdateBoundsExistingEntries(t *testing.T) {
	resetChannelConcurrencyForTest()
	hook := useCountingRedisChannelConcurrencyLoads(t)
	setting := operation_setting.ChannelConcurrencySetting{
		SlotTTLMinutes: 1, WaitEnabled: true, WaitTimeoutMS: 5000, WaitIntervalMS: 100,
		CooldownEnabled: true, CooldownSeconds: 30, LoadCacheEnabled: true, LoadCacheTTLMS: 1000, MaxAcquireAttempts: 8,
	}
	useChannelConcurrencyLoadSetting(t, setting)
	channels := []*model.Channel{{Id: 409, MaxConcurrency: 4}}
	_, err := GetChannelConcurrencyLoads(context.Background(), channels)
	require.NoError(t, err)
	commandsAfterFirst := len(hook.Commands())

	setting.LoadCacheTTLMS = 5
	operation_setting.SetChannelConcurrencySettingForTest(setting)
	time.Sleep(10 * time.Millisecond)
	_, err = GetChannelConcurrencyLoads(context.Background(), channels)
	require.NoError(t, err)
	require.Greater(t, len(hook.Commands()), commandsAfterFirst)
}

func TestChannelConcurrencyLoadCacheKeyAndSnapshotsAreIsolated(t *testing.T) {
	resetChannelConcurrencyForTest()
	hook := useCountingRedisChannelConcurrencyLoads(t)
	useChannelConcurrencyLoadSetting(t, operation_setting.ChannelConcurrencySetting{
		SlotTTLMinutes: 1, WaitEnabled: true, WaitTimeoutMS: 5000, WaitIntervalMS: 100,
		CooldownEnabled: true, CooldownSeconds: 30, LoadCacheEnabled: true, LoadCacheTTLMS: 1000, MaxAcquireAttempts: 8,
	})
	firstChannel := &model.Channel{Id: 406, MaxConcurrency: 2}
	secondChannel := &model.Channel{Id: 407, MaxConcurrency: 3}
	loads, err := GetChannelConcurrencyLoads(context.Background(), []*model.Channel{firstChannel, secondChannel})
	require.NoError(t, err)
	loads[firstChannel.Id] = ChannelConcurrencyLoad{ChannelID: firstChannel.Id, MaxConcurrency: 99, Active: 99}
	commandsAfterFirst := len(hook.Commands())

	reversed, err := GetChannelConcurrencyLoads(context.Background(), []*model.Channel{secondChannel, firstChannel})
	require.NoError(t, err)
	require.Equal(t, 2, reversed[firstChannel.Id].MaxConcurrency)
	require.Zero(t, reversed[firstChannel.Id].Active)
	require.Equal(t, commandsAfterFirst, len(hook.Commands()))

	firstChannel.MaxConcurrency = 4
	changedLimit, err := GetChannelConcurrencyLoads(context.Background(), []*model.Channel{firstChannel, secondChannel})
	require.NoError(t, err)
	require.Equal(t, 4, changedLimit[firstChannel.Id].MaxConcurrency)
	require.Greater(t, len(hook.Commands()), commandsAfterFirst)
	commandsAfterLimitChange := len(hook.Commands())

	_, err = GetChannelConcurrencyLoads(context.Background(), []*model.Channel{firstChannel, {Id: 408, MaxConcurrency: 3}})
	require.NoError(t, err)
	require.Greater(t, len(hook.Commands()), commandsAfterLimitChange)
}

func TestChannelConcurrencyLoadSingleflightCoalescesConcurrentMisses(t *testing.T) {
	resetChannelConcurrencyForTest()
	hook := useCountingRedisChannelConcurrencyLoads(t)
	releaseFetch := make(chan struct{})
	hook.blockCommand = "time"
	hook.block = releaseFetch
	hook.started = make(chan struct{})
	useChannelConcurrencyLoadSetting(t, operation_setting.ChannelConcurrencySetting{
		SlotTTLMinutes:     1,
		WaitEnabled:        true,
		WaitTimeoutMS:      5000,
		WaitIntervalMS:     100,
		CooldownEnabled:    true,
		CooldownSeconds:    30,
		LoadCacheEnabled:   true,
		LoadCacheTTLMS:     1000,
		MaxAcquireAttempts: 8,
	})
	channels := []*model.Channel{{Id: 404, MaxConcurrency: 4}}
	start := make(chan struct{})
	errors := make(chan error, 16)
	var waitGroup sync.WaitGroup
	for range 16 {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			<-start
			loads, err := GetChannelConcurrencyLoads(context.Background(), channels)
			if err == nil && len(loads) != 1 {
				err = fmt.Errorf("expected one load, got %d", len(loads))
			}
			errors <- err
		}()
	}
	close(start)
	<-hook.started
	time.Sleep(20 * time.Millisecond)
	close(releaseFetch)
	waitGroup.Wait()
	close(errors)
	for err := range errors {
		require.NoError(t, err)
	}
	require.Equal(t, 1, countRedisLoadCommands(hook.Commands(), "time"))
}

func TestChannelConcurrencyLoadCacheEvictsOldestEntryAtFixedCapacity(t *testing.T) {
	resetChannelConcurrencyForTest()
	hook := useCountingRedisChannelConcurrencyLoads(t)
	useChannelConcurrencyLoadSetting(t, operation_setting.ChannelConcurrencySetting{
		SlotTTLMinutes:     1,
		WaitEnabled:        true,
		WaitTimeoutMS:      5000,
		WaitIntervalMS:     100,
		CooldownEnabled:    true,
		CooldownSeconds:    30,
		LoadCacheEnabled:   true,
		LoadCacheTTLMS:     5000,
		MaxAcquireAttempts: 8,
	})
	oldest := []*model.Channel{{Id: 450, MaxConcurrency: 2}}
	_, err := GetChannelConcurrencyLoads(context.Background(), oldest)
	require.NoError(t, err)

	for offset := range channelConcurrencyLoadCacheMaxEntries {
		channels := []*model.Channel{{Id: 1000 + offset, MaxConcurrency: 2}}
		_, err = GetChannelConcurrencyLoads(context.Background(), channels)
		require.NoError(t, err)
	}
	commandsBeforeOldestRead := len(hook.Commands())

	_, err = GetChannelConcurrencyLoads(context.Background(), oldest)
	require.NoError(t, err)
	require.Greater(t, len(hook.Commands()), commandsBeforeOldestRead)
}

func TestChannelConcurrencyLoadFetchUsesStableBatchesAndOneRedisTime(t *testing.T) {
	resetChannelConcurrencyForTest()
	hook := useCountingRedisChannelConcurrencyLoads(t)
	useChannelConcurrencyLoadSetting(t, operation_setting.ChannelConcurrencySetting{
		SlotTTLMinutes:     1,
		WaitEnabled:        true,
		WaitTimeoutMS:      5000,
		WaitIntervalMS:     100,
		CooldownEnabled:    true,
		CooldownSeconds:    30,
		LoadCacheEnabled:   false,
		LoadCacheTTLMS:     200,
		MaxAcquireAttempts: 8,
	})
	channels := make([]*model.Channel, 0, 125)
	for offset := range 125 {
		channels = append(channels, &model.Channel{Id: 2000 + offset, MaxConcurrency: 2})
	}

	loads, err := GetChannelConcurrencyLoads(context.Background(), channels)
	require.NoError(t, err)
	require.Len(t, loads, 125)
	require.Equal(t, 1, countRedisLoadCommands(hook.Commands(), "time"))
	require.Equal(t, 125, countRedisLoadCommands(hook.Commands(), "zcard"))
	require.Len(t, hook.PipelineSizes(), 3)
	for _, pipelineSize := range hook.PipelineSizes() {
		require.LessOrEqual(t, pipelineSize, 4*channelConcurrencyLoadBatchSize)
	}
}

func TestChannelConcurrencyLoadFetchSlotSaturationFallsBackToMemory(t *testing.T) {
	resetChannelConcurrencyForTest()
	hook := useCountingRedisChannelConcurrencyLoads(t)
	useChannelConcurrencyLoadSetting(t, operation_setting.ChannelConcurrencySetting{
		SlotTTLMinutes:     1,
		WaitEnabled:        true,
		WaitTimeoutMS:      5000,
		WaitIntervalMS:     100,
		CooldownEnabled:    true,
		CooldownSeconds:    30,
		LoadCacheEnabled:   false,
		LoadCacheTTLMS:     200,
		MaxAcquireAttempts: 8,
	})
	previousTimeout := channelConcurrencyLoadFetchTimeout
	channelConcurrencyLoadFetchTimeout = 25 * time.Millisecond
	t.Cleanup(func() {
		channelConcurrencyLoadFetchTimeout = previousTimeout
	})
	for range cap(channelConcurrencyLoadFetchSlots) {
		channelConcurrencyLoadFetchSlots <- struct{}{}
	}
	t.Cleanup(func() {
		for range cap(channelConcurrencyLoadFetchSlots) {
			<-channelConcurrencyLoadFetchSlots
		}
	})

	startedAt := time.Now()
	loads, err := GetChannelConcurrencyLoads(context.Background(), []*model.Channel{{Id: 2600, MaxConcurrency: 3}})
	require.NoError(t, err)
	require.Equal(t, 3, loads[2600].MaxConcurrency)
	require.Zero(t, loads[2600].Active)
	require.GreaterOrEqual(t, time.Since(startedAt), 20*time.Millisecond)
	require.Empty(t, hook.Commands())
}

func TestFreshChannelConcurrencyLoadBypassesAndRefreshesSnapshotCache(t *testing.T) {
	resetChannelConcurrencyForTest()
	hook := useCountingRedisChannelConcurrencyLoads(t)
	useChannelConcurrencyLoadSetting(t, operation_setting.ChannelConcurrencySetting{
		SlotTTLMinutes:     1,
		WaitEnabled:        true,
		WaitTimeoutMS:      5000,
		WaitIntervalMS:     100,
		CooldownEnabled:    true,
		CooldownSeconds:    30,
		LoadCacheEnabled:   true,
		LoadCacheTTLMS:     1000,
		MaxAcquireAttempts: 8,
	})
	channel := &model.Channel{Id: 2700, MaxConcurrency: 3}
	channels := []*model.Channel{channel}

	loads, err := GetChannelConcurrencyLoads(context.Background(), channels)
	require.NoError(t, err)
	require.Zero(t, loads[channel.Id].Active)
	require.NoError(t, common.RDB.ZAdd(context.Background(), channelConcurrencyRedisKey(channel.Id), &redis.Z{
		Score:  float64(time.Now().Add(time.Minute).UnixMilli()),
		Member: "external-slot",
	}).Err())
	commandsBeforeFresh := len(hook.Commands())

	freshLoads, err := GetChannelConcurrencyLoadsFresh(context.Background(), channels)
	require.NoError(t, err)
	require.Equal(t, 1, freshLoads[channel.Id].Active)
	require.Greater(t, len(hook.Commands()), commandsBeforeFresh)
	commandsAfterFresh := len(hook.Commands())

	cachedLoads, err := GetChannelConcurrencyLoads(context.Background(), channels)
	require.NoError(t, err)
	require.Equal(t, freshLoads, cachedLoads)
	require.Equal(t, commandsAfterFresh, len(hook.Commands()))
}

func TestFreshChannelConcurrencyLoadSingleflightCoalescesConcurrentReads(t *testing.T) {
	resetChannelConcurrencyForTest()
	hook := useCountingRedisChannelConcurrencyLoads(t)
	releaseFetch := make(chan struct{})
	hook.blockCommand = "time"
	hook.block = releaseFetch
	hook.started = make(chan struct{})
	useChannelConcurrencyLoadSetting(t, operation_setting.ChannelConcurrencySetting{
		SlotTTLMinutes: 1, WaitEnabled: true, WaitTimeoutMS: 5000, WaitIntervalMS: 100,
		CooldownEnabled: true, CooldownSeconds: 30, LoadCacheEnabled: true, LoadCacheTTLMS: 1000, MaxAcquireAttempts: 8,
	})
	channels := []*model.Channel{{Id: 2750, MaxConcurrency: 4}}
	start := make(chan struct{})
	errors := make(chan error, 16)
	var waitGroup sync.WaitGroup
	for range 16 {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			<-start
			loads, err := GetChannelConcurrencyLoadsFresh(context.Background(), channels)
			if err == nil && len(loads) != 1 {
				err = fmt.Errorf("expected one load, got %d", len(loads))
			}
			errors <- err
		}()
	}
	close(start)
	<-hook.started
	time.Sleep(20 * time.Millisecond)
	close(releaseFetch)
	waitGroup.Wait()
	close(errors)
	for err := range errors {
		require.NoError(t, err)
	}
	require.Equal(t, 1, countRedisLoadCommands(hook.Commands(), "time"))
}

func TestOrdinaryAndFreshChannelConcurrencyLoadsUseSeparateSingleflightKeys(t *testing.T) {
	resetChannelConcurrencyForTest()
	hook := useCountingRedisChannelConcurrencyLoads(t)
	releaseFetch := make(chan struct{})
	hook.blockCommand = "time"
	hook.block = releaseFetch
	hook.started = make(chan struct{})
	useChannelConcurrencyLoadSetting(t, operation_setting.ChannelConcurrencySetting{
		SlotTTLMinutes: 1, WaitEnabled: true, WaitTimeoutMS: 5000, WaitIntervalMS: 100,
		CooldownEnabled: true, CooldownSeconds: 30, LoadCacheEnabled: true, LoadCacheTTLMS: 1000, MaxAcquireAttempts: 8,
	})
	channels := []*model.Channel{{Id: 2751, MaxConcurrency: 4}}
	errors := make(chan error, 2)
	var waitGroup sync.WaitGroup
	waitGroup.Add(2)
	go func() {
		defer waitGroup.Done()
		_, err := GetChannelConcurrencyLoads(context.Background(), channels)
		errors <- err
	}()
	<-hook.started
	go func() {
		defer waitGroup.Done()
		_, err := GetChannelConcurrencyLoadsFresh(context.Background(), channels)
		errors <- err
	}()
	deadline := time.Now().Add(time.Second)
	for countRedisLoadCommands(hook.Commands(), "time") < 2 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	separateFetches := countRedisLoadCommands(hook.Commands(), "time") == 2
	close(releaseFetch)
	waitGroup.Wait()
	close(errors)
	for err := range errors {
		require.NoError(t, err)
	}
	require.True(t, separateFetches)
}

func TestChannelConcurrencyLoadCallerCancellationDoesNotCancelSharedFetch(t *testing.T) {
	resetChannelConcurrencyForTest()
	hook := useCountingRedisChannelConcurrencyLoads(t)
	releaseFetch := make(chan struct{})
	hook.blockCommand = "time"
	hook.block = releaseFetch
	hook.started = make(chan struct{})
	useChannelConcurrencyLoadSetting(t, operation_setting.ChannelConcurrencySetting{
		SlotTTLMinutes: 1, WaitEnabled: true, WaitTimeoutMS: 5000, WaitIntervalMS: 100,
		CooldownEnabled: true, CooldownSeconds: 30, LoadCacheEnabled: true, LoadCacheTTLMS: 1000, MaxAcquireAttempts: 8,
	})
	channels := []*model.Channel{{Id: 2752, MaxConcurrency: 4}}
	canceledContext, cancel := context.WithCancel(context.Background())
	canceledResult := make(chan error, 1)
	sharedResult := make(chan error, 1)
	go func() {
		_, err := GetChannelConcurrencyLoads(canceledContext, channels)
		canceledResult <- err
	}()
	<-hook.started
	go func() {
		loads, err := GetChannelConcurrencyLoads(context.Background(), channels)
		if err == nil && len(loads) != 1 {
			err = fmt.Errorf("expected one load, got %d", len(loads))
		}
		sharedResult <- err
	}()
	time.Sleep(20 * time.Millisecond)
	cancel()
	require.ErrorIs(t, <-canceledResult, context.Canceled)
	require.Equal(t, 1, countRedisLoadCommands(hook.Commands(), "time"))
	close(releaseFetch)
	require.NoError(t, <-sharedResult)
}

func TestChannelConcurrencyLoadFetchTimeoutFallsBackToMemory(t *testing.T) {
	resetChannelConcurrencyForTest()
	hook := useCountingRedisChannelConcurrencyLoads(t)
	neverRelease := make(chan struct{})
	hook.blockCommand = "time"
	hook.block = neverRelease
	hook.started = make(chan struct{})
	useChannelConcurrencyLoadSetting(t, operation_setting.ChannelConcurrencySetting{
		SlotTTLMinutes: 1, WaitEnabled: true, WaitTimeoutMS: 5000, WaitIntervalMS: 100,
		CooldownEnabled: true, CooldownSeconds: 30, LoadCacheEnabled: false, LoadCacheTTLMS: 200, MaxAcquireAttempts: 8,
	})
	previousTimeout := channelConcurrencyLoadFetchTimeout
	channelConcurrencyLoadFetchTimeout = 30 * time.Millisecond
	t.Cleanup(func() {
		channelConcurrencyLoadFetchTimeout = previousTimeout
	})
	startedAt := time.Now()

	loads, err := GetChannelConcurrencyLoads(context.Background(), []*model.Channel{{Id: 2753, MaxConcurrency: 5}})
	require.NoError(t, err)
	require.Equal(t, 5, loads[2753].MaxConcurrency)
	require.Zero(t, loads[2753].Active)
	require.GreaterOrEqual(t, time.Since(startedAt), 25*time.Millisecond)
	require.Less(t, time.Since(startedAt), time.Second)
}

func TestChannelConcurrencyLoadRedisFailureFallsBackToMemory(t *testing.T) {
	resetChannelConcurrencyForTest()
	hook := useCountingRedisChannelConcurrencyLoads(t)
	hook.failCommand = "time"
	hook.failErr = errors.New("redis load unavailable")
	useChannelConcurrencyLoadSetting(t, operation_setting.ChannelConcurrencySetting{
		SlotTTLMinutes: 1, WaitEnabled: true, WaitTimeoutMS: 5000, WaitIntervalMS: 100,
		CooldownEnabled: true, CooldownSeconds: 30, LoadCacheEnabled: false, LoadCacheTTLMS: 200, MaxAcquireAttempts: 8,
	})

	loads, err := GetChannelConcurrencyLoads(context.Background(), []*model.Channel{{Id: 2754, MaxConcurrency: 6}})
	require.NoError(t, err)
	require.Equal(t, 6, loads[2754].MaxConcurrency)
	require.Zero(t, loads[2754].Active)
}

func TestChannelConcurrencyLoadFetchAllowsAtMostTwoConcurrentRedisReads(t *testing.T) {
	resetChannelConcurrencyForTest()
	hook := useCountingRedisChannelConcurrencyLoads(t)
	releaseFetch := make(chan struct{})
	hook.blockCommand = "time"
	hook.block = releaseFetch
	hook.started = make(chan struct{})
	useChannelConcurrencyLoadSetting(t, operation_setting.ChannelConcurrencySetting{
		SlotTTLMinutes: 1, WaitEnabled: true, WaitTimeoutMS: 5000, WaitIntervalMS: 100,
		CooldownEnabled: true, CooldownSeconds: 30, LoadCacheEnabled: true, LoadCacheTTLMS: 1000, MaxAcquireAttempts: 8,
	})
	errors := make(chan error, 3)
	var waitGroup sync.WaitGroup
	for offset := range 3 {
		waitGroup.Add(1)
		go func(channelID int) {
			defer waitGroup.Done()
			_, err := GetChannelConcurrencyLoads(context.Background(), []*model.Channel{{Id: channelID, MaxConcurrency: 2}})
			errors <- err
		}(2760 + offset)
	}
	<-hook.started
	deadline := time.Now().Add(time.Second)
	for countRedisLoadCommands(hook.Commands(), "time") < 2 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	require.Equal(t, 2, countRedisLoadCommands(hook.Commands(), "time"))
	time.Sleep(20 * time.Millisecond)
	require.Equal(t, 2, countRedisLoadCommands(hook.Commands(), "time"))
	close(releaseFetch)
	waitGroup.Wait()
	close(errors)
	for err := range errors {
		require.NoError(t, err)
	}
	require.Equal(t, 3, countRedisLoadCommands(hook.Commands(), "time"))
}

func TestChannelCandidateOrderingFreshlyConfirmsAllCachedCooldowns(t *testing.T) {
	resetChannelConcurrencyForTest()
	hook := useCountingRedisChannelConcurrencyLoads(t)
	useChannelConcurrencyLoadSetting(t, operation_setting.ChannelConcurrencySetting{
		SlotTTLMinutes:     1,
		WaitEnabled:        true,
		WaitTimeoutMS:      5000,
		WaitIntervalMS:     100,
		CooldownEnabled:    true,
		CooldownSeconds:    30,
		LoadCacheEnabled:   true,
		LoadCacheTTLMS:     5000,
		MaxAcquireAttempts: 8,
	})
	channel := &model.Channel{Id: 2800, MaxConcurrency: 2}
	candidates := []*model.Channel{channel}
	require.NoError(t, MarkChannelConcurrencyCooldown(context.Background(), channel.Id, time.Minute, "test"))

	ordered, err := orderChannelCandidatesByConcurrencyLoad(context.Background(), candidates, "default")
	require.NoError(t, err)
	require.Empty(t, ordered)
	require.NoError(t, common.RDB.Del(context.Background(), channelConcurrencyCooldownRedisKey(channel.Id)).Err())
	timeReadsBeforeRecovery := countRedisLoadCommands(hook.Commands(), "time")

	ordered, err = orderChannelCandidatesByConcurrencyLoad(context.Background(), candidates, "default")
	require.NoError(t, err)
	require.Equal(t, candidates, ordered)
	require.Equal(t, timeReadsBeforeRecovery+1, countRedisLoadCommands(hook.Commands(), "time"))
}

func TestChannelCandidateOrderingKeepsCachedCooldownsWhenFreshConfirmationFails(t *testing.T) {
	resetChannelConcurrencyForTest()
	hook := useCountingRedisChannelConcurrencyLoads(t)
	useChannelConcurrencyLoadSetting(t, operation_setting.ChannelConcurrencySetting{
		SlotTTLMinutes: 1, WaitEnabled: true, WaitTimeoutMS: 5000, WaitIntervalMS: 100,
		CooldownEnabled: true, CooldownSeconds: 30, LoadCacheEnabled: true, LoadCacheTTLMS: 5000, MaxAcquireAttempts: 8,
	})
	channel := &model.Channel{Id: 2801, MaxConcurrency: 2}
	candidates := []*model.Channel{channel}
	require.NoError(t, MarkChannelConcurrencyCooldown(context.Background(), channel.Id, time.Minute, "test"))
	ordered, err := orderChannelCandidatesByConcurrencyLoad(context.Background(), candidates, "default")
	require.NoError(t, err)
	require.Empty(t, ordered)
	require.NoError(t, common.RDB.Del(context.Background(), channelConcurrencyCooldownRedisKey(channel.Id)).Err())
	hook.failCommand = "time"
	hook.failErr = errors.New("redis load unavailable")

	ordered, err = orderChannelCandidatesByConcurrencyLoad(context.Background(), candidates, "default")
	require.NoError(t, err)
	require.Empty(t, ordered)
}

func TestChannelCandidateOrderingUsesGroupWeightForEqualLoad(t *testing.T) {
	globalUnlimitedWeight := uint(100)
	globalLimitedWeight := uint(0)
	unlimited := &model.Channel{
		Id:            2802,
		Weight:        &globalUnlimitedWeight,
		GroupPriority: common.GetPointer(`{"vip":{"weight":0}}`),
	}
	limited := &model.Channel{
		Id:             2803,
		Weight:         &globalLimitedWeight,
		GroupPriority:  common.GetPointer(`{"vip":{"weight":100}}`),
		MaxConcurrency: 1,
	}
	ordered, _, err := orderChannelCandidatesWithLoads(
		[]*model.Channel{unlimited, limited},
		map[int]ChannelConcurrencyLoad{
			unlimited.Id: {ChannelID: unlimited.Id},
			limited.Id:   {ChannelID: limited.Id, MaxConcurrency: 1},
		},
		"vip",
	)

	require.NoError(t, err)
	require.Equal(t, []int{limited.Id, unlimited.Id}, []int{ordered[0].Id, ordered[1].Id})
}

func TestChannelSelectionFallsBackToLowerPriorityBeforeWaiting(t *testing.T) {
	resetChannelConcurrencyForTest()
	useCountingRedisChannelConcurrencyLoads(t)
	database := useChannelConcurrencySelectionDatabase(t)
	useChannelConcurrencyLoadSetting(t, operation_setting.ChannelConcurrencySetting{
		SlotTTLMinutes:     1,
		WaitEnabled:        false,
		WaitTimeoutMS:      5000,
		WaitIntervalMS:     100,
		CooldownEnabled:    false,
		CooldownSeconds:    30,
		LoadCacheEnabled:   false,
		LoadCacheTTLMS:     200,
		MaxAcquireAttempts: 8,
	})
	highPriority := int64(10)
	lowPriority := int64(0)
	weight := uint(100)
	high := &model.Channel{
		Id: 2810, Type: constant.ChannelTypeOpenAI, Key: "key-high", Status: common.ChannelStatusEnabled,
		Name: "high-full", Weight: &weight, Priority: &highPriority, Models: "gpt-priority-fallback", Group: "default", MaxConcurrency: 1,
	}
	low := &model.Channel{
		Id: 2811, Type: constant.ChannelTypeOpenAI, Key: "key-low", Status: common.ChannelStatusEnabled,
		Name: "low-unlimited", Weight: &weight, Priority: &lowPriority, Models: "gpt-priority-fallback", Group: "default",
	}
	require.NoError(t, database.Create(&[]*model.Channel{high, low}).Error)
	require.NoError(t, database.Create(&[]model.Ability{
		{Group: "default", Model: "gpt-priority-fallback", ChannelId: high.Id, Enabled: true, Priority: &highPriority, Weight: weight},
		{Group: "default", Model: "gpt-priority-fallback", ChannelId: low.Id, Enabled: true, Priority: &lowPriority, Weight: weight},
	}).Error)
	model.InitChannelCache()
	heldLease, acquired, err := TryAcquireChannelConcurrency(context.Background(), high)
	require.NoError(t, err)
	require.True(t, acquired)
	t.Cleanup(func() { require.NoError(t, ReleaseChannelConcurrency(context.Background(), heldLease)) })

	gin.SetMode(gin.TestMode)
	requestContext, _ := gin.CreateTestContext(httptest.NewRecorder())
	requestContext.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
	retry := 0
	param := &RetryParam{
		Ctx: requestContext, TokenGroup: "default", ModelName: "gpt-priority-fallback", Retry: &retry,
	}
	selected, _, err := CacheGetRandomSatisfiedChannel(param)

	require.NoError(t, err)
	require.NotNil(t, selected)
	require.Equal(t, low.Id, selected.Id)
	require.Equal(t, 1, param.GetRetry())
}

func TestChannelSelectionSharesAcquireBudgetThenWaitsForBestCandidate(t *testing.T) {
	resetChannelConcurrencyForTest()
	hook := useCountingRedisChannelConcurrencyLoads(t)
	database := useChannelConcurrencySelectionDatabase(t)
	useChannelConcurrencyLoadSetting(t, operation_setting.ChannelConcurrencySetting{
		SlotTTLMinutes:       1,
		WaitEnabled:          true,
		WaitTimeoutMS:        500,
		WaitIntervalMS:       100,
		MaxWaitingPerChannel: 4,
		CooldownEnabled:      false,
		CooldownSeconds:      30,
		LoadCacheEnabled:     false,
		LoadCacheTTLMS:       200,
		MaxAcquireAttempts:   2,
	})
	priority := int64(0)
	weight := uint(100)
	channels := make([]*model.Channel, 0, 4)
	heldLeases := make([]*ChannelConcurrencyLease, 0, 4)
	for offset := range 4 {
		channel := &model.Channel{
			Id:             2900 + offset,
			Type:           constant.ChannelTypeOpenAI,
			Key:            fmt.Sprintf("key-%d", offset),
			Status:         common.ChannelStatusEnabled,
			Name:           fmt.Sprintf("budget-%d", offset),
			Weight:         &weight,
			Priority:       &priority,
			Models:         "gpt-budget",
			Group:          "default",
			MaxConcurrency: 1,
		}
		require.NoError(t, database.Create(channel).Error)
		require.NoError(t, database.Create(&model.Ability{
			Group: "default", Model: "gpt-budget", ChannelId: channel.Id, Enabled: true, Priority: &priority, Weight: weight,
		}).Error)
		lease, acquired, err := TryAcquireChannelConcurrency(context.Background(), channel)
		require.NoError(t, err)
		require.True(t, acquired)
		channels = append(channels, channel)
		heldLeases = append(heldLeases, lease)
	}
	model.InitChannelCache()
	commandsBeforeSelection := len(hook.Commands())
	gin.SetMode(gin.TestMode)
	requestContext, _ := gin.CreateTestContext(httptest.NewRecorder())
	requestContext.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
	retry := 0
	go func() {
		time.Sleep(20 * time.Millisecond)
		for _, lease := range heldLeases {
			_ = ReleaseChannelConcurrency(context.Background(), lease)
		}
	}()

	selected, _, err := CacheGetRandomSatisfiedChannel(&RetryParam{
		Ctx: requestContext, TokenGroup: "default", ModelName: "gpt-budget", Retry: &retry,
	})
	require.NoError(t, err)
	require.NotNil(t, selected)
	require.NotNil(t, getChannelConcurrencyLeaseForContext(requestContext))
	selectionCommands := hook.Commands()[commandsBeforeSelection:]
	authoritativeAttempts := countRedisLoadCommands(selectionCommands, "eval") + countRedisLoadCommands(selectionCommands, "evalsha")
	require.LessOrEqual(t, authoritativeAttempts, 6)
	require.NoError(t, ReleaseChannelConcurrencyForContext(requestContext))
}

func TestStaleChannelConcurrencyLoadNeverBypassesAuthoritativeSlotAcquire(t *testing.T) {
	resetChannelConcurrencyForTest()
	useCountingRedisChannelConcurrencyLoads(t)
	database := useChannelConcurrencySelectionDatabase(t)
	useChannelConcurrencyLoadSetting(t, operation_setting.ChannelConcurrencySetting{
		SlotTTLMinutes:       1,
		WaitEnabled:          false,
		WaitTimeoutMS:        5000,
		WaitIntervalMS:       100,
		MaxWaitingPerChannel: 1,
		CooldownEnabled:      false,
		CooldownSeconds:      30,
		LoadCacheEnabled:     true,
		LoadCacheTTLMS:       5000,
		MaxAcquireAttempts:   8,
	})
	priority := int64(0)
	weight := uint(100)
	channel := &model.Channel{
		Id: 2950, Type: constant.ChannelTypeOpenAI, Key: "key-stale", Status: common.ChannelStatusEnabled,
		Name: "stale-load", Weight: &weight, Priority: &priority, Models: "gpt-stale", Group: "default", MaxConcurrency: 1,
	}
	require.NoError(t, database.Create(channel).Error)
	require.NoError(t, database.Create(&model.Ability{
		Group: "default", Model: "gpt-stale", ChannelId: channel.Id, Enabled: true, Priority: &priority, Weight: weight,
	}).Error)
	model.InitChannelCache()
	loads, err := GetChannelConcurrencyLoads(context.Background(), []*model.Channel{channel})
	require.NoError(t, err)
	require.Zero(t, loads[channel.Id].Active)
	heldLease, acquired, err := TryAcquireChannelConcurrency(context.Background(), channel)
	require.NoError(t, err)
	require.True(t, acquired)
	t.Cleanup(func() {
		require.NoError(t, ReleaseChannelConcurrency(context.Background(), heldLease))
	})
	gin.SetMode(gin.TestMode)
	requestContext, _ := gin.CreateTestContext(httptest.NewRecorder())
	requestContext.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
	retry := 0

	selected, _, err := CacheGetRandomSatisfiedChannel(&RetryParam{
		Ctx: requestContext, TokenGroup: "default", ModelName: "gpt-stale", Retry: &retry,
	})
	require.ErrorIs(t, err, ErrChannelConcurrencyLimit)
	require.Nil(t, selected)
	require.Nil(t, getChannelConcurrencyLeaseForContext(requestContext))
	require.Equal(t, int64(1), common.RDB.ZCard(context.Background(), channelConcurrencyRedisKey(channel.Id)).Val())
}

func TestAutoGroupChannelSelectionUsesConcurrencyAcquireBudget(t *testing.T) {
	resetChannelConcurrencyForTest()
	useCountingRedisChannelConcurrencyLoads(t)
	database := useChannelConcurrencySelectionDatabase(t)
	useChannelConcurrencyLoadSetting(t, operation_setting.ChannelConcurrencySetting{
		SlotTTLMinutes:       1,
		WaitEnabled:          true,
		WaitTimeoutMS:        500,
		WaitIntervalMS:       100,
		MaxWaitingPerChannel: 1,
		CooldownEnabled:      false,
		CooldownSeconds:      30,
		LoadCacheEnabled:     false,
		LoadCacheTTLMS:       200,
		MaxAcquireAttempts:   1,
	})
	priority := int64(0)
	weight := uint(100)
	channel := &model.Channel{
		Id: 3000, Type: constant.ChannelTypeOpenAI, Key: "key-auto", Status: common.ChannelStatusEnabled,
		Name: "auto-budget", Weight: &weight, Priority: &priority, Models: "gpt-auto-budget", Group: "vip", MaxConcurrency: 1,
	}
	require.NoError(t, database.Create(channel).Error)
	require.NoError(t, database.Create(&model.Ability{
		Group: "vip", Model: "gpt-auto-budget", ChannelId: channel.Id, Enabled: true, Priority: &priority, Weight: weight,
	}).Error)
	model.InitChannelCache()
	heldLease, acquired, err := TryAcquireChannelConcurrency(context.Background(), channel)
	require.NoError(t, err)
	require.True(t, acquired)
	gin.SetMode(gin.TestMode)
	requestContext, _ := gin.CreateTestContext(httptest.NewRecorder())
	requestContext.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
	common.SetContextKey(requestContext, constant.ContextKeyUserGroup, "default")
	common.SetContextKey(requestContext, constant.ContextKeyTokenAutoGroups, []string{"vip"})
	retry := 0
	go func() {
		time.Sleep(20 * time.Millisecond)
		_ = ReleaseChannelConcurrency(context.Background(), heldLease)
	}()

	selected, selectedGroup, err := CacheGetRandomSatisfiedChannel(&RetryParam{
		Ctx: requestContext, TokenGroup: "auto", ModelName: "gpt-auto-budget", Retry: &retry,
	})
	require.NoError(t, err)
	require.NotNil(t, selected)
	require.Equal(t, "vip", selectedGroup)
	require.NotNil(t, getChannelConcurrencyLeaseForContext(requestContext))
	require.NoError(t, ReleaseChannelConcurrencyForContext(requestContext))
}

func TestAutoGroupSelectionResetsAcquireBudgetForEachGroup(t *testing.T) {
	resetChannelConcurrencyForTest()
	useCountingRedisChannelConcurrencyLoads(t)
	database := useChannelConcurrencySelectionDatabase(t)
	useChannelConcurrencyLoadSetting(t, operation_setting.ChannelConcurrencySetting{
		SlotTTLMinutes:       1,
		WaitEnabled:          false,
		WaitTimeoutMS:        500,
		WaitIntervalMS:       100,
		MaxWaitingPerChannel: 1,
		CooldownEnabled:      false,
		CooldownSeconds:      30,
		LoadCacheEnabled:     true,
		LoadCacheTTLMS:       5000,
		MaxAcquireAttempts:   2,
	})
	priority := int64(0)
	preferredWeight := uint(100)
	fallbackWeight := uint(0)
	firstGroupFull := &model.Channel{
		Id: 3010, Type: constant.ChannelTypeOpenAI, Key: "key-first-full", Status: common.ChannelStatusEnabled,
		Name: "first-group-full", Weight: &preferredWeight, Priority: &priority, Models: "gpt-auto-budget-reset", Group: "vip", MaxConcurrency: 1,
	}
	secondGroupFull := &model.Channel{
		Id: 3011, Type: constant.ChannelTypeOpenAI, Key: "key-second-full", Status: common.ChannelStatusEnabled,
		Name: "second-group-full", Weight: &preferredWeight, Priority: &priority, Models: "gpt-auto-budget-reset", Group: "default", MaxConcurrency: 1,
	}
	secondGroupFree := &model.Channel{
		Id: 3012, Type: constant.ChannelTypeOpenAI, Key: "key-second-free", Status: common.ChannelStatusEnabled,
		Name: "second-group-free", Weight: &fallbackWeight, Priority: &priority, Models: "gpt-auto-budget-reset", Group: "default", MaxConcurrency: 1,
	}
	require.NoError(t, database.Create(&[]*model.Channel{firstGroupFull, secondGroupFull, secondGroupFree}).Error)
	require.NoError(t, database.Create(&[]model.Ability{
		{Group: "vip", Model: "gpt-auto-budget-reset", ChannelId: firstGroupFull.Id, Enabled: true, Priority: &priority, Weight: preferredWeight},
		{Group: "default", Model: "gpt-auto-budget-reset", ChannelId: secondGroupFull.Id, Enabled: true, Priority: &priority, Weight: preferredWeight},
		{Group: "default", Model: "gpt-auto-budget-reset", ChannelId: secondGroupFree.Id, Enabled: true, Priority: &priority, Weight: fallbackWeight},
	}).Error)
	model.InitChannelCache()
	_, err := GetChannelConcurrencyLoads(context.Background(), []*model.Channel{secondGroupFull, secondGroupFree})
	require.NoError(t, err)
	firstGroupLease, acquired, err := TryAcquireChannelConcurrency(context.Background(), firstGroupFull)
	require.NoError(t, err)
	require.True(t, acquired)
	secondGroupLease, acquired, err := TryAcquireChannelConcurrency(context.Background(), secondGroupFull)
	require.NoError(t, err)
	require.True(t, acquired)
	t.Cleanup(func() {
		require.NoError(t, ReleaseChannelConcurrency(context.Background(), firstGroupLease))
		require.NoError(t, ReleaseChannelConcurrency(context.Background(), secondGroupLease))
	})

	gin.SetMode(gin.TestMode)
	requestContext, _ := gin.CreateTestContext(httptest.NewRecorder())
	requestContext.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
	common.SetContextKey(requestContext, constant.ContextKeyUserGroup, "default")
	common.SetContextKey(requestContext, constant.ContextKeyTokenAutoGroups, []string{"vip", "default"})
	retry := 0
	selected, selectedGroup, err := CacheGetRandomSatisfiedChannel(&RetryParam{
		Ctx: requestContext, TokenGroup: "auto", ModelName: "gpt-auto-budget-reset", Retry: &retry,
	})

	require.NoError(t, err)
	require.NotNil(t, selected)
	require.Equal(t, secondGroupFree.Id, selected.Id)
	require.Equal(t, "default", selectedGroup)
	require.NoError(t, ReleaseChannelConcurrencyForContext(requestContext))
}
