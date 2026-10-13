package controller

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type channelLoadMonitorResponse struct {
	Success bool `json:"success"`
	Data    struct {
		Items      []channelLoadMonitorItem  `json:"items"`
		Total      int                       `json:"total"`
		Page       int                       `json:"page"`
		PageSize   int                       `json:"page_size"`
		TypeCounts map[int]int64             `json:"type_counts"`
		Groups     []string                  `json:"groups"`
		Summary    channelLoadMonitorSummary `json:"summary"`
		SnapshotAt int64                     `json:"snapshot_at"`
	} `json:"data"`
}

type channelLoadMonitorFixture struct {
	highPriority *model.Channel
	overLimit    *model.Channel
	running      *model.Channel
	unlimited    *model.Channel
}

func setupChannelLoadMonitorTest(t *testing.T) (*redis.Client, channelLoadMonitorFixture) {
	t.Helper()
	previousDB := model.DB
	previousLogDB := model.LOG_DB
	previousRDB := common.RDB
	previousRedisEnabled := common.RedisEnabled
	previousDatabaseType, previousLogDatabaseType := common.MainDatabaseType(), common.LogDatabaseType()
	previousSetting := operation_setting.GetChannelConcurrencySetting()

	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, database.AutoMigrate(&model.Channel{}, &model.Log{}))
	miniRedis := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: miniRedis.Addr()})

	model.DB = database
	model.LOG_DB = database
	common.RDB = redisClient
	common.RedisEnabled = true
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	setting := previousSetting
	setting.LoadCacheEnabled = false
	setting.CooldownEnabled = true
	operation_setting.SetChannelConcurrencySettingForTest(setting)

	priority10 := int64(10)
	priority7 := int64(7)
	priority5 := int64(5)
	priority1 := int64(1)
	fixture := channelLoadMonitorFixture{
		highPriority: &model.Channel{
			Id: 8101, Type: 1, Key: "secret-high", Status: common.ChannelStatusEnabled,
			Name: "high priority", Group: "alpha", MaxConcurrency: 2, Priority: &priority10,
		},
		overLimit: &model.Channel{
			Id: 8102, Type: 1, Key: "secret-over", Status: common.ChannelStatusEnabled,
			Name: "over limit", Group: "alpha", MaxConcurrency: 1, Priority: &priority7,
		},
		running: &model.Channel{
			Id: 8103, Type: 2, Key: "secret-running", Status: common.ChannelStatusEnabled,
			Name: "running", Group: "beta", MaxConcurrency: 4, Priority: &priority5,
		},
		unlimited: &model.Channel{
			Id: 8104, Type: 1, Key: "secret-unlimited", Status: common.ChannelStatusManuallyDisabled,
			Name: "unlimited", Group: "unlimited-only", MaxConcurrency: 0, Priority: &priority1,
		},
	}
	for _, channel := range []*model.Channel{fixture.highPriority, fixture.overLimit, fixture.running, fixture.unlimited} {
		require.NoError(t, database.Create(channel).Error)
	}
	recentTimestamp := time.Now().Unix()
	for channelID, count := range map[int]int{
		fixture.highPriority.Id: 2,
		fixture.overLimit.Id:    1,
		fixture.running.Id:      3,
		fixture.unlimited.Id:    4,
	} {
		for range count {
			require.NoError(t, database.Create(&model.Log{
				CreatedAt: recentTimestamp,
				Type:      model.LogTypeConsume,
				ChannelId: channelID,
			}).Error)
		}
	}
	require.NoError(t, database.Create(&model.Log{
		CreatedAt: recentTimestamp - 120,
		Type:      model.LogTypeConsume,
		ChannelId: fixture.highPriority.Id,
	}).Error)
	require.NoError(t, database.Create(&model.Log{
		CreatedAt: recentTimestamp,
		Type:      model.LogTypeError,
		ChannelId: fixture.unlimited.Id,
	}).Error)
	require.NoError(t, database.Create(&model.Log{
		CreatedAt: recentTimestamp,
		Type:      model.LogTypeConsume,
		ChannelId: 9999,
	}).Error)

	expiresAt := float64(time.Now().Add(time.Minute).UnixMilli())
	for tokenIndex := range 2 {
		require.NoError(t, redisClient.ZAdd(context.Background(),
			fmt.Sprintf("new-api:channel_concurrency:%d", fixture.highPriority.Id),
			&redis.Z{Score: expiresAt, Member: fmt.Sprintf("high-%d", tokenIndex)},
		).Err())
		require.NoError(t, redisClient.ZAdd(context.Background(),
			fmt.Sprintf("new-api:channel_concurrency:%d", fixture.overLimit.Id),
			&redis.Z{Score: expiresAt, Member: fmt.Sprintf("over-%d", tokenIndex)},
		).Err())
	}
	require.NoError(t, redisClient.ZAdd(context.Background(),
		fmt.Sprintf("new-api:channel_concurrency:%d", fixture.running.Id),
		&redis.Z{Score: expiresAt, Member: "running-0"},
	).Err())
	require.NoError(t, redisClient.Set(context.Background(),
		fmt.Sprintf("new-api:channel_concurrency_wait:%d", fixture.highPriority.Id), 2, time.Minute,
	).Err())
	require.NoError(t, redisClient.Set(context.Background(),
		fmt.Sprintf("new-api:channel_concurrency_cooldown:%d", fixture.highPriority.Id), "test", time.Minute,
	).Err())

	t.Cleanup(func() {
		model.DB = previousDB
		model.LOG_DB = previousLogDB
		common.RDB = previousRDB
		common.RedisEnabled = previousRedisEnabled
		common.SetDatabaseTypes(previousDatabaseType, previousLogDatabaseType)
		operation_setting.SetChannelConcurrencySettingForTest(previousSetting)
		require.NoError(t, redisClient.Close())
	})
	return redisClient, fixture
}

