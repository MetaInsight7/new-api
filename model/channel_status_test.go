package model

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/alicebob/miniredis/v2"
	"github.com/glebarez/sqlite"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

type countChannelPublishHook struct {
	count atomic.Int64
}

func (hook *countChannelPublishHook) BeforeProcess(ctx context.Context, cmd redis.Cmder) (context.Context, error) {
	if cmd.Name() == "publish" {
		hook.count.Add(1)
	}
	return ctx, nil
}

func (*countChannelPublishHook) AfterProcess(context.Context, redis.Cmder) error {
	return nil
}

func (*countChannelPublishHook) BeforeProcessPipeline(ctx context.Context, _ []redis.Cmder) (context.Context, error) {
	return ctx, nil
}

func (*countChannelPublishHook) AfterProcessPipeline(context.Context, []redis.Cmder) error {
	return nil
}

func TestBatchSetChannelTagDatabaseMatrix(t *testing.T) {
	for _, dialect := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			var driver gorm.Dialector
			switch dialect {
			case "sqlite":
				driver = sqlite.Open(filepath.Join(t.TempDir(), "channel-tags.db"))
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
			db, err := gorm.Open(driver, &gorm.Config{NamingStrategy: schema.NamingStrategy{TablePrefix: "channel_tag_test_"}})
			require.NoError(t, err)
			sqlDB, err := db.DB()
			require.NoError(t, err)
			sqlDB.SetMaxOpenConns(2)
			previousDB, previousType := DB, common.MainDatabaseType()
			DB = db
			common.SetMainDatabaseType(common.DatabaseType(dialect))
			initCol()
			t.Cleanup(func() {
				DB = previousDB
				common.SetMainDatabaseType(previousType)
				initCol()
				assert.NoError(t, db.Migrator().DropTable(&Ability{}, &Channel{}, &SystemTask{}))
				assert.NoError(t, sqlDB.Close())
			})
			require.NoError(t, db.AutoMigrate(&Channel{}, &Ability{}, &SystemTask{}))
			var version string
			versionQuery := "SELECT version()"
			if dialect == "sqlite" {
				versionQuery = "SELECT sqlite_version()"
			}
			require.NoError(t, db.Raw(versionQuery).Scan(&version).Error)
			t.Logf("database version: %s", version)

			oldTag, newTag := "old-tag", "new-tag"
			priority, weight := int64(7), uint(23)
			channels := []Channel{
				{Name: "enabled", Status: common.ChannelStatusEnabled},
				{Name: "disabled", Status: common.ChannelStatusAutoDisabled},
				{Name: "unselected", Status: common.ChannelStatusEnabled},
			}
			for i := range channels {
				channel := &channels[i]
				channel.Models, channel.Group = "model-a,model-b", "default,second"
				channel.Tag, channel.Priority, channel.Weight = &oldTag, &priority, &weight
				require.NoError(t, db.Create(channel).Error)
				require.NoError(t, channel.AddAbilities(db))
			}
			ids := []int{channels[0].Id, channels[1].Id}
			for _, tc := range []struct {
				name string
				tag  *string
			}{
				{name: "set", tag: &newTag},
				{name: "clear", tag: nil},
			} {
				t.Run(tc.name, func(t *testing.T) {
					require.NoError(t, BatchSetChannelTag(ids, tc.tag))
					for i, channel := range channels {
						wantTag := tc.tag
						if i == 2 {
							wantTag = &oldTag
						}
						var stored Channel
						require.NoError(t, db.First(&stored, channel.Id).Error)
						assert.Equal(t, wantTag, stored.Tag, channel.Name)
						assert.Equal(t, channel.Status, stored.Status, channel.Name)
						var abilities []Ability
						require.NoError(t, db.Where("channel_id = ?", channel.Id).Find(&abilities).Error)
						require.Len(t, abilities, 4)
						for _, ability := range abilities {
							assert.Equal(t, wantTag, ability.Tag, channel.Name)
							assert.Equal(t, channel.Status == common.ChannelStatusEnabled, ability.Enabled, channel.Name)
							assert.Equal(t, channel.Priority, ability.Priority, channel.Name)
							assert.Equal(t, weight, ability.Weight, channel.Name)
						}
					}
				})
			}
		})
	}
}

