package operation_setting

import (
	"fmt"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/setting/config"
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

func TestChannelConcurrencyCooldownPolicyDefaults(t *testing.T) {
	setting := GetChannelConcurrencySetting()
	policy := GetChannelConcurrencyCooldownPolicy()

	require.True(t, setting.CooldownEnabled)
	require.Equal(t, defaultChannelConcurrencyCooldownSecs, setting.CooldownSeconds)
	require.True(t, setting.CooldownOnStatus429)
	require.False(t, setting.CooldownOnMessageMatch)
	require.True(t, policy.Enabled)
	require.Equal(t, 30*time.Second, policy.Duration)
	require.True(t, policy.OnStatus429)
	require.False(t, policy.OnMessageMatch)
}

func TestChannelConcurrencyCooldownPolicyInvalidDurationUsesRuntimeDefault(t *testing.T) {
	original := GetChannelConcurrencySetting()
	t.Cleanup(func() {
		SetChannelConcurrencySettingForTest(original)
	})

	for _, seconds := range []int{0, -1, maxChannelConcurrencyCooldownSeconds + 1} {
		setting := original
		setting.CooldownSeconds = seconds
		SetChannelConcurrencySettingForTest(setting)

		policy := GetChannelConcurrencyCooldownPolicy()
		require.Equal(t, time.Duration(defaultChannelConcurrencyCooldownSecs)*time.Second, policy.Duration)
		require.Equal(t, policy.Duration, GetChannelConcurrencyCooldown())
	}
}

func TestChannelConcurrencyCooldownPolicyAcceptsSupportedDurationBounds(t *testing.T) {
	original := GetChannelConcurrencySetting()
	t.Cleanup(func() {
		SetChannelConcurrencySettingForTest(original)
	})

	for _, seconds := range []int{1, maxChannelConcurrencyCooldownSeconds} {
		setting := original
		setting.CooldownSeconds = seconds
		SetChannelConcurrencySettingForTest(setting)

		require.Equal(t, time.Duration(seconds)*time.Second, GetChannelConcurrencyCooldownPolicy().Duration)
	}
}

func TestChannelConcurrencyCooldownPolicyRejectsNonIntegerHotUpdate(t *testing.T) {
	original := GetChannelConcurrencySetting()
	t.Cleanup(func() {
		SetChannelConcurrencySettingForTest(original)
	})

	configValue := config.GlobalConfig.Get("channel_concurrency_setting")
	err := config.UpdateConfigFromMap(configValue, map[string]string{"cooldown_seconds": "1.5"})
	require.Error(t, err)
	require.Equal(t, original.CooldownSeconds, GetChannelConcurrencySetting().CooldownSeconds)
}

func TestChannelConcurrencyCooldownPolicyHotUpdateIsConsistent(t *testing.T) {
	original := GetChannelConcurrencySetting()
	t.Cleanup(func() {
		SetChannelConcurrencySettingForTest(original)
	})

	configValue := config.GlobalConfig.Get("channel_concurrency_setting")
	first := map[string]string{
		"cooldown_enabled":          "true",
		"cooldown_seconds":          "1",
		"cooldown_on_status_429":    "true",
		"cooldown_on_message_match": "false",
	}
	second := map[string]string{
		"cooldown_enabled":          "false",
		"cooldown_seconds":          "3600",
		"cooldown_on_status_429":    "false",
		"cooldown_on_message_match": "true",
	}
	require.NoError(t, config.UpdateConfigFromMap(configValue, first))

	done := make(chan struct{})
	readError := make(chan error, 1)
	updateError := make(chan error, 1)
	go func() {
		defer close(done)
		for range 5000 {
			if err := config.UpdateConfigFromMap(configValue, second); err != nil {
				updateError <- err
				return
			}
			if err := config.UpdateConfigFromMap(configValue, first); err != nil {
				updateError <- err
				return
			}
		}
	}()

	for {
		select {
		case <-done:
			select {
			case err := <-updateError:
				require.NoError(t, err)
			default:
			}
			select {
			case err := <-readError:
				require.NoError(t, err)
			default:
			}
			return
		default:
			policy := GetChannelConcurrencyCooldownPolicy()
			isFirst := policy.Enabled && policy.Duration == time.Second && policy.OnStatus429 && !policy.OnMessageMatch
			isSecond := !policy.Enabled && policy.Duration == time.Hour && !policy.OnStatus429 && policy.OnMessageMatch
			if !isFirst && !isSecond {
				select {
				case readError <- fmt.Errorf("observed partial cooldown policy: %+v", policy):
				default:
				}
			}
		}
	}
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