func requestChannelLoadMonitor(t *testing.T, query url.Values) channelLoadMonitorResponse {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ginContext, _ := gin.CreateTestContext(recorder)
	ginContext.Request = httptest.NewRequest(http.MethodGet, "/api/channel/concurrency/monitor?"+query.Encode(), nil)
	GetChannelLoadMonitor(ginContext)

	var body channelLoadMonitorResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &body), recorder.Body.String())
	require.True(t, body.Success, recorder.Body.String())
	return body
}

func TestChannelLoadMonitorReturnsPageIndependentSummaryAndRedactsKeys(t *testing.T) {
	_, fixture := setupChannelLoadMonitorTest(t)

	firstPage := requestChannelLoadMonitor(t, url.Values{"p": {"1"}, "page_size": {"2"}})
	require.Equal(t, 3, firstPage.Data.Total)
	require.Equal(t, 1, firstPage.Data.Page)
	require.Equal(t, 2, firstPage.Data.PageSize)
	require.Len(t, firstPage.Data.Items, 2)
	assert.Equal(t, fixture.highPriority.Id, firstPage.Data.Items[0].Channel.Id)
	assert.Equal(t, fixture.overLimit.Id, firstPage.Data.Items[1].Channel.Id)
	assert.Empty(t, firstPage.Data.Items[0].Channel.Key)
	assert.Equal(t, channelLoadMonitorSummary{
		LimitedRPM: 6, Active: 5, Capacity: 7, Waiting: 2, CoolingDown: 1,
	}, firstPage.Data.Summary)
	assert.Equal(t, map[int]int64{1: 2, 2: 1}, firstPage.Data.TypeCounts)
	assert.Equal(t, []string{"alpha", "beta"}, firstPage.Data.Groups)
	assert.Positive(t, firstPage.Data.SnapshotAt)

	secondPage := requestChannelLoadMonitor(t, url.Values{"p": {"2"}, "page_size": {"2"}})
	require.Len(t, secondPage.Data.Items, 1)
	assert.Equal(t, fixture.running.Id, secondPage.Data.Items[0].Channel.Id)
	assert.Equal(t, firstPage.Data.Summary, secondPage.Data.Summary)
	assert.NotNil(t, secondPage.Data.Items[0].Concurrency)
}

