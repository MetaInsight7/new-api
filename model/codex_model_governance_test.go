package model

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/glebarez/sqlite"
	sqlmysql "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newCodexGovernanceTestDatabase(t *testing.T, dialect string) *gorm.DB {
	t.Helper()
	configuration := &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)}
	var database *gorm.DB
	if dialect == "sqlite" {
		var err error
		database, err = gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "governance.db")), configuration)
		require.NoError(t, err)
	} else {
		name := fmt.Sprintf("newapi_codex_governance_%d", time.Now().UnixNano())
		var adminDriver, isolatedDriver gorm.Dialector
		if dialect == "mysql" {
			dsn := os.Getenv("TEST_MYSQL_DSN")
			if dsn == "" {
				t.Skip("TEST_MYSQL_DSN is not configured")
			}
			configuration, err := sqlmysql.ParseDSN(dsn)
			require.NoError(t, err)
			require.Equal(t, "tcp", configuration.Net)
			host, _, err := net.SplitHostPort(configuration.Addr)
			require.NoError(t, err)
			require.True(t, net.ParseIP(host).IsLoopback(), "database tests require loopback instances")
			adminDriver = mysql.Open(dsn)
			configuration.DBName = name
			isolatedDriver = mysql.Open(configuration.FormatDSN())
		} else {
			dsn := os.Getenv("TEST_POSTGRES_DSN")
			if dsn == "" {
				t.Skip("TEST_POSTGRES_DSN is not configured")
			}
			parsed, err := url.Parse(dsn)
			require.NoError(t, err)
			require.True(t, net.ParseIP(parsed.Hostname()).IsLoopback(), "database tests require loopback instances")
			adminDriver = postgres.Open(dsn)
			parsed.Path = "/" + name
			isolatedDriver = postgres.Open(parsed.String())
		}
		admin, err := gorm.Open(adminDriver, configuration)
		require.NoError(t, err)
		adminSQL, err := admin.DB()
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, adminSQL.Close()) })
		createSQL := "CREATE DATABASE " + name
		if dialect == "mysql" {
			createSQL += " CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci"
		}
		require.NoError(t, admin.Exec(createSQL).Error)
		t.Cleanup(func() { require.NoError(t, admin.Exec("DROP DATABASE "+name).Error) })
		database, err = gorm.Open(isolatedDriver, configuration)
		require.NoError(t, err)
		t.Logf("isolated %s database: %s", dialect, name)
	}
	sqlDB, err := database.DB()
	require.NoError(t, err)
	if dialect == "sqlite" {
		sqlDB.SetMaxOpenConns(1)
	}
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	previousDB, previousType, previousMemoryCache := DB, common.MainDatabaseType(), common.MemoryCacheEnabled
	previousGroups, previousChannels, previousAdvanced := group2model2channels, channelsIDM, channel2advancedCustomConfig
	DB = database
	common.SetMainDatabaseType(common.DatabaseType(dialect))
	common.MemoryCacheEnabled = false
	initCol()
	t.Cleanup(func() {
		DB = previousDB
		common.SetMainDatabaseType(previousType)
		common.MemoryCacheEnabled = previousMemoryCache
		initCol()
		channelSyncLock.Lock()
		group2model2channels, channelsIDM, channel2advancedCustomConfig = previousGroups, previousChannels, previousAdvanced
		channelSyncLock.Unlock()
	})
	var version string
	query := "SELECT version()"
	if dialect == "sqlite" {
		query = "SELECT sqlite_version()"
	}
	require.NoError(t, database.Raw(query).Scan(&version).Error)
	t.Logf("%s version: %s", dialect, version)
	return database
}