// channelsRevision is the id of the newest channels sync task, 0 without one.
func channelsRevision(t *testing.T) int64 {
	t.Helper()
	revisions, err := GetLatestSyncRevisions()
	require.NoError(t, err)
	return revisions[SyncTopicChannels]
}

func setupChannelStatusTest(t *testing.T) {
	t.Helper()
	truncateTables(t)
	require.NoError(t, DB.Exec("DELETE FROM abilities").Error)
	require.NoError(t, DB.Exec("DELETE FROM channels").Error)

	memoryCacheEnabled := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = false
	t.Cleanup(func() {
		common.MemoryCacheEnabled = memoryCacheEnabled
	})
}

func TestDeleteDisabledChannelOnlyPublishesRealChanges(t *testing.T) {
	setupChannelStatusTest(t)
	beforeRevision := channelsRevision(t)
	deleted, err := DeleteDisabledChannel()
	require.NoError(t, err)
	assert.Zero(t, deleted)
	assert.Equal(t, beforeRevision, channelsRevision(t))

	channel := Channel{Name: "disabled-delete", Key: "key", Status: common.ChannelStatusManuallyDisabled}
	require.NoError(t, DB.Create(&channel).Error)
	deleted, err = DeleteDisabledChannel()
	require.NoError(t, err)
	assert.EqualValues(t, 1, deleted)
	afterRevision := channelsRevision(t)
	assert.Greater(t, afterRevision, beforeRevision)

	deleted, err = DeleteDisabledChannel()
	require.NoError(t, err)
	assert.Zero(t, deleted)
	assert.Equal(t, afterRevision, channelsRevision(t))
}

func TestUpdateChannelStatusPersistsMultiKeyState(t *testing.T) {
	setupChannelStatusTest(t)

	channel := Channel{
		Name:   "multi-key-status",
		Key:    "key-a\nkey-b",
		Status: common.ChannelStatusEnabled,
		ChannelInfo: ChannelInfo{
			IsMultiKey:           true,
			MultiKeySize:         2,
			MultiKeyMode:         constant.MultiKeyModePolling,
			MultiKeyPollingIndex: 1,
		},
	}
	require.NoError(t, DB.Create(&channel).Error)

	changed := UpdateChannelStatus(channel.Id, "key-a", common.ChannelStatusAutoDisabled, "provider rejected key")
	require.True(t, changed)

	var stored Channel
	require.NoError(t, DB.First(&stored, channel.Id).Error)
	assert.Equal(t, common.ChannelStatusEnabled, stored.Status)
	assert.Equal(t, common.ChannelStatusAutoDisabled, stored.ChannelInfo.MultiKeyStatusList[0])
	assert.Equal(t, "provider rejected key", stored.ChannelInfo.MultiKeyDisabledReason[0])
	assert.NotZero(t, stored.ChannelInfo.MultiKeyDisabledTime[0])
	assert.Equal(t, 1, stored.ChannelInfo.MultiKeyPollingIndex)
}

func TestUpdateChannelStatusRollsBackChannelAndAbilityTogether(t *testing.T) {
	setupChannelStatusTest(t)
	channel := Channel{Name: "status-atomic", Key: "key", Status: common.ChannelStatusEnabled, Models: "status-model", Group: "default"}
	require.NoError(t, channel.Insert())
	beforeRevision := channelsRevision(t)

	failure := errors.New("ability update failed")
	callbackName := "test:fail_atomic_ability_update"
	require.NoError(t, DB.Callback().Update().Before("gorm:update").Register(callbackName, func(tx *gorm.DB) {
		if strings.HasSuffix(tx.Statement.Table, "abilities") {
			tx.AddError(failure)
		}
	}))
	t.Cleanup(func() { _ = DB.Callback().Update().Remove(callbackName) })

	assert.False(t, UpdateChannelStatusManually(channel.Id, common.ChannelStatusManuallyDisabled, "manual operation"))
	require.NoError(t, DB.Callback().Update().Remove(callbackName))

	var stored Channel
	require.NoError(t, DB.First(&stored, channel.Id).Error)
	assert.Equal(t, common.ChannelStatusEnabled, stored.Status)
	assert.Empty(t, stored.OtherInfo)
	var ability Ability
	require.NoError(t, DB.Where("channel_id = ?", channel.Id).First(&ability).Error)
	assert.True(t, ability.Enabled)
	assert.Equal(t, beforeRevision, channelsRevision(t))
}

