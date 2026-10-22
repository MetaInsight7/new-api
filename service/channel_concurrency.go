package service

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"golang.org/x/sync/singleflight"
)

var channelConcurrencyAcquireScriptSrc = `
local key = KEYS[1]
local cooldown_key = KEYS[2]
local ttl = tonumber(ARGV[1])
local max = tonumber(ARGV[2])
local token = ARGV[3]

if ARGV[4] == '1' and redis.call('EXISTS', cooldown_key) == 1 then
	return -1
end

local redis_time = redis.call('TIME')
local now = tonumber(redis_time[1]) * 1000 + math.floor(tonumber(redis_time[2]) / 1000)
redis.call('ZREMRANGEBYSCORE', key, '-inf', now)

if redis.call('ZSCORE', key, token) then
	redis.call('ZADD', key, now + ttl, token)
	redis.call('PEXPIRE', key, ttl)
	return 1
end

local count = redis.call('ZCARD', key)
if count >= max then
	redis.call('PEXPIRE', key, ttl)
	return 0
end

redis.call('ZADD', key, now + ttl, token)
redis.call('PEXPIRE', key, ttl)
return 1
`

var channelConcurrencyAcquireScript = redis.NewScript(channelConcurrencyAcquireScriptSrc)

var channelConcurrencyRenewScriptSrc = `
local key = KEYS[1]
local ttl = tonumber(ARGV[1])
local token = ARGV[2]

if not redis.call('ZSCORE', key, token) then
	return 0
end

local redis_time = redis.call('TIME')
local now = tonumber(redis_time[1]) * 1000 + math.floor(tonumber(redis_time[2]) / 1000)
redis.call('ZADD', key, now + ttl, token)
redis.call('PEXPIRE', key, ttl)
return 1
`

var channelConcurrencyRenewScript = redis.NewScript(channelConcurrencyRenewScriptSrc)

const channelConcurrencyUncertainAcquireCleanupTimeout = 500 * time.Millisecond

var channelConcurrencyWaitRegisterScriptSrc = `
local key = KEYS[1]
local max_waiting = tonumber(ARGV[1])
local ttl = tonumber(ARGV[2])

local waiting = tonumber(redis.call('GET', key) or '0')
if waiting >= max_waiting then
	return 0
end

redis.call('INCR', key)
redis.call('EXPIRE', key, ttl)
return 1
`

var channelConcurrencyWaitRegisterScript = redis.NewScript(channelConcurrencyWaitRegisterScriptSrc)

type ChannelConcurrencyLease struct {
	ChannelID int

	token       string
	useRedis    bool
	renewCancel context.CancelFunc
	released    atomic.Bool
}

type ChannelConcurrencyLoad struct {
	ChannelID      int
	MaxConcurrency int
	Active         int
	Waiting        int
	CoolingDown    bool
	LoadRate       float64
}

type channelConcurrencyWaitingLease struct {
	channelID int
	useRedis  bool
	released  atomic.Bool
}

type cachedChannelConcurrencyLoads struct {
	loads     map[int]ChannelConcurrencyLoad
	expiresAt time.Time
	storedAt  time.Time
}

const (
	channelConcurrencyLoadBatchSize       = 50
	channelConcurrencyLoadCacheMaxEntries = 256
)

var (
	channelConcurrencyLoadFetchTimeout = 3 * time.Second
	channelConcurrencyLoadFetchSlots   = make(chan struct{}, 2)
)

var (
	channelConcurrencyMemoryMu        sync.Mutex
	channelConcurrencyMemorySlots     = make(map[int]map[string]time.Time)
	channelConcurrencyMemoryWaits     = make(map[int]int)
	channelConcurrencyMemoryCooldowns = make(map[int]time.Time)
	channelConcurrencyRequestPrefix   = common.GetUUID()
	channelConcurrencyLoadCacheMu     sync.RWMutex
	channelConcurrencyLoadCache       = make(map[string]cachedChannelConcurrencyLoads)
	channelConcurrencyLoadGroup       singleflight.Group
	channelConcurrencyRenewInterval   = func(ttl time.Duration) time.Duration {
		interval := ttl / 3
		if interval < time.Second {
			return time.Second
		}
		return interval
	}
)

func TryAcquireChannelConcurrency(ctx context.Context, channel *model.Channel) (*ChannelConcurrencyLease, bool, error) {
	return tryAcquireChannelConcurrencyWithToken(ctx, channel, newChannelConcurrencyToken())
}