func TestCodexGovernanceDatabaseMatrix(t *testing.T) {
	for _, dialect := range []string{"sqlite", "mysql", "postgres"} {
		for _, upgrade := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/upgrade_%t", dialect, upgrade), func(t *testing.T) {
				database := newCodexGovernanceTestDatabase(t, dialect)
				if upgrade {
					require.NoError(t, database.AutoMigrate(&Channel{}, &Ability{}))
					require.NoError(t, database.Create(&Channel{Id: 99, Type: constant.ChannelTypeOpenAI, Status: 1, Key: "existing", Models: "existing", Group: "default"}).Error)
					require.NoError(t, database.Create(&Ability{ChannelId: 99, Model: "existing", Group: "default", Enabled: false}).Error)
					compatibilityChannel := Channel{Id: 98, Type: constant.ChannelTypeCodex, Status: 1, Models: "legacy-model", Group: "default"}
					require.NoError(t, database.Create(&compatibilityChannel).Error)
					otherDatabase, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "other.db")), &gorm.Config{})
					require.NoError(t, err)
					otherSQL, err := otherDatabase.DB()
					require.NoError(t, err)
					t.Cleanup(func() { require.NoError(t, otherSQL.Close()) })
					require.NoError(t, otherDatabase.AutoMigrate(&CodexModelGovernanceRecord{}))
					DB = otherDatabase
					err = database.Transaction(func(tx *gorm.DB) error {
						if err := compatibilityChannel.AddAbilities(tx); err != nil {
							return err
						}
						if err := compatibilityChannel.UpdateAbilities(tx); err != nil {
							return err
						}
						return ReapplyCodexModelGovernanceWithDB(tx, []int{98})
					})
					DB = database
					require.NoError(t, err, "missing-table compatibility must use the transaction database, not global DB")
				}
				for range 2 {
					require.NoError(t, database.AutoMigrate(&Channel{}, &Ability{}, &CodexModelGovernanceRecord{}, &CodexModelGovernanceProbeState{}, &CodexModelGovernanceAlertCooldownRecord{}))
				}
				if upgrade {
					var existing Channel
					require.NoError(t, database.First(&existing, 99).Error)
					assert.Equal(t, "existing", existing.Key)
					var ability Ability
					require.NoError(t, database.First(&ability, "channel_id = ?", 99).Error)
					assert.False(t, ability.Enabled)
				}
				assert.True(t, database.Migrator().HasIndex(&CodexModelGovernanceRecord{}, "idx_codex_model_governance_records_model_name"))
				verifyCodexGovernanceLifecycle(t, database)
				verifyCodexGovernancePersistence(t, database)
			})
		}
	}
}