func TestChannelInsertDeleteAtomicityDatabaseMatrix(t *testing.T) {
	for _, dialect := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			var driver gorm.Dialector
			switch dialect {
			case "sqlite":
				driver = sqlite.Open(filepath.Join(t.TempDir(), "channel-lifecycle.db"))
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

			tablePrefix := fmt.Sprintf("channel_lifecycle_%s_%d_", dialect, os.Getpid())
			database, err := gorm.Open(driver, &gorm.Config{NamingStrategy: schema.NamingStrategy{TablePrefix: tablePrefix}})
			require.NoError(t, err)
			sqlDB, err := database.DB()
			require.NoError(t, err)
			sqlDB.SetMaxOpenConns(2)
			previousDB, previousLogDB := DB, LOG_DB
			previousMainType, previousLogType := common.MainDatabaseType(), common.LogDatabaseType()
			previousMemoryCache, previousRedis := common.MemoryCacheEnabled, common.RedisEnabled
			DB, LOG_DB = database, database
			common.SetDatabaseTypes(common.DatabaseType(dialect), common.DatabaseType(dialect))
			common.MemoryCacheEnabled, common.RedisEnabled = false, false
			initCol()
			t.Cleanup(func() {
				DB, LOG_DB = previousDB, previousLogDB
				common.SetDatabaseTypes(previousMainType, previousLogType)
				common.MemoryCacheEnabled, common.RedisEnabled = previousMemoryCache, previousRedis
				initCol()
				assert.NoError(t, database.Migrator().DropTable(&Ability{}, &Channel{}, &SystemTask{}))
				assert.NoError(t, sqlDB.Close())
			})
			require.NoError(t, database.Migrator().DropTable(&Ability{}, &Channel{}, &SystemTask{}))
			require.NoError(t, database.AutoMigrate(&Channel{}, &Ability{}, &SystemTask{}))

			beforeInsertRevision := channelsRevision(t)
			insertFailure := errors.New("ability creation failed")
			insertCallback := "test:fail_channel_insert_ability_creation"
			require.NoError(t, database.Callback().Create().Before("gorm:create").Register(insertCallback, func(tx *gorm.DB) {
				if strings.HasSuffix(tx.Statement.Table, "abilities") {
					tx.AddError(insertFailure)
				}
			}))
			failedInsert := Channel{Name: "insert-atomic", Key: "key", Status: common.ChannelStatusEnabled, Models: "insert-model", Group: "default"}
			require.ErrorIs(t, failedInsert.Insert(), insertFailure)
			require.NoError(t, database.Callback().Create().Remove(insertCallback))

			var channelCount, abilityCount int64
			require.NoError(t, database.Model(&Channel{}).Where("name = ?", failedInsert.Name).Count(&channelCount).Error)
			require.NoError(t, database.Model(&Ability{}).Where("channel_id = ?", failedInsert.Id).Count(&abilityCount).Error)
			assert.Zero(t, channelCount)
			assert.Zero(t, abilityCount)
			assert.Equal(t, beforeInsertRevision, channelsRevision(t))

			channel := Channel{Name: "delete-atomic", Key: "key", Status: common.ChannelStatusEnabled, Models: "delete-model", Group: "default"}
			require.NoError(t, channel.Insert())
			beforeDeleteRevision := channelsRevision(t)
			deleteFailure := errors.New("ability deletion failed")
			deleteCallback := "test:fail_channel_delete_ability_deletion"
			require.NoError(t, database.Callback().Delete().Before("gorm:delete").Register(deleteCallback, func(tx *gorm.DB) {
				if strings.HasSuffix(tx.Statement.Table, "abilities") {
					tx.AddError(deleteFailure)
				}
			}))
			require.ErrorIs(t, channel.Delete(), deleteFailure)
			require.NoError(t, database.Callback().Delete().Remove(deleteCallback))

			var stored Channel
			require.NoError(t, database.First(&stored, channel.Id).Error)
			require.NoError(t, database.Model(&Ability{}).Where("channel_id = ?", channel.Id).Count(&abilityCount).Error)
			assert.Positive(t, abilityCount)
			assert.Equal(t, beforeDeleteRevision, channelsRevision(t))

			require.NoError(t, channel.Delete())
			afterDeleteRevision := channelsRevision(t)
			assert.Greater(t, afterDeleteRevision, beforeDeleteRevision)

			require.NoError(t, channel.Delete())
			assert.Equal(t, afterDeleteRevision, channelsRevision(t))

			deletedCount, err := BatchDeleteChannels([]int{channel.Id, channel.Id + 1_000_000})
			require.NoError(t, err)
			assert.Zero(t, deletedCount)
			assert.Equal(t, afterDeleteRevision, channelsRevision(t))
		})
	}
}

