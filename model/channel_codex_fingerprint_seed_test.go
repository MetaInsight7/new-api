package model

import (
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupCodexFingerprintSeedTestDB(t *testing.T) {
	t.Helper()
	originalDB := DB
	originalMainDatabaseType := common.MainDatabaseType()
	originalLogDatabaseType := common.LogDatabaseType()

	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/codex-fingerprint.db?_pragma=busy_timeout(5000)"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(8)
	require.NoError(t, db.AutoMigrate(&Channel{}, &Ability{}))

	DB = db
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	t.Cleanup(func() {
		DB = originalDB
		common.SetDatabaseTypes(originalMainDatabaseType, originalLogDatabaseType)
		require.NoError(t, sqlDB.Close())
	})
}

func insertFingerprintSeedTestChannel(t *testing.T, channelType, status int, seed string) Channel {
	t.Helper()
	channel := Channel{
		Type: channelType, Key: `{"access_token":"at","account_id":"acct"}`,
		Name: "fingerprint-test", Status: status, Models: "gpt-5-codex", Group: "default",
		CodexFingerprintSeed: seed,
	}
	require.NoError(t, DB.Create(&channel).Error)
	return channel
}

func requireFingerprintUUID(t *testing.T, value string) {
	t.Helper()
	_, err := uuid.Parse(value)
	require.NoError(t, err)
}

func TestEnsureCodexFingerprintSeedPreservesAndRepairs(t *testing.T) {
	setupCodexFingerprintSeedTestDB(t)
	channel := insertFingerprintSeedTestChannel(t, constant.ChannelTypeCodex, common.ChannelStatusEnabled, "")

	first, err := EnsureCodexFingerprintSeed(channel.Id)
	require.NoError(t, err)
	requireFingerprintUUID(t, first)
	second, err := EnsureCodexFingerprintSeed(channel.Id)
	require.NoError(t, err)
	require.Equal(t, first, second)

	require.NoError(t, DB.Model(&Channel{}).Where("id = ?", channel.Id).Update("codex_fingerprint_seed", "invalid").Error)
	repaired, err := EnsureCodexFingerprintSeed(channel.Id)
	require.NoError(t, err)
	requireFingerprintUUID(t, repaired)
	require.NotEqual(t, "invalid", repaired)
}

func TestEnsureCodexFingerprintSeedConvergesConcurrentCallers(t *testing.T) {
	setupCodexFingerprintSeedTestDB(t)
	channel := insertFingerprintSeedTestChannel(t, constant.ChannelTypeCodex, common.ChannelStatusEnabled, "")
	const callers = 8
	results := make(chan string, callers)
	errors := make(chan error, callers)
	var waitGroup sync.WaitGroup
	waitGroup.Add(callers)
	for i := 0; i < callers; i++ {
		go func() {
			defer waitGroup.Done()
			seed, err := EnsureCodexFingerprintSeed(channel.Id)
			if err != nil {
				errors <- err
				return
			}
			results <- seed
		}()
	}
	waitGroup.Wait()
	close(results)
	close(errors)
	for err := range errors {
		require.NoError(t, err)
	}

	var observed string
	for seed := range results {
		requireFingerprintUUID(t, seed)
		if observed == "" {
			observed = seed
		} else {
			require.Equal(t, observed, seed)
		}
	}
	require.NotEmpty(t, observed)
}

func TestCodexFingerprintSeedLifecycleDefaultsAndIsolation(t *testing.T) {
	setupCodexFingerprintSeedTestDB(t)
	enabled := insertFingerprintSeedTestChannel(t, constant.ChannelTypeCodex, common.ChannelStatusEnabled, "")
	disabled := insertFingerprintSeedTestChannel(t, constant.ChannelTypeCodex, common.ChannelStatusManuallyDisabled, "")
	nonCodex := insertFingerprintSeedTestChannel(t, constant.ChannelTypeOpenAI, common.ChannelStatusEnabled, "")

	require.NoError(t, enabled.Update())
	var stored Channel
	require.NoError(t, DB.First(&stored, enabled.Id).Error)
	requireFingerprintUUID(t, stored.CodexFingerprintSeed)
	require.Empty(t, disabled.CodexFingerprintSeed)
	require.Empty(t, nonCodex.CodexFingerprintSeed)

	mode := "session"
	setting := stored.GetSetting()
	setting.CodexFingerprintMode = mode
	stored.SetSetting(setting)
	require.NoError(t, stored.Update())
	var updated Channel
	require.NoError(t, DB.First(&updated, stored.Id).Error)
	require.Equal(t, mode, updated.GetSetting().CodexFingerprintMode)
	require.Equal(t, stored.CodexFingerprintSeed, updated.CodexFingerprintSeed)

	updated.Type = constant.ChannelTypeOpenAI
	require.NoError(t, updated.Update())
	var converted Channel
	require.NoError(t, DB.First(&converted, stored.Id).Error)
	require.Empty(t, converted.CodexFingerprintSeed)
}

func TestUpdateCodexFingerprintModeByIdsIsAtomicAndPreservesSettings(t *testing.T) {
	setupCodexFingerprintSeedTestDB(t)
	first := insertFingerprintSeedTestChannel(t, constant.ChannelTypeCodex, common.ChannelStatusEnabled, "")
	first.SetSetting(dto.ChannelSettings{Proxy: "http://proxy.invalid", CodexFingerprintMode: "off"})
	require.NoError(t, DB.Model(&first).Updates(first).Error)
	second := insertFingerprintSeedTestChannel(t, constant.ChannelTypeOpenAI, common.ChannelStatusEnabled, "")

	updatedCount, err := UpdateCodexFingerprintModeByIds([]int{first.Id, second.Id}, "session")
	require.NoError(t, err)
	require.Equal(t, 1, updatedCount)
	var updated Channel
	require.NoError(t, DB.First(&updated, first.Id).Error)
	require.Equal(t, "session", updated.GetSetting().CodexFingerprintMode)
	require.Equal(t, "http://proxy.invalid", updated.GetSetting().Proxy)
	require.NotEmpty(t, updated.CodexFingerprintSeed)

	var untouched Channel
	require.NoError(t, DB.First(&untouched, second.Id).Error)
	require.Empty(t, untouched.GetSetting().CodexFingerprintMode)
}