func verifyCodexGovernanceLifecycle(t *testing.T, database *gorm.DB) {
	t.Helper()
	priority, weight := int64(5), uint(1)
	channels := []Channel{
		{Id: 11, Type: constant.ChannelTypeCodex, Status: 1, Key: "key", Models: "configured-alias,healthy", Group: "default,vip", Priority: &priority, Weight: &weight, Tag: common.GetPointer("governance"), GroupPriority: common.GetPointer(`{"vip":{"priority":20,"weight":7}}`)},
		{Id: 12, Type: constant.ChannelTypeCodex, Status: 1, Models: "configured-alias,healthy", Group: "default,vip", Priority: &priority, Weight: &weight},
		{Id: 13, Type: constant.ChannelTypeOpenAI, Status: 1, Models: "configured-alias", Group: "default,vip", Priority: &priority, Weight: &weight},
		{Id: 14, Type: constant.ChannelTypeCodex, Status: 2, Models: "configured-alias", Group: "default,vip"},
	}
	require.NoError(t, database.Create(&channels).Error)
	for _, channel := range channels {
		require.NoError(t, channel.AddAbilities(nil))
	}
	canceledContext, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := channels[0].codexAbilityGovernanceByModel(database.WithContext(canceledContext), channels[0].GetModels())
	require.ErrorIs(t, err, context.Canceled, "database failures must not be treated as missing governance schema")
	listed, err := GetCodexModelGovernanceChannels(context.Background(), false)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(listed), 3)
	for _, channel := range listed {
		assert.Equal(t, constant.ChannelTypeCodex, channel.Type)
	}
	assert.Empty(t, listed[0].Key)
	record, err := UpsertCodexModelGovernancePending(CodexModelGovernancePendingInput{ModelName: "configured-alias", Source: CodexModelGovernanceSourceOfficialNotice})
	require.NoError(t, err)
	assert.False(t, record.AbilitiesDisabled)
	assert.Equal(t, []int{11, 12, 14}, DecodeCodexModelGovernanceChannelIDs(record.AffectedChannelIDs))
	require.NoError(t, ReviewCodexModelGovernanceRecord(record.ID, CodexModelGovernanceStatusIgnored, 42, "not applicable"))
	record, err = UpsertCodexModelGovernancePending(CodexModelGovernancePendingInput{ModelName: "configured-alias", Source: CodexModelGovernanceSourceProbe, AffectedChannelIDs: []int{11}, DisableAbilities: true, LastError: "api_key=sk-not-a-real-secret"})
	require.NoError(t, err)
	assert.Equal(t, []int{11}, CodexModelGovernanceDisabledChannelIDs(*record))
	assert.NotContains(t, record.LastError, "sk-not-a-real-secret")
	require.Error(t, database.Create(&CodexModelGovernanceRecord{ModelName: "configured-alias", Status: "active", Source: "manual"}).Error)
	require.NoError(t, channels[0].UpdateAbilities(nil))
	require.NoError(t, (&Channel{Id: 11}).UpdateAbilities(nil))
	require.NoError(t, UpdateAbilityStatus(11, true))
	require.NoError(t, UpdateAbilityStatusByTag("governance", true))
	require.True(t, UpdateChannelStatus(11, "", common.ChannelStatusAutoDisabled, "temporary upstream failure"))
	require.True(t, UpdateChannelStatus(11, "", common.ChannelStatusEnabled, "automatic recovery"))
	require.True(t, UpdateChannelStatusManually(11, common.ChannelStatusBanned, "operator decision"))
	require.True(t, UpdateChannelStatusManually(11, common.ChannelStatusEnabled, "operator recovery"))
	for _, cacheEnabled := range []bool{false, true} {
		common.MemoryCacheEnabled = cacheEnabled
		InitChannelCache()
		candidates, err := GetSatisfiedChannelCandidates("vip", "configured-alias", 0, nil)
		require.NoError(t, err)
		candidateIDs := make([]int, 0, len(candidates))
		for _, candidate := range candidates {
			candidateIDs = append(candidateIDs, candidate.Id)
		}
		assert.ElementsMatch(t, []int{12, 13}, candidateIDs)
		healthy, err := GetRandomSatisfiedChannel("vip", "healthy", 0, nil)
		require.NoError(t, err)
		require.NotNil(t, healthy)
		assert.Equal(t, 11, healthy.Id)
		assert.EqualValues(t, 20, healthy.GetPriorityForGroup("vip"))
		assert.EqualValues(t, 7, healthy.GetWeightForGroup("vip"))
	}
	require.NoError(t, RestoreCodexModelGovernanceAfterProbeSuccess("configured-alias", 11))
	record, err = GetCodexModelGovernanceRecord(record.ID)
	require.NoError(t, err)
	assert.Equal(t, CodexModelGovernanceStatusActive, record.Status)
	require.NoError(t, ReviewCodexModelGovernanceRecord(record.ID, CodexModelGovernanceStatusUnsupportedDisabled, 42, "confirmed"))
	require.Error(t, ReviewCodexModelGovernanceRecord(record.ID, CodexModelGovernanceStatusIgnored, 42, "cannot bypass disabled scope"))
	require.NoError(t, RestoreCodexModelGovernanceAfterProbeSuccess("configured-alias", 11))
	record, err = GetCodexModelGovernanceRecord(record.ID)
	require.NoError(t, err)
	assert.Equal(t, CodexModelGovernanceStatusUnsupportedDisabled, record.Status)
	assert.Equal(t, []int{12, 14}, CodexModelGovernanceDisabledChannelIDs(*record))
	require.NoError(t, database.Model(&Channel{}).Where("id = ?", 12).Update("status", 2).Error)
	require.Error(t, ReviewCodexModelGovernanceRecord(record.ID, CodexModelGovernanceStatusActive, 42, "no available channel to restore"))
	record, err = GetCodexModelGovernanceRecord(record.ID)
	require.NoError(t, err)
	assert.Equal(t, CodexModelGovernanceStatusUnsupportedDisabled, record.Status)
	assert.Equal(t, []int{12, 14}, CodexModelGovernanceDisabledChannelIDs(*record))
	var ability Ability
	require.NoError(t, database.First(&ability, "channel_id = ? AND model = ?", 12, "configured-alias").Error)
	assert.False(t, ability.Enabled)
	require.NoError(t, ReviewCodexModelGovernanceRecord(record.ID, CodexModelGovernanceStatusRemoved, 42, "confirmed removal"))
	var removedChannels []Channel
	require.NoError(t, database.Where("id IN ?", []int{11, 12, 14}).Find(&removedChannels).Error)
	for _, channel := range removedChannels {
		assert.NotContains(t, channel.Models, "configured-alias")
	}
	newChannel := Channel{Id: 15, Type: constant.ChannelTypeCodex, Status: 1, Models: "configured-alias", Group: "default,vip"}
	require.NoError(t, database.Create(&newChannel).Error)
	require.NoError(t, newChannel.AddAbilities(nil))
	record, err = UpsertCodexModelGovernancePending(CodexModelGovernancePendingInput{ModelName: "configured-alias", Source: CodexModelGovernanceSourceProbe, DisableAbilities: true, AffectedChannelIDs: []int{15}})
	require.NoError(t, err)
	assert.Equal(t, CodexModelGovernanceStatusRemoved, record.Status)
	assert.Contains(t, DecodeCodexModelGovernanceChannelIDs(record.AffectedChannelIDs), 15)
	require.NoError(t, database.First(&newChannel, 15).Error)
	assert.Empty(t, newChannel.Models)
	require.NoError(t, database.Model(&Channel{}).Where("id = ?", 11).Update("models", "configured-alias,healthy").Error)
	require.NoError(t, channels[0].UpdateAbilities(nil))
	var abilityCount int64
	require.NoError(t, database.Model(&Ability{}).Where("channel_id = ? AND model = ?", 11, "configured-alias").Count(&abilityCount).Error)
	assert.Zero(t, abilityCount)
	require.NoError(t, database.Model(&Ability{}).Where("channel_id = ? AND model = ?", 11, "healthy").Update("enabled", false).Error)
	require.Error(t, ReviewCodexModelGovernanceRecord(record.ID, CodexModelGovernanceStatusActive, 42, "removed models cannot be restored"))
	record, err = GetCodexModelGovernanceRecord(record.ID)
	require.NoError(t, err)
	assert.Equal(t, CodexModelGovernanceStatusRemoved, record.Status)
	assert.True(t, record.AbilitiesDisabled)
	assert.Equal(t, 42, record.ReviewedBy)
	require.NoError(t, database.Model(&Ability{}).Where("channel_id = ? AND model = ?", 11, "configured-alias").Count(&abilityCount).Error)
	assert.Zero(t, abilityCount)
	ability = Ability{}
	require.NoError(t, database.First(&ability, "channel_id = ? AND model = ?", 11, "healthy").Error)
	assert.False(t, ability.Enabled, "failed restoration must not release another model's suppression")
	var channel Channel
	require.NoError(t, database.First(&channel, 12).Error)
	assert.Equal(t, "healthy", channel.Models)
	channel = Channel{}
	require.NoError(t, database.First(&channel, 13).Error)
	assert.Equal(t, "configured-alias", channel.Models)
	common.MemoryCacheEnabled = false
}

