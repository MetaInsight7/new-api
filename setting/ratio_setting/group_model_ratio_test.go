package ratio_setting

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEffectiveGroupRatioPrefersModelOverride(t *testing.T) {
	originalGroups := GroupRatio2JSONString()
	originalSpecial := GroupGroupRatio2JSONString()
	originalModels := GroupModelRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, UpdateGroupRatioByJSONString(originalGroups))
		require.NoError(t, UpdateGroupGroupRatioByJSONString(originalSpecial))
		require.NoError(t, UpdateGroupModelRatioByJSONString(originalModels))
	})

	require.NoError(t, UpdateGroupRatioByJSONString(`{"default":1,"vip":2}`))
	require.NoError(t, UpdateGroupGroupRatioByJSONString(`{"vip":{"vip":0.8}}`))
	require.NoError(t, UpdateGroupModelRatioByJSONString(`{"vip":{"gpt-5":0.3,"free-model":0}}`))

	modelOverride := GetEffectiveGroupRatio("vip", "vip", "gpt-5")
	require.Equal(t, 0.3, modelOverride.GroupRatio)
	require.True(t, modelOverride.HasGroupModelRatio)
	require.False(t, modelOverride.HasSpecialRatio)
	require.Equal(t, "vip", modelOverride.GroupModelRatioGroup)
	require.Equal(t, "gpt-5", modelOverride.GroupModelRatioModel)

	freeModel := GetEffectiveGroupRatio("vip", "vip", "free-model")
	require.Zero(t, freeModel.GroupRatio)
	require.True(t, freeModel.HasGroupModelRatio)

	userOverride := GetEffectiveGroupRatio("vip", "vip", "other-model")
	require.Equal(t, 0.8, userOverride.GroupRatio)
	require.True(t, userOverride.HasSpecialRatio)

	defaultOverride := GetEffectiveGroupRatio("default", "vip", "other-model")
	require.Equal(t, 2.0, defaultOverride.GroupRatio)
}

func TestCheckGroupModelRatioRejectsNegativeValues(t *testing.T) {
	require.NoError(t, CheckGroupModelRatio(`{"vip":{"gpt-5":0}}`))
	require.Error(t, CheckGroupModelRatio(`{"vip":{"gpt-5":-0.1}}`))
	require.Error(t, CheckGroupModelRatio(`not-json`))
}