func AcquireChannelConcurrencyWithWait(ctx context.Context, channel *model.Channel) (*ChannelConcurrencyLease, bool, error) {
	lease, ok, err := TryAcquireChannelConcurrency(ctx, channel)
	if err != nil || ok {
		return lease, ok, err
	}
	if channel == nil {
		return nil, false, fmt.Errorf("channel is nil")
	}
	maxConcurrency := channel.GetMaxConcurrency()
	if maxConcurrency <= 0 {
		return nil, true, nil
	}
	if !operation_setting.IsChannelConcurrencyWaitEnabled() {
		return nil, false, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	maxWaiting := operation_setting.GetChannelConcurrencyMaxWaiting(maxConcurrency)
	waitingLease, registered, err := acquireChannelConcurrencyWaiting(ctx, channel.Id, maxWaiting)
	if err != nil {
		return nil, false, err
	}
	if !registered {
		return nil, false, ErrChannelConcurrencyLimit
	}
	defer func() {
		releaseChannelConcurrencyWaitingLeaseWithLog(waitingLease, channel.Id)
	}()

	waitCtx, cancel := context.WithTimeout(ctx, operation_setting.GetChannelConcurrencyWaitTimeout())
	defer cancel()

	interval := operation_setting.GetChannelConcurrencyWaitInterval()
	maxInterval := interval * 8
	if maxInterval > time.Second {
		maxInterval = time.Second
	}
	if maxInterval < interval {
		maxInterval = interval
	}
	timer := time.NewTimer(withChannelConcurrencyJitter(interval))
	defer timer.Stop()

	for {
		select {
		case <-waitCtx.Done():
			return nil, false, ErrChannelConcurrencyLimit
		case <-timer.C:
			lease, ok, err = TryAcquireChannelConcurrency(waitCtx, channel)
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil, false, ErrChannelConcurrencyLimit
			}
			if err != nil || ok {
				return lease, ok, err
			}
			interval *= 2
			if interval > maxInterval {
				interval = maxInterval
			}
			timer.Reset(withChannelConcurrencyJitter(interval))
		}
	}
}

func withChannelConcurrencyJitter(interval time.Duration) time.Duration {
	if interval <= 1 {
		return interval
	}
	half := interval / 2
	return half + time.Duration(rand.Int63n(int64(interval-half)))
}

func tryAcquireChannelConcurrencyWithToken(ctx context.Context, channel *model.Channel, token string) (*ChannelConcurrencyLease, bool, error) {
	if channel == nil {
		return nil, false, fmt.Errorf("channel is nil")
	}

	if ctx == nil {
		ctx = context.Background()
	}

	maxConcurrency := channel.GetMaxConcurrency()
	if maxConcurrency <= 0 {
		return nil, true, nil
	}

	lease := &ChannelConcurrencyLease{
		ChannelID: channel.Id,
		token:     token,
		useRedis:  common.RedisEnabled && common.RDB != nil,
	}
	cooldownEnabled := operation_setting.GetChannelConcurrencyCooldownPolicy().Enabled

	if lease.useRedis {
		ok, err := acquireRedisChannelConcurrency(ctx, channel.Id, maxConcurrency, token, cooldownEnabled)
		if err != nil {
			return nil, false, fmt.Errorf("acquire channel concurrency in redis failed for channel %d: %w", channel.Id, err)
		} else if !ok {
			return nil, false, nil
		} else {
			startChannelConcurrencyLeaseRenewal(lease)
			return lease, true, nil
		}
	}

	if !acquireMemoryChannelConcurrency(channel.Id, maxConcurrency, token, cooldownEnabled) {
		return nil, false, nil
	}
	startChannelConcurrencyLeaseRenewal(lease)
	return lease, true, nil
}

func GetChannelConcurrencyLoads(ctx context.Context, channels []*model.Channel) (map[int]ChannelConcurrencyLoad, error) {
	return getChannelConcurrencyLoads(ctx, channels, true, true)
}

func GetChannelConcurrencyLoadsFresh(ctx context.Context, channels []*model.Channel) (map[int]ChannelConcurrencyLoad, error) {
	return getChannelConcurrencyLoads(ctx, channels, false, true)
}

// GetChannelConcurrencyLoadsAuthoritative is reserved for accuracy-sensitive
// management views. It bypasses the snapshot cache and never falls back to one
// process's memory when Redis is the shared source of truth.
func GetChannelConcurrencyLoadsAuthoritative(ctx context.Context, channels []*model.Channel) (map[int]ChannelConcurrencyLoad, error) {
	return getChannelConcurrencyLoads(ctx, channels, false, false)
}

