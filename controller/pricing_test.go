package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type pricingResponse struct {
	Success           bool                           `json:"success"`
	Data              []model.Pricing                `json:"data"`
	Vendors           []model.PricingVendor          `json:"vendors"`
	GroupRatio        map[string]float64             `json:"group_ratio"`
	GroupModelRatio   map[string]map[string]float64  `json:"group_model_ratio"`
	UsableGroup       map[string]string              `json:"usable_group"`
	SupportedEndpoint map[string]common.EndpointInfo `json:"supported_endpoint"`
	AutoGroups        []string                       `json:"auto_groups"`
	PricingVersion    string                         `json:"pricing_version"`
}

func setupPricingControllerTest(testContext *testing.T) {
	testContext.Helper()

	originalDB, originalLogDB := model.DB, model.LOG_DB
	originalMainType, originalLogType := common.MainDatabaseType(), common.LogDatabaseType()
	originalRedisEnabled := common.RedisEnabled
	originalGinMode := gin.Mode()
	originalGroups := ratio_setting.GroupRatio2JSONString()
	originalGroupGroups := ratio_setting.GroupGroupRatio2JSONString()
	originalGroupModels := ratio_setting.GroupModelRatio2JSONString()
	originalUsableGroups := setting.UserUsableGroups2JSONString()
	originalAutoGroups := setting.AutoGroups2JsonString()
	specialGroups := ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup
	originalSpecialGroups := specialGroups.ReadAll()
	testContext.Cleanup(func() {
		model.DB, model.LOG_DB = originalDB, originalLogDB
		common.SetDatabaseTypes(originalMainType, originalLogType)
		common.RedisEnabled = originalRedisEnabled
		gin.SetMode(originalGinMode)
		require.NoError(testContext, ratio_setting.UpdateGroupRatioByJSONString(originalGroups))
		require.NoError(testContext, ratio_setting.UpdateGroupGroupRatioByJSONString(originalGroupGroups))
		require.NoError(testContext, ratio_setting.UpdateGroupModelRatioByJSONString(originalGroupModels))
		require.NoError(testContext, setting.UpdateUserUsableGroupsByJSONString(originalUsableGroups))
		require.NoError(testContext, setting.UpdateAutoGroupsByJsonString(originalAutoGroups))
		specialGroups.Clear()
		specialGroups.AddAll(originalSpecialGroups)
		model.InvalidatePricingCache()
	})

	db := setupModelListControllerTestDB(testContext)
	require.NoError(testContext, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"vip":2,"hidden":3}`))
	require.NoError(testContext, ratio_setting.UpdateGroupGroupRatioByJSONString(`{}`))
	require.NoError(testContext, ratio_setting.UpdateGroupModelRatioByJSONString(`{
		"vip":{"zz-pricing-token":0.3,"zz-pricing-free":0},
		"hidden":{"zz-pricing-token":0.1}
	}`))
	require.NoError(testContext, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default","vip":"VIP"}`))
	require.NoError(testContext, setting.UpdateAutoGroupsByJsonString(`["default","vip","hidden"]`))
	specialGroups.Clear()
	require.NoError(testContext, db.Create(&model.Channel{Id: 1, Type: constant.ChannelTypeOpenAI, Status: common.ChannelStatusEnabled}).Error)
	require.NoError(testContext, db.Create(&[]model.Ability{
		{Group: "vip", Model: "zz-pricing-token", ChannelId: 1, Enabled: true},
		{Group: "vip", Model: "zz-pricing-free", ChannelId: 1, Enabled: true},
		{Group: "default", Model: "zz-pricing-unconfigured", ChannelId: 1, Enabled: true},
	}).Error)
	model.InvalidatePricingCache()
}

func requestPricing(testContext *testing.T, userID int) pricingResponse {
	testContext.Helper()

	router := gin.New()
	router.GET("/api/pricing", func(context *gin.Context) {
		if userID != 0 {
			context.Set("id", userID)
		}
	}, GetPricing)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/pricing", nil))
	require.Equal(testContext, http.StatusOK, response.Code)
	assert.Contains(testContext, response.Header().Get("Content-Type"), "application/json")
	var payload pricingResponse
	require.NoError(testContext, common.Unmarshal(response.Body.Bytes(), &payload))
	require.True(testContext, payload.Success)
	return payload
}

