package controller

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func channelStatusDatabase(t *testing.T, kind, dsn string) (*gorm.DB, int) {
	t.Helper()
	database, isolatedDSN := newAuditTestDatabase(t, kind, dsn)
	require.NoError(t, database.AutoMigrate(&model.Option{}))
	previousDB, previousLogDB := model.DB, model.LOG_DB
	previousType, previousLogType := common.MainDatabaseType(), common.LogDatabaseType()
	previousMaster, previousCache, previousRedis, previousSQLite := common.IsMasterNode, common.MemoryCacheEnabled, common.RedisEnabled, common.SQLitePath
	t.Cleanup(func() {
		connection, err := database.DB()
		require.NoError(t, err)
		require.NoError(t, connection.Close())
		model.DB, model.LOG_DB = previousDB, previousLogDB
		common.SetDatabaseTypes(previousType, previousLogType)
		common.IsMasterNode, common.MemoryCacheEnabled, common.RedisEnabled, common.SQLitePath = previousMaster, previousCache, previousRedis, previousSQLite
		if model.DB != nil {
			model.InitChannelCache()
		}
	})
	common.IsMasterNode, common.MemoryCacheEnabled, common.RedisEnabled = false, false, false
	if kind == "sqlite" {
		common.SQLitePath, isolatedDSN = isolatedDSN, ""
	}
	t.Setenv("SQL_DSN", isolatedDSN)
	t.Setenv("LOG_SQL_DSN", "")
	require.NoError(t, model.InitDB())
	database = model.DB
	model.LOG_DB = database
	common.SetLogDatabaseType(common.MainDatabaseType())
	require.NoError(t, database.AutoMigrate(&model.Channel{}, &model.Ability{}, &model.User{}, &model.AuditLog{}, &model.CodexModelGovernanceRecord{}))
	query := "SELECT version()"
	if kind == "sqlite" {
		query = "SELECT sqlite_version()"
	}
	var version string
	require.NoError(t, database.Raw(query).Scan(&version).Error)
	t.Logf("database=%s version=%s", kind, version)
	root := &model.User{Username: "channel-status-root", Role: common.RoleRootUser, Status: common.UserStatusEnabled}
	require.NoError(t, database.Create(root).Error)
	return database, root.Id
}

func channelStatusRequest(t *testing.T, userID int, handler gin.HandlerFunc, path string, body any, output any) {
	t.Helper()
	payload, err := common.Marshal(body)
	require.NoError(t, err)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Set("id", userID)
	context.Set("role", common.RoleRootUser)
	context.Params = gin.Params{{Key: "id", Value: filepath.Base(filepath.Dir(path))}}
	context.Request = httptest.NewRequest(http.MethodPost, path, bytes.NewReader(payload))
	context.Request.Header.Set("Content-Type", "application/json")
	handler(context)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), output), recorder.Body.String())
}

