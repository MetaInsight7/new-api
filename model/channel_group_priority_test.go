package model

import (
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupChannelGroupPriorityTestDB(t *testing.T, memoryCacheEnabled bool) *gorm.DB {
	t.Helper()

	originalDB := DB
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	originalGroups := group2model2channels
	originalChannels := channelsIDM
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	DB = db
	common.MemoryCacheEnabled = memoryCacheEnabled
	require.NoError(t, db.AutoMigrate(&Channel{}, &Ability{}))

	t.Cleanup(func() {
		DB = originalDB
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
		channelSyncLock.Lock()
		group2model2channels = originalGroups
		channelsIDM = originalChannels
		channelSyncLock.Unlock()
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func TestChannelGroupPriorityParsingAndFallback(t *testing.T) {
	globalPriority := int64(7)
	globalWeight := uint(11)
	channel := &Channel{
		Priority:      &globalPriority,
		Weight:        &globalWeight,
		GroupPriority: common.GetPointer(`{"vip":{"priority":-3,"weight":0},"unknown":{"priority":9}}`),
	}

	overrides, err := channel.ParseGroupPriorityMap()
	require.NoError(t, err)
	require.Equal(t, int64(-3), *overrides["vip"].Priority)
	require.Equal(t, uint(0), *overrides["vip"].Weight)
	assert.Equal(t, int64(9), channel.GetPriorityForGroup("unknown"))
	assert.Equal(t, globalPriority, channel.GetPriorityForGroup("default"))
	assert.Equal(t, uint64(0), channel.GetWeightForGroup("vip"))
	assert.Equal(t, uint64(globalWeight), channel.GetWeightForGroup("default"))

	channel.GroupPriority = common.GetPointer(`{"vip":{"weight":-1}}`)
	assert.Error(t, channel.ValidateGroupPriority())
	assert.Equal(t, globalPriority, channel.GetPriorityForGroup("vip"))
	assert.Equal(t, uint64(globalWeight), channel.GetWeightForGroup("vip"))

	for _, raw := range []string{
		`null`,
		`{"vip":{"unknown":1}}`,
		`{"vip":{"priority":1}} trailing`,
	} {
		channel.GroupPriority = common.GetPointer(raw)
		assert.Error(t, channel.ValidateGroupPriority(), raw)
		assert.Equal(t, globalPriority, channel.GetPriorityForGroup("vip"), raw)
	}
}

func TestGroupPriorityRoutingUsesEffectiveValuesOnBothCachePaths(t *testing.T) {
	for _, memoryCacheEnabled := range []bool{false, true} {
		t.Run(fmt.Sprintf("memory_cache_%t", memoryCacheEnabled), func(t *testing.T) {
			db := setupChannelGroupPriorityTestDB(t, memoryCacheEnabled)
			modelName := "group-priority-routing-model"
			priorityOne := int64(10)
			priorityTwo := int64(0)
			weight := uint(1)
			overrides := common.GetPointer(`{"vip":{"priority":20,"weight":30}}`)
			channels := []Channel{
				{Id: 1, Type: constant.ChannelTypeOpenAI, Key: "key-1", Status: common.ChannelStatusEnabled, Models: modelName, Group: "default,vip", Priority: &priorityOne, Weight: &weight},
				{Id: 2, Type: constant.ChannelTypeOpenAI, Key: "key-2", Status: common.ChannelStatusEnabled, Models: modelName, Group: "default,vip", Priority: &priorityTwo, Weight: &weight, GroupPriority: overrides},
			}
			require.NoError(t, db.Create(&channels).Error)
			require.NoError(t, db.Create(&[]Ability{
				{Group: "default", Model: modelName, ChannelId: 1, Enabled: true, Priority: &priorityOne, Weight: weight},
				{Group: "default", Model: modelName, ChannelId: 2, Enabled: true, Priority: &priorityTwo, Weight: weight},
				{Group: "vip", Model: modelName, ChannelId: 1, Enabled: true, Priority: &priorityOne, Weight: weight},
				{Group: "vip", Model: modelName, ChannelId: 2, Enabled: true, Priority: &priorityTwo, Weight: weight},
			}).Error)
			if memoryCacheEnabled {
				InitChannelCache()
			}

			defaultChannel, err := GetRandomSatisfiedChannel("default", modelName, 0, nil)
			require.NoError(t, err)
			require.NotNil(t, defaultChannel)
			assert.Equal(t, 1, defaultChannel.Id)

			vipChannel, err := GetRandomSatisfiedChannel("vip", modelName, 0, nil)
			require.NoError(t, err)
			require.NotNil(t, vipChannel)
			assert.Equal(t, 2, vipChannel.Id)

			candidates, err := GetSatisfiedChannelCandidates("vip", modelName, 0, nil)
			require.NoError(t, err)
			require.Len(t, candidates, 1)
			assert.Equal(t, 2, candidates[0].Id)
		})
	}
}

func TestModelAvailabilityProbeUsesGlobalChannelPriorityAndWeight(t *testing.T) {
	db := setupChannelGroupPriorityTestDB(t, false)
	modelName := "probe-global-priority-model"
	globalPriorityOne := int64(10)
	globalPriorityTwo := int64(0)
	globalWeightOne := uint(1)
	globalWeightTwo := uint(2)
	require.NoError(t, db.Create(&[]Channel{
		{Id: 11, Type: constant.ChannelTypeOpenAI, Key: "key-11", Status: common.ChannelStatusEnabled, Name: "global-first", Models: modelName, Group: "default", Priority: &globalPriorityOne, Weight: &globalWeightOne, GroupPriority: common.GetPointer(`{"default":{"priority":-100,"weight":0}}`)},
		{Id: 12, Type: constant.ChannelTypeOpenAI, Key: "key-12", Status: common.ChannelStatusEnabled, Name: "global-second", Models: modelName, Group: "default", Priority: &globalPriorityTwo, Weight: &globalWeightTwo, GroupPriority: common.GetPointer(`{"default":{"priority":100,"weight":100}}`)},
	}).Error)
	abilityPriorityOne := int64(0)
	abilityPriorityTwo := int64(20)
	require.NoError(t, db.Create(&[]Ability{
		{Group: "default", Model: modelName, ChannelId: 11, Enabled: true, Priority: &abilityPriorityOne, Weight: 100},
		{Group: "default", Model: modelName, ChannelId: 12, Enabled: true, Priority: &abilityPriorityTwo, Weight: 1},
	}).Error)

	targets, err := GetModelAvailabilityProbeTargets(modelName)
	require.NoError(t, err)
	require.Len(t, targets, 2)
	assert.Equal(t, 11, targets[0].ChannelID)
	assert.Equal(t, 12, targets[1].ChannelID)
}

func TestGroupPriorityWeightOverflowFailsSafely(t *testing.T) {
	for _, memoryCacheEnabled := range []bool{false, true} {
		t.Run(fmt.Sprintf("memory_cache_%t", memoryCacheEnabled), func(t *testing.T) {
			db := setupChannelGroupPriorityTestDB(t, memoryCacheEnabled)
			modelName := "group-priority-overflow-model"
			priority := int64(0)
			weight := uint(1)
			overflowWeight := fmt.Sprintf(`{"default":{"weight":%d}}`, ^uint(0))
			require.NoError(t, db.Create(&[]Channel{
				{Id: 31, Type: constant.ChannelTypeOpenAI, Key: "key-31", Status: common.ChannelStatusEnabled, Models: modelName, Group: "default", Priority: &priority, Weight: &weight, GroupPriority: &overflowWeight},
				{Id: 32, Type: constant.ChannelTypeOpenAI, Key: "key-32", Status: common.ChannelStatusEnabled, Models: modelName, Group: "default", Priority: &priority, Weight: &weight},
			}).Error)
			require.NoError(t, db.Create(&[]Ability{
				{Group: "default", Model: modelName, ChannelId: 31, Enabled: true, Priority: &priority, Weight: weight},
				{Group: "default", Model: modelName, ChannelId: 32, Enabled: true, Priority: &priority, Weight: weight},
			}).Error)
			if memoryCacheEnabled {
				InitChannelCache()
			}

			_, err := GetRandomSatisfiedChannel("default", modelName, 0, nil)
			require.ErrorContains(t, err, "channel weight overflow")
		})
	}
}
