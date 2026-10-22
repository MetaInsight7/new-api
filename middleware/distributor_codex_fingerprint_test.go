package middleware

import (
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupCodexDistributorSeedTest(t *testing.T) *gorm.DB {
	t.Helper()
	previousDB := model.DB
	previousDatabaseType := common.MainDatabaseType()
	previousMemoryCacheEnabled := common.MemoryCacheEnabled

	database, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "codex-distributor.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, database.AutoMigrate(&model.Channel{}))
	model.DB = database
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	common.MemoryCacheEnabled = false
	t.Cleanup(func() {
		model.DB = previousDB
		common.SetMainDatabaseType(previousDatabaseType)
		common.MemoryCacheEnabled = previousMemoryCacheEnabled
	})
	return database
}

func insertCodexDistributorSeedChannel(t *testing.T, database *gorm.DB, seed string) model.Channel {
	t.Helper()
	channel := model.Channel{
		Name: "codex-distributor-seed", Type: constant.ChannelTypeCodex,
		Key: "unused-test-key", Status: common.ChannelStatusEnabled,
		CodexFingerprintSeed: seed,
	}
	channel.SetSetting(dto.ChannelSettings{CodexFingerprintMode: dto.CodexFingerprintModeFull})
	require.NoError(t, database.Create(&channel).Error)
	return channel
}

func TestSetupContextForSelectedCodexChannelUsesCachedFingerprintSeed(t *testing.T) {
	database := setupCodexDistributorSeedTest(t)
	const seed = "2e9bce53-8850-4ae9-8c70-a74d9cecb893"
	channel := insertCodexDistributorSeedChannel(t, database, seed)

	var queryCount atomic.Int64
	const callbackName = "test:count-codex-seed-queries"
	require.NoError(t, database.Callback().Query().Before("gorm:query").Register(callbackName, func(*gorm.DB) {
		queryCount.Add(1)
	}))
	t.Cleanup(func() { require.NoError(t, database.Callback().Query().Remove(callbackName)) })

	context, _ := gin.CreateTestContext(nil)
	require.Nil(t, SetupContextForSelectedChannel(context, &channel, "gpt-5-codex"))
	assert.Equal(t, seed, common.GetContextKeyString(context, constant.ContextKeyChannelCodexFingerprintSeed))
	assert.Zero(t, queryCount.Load(), "a usable seed already loaded with the channel must stay on the in-memory request path")
}

func TestSetupContextForSelectedCodexChannelRepairsMissingFingerprintSeed(t *testing.T) {
	database := setupCodexDistributorSeedTest(t)
	channel := insertCodexDistributorSeedChannel(t, database, "")

	context, _ := gin.CreateTestContext(nil)
	require.Nil(t, SetupContextForSelectedChannel(context, &channel, "gpt-5-codex"))
	seed := common.GetContextKeyString(context, constant.ContextKeyChannelCodexFingerprintSeed)
	_, err := uuid.Parse(seed)
	require.NoError(t, err)

	var stored model.Channel
	require.NoError(t, database.First(&stored, channel.Id).Error)
	assert.Equal(t, seed, stored.CodexFingerprintSeed)
}
