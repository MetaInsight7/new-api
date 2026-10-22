package model_setting

import (
	"testing"

	"github.com/QuantumNous/new-api/setting/config"
	"github.com/stretchr/testify/require"
)

func TestGetCodexSettingsReturnsSnapshot(t *testing.T) {
	settings := config.GlobalConfig.Get("codex")
	original := GetCodexSettings().ImageCarrierModel
	t.Cleanup(func() {
		require.NoError(t, config.UpdateConfigFromMap(settings, map[string]string{"image_carrier_model": original}))
	})

	require.NoError(t, config.UpdateConfigFromMap(settings, map[string]string{"image_carrier_model": "snapshot-before"}))
	snapshot := GetCodexSettings()
	require.NoError(t, config.UpdateConfigFromMap(settings, map[string]string{"image_carrier_model": "snapshot-after"}))

	require.Equal(t, "snapshot-before", snapshot.ImageCarrierModel)
	require.Equal(t, "snapshot-after", GetCodexSettings().ImageCarrierModel)
}
