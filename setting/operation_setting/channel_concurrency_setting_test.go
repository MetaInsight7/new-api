package operation_setting

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestChannelConcurrencySlotAndWaitingDefaults(t *testing.T) {
	setting := GetChannelConcurrencySetting()

	require.True(t, setting.WaitEnabled)
	require.Equal(t, 30, GetChannelConcurrencySlotTTLMinutes())
	require.Equal(t, 5*time.Second, GetChannelConcurrencyWaitTimeout())
	require.Equal(t, 100*time.Millisecond, GetChannelConcurrencyWaitInterval())
	require.Equal(t, 3, GetChannelConcurrencyMaxWaiting(3))
	require.Equal(t, 1, GetChannelConcurrencyMaxWaiting(0))
}

func TestChannelConcurrencySlotAndWaitingInvalidValuesUseRuntimeDefaults(t *testing.T) {
	original := GetChannelConcurrencySetting()
	t.Cleanup(func() {
		SetChannelConcurrencySettingForTest(original)
	})

	testCases := []struct {
		name    string
		setting ChannelConcurrencySetting
	}{
		{
			name: "non-positive durations",
			setting: ChannelConcurrencySetting{
				SlotTTLMinutes:       0,
				WaitEnabled:          true,
				WaitTimeoutMS:        -1,
				WaitIntervalMS:       0,
				MaxWaitingPerChannel: -1,
			},
		},
		{
			name: "values above supported maxima",
			setting: ChannelConcurrencySetting{
				SlotTTLMinutes:       maxChannelConcurrencySlotTTLMinutes + 1,
				WaitEnabled:          true,
				WaitTimeoutMS:        maxChannelConcurrencyWaitTimeoutMS + 1,
				WaitIntervalMS:       maxChannelConcurrencyWaitIntervalMS + 1,
				MaxWaitingPerChannel: maxChannelConcurrencyMaxWaitingPerChannel + 1,
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			SetChannelConcurrencySettingForTest(testCase.setting)

			require.Equal(t, defaultChannelConcurrencySlotTTLMinutes, GetChannelConcurrencySlotTTLMinutes())
			require.Equal(t, time.Duration(defaultChannelConcurrencyWaitTimeoutMS)*time.Millisecond, GetChannelConcurrencyWaitTimeout())
			require.Equal(t, time.Duration(defaultChannelConcurrencyWaitIntervalMS)*time.Millisecond, GetChannelConcurrencyWaitInterval())
			require.Equal(t, maxChannelConcurrencyMaxWaitingPerChannel, GetChannelConcurrencyMaxWaiting(maxChannelConcurrencyMaxWaitingPerChannel+1))
		})
	}
}

func TestChannelConcurrencyExplicitWaitingLimitUsesSupportedValue(t *testing.T) {
	original := GetChannelConcurrencySetting()
	t.Cleanup(func() {
		SetChannelConcurrencySettingForTest(original)
	})

	setting := original
	setting.MaxWaitingPerChannel = 7
	SetChannelConcurrencySettingForTest(setting)

	require.Equal(t, 7, GetChannelConcurrencyMaxWaiting(3))
}

func TestGetChannelConcurrencySettingReturnsSnapshot(t *testing.T) {
	original := GetChannelConcurrencySetting()
	t.Cleanup(func() {
		SetChannelConcurrencySettingForTest(original)
	})

	snapshot := GetChannelConcurrencySetting()
	snapshot.WaitTimeoutMS = 1

	require.NotEqual(t, time.Millisecond, GetChannelConcurrencyWaitTimeout())
}