func getChannelConcurrencyLoadsFreshForConfirmation(ctx context.Context, channels []*model.Channel) (map[int]ChannelConcurrencyLoad, error) {
	return getChannelConcurrencyLoads(ctx, channels, false, false)
}

func getChannelConcurrencyLoads(ctx context.Context, channels []*model.Channel, allowCache bool, fallbackToMemory bool) (map[int]ChannelConcurrencyLoad, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	loads := make(map[int]ChannelConcurrencyLoad, len(channels))
	if len(channels) == 0 {
		return loads, nil
	}
	for _, channel := range channels {
		if channel == nil {
			continue
		}
		loads[channel.Id] = ChannelConcurrencyLoad{
			ChannelID:      channel.Id,
			MaxConcurrency: channel.GetMaxConcurrency(),
		}
	}

	bounded := boundedChannelConcurrencyLoads(loads)
	if len(bounded) == 0 {
		return loads, nil
	}
	if common.RedisEnabled && common.RDB != nil {
		redisLoads, err := getCachedRedisChannelConcurrencyLoads(ctx, bounded, allowCache)
		if err == nil {
			for channelID, load := range redisLoads {
				loads[channelID] = load
			}
			return loads, nil
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if !fallbackToMemory {
			return nil, err
		}
		common.SysError(fmt.Sprintf("get channel concurrency loads from redis failed, fallback to memory: %s", err.Error()))
	}

	return getMemoryChannelConcurrencyLoads(loads), nil
}

func getCachedRedisChannelConcurrencyLoads(ctx context.Context, bounded map[int]ChannelConcurrencyLoad, allowCache bool) (map[int]ChannelConcurrencyLoad, error) {
	ttl := operation_setting.GetChannelConcurrencyLoadCacheTTL()
	if ttl <= 0 {
		return fetchRedisChannelConcurrencyLoads(bounded)
	}
	key := channelConcurrencyLoadCacheKey(bounded)
	groupKey := key
	if allowCache {
		if cached, ok := lookupChannelConcurrencyLoadCache(key, time.Now(), ttl); ok {
			return cached, nil
		}
	} else {
		groupKey = "fresh:" + key
	}
	resultChannel := channelConcurrencyLoadGroup.DoChan(groupKey, func() (any, error) {
		now := time.Now()
		if allowCache {
			if cached, ok := lookupChannelConcurrencyLoadCache(key, now, ttl); ok {
				return cached, nil
			}
		}
		loads, err := fetchRedisChannelConcurrencyLoads(bounded)
		if err != nil {
			return nil, err
		}
		storeChannelConcurrencyLoadCache(key, loads, now.Add(ttl))
		return loads, nil
	})
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-resultChannel:
		if result.Err != nil {
			return nil, result.Err
		}
		loads, _ := result.Val.(map[int]ChannelConcurrencyLoad)
		return cloneChannelConcurrencyLoads(loads), nil
	}
}

func fetchRedisChannelConcurrencyLoads(bounded map[int]ChannelConcurrencyLoad) (map[int]ChannelConcurrencyLoad, error) {
	fetchContext, cancel := context.WithTimeout(context.Background(), channelConcurrencyLoadFetchTimeout)
	defer cancel()
	select {
	case channelConcurrencyLoadFetchSlots <- struct{}{}:
		defer func() { <-channelConcurrencyLoadFetchSlots }()
	case <-fetchContext.Done():
		return nil, fmt.Errorf("channel concurrency load fetch slots saturated: %w", fetchContext.Err())
	}
	return getRedisChannelConcurrencyLoads(fetchContext, bounded)
}

func channelConcurrencyLoadCacheKey(bounded map[int]ChannelConcurrencyLoad) string {
	channelIDs := make([]int, 0, len(bounded))
	for channelID := range bounded {
		channelIDs = append(channelIDs, channelID)
	}
	sort.Ints(channelIDs)
	var builder strings.Builder
	for _, channelID := range channelIDs {
		builder.WriteString(strconv.Itoa(channelID))
		builder.WriteByte(':')
		builder.WriteString(strconv.Itoa(bounded[channelID].MaxConcurrency))
		builder.WriteByte(',')
	}
	return builder.String()
}

