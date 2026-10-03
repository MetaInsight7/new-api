package helper

import (
	"net/http/httptest"
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestHandleGroupRatioUsesBillingModelOverride(t *testing.T) {
	originalGroups := ratio_setting.GroupRatio2JSONString()
	originalSpecial := ratio_setting.GroupGroupRatio2JSONString()
	originalModels := ratio_setting.GroupModelRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalGroups))
		require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(originalSpecial))
		require.NoError(t, ratio_setting.UpdateGroupModelRatioByJSONString(originalModels))
	})

	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"vip":2}`))
	require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(`{"default":{"vip":0.8}}`))
	require.NoError(t, ratio_setting.UpdateGroupModelRatioByJSONString(`{"vip":{"canonical-model":0.3}}`))

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	info := &relaycommon.RelayInfo{
		OriginModelName:  "upstream-model",
		BillingModelName: "canonical-model",
		UserGroup:        "default",
		UsingGroup:       "vip",
	}

	got := HandleGroupRatio(ctx, info)
	require.Equal(t, 0.3, got.GroupRatio)
	require.True(t, got.HasGroupModelRatio)
	require.False(t, got.HasSpecialRatio)
	require.Equal(t, "vip", got.GroupModelRatioGroup)
	require.Equal(t, "canonical-model", got.GroupModelRatioModel)
}
