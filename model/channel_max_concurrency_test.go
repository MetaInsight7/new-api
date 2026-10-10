package model

import (
	"os"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestChannelUpdateWithMaxConcurrencyPersistsExplicitZeroAcrossDatabases(t *testing.T) {
	for _, dialect := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			var driver gorm.Dialector
			switch dialect {
			case "sqlite":
				driver = sqlite.Open(":memory:")
			case "mysql":
				dsn := os.Getenv("TEST_MYSQL_DSN")
				if dsn == "" {
					t.Skip("TEST_MYSQL_DSN is not configured")
				}
				driver = mysql.Open(dsn)
			case "postgres":
				dsn := os.Getenv("TEST_POSTGRES_DSN")
				if dsn == "" {
					t.Skip("TEST_POSTGRES_DSN is not configured")
				}
				driver = postgres.Open(dsn)
			}

			database, err := gorm.Open(driver, &gorm.Config{
				NamingStrategy: schema.NamingStrategy{TablePrefix: "channel_max_concurrency_test_"},
			})
			require.NoError(t, err)
			sqlDatabase, err := database.DB()
			require.NoError(t, err)
			sqlDatabase.SetMaxOpenConns(1)

			previousDB := DB
			previousDatabaseType := common.MainDatabaseType()
			DB = database
			common.SetMainDatabaseType(common.DatabaseType(dialect))
			t.Cleanup(func() {
				require.NoError(t, database.Migrator().DropTable(&Ability{}, &Channel{}))
				DB = previousDB
				common.SetMainDatabaseType(previousDatabaseType)
				require.NoError(t, sqlDatabase.Close())
			})

			require.False(t, database.Migrator().HasTable(&Channel{}), "use an empty test database")
			require.False(t, database.Migrator().HasTable(&Ability{}), "use an empty test database")
			require.NoError(t, database.AutoMigrate(&Channel{}, &Ability{}))

			channel := Channel{
				Type:           constant.ChannelTypeOpenAI,
				Key:            "sk-test",
				Status:         common.ChannelStatusEnabled,
				Name:           "bounded",
				Models:         "gpt-test",
				Group:          "default",
				MaxConcurrency: 5,
			}
			require.NoError(t, database.Create(&channel).Error)

			channel.MaxConcurrency = 0
			require.NoError(t, channel.UpdateWithMaxConcurrency())

			var persisted Channel
			require.NoError(t, database.First(&persisted, "id = ?", channel.Id).Error)
			assert.Zero(t, persisted.MaxConcurrency)
		})
	}
}