// writeAfterNextChannelRead stands for another node: right after the next
// read of the channel, before the writer under test writes, it stores the
// columns write returns for the channel it reads.
func writeAfterNextChannelRead(t *testing.T, channelID int, write func(stored *Channel) map[string]any) {
	t.Helper()
	var done atomic.Bool
	require.NoError(t, DB.Callback().Query().After("gorm:query").Register("test:other_node_write", func(tx *gorm.DB) {
		if !strings.HasSuffix(tx.Statement.Table, "channels") || !done.CompareAndSwap(false, true) {
			return
		}
		var stored Channel
		require.NoError(t, DB.First(&stored, "id = ?", channelID).Error)
		require.NoError(t, DB.Model(&Channel{}).Where("id = ?", channelID).Updates(write(&stored)).Error)
	}))
	t.Cleanup(func() { _ = DB.Callback().Query().Remove("test:other_node_write") })
}

// A writer of other_info writes back only while no other writer, on any node,
// changed it since the read; otherwise it reads again, so both changes are
// kept. It writes its own columns only, so a stale read never puts back the
// key, counters, models or channel_info another writer stored.
func TestChannelOtherInfoWritesKeepConcurrentChanges(t *testing.T) {
	useScheduledChannelDatabase(t)
	common.MemoryCacheEnabled = false

	t.Run("a status change and a balance query of another node", func(t *testing.T) {
		channel := Channel{Name: "status-race", Key: "original-key", Status: common.ChannelStatusEnabled, Models: "original-model", Group: "default", UsedQuota: 100}
		require.NoError(t, channel.Insert())
		concurrentInfo := ChannelInfo{MultiKeyMode: constant.MultiKeyModePolling, MultiKeyPollingIndex: 1}
		info := channel.GetOtherInfo()
		info[channelBalanceDetailKey] = map[string]any{"amount": 7, "currency": "USD"}
		channel.SetOtherInfo(info)
		require.NoError(t, DB.Model(&Channel{}).Where("id = ?", channel.Id).Updates(map[string]any{
			"other_info":   channel.OtherInfo,
			"key":          "rotated-key",
			"used_quota":   gorm.Expr("used_quota + ?", 250),
			"models":       "concurrent-model",
			"channel_info": concurrentInfo,
		}).Error)

		require.True(t, UpdateChannelStatus(channel.Id, "", common.ChannelStatusManuallyDisabled, "manual operation"))

		var stored Channel
		require.NoError(t, DB.First(&stored, channel.Id).Error)
		assert.Equal(t, common.ChannelStatusManuallyDisabled, stored.Status)
		assert.Equal(t, "manual operation", stored.GetOtherInfo()["status_reason"])
		detail := stored.GetBalanceDetail()
		require.NotNil(t, detail, stored.OtherInfo)
		require.NotNil(t, detail.Amount)
		assert.Equal(t, 7.0, *detail.Amount)
		assert.Equal(t, "rotated-key", stored.Key)
		assert.Equal(t, int64(350), stored.UsedQuota)
		assert.Equal(t, "concurrent-model", stored.Models)
		assert.Equal(t, concurrentInfo, stored.ChannelInfo)
	})

	t.Run("a balance query and a status change of another node", func(t *testing.T) {
		channel := Channel{Name: "balance-race", Key: "key", Status: common.ChannelStatusEnabled, Models: "balance-model", Group: "default"}
		require.NoError(t, channel.Insert())
		// A channel stored before other_info was written has it NULL.
		require.NoError(t, DB.Model(&Channel{}).Where("id = ?", channel.Id).Update("other_info", gorm.Expr("NULL")).Error)
		writeAfterNextChannelRead(t, channel.Id, func(stored *Channel) map[string]any {
			info := stored.GetOtherInfo()
			info["status_reason"] = "other node"
			stored.SetOtherInfo(info)
			return map[string]any{"other_info": stored.OtherInfo, "status": common.ChannelStatusManuallyDisabled}
		})
		amount := 3.5
		detail, err := UpdateChannelBalanceDetail(channel.Id, func(*ChannelBalanceDetail) (*ChannelBalanceDetail, *float64) {
			return &ChannelBalanceDetail{BalanceResult: jsplugin.BalanceResult{Amount: &amount, Currency: "USD"}}, &amount
		})
		require.NoError(t, err)
		require.NotNil(t, detail)

		var stored Channel
		require.NoError(t, DB.First(&stored, channel.Id).Error)
		assert.Equal(t, common.ChannelStatusManuallyDisabled, stored.Status)
		assert.Equal(t, "other node", stored.GetOtherInfo()["status_reason"])
		assert.Equal(t, 3.5, stored.Balance)
		assert.NotZero(t, stored.BalanceUpdatedTime)
		require.NotNil(t, stored.GetBalanceDetail())
		assert.Equal(t, 3.5, *stored.GetBalanceDetail().Amount)

		// Storing the same failure twice writes the same values again, which
		// MySQL reports as no changed row.
		for range 2 {
			_, err = UpdateChannelBalanceDetail(channel.Id, func(stored *ChannelBalanceDetail) (*ChannelBalanceDetail, *float64) {
				stored.Error = "upstream failed"
				return stored, nil
			})
			require.NoError(t, err)
		}
		require.NoError(t, DB.First(&stored, channel.Id).Error)
		assert.Equal(t, "upstream failed", stored.GetBalanceDetail().Error)
		assert.Equal(t, "other node", stored.GetOtherInfo()["status_reason"])
	})
}

