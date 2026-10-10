package model

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestUpdateOptionRejectsInvalidChannelConcurrencySettingsBeforePersistence(t *testing.T) {
	previousDB := DB
	previousSetting := operation_setting.GetChannelConcurrencySetting()
	common.OptionMapRWMutex.Lock()
	previousOptionMap := common.OptionMap
	common.OptionMap = map[string]string{}
	common.OptionMapRWMutex.Unlock()
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, database.AutoMigrate(&Option{}))
	DB = database
	t.Cleanup(func() {
		DB = previousDB
		operation_setting.SetChannelConcurrencySettingForTest(previousSetting)
		common.OptionMapRWMutex.Lock()
		common.OptionMap = previousOptionMap
		common.OptionMapRWMutex.Unlock()
	})

	testCases := []struct {
		key   string
		value string
	}{
		{key: "channel_concurrency_setting.slot_ttl_minutes", value: "0"},
		{key: "channel_concurrency_setting.wait_enabled", value: "enabled"},
		{key: "channel_concurrency_setting.wait_timeout_ms", value: "60001"},
		{key: "channel_concurrency_setting.wait_interval_ms", value: "5001"},
		{key: "channel_concurrency_setting.max_waiting_per_channel", value: "-1"},
		{key: "channel_concurrency_setting.cooldown_seconds", value: "0"},
		{key: "channel_concurrency_setting.cooldown_seconds", value: "3601"},
		{key: "channel_concurrency_setting.cooldown_seconds", value: "1.5"},
		{key: "channel_concurrency_setting.cooldown_enabled", value: "enabled"},
		{key: "channel_concurrency_setting.load_cache_enabled", value: "enabled"},
		{key: "channel_concurrency_setting.load_cache_ttl_ms", value: "5001"},
		{key: "channel_concurrency_setting.max_acquire_attempts", value: "101"},
	}
	for _, testCase := range testCases {
		err = UpdateOption(testCase.key, testCase.value)
		require.Error(t, err)
	}

	var count int64
	require.NoError(t, database.Model(&Option{}).Where("key LIKE ?", "channel_concurrency_setting.%").Count(&count).Error)
	require.Zero(t, count)
	require.Equal(t, previousSetting.CooldownSeconds, operation_setting.GetChannelConcurrencySetting().CooldownSeconds)
}

func TestUpdateOptionAppliesChannelConcurrencyLoadObservationWithoutRestart(t *testing.T) {
	previousDB := DB
	previousSetting := operation_setting.GetChannelConcurrencySetting()
	common.OptionMapRWMutex.Lock()
	previousOptionMap := common.OptionMap
	common.OptionMap = map[string]string{}
	common.OptionMapRWMutex.Unlock()
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, database.AutoMigrate(&Option{}))
	DB = database
	t.Cleanup(func() {
		DB = previousDB
		operation_setting.SetChannelConcurrencySettingForTest(previousSetting)
		common.OptionMapRWMutex.Lock()
		common.OptionMap = previousOptionMap
		common.OptionMapRWMutex.Unlock()
	})

	require.NoError(t, UpdateOption("channel_concurrency_setting.load_cache_enabled", "true"))
	require.NoError(t, UpdateOption("channel_concurrency_setting.load_cache_ttl_ms", "750"))
	require.NoError(t, UpdateOption("channel_concurrency_setting.max_acquire_attempts", "5"))
	require.Equal(t, 750*time.Millisecond, operation_setting.GetChannelConcurrencyLoadCacheTTL())
	require.Equal(t, 5, operation_setting.GetChannelConcurrencyMaxAcquireAttempts())

	var count int64
	require.NoError(t, database.Model(&Option{}).Where("key LIKE ?", "channel_concurrency_setting.%").Count(&count).Error)
	require.EqualValues(t, 3, count)
}
