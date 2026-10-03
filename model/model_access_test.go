package model

import (
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupModelAccessTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	originalDB := DB
	originalGroupCol := commonGroupCol

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	DB = db
	commonGroupCol = "`group`"
	require.NoError(t, db.AutoMigrate(&Channel{}, &Ability{}, &Model{}, &Vendor{}))

	t.Cleanup(func() {
		DB = originalDB
		commonGroupCol = originalGroupCol
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func TestGetModelAccessRowsForGroupsFiltersDisabledRoutes(t *testing.T) {
	db := setupModelAccessTestDB(t)
	priority := int64(0)
	weight := uint(100)
	require.NoError(t, db.Create(&[]Channel{
		{Id: 1, Type: constant.ChannelTypeOpenAI, Status: common.ChannelStatusEnabled, Key: "enabled", Models: "allowed", Group: "default", Priority: &priority, Weight: &weight},
		{Id: 2, Type: constant.ChannelTypeAnthropic, Status: common.ChannelStatusManuallyDisabled, Key: "disabled", Models: "disabled-channel", Group: "default", Priority: &priority, Weight: &weight},
	}).Error)
	require.NoError(t, db.Create(&[]Ability{
		{Group: "default", Model: "allowed", ChannelId: 1, Enabled: true, Priority: &priority, Weight: weight},
		{Group: "default", Model: "disabled-ability", ChannelId: 1, Enabled: false, Priority: &priority, Weight: weight},
		{Group: "default", Model: "disabled-channel", ChannelId: 2, Enabled: true, Priority: &priority, Weight: weight},
	}).Error)

	rows, err := GetModelAccessRowsForGroups([]string{" default ", "default", ""})
	require.NoError(t, err)
	require.Equal(t, []ModelAccessRow{{GroupName: "default", Model: "allowed", ChannelType: constant.ChannelTypeOpenAI}}, rows)
}

func TestGetPublicModelMetadataMapPrefersStoredMetadataAndInfersWithoutWriting(t *testing.T) {
	db := setupModelAccessTestDB(t)
	exactVendor := Vendor{Name: "Exact Vendor", Icon: "exact", Status: 1}
	ruleVendor := Vendor{Name: "Rule Vendor", Icon: "rule", Status: 1}
	require.NoError(t, db.Create(&exactVendor).Error)
	require.NoError(t, db.Create(&ruleVendor).Error)
	require.NoError(t, db.Create(&[]Model{
		{ModelName: "claude-exact", VendorID: exactVendor.Id, Status: 1, NameRule: NameRuleExact},
		{ModelName: "claude-", VendorID: ruleVendor.Id, Status: 1, NameRule: NameRulePrefix},
	}).Error)

	metadata, err := GetPublicModelMetadataMap([]string{"claude-exact", "claude-rule-child", "gemini-inferred-child", "unrecognized-model"})
	require.NoError(t, err)
	require.Equal(t, "Exact Vendor", metadata["claude-exact"].Vendor.Name)
	require.Equal(t, "Rule Vendor", metadata["claude-rule-child"].Vendor.Name)
	require.Equal(t, "Google", metadata["gemini-inferred-child"].Vendor.Name)
	require.Equal(t, "Gemini.Color", metadata["gemini-inferred-child"].Vendor.Icon)
	_, exists := metadata["unrecognized-model"]
	require.False(t, exists)

	var modelCount int64
	var vendorCount int64
	require.NoError(t, db.Model(&Model{}).Count(&modelCount).Error)
	require.NoError(t, db.Model(&Vendor{}).Count(&vendorCount).Error)
	require.EqualValues(t, 2, modelCount)
	require.EqualValues(t, 2, vendorCount)
}

func TestGetPublicModelMetadataMapInfersDefaultVendorForUnconfiguredModel(t *testing.T) {
	db := setupModelAccessTestDB(t)

	metadata, err := GetPublicModelMetadataMap([]string{"claude-opus-4-6", "unrecognized-model"})
	require.NoError(t, err)
	require.Equal(t, "Anthropic", metadata["claude-opus-4-6"].Vendor.Name)
	require.Equal(t, "Claude.Color", metadata["claude-opus-4-6"].Vendor.Icon)
	_, exists := metadata["unrecognized-model"]
	require.False(t, exists)

	var modelCount int64
	var vendorCount int64
	require.NoError(t, db.Model(&Model{}).Count(&modelCount).Error)
	require.NoError(t, db.Model(&Vendor{}).Count(&vendorCount).Error)
	require.Zero(t, modelCount)
	require.Zero(t, vendorCount)
}