func verifyCodexGovernancePersistence(t *testing.T, database *gorm.DB) {
	t.Helper()
	count, confirmed, err := RecordCodexModelGovernanceProbeUnsupportedFailure("persistent-model", 11, 2)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
	assert.False(t, confirmed)
	require.NoError(t, database.AutoMigrate(&CodexModelGovernanceRecord{}, &CodexModelGovernanceProbeState{}, &CodexModelGovernanceAlertCooldownRecord{}))
	count, confirmed, err = RecordCodexModelGovernanceProbeUnsupportedFailure("persistent-model", 11, 2)
	require.NoError(t, err)
	assert.Equal(t, 2, count)
	assert.True(t, confirmed)
	require.NoError(t, ResetCodexModelGovernanceProbeFailure("persistent-model", 11))
	count, confirmed, err = RecordCodexModelGovernanceProbeUnsupportedFailure("persistent-model", 11, 2)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
	assert.False(t, confirmed)
	var workers sync.WaitGroup
	errors := make(chan error, 6)
	for range 6 {
		workers.Go(func() {
			_, err := UpsertCodexModelGovernancePending(CodexModelGovernancePendingInput{ModelName: "concurrent-model", Source: CodexModelGovernanceSourceOfficialNotice})
			errors <- err
		})
	}
	workers.Wait()
	close(errors)
	for err := range errors {
		require.NoError(t, err)
	}
	var recordCount int64
	require.NoError(t, database.Model(&CodexModelGovernanceRecord{}).Where("model_name = ?", "concurrent-model").Count(&recordCount).Error)
	assert.EqualValues(t, 1, recordCount)
	reservations := make(chan *CodexModelGovernanceAlertCooldownReservation, 6)
	errors = make(chan error, 6)
	for workerID := range 6 {
		workers.Go(func() {
			reservation, allowed, err := ReserveCodexModelGovernanceAlertCooldown("persistent-alert", time.Hour, time.Minute, fmt.Sprint(workerID))
			errors <- err
			if allowed {
				reservations <- reservation
			}
		})
	}
	workers.Wait()
	close(errors)
	close(reservations)
	for err := range errors {
		require.NoError(t, err)
	}
	require.Len(t, reservations, 1)
	reservation := <-reservations
	stale := *reservation
	stale.ReservationToken = "different-owner"
	require.Error(t, CommitCodexModelGovernanceAlertCooldown(&stale))
	require.NoError(t, RollbackCodexModelGovernanceAlertCooldown(&stale))
	require.NoError(t, RollbackCodexModelGovernanceAlertCooldown(reservation))
	reservation, allowed, err := ReserveCodexModelGovernanceAlertCooldown("persistent-alert", time.Hour, time.Minute, "new-owner")
	require.NoError(t, err)
	require.True(t, allowed)
	require.NoError(t, CommitCodexModelGovernanceAlertCooldown(reservation))
	require.NoError(t, database.AutoMigrate(&CodexModelGovernanceAlertCooldownRecord{}))
	_, allowed, err = ReserveCodexModelGovernanceAlertCooldown("persistent-alert", time.Hour, time.Minute, "third-owner")
	require.NoError(t, err)
	assert.False(t, allowed)
}

