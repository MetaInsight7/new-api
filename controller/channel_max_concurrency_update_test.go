package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupChannelMaxConcurrencyUpdateTest(t *testing.T) *gorm.DB {
	t.Helper()
	previousDB := model.DB
	previousLogDB := model.LOG_DB
	previousDatabaseType := common.MainDatabaseType()
	previousLogDatabaseType := common.LogDatabaseType()
	previousMemoryCacheEnabled := common.MemoryCacheEnabled
	previousRedisEnabled := common.RedisEnabled

	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDatabase, err := database.DB()
	require.NoError(t, err)
	sqlDatabase.SetMaxOpenConns(1)
	require.NoError(t, database.AutoMigrate(
		&model.Channel{},
		&model.Ability{},
		&model.User{},
		&model.AuditLog{},
	))
	model.DB = database
	model.LOG_DB = database
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	common.MemoryCacheEnabled = false
	common.RedisEnabled = false
	require.NoError(t, database.Create(&model.User{
		Id:       1,
		Username: "channel-concurrency-editor",
		Password: "unused-password",
		Role:     common.RoleRootUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
		AffCode:  "channel-concurrency-editor",
	}).Error)

	t.Cleanup(func() {
		model.DB = previousDB
		model.LOG_DB = previousLogDB
		common.SetDatabaseTypes(previousDatabaseType, previousLogDatabaseType)
		common.MemoryCacheEnabled = previousMemoryCacheEnabled
		common.RedisEnabled = previousRedisEnabled
		require.NoError(t, sqlDatabase.Close())
	})
	return database
}

func updateChannelMaxConcurrency(t *testing.T, body string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Set("id", 1)
	context.Set("role", common.RoleRootUser)
	context.Request = httptest.NewRequest(http.MethodPut, "/api/channel/", strings.NewReader(body))
	context.Request.Header.Set("Content-Type", "application/json")
	UpdateChannel(context)
	return recorder
}

func insertBoundedChannel(t *testing.T, database *gorm.DB, maxConcurrency int) model.Channel {
	t.Helper()
	channel := model.Channel{
		Type:           constant.ChannelTypeOpenAI,
		Key:            "sk-test",
		Status:         common.ChannelStatusEnabled,
		Name:           "bounded",
		Models:         "gpt-test",
		Group:          "default",
		MaxConcurrency: maxConcurrency,
	}
	require.NoError(t, database.Create(&channel).Error)
	return channel
}

func requireChannelUpdateSuccess(t *testing.T, response *httptest.ResponseRecorder) {
	t.Helper()
	require.Equal(t, http.StatusOK, response.Code)
	var result struct {
		Success bool `json:"success"`
	}
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &result))
	require.True(t, result.Success, response.Body.String())
}

func TestUpdateChannelPersistsExplicitZeroMaxConcurrency(t *testing.T) {
	database := setupChannelMaxConcurrencyUpdateTest(t)
	channel := insertBoundedChannel(t, database, 5)

	response := updateChannelMaxConcurrency(t, fmt.Sprintf(`{"id":%d,"max_concurrency":0}`, channel.Id))
	requireChannelUpdateSuccess(t, response)

	var persisted model.Channel
	require.NoError(t, database.First(&persisted, "id = ?", channel.Id).Error)
	assert.Zero(t, persisted.MaxConcurrency)
}

func TestUpdateChannelMaxConcurrencyPresenceSemantics(t *testing.T) {
	tests := []struct {
		name               string
		request            func(channelID int) string
		wantMaxConcurrency int
	}{
		{
			name: "positive value replaces the limit",
			request: func(channelID int) string {
				return fmt.Sprintf(`{"id":%d,"max_concurrency":8}`, channelID)
			},
			wantMaxConcurrency: 8,
		},
		{
			name: "missing value preserves the limit",
			request: func(channelID int) string {
				return fmt.Sprintf(`{"id":%d,"name":"renamed"}`, channelID)
			},
			wantMaxConcurrency: 5,
		},
		{
			name: "null value preserves the limit",
			request: func(channelID int) string {
				return fmt.Sprintf(`{"id":%d,"name":"renamed","max_concurrency":null}`, channelID)
			},
			wantMaxConcurrency: 5,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			database := setupChannelMaxConcurrencyUpdateTest(t)
			channel := insertBoundedChannel(t, database, 5)

			response := updateChannelMaxConcurrency(t, test.request(channel.Id))
			requireChannelUpdateSuccess(t, response)

			var persisted model.Channel
			require.NoError(t, database.First(&persisted, "id = ?", channel.Id).Error)
			assert.Equal(t, test.wantMaxConcurrency, persisted.MaxConcurrency)
		})
	}
}

func TestUpdateChannelRejectsNegativeMaxConcurrencyWithoutChangingStoredValue(t *testing.T) {
	database := setupChannelMaxConcurrencyUpdateTest(t)
	channel := insertBoundedChannel(t, database, 5)

	response := updateChannelMaxConcurrency(t, fmt.Sprintf(`{"id":%d,"max_concurrency":-1}`, channel.Id))
	require.Equal(t, http.StatusOK, response.Code)
	var result struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &result))
	assert.False(t, result.Success)
	assert.Contains(t, result.Message, "cannot be negative")

	var persisted model.Channel
	require.NoError(t, database.First(&persisted, "id = ?", channel.Id).Error)
	assert.Equal(t, 5, persisted.MaxConcurrency)
}

func TestAddChannelRejectsNegativeMaxConcurrency(t *testing.T) {
	database := setupChannelMaxConcurrencyUpdateTest(t)
	response := postAddChannel(
		t,
		1,
		common.RoleRootUser,
		`{"mode":"single","channel":{"type":1,"name":"invalid","key":"sk-test","models":"gpt-test","group":"default","max_concurrency":-1}}`,
	)
	require.Equal(t, http.StatusOK, response.Code)
	var result struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &result))
	assert.False(t, result.Success)
	assert.Contains(t, result.Message, "cannot be negative")

	var count int64
	require.NoError(t, database.Model(&model.Channel{}).Count(&count).Error)
	assert.Zero(t, count)
}
