package model

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaydto "github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

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
	assert.Same(t, initialCached, cachedAfterNoop, "a rejected key update must not rebuild the full channel cache")
	assert.Empty(t, cachedAfterNoop.ChannelInfo.MultiKeyStatusList)

	assert.True(t, UpdateChannelStatus(channel.Id, "key-a", common.ChannelStatusAutoDisabled, "provider rejected key"))
	cachedAfterChange, err := CacheGetChannel(channel.Id)
	require.NoError(t, err)
	assert.NotSame(t, initialCached, cachedAfterChange, "a persisted key update must refresh the channel cache")
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
					Key: "key-a\nkey-b", Status: 4, Models: "guard-model", Group: "default",
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
				for _, status := range []int{1, 2, 3, 4} {
					assert.False(t, UpdateChannelStatus(channel.Id, "key-b", status, "automatic task"))
				}
				stored, err := GetChannelById(channel.Id, true)
				require.NoError(t, err)
				assert.Equal(t, 4, stored.Status)
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
	assert.False(t, UpdateChannelStatus(channel.Id, "key-a", 4, "automatic task"))
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
	require.True(t, UpdateChannelStatusManually(channel.Id, 4, "manual ban"))
	stale.Models = "updated-model"
	require.NoError(t, stale.Update())
	stored, err := GetChannelById(channel.Id, true)
	require.NoError(t, err)
	assert.Equal(t, 4, stored.Status)
	assert.Equal(t, "updated-model", stored.Models)
	assert.Equal(t, "manual ban", stored.GetOtherInfo()["status_reason"])
	assert.Equal(t, "keep", stored.GetOtherInfo()["provider_note"])
	candidates, err := GetSatisfiedChannelCandidates("default", "updated-model", 0, nil)
	require.NoError(t, err)
	assert.Empty(t, candidates)
}

func TestSaveStatusStateFromSingleKeySnapshotPreservesUnownedColumns(t *testing.T) {
	setupChannelStatusTest(t)

	channel := Channel{
		Name:        "single-key-status",
		Key:         "original-key",
		Status:      common.ChannelStatusEnabled,
		Models:      "original-model",
		Group:       "default",
		UsedQuota:   100,
		ChannelInfo: ChannelInfo{},
	}
	require.NoError(t, DB.Create(&channel).Error)

	stale, err := GetChannelById(channel.Id, true)
	require.NoError(t, err)

	concurrentChannelInfo := ChannelInfo{
		IsMultiKey:           true,
		MultiKeySize:         2,
		MultiKeyMode:         constant.MultiKeyModePolling,
		MultiKeyPollingIndex: 1,
	}
	require.NoError(t, DB.Model(&Channel{}).Where("id = ?", channel.Id).Updates(map[string]any{
		"key":          "rotated-key",
		"used_quota":   gorm.Expr("used_quota + ?", 250),
		"models":       "concurrent-model",
		"channel_info": concurrentChannelInfo,
	}).Error)

	stale.Status = common.ChannelStatusManuallyDisabled
	stale.SetOtherInfo(map[string]any{
		"status_reason": "manual operation",
		"status_time":   int64(1234),
	})
	require.NoError(t, stale.saveStatusState())

	var stored Channel
	require.NoError(t, DB.First(&stored, channel.Id).Error)
	assert.Equal(t, common.ChannelStatusManuallyDisabled, stored.Status)
	assert.Equal(t, "rotated-key", stored.Key)
	assert.Equal(t, int64(350), stored.UsedQuota)
	assert.Equal(t, "concurrent-model", stored.Models)
	assert.Equal(t, concurrentChannelInfo, stored.ChannelInfo)

	otherInfo := stored.GetOtherInfo()
	assert.Equal(t, "manual operation", otherInfo["status_reason"])
	assert.Equal(t, float64(1234), otherInfo["status_time"])
}