func lookupChannelConcurrencyLoadCache(key string, now time.Time, ttl time.Duration) (map[int]ChannelConcurrencyLoad, bool) {
	channelConcurrencyLoadCacheMu.RLock()
	cached, ok := channelConcurrencyLoadCache[key]
	channelConcurrencyLoadCacheMu.RUnlock()
	if !ok || !now.Before(cached.expiresAt) || !now.Before(cached.storedAt.Add(ttl)) {
		return nil, false
	}
	return cloneChannelConcurrencyLoads(cached.loads), true
}

func storeChannelConcurrencyLoadCache(key string, loads map[int]ChannelConcurrencyLoad, expiresAt time.Time) {
	channelConcurrencyLoadCacheMu.Lock()
	defer channelConcurrencyLoadCacheMu.Unlock()
	now := time.Now()
	for cacheKey, cached := range channelConcurrencyLoadCache {
		if !now.Before(cached.expiresAt) {
			delete(channelConcurrencyLoadCache, cacheKey)
		}
	}
	if _, exists := channelConcurrencyLoadCache[key]; !exists && len(channelConcurrencyLoadCache) >= channelConcurrencyLoadCacheMaxEntries {
		oldestKey := ""
		var oldestTime time.Time
		for cacheKey, cached := range channelConcurrencyLoadCache {
			if oldestKey == "" || cached.storedAt.Before(oldestTime) || (cached.storedAt.Equal(oldestTime) && cacheKey < oldestKey) {
				oldestKey = cacheKey
				oldestTime = cached.storedAt
			}
		}
		delete(channelConcurrencyLoadCache, oldestKey)
	}
	channelConcurrencyLoadCache[key] = cachedChannelConcurrencyLoads{
		loads:     cloneChannelConcurrencyLoads(loads),
		expiresAt: expiresAt,
		storedAt:  now,
	}
}

func cloneChannelConcurrencyLoads(loads map[int]ChannelConcurrencyLoad) map[int]ChannelConcurrencyLoad {
	cloned := make(map[int]ChannelConcurrencyLoad, len(loads))
	for channelID, load := range loads {
		cloned[channelID] = load
	}
	return cloned
}

func MarkChannelConcurrencyCooldown(ctx context.Context, channelID int, duration time.Duration, reason string) error {
	policy := operation_setting.GetChannelConcurrencyCooldownPolicy()
	if !policy.Enabled {
		return nil
	}
	if duration <= 0 {
		duration = policy.Duration
	}
	return markChannelConcurrencyCooldown(ctx, channelID, duration, reason)
}