func TestCodexGovernanceInterleavedAbilityWrites(test *testing.T) {
	scenarios := []struct {
		name             string
		governanceAction string
		abilityAction    string
		wantStatus       string
		restoreChannelID int
	}{
		{"disable/partial_rebuild", "disable", "rebuild", CodexModelGovernanceStatusUnsupportedDisabled, 0},
		{"auto_disable/partial_add_empty", "auto_disable", "add", CodexModelGovernanceStatusUnsupportedPendingReview, 0},
		{"disable/enable_abilities", "disable", "enable", CodexModelGovernanceStatusUnsupportedDisabled, 0},
		{"disable/enable_tag", "disable", "enable_tag", CodexModelGovernanceStatusUnsupportedDisabled, 0},
		{"restore/partial_rebuild", "restore", "rebuild", CodexModelGovernanceStatusUnsupportedPendingReview, 11},
		{"restore/enable_channel", "restore", "enable_channel", CodexModelGovernanceStatusUnsupportedPendingReview, 11},
		{"restore_other/partial_rebuild_tx", "restore", "rebuild_tx", CodexModelGovernanceStatusUnsupportedPendingReview, 12},
		{"remove/partial_rebuild", "remove", "rebuild", CodexModelGovernanceStatusRemoved, 0},
		{"remove/enable_channel", "remove", "enable_channel", CodexModelGovernanceStatusRemoved, 0},
	}
	for _, dialect := range []string{"mysql", "postgres"} {
		for _, scenario := range scenarios {
			test.Run(dialect+"/"+scenario.name, func(test *testing.T) {
				database := newCodexGovernanceTestDatabase(test, dialect)
				require.NoError(test, database.AutoMigrate(&Channel{}, &Ability{}, &CodexModelGovernanceRecord{}))
				channels := []Channel{
					{Id: 11, Type: constant.ChannelTypeCodex, Status: common.ChannelStatusEnabled, Models: "governed,healthy", Group: "default,vip", Tag: common.GetPointer("interleaving")},
					{Id: 12, Type: constant.ChannelTypeCodex, Status: common.ChannelStatusEnabled, Models: "governed,healthy", Group: "default,vip", Tag: common.GetPointer("interleaving")},
				}
				require.NoError(test, database.Create(&channels).Error)
				for _, channel := range channels {
					require.NoError(test, channel.AddAbilities(nil))
				}
				source := CodexModelGovernanceSourceOfficialNotice
				if scenario.governanceAction == "restore" {
					source = CodexModelGovernanceSourceProbe
				}
				record, err := UpsertCodexModelGovernancePending(CodexModelGovernancePendingInput{
					ModelName: "governed", Source: source, AffectedChannelIDs: []int{11, 12},
					DisableAbilities: scenario.governanceAction == "restore",
				})
				require.NoError(test, err)
				if scenario.abilityAction == "enable_channel" {
					require.True(test, UpdateChannelStatusManually(11, common.ChannelStatusManuallyDisabled, "interleaving fixture"))
				}
				if scenario.abilityAction == "add" {
					require.NoError(test, channels[0].DeleteAbilities())
				}
				governanceAction := func() error {
					switch scenario.governanceAction {
					case "auto_disable":
						_, err := UpsertCodexModelGovernancePending(CodexModelGovernancePendingInput{
							ModelName: "governed", Source: CodexModelGovernanceSourceProbe,
							AffectedChannelIDs: []int{11, 12}, DisableAbilities: true,
						})
						return err
					case "restore":
						return RestoreCodexModelGovernanceAfterProbeSuccess("governed", scenario.restoreChannelID)
					default:
						return ReviewCodexModelGovernanceRecord(record.ID, scenario.wantStatus, 42, "interleaved review")
					}
				}
				abilityAction := func() error {
					switch scenario.abilityAction {
					case "add":
						return (&Channel{Id: 11}).AddAbilities(nil)
					case "rebuild_tx":
						return DB.Transaction(func(tx *gorm.DB) error {
							var partial Channel
							if err := tx.Select("id", "models", "status", "group", "priority", "weight", "tag").First(&partial, 11).Error; err != nil {
								return err
							}
							return partial.UpdateAbilities(tx)
						})
					case "enable":
						return UpdateAbilityStatus(11, true)
					case "enable_tag":
						return UpdateAbilityStatusByTag("interleaving", true)
					case "enable_channel":
						if !UpdateChannelStatusManually(11, common.ChannelStatusEnabled, "interleaved enable") {
							return fmt.Errorf("channel enable failed")
						}
						return nil
					default:
						return (&Channel{Id: 11}).UpdateAbilities(nil)
					}
				}
				runCodexGovernanceAbilityInterleaving(test, database, governanceAction, abilityAction)
				record, err = GetCodexModelGovernanceRecord(record.ID)
				require.NoError(test, err)
				assert.Equal(test, scenario.wantStatus, record.Status)
				assert.Equal(test, []int{11, 12}, DecodeCodexModelGovernanceChannelIDs(record.AffectedChannelIDs))
				wantDisabledChannelIDs := []int{11, 12}
				if scenario.governanceAction == "restore" {
					wantDisabledChannelIDs = []int{12}
					if scenario.restoreChannelID == 12 {
						wantDisabledChannelIDs = []int{11}
					}
				}
				assert.Equal(test, wantDisabledChannelIDs, CodexModelGovernanceDisabledChannelIDs(*record))
				assert.True(test, record.AbilitiesDisabled)
				for _, channelID := range []int{11, 12} {
					var channel Channel
					require.NoError(test, database.First(&channel, channelID).Error)
					assert.Equal(test, common.ChannelStatusEnabled, channel.Status)
					wantModels := "governed,healthy"
					wantKeys := []string{"default:governed", "vip:governed", "default:healthy", "vip:healthy"}
					if scenario.governanceAction == "remove" {
						wantModels = "healthy"
						wantKeys = []string{"default:healthy", "vip:healthy"}
					}
					assert.Equal(test, wantModels, channel.Models)
					var abilities []Ability
					require.NoError(test, database.Where("channel_id = ?", channelID).Find(&abilities).Error)
					keys := make([]string, 0, len(abilities))
					for _, ability := range abilities {
						keys = append(keys, ability.Group+":"+ability.Model)
						wantEnabled := ability.Model == "healthy" || (scenario.governanceAction == "restore" && channelID == scenario.restoreChannelID)
						assert.Equal(test, wantEnabled, ability.Enabled, "channel %d, %s/%s", channelID, ability.Group, ability.Model)
					}
					assert.ElementsMatch(test, wantKeys, keys)
				}
			})
		}
	}
}

