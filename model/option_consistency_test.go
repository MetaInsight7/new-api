package model

import (
	"errors"
	"maps"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupOptionConsistencyTest(t *testing.T) *gorm.DB {
	t.Helper()
	database, err := gorm.Open(sqlite.Open(t.TempDir()+"/options.db?_pragma=busy_timeout(5000)"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := database.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(4)
	require.NoError(t, database.AutoMigrate(&Option{}, &SystemTask{}))

	previousDB, previousType := DB, common.MainDatabaseType()
	previousSnapshot := CurrentRequestPolicy()
	previousStreaming := operation_setting.GetStreamingSetting()
	common.OptionMapRWMutex.Lock()
	previousOptions := maps.Clone(common.OptionMap)
	common.OptionMap = maps.Clone(previousSnapshot.Options)
	common.OptionMapRWMutex.Unlock()
	optionRuntimeMutationMutex.Lock()
	previousRuntimeRevision := optionRuntimeRevision
	streamingOptionMutex.Lock()
	previousStreamingRevision := streamingOptionRevision
	streamingOptionMutex.Unlock()
	optionRuntimeMutationMutex.Unlock()
	DB = database
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	initCol()

	t.Cleanup(func() {
		requestPolicyOptionMutex.Lock()
		for key, value := range previousSnapshot.Options {
			require.NoError(t, updateOptionMap(key, value))
		}
		requestPolicySnapshot.Store(previousSnapshot)
		requestPolicyOptionMutex.Unlock()
		require.NoError(t, applyStreamingOptions(map[string]string{
			"streaming_setting.first_response_timeout_enabled": strconv.FormatBool(previousStreaming.FirstResponseTimeoutEnabled),
			"streaming_setting.first_response_timeout_seconds": strconv.FormatInt(previousStreaming.FirstResponseTimeoutSeconds, 10),
		}))
		common.OptionMapRWMutex.Lock()
		common.OptionMap = previousOptions
		common.OptionMapRWMutex.Unlock()
		optionRuntimeMutationMutex.Lock()
		optionRuntimeRevision = previousRuntimeRevision
		streamingOptionMutex.Lock()
		streamingOptionRevision = previousStreamingRevision
		streamingOptionMutex.Unlock()
		optionRuntimeMutationMutex.Unlock()
		DB = previousDB
		common.SetMainDatabaseType(previousType)
		initCol()
		require.NoError(t, sqlDB.Close())
	})
	return database
}

func TestLoadOptionsFromDatabaseReadsOnceAndKeepsRuntimeOnReadFailure(t *testing.T) {
	database := setupOptionConsistencyTest(t)
	require.NoError(t, updateOptionMap("RetryTimes", "7"))
	snapshot, err := BuildRequestPolicy(map[string]string{"RetryTimes": "7"})
	require.NoError(t, err)
	requestPolicySnapshot.Store(snapshot)
	require.NoError(t, database.Create(&Option{Key: "RetryTimes", Value: "2"}).Error)

	var queries atomic.Int32
	require.NoError(t, database.Callback().Query().Before("gorm:query").Register("option_read_failure", func(tx *gorm.DB) {
		if tx.Statement.Table == "options" && queries.Add(1) == 1 {
			tx.AddError(errors.New("options unavailable"))
		}
	}))
	err = LoadOptionsFromDatabase()
	require.ErrorContains(t, err, "options unavailable")
	assert.EqualValues(t, 1, queries.Load())
	assert.Same(t, snapshot, CurrentRequestPolicy())
	assert.Equal(t, 7, common.RetryTimes)
	require.NoError(t, database.Callback().Query().Remove("option_read_failure"))

	queries.Store(0)
	require.NoError(t, database.Callback().Query().Before("gorm:query").Register("option_read_count", func(tx *gorm.DB) {
		if tx.Statement.Table == "options" {
			queries.Add(1)
		}
	}))
	require.NoError(t, LoadOptionsFromDatabase())
	assert.EqualValues(t, 1, queries.Load())
	assert.Equal(t, 2, common.RetryTimes)
	require.NoError(t, database.Callback().Query().Remove("option_read_count"))
}

func TestStaleOptionLoadCannotOverwriteCommittedUpdate(t *testing.T) {
	database := setupOptionConsistencyTest(t)
	require.NoError(t, database.Create(&Option{Key: "RetryTimes", Value: "1"}).Error)

	queried := make(chan struct{})
	release := make(chan struct{})
	var intercepted atomic.Bool
	require.NoError(t, database.Callback().Query().After("gorm:query").Register("option_stale_load", func(tx *gorm.DB) {
		if tx.Statement.Table == "options" && intercepted.CompareAndSwap(false, true) {
			close(queried)
			<-release
		}
	}))
	loadDone := make(chan error, 1)
	go func() { loadDone <- LoadOptionsFromDatabase() }()
	select {
	case <-queried:
	case <-time.After(5 * time.Second):
		require.FailNow(t, "option load did not reach the database callback")
	}
	require.NoError(t, UpdateRequestPolicyOptions(map[string]string{"RetryTimes": "3"}))
	close(release)
	require.NoError(t, <-loadDone)
	require.NoError(t, database.Callback().Query().Remove("option_stale_load"))

	assert.Equal(t, 3, common.RetryTimes)
	assert.Equal(t, 3, CurrentRequestPolicy().RetryTimes)
	var stored Option
	require.NoError(t, database.First(&stored, "key = ?", "RetryTimes").Error)
	assert.Equal(t, "3", stored.Value)
}

func TestOptionWriteFailureDoesNotMutateRuntimeOrSync(t *testing.T) {
	database := setupOptionConsistencyTest(t)
	require.NoError(t, database.Create(&Option{Key: "RetryTimes", Value: "2"}).Error)
	require.NoError(t, LoadOptionsFromDatabase())
	snapshot := CurrentRequestPolicy()
	optionRuntimeMutationMutex.Lock()
	revision := optionRuntimeRevision
	optionRuntimeMutationMutex.Unlock()

	var before int64
	require.NoError(t, database.Model(&SystemTask{}).Where("type = ?", SyncTaskType(SyncTopicOptions)).Count(&before).Error)
	require.NoError(t, database.Callback().Update().Before("gorm:update").Register("option_write_failure", func(tx *gorm.DB) {
		if tx.Statement.Table == "options" {
			tx.AddError(errors.New("write rejected"))
		}
	}))
	err := UpdateRequestPolicyOptions(map[string]string{"RetryTimes": "8"})
	require.ErrorContains(t, err, "write rejected")
	require.NoError(t, database.Callback().Update().Remove("option_write_failure"))

	assert.Same(t, snapshot, CurrentRequestPolicy())
	assert.Equal(t, 2, common.RetryTimes)
	optionRuntimeMutationMutex.Lock()
	assert.Equal(t, revision, optionRuntimeRevision)
	optionRuntimeMutationMutex.Unlock()
	var after int64
	require.NoError(t, database.Model(&SystemTask{}).Where("type = ?", SyncTaskType(SyncTopicOptions)).Count(&after).Error)
	assert.Equal(t, before, after)
}

func TestBulkOptionUpdateAdvancesRuntimeAndSyncOnce(t *testing.T) {
	database := setupOptionConsistencyTest(t)
	optionRuntimeMutationMutex.Lock()
	runtimeBefore := optionRuntimeRevision
	streamingOptionMutex.Lock()
	streamingBefore := streamingOptionRevision
	streamingOptionMutex.Unlock()
	optionRuntimeMutationMutex.Unlock()

	require.NoError(t, UpdateOptionsBulk(map[string]string{
		"Notice": "updated",
		"streaming_setting.first_response_timeout_seconds": "1200",
	}))

	optionRuntimeMutationMutex.Lock()
	assert.Equal(t, runtimeBefore+1, optionRuntimeRevision)
	streamingOptionMutex.Lock()
	assert.Equal(t, streamingBefore+1, streamingOptionRevision)
	streamingOptionMutex.Unlock()
	optionRuntimeMutationMutex.Unlock()
	var syncTasks int64
	require.NoError(t, database.Model(&SystemTask{}).Where("type = ?", SyncTaskType(SyncTopicOptions)).Count(&syncTasks).Error)
	assert.EqualValues(t, 1, syncTasks)
	assert.Equal(t, int64(1200), operation_setting.GetStreamingSetting().FirstResponseTimeoutSeconds)
}