func TestAutoDisableKeepsManualDisable(t *testing.T) {
	for _, cacheEnabled := range []bool{false, true} {
		for _, tc := range []struct {
			name          string
			multiKey      bool
			initialStatus int
			wantChanged   bool
			wantStatus    int
		}{
			{name: "single key manual disable is kept", initialStatus: common.ChannelStatusManuallyDisabled, wantStatus: common.ChannelStatusManuallyDisabled},
			{name: "multi key manual disable is kept", multiKey: true, initialStatus: common.ChannelStatusManuallyDisabled, wantStatus: common.ChannelStatusManuallyDisabled},
			{name: "single key enabled is auto disabled", initialStatus: common.ChannelStatusEnabled, wantChanged: true, wantStatus: common.ChannelStatusAutoDisabled},
			{name: "multi key enabled is auto disabled", multiKey: true, initialStatus: common.ChannelStatusEnabled, wantChanged: true, wantStatus: common.ChannelStatusAutoDisabled},
		} {
			t.Run(fmt.Sprintf("cache=%t/%s", cacheEnabled, tc.name), func(t *testing.T) {
				setupChannelStatusTest(t)
				common.MemoryCacheEnabled = cacheEnabled

				channel := Channel{Name: t.Name(), Key: "key-a", Status: tc.initialStatus, Models: "test-model", Group: "default"}
				usingKey := "key-a"
				if tc.multiKey {
					// key-b already failed, so a failure on key-a disables every key.
					channel.Key = "key-a\nkey-b"
					channel.ChannelInfo = ChannelInfo{IsMultiKey: true, MultiKeySize: 2, MultiKeyStatusList: map[int]int{1: common.ChannelStatusAutoDisabled}}
				}
				channel.SetOtherInfo(map[string]any{"status_reason": "manual operation", "status_time": int64(1234)})
				require.NoError(t, channel.Insert())
				InitChannelCache()

				changed := UpdateChannelStatus(channel.Id, usingKey, common.ChannelStatusAutoDisabled, "provider rejected key")
				assert.Equal(t, tc.wantChanged, changed)

				stored, err := GetChannelById(channel.Id, true)
				require.NoError(t, err)
				assert.Equal(t, tc.wantStatus, stored.Status)
				if !tc.wantChanged {
					assert.Equal(t, "manual operation", stored.GetOtherInfo()["status_reason"])
					assert.Equal(t, float64(1234), stored.GetOtherInfo()["status_time"])
					assert.Equal(t, channel.ChannelInfo.MultiKeyStatusList, stored.ChannelInfo.MultiKeyStatusList)
				}
				var abilities []Ability
				require.NoError(t, DB.Where("channel_id = ?", channel.Id).Find(&abilities).Error)
				require.NotEmpty(t, abilities)
				for _, ability := range abilities {
					assert.False(t, ability.Enabled)
				}
				if cacheEnabled {
					cached, err := CacheGetChannel(channel.Id)
					require.NoError(t, err)
					assert.Equal(t, tc.wantStatus, cached.Status)
					assert.Equal(t, stored.ChannelInfo.MultiKeyStatusList, cached.ChannelInfo.MultiKeyStatusList)
				}
			})
		}
	}
}

