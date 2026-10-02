package service

import (
	"testing"

	"github.com/QuantumNous/new-api/setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeModelAccessIdentityGroupUsesBaseDefault(t *testing.T) {
	assert.Equal(t, "default", normalizeModelAccessIdentityGroup(""))
	assert.Equal(t, "default", normalizeModelAccessIdentityGroup("  "))
	assert.Equal(t, "vip", normalizeModelAccessIdentityGroup(" vip "))
}

func TestResolveTokenAccessGroupsDoesNotSpecialCaseGroupNames(t *testing.T) {
	originalGroups := setting.UserUsableGroups2JSONString()
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default","vip":"VIP"}`))
	t.Cleanup(func() {
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(originalGroups))
	})

	assert.Equal(t, []string{"vip"}, resolveTokenAccessGroups("plg", "vip"))
}
