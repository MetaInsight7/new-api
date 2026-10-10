package operation_setting

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/setting/config"
)

const (
	defaultChannelConcurrencySlotTTLMinutes     = 30
	defaultChannelConcurrencyWaitTimeoutMS      = 5000
	defaultChannelConcurrencyWaitIntervalMS     = 100
	defaultChannelConcurrencyCooldownSecs       = 30
	defaultChannelConcurrencyLoadCacheTTLMS     = 200
	defaultChannelConcurrencyMaxAcquireAttempts = 8

	maxChannelConcurrencySlotTTLMinutes       = 24 * 60
	maxChannelConcurrencyWaitTimeoutMS        = 60 * 1000
	maxChannelConcurrencyWaitIntervalMS       = 5 * 1000
	maxChannelConcurrencyCooldownSeconds      = 60 * 60
	maxChannelConcurrencyMaxWaitingPerChannel = 10000
	maxChannelConcurrencyLoadCacheTTLMS       = 5000
	maxChannelConcurrencyMaxAcquireAttempts   = 100
)

type ChannelConcurrencySetting struct {
	SlotTTLMinutes         int  `json:"slot_ttl_minutes"`
	WaitEnabled            bool `json:"wait_enabled"`
	WaitTimeoutMS          int  `json:"wait_timeout_ms"`
	WaitIntervalMS         int  `json:"wait_interval_ms"`
	MaxWaitingPerChannel   int  `json:"max_waiting_per_channel"`
	CooldownEnabled        bool `json:"cooldown_enabled"`
	CooldownSeconds        int  `json:"cooldown_seconds"`
	CooldownOnStatus429    bool `json:"cooldown_on_status_429"`
	CooldownOnMessageMatch bool `json:"cooldown_on_message_match"`
	LoadCacheEnabled       bool `json:"load_cache_enabled"`
	LoadCacheTTLMS         int  `json:"load_cache_ttl_ms"`
	MaxAcquireAttempts     int  `json:"max_acquire_attempts"`
}

var channelConcurrencySetting = ChannelConcurrencySetting{
	SlotTTLMinutes:         defaultChannelConcurrencySlotTTLMinutes,
	WaitEnabled:            true,
	WaitTimeoutMS:          defaultChannelConcurrencyWaitTimeoutMS,
	WaitIntervalMS:         defaultChannelConcurrencyWaitIntervalMS,
	MaxWaitingPerChannel:   0,
	CooldownEnabled:        true,
	CooldownSeconds:        defaultChannelConcurrencyCooldownSecs,
	CooldownOnStatus429:    true,
	CooldownOnMessageMatch: false,
	LoadCacheEnabled:       true,
	LoadCacheTTLMS:         defaultChannelConcurrencyLoadCacheTTLMS,
	MaxAcquireAttempts:     defaultChannelConcurrencyMaxAcquireAttempts,
}

type ChannelConcurrencyCooldownPolicy struct {
	Enabled        bool
	Duration       time.Duration
	OnStatus429    bool
	OnMessageMatch bool
}

var channelConcurrencySettingMu sync.RWMutex

func init() {
	config.GlobalConfig.Register("channel_concurrency_setting", &channelConcurrencySetting)
}

func (s *ChannelConcurrencySetting) LockConfig() {
	channelConcurrencySettingMu.Lock()
}

func (s *ChannelConcurrencySetting) UnlockConfig() {
	channelConcurrencySettingMu.Unlock()
}

func (s *ChannelConcurrencySetting) RLockConfig() {
	channelConcurrencySettingMu.RLock()
}

func (s *ChannelConcurrencySetting) RUnlockConfig() {
	channelConcurrencySettingMu.RUnlock()
}

func (s *ChannelConcurrencySetting) ValidateConfigUpdate(values map[string]string) error {
	return validateChannelConcurrencyConfigUpdate(values)
}

func ValidateChannelConcurrencyOption(key string, value string) error {
	configKey, ok := strings.CutPrefix(key, "channel_concurrency_setting.")
	if !ok {
		return nil
	}
	return validateChannelConcurrencyConfigUpdate(map[string]string{configKey: value})
}

func validateChannelConcurrencyConfigUpdate(values map[string]string) error {
	integerSettings := []struct {
		key string
		min int
		max int
	}{
		{key: "slot_ttl_minutes", min: 1, max: maxChannelConcurrencySlotTTLMinutes},
		{key: "wait_timeout_ms", min: 1, max: maxChannelConcurrencyWaitTimeoutMS},
		{key: "wait_interval_ms", min: 1, max: maxChannelConcurrencyWaitIntervalMS},
		{key: "max_waiting_per_channel", min: 0, max: maxChannelConcurrencyMaxWaitingPerChannel},
		{key: "cooldown_seconds", min: 1, max: maxChannelConcurrencyCooldownSeconds},
		{key: "load_cache_ttl_ms", min: 1, max: maxChannelConcurrencyLoadCacheTTLMS},
		{key: "max_acquire_attempts", min: 1, max: maxChannelConcurrencyMaxAcquireAttempts},
	}
	for _, integerSetting := range integerSettings {
		value, ok := values[integerSetting.key]
		if !ok {
			continue
		}
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < integerSetting.min || parsed > integerSetting.max {
			return fmt.Errorf("channel concurrency setting %s must be an integer between %d and %d", integerSetting.key, integerSetting.min, integerSetting.max)
		}
	}
	for _, key := range []string{"wait_enabled", "cooldown_enabled", "cooldown_on_status_429", "cooldown_on_message_match", "load_cache_enabled"} {
		if value, ok := values[key]; ok {
			if _, err := strconv.ParseBool(value); err != nil {
				return fmt.Errorf("channel concurrency setting %s must be a boolean", key)
			}
		}
	}
	return nil
}