func TestPermanentChannelStatusDatabaseMatrix(t *testing.T) {
	for _, dialect := range []struct{ kind, env string }{{"sqlite", ""}, {"mysql", "TEST_MYSQL_DSN"}, {"postgres", "TEST_POSTGRES_DSN"}} {
		t.Run(dialect.kind, func(t *testing.T) {
			if dialect.env != "" && os.Getenv(dialect.env) == "" {
				t.Skip("set " + dialect.env + " to run this database")
			}
			_, rootID := channelStatusDatabase(t, dialect.kind, os.Getenv(dialect.env))
			for _, cacheEnabled := range []bool{false, true} {
				for _, channelType := range []int{1, 57} {
					for _, multiKey := range []bool{false, true} {
						t.Run(fmt.Sprintf("cache=%t/type=%d/multi_key=%t", cacheEnabled, channelType, multiKey), func(t *testing.T) {
							common.MemoryCacheEnabled = cacheEnabled
							channel := &model.Channel{Name: t.Name(), Type: channelType, Key: "key-one\nkey-two", Status: 1,
								Models: "permanent-status-model", Group: "default,vip", UsedQuota: 77,
								Priority: common.GetPointer(int64(11)), Weight: common.GetPointer(uint(3)),
								GroupPriority: common.GetPointer(`{"vip":{"priority":50,"weight":9}}`),
								ChannelInfo: model.ChannelInfo{IsMultiKey: multiKey, MultiKeySize: 2, MultiKeyMode: "polling", MultiKeyPollingIndex: 1,
									MultiKeyStatusList: map[int]int{0: 3}, MultiKeyDisabledReason: map[int]string{0: "key failure"}, MultiKeyDisabledTime: map[int]int64{0: 1234}},
							}
							require.NoError(t, channel.Insert())
							stale, err := model.GetChannelById(channel.Id, true)
							require.NoError(t, err)
							t.Cleanup(func() { require.NoError(t, channel.Delete()); model.InitChannelCache() })
							path := "/api/channel/" + strconv.Itoa(channel.Id) + "/status"
							var response struct {
								Success bool `json:"success"`
								Data    bool `json:"data"`
							}
							channelStatusRequest(t, rootID, UpdateChannelStatus, path, ChannelStatusRequest{Status: 4}, &response)
							require.True(t, response.Success)
							require.True(t, response.Data)
							if channelType == 1 {
								stale.Name = "Updated while permanently unavailable"
								require.NoError(t, stale.Update())
								model.InitChannelCache()
							}
							banned, err := model.GetChannelById(channel.Id, true)
							require.NoError(t, err)
							assert.Equal(t, 4, banned.Status)
							assert.Equal(t, channel.ChannelInfo, banned.ChannelInfo)
							candidate, err := model.GetRandomSatisfiedChannel("vip", "permanent-status-model", 0, nil)
							require.NoError(t, err)
							assert.Nil(t, candidate)
							if cacheEnabled {
								cached, err := model.CacheGetChannel(channel.Id)
								require.NoError(t, err)
								assert.Equal(t, 4, cached.Status)
								model.CacheUpdateChannelStatus(channel.Id, 1)
							}
							for _, status := range []int{1, 2, 3, 4} {
								assert.False(t, model.UpdateChannelStatus(channel.Id, "key-two", status, "automatic task"))
							}
							unchanged, err := model.GetChannelById(channel.Id, true)
							require.NoError(t, err)
							assert.Equal(t, banned, unchanged)
							for _, status := range []int{2, 4, 1} {
								channelStatusRequest(t, rootID, UpdateChannelStatus, path, ChannelStatusRequest{Status: status}, &response)
								require.True(t, response.Success)
								require.True(t, response.Data)
							}
							restored, err := model.GetChannelById(channel.Id, true)
							require.NoError(t, err)
							assert.Equal(t, 1, restored.Status)
							assert.Equal(t, channel.ChannelInfo, restored.ChannelInfo)
							assert.Equal(t, channel.Key, restored.Key)
							assert.Equal(t, int64(77), restored.UsedQuota)
							assert.Equal(t, channel.GroupPriority, restored.GroupPriority)
							assert.Equal(t, int64(50), restored.GetPriorityForGroup("vip"))
							candidate, err = model.GetRandomSatisfiedChannel("vip", "permanent-status-model", 0, nil)
							require.NoError(t, err)
							require.NotNil(t, candidate)
							assert.Equal(t, channel.Id, candidate.Id)
							assert.False(t, model.UpdateChannelStatus(channel.Id, "key-two", 4, "automatic task"))
						})
					}
				}
			}
			t.Run("manual batch and tag override", func(t *testing.T) {
				common.MemoryCacheEnabled = true
				tag := "manual-permanent-status"
				var ids []int
				for _, channelType := range []int{1, 57} {
					channel := &model.Channel{Name: tag, Type: channelType, Key: "key", Status: 4, Models: "batch-model", Group: "default", Tag: &tag}
					require.NoError(t, channel.Insert())
					t.Cleanup(func() { require.NoError(t, channel.Delete()); model.InitChannelCache() })
					ids = append(ids, channel.Id)
				}
				var response struct {
					Success bool `json:"success"`
					Data    int  `json:"data"`
				}
				for _, status := range []int{0, 3, 5} {
					channelStatusRequest(t, rootID, UpdateChannelStatus, "/api/channel/"+strconv.Itoa(ids[0])+"/status", ChannelStatusRequest{Status: status}, &response)
					assert.False(t, response.Success)
					channelStatusRequest(t, rootID, BatchUpdateChannelStatus, "/api/channel/status/batch", ChannelStatusBatchRequest{Ids: ids, Status: status}, &response)
					assert.False(t, response.Success)
				}
				for _, status := range []int{1, 2, 4} {
					channelStatusRequest(t, rootID, BatchUpdateChannelStatus, "/api/channel/status/batch", ChannelStatusBatchRequest{Ids: ids, Status: status}, &response)
					assert.True(t, response.Success)
					assert.Equal(t, 2, response.Data)
					for _, channelID := range ids {
						stored, err := model.GetChannelById(channelID, true)
						require.NoError(t, err)
						assert.Equal(t, status, stored.Status)
					}
				}
				channelStatusRequest(t, rootID, EnableTagChannels, "/api/channel/enable_tag", ChannelTag{Tag: tag}, &response)
				assert.True(t, response.Success)
				for _, channelID := range ids {
					stored, err := model.GetChannelById(channelID, true)
					require.NoError(t, err)
					assert.Equal(t, 1, stored.Status)
				}
			})
			t.Run("list and search status filters", func(t *testing.T) {
				common.MemoryCacheEnabled = false
				tag := "filter-permanent-status"
				var bannedIDs, disabledIDs []int
				for _, status := range []int{1, 2, 3, 4} {
					channel := &model.Channel{Name: tag, Type: 1, Key: "key", Status: status, Group: "default", Tag: &tag}
					require.NoError(t, channel.Insert())
					t.Cleanup(func() { require.NoError(t, channel.Delete()) })
					if status == 4 {
						bannedIDs = append(bannedIDs, channel.Id)
					}
					if status != 1 {
						disabledIDs = append(disabledIDs, channel.Id)
					}
				}
				for _, handler := range []gin.HandlerFunc{GetAllChannels, SearchChannels} {
					for _, tagMode := range []bool{false, true} {
						for _, filter := range []string{"banned", "4", "disabled"} {
							recorder := httptest.NewRecorder()
							context, _ := gin.CreateTestContext(recorder)
							context.Request = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/channel/?status=%s&tag_mode=%t&keyword=%s", filter, tagMode, tag), nil)
							context.Set("role", common.RoleRootUser)
							handler(context)
							var response struct {
								Success bool `json:"success"`
								Data    struct {
									Items []*model.Channel `json:"items"`
								} `json:"data"`
							}
							require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
							assert.True(t, response.Success)
							expected := bannedIDs
							if filter == "disabled" {
								expected = disabledIDs
							}
							assert.ElementsMatch(t, expected, channelIDsFromChannels(response.Data.Items), "filter=%s tag_mode=%t", filter, tagMode)
						}
					}
				}
			})
		})
	}
}

