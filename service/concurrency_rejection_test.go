package service

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	kittypes "github.com/QuantumNous/new-api/relaykit/types"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestChannelConcurrencyLimitErrorPreservesCandidates(t *testing.T) {
	err := NewChannelConcurrencyLimitError([]ChannelConcurrencyCandidate{
		{ChannelID: 22, Limit: 50},
		{ChannelID: 11, Limit: 100},
		{ChannelID: 22, Limit: 50},
	})

	require.ErrorIs(t, err, ErrChannelConcurrencyLimit)
	assert.Equal(t, []ChannelConcurrencyCandidate{
		{ChannelID: 22, Limit: 50},
		{ChannelID: 11, Limit: 100},
	}, ChannelConcurrencyCandidates(err))
	assert.Empty(t, ChannelConcurrencyCandidates(errors.New("other")))
}

func TestRecordConcurrencyLimitRejectionWritesOneScopedErrorLog(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previousDB, previousLogDB := model.DB, model.LOG_DB
	previousMainDatabaseType := common.MainDatabaseType()
	previousLogDatabaseType := common.LogDatabaseType()
	previousErrorLogEnabled := constant.ErrorLogEnabled

	database, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "concurrency-log.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, database.AutoMigrate(&model.Log{}))
	model.DB, model.LOG_DB = database, database
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	constant.ErrorLogEnabled = true
	t.Cleanup(func() {
		model.DB, model.LOG_DB = previousDB, previousLogDB
		common.SetDatabaseTypes(previousMainDatabaseType, previousLogDatabaseType)
		constant.ErrorLogEnabled = previousErrorLogEnabled
	})

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Set("id", 17)
	ctx.Set("username", "capacity-user")
	ctx.Set("token_name", "capacity-token")
	ctx.Set("token_id", 23)
	common.SetContextKey(ctx, constant.ContextKeyUsingGroup, "default")
	ctx.Set(common.RequestIdKey, "request-capacity-1")

	rejection := ConcurrencyLimitRejection{
		Scope:     ConcurrencyScopeChannel,
		ModelName: "gpt-capacity",
		Candidates: []ChannelConcurrencyCandidate{
			{ChannelID: 101, Limit: 100},
			{ChannelID: 202, Limit: 50},
		},
	}
	RecordConcurrencyLimitRejection(ctx, rejection)
	RecordConcurrencyLimitRejection(ctx, rejection)
	RecordRelayErrorLog(ctx, 101, kittypes.NewErrorWithStatusCode(errors.New("duplicate"), kittypes.ErrorCodeGetChannelFailed, http.StatusTooManyRequests), nil)

	var stored []model.Log
	require.NoError(t, database.Find(&stored).Error)
	require.Len(t, stored, 1)
	assert.Equal(t, model.LogTypeError, stored[0].Type)
	assert.Equal(t, 17, stored[0].UserId)
	assert.Equal(t, "gpt-capacity", stored[0].ModelName)
	assert.Equal(t, "default", stored[0].Group)
	assert.Equal(t, "request-capacity-1", stored[0].RequestId)
	assert.Zero(t, stored[0].ChannelId)
	assert.Zero(t, stored[0].Quota)
	assert.Zero(t, stored[0].PromptTokens)
	assert.Zero(t, stored[0].CompletionTokens)

	adminOther, err := common.StrToMap(stored[0].Other)
	require.NoError(t, err)
	assert.Equal(t, float64(http.StatusTooManyRequests), adminOther["status_code"])
	assert.Equal(t, "channel_concurrency_limit", adminOther["error_code"])
	assert.Equal(t, "local_gateway", adminOther["error_source"])
	assert.Equal(t, string(ConcurrencyScopeChannel), adminOther["concurrency_scope"])
	adminInfo, ok := adminOther["admin_info"].(map[string]any)
	require.True(t, ok)
	concurrency, ok := adminInfo["concurrency"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, false, concurrency["upstream_attempted"])
	assert.Len(t, concurrency["candidates"], 2)

	logs, total, err := model.GetUserLogs(17, model.LogFilter{Type: model.LogTypeError}, 0, 10)
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, logs, 1)
	userOther, err := common.StrToMap(logs[0].Other)
	require.NoError(t, err)
	assert.NotContains(t, userOther, "admin_info")
	assert.Equal(t, "channel_concurrency_limit", userOther["error_code"])
}

func TestRecordConcurrencyLimitRejectionHandlesMissingRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previousErrorLogEnabled := constant.ErrorLogEnabled
	constant.ErrorLogEnabled = false
	t.Cleanup(func() {
		constant.ErrorLogEnabled = previousErrorLogEnabled
	})

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	require.NotPanics(t, func() {
		RecordConcurrencyLimitRejection(ctx, ConcurrencyLimitRejection{
			Scope:     ConcurrencyScopeUser,
			ModelName: "gpt-capacity",
			PreRelay:  true,
		})
	})
}