func GetChannelConcurrencySetting() ChannelConcurrencySetting {
	channelConcurrencySettingMu.RLock()
	defer channelConcurrencySettingMu.RUnlock()
	return channelConcurrencySetting
}

func SetChannelConcurrencySettingForTest(setting ChannelConcurrencySetting) {
	channelConcurrencySettingMu.Lock()
	defer channelConcurrencySettingMu.Unlock()
	channelConcurrencySetting = setting
}

func GetChannelConcurrencySlotTTLMinutes() int {
	channelConcurrencySettingMu.RLock()
	defer channelConcurrencySettingMu.RUnlock()
	if channelConcurrencySetting.SlotTTLMinutes <= 0 || channelConcurrencySetting.SlotTTLMinutes > maxChannelConcurrencySlotTTLMinutes {
		return defaultChannelConcurrencySlotTTLMinutes
	}
	return channelConcurrencySetting.SlotTTLMinutes
}

func GetChannelConcurrencySlotTTL() time.Duration {
	return time.Duration(GetChannelConcurrencySlotTTLMinutes()) * time.Minute
}

func IsChannelConcurrencyWaitEnabled() bool {
	channelConcurrencySettingMu.RLock()
	defer channelConcurrencySettingMu.RUnlock()
	return channelConcurrencySetting.WaitEnabled
}

func GetChannelConcurrencyWaitTimeout() time.Duration {
	channelConcurrencySettingMu.RLock()
	defer channelConcurrencySettingMu.RUnlock()
	if channelConcurrencySetting.WaitTimeoutMS <= 0 || channelConcurrencySetting.WaitTimeoutMS > maxChannelConcurrencyWaitTimeoutMS {
		return time.Duration(defaultChannelConcurrencyWaitTimeoutMS) * time.Millisecond
	}
	return time.Duration(channelConcurrencySetting.WaitTimeoutMS) * time.Millisecond
}

func GetChannelConcurrencyWaitInterval() time.Duration {
	channelConcurrencySettingMu.RLock()
	defer channelConcurrencySettingMu.RUnlock()
	if channelConcurrencySetting.WaitIntervalMS <= 0 || channelConcurrencySetting.WaitIntervalMS > maxChannelConcurrencyWaitIntervalMS {
		return time.Duration(defaultChannelConcurrencyWaitIntervalMS) * time.Millisecond
	}
	return time.Duration(channelConcurrencySetting.WaitIntervalMS) * time.Millisecond
}

func GetChannelConcurrencyMaxWaiting(maxConcurrency int) int {
	channelConcurrencySettingMu.RLock()
	defer channelConcurrencySettingMu.RUnlock()
	maxWaiting := 1
	if channelConcurrencySetting.MaxWaitingPerChannel > 0 {
		maxWaiting = channelConcurrencySetting.MaxWaitingPerChannel
	} else if maxConcurrency > 0 {
		maxWaiting = maxConcurrency
	}
	if maxWaiting > maxChannelConcurrencyMaxWaitingPerChannel {
		return maxChannelConcurrencyMaxWaitingPerChannel
	}
	return maxWaiting
}

func IsChannelConcurrencyCooldownEnabled() bool {
	return GetChannelConcurrencyCooldownPolicy().Enabled
}

func IsChannelConcurrencyCooldownOnStatus429() bool {
	return GetChannelConcurrencyCooldownPolicy().OnStatus429
}

func IsChannelConcurrencyCooldownOnMessageMatch() bool {
	return GetChannelConcurrencyCooldownPolicy().OnMessageMatch
}

func GetChannelConcurrencyCooldown() time.Duration {
	return GetChannelConcurrencyCooldownPolicy().Duration
}

func GetChannelConcurrencyCooldownPolicy() ChannelConcurrencyCooldownPolicy {
	channelConcurrencySettingMu.RLock()
	defer channelConcurrencySettingMu.RUnlock()
	cooldownSeconds := channelConcurrencySetting.CooldownSeconds
	if cooldownSeconds <= 0 || cooldownSeconds > maxChannelConcurrencyCooldownSeconds {
		cooldownSeconds = defaultChannelConcurrencyCooldownSecs
	}
	return ChannelConcurrencyCooldownPolicy{
		Enabled:        channelConcurrencySetting.CooldownEnabled,
		Duration:       time.Duration(cooldownSeconds) * time.Second,
		OnStatus429:    channelConcurrencySetting.CooldownOnStatus429,
		OnMessageMatch: channelConcurrencySetting.CooldownOnMessageMatch,
	}
}

func GetChannelConcurrencyLoadCacheTTL() time.Duration {
	channelConcurrencySettingMu.RLock()
	defer channelConcurrencySettingMu.RUnlock()
	if !channelConcurrencySetting.LoadCacheEnabled {
		return 0
	}
	if channelConcurrencySetting.LoadCacheTTLMS <= 0 || channelConcurrencySetting.LoadCacheTTLMS > maxChannelConcurrencyLoadCacheTTLMS {
		return time.Duration(defaultChannelConcurrencyLoadCacheTTLMS) * time.Millisecond
	}
	return time.Duration(channelConcurrencySetting.LoadCacheTTLMS) * time.Millisecond
}

func GetChannelConcurrencyMaxAcquireAttempts() int {
	channelConcurrencySettingMu.RLock()
	defer channelConcurrencySettingMu.RUnlock()
	if channelConcurrencySetting.MaxAcquireAttempts <= 0 || channelConcurrencySetting.MaxAcquireAttempts > maxChannelConcurrencyMaxAcquireAttempts {
		return defaultChannelConcurrencyMaxAcquireAttempts
	}
	return channelConcurrencySetting.MaxAcquireAttempts
}
