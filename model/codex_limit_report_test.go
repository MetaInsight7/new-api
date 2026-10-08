package model

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/constant"
	"github.com/glebarez/sqlite"
	sqlmysql "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func newCodexReportTestDatabase(t *testing.T, dialect, dsn string) *gorm.DB {
	t.Helper()
	if dialect == "sqlite" {
		database, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "report.db")), &gorm.Config{})
		require.NoError(t, err)
		connection, err := database.DB()
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, connection.Close()) })
		return database
	}
	name := fmt.Sprintf("newapi_codex_report_%d", time.Now().UnixNano())
	var original, isolated gorm.Dialector
	if dialect == "mysql" {
		config, err := sqlmysql.ParseDSN(dsn)
		require.NoError(t, err)
		require.Equal(t, "tcp", config.Net)
		host, _, err := net.SplitHostPort(config.Addr)
		require.NoError(t, err)
		require.True(t, net.ParseIP(host).IsLoopback(), "database tests only permit loopback instances")
		original = mysql.Open(dsn)
		config.DBName = name
		isolated = mysql.Open(config.FormatDSN())
	} else {
		parsed, err := url.Parse(dsn)
		require.NoError(t, err)
		require.True(t, net.ParseIP(parsed.Hostname()).IsLoopback(), "database tests only permit loopback instances")
		original = postgres.Open(dsn)
		parsed.Path = "/" + name
		isolated = postgres.Open(parsed.String())
	}
	admin, err := gorm.Open(original, &gorm.Config{})
	require.NoError(t, err)
	adminConnection, err := admin.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, adminConnection.Close()) })
	createSQL := "CREATE DATABASE " + name
	if dialect == "mysql" {
		createSQL += " CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci"
	}
	require.NoError(t, admin.Exec(createSQL).Error)
	t.Cleanup(func() { require.NoError(t, admin.Exec("DROP DATABASE "+name).Error) })
	database, err := gorm.Open(isolated, &gorm.Config{})
	require.NoError(t, err)
	connection, err := database.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, connection.Close()) })
	return database
}

func TestCodexLimitReportDatabaseAggregation(t *testing.T) {
	for _, dialect := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
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
			logDatabase := newCodexReportTestDatabase(t, dialect, dsn)
			previousDB, previousLogDB := DB, LOG_DB
			DB, LOG_DB = database, database
			t.Cleanup(func() { DB, LOG_DB = previousDB, previousLogDB })
			require.NoError(t, database.AutoMigrate(&Channel{}, &Log{}))
			require.NoError(t, logDatabase.AutoMigrate(&Log{}))
			var version string
			if dialect == "sqlite" {
				require.NoError(t, database.Raw("select sqlite_version()").Scan(&version).Error)
			} else {
				require.NoError(t, database.Raw("select version()").Scan(&version).Error)
			}
			t.Logf("%s version: %s", dialect, version)
			channels := []Channel{
				{Id: 31, Type: constant.ChannelTypeCodex, Status: 1, Key: "credential-a"},
				{Id: 32, Type: constant.ChannelTypeCodex, Status: 2, Key: "credential-b"},
				{Id: 33, Type: constant.ChannelTypeCodex, Status: 3, Key: "credential-c"},
				{Id: 34, Type: constant.ChannelTypeCodex, Status: 4, Key: "credential-d"},
				{Id: 35, Type: constant.ChannelTypeOpenAI, Status: 1},
			}
			require.NoError(t, database.Create(&channels).Error)
			listed, err := GetCodexLimitReportChannels(context.Background())
			require.NoError(t, err)
			require.Len(t, listed, 4)
			assert.Equal(t, "credential-b", listed[1].Key)
			assert.Equal(t, 4, listed[3].Status)
			logs := []Log{
				{ChannelId: 31, Type: LogTypeConsume, CreatedAt: 100, PromptTokens: 10, CompletionTokens: 2, Quota: 50},
				{ChannelId: 31, Type: LogTypeConsume, CreatedAt: 200, PromptTokens: 20, CompletionTokens: 3, Quota: 70},
				{ChannelId: 32, Type: LogTypeConsume, CreatedAt: 150, PromptTokens: 7, CompletionTokens: 1, Quota: 30},
				{ChannelId: 31, Type: LogTypeConsume, CreatedAt: 99, PromptTokens: 900, Quota: 900},
				{ChannelId: 31, Type: LogTypeConsume, CreatedAt: 201, PromptTokens: 900, Quota: 900},
				{ChannelId: 31, Type: LogTypeRefund, CreatedAt: 150, Quota: 500},
				{ChannelId: 31, Type: LogTypeError, CreatedAt: 150, PromptTokens: 900},
				{ChannelId: 35, Type: LogTypeConsume, CreatedAt: 150, PromptTokens: 900, Quota: 900},
			}
			require.NoError(t, database.Create(&logs).Error)
			require.NoError(t, logDatabase.Create(&logs).Error)
			for range 2 {
				require.NoError(t, database.AutoMigrate(&Channel{}, &Log{}))
				require.NoError(t, logDatabase.AutoMigrate(&Log{}))
			}
			assert.True(t, database.Migrator().HasIndex(&Log{}, "idx_created_at_type"))
			assert.True(t, logDatabase.Migrator().HasIndex(&Log{}, "idx_created_at_type"))
			var count int64
			require.NoError(t, database.Model(&Log{}).Count(&count).Error)
			assert.Equal(t, int64(8), count)
			for _, separateLogs := range []bool{false, true} {
				t.Run(fmt.Sprintf("separate_logs_%t", separateLogs), func(t *testing.T) {
					if separateLogs {
						LOG_DB = logDatabase
						require.NoError(t, database.Create(&Log{ChannelId: 31, Type: LogTypeConsume, CreatedAt: 150, Quota: 9999}).Error)
					}
					stats, err := GetCodexChannelUsageStats(context.Background(), []int{31, 32, 33, 34}, 100, 200)
					require.NoError(t, err)
					assert.Equal(t, map[int]CodexChannelUsageStat{
						31: {ChannelID: 31, TokenUsed: 35, Quota: 120},
						32: {ChannelID: 32, TokenUsed: 8, Quota: 30},
					}, stats)
					channelIDs := make([]int, 501)
					for index := range channelIDs {
						channelIDs[index] = index + 1000
					}
					channelIDs[0], channelIDs[500] = 31, 32
					batched, err := GetCodexChannelUsageStats(context.Background(), channelIDs, 100, 200)
					require.NoError(t, err)
					assert.Equal(t, stats, batched)
				})
			}
			empty, err := GetCodexChannelUsageStats(context.Background(), nil, 100, 200)
			require.NoError(t, err)
			assert.Empty(t, empty)
			cancelled, cancel := context.WithCancel(context.Background())
			cancel()
			_, err = GetCodexChannelUsageStats(cancelled, []int{31}, 100, 200)
			require.ErrorIs(t, err, context.Canceled)
		})
	}
}