func markChannelConcurrencyCooldown(ctx context.Context, channelID int, duration time.Duration, reason string) error {
	if channelID <= 0 {
		return fmt.Errorf("channel id is invalid")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	if common.RedisEnabled && common.RDB != nil {
		if err := common.RDB.Set(ctx, channelConcurrencyCooldownRedisKey(channelID), reason, duration).Err(); err == nil {
			return nil
		} else {
			common.SysError(fmt.Sprintf("mark channel concurrency cooldown in redis failed, fallback to memory: channel_id=%d, error=%s", channelID, err.Error()))
		}
	}

	channelConcurrencyMemoryMu.Lock()
	defer channelConcurrencyMemoryMu.Unlock()
	channelConcurrencyMemoryCooldowns[channelID] = time.Now().Add(duration)
	return nil
}

func ReleaseChannelConcurrency(ctx context.Context, lease *ChannelConcurrencyLease) error {
	if lease == nil {
		return nil
	}
	if !lease.released.CompareAndSwap(false, true) {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	if lease.useRedis {
		if common.RDB == nil {
			lease.released.Store(false)
			if lease.renewCancel != nil {
				lease.renewCancel()
			}
			return fmt.Errorf("release channel concurrency in redis failed for channel %d: redis client is nil", lease.ChannelID)
		}
		releaseCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		err := common.RDB.ZRem(releaseCtx, channelConcurrencyRedisKey(lease.ChannelID), lease.token).Err()
		if err != nil {
			lease.released.Store(false)
			if lease.renewCancel != nil {
				lease.renewCancel()
			}
			return err
		}
		if lease.renewCancel != nil {
			lease.renewCancel()
		}
		return nil
	}

	releaseMemoryChannelConcurrency(lease.ChannelID, lease.token)
	if lease.renewCancel != nil {
		lease.renewCancel()
	}
	return nil
}

func EnsureChannelConcurrencyForContext(c *gin.Context, channel *model.Channel) (bool, error) {
	if c == nil {
		return false, fmt.Errorf("gin context is nil")
	}
	if channel == nil {
		return false, fmt.Errorf("channel is nil")
	}
	if lease := getChannelConcurrencyLeaseForContext(c); lease != nil {
		if lease.ChannelID == channel.Id {
			return true, nil
		}
		if err := ReleaseChannelConcurrencyForContext(c); err != nil {
			return false, err
		}
	}
	return AcquireChannelConcurrencyForContext(c, channel)
}

func EnsureChannelConcurrencyWithWaitForContext(c *gin.Context, channel *model.Channel) (bool, error) {
	if c == nil {
		return false, fmt.Errorf("gin context is nil")
	}
	if channel == nil {
		return false, fmt.Errorf("channel is nil")
	}
	if lease := getChannelConcurrencyLeaseForContext(c); lease != nil {
		if lease.ChannelID == channel.Id {
			return true, nil
		}
		if err := ReleaseChannelConcurrencyForContext(c); err != nil {
			return false, err
		}
	}
	return AcquireChannelConcurrencyWithWaitForContext(c, channel)
}

func AcquireChannelConcurrencyForContext(c *gin.Context, channel *model.Channel) (bool, error) {
	if c == nil {
		return false, fmt.Errorf("gin context is nil")
	}
	ctx := context.Background()
	if c.Request != nil {
		ctx = c.Request.Context()
	}
	lease, ok, err := TryAcquireChannelConcurrency(ctx, channel)
	if err != nil || !ok {
		return ok, err
	}
	if lease != nil && c != nil {
		common.SetContextKey(c, constant.ContextKeyChannelConcurrencyLease, lease)
	}
	return true, nil
}

func AcquireChannelConcurrencyWithWaitForContext(c *gin.Context, channel *model.Channel) (bool, error) {
	if c == nil {
		return false, fmt.Errorf("gin context is nil")
	}
	ctx := context.Background()
	if c.Request != nil {
		ctx = c.Request.Context()
	}
	lease, ok, err := AcquireChannelConcurrencyWithWait(ctx, channel)
	if err != nil || !ok {
		return ok, err
	}
	if lease != nil {
		common.SetContextKey(c, constant.ContextKeyChannelConcurrencyLease, lease)
	}
	return true, nil
}

func ReleaseChannelConcurrencyForContext(c *gin.Context) error {
	if c == nil {
		return nil
	}
	lease := getChannelConcurrencyLeaseForContext(c)
	if lease == nil {
		return nil
	}
	c.Set(string(constant.ContextKeyChannelConcurrencyLease), nil)

	return ReleaseChannelConcurrency(context.Background(), lease)
}

func getChannelConcurrencyLeaseForContext(c *gin.Context) *ChannelConcurrencyLease {
	if c == nil {
		return nil
	}
	value, ok := common.GetContextKey(c, constant.ContextKeyChannelConcurrencyLease)
	if !ok || value == nil {
		return nil
	}
	lease, _ := value.(*ChannelConcurrencyLease)
	return lease
}

func acquireRedisChannelConcurrency(ctx context.Context, channelID int, maxConcurrency int, token string, cooldownEnabled bool) (bool, error) {
	cooldownFlag := "0"
	if cooldownEnabled {
		cooldownFlag = "1"
	}
	result, err := channelConcurrencyAcquireScript.Run(
		ctx,
		common.RDB,
		[]string{channelConcurrencyRedisKey(channelID), channelConcurrencyCooldownRedisKey(channelID)},
		operation_setting.GetChannelConcurrencySlotTTL().Milliseconds(),
		maxConcurrency,
		token,
		cooldownFlag,
	).Int()
	if err != nil {
		cleanupUncertainChannelConcurrencyAcquire(channelID, token)
		return false, fmt.Errorf("acquire channel concurrency in redis failed: %w", err)
	}
	return result == 1, nil
}

func cleanupUncertainChannelConcurrencyAcquire(channelID int, token string) {
	rdb := common.RDB
	if rdb == nil {
		return
	}
	go func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), channelConcurrencyUncertainAcquireCleanupTimeout)
		defer cancel()
		if err := rdb.ZRem(cleanupCtx, channelConcurrencyRedisKey(channelID), token).Err(); err != nil {
			common.SysError(fmt.Sprintf("cleanup uncertain channel concurrency acquire failed: channel_id=%d, error=%s", channelID, err.Error()))
		}
	}()
}