func TestChannelLoadMonitorTypeCountsMatchExistingChannelsBeforeTypeFilter(t *testing.T) {
	_, _ = setupChannelLoadMonitorTest(t)

	allTypes := requestChannelLoadMonitor(t, url.Values{"type": {"2"}})
	assert.Equal(t, map[int]int64{1: 2, 2: 1}, allTypes.Data.TypeCounts)

	betaTypes := requestChannelLoadMonitor(t, url.Values{"group": {"beta"}})
	assert.Equal(t, map[int]int64{2: 1}, betaTypes.Data.TypeCounts)

	searchedTypes := requestChannelLoadMonitor(t, url.Values{"keyword": {"running"}})
	assert.Equal(t, map[int]int64{2: 1}, searchedTypes.Data.TypeCounts)
}

func TestChannelLoadMonitorAppliesStaticAndLoadFilters(t *testing.T) {
	_, fixture := setupChannelLoadMonitorTest(t)

	tests := []struct {
		name           string
		query          url.Values
		wantIDs        []int
		wantTotal      int
		wantLimitedRPM int64
	}{
		{name: "name", query: url.Values{"keyword": {"running"}}, wantIDs: []int{fixture.running.Id}, wantTotal: 1, wantLimitedRPM: 3},
		{name: "exact id", query: url.Values{"keyword": {strconv.Itoa(fixture.overLimit.Id)}}, wantIDs: []int{fixture.overLimit.Id}, wantTotal: 1, wantLimitedRPM: 1},
		{name: "group", query: url.Values{"group": {"beta"}}, wantIDs: []int{fixture.running.Id}, wantTotal: 1, wantLimitedRPM: 3},
		{name: "enabled", query: url.Values{"status": {"enabled"}}, wantIDs: []int{fixture.highPriority.Id, fixture.overLimit.Id, fixture.running.Id}, wantTotal: 3, wantLimitedRPM: 6},
		{name: "type", query: url.Values{"type": {"2"}}, wantIDs: []int{fixture.running.Id}, wantTotal: 1, wantLimitedRPM: 3},
		{name: "full", query: url.Values{"load_state": {"full"}}, wantIDs: []int{fixture.highPriority.Id, fixture.overLimit.Id}, wantTotal: 2, wantLimitedRPM: 3},
		{name: "queued", query: url.Values{"load_state": {"queued"}}, wantIDs: []int{fixture.highPriority.Id}, wantTotal: 1, wantLimitedRPM: 2},
		{name: "cooling", query: url.Values{"load_state": {"cooling"}}, wantIDs: []int{fixture.highPriority.Id}, wantTotal: 1, wantLimitedRPM: 2},
		{name: "legacy unlimited filter returns no channels", query: url.Values{"load_state": {"unlimited"}}, wantIDs: []int{}, wantTotal: 0},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			body := requestChannelLoadMonitor(t, testCase.query)
			require.Equal(t, testCase.wantTotal, body.Data.Total)
			assert.Equal(t, testCase.wantLimitedRPM, body.Data.Summary.LimitedRPM)
			actualIDs := make([]int, 0, len(body.Data.Items))
			for _, item := range body.Data.Items {
				actualIDs = append(actualIDs, item.Channel.Id)
			}
			assert.Equal(t, testCase.wantIDs, actualIDs)
		})
	}
}

func TestChannelLoadMonitorSortsByLiveLoadAndWaiting(t *testing.T) {
	_, fixture := setupChannelLoadMonitorTest(t)

	byLoad := requestChannelLoadMonitor(t, url.Values{"sort_by": {"load"}, "sort_order": {"desc"}})
	require.Len(t, byLoad.Data.Items, 3)
	assert.Equal(t, fixture.overLimit.Id, byLoad.Data.Items[0].Channel.Id)
	assert.Equal(t, fixture.highPriority.Id, byLoad.Data.Items[1].Channel.Id)

	byWaiting := requestChannelLoadMonitor(t, url.Values{"sort_by": {"waiting"}, "sort_order": {"desc"}})
	require.Len(t, byWaiting.Data.Items, 3)
	assert.Equal(t, fixture.highPriority.Id, byWaiting.Data.Items[0].Channel.Id)
}
