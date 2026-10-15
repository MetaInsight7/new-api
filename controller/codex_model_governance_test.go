package controller

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupCodexGovernanceControllerTest(t *testing.T) *gorm.DB {
	t.Helper()
	previousDB, previousLogDB := model.DB, model.LOG_DB
	previousCache, previousRedis := common.MemoryCacheEnabled, common.RedisEnabled
	previousMainType, previousLogType := common.MainDatabaseType(), common.LogDatabaseType()
	previousStreamingTimeout := constant.StreamingTimeout
	setting := operation_setting.GetCodexModelGovernanceSetting()
	previousSetting := *setting
	database := setupModelListControllerTestDB(t)
	sqlDB, err := database.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	common.MemoryCacheEnabled = false
	constant.StreamingTimeout = 60
	require.NoError(t, database.AutoMigrate(&model.CodexModelGovernanceRecord{}, &model.CodexModelGovernanceProbeState{}, &model.CodexModelGovernanceAlertCooldownRecord{}, &model.SystemTask{}, &model.SystemTaskLock{}))
	require.NoError(t, database.Create(&model.User{Id: 9001, Username: "governance-root", Role: common.RoleRootUser, Status: common.UserStatusEnabled, Group: "default", Quota: 1000000, Setting: `{"accept_unset_model_ratio_model":true}`}).Error)
	setting.Enabled, setting.ProbeEnabled = false, false
	setting.UnsupportedMessagePatterns = []string{operation_setting.DefaultCodexUnsupportedPattern}
	t.Cleanup(func() {
		model.DB, model.LOG_DB = previousDB, previousLogDB
		common.MemoryCacheEnabled, common.RedisEnabled = previousCache, previousRedis
		common.SetDatabaseTypes(previousMainType, previousLogType)
		constant.StreamingTimeout = previousStreamingTimeout
		*setting = previousSetting
	})
	return database
}

func TestCodexGovernanceProbesUseBaseOptionsAndConfiguredModel(t *testing.T) {
	database := setupCodexGovernanceControllerTest(t)
	var requests atomic.Int32
	mode := "unsupported"
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests.Add(1)
		assert.Equal(t, "/backend-api/codex/responses", request.URL.Path)
		body, err := io.ReadAll(request.Body)
		require.NoError(t, err)
		assert.Contains(t, string(body), `"model":"gpt-5.4"`)
		assert.Contains(t, string(body), "ping")
		assert.Contains(t, string(body), `"stream":true`)
		assert.Equal(t, "Bearer fake-probe-token", request.Header.Get("Authorization"))
		writer.Header().Set("Content-Type", "application/json")
		if mode == "unsupported" {
			writer.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(writer, `{"error":{"message":"The 'gpt-5.4' model is not supported when using Codex with a ChatGPT account.","type":"invalid_request_error"}}`)
			return
		}
		if mode == "temporary" {
			writer.WriteHeader(http.StatusTooManyRequests)
			_, _ = io.WriteString(writer, `{"error":{"message":"rate limit exceeded","type":"rate_limit_error"}}`)
			return
		}
		writer.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(writer, "data: {\"type\":\"response.created\",\"response\":{\"id\":\"probe\",\"model\":\"gpt-5.4\",\"created_at\":1710000000}}\n\ndata: {\"type\":\"response.output_text.delta\",\"delta\":\"pong\"}\n\ndata: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\",\"usage\":{\"input_tokens\":2,\"output_tokens\":1,\"total_tokens\":3}}}\n\n")
	}))
	t.Cleanup(server.Close)
	if service.GetHttpClient() == nil {
		service.InitHttpClient()
	}
	channel := model.Channel{Id: 31, Type: constant.ChannelTypeCodex, Status: 1, Name: "probe", BaseURL: common.GetPointer(server.URL), Key: `{"access_token":"fake-probe-token","account_id":"fake-account"}`, Models: "gpt-5.4-mini,gpt-5.4-mini", ModelMapping: common.GetPointer(`{"gpt-5.4-mini":"gpt-5.4"}`), Group: "default"}
	require.NoError(t, database.Create(&channel).Error)
	require.NoError(t, channel.AddAbilities(nil))
	require.NoError(t, database.Create(&model.Channel{Id: 32, Type: constant.ChannelTypeCodex, Status: 4, Models: "gpt-5.4-mini", Group: "default"}).Error)
	processed, err := runCodexGovernanceProbes(context.Background(), nil)
	require.NoError(t, err)
	assert.Zero(t, processed)
	assert.Zero(t, requests.Load())
	setting := operation_setting.GetCodexModelGovernanceSetting()
	setting.Enabled, setting.ProbeEnabled = true, true
	for _, responseMode := range []string{"unsupported", "temporary", "unsupported"} {
		mode = responseMode
		processed, err = runCodexGovernanceProbes(context.Background(), nil)
		require.NoError(t, err)
		assert.Equal(t, 1, processed)
		_, err = model.GetCodexModelGovernanceRecordByModelName("gpt-5.4-mini")
		require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	}
	processed, err = runCodexGovernanceProbes(context.Background(), nil)
	require.NoError(t, err)
	assert.Equal(t, 1, processed)
	record, err := model.GetCodexModelGovernanceRecordByModelName("gpt-5.4-mini")
	require.NoError(t, err)
	assert.Equal(t, []int{31}, model.CodexModelGovernanceDisabledChannelIDs(*record))
	assert.Equal(t, []int{31, 32}, model.DecodeCodexModelGovernanceChannelIDs(record.AffectedChannelIDs))
	_, err = model.GetCodexModelGovernanceRecordByModelName("gpt-5.4")
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	mode = "healthy"
	_, err = runCodexGovernanceProbes(context.Background(), nil)
	require.NoError(t, err)
	record, err = model.GetCodexModelGovernanceRecord(record.ID)
	require.NoError(t, err)
	assert.Equal(t, model.CodexModelGovernanceStatusActive, record.Status)
	var ability model.Ability
	require.NoError(t, database.First(&ability, "channel_id = ?", 31).Error)
	assert.True(t, ability.Enabled)
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	before := requests.Load()
	_, err = runCodexGovernanceProbes(cancelled, nil)
	assert.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, before, requests.Load())
}

