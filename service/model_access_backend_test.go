package service

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/stretchr/testify/require"
)

func setupBackendModelAccessTest(t *testing.T) {
	t.Helper()
	originalDB := model.DB
	originalLogDB := model.LOG_DB
	originalSQLitePath := common.SQLitePath
	originalIsMaster := common.IsMasterNode
	originalRedis := common.RedisEnabled
	originalMainType := common.MainDatabaseType()
	originalLogType := common.LogDatabaseType()
	originalModelRatios := ratio_setting.ModelRatio2JSONString()
	originalGroupRatios := ratio_setting.GroupRatio2JSONString()
	originalUsableGroups := setting.UserUsableGroups2JSONString()
	originalAutoGroups := setting.AutoGroups2JsonString()
	originalSelfUse := operation_setting.SelfUseModeEnabled
	originalDSN, hadDSN := os.LookupEnv("SQL_DSN")

	common.SQLitePath = fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	common.IsMasterNode = false
	common.RedisEnabled = false
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	require.NoError(t, os.Setenv("SQL_DSN", "local"))
	require.NoError(t, model.InitDB())
	require.NoError(t, model.DB.AutoMigrate(&model.Channel{}, &model.Ability{}, &model.Model{}, &model.Vendor{}, &model.ModelAvailabilityState{}))
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{"shared":1,"default-only":1,"vip-only":1}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"vip":2}`))
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default","vip":"VIP","auto":"Auto"}`))
	require.NoError(t, setting.UpdateAutoGroupsByJsonString(`["default","vip"]`))
	operation_setting.SelfUseModeEnabled = false

	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(originalModelRatios))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalGroupRatios))
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(originalUsableGroups))
		require.NoError(t, setting.UpdateAutoGroupsByJsonString(originalAutoGroups))
		operation_setting.SelfUseModeEnabled = originalSelfUse
		model.DB = originalDB
		model.LOG_DB = originalLogDB
		common.SQLitePath = originalSQLitePath
		common.IsMasterNode = originalIsMaster
		common.RedisEnabled = originalRedis
		common.SetDatabaseTypes(originalMainType, originalLogType)
		if hadDSN {
			require.NoError(t, os.Setenv("SQL_DSN", originalDSN))
		} else {
			require.NoError(t, os.Unsetenv("SQL_DSN"))
		}
	})
}

func seedBackendModelAccessChannel(t *testing.T, channelID int, group string, modelNames ...string) {
	t.Helper()
	priority := int64(0)
	weight := uint(100)
	db := model.DB
	require.NoError(t, db.Create(&model.Channel{
		Id: channelID, Type: constant.ChannelTypeOpenAI, Status: common.ChannelStatusEnabled,
		Key: fmt.Sprintf("key-%d", channelID), Models: strings.Join(modelNames, ","), Group: group,
		Priority: &priority, Weight: &weight,
	}).Error)
	abilities := make([]model.Ability, 0, len(modelNames))
	for _, modelName := range modelNames {
		abilities = append(abilities, model.Ability{
			Group: group, Model: modelName, ChannelId: channelID, Enabled: true,
			Priority: &priority, Weight: weight,
		})
	}
	require.NoError(t, db.Create(&abilities).Error)
}

func TestResolveUserModelAccessUsesBaseGroupsAndAvailabilityStates(t *testing.T) {
	setupBackendModelAccessTest(t)
	seedBackendModelAccessChannel(t, 1, "default", "shared", "default-only", "disabled-channel")
	seedBackendModelAccessChannel(t, 2, "vip", "shared", "vip-only")
	require.NoError(t, model.DB.Model(&model.Ability{}).Where("model = ?", "disabled-channel").Update("enabled", false).Error)
	require.NoError(t, model.DB.Create(&model.ModelAvailabilityState{ModelName: "shared", Status: model.ModelAvailabilityTemporaryFailure}).Error)
	require.NoError(t, model.DB.Create(&model.ModelAvailabilityState{ModelName: "vip-only", Status: model.ModelAvailabilityOfficialUnsupported}).Error)

	access, err := ResolveUserModelAccess(&model.UserBase{Group: "default"})
	require.NoError(t, err)
	require.Equal(t, ModelAccessScopeSelectableGroup, access.ScopeMode)
	require.Equal(t, []string{"default-only", "shared"}, access.IdentityModelIDs)
	require.Equal(t, []string{"auto", "default", "vip"}, modelAccessScopeIDsForBackendTest(access.Groups))
	require.Equal(t, []string{"default-only", "shared", "vip-only"}, modelAccessModelIDsForBackendTest(access.Models))
	require.NotContains(t, modelAccessModelIDsForBackendTest(access.Models), "disabled-channel")

	byID := make(map[string]ModelAccessModel, len(access.Models))
	for _, item := range access.Models {
		byID[item.ID] = item
	}
	require.Equal(t, model.ModelAvailabilityTemporaryFailure, byID["shared"].AvailabilityStatus)
	require.Equal(t, model.ModelAvailabilityOfficialUnsupported, byID["vip-only"].AvailabilityStatus)
}

func modelAccessScopeIDsForBackendTest(scopes []ModelAccessScope) []string {
	ids := make([]string, len(scopes))
	for i, scope := range scopes {
		ids[i] = scope.ID
	}
	return ids
}

func modelAccessModelIDsForBackendTest(models []ModelAccessModel) []string {
	ids := make([]string, len(models))
	for i, item := range models {
		ids[i] = item.ID
	}
	return ids
}