func TestUpdateChannelStatusBumpsChannelsExactlyOnceForEveryChange(t *testing.T) {
	for _, cacheEnabled := range []bool{false, true} {
		t.Run(fmt.Sprintf("cache=%t", cacheEnabled), func(t *testing.T) {
			setupChannelStatusTest(t)
			common.MemoryCacheEnabled = cacheEnabled
			countRevisions := func() int64 {
				var count int64
				require.NoError(t, DB.Model(&SystemTask{}).Where("type = ?", SyncTaskType(SyncTopicChannels)).Count(&count).Error)
				return count
			}
			single := Channel{Name: "status-single", Key: "key-a", Status: common.ChannelStatusEnabled, Models: "status-model", Group: "default"}
			multi := Channel{Name: "status-multi", Key: "key-a\nkey-b", Status: common.ChannelStatusEnabled, Models: "status-model", Group: "default",
				ChannelInfo: ChannelInfo{IsMultiKey: true, MultiKeySize: 2, MultiKeyMode: constant.MultiKeyModePolling}}
			require.NoError(t, single.Insert())
			require.NoError(t, multi.Insert())
			InitChannelCache()

			for _, step := range []struct {
				name     string
				id       int
				usingKey string
				status   int
				changed  bool
			}{
				{name: "a request disables a key", id: multi.Id, usingKey: "key-a", status: common.ChannelStatusAutoDisabled, changed: true},
				{name: "a request disables the same key again", id: multi.Id, usingKey: "key-a", status: common.ChannelStatusAutoDisabled},
				{name: "a request disables a single-key channel", id: single.Id, usingKey: "key-a", status: common.ChannelStatusAutoDisabled, changed: true},
				{name: "a test re-enables the channel", id: single.Id, usingKey: "key-a", status: common.ChannelStatusEnabled, changed: true},
				{name: "an admin disables a channel", id: single.Id, status: common.ChannelStatusManuallyDisabled, changed: true},
				{name: "an admin disables it again", id: single.Id, status: common.ChannelStatusManuallyDisabled},
				{name: "an admin enables a channel", id: single.Id, status: common.ChannelStatusEnabled, changed: true},
				{name: "an admin disables a multi-key channel", id: multi.Id, status: common.ChannelStatusManuallyDisabled, changed: true},
			} {
				before := countRevisions()
				changed := UpdateChannelStatus(step.id, step.usingKey, step.status, step.name)
				assert.Equal(t, step.changed, changed, step.name)
				after := countRevisions()
				if step.changed {
					assert.Equal(t, before+1, after, step.name)
				} else {
					assert.Equal(t, before, after, step.name)
				}
			}
		})
	}
}