func setupChannelStatusRedisTest(test *testing.T, database int) (context.Context, *redis.Client, string) {
	test.Helper()
	address := os.Getenv("TEST_REDIS_ADDR")
	if address == "" {
		test.Skip("TEST_REDIS_ADDR is required for real Redis verification")
	}
	host, _, err := net.SplitHostPort(address)
	require.NoError(test, err)
	require.True(test, net.ParseIP(host).IsLoopback(), "Redis tests require a loopback instance")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	test.Cleanup(cancel)
	publisher := redis.NewClient(&redis.Options{Addr: address, DB: database})
	observer := redis.NewClient(&redis.Options{Addr: address, DB: database})
	test.Cleanup(func() {
		require.NoError(test, observer.Close())
		require.NoError(test, publisher.Close())
	})
	require.NoError(test, publisher.Ping(ctx).Err())
	require.NoError(test, observer.Ping(ctx).Err())
	version, err := observer.Info(ctx, "server").Result()
	require.NoError(test, err)
	for line := range strings.SplitSeq(version, "\n") {
		if version, found := strings.CutPrefix(line, "redis_version:"); found {
			test.Logf("Redis version=%s; two loopback clients; private DB/topic=%d", strings.TrimSpace(version), database)
		}
	}
	previousRedisEnabled, previousClient := common.RedisEnabled, common.RDB
	common.RedisEnabled, common.RDB = true, publisher
	test.Cleanup(func() {
		common.RedisEnabled, common.RDB = previousRedisEnabled, previousClient
	})
	return ctx, observer, fmt.Sprintf("new-api:config-changed:db:%d", database)
}

func receiveChannelStatusNotificationsThroughBarrier(test *testing.T, ctx context.Context, observer *redis.Client, subscription *redis.PubSub, topic string) []string {
	test.Helper()
	barrier := "channel-status-producer-barrier:" + test.Name()
	recipients, err := observer.Publish(ctx, topic, barrier).Result()
	require.NoError(test, err)
	require.Positive(test, recipients, "the confirmed subscriber must still be registered")
	var payloads []string
	for {
		message, err := subscription.ReceiveMessage(ctx)
		require.NoError(test, err)
		require.Equal(test, topic, message.Channel)
		if message.Payload == barrier {
			return payloads
		}
		payloads = append(payloads, message.Payload)
	}
}

