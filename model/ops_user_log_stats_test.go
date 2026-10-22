package model

import (
	"fmt"
	"os"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type legacyOpsUserLogStatsMeta struct {
	Id         int   `gorm:"primaryKey;autoIncrement:false"`
	LastLogId  int64 `gorm:"column:last_log_id;default:0"`
	Backfilled bool  `gorm:"column:backfilled;default:false"`
	UpdatedAt  int64 `gorm:"column:updated_at;default:0"`
}

func (legacyOpsUserLogStatsMeta) TableName() string {
	return "ops_user_log_stats_meta"
}

func TestSyncOpsUserLogStatsBatchesWritesWithinParameterBudget(t *testing.T) {
	const (
		userCount               = 5000
		maximumUpsertParameters = 3500
	)

	config := newGormConfig(true)
	config.Logger = logger.Discard
	database, err := gorm.Open(sqlite.Open(":memory:"), config)
	require.NoError(t, err)
	sqlDatabase, err := database.DB()
	require.NoError(t, err)
	sqlDatabase.SetMaxOpenConns(1)

	previousDB, previousLogDB := DB, LOG_DB
	previousMainType, previousLogType := common.MainDatabaseType(), common.LogDatabaseType()
	t.Cleanup(func() {
		DB, LOG_DB = previousDB, previousLogDB
		common.SetDatabaseTypes(previousMainType, previousLogType)
		require.NoError(t, sqlDatabase.Close())
	})
	DB, LOG_DB = database, database
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)

	require.NoError(t, database.AutoMigrate(&Log{}, &OpsUserLogStatsRow{}, &OpsUserLogStatsMeta{}))
	logs := make([]Log, userCount)
	for index := range logs {
		logs[index] = Log{
			UserId:    index + 1,
			Username:  fmt.Sprintf("stats-user-%04d", index),
			CreatedAt: 1700000000,
			Type:      LogTypeConsume,
			TokenName: "playground-regression",
			Group:     "default",
		}
	}
	require.NoError(t, database.CreateInBatches(&logs, 250).Error)
	require.NoError(t, database.Create(&OpsUserLogStatsMeta{Id: 1}).Error)

	var upsertParameterCounts []int
	require.NoError(t, database.Callback().Create().After("gorm:create").Register("test:capture_ops_stats_create", func(tx *gorm.DB) {
		if tx.Statement.Table == "ops_user_log_stats" {
			upsertParameterCounts = append(upsertParameterCounts, len(tx.Statement.Vars))
		}
	}))

	require.NoError(t, SyncOpsUserLogStats())
	require.Greater(t, len(upsertParameterCounts), 1)
	for _, parameterCount := range upsertParameterCounts {
		require.LessOrEqual(t, parameterCount, maximumUpsertParameters)
	}

	var aggregateCount int64
	require.NoError(t, database.Model(&OpsUserLogStatsRow{}).Count(&aggregateCount).Error)
	require.EqualValues(t, userCount, aggregateCount)
	var meta OpsUserLogStatsMeta
	require.NoError(t, database.First(&meta, "id = ?", 1).Error)
	require.EqualValues(t, logs[len(logs)-1].Id, meta.LastLogId)
	require.True(t, meta.Backfilled)
}

func TestSyncOpsUserLogStatsWaitsForMissingLowerID(t *testing.T) {
	config := newGormConfig(true)
	config.Logger = logger.Discard
	database, err := gorm.Open(sqlite.Open(":memory:"), config)
	require.NoError(t, err)
	sqlDatabase, err := database.DB()
	require.NoError(t, err)
	sqlDatabase.SetMaxOpenConns(1)

	previousDB, previousLogDB := DB, LOG_DB
	previousMainType, previousLogType := common.MainDatabaseType(), common.LogDatabaseType()
	t.Cleanup(func() {
		DB, LOG_DB = previousDB, previousLogDB
		common.SetDatabaseTypes(previousMainType, previousLogType)
		require.NoError(t, sqlDatabase.Close())
	})
	DB, LOG_DB = database, database
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)

	require.NoError(t, database.AutoMigrate(&Log{}, &OpsUserLogStatsRow{}, &OpsUserLogStatsMeta{}))
	require.NoError(t, database.Create(&OpsUserLogStatsMeta{Id: 1}).Error)
	require.NoError(t, database.Create(&Log{
		Id:        2,
		UserId:    4002,
		CreatedAt: 1750000002,
		Type:      LogTypeConsume,
		TokenId:   2,
		TokenName: "api-second",
	}).Error)

	require.NoError(t, SyncOpsUserLogStats())
	require.NoError(t, database.Create(&Log{
		Id:        1,
		UserId:    4001,
		CreatedAt: 1750000001,
		Type:      LogTypeConsume,
		TokenId:   1,
		TokenName: "api-first",
	}).Error)
	require.NoError(t, SyncOpsUserLogStats())

	stats, err := GetOpsUserLogStats([]int{4001, 4002})
	require.NoError(t, err)
	byUser := make(map[int]int, len(stats))
	for _, row := range stats {
		byUser[row.UserId] = row.ApiKeyCount
	}
	require.Equal(t, 1, byUser[4001])
	require.Equal(t, 1, byUser[4002])
}

func TestOpsUserLogStatsMetaMigrationMatrix(t *testing.T) {
	for _, dialect := range []string{"sqlite", "mysql", "postgres"} {
		for _, upgrade := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/upgrade_%t", dialect, upgrade), func(t *testing.T) {
				var dsn string
				switch dialect {
				case "mysql":
					dsn = os.Getenv("TEST_MYSQL_DSN")
					if dsn == "" {
						t.Skip("TEST_MYSQL_DSN is not configured")
					}
				case "postgres":
					dsn = os.Getenv("TEST_POSTGRES_DSN")
					if dsn == "" {
						t.Skip("TEST_POSTGRES_DSN is not configured")
					}
				}
				database := newCodexReportTestDatabase(t, dialect, dsn)
				if upgrade {
					require.NoError(t, database.AutoMigrate(&legacyOpsUserLogStatsMeta{}))
					require.NoError(t, database.Create(&legacyOpsUserLogStatsMeta{
						Id:         1,
						LastLogId:  42,
						Backfilled: true,
						UpdatedAt:  123,
					}).Error)
				}

				require.NoError(t, database.AutoMigrate(&OpsUserLogStatsMeta{}))
				require.NoError(t, database.AutoMigrate(&OpsUserLogStatsMeta{}))
				require.True(t, database.Migrator().HasColumn(&OpsUserLogStatsMeta{}, "pending_log_id"))
				require.True(t, database.Migrator().HasColumn(&OpsUserLogStatsMeta{}, "pending_since"))

				if upgrade {
					var stored OpsUserLogStatsMeta
					require.NoError(t, database.First(&stored, "id = ?", 1).Error)
					require.EqualValues(t, 42, stored.LastLogId)
					require.True(t, stored.Backfilled)
					require.Zero(t, stored.PendingLogId)
					require.Zero(t, stored.PendingSince)
				}
			})
		}
	}
}