func TestCodexGovernanceReviewsBindActionAndRecordReviewer(t *testing.T) {
	setupCodexGovernanceControllerTest(t)
	record, err := model.UpsertCodexModelGovernancePending(model.CodexModelGovernancePendingInput{ModelName: "review-model", Source: model.CodexModelGovernanceSourceOfficialNotice})
	require.NoError(t, err)
	for _, testCase := range []struct {
		role        int
		bodyAction  string
		wantSuccess bool
		wantCode    int
	}{
		{common.RoleCommonUser, "ignore", false, http.StatusForbidden},
		{common.RoleRootUser, "confirm_remove", false, http.StatusOK},
		{common.RoleRootUser, "ignore", true, http.StatusOK},
	} {
		response := httptest.NewRecorder()
		requestContext, _ := gin.CreateTestContext(response)
		requestContext.Set("id", 9001)
		requestContext.Set("role", testCase.role)
		requestContext.Params = gin.Params{{Key: "id", Value: strconv.Itoa(record.ID)}}
		requestContext.Request = httptest.NewRequest(http.MethodPost, "/review/ignore", strings.NewReader(`{"action":"`+testCase.bodyAction+`","note":"reviewed"}`))
		ReviewCodexModelGovernanceAction(service.CodexModelGovernanceReviewActionIgnore)(requestContext)
		assert.Equal(t, testCase.wantCode, response.Code)
		var result struct {
			Success bool `json:"success"`
		}
		require.NoError(t, common.Unmarshal(response.Body.Bytes(), &result))
		assert.Equal(t, testCase.wantSuccess, result.Success)
	}
	record, err = model.GetCodexModelGovernanceRecord(record.ID)
	require.NoError(t, err)
	assert.Equal(t, model.CodexModelGovernanceStatusIgnored, record.Status)
	assert.Equal(t, 9001, record.ReviewedBy)
	assert.Equal(t, "reviewed", record.ReviewNote)
}

func TestCodexGovernanceHandlersUseExistingTaskLease(t *testing.T) {
	setupCodexGovernanceControllerTest(t)
	StartCodexModelGovernanceTask()
	for _, handler := range []codexGovernanceTaskHandler{{}, {notice: true}} {
		assert.False(t, handler.Enabled())
		assert.Equal(t, 24*time.Hour, handler.Interval())
		task, created, err := service.EnqueueSystemTask(handler.Type(), nil)
		require.NoError(t, err)
		assert.True(t, created)
		duplicate, created, err := service.EnqueueSystemTask(handler.Type(), nil)
		require.NoError(t, err)
		assert.False(t, created)
		assert.Equal(t, task.TaskID, duplicate.TaskID)
		claimed, allowed, err := model.ClaimSystemTask(task.ID, handler.Type(), "governance-runner", time.Now().Add(time.Minute).Unix())
		require.NoError(t, err)
		require.True(t, allowed)
		_, allowed, err = model.ClaimSystemTask(task.ID, handler.Type(), "other-runner", time.Now().Add(time.Minute).Unix())
		require.NoError(t, err)
		assert.False(t, allowed)
		handler.Run(context.Background(), claimed, "governance-runner")
		finished, err := model.GetSystemTaskByTaskID(task.TaskID)
		require.NoError(t, err)
		assert.Equal(t, model.SystemTaskStatusSucceeded, finished.Status)
		assert.ErrorIs(t, model.UpdateSystemTaskState(task.TaskID, "other-runner", nil), model.ErrSystemTaskLockLost)
	}
}