func TestChannelStatusNotificationsRealRedisProducer(test *testing.T) {
	ctx, observer, topic := setupChannelStatusRedisTest(test, 13)
	var sqliteVersion string
	require.NoError(test, DB.Raw("SELECT sqlite_version()").Scan(&sqliteVersion).Error)
	test.Logf("SQLite version=%s; existing in-memory fixture", sqliteVersion)

	for _, testCase := range []struct {
		name              string
		initialStatus     int
		targetStatus      int
		manual            bool
		multiKey          bool
		failAbilityUpdate bool
		wantChanged       bool
		wantStatus        int
		wantClose         bool
	}{
		{
			name:          "automatic_permanent_ban_creation_rejected",
			initialStatus: common.ChannelStatusEnabled, targetStatus: common.ChannelStatusBanned,
			wantStatus: common.ChannelStatusEnabled,
		},
		{
			name:          "automatic_permanent_ban_overwrite_rejected",
			initialStatus: common.ChannelStatusBanned, targetStatus: common.ChannelStatusEnabled,
			wantStatus: common.ChannelStatusBanned,
		},
		{
			name:          "multi_key_disable_keeps_channel_enabled_without_close",
			initialStatus: common.ChannelStatusEnabled, targetStatus: common.ChannelStatusAutoDisabled,
			multiKey: true, wantChanged: true, wantStatus: common.ChannelStatusEnabled,
		},
		{
			name:          "manual_permanent_ban_publishes_channel_close",
			initialStatus: common.ChannelStatusEnabled, targetStatus: common.ChannelStatusBanned,
			manual: true, wantChanged: true, wantStatus: common.ChannelStatusBanned, wantClose: true,
		},
		{
			name:          "manual_enable_publishes_cache_only",
			initialStatus: common.ChannelStatusBanned, targetStatus: common.ChannelStatusEnabled,
			manual: true, wantChanged: true, wantStatus: common.ChannelStatusEnabled,
		},
		{
			name:          "transaction_rollback_does_not_publish",
			initialStatus: common.ChannelStatusEnabled, targetStatus: common.ChannelStatusBanned,
			manual: true, failAbilityUpdate: true, wantStatus: common.ChannelStatusEnabled,
		},
	} {
		test.Run(testCase.name, func(test *testing.T) {
			setupChannelStatusTest(test)
			channel := Channel{
				Name: testCase.name, Type: constant.ChannelTypeOpenAI,
				Key: "producer-key-a\nproducer-key-b", Status: testCase.initialStatus,
				Models: "producer-model", Group: "default",
			}
			if testCase.multiKey {
				channel.ChannelInfo = ChannelInfo{
					IsMultiKey: true, MultiKeySize: 2, MultiKeyMode: constant.MultiKeyModePolling,
				}
			}
			channel.SetOtherInfo(map[string]any{"status_reason": "original reason", "status_time": int64(1234)})
			require.NoError(test, DB.Create(&channel).Error)
			ability := Ability{
				ChannelId: channel.Id, Model: "producer-model", Group: "default",
				Enabled: testCase.initialStatus == common.ChannelStatusEnabled,
			}
			require.NoError(test, DB.Create(&ability).Error)
			subscription := observer.Subscribe(ctx, topic)
			test.Cleanup(func() { require.NoError(test, subscription.Close()) })
			confirmation, err := subscription.Receive(ctx)
			require.NoError(test, err)
			require.Equal(test, &redis.Subscription{Kind: "subscribe", Channel: topic, Count: 1}, confirmation)

			callbackTriggered := false
			var uncommittedChannel Channel
			var uncommittedReadError error
			if testCase.failAbilityUpdate {
				const callbackName = "test:channel_status_notification_rollback"
				forcedError := errors.New("forced ability update failure")
				require.NoError(test, DB.Callback().Update().Before("gorm:update").Register(callbackName, func(transaction *gorm.DB) {
					if transaction.Statement.Table == "abilities" {
						callbackTriggered = true
						uncommittedReadError = transaction.Session(&gorm.Session{NewDB: true}).First(&uncommittedChannel, channel.Id).Error
						transaction.AddError(forcedError)
					}
				}))
				test.Cleanup(func() { require.NoError(test, DB.Callback().Update().Remove(callbackName)) })
			}
			const reason = "producer reason must not enter Redis"
			var changed bool
			if testCase.manual {
				changed = UpdateChannelStatusManually(channel.Id, testCase.targetStatus, reason)
			} else {
				changed = UpdateChannelStatus(channel.Id, "producer-key-a", testCase.targetStatus, reason)
			}
			assert.Equal(test, testCase.wantChanged, changed)
			if testCase.failAbilityUpdate {
				require.True(test, callbackTriggered)
				require.NoError(test, uncommittedReadError)
				assert.Equal(test, common.ChannelStatusBanned, uncommittedChannel.Status, "failure follows the uncommitted channel write")
			}

			payloads := receiveChannelStatusNotificationsThroughBarrier(test, ctx, observer, subscription, topic)
			if testCase.wantChanged {
				require.Len(test, payloads, 1)
				var event struct {
					Scope           string `json:"scope"`
					Source          string `json:"source"`
					CloseChannelIDs []int  `json:"close_channel_ids"`
				}
				require.NoError(test, common.UnmarshalJsonStr(payloads[0], &event))
				assert.Equal(test, common.ConfigScopeChannels, event.Scope)
				assert.Equal(test, common.GetReplicaID(), event.Source)
				if testCase.wantClose {
					assert.Equal(test, []int{channel.Id}, event.CloseChannelIDs)
				} else {
					assert.Empty(test, event.CloseChannelIDs, "an enabled channel must not close all of its sessions")
				}
				assert.NotContains(test, payloads[0], "producer-key")
				assert.NotContains(test, payloads[0], reason)
			} else {
				assert.Empty(test, payloads, "rejected or rolled-back writes must not publish before the broker barrier")
			}
			stored, err := GetChannelById(channel.Id, true)
			require.NoError(test, err)
			assert.Equal(test, testCase.wantStatus, stored.Status)
			if !testCase.wantChanged {
				assert.Equal(test, channel.OtherInfo, stored.OtherInfo)
				assert.Equal(test, channel.ChannelInfo, stored.ChannelInfo)
			}
			if testCase.multiKey {
				assert.Equal(test, common.ChannelStatusAutoDisabled, stored.ChannelInfo.MultiKeyStatusList[0])
				assert.NotContains(test, stored.ChannelInfo.MultiKeyStatusList, 1)
			}
			var storedAbility Ability
			require.NoError(test, DB.Where("channel_id = ?", channel.Id).First(&storedAbility).Error)
			assert.Equal(test, testCase.wantStatus == common.ChannelStatusEnabled, storedAbility.Enabled)
		})
	}
}