func TestUpdateChannelStatusPublishesExactlyOnceAfterCommit(t *testing.T) {
	setupChannelStatusTest(t)
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr(), DB: 7})
	hook := &countChannelPublishHook{}
	client.AddHook(hook)
	previousClient, previousEnabled := common.RDB, common.RedisEnabled
	common.RDB, common.RedisEnabled = client, true
	t.Cleanup(func() {
		common.RDB, common.RedisEnabled = previousClient, previousEnabled
		require.NoError(t, client.Close())
	})

	channel := Channel{Name: "pubsub", Key: "key", Status: common.ChannelStatusEnabled, Models: "model", Group: "default"}
	require.NoError(t, channel.Insert())
	require.True(t, UpdateChannelStatusManually(channel.Id, common.ChannelStatusManuallyDisabled, "manual"))
	assert.Equal(t, int64(1), hook.count.Load())

	require.False(t, UpdateChannelStatusManually(channel.Id, common.ChannelStatusManuallyDisabled, "manual"))
	assert.Equal(t, int64(1), hook.count.Load())

	failure := errors.New("ability update failed")
	callbackName := "test:fail_pubsub_ability_update"
	require.NoError(t, DB.Callback().Update().Before("gorm:update").Register(callbackName, func(tx *gorm.DB) {
		if strings.HasSuffix(tx.Statement.Table, "abilities") {
			tx.AddError(failure)
		}
	}))
	require.False(t, UpdateChannelStatusManually(channel.Id, common.ChannelStatusEnabled, "manual"))
	require.NoError(t, DB.Callback().Update().Remove(callbackName))
	assert.Equal(t, int64(1), hook.count.Load())
}

func TestAutoDisablePluginKeyAlwaysBumpsChannels(t *testing.T) {
	setupChannelStatusTest(t)
	master := common.IsMasterNode
	t.Cleanup(func() { common.IsMasterNode = master })
	for key, hooks := range map[string]string{
		"status-scheduled": `export function scheduled() { return {next: 3600}; }`,
		"status-plain":     "",
	} {
		_, err := jsplugin.DefaultRegistry.Register(`export const meta = {apiVersion: 1, key: "`+key+`", name: "Status", version: "1.0.0", author: {name: "Test"}, relay: [{incoming_path: "/v1/chat/completions", upstream_path: "/chat"}]};`+hooks, jsplugin.Options{})
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, jsplugin.DefaultRegistry.Unregister(key)) })
	}
	revision := func() int64 { return channelsRevision(t) }

	for _, tc := range []struct {
		plugin string
		master bool
	}{
		{plugin: "status-scheduled"},
		{plugin: "status-scheduled", master: true},
		{plugin: "status-plain"},
	} {
		t.Run(fmt.Sprintf("%s/master=%t", tc.plugin, tc.master), func(t *testing.T) {
			common.IsMasterNode = tc.master
			channel := Channel{Type: constant.ChannelTypeTaskPlugin, Name: t.Name(), Key: "key-a", Status: common.ChannelStatusEnabled, Models: "status-model", Group: "default"}
			channel.SetSetting(dto.ChannelSettings{TaskPluginKey: tc.plugin})
			require.NoError(t, channel.Insert())

			before := revision()
			require.True(t, UpdateChannelStatus(channel.Id, "key-a", common.ChannelStatusAutoDisabled, "quota"))
			assert.NotEqual(t, before, revision())
		})
	}
}

func TestUpdateChannelStatusRefreshesCacheOnlyForRealMultiKeyChanges(t *testing.T) {
	setupChannelStatusTest(t)
	common.MemoryCacheEnabled = true

	channel := Channel{
		Name:   "multi-key-cache-refresh",
		Type:   constant.ChannelTypeOpenAI,
		Key:    "key-a\nkey-b",
		Status: common.ChannelStatusEnabled,
		ChannelInfo: ChannelInfo{
			IsMultiKey:   true,
			MultiKeySize: 2,
		},
	}
	require.NoError(t, DB.Create(&channel).Error)
	InitChannelCache()

	initialCached, err := CacheGetChannel(channel.Id)
	require.NoError(t, err)
	assert.False(t, UpdateChannelStatus(channel.Id, "missing-key", common.ChannelStatusAutoDisabled, "not selected from this channel"))
	cachedAfterNoop, err := CacheGetChannel(channel.Id)
	require.NoError(t, err)
	assert.Same(t, initialCached, cachedAfterNoop)
	assert.Empty(t, cachedAfterNoop.ChannelInfo.MultiKeyStatusList)

	assert.True(t, UpdateChannelStatus(channel.Id, "key-a", common.ChannelStatusAutoDisabled, "provider rejected key"))
	cachedAfterChange, err := CacheGetChannel(channel.Id)
	require.NoError(t, err)
	assert.NotSame(t, initialCached, cachedAfterChange)
	assert.Equal(t, common.ChannelStatusAutoDisabled, cachedAfterChange.ChannelInfo.MultiKeyStatusList[0])
}