func startChannelConcurrencyLeaseRenewal(lease *ChannelConcurrencyLease) {
	if lease == nil {
		return
	}
	ttl := operation_setting.GetChannelConcurrencySlotTTL()
	interval := channelConcurrencyRenewInterval(ttl)
	if interval <= 0 || interval >= ttl {
		interval = ttl / 3
		if interval <= 0 {
			interval = time.Second
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	lease.renewCancel = cancel
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if lease.released.Load() {
					return
				}
				if lease.useRedis {
					if common.RDB == nil {
						continue
					}
					renewCtx, renewCancel := context.WithTimeout(context.Background(), 3*time.Second)
					if ok, err := renewRedisChannelConcurrency(renewCtx, lease.ChannelID, lease.token); err != nil {
						common.SysError(fmt.Sprintf("renew channel concurrency lease in redis failed: channel_id=%d, error=%s", lease.ChannelID, err.Error()))
					} else if !ok {
						renewCancel()
						return
					}
					renewCancel()
					continue
				}
				refreshMemoryChannelConcurrency(lease.ChannelID, lease.token)
			}
		}
	}()
}

func renewRedisChannelConcurrency(ctx context.Context, channelID int, token string) (bool, error) {
	result, err := channelConcurrencyRenewScript.Run(
		ctx,
		common.RDB,
		[]string{channelConcurrencyRedisKey(channelID)},
		operation_setting.GetChannelConcurrencySlotTTL().Milliseconds(),
		token,
	).Int()
	if err != nil {
		return false, fmt.Errorf("renew channel concurrency in redis failed: %w", err)
	}
	return result == 1, nil
}

func channelConcurrencyRedisKey(channelID int) string {
	return fmt.Sprintf("new-api:channel_concurrency:%d", channelID)
}

func channelConcurrencyWaitingRedisKey(channelID int) string {
	return fmt.Sprintf("new-api:channel_concurrency_wait:%d", channelID)
}

func channelConcurrencyCooldownRedisKey(channelID int) string {
	return fmt.Sprintf("new-api:channel_concurrency_cooldown:%d", channelID)
}

func newChannelConcurrencyToken() string {
	return channelConcurrencyRequestPrefix + ":" + common.GetUUID()
}

func getRedisChannelConcurrencyLoads(ctx context.Context, initial map[int]ChannelConcurrencyLoad) (map[int]ChannelConcurrencyLoad, error) {
	initial = boundedChannelConcurrencyLoads(initial)
	if len(initial) == 0 {
		return map[int]ChannelConcurrencyLoad{}, nil
	}

	now, err := common.RDB.Time(ctx).Result()
	if err != nil {
		return nil, err
	}
	cutoff := strconv.FormatInt(now.UnixMilli(), 10)
	channelIDs := make([]int, 0, len(initial))
	for channelID := range initial {
		channelIDs = append(channelIDs, channelID)
	}
	sort.Ints(channelIDs)

	type loadCommands struct {
		channelID   int
		activeCmd   *redis.IntCmd
		waitingCmd  *redis.StringCmd
		cooldownCmd *redis.IntCmd
	}

	loads := make(map[int]ChannelConcurrencyLoad, len(initial))
	for channelID, load := range initial {
		loads[channelID] = load
	}
	for start := 0; start < len(channelIDs); start += channelConcurrencyLoadBatchSize {
		end := start + channelConcurrencyLoadBatchSize
		if end > len(channelIDs) {
			end = len(channelIDs)
		}
		pipe := common.RDB.Pipeline()
		commands := make([]loadCommands, 0, end-start)
		for _, channelID := range channelIDs[start:end] {
			key := channelConcurrencyRedisKey(channelID)
			pipe.ZRemRangeByScore(ctx, key, "-inf", cutoff)
			commands = append(commands, loadCommands{
				channelID:   channelID,
				activeCmd:   pipe.ZCard(ctx, key),
				waitingCmd:  pipe.Get(ctx, channelConcurrencyWaitingRedisKey(channelID)),
				cooldownCmd: pipe.Exists(ctx, channelConcurrencyCooldownRedisKey(channelID)),
			})
		}
		if _, err := pipe.Exec(ctx); err != nil && err != redis.Nil {
			return nil, err
		}
		for _, command := range commands {
			load := loads[command.channelID]
			if active, err := command.activeCmd.Result(); err == nil {
				load.Active = int(active)
			}
			if waitingValue, err := command.waitingCmd.Result(); err == nil {
				if waiting, parseErr := strconv.Atoi(waitingValue); parseErr == nil && waiting > 0 {
					load.Waiting = waiting
				}
			}
			if coolingDown, err := command.cooldownCmd.Result(); err == nil {
				load.CoolingDown = coolingDown > 0
			}
			load.LoadRate = calculateChannelConcurrencyLoadRate(load.Active, load.Waiting, load.MaxConcurrency)
			loads[command.channelID] = load
		}
	}
	return loads, nil
}

