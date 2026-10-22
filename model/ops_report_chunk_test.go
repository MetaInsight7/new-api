package model

import (
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type opsReportLogFixture struct {
	Id        int    `gorm:"column:id"`
	UserId    int    `gorm:"column:user_id"`
	Ip        string `gorm:"column:ip"`
	CreatedAt int64  `gorm:"column:created_at"`
}

type opsReportQueryObservation struct {
	SQL            string
	ParameterCount int
}

func TestOpsReportQueriesBatchLargeIDSets(t *testing.T) {
	config := newGormConfig(false)
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

	require.NoError(t, database.Exec(`CREATE TABLE users (id INTEGER PRIMARY KEY, deleted_at DATETIME)`).Error)
	require.NoError(t, database.Exec(`
		CREATE TABLE user_subscriptions (
			id INTEGER PRIMARY KEY,
			user_id INTEGER NOT NULL,
			amount_total INTEGER NOT NULL,
			amount_used INTEGER NOT NULL,
			source TEXT NOT NULL,
			status TEXT NOT NULL,
			end_time INTEGER NOT NULL
		)`).Error)
	require.NoError(t, database.Exec(`
		CREATE TABLE logs (
			id INTEGER PRIMARY KEY,
			user_id INTEGER NOT NULL,
			ip TEXT NOT NULL,
			created_at INTEGER NOT NULL
		)`).Error)

	now := common.GetTimestamp()
	require.NoError(t, database.Exec(`INSERT INTO users (id) VALUES (1)`).Error)
	require.NoError(t, database.Exec(`
		INSERT INTO user_subscriptions
			(id, user_id, amount_total, amount_used, source, status, end_time)
		VALUES
			(1, 1, 100, 20, 'earliest', 'active', ?),
			(2, 1, 50, 10, 'later', 'active', ?)`, now+100, now+200).Error)

	userIds := make([]int, 1201)
	logs := make([]opsReportLogFixture, len(userIds))
	for index := range userIds {
		userIds[index] = index + 1
		logs[index] = opsReportLogFixture{
			Id:        index + 1,
			UserId:    index + 1,
			Ip:        fmt.Sprintf("192.0.2.%d", index%250+1),
			CreatedAt: now,
		}
	}
	require.NoError(t, database.Table("logs").CreateInBatches(&logs, 250).Error)

	var observations []opsReportQueryObservation
	require.NoError(t, database.Callback().Row().After("gorm:row").Register("test:capture_ops_report_queries", func(tx *gorm.DB) {
		sql := strings.ToLower(tx.Statement.SQL.String())
		if strings.Contains(sql, "user_subscriptions") || strings.Contains(sql, "from logs") {
			observations = append(observations, opsReportQueryObservation{
				SQL:            sql,
				ParameterCount: len(tx.Statement.Vars),
			})
		}
	}))

	remaining, err := GetOpsUserSubscriptionRemaining(userIds)
	require.NoError(t, err)
	require.EqualValues(t, 120, remaining[1])

	sources, err := GetOpsUserSubscriptionSources(userIds)
	require.NoError(t, err)
	require.Equal(t, "earliest", sources[1].Source)

	lastIPs, err := GetOpsUsersLastIP(userIds, now-1)
	require.NoError(t, err)
	require.Len(t, lastIPs, len(userIds))

	var remainingQueries, sourceQueries, maxIdQueries, lookupQueries int
	for _, observation := range observations {
		switch {
		case strings.Contains(observation.SQL, "sum(case"):
			remainingQueries++
			require.LessOrEqual(t, observation.ParameterCount, opsReportChunkSize+2)
		case strings.Contains(observation.SQL, "user_subscriptions"):
			sourceQueries++
			require.LessOrEqual(t, observation.ParameterCount, opsReportChunkSize+2)
		case strings.Contains(observation.SQL, "max(id)"):
			maxIdQueries++
			require.LessOrEqual(t, observation.ParameterCount, opsReportChunkSize+1)
		case strings.Contains(observation.SQL, "select user_id, ip"):
			lookupQueries++
			require.LessOrEqual(t, observation.ParameterCount, opsReportChunkSize)
		}
	}
	require.Equal(t, 3, remainingQueries)
	require.Equal(t, 3, sourceQueries)
	require.Equal(t, 3, maxIdQueries)
	require.Equal(t, 3, lookupQueries)
}