func TestAutomaticChannelStatusPreservesPermanentUnavailability(t *testing.T) {
	for _, cacheEnabled := range []bool{false, true} {
		for _, multiKey := range []bool{false, true} {
			t.Run(fmt.Sprintf("cache=%t/multi_key=%t", cacheEnabled, multiKey), func(t *testing.T) {
				setupChannelStatusTest(t)
				common.MemoryCacheEnabled = cacheEnabled
				channel := Channel{
					Name: "permanently-unavailable", Type: constant.ChannelTypeOpenAI,
					Key: "key-a\nkey-b", Status: common.ChannelStatusBanned, Models: "guard-model", Group: "default",
					ChannelInfo: ChannelInfo{IsMultiKey: multiKey, MultiKeySize: 2,
						MultiKeyMode: constant.MultiKeyModePolling, MultiKeyPollingIndex: 1,
						MultiKeyStatusList:     map[int]int{0: common.ChannelStatusAutoDisabled},
						MultiKeyDisabledReason: map[int]string{0: "existing key failure"},
						MultiKeyDisabledTime:   map[int]int64{0: 1234}},
				}
				channel.SetOtherInfo(map[string]any{"status_reason": "manual ban", "status_time": int64(1000)})
				require.NoError(t, DB.Create(&channel).Error)
				InitChannelCache()
				if cacheEnabled {
					CacheUpdateChannelStatus(channel.Id, common.ChannelStatusEnabled)
				}
				for _, status := range []int{common.ChannelStatusEnabled, common.ChannelStatusManuallyDisabled, common.ChannelStatusAutoDisabled, common.ChannelStatusBanned} {
					assert.False(t, UpdateChannelStatus(channel.Id, "key-b", status, "automatic task"))
				}
				stored, err := GetChannelById(channel.Id, true)
				require.NoError(t, err)
				assert.Equal(t, common.ChannelStatusBanned, stored.Status)
				assert.Equal(t, channel.ChannelInfo, stored.ChannelInfo)
				assert.Equal(t, channel.OtherInfo, stored.OtherInfo)
			})
		}
	}
}

func TestAutomaticChannelStatusCannotCreatePermanentUnavailability(t *testing.T) {
	setupChannelStatusTest(t)
	channel := Channel{Name: "automatic-ban-rejected", Key: "key-a", Status: common.ChannelStatusEnabled}
	require.NoError(t, DB.Create(&channel).Error)
	assert.False(t, UpdateChannelStatus(channel.Id, "key-a", common.ChannelStatusBanned, "automatic task"))
	stored, err := GetChannelById(channel.Id, true)
	require.NoError(t, err)
	assert.Equal(t, common.ChannelStatusEnabled, stored.Status)
}

func TestStaleChannelUpdateCannotClearPermanentUnavailability(t *testing.T) {
	setupChannelStatusTest(t)
	channel := Channel{Name: "stale-task-update", Key: "key-a", Status: common.ChannelStatusEnabled, Models: "original-model", Group: "default"}
	channel.SetOtherInfo(map[string]any{"provider_note": "keep"})
	require.NoError(t, channel.Insert())
	stale, err := GetChannelById(channel.Id, true)
	require.NoError(t, err)
	require.True(t, UpdateChannelStatusManually(channel.Id, common.ChannelStatusBanned, "manual ban"))
	stale.Models = "updated-model"
	require.NoError(t, stale.Update())
	stored, err := GetChannelById(channel.Id, true)
	require.NoError(t, err)
	assert.Equal(t, common.ChannelStatusBanned, stored.Status)
	assert.Equal(t, "updated-model", stored.Models)
	assert.Equal(t, "manual ban", stored.GetOtherInfo()["status_reason"])
	assert.Equal(t, "keep", stored.GetOtherInfo()["provider_note"])
	candidates, err := GetSatisfiedChannelCandidates("default", "updated-model", 0, nil)
	require.NoError(t, err)
	assert.Empty(t, candidates)
}