func acquireMemoryChannelConcurrency(channelID int, maxConcurrency int, token string, cooldownEnabled bool) bool {
	channelConcurrencyMemoryMu.Lock()
	defer channelConcurrencyMemoryMu.Unlock()

	now := time.Now()
	cleanupMemoryChannelConcurrencyLocked(now)
	if cooldownEnabled {
		if cooldownUntil, ok := channelConcurrencyMemoryCooldowns[channelID]; ok && cooldownUntil.After(now) {
			return false
		}
	}
	slots := channelConcurrencyMemorySlots[channelID]
	if slots == nil {
		slots = make(map[string]time.Time)
		channelConcurrencyMemorySlots[channelID] = slots
	}

	if _, exists := slots[token]; exists {
		slots[token] = now.Add(operation_setting.GetChannelConcurrencySlotTTL())
		return true
	}

	if len(slots) >= maxConcurrency {
		return false
	}
	slots[token] = now.Add(operation_setting.GetChannelConcurrencySlotTTL())
	return true
}

func releaseMemoryChannelConcurrency(channelID int, token string) {
	channelConcurrencyMemoryMu.Lock()
	defer channelConcurrencyMemoryMu.Unlock()

	slots := channelConcurrencyMemorySlots[channelID]
	if slots == nil {
		return
	}
	delete(slots, token)
	if len(slots) == 0 {
		delete(channelConcurrencyMemorySlots, channelID)
	}
}

func refreshMemoryChannelConcurrency(channelID int, token string) {
	channelConcurrencyMemoryMu.Lock()
	defer channelConcurrencyMemoryMu.Unlock()

	slots := channelConcurrencyMemorySlots[channelID]
	if slots == nil {
		return
	}
	if _, ok := slots[token]; ok {
		slots[token] = time.Now().Add(operation_setting.GetChannelConcurrencySlotTTL())
	}
}

func acquireChannelConcurrencyWaiting(ctx context.Context, channelID int, maxWaiting int) (*channelConcurrencyWaitingLease, bool, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	lease := &channelConcurrencyWaitingLease{
		channelID: channelID,
		useRedis:  common.RedisEnabled && common.RDB != nil,
	}
	if lease.useRedis {
		ttlSeconds := int64((operation_setting.GetChannelConcurrencyWaitTimeout() + time.Minute) / time.Second)
		registered, err := channelConcurrencyWaitRegisterScript.Run(
			ctx,
			common.RDB,
			[]string{channelConcurrencyWaitingRedisKey(channelID)},
			maxWaiting,
			ttlSeconds,
		).Int()
		if err != nil {
			return nil, false, fmt.Errorf("register channel concurrency waiting in redis failed for channel %d: %w", channelID, err)
		}
		if registered != 1 {
			lease.released.Store(true)
			return lease, false, nil
		}
		return lease, true, nil
	}

	channelConcurrencyMemoryMu.Lock()
	defer channelConcurrencyMemoryMu.Unlock()
	if channelConcurrencyMemoryWaits[channelID] >= maxWaiting {
		lease.released.Store(true)
		return lease, false, nil
	}
	channelConcurrencyMemoryWaits[channelID]++
	return lease, true, nil
}

func releaseChannelConcurrencyWaitingLeaseWithLog(lease *channelConcurrencyWaitingLease, channelID int) {
	releaseCtx, releaseCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer releaseCancel()
	if err := lease.Release(releaseCtx); err != nil {
		common.SysError(fmt.Sprintf("release channel concurrency waiting lease failed: channel_id=%d, error=%s", channelID, err.Error()))
	}
}

func incrementChannelConcurrencyWaiting(ctx context.Context, channelID int, maxConcurrency int) (bool, error) {
	maxWaiting := operation_setting.GetChannelConcurrencyMaxWaiting(maxConcurrency)
	_, registered, err := acquireChannelConcurrencyWaiting(ctx, channelID, maxWaiting)
	return registered, err
}