func TestChannelTagNotificationsRealRedisProducer(test *testing.T) {
	ctx, observer, topic := setupChannelStatusRedisTest(test, 12)
	for _, dialect := range []string{"sqlite", "mysql", "postgres"} {
		test.Run(dialect, func(test *testing.T) {
			database := newCodexGovernanceTestDatabase(test, dialect)
			require.NoError(test, database.AutoMigrate(&Channel{}, &Ability{}))
			common.MemoryCacheEnabled = true
			for _, testCase := range []struct {
				name      string
				enable    bool
				failTable string
			}{
				{name: "enable_banned_tag_publishes_cache_only", enable: true},
				{name: "disable_tag_publishes_only_disabled_target_ids"},
				{name: "enable_channel_error_does_not_publish", enable: true, failTable: "channels"},
				{name: "enable_ability_error_rolls_back_without_publish", enable: true, failTable: "abilities"},
				{name: "disable_channel_error_does_not_publish", failTable: "channels"},
				{name: "disable_ability_error_rolls_back_without_publish", failTable: "abilities"},
			} {
				test.Run(testCase.name, func(test *testing.T) {
					require.NoError(test, database.Exec("DELETE FROM abilities").Error)
					require.NoError(test, database.Exec("DELETE FROM channels").Error)
					tag := "notification-tag"
					otherTag := "unrelated-tag"
					firstStatus := common.ChannelStatusEnabled
					if testCase.enable {
						firstStatus = common.ChannelStatusBanned
					}
					channels := []Channel{
						{
							Name: "partial-key-target", Type: constant.ChannelTypeCodex, Tag: &tag,
							Key: "tag-key-a\ntag-key-b", Status: firstStatus, Models: "tag-model", Group: "default",
							ChannelInfo: ChannelInfo{IsMultiKey: true, MultiKeySize: 2, MultiKeyMode: constant.MultiKeyModePolling,
								MultiKeyStatusList: map[int]int{0: common.ChannelStatusAutoDisabled}},
						},
						{
							Name: "exhausted-target", Type: constant.ChannelTypeCodex, Tag: &tag,
							Key: "exhausted-key", Status: common.ChannelStatusAutoDisabled, Models: "tag-model", Group: "default",
							ChannelInfo: ChannelInfo{IsMultiKey: true, MultiKeySize: 1, MultiKeyMode: constant.MultiKeyModePolling,
								MultiKeyStatusList: map[int]int{0: common.ChannelStatusAutoDisabled}},
						},
						{
							Name: "unrelated-enabled", Type: constant.ChannelTypeOpenAI, Tag: &otherTag,
							Key: "unrelated-key", Status: common.ChannelStatusEnabled, Models: "tag-model", Group: "default",
						},
					}
					channels[0].SetSetting(relaydto.ChannelSettings{CodexFingerprintMode: relaydto.CodexFingerprintModeFull})
					channels[1].SetSetting(relaydto.ChannelSettings{CodexFingerprintMode: relaydto.CodexFingerprintModeOff})
					channels[1].SetOtherInfo(map[string]any{
						"status_reason": ChannelStatusReasonAllKeysDisabled, "status_time": int64(1234),
					})
					for index := range channels {
						require.NoError(test, database.Create(&channels[index]).Error)
						ability := Ability{ChannelId: channels[index].Id, Model: "tag-model", Group: "default", Tag: channels[index].Tag,
							Enabled: channels[index].Status == common.ChannelStatusEnabled}
						require.NoError(test, database.Create(&ability).Error)
					}
					InitChannelCache()
					subscription := observer.Subscribe(ctx, topic)
					test.Cleanup(func() { require.NoError(test, subscription.Close()) })
					confirmation, err := subscription.Receive(ctx)
					require.NoError(test, err)
					require.Equal(test, &redis.Subscription{Kind: "subscribe", Channel: topic, Count: 1}, confirmation)
					forcedError := errors.New("forced tag write failure")
					callbackTriggered := false
					var callbackReadError error
					if testCase.failTable != "" {
						const callbackName = "test:channel_tag_notification_rollback"
						require.NoError(test, database.Callback().Update().Before("gorm:update").Register(callbackName, func(transaction *gorm.DB) {
							if transaction.Statement.Table == testCase.failTable {
								if testCase.failTable == "abilities" && !testCase.enable {
									var firstTarget Channel
									callbackReadError = transaction.Session(&gorm.Session{NewDB: true}).First(&firstTarget, channels[0].Id).Error
									if callbackReadError != nil {
										transaction.AddError(callbackReadError)
										return
									}
									if firstTarget.Status != common.ChannelStatusManuallyDisabled {
										return
									}
								}
								callbackTriggered = true
								transaction.AddError(forcedError)
							}
						}))
						test.Cleanup(func() { require.NoError(test, database.Callback().Update().Remove(callbackName)) })
					}
					if testCase.enable {
						err = EnableChannelByTag(tag)
					} else {
						err = DisableChannelByTag(tag)
					}
					if testCase.failTable == "" {
						require.NoError(test, err)
					} else {
						require.Error(test, err)
						require.True(test, callbackTriggered)
						require.NoError(test, callbackReadError)
					}
					payloads := receiveChannelStatusNotificationsThroughBarrier(test, ctx, observer, subscription, topic)
					if testCase.failTable != "" {
						assert.Empty(test, payloads, "failed tag operations must not emit an earlier per-channel event")
					} else {
						require.Len(test, payloads, 1, "one event after the whole tag operation commits")
						var event struct {
							Scope           string `json:"scope"`
							Source          string `json:"source"`
							CloseChannelIDs []int  `json:"close_channel_ids"`
						}
						require.NoError(test, common.UnmarshalJsonStr(payloads[0], &event))
						assert.Equal(test, common.ConfigScopeChannels, event.Scope)
						assert.Equal(test, common.GetReplicaID(), event.Source)
						if testCase.enable {
							assert.Empty(test, event.CloseChannelIDs)
						} else {
							assert.Equal(test, []int{channels[0].Id, channels[1].Id}, event.CloseChannelIDs)
						}
						assert.NotContains(test, payloads[0], "tag-key")
						assert.NotContains(test, payloads[0], "exhausted-key")
					}
					for index := range channels {
						stored, err := GetChannelById(channels[index].Id, true)
						require.NoError(test, err)
						wantStatus := channels[index].Status
						if index < 2 && testCase.failTable == "" {
							wantStatus = common.ChannelStatusManuallyDisabled
							if testCase.enable {
								wantStatus = common.ChannelStatusEnabled
							}
						}
						assert.Equal(test, wantStatus, stored.Status)
						assert.Equal(test, channels[index].ChannelInfo, stored.ChannelInfo)
						cached, err := CacheGetChannel(channels[index].Id)
						require.NoError(test, err)
						assert.Equal(test, wantStatus, cached.Status, "cache follows successful tag commits")
						if testCase.failTable != "" {
							assert.Equal(test, channels[index].OtherInfo, stored.OtherInfo)
							assert.Equal(test, channels[index].CodexFingerprintSeed, stored.CodexFingerprintSeed)
						}
						if index == 0 && testCase.enable && testCase.failTable == "" {
							assert.NotEmpty(test, stored.CodexFingerprintSeed)
						}
						if index == 1 {
							assert.Empty(test, stored.CodexFingerprintSeed, "off remains a fingerprint kill-switch")
							if !testCase.enable && testCase.failTable == "" {
								assert.Equal(test, "manual tag operation", stored.GetOtherInfo()["status_reason"])
							}
						}
						var storedAbility Ability
						require.NoError(test, database.Where("channel_id = ?", channels[index].Id).First(&storedAbility).Error)
						assert.Equal(test, wantStatus == common.ChannelStatusEnabled, storedAbility.Enabled)
					}
				})
			}
		})
	}
}