func TestGetPricingPublishesVisibleModelOverrides(testContext *testing.T) {
	setupPricingControllerTest(testContext)

	payload := requestPricing(testContext, 0)
	assert.Equal(testContext, map[string]map[string]float64{
		"vip": {"zz-pricing-token": 0.3, "zz-pricing-free": 0},
	}, payload.GroupModelRatio)
	assert.Equal(testContext, map[string]float64{"default": 1, "vip": 2}, payload.GroupRatio)
	assert.Equal(testContext, map[string]string{"default": "Default", "vip": "VIP"}, payload.UsableGroup)
	assert.Equal(testContext, []string{"default", "vip"}, payload.AutoGroups)
	assert.Equal(testContext, "a42d372ccf0b5dd13ecf71203521f9d2", payload.PricingVersion)
	assert.Len(testContext, payload.Data, 3)
}

func TestGetPricingScopesOverridesToCallerAndEnabledModels(testContext *testing.T) {
	setupPricingControllerTest(testContext)
	require.NoError(testContext, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"vip":2,"hidden":3,"member":4}`))
	require.NoError(testContext, ratio_setting.UpdateGroupGroupRatioByJSONString(`{"member":{"default":0.8,"hidden":0.9,"member":1.2}}`))
	require.NoError(testContext, ratio_setting.UpdateGroupModelRatioByJSONString(`{
		"default":{
			"zz-pricing-token":0.6,"zz-pricing-all":0.4,"gpt-4-gizmo-*":0.25,
			"gpt-4-gizmo-pricing-public":9,"zz-pricing-hidden":0.7,
			"zz-pricing-disabled":0.8,"zz-pricing-empty-group":0.9,"zz-pricing-missing":0.95
		},
		"vip":{
			"zz-pricing-token":0.3,"zz-pricing-free":0,"zz-pricing-all":0.7,
			"gemini-2.5-flash-thinking-*":0.45,"gpt-4-gizmo-*":0.55,"zz-pricing-disabled":0.66
		},
		"hidden":{"zz-pricing-token":0.1,"zz-pricing-all":0.2,"gpt-4-gizmo-*":0.15,"zz-pricing-hidden":1.5},
		"member":{"zz-pricing-all":0.6},
		"all":{"zz-pricing-all":0.01}
	}`))
	ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup.Set("member", map[string]string{
		"-:vip":    "",
		"+:hidden": "Private",
	})
	require.NoError(testContext, model.DB.Create(&model.User{
		Id: 7101, Username: "pricing-member", Group: "member", Status: common.UserStatusEnabled,
	}).Error)
	require.NoError(testContext, model.DB.Create(&model.Channel{Id: 2, Type: constant.ChannelTypeOpenAI, Status: common.ChannelStatusEnabled}).Error)
	require.NoError(testContext, model.DB.Create(&[]model.Ability{
		{Group: "vip", Model: "zz-pricing-token", ChannelId: 2, Enabled: true},
		{Group: "all", Model: "zz-pricing-all", ChannelId: 1, Enabled: true},
		{Group: "default", Model: "gpt-4-gizmo-pricing-public", ChannelId: 1, Enabled: true},
		{Group: "hidden", Model: "gpt-4-gizmo-pricing-public", ChannelId: 1, Enabled: true},
		{Group: "vip", Model: "gpt-4-gizmo-pricing-public", ChannelId: 1, Enabled: false},
		{Group: "vip", Model: "gemini-2.5-flash-thinking-128", ChannelId: 1, Enabled: true},
		{Group: "hidden", Model: "zz-pricing-hidden", ChannelId: 1, Enabled: true},
		{Group: "vip", Model: "zz-pricing-disabled", ChannelId: 1, Enabled: false},
		{Group: "all", Model: "zz-pricing-disabled", ChannelId: 1, Enabled: false},
		{Group: "", Model: "zz-pricing-empty-group", ChannelId: 1, Enabled: true},
	}).Error)
	model.InvalidatePricingCache()

	anonymous := requestPricing(testContext, 0)
	assert.Equal(testContext, map[string]map[string]float64{
		"default": {"zz-pricing-all": 0.4, "gpt-4-gizmo-pricing-public": 0.25},
		"vip": {
			"zz-pricing-token": 0.3, "zz-pricing-free": 0, "zz-pricing-all": 0.7,
			"gemini-2.5-flash-thinking-128": 0.45,
		},
	}, anonymous.GroupModelRatio)
	var anonymousModels []string
	for _, item := range anonymous.Data {
		anonymousModels = append(anonymousModels, item.ModelName)
	}
	assert.ElementsMatch(testContext, []string{
		"zz-pricing-token", "zz-pricing-free", "zz-pricing-unconfigured", "zz-pricing-all",
		"gpt-4-gizmo-pricing-public", "gemini-2.5-flash-thinking-128",
	}, anonymousModels)
	publicPricing := pricingByModelName(anonymous.Data)
	assert.ElementsMatch(testContext, []string{"default", "hidden"}, publicPricing["gpt-4-gizmo-pricing-public"].EnableGroup)
	assert.Equal(testContext, []string{"all"}, publicPricing["zz-pricing-all"].EnableGroup)
	assert.Equal(testContext, []string{"vip"}, publicPricing["zz-pricing-token"].EnableGroup)

	member := requestPricing(testContext, 7101)
	assert.Equal(testContext, map[string]map[string]float64{
		"default": {"zz-pricing-all": 0.4, "gpt-4-gizmo-pricing-public": 0.25},
		"hidden":  {"zz-pricing-all": 0.2, "gpt-4-gizmo-pricing-public": 0.15, "zz-pricing-hidden": 1.5},
		"member":  {"zz-pricing-all": 0.6},
	}, member.GroupModelRatio)
	assert.Equal(testContext, map[string]float64{"default": 0.8, "hidden": 0.9, "member": 1.2}, member.GroupRatio)
	assert.Equal(testContext, map[string]string{"default": "Default", "hidden": "Private", "member": "User group"}, member.UsableGroup)
	assert.Equal(testContext, []string{"default", "hidden"}, member.AutoGroups)
	memberPricing := pricingByModelName(member.Data)
	assert.Len(testContext, memberPricing, 4)
	assert.Equal(testContext, publicPricing["gpt-4-gizmo-pricing-public"], memberPricing["gpt-4-gizmo-pricing-public"])
	assert.Equal(testContext, publicPricing["zz-pricing-all"], memberPricing["zz-pricing-all"])
	assert.Equal(testContext, anonymous.Vendors, member.Vendors)
	assert.Equal(testContext, anonymous.SupportedEndpoint, member.SupportedEndpoint)
	assert.Equal(testContext, anonymous.PricingVersion, member.PricingVersion)
	assert.Equal(testContext, anonymous, requestPricing(testContext, 0))
}

func TestGetPricingEmptyOverridesPreserveLegacyResponse(testContext *testing.T) {
	setupPricingControllerTest(testContext)
	baseline := requestPricing(testContext, 0)
	baseline.GroupModelRatio = map[string]map[string]float64{}

	tests := []struct {
		name   string
		ratios string
	}{
		{name: "no configured overrides", ratios: `{}`},
		{name: "only hidden or unavailable overrides", ratios: `{
			"hidden":{"zz-pricing-token":0.1},
			"default":{"zz-pricing-token":0.2,"zz-pricing-missing":0.3}
		}`},
	}
	for _, test := range tests {
		testContext.Run(test.name, func(testContext *testing.T) {
			require.NoError(testContext, ratio_setting.UpdateGroupModelRatioByJSONString(test.ratios))
			payload := requestPricing(testContext, 0)
			require.NotNil(testContext, payload.GroupModelRatio)
			assert.Empty(testContext, payload.GroupModelRatio)
			assert.Equal(testContext, baseline, payload)
		})
	}
}

func TestGetPricingNoVisibleGroupsDoesNotExpandAll(testContext *testing.T) {
	setupPricingControllerTest(testContext)
	require.NoError(testContext, model.DB.Create(&model.Ability{
		Group: "all", Model: "zz-pricing-all", ChannelId: 1, Enabled: true,
	}).Error)
	require.NoError(testContext, ratio_setting.UpdateGroupModelRatioByJSONString(`{"default":{"zz-pricing-all":0.5}}`))
	model.InvalidatePricingCache()
	baseline := requestPricing(testContext, 0)
	require.NoError(testContext, setting.UpdateUserUsableGroupsByJSONString(`{}`))

	payload := requestPricing(testContext, 0)
	require.NotNil(testContext, payload.GroupModelRatio)
	assert.Empty(testContext, payload.GroupModelRatio)
	assert.Empty(testContext, payload.Data)
	assert.Empty(testContext, payload.UsableGroup)
	assert.Empty(testContext, payload.GroupRatio)
	assert.Empty(testContext, payload.AutoGroups)
	assert.Equal(testContext, baseline.Vendors, payload.Vendors)
	assert.Equal(testContext, baseline.SupportedEndpoint, payload.SupportedEndpoint)
	assert.Equal(testContext, baseline.PricingVersion, payload.PricingVersion)

	require.NoError(testContext, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default","vip":"VIP"}`))
	assert.Equal(testContext, baseline, requestPricing(testContext, 0))
}
