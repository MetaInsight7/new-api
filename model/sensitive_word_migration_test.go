package model

import (
	"fmt"
	"path/filepath"
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/request_plugin_setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func useConcurrentSensitiveWordMigrationDB(t *testing.T) *gorm.DB {
	t.Helper()
	previousDB := DB
	previousType := common.MainDatabaseType()
	dsn := fmt.Sprintf(
		"file:%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)",
		filepath.Join(t.TempDir(), "sensitive-word-migration.db"),
	)
	database, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, database.AutoMigrate(
		&Option{}, &SensitiveWordRule{}, &SensitiveWordRuleWord{},
		&SensitiveWordRuleGroup{}, &SensitiveWordPolicy{}, &SensitiveWordAuditEvent{},
	))
	DB = database
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	t.Cleanup(func() {
		DB = previousDB
		common.SetMainDatabaseType(previousType)
		sqlDB, dbErr := database.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
		invalidateSensitiveWordRuntime()
	})
	return database
}

func TestSensitiveWordMigrationSerializesNodesWithoutRewritingSources(t *testing.T) {
	database := useConcurrentSensitiveWordMigrationDB(t)
	requestPlugins := `[{"id":"rp_keep","name":"Keep","plugin":"request-redact","enabled":true,"global":true,"groups":[],"models":[],"config":{"replacement":"[X]"}}]`
	legacyOptions := []Option{
		{Key: "CheckSensitiveEnabled", Value: "true"},
		{Key: "CheckSensitiveOnPromptEnabled", Value: "true"},
		{Key: "StopOnSensitiveEnabled", Value: "false"},
		{Key: "SensitiveWords", Value: " Bad \n\nworse\r\nBAD"},
		{Key: "StreamCacheQueueLength", Value: "3"},
		{Key: request_plugin_setting.InstancesOption, Value: requestPlugins},
	}
	require.NoError(t, database.Create(&legacyOptions).Error)

	const nodes = 8
	errorsByNode := make(chan error, nodes)
	var wait sync.WaitGroup
	for range nodes {
		wait.Add(1)
		go func() {
			defer wait.Done()
			errorsByNode <- MigrateSensitiveWordData()
		}()
	}
	wait.Wait()
	close(errorsByNode)
	for migrationErr := range errorsByNode {
		require.NoError(t, migrationErr)
	}

	rules, err := ListSensitiveWordRules()
	require.NoError(t, err)
	require.Len(t, rules, 1)
	detail, err := GetSensitiveWordRuleDetail(rules[0].ID)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"Bad", "worse"}, detail.Words)

	for _, expected := range legacyOptions {
		var stored Option
		require.NoError(t, database.Where(&Option{Key: expected.Key}).First(&stored).Error)
		assert.Equal(t, expected.Value, stored.Value, expected.Key)
	}
	var marker Option
	require.NoError(t, database.Where(&Option{Key: sensitiveWordMigrationKey}).First(&marker).Error)
	assert.Equal(t, sensitiveWordMigrationValue, marker.Value)
	require.NoError(t, MigrateSensitiveWordData())
	rules, err = ListSensitiveWordRules()
	require.NoError(t, err)
	assert.Len(t, rules, 1)
}