func TestMultiKeyEnableRestoresOnlyExhaustedChannels(t *testing.T) {
	previousDB, previousLogDB := model.DB, model.LOG_DB
	previousType, previousLogType := common.MainDatabaseType(), common.LogDatabaseType()
	previousMaster, previousCache, previousRedis, previousSQLite := common.IsMasterNode, common.MemoryCacheEnabled, common.RedisEnabled, common.SQLitePath
	t.Cleanup(func() {
		model.DB, model.LOG_DB = previousDB, previousLogDB
		common.SetDatabaseTypes(previousType, previousLogType)
		common.IsMasterNode, common.MemoryCacheEnabled, common.RedisEnabled, common.SQLitePath = previousMaster, previousCache, previousRedis, previousSQLite
	})
	t.Setenv("SQL_DSN", os.Getenv("TEST_CHANNEL_SQL_DSN"))
	t.Setenv("LOG_SQL_DSN", "")
	common.IsMasterNode, common.MemoryCacheEnabled, common.RedisEnabled = false, false, false
	common.SQLitePath = filepath.Join(t.TempDir(), "channel.db")
	require.NoError(t, model.InitDB())
	database := model.DB
	sqlDB, err := database.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	model.LOG_DB = database
	common.SetLogDatabaseType(common.MainDatabaseType())
	require.NoError(t, database.AutoMigrate(&model.Channel{}, &model.Ability{}, &model.User{}, &model.Log{}, &model.AuditLog{}))
	root := &model.User{Username: "multi-key-review-root", Role: common.RoleRootUser, Status: common.UserStatusEnabled}
	require.NoError(t, database.Create(root).Error)
	t.Cleanup(func() { require.NoError(t, database.Unscoped().Delete(root).Error) })
	versionQuery := "SELECT VERSION()"
	if common.UsingMainDatabase(common.DatabaseTypeSQLite) {
		versionQuery = "SELECT sqlite_version()"
	}
	var version string
	require.NoError(t, database.Raw(versionQuery).Scan(&version).Error)
	t.Logf("database=%s version=%s", common.MainDatabaseType(), version)

	for _, cacheEnabled := range []bool{false, true} {
		for _, action := range []string{"enable_key", "enable_all_keys"} {
			for _, tc := range []struct {
				name           string
				initialStatus  int
				manualOverride string
				wantStatus     int
			}{
				{name: "key exhaustion restores", initialStatus: common.ChannelStatusEnabled, wantStatus: common.ChannelStatusEnabled},
				{name: "manual disable is preserved", initialStatus: common.ChannelStatusManuallyDisabled, wantStatus: common.ChannelStatusManuallyDisabled},
				{name: "manual disable after exhaustion is preserved", initialStatus: common.ChannelStatusEnabled, manualOverride: "status", wantStatus: common.ChannelStatusManuallyDisabled},
				{name: "tag disable after exhaustion is preserved", initialStatus: common.ChannelStatusEnabled, manualOverride: "tag", wantStatus: common.ChannelStatusManuallyDisabled},
				{name: "permanent unavailability is preserved", initialStatus: 4, wantStatus: 4},
			} {
				t.Run(fmt.Sprintf("cache=%t/%s/%s", cacheEnabled, action, tc.name), func(t *testing.T) {
					common.MemoryCacheEnabled = cacheEnabled
					tag := t.Name()
					channel := &model.Channel{Name: t.Name(), Type: 1, Key: "key-one\nkey-two", Status: tc.initialStatus, Models: "test-model", Group: "default", Tag: &tag,
						ChannelInfo: model.ChannelInfo{IsMultiKey: true, MultiKeySize: 2, MultiKeyStatusList: map[int]int{1: common.ChannelStatusManuallyDisabled}},
					}
					require.NoError(t, channel.Insert())
					t.Cleanup(func() {
						require.NoError(t, channel.Delete())
						model.InitChannelCache()
					})
					for _, operation := range []string{"disable_key", action} {
						if operation == action {
							if tc.manualOverride == "status" {
								model.UpdateChannelStatus(channel.Id, "", common.ChannelStatusManuallyDisabled, "manual operation")
							} else if tc.manualOverride == "tag" {
								require.NoError(t, model.DisableChannelByTag(tag))
							}
						}
						payload, err := common.Marshal(MultiKeyManageRequest{ChannelId: channel.Id, Action: operation, KeyIndex: common.GetPointer(0)})
						require.NoError(t, err)
						recorder := httptest.NewRecorder()
						c, _ := gin.CreateTestContext(recorder)
						c.Set("id", root.Id)
						c.Set("role", common.RoleRootUser)
						c.Request = httptest.NewRequest(http.MethodPost, "/api/channel/multi_key", bytes.NewReader(payload))
						c.Request.Header.Set("Content-Type", "application/json")
						ManageMultiKeys(c)
						var result struct {
							Success bool `json:"success"`
						}
						require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &result))
						require.True(t, result.Success, recorder.Body.String())
					}
					loaded, err := model.GetChannelById(channel.Id, true)
					require.NoError(t, err)
					assert.Equal(t, tc.wantStatus, loaded.Status)
					assert.NotContains(t, loaded.ChannelInfo.MultiKeyStatusList, 0)
					assert.NotContains(t, loaded.ChannelInfo.MultiKeyDisabledReason, 0)
					assert.NotContains(t, loaded.ChannelInfo.MultiKeyDisabledTime, 0)
					var ability model.Ability
					require.NoError(t, database.Where("channel_id = ?", channel.Id).First(&ability).Error)
					assert.Equal(t, tc.wantStatus == common.ChannelStatusEnabled, ability.Enabled)
				})
			}
		}
	}
}
