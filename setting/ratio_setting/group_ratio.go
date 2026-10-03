package ratio_setting

import (
	"encoding/json"
	"errors"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/types"
)

var defaultGroupRatio = map[string]float64{
	"default": 1,
	"vip":     1,
	"svip":    1,
}

var groupRatioMap = types.NewRWMap[string, float64]()

var defaultGroupGroupRatio = map[string]map[string]float64{
	"vip": {
		"edit_this": 0.9,
	},
}

var groupGroupRatioMap = types.NewRWMap[string, map[string]float64]()

var groupModelRatioMap = types.NewRWMap[string, map[string]float64]()

var defaultGroupSpecialUsableGroup = map[string]map[string]string{}

type GroupRatioSetting struct {
	GroupRatio              *types.RWMap[string, float64]            `json:"group_ratio"`
	GroupGroupRatio         *types.RWMap[string, map[string]float64] `json:"group_group_ratio"`
	GroupModelRatio         *types.RWMap[string, map[string]float64] `json:"group_model_ratio"`
	GroupSpecialUsableGroup *types.RWMap[string, map[string]string]  `json:"group_special_usable_group"`
}

var groupRatioSetting GroupRatioSetting

func init() {
	groupSpecialUsableGroup := types.NewRWMap[string, map[string]string]()
	groupSpecialUsableGroup.AddAll(defaultGroupSpecialUsableGroup)

	groupRatioMap.AddAll(defaultGroupRatio)
	groupGroupRatioMap.AddAll(defaultGroupGroupRatio)

	groupRatioSetting = GroupRatioSetting{
		GroupSpecialUsableGroup: groupSpecialUsableGroup,
		GroupRatio:              groupRatioMap,
		GroupGroupRatio:         groupGroupRatioMap,
		GroupModelRatio:         groupModelRatioMap,
	}

	config.GlobalConfig.Register("group_ratio_setting", &groupRatioSetting)
}

func GetGroupRatioSetting() *GroupRatioSetting {
	if groupRatioSetting.GroupSpecialUsableGroup == nil {
		groupRatioSetting.GroupSpecialUsableGroup = types.NewRWMap[string, map[string]string]()
		groupRatioSetting.GroupSpecialUsableGroup.AddAll(defaultGroupSpecialUsableGroup)
	}
	if groupRatioSetting.GroupModelRatio == nil {
		groupRatioSetting.GroupModelRatio = groupModelRatioMap
	}
	return &groupRatioSetting
}

func GetGroupRatioCopy() map[string]float64 {
	return groupRatioMap.ReadAll()
}

func ContainsGroupRatio(name string) bool {
	_, ok := groupRatioMap.Get(name)
	return ok
}

func GroupRatio2JSONString() string {
	return groupRatioMap.MarshalJSONString()
}

func UpdateGroupRatioByJSONString(jsonStr string) error {
	return types.LoadFromJsonString(groupRatioMap, jsonStr)
}

func GetGroupRatio(name string) float64 {
	ratio, ok := groupRatioMap.Get(name)
	if !ok {
		common.SysLog("group ratio not found: " + name)
		return 1
	}
	return ratio
}

func GetGroupGroupRatio(userGroup, usingGroup string) (float64, bool) {
	gp, ok := groupGroupRatioMap.Get(userGroup)
	if !ok {
		return -1, false
	}
	ratio, ok := gp[usingGroup]
	if !ok {
		return -1, false
	}
	return ratio, true
}

func GroupGroupRatio2JSONString() string {
	return groupGroupRatioMap.MarshalJSONString()
}

func UpdateGroupGroupRatioByJSONString(jsonStr string) error {
	return types.LoadFromJsonString(groupGroupRatioMap, jsonStr)
}

func GetGroupModelRatio(groupName, modelName string) (float64, bool, string) {
	models, ok := groupModelRatioMap.Get(groupName)
	if !ok {
		return -1, false, ""
	}
	matchedModel := FormatMatchingModelName(modelName)
	ratio, ok := models[matchedModel]
	if !ok {
		return -1, false, matchedModel
	}
	return ratio, true, matchedModel
}

func GetGroupModelRatioCopy() map[string]map[string]float64 {
	source := groupModelRatioMap.ReadAll()
	copy := make(map[string]map[string]float64, len(source))
	for group, ratios := range source {
		copy[group] = make(map[string]float64, len(ratios))
		for modelName, ratio := range ratios {
			copy[group][modelName] = ratio
		}
	}
	return copy
}

func GroupModelRatio2JSONString() string {
	return groupModelRatioMap.MarshalJSONString()
}

func UpdateGroupModelRatioByJSONString(jsonStr string) error {
	return types.LoadFromJsonString(groupModelRatioMap, jsonStr)
}

func GetEffectiveGroupRatio(userGroup, usingGroup, modelName string) types.GroupRatioInfo {
	info := types.GroupRatioInfo{GroupRatio: 1, GroupSpecialRatio: -1}
	if ratio, ok, matchedModel := GetGroupModelRatio(usingGroup, modelName); ok {
		info.GroupRatio = ratio
		info.GroupModelRatio = ratio
		info.HasGroupModelRatio = true
		info.GroupModelRatioGroup = usingGroup
		info.GroupModelRatioModel = matchedModel
		return info
	}
	if ratio, ok := GetGroupGroupRatio(userGroup, usingGroup); ok {
		info.GroupRatio = ratio
		info.GroupSpecialRatio = ratio
		info.HasSpecialRatio = true
		return info
	}
	info.GroupRatio = GetGroupRatio(usingGroup)
	return info
}

func CheckGroupRatio(jsonStr string) error {
	checkGroupRatio := make(map[string]float64)
	err := json.Unmarshal([]byte(jsonStr), &checkGroupRatio)
	if err != nil {
		return err
	}
	for name, ratio := range checkGroupRatio {
		if ratio < 0 {
			return errors.New("group ratio must be not less than 0: " + name)
		}
	}
	return nil
}

func CheckGroupModelRatio(jsonStr string) error {
	checkGroupModelRatio := make(map[string]map[string]float64)
	if err := common.UnmarshalJsonStr(jsonStr, &checkGroupModelRatio); err != nil {
		return err
	}
	for groupName, modelRatios := range checkGroupModelRatio {
		for modelName, ratio := range modelRatios {
			if ratio < 0 {
				return errors.New("group model ratio must be not less than 0: " + groupName + "/" + modelName)
			}
		}
	}
	return nil
}
