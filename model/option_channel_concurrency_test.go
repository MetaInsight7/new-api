package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestUpdateOptionRejectsInvalidChannelConcurrencyCooldownBeforePersistence(t *testing.T) {
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
		{key: "channel_concurrency_setting.cooldown_seconds", value: "0"},
		{key: "channel_concurrency_setting.cooldown_seconds", value: "3601"},
		{key: "channel_concurrency_setting.cooldown_seconds", value: "1.5"},
		{key: "channel_concurrency_setting.cooldown_enabled", value: "enabled"},
	}
	for _, testCase := range testCases {
		err = UpdateOption(testCase.key, testCase.value)
		require.Error(t, err)
	}

	var count int64
	require.NoError(t, database.Model(&Option{}).Where("key LIKE ?", "channel_concurrency_setting.cooldown_%").Count(&count).Error)
	require.Zero(t, count)
	require.Equal(t, previousSetting.CooldownSeconds, operation_setting.GetChannelConcurrencySetting().CooldownSeconds)
}