func runCodexGovernanceAbilityInterleaving(test *testing.T, database *gorm.DB, governanceAction, abilityAction func() error) {
	test.Helper()
	operationContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	previousDB := DB
	DB = database.WithContext(operationContext)
	governanceRecordLocked := make(chan struct{})
	abilityChannelLocked := make(chan struct{})
	governanceChannelRequested := make(chan struct{})
	resumeGovernance := make(chan struct{})
	resumeAbility := make(chan struct{})
	abilityFinished := make(chan struct{})
	var recordCaptured, governanceResumed, channelRequested atomic.Bool
	var recordOnce, abilityOnce, channelOnce sync.Once
	var workers sync.WaitGroup
	const afterLockCallback = "test:codex_governance_after_lock"
	const channelRequestCallback = "test:codex_governance_channel_request"
	const recordWriteCallback = "test:codex_governance_record_write"
	test.Cleanup(func() {
		cancel()
		workers.Wait()
		DB = previousDB
		require.NoError(test, database.Callback().Query().Remove(afterLockCallback))
		require.NoError(test, database.Callback().Query().Remove(channelRequestCallback))
		require.NoError(test, database.Callback().Update().Remove(recordWriteCallback))
	})
	require.NoError(test, database.Callback().Query().After("gorm:after_query").Register(afterLockCallback, func(tx *gorm.DB) {
		if _, locked := tx.Statement.Clauses["FOR"]; !locked || tx.Error != nil {
			return
		}
		if tx.Statement.Table == "codex_model_governance_records" {
			if _, record := tx.Statement.Dest.(*CodexModelGovernanceRecord); record {
				recordOnce.Do(func() {
					recordCaptured.Store(true)
					close(governanceRecordLocked)
					select {
					case <-resumeGovernance:
					case <-operationContext.Done():
						tx.AddError(operationContext.Err())
					}
				})
			}
		} else if tx.Statement.Table == "channels" && recordCaptured.Load() && !governanceResumed.Load() {
			abilityOnce.Do(func() {
				close(abilityChannelLocked)
				select {
				case <-resumeAbility:
				case <-operationContext.Done():
					tx.AddError(operationContext.Err())
				}
			})
		}
	}))
	require.NoError(test, database.Callback().Query().Before("gorm:query").Register(channelRequestCallback, func(tx *gorm.DB) {
		if tx.Statement.Table == "channels" && governanceResumed.Load() {
			if _, locked := tx.Statement.Clauses["FOR"]; locked {
				channelOnce.Do(func() {
					channelRequested.Store(true)
					close(governanceChannelRequested)
				})
			}
		}
	}))
	require.NoError(test, database.Callback().Update().Before("gorm:update").Register(recordWriteCallback, func(tx *gorm.DB) {
		if tx.Statement.Table == "codex_model_governance_records" && channelRequested.Load() {
			select {
			case <-abilityFinished:
			case <-operationContext.Done():
				tx.AddError(operationContext.Err())
			}
		}
	}))
	governanceResult := make(chan error, 1)
	abilityResult := make(chan error, 1)
	workers.Go(func() { governanceResult <- governanceAction() })
	waitCodexGovernanceInterleavingStep(test, operationContext, governanceRecordLocked, "governance record lock")
	workers.Go(func() {
		err := abilityAction()
		close(abilityFinished)
		abilityResult <- err
	})
	waitCodexGovernanceInterleavingStep(test, operationContext, abilityChannelLocked, "ability writer channel lock")
	governanceResumed.Store(true)
	close(resumeGovernance)
	waitCodexGovernanceInterleavingStep(test, operationContext, governanceChannelRequested, "governance channel lock request")
	test.Log("forced interleaving: governance holds record; ability writer holds channel; governance requests channel")
	close(resumeAbility)
	for _, result := range []struct {
		name   string
		result <-chan error
	}{{"ability writer", abilityResult}, {"governance writer", governanceResult}} {
		select {
		case err := <-result.result:
			require.NoError(test, err, result.name)
		case <-operationContext.Done():
			test.Fatalf("%s did not finish: %v", result.name, operationContext.Err())
		}
	}
}

func waitCodexGovernanceInterleavingStep(test *testing.T, operationContext context.Context, reached <-chan struct{}, name string) {
	test.Helper()
	select {
	case <-reached:
	case <-operationContext.Done():
		test.Fatalf("%s was not reached: %v", name, operationContext.Err())
	}
}