func decrementChannelConcurrencyWaiting(ctx context.Context, channelID int) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if common.RedisEnabled && common.RDB != nil {
		key := channelConcurrencyWaitingRedisKey(channelID)
		value, err := common.RDB.Decr(ctx, key).Result()
		if err == nil {
			if value <= 0 {
				_ = common.RDB.Del(ctx, key).Err()
			}
			return nil
		}
		common.SysError(fmt.Sprintf("decrement channel concurrency waiting in redis failed, fallback to memory: channel_id=%d, error=%s", channelID, err.Error()))
	}

	channelConcurrencyMemoryMu.Lock()
	defer channelConcurrencyMemoryMu.Unlock()
	if channelConcurrencyMemoryWaits[channelID] <= 1 {
		delete(channelConcurrencyMemoryWaits, channelID)
		return nil
	}
	channelConcurrencyMemoryWaits[channelID]--
	return nil
}

func (lease *channelConcurrencyWaitingLease) Release(ctx context.Context) error {
	if lease == nil {
		return nil
	}
	if !lease.released.CompareAndSwap(false, true) {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if lease.useRedis {
		if common.RDB == nil {
			return nil
		}
		key := channelConcurrencyWaitingRedisKey(lease.channelID)
		value, err := common.RDB.Decr(ctx, key).Result()
		if err != nil {
			lease.released.Store(false)
			return fmt.Errorf("decrement channel concurrency waiting in redis failed for channel %d: %w", lease.channelID, err)
		}
		if value <= 0 {
			_ = common.RDB.Del(ctx, key).Err()
		}
		return nil
	}

	channelConcurrencyMemoryMu.Lock()
	defer channelConcurrencyMemoryMu.Unlock()
	if channelConcurrencyMemoryWaits[lease.channelID] <= 1 {
		delete(channelConcurrencyMemoryWaits, lease.channelID)
		return nil
	}
	channelConcurrencyMemoryWaits[lease.channelID]--
	return nil
}

func getMemoryChannelConcurrencyLoads(initial map[int]ChannelConcurrencyLoad) map[int]ChannelConcurrencyLoad {
	channelConcurrencyMemoryMu.Lock()
	defer channelConcurrencyMemoryMu.Unlock()

	now := time.Now()
	cleanupMemoryChannelConcurrencyLocked(now)

	loads := make(map[int]ChannelConcurrencyLoad, len(initial))
	for channelID, load := range initial {
		if load.MaxConcurrency <= 0 {
			loads[channelID] = load
			continue
		}
		load.Active = len(channelConcurrencyMemorySlots[channelID])
		load.Waiting = channelConcurrencyMemoryWaits[channelID]
		if cooldownUntil, ok := channelConcurrencyMemoryCooldowns[channelID]; ok {
			load.CoolingDown = cooldownUntil.After(now)
		}
		load.LoadRate = calculateChannelConcurrencyLoadRate(load.Active, load.Waiting, load.MaxConcurrency)
		loads[channelID] = load
	}
	return loads
}

func boundedChannelConcurrencyLoads(loads map[int]ChannelConcurrencyLoad) map[int]ChannelConcurrencyLoad {
	bounded := make(map[int]ChannelConcurrencyLoad, len(loads))
	for channelID, load := range loads {
		if load.MaxConcurrency > 0 {
			bounded[channelID] = load
		}
	}
	return bounded
}

func cleanupMemoryChannelConcurrencyLocked(now time.Time) {
	for channelID, slots := range channelConcurrencyMemorySlots {
		for token, expiresAt := range slots {
			if !expiresAt.After(now) {
				delete(slots, token)
			}
		}
		if len(slots) == 0 {
			delete(channelConcurrencyMemorySlots, channelID)
		}
	}
	for channelID, expiresAt := range channelConcurrencyMemoryCooldowns {
		if !expiresAt.After(now) {
			delete(channelConcurrencyMemoryCooldowns, channelID)
		}
	}
}

func calculateChannelConcurrencyLoadRate(active int, waiting int, maxConcurrency int) float64 {
	if maxConcurrency <= 0 {
		return 0
	}
	return float64(active+waiting) / float64(maxConcurrency)
}

func resetChannelConcurrencyForTest() {
	channelConcurrencyMemoryMu.Lock()
	channelConcurrencyMemorySlots = make(map[int]map[string]time.Time)
	channelConcurrencyMemoryWaits = make(map[int]int)
	channelConcurrencyMemoryCooldowns = make(map[int]time.Time)
	channelConcurrencyMemoryMu.Unlock()

	channelConcurrencyLoadCacheMu.Lock()
	channelConcurrencyLoadCache = make(map[string]cachedChannelConcurrencyLoads)
	channelConcurrencyLoadCacheMu.Unlock()
}
