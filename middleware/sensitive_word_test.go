package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupSensitiveWordDistributorTest(t *testing.T) *gorm.DB {
	t.Helper()
	previousDB, previousLogDB := model.DB, model.LOG_DB
	previousMainType, previousLogType := common.MainDatabaseType(), common.LogDatabaseType()
	previousRedis, previousRedisEnabled := common.RDB, common.RedisEnabled
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := database.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, database.AutoMigrate(
		&model.User{}, &model.Token{}, &model.Log{}, &model.Option{},
		&model.SensitiveWordRule{}, &model.SensitiveWordRuleWord{}, &model.SensitiveWordRuleGroup{},
		&model.SensitiveWordPolicy{}, &model.SensitiveWordAuditEvent{},
	))
	model.DB, model.LOG_DB = database, database
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	common.RDB, common.RedisEnabled = nil, false
	t.Cleanup(func() {
		model.DB, model.LOG_DB = previousDB, previousLogDB
		common.SetDatabaseTypes(previousMainType, previousLogType)
		common.RDB, common.RedisEnabled = previousRedis, previousRedisEnabled
		require.NoError(t, sqlDB.Close())
	})
	return database
}

func TestDistributeChecksSensitiveWordsBeforeMissingChannel(t *testing.T) {
	database := setupSensitiveWordDistributorTest(t)
	require.NoError(t, i18n.Init())
	require.NoError(t, model.MigrateSensitiveWordData())
	policy := model.GetSensitiveWordPolicy()
	policy.Enabled = true
	policy.CheckPrompt = true
	require.NoError(t, model.SaveSensitiveWordPolicy(policy, 1))

	markers := []string{
		"distributor-chat-sensitive-marker",
		"distributor-responses-sensitive-marker",
		"distributor-tool-result-sensitive-marker",
		"distributor-claude-sensitive-marker",
		"distributor-gemini-sensitive-marker",
		"distributor-image-sensitive-marker",
	}
	_, err := model.UpsertSensitiveWordRuleWithMode(
		0, "distributor boundary", markers, model.SensitiveWordScopeGlobal, nil, 1, model.SensitiveWordModeBlock,
	)
	require.NoError(t, err)
	user := &model.User{
		Username: "distributor-sensitive-user", Password: "password", Role: common.RoleCommonUser,
		Status: common.UserStatusEnabled, Group: "default", Quota: 4_000_000,
		AuthVersion: 1, AffCode: "distributor-sensitive-aff",
	}
	require.NoError(t, database.Create(user).Error)
	token := &model.Token{
		UserId: user.Id, Key: "distributor-sensitive-token", Name: "distributor sensitive token",
		Status: common.TokenStatusEnabled, RemainQuota: 3_000_000, Group: "default",
	}
	require.NoError(t, database.Create(token).Error)

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	handlerCalled := false
	engine.POST("/*path", RequestId(), func(c *gin.Context) {
		c.Set("id", user.Id)
		c.Set("username", user.Username)
		c.Set("token_id", token.Id)
		c.Set("token_name", token.Name)
		common.SetContextKey(c, constant.ContextKeyUserId, user.Id)
		common.SetContextKey(c, constant.ContextKeyUserName, user.Username)
		common.SetContextKey(c, constant.ContextKeyUserGroup, "default")
		common.SetContextKey(c, constant.ContextKeyUsingGroup, "default")
		common.SetContextKey(c, constant.ContextKeyTokenGroup, "default")
		common.SetContextKey(c, constant.ContextKeyTokenId, token.Id)
	}, Distribute(), func(c *gin.Context) {
		handlerCalled = true
		c.Status(http.StatusNoContent)
	})

	for _, testCase := range []struct {
		name       string
		path       string
		body       string
		claudeBody bool
		geminiBody bool
	}{
		{
			name: "chat completions", path: "/v1/chat/completions",
			body: `{"model":"no-channel-chat","messages":[{"role":"user","content":"distributor-chat-sensitive-marker"}]}`,
		},
		{
			name: "responses user input", path: "/v1/responses",
			body: `{"model":"no-channel-responses","input":"distributor-responses-sensitive-marker"}`,
		},
		{
			name: "responses tool result without new user text", path: "/v1/responses",
			body: `{"model":"no-channel-responses","input":[{"type":"function_call_output","call_id":"call-1","output":"distributor-tool-result-sensitive-marker"}]}`,
		},
		{
			name: "claude messages", path: "/v1/messages", claudeBody: true,
			body: `{"model":"no-channel-claude","max_tokens":1,"messages":[{"role":"user","content":"distributor-claude-sensitive-marker"}]}`,
		},
		{
			name: "gemini", path: "/v1beta/models/no-channel-gemini:generateContent", geminiBody: true,
			body: `{"contents":[{"role":"user","parts":[{"text":"distributor-gemini-sensitive-marker"}]}]}`,
		},
		{
			name: "image generation", path: "/v1/images/generations",
			body: `{"model":"no-channel-image","prompt":"distributor-image-sensitive-marker"}`,
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			handlerCalled = false
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, testCase.path, strings.NewReader(testCase.body))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Accept-Language", "en")
			engine.ServeHTTP(recorder, request)

			require.Equal(t, http.StatusBadRequest, recorder.Code)
			assert.False(t, handlerCalled, "the relay handler and channel selection must not run")
			assert.NotContains(t, recorder.Body.String(), "model_not_found")
			assert.NotContains(t, recorder.Body.String(), "request id:")
			if testCase.claudeBody {
				var response struct {
					Type      string `json:"type"`
					RequestID string `json:"request_id"`
					Error     struct {
						Type    string `json:"type"`
						Message string `json:"message"`
					} `json:"error"`
				}
				require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
				assert.Equal(t, "error", response.Type)
				assert.Equal(t, "invalid_request_error", response.Error.Type)
				assert.Equal(t, i18n.Translate(i18n.LangEn, i18n.MsgSensitiveWordBlocked), response.Error.Message)
				assert.NotEmpty(t, response.RequestID)
				return
			}
			if testCase.geminiBody {
				var response struct {
					Error struct {
						Code    int    `json:"code"`
						Message string `json:"message"`
						Status  string `json:"status"`
					} `json:"error"`
				}
				require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
				assert.Equal(t, http.StatusBadRequest, response.Error.Code)
				assert.Equal(t, "INVALID_ARGUMENT", response.Error.Status)
				assert.Equal(t, i18n.Translate(i18n.LangEn, i18n.MsgSensitiveWordBlocked), response.Error.Message)
				return
			}
			var response struct {
				Error struct {
					Type    string `json:"type"`
					Code    string `json:"code"`
					Message string `json:"message"`
				} `json:"error"`
			}
			require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
			assert.Equal(t, "invalid_request_error", response.Error.Type)
			assert.Equal(t, "sensitive_words_detected", response.Error.Code)
			assert.Equal(t, i18n.Translate(i18n.LangEn, i18n.MsgSensitiveWordBlocked), response.Error.Message)
		})
	}

	var storedUser model.User
	require.NoError(t, database.First(&storedUser, user.Id).Error)
	assert.Equal(t, len(markers), storedUser.SensitiveWordViolationCount)
}

func TestSensitiveWordRelayFormatExcludesVideoAndAsyncTasks(t *testing.T) {
	for _, path := range []string{
		"/v1/video/generations",
		"/v1/video/generations/task-1",
		"/v1/videos",
		"/v1/videos/video-1/remix",
		"/v1/tasks/video-plugin",
	} {
		_, supported := sensitiveWordRelayFormat(path)
		assert.False(t, supported, path)
	}
}

func TestSensitiveWordAutoBanResponseIsLocalizedAndNotRetryable(t *testing.T) {
	database := setupSensitiveWordDistributorTest(t)
	require.NoError(t, i18n.Init())
	require.NoError(t, model.MigrateSensitiveWordData())
	policy := model.GetSensitiveWordPolicy()
	policy.Enabled = true
	policy.CheckPrompt = true
	policy.AutoBanEnabled = true
	policy.BanThreshold = 1
	require.NoError(t, model.SaveSensitiveWordPolicy(policy, 1))

	const marker = "auto-ban-response-marker"
	_, err := model.UpsertSensitiveWordRuleWithMode(
		0, "auto-ban response", []string{marker}, model.SensitiveWordScopeGlobal, nil, 1, model.SensitiveWordModeBlock,
	)
	require.NoError(t, err)
	user := &model.User{
		Username: "auto-ban-response-user", Password: "password", Role: common.RoleCommonUser,
		Status: common.UserStatusEnabled, Group: "default", Quota: 4_000_000,
		AuthVersion: 1, AffCode: "auto-ban-response-aff",
	}
	require.NoError(t, database.Create(user).Error)
	token := &model.Token{
		UserId: user.Id, Key: "auto-ban-response-token", Name: "auto-ban response token",
		Status: common.TokenStatusEnabled, RemainQuota: 3_000_000, Group: "default",
	}
	require.NoError(t, database.Create(token).Error)

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	handlerCalled := false
	engine.POST("/v1/chat/completions", RequestId(), func(c *gin.Context) {
		c.Set("id", user.Id)
		c.Set("username", user.Username)
		c.Set("token_id", token.Id)
		c.Set("token_name", token.Name)
		common.SetContextKey(c, constant.ContextKeyUserId, user.Id)
		common.SetContextKey(c, constant.ContextKeyUserName, user.Username)
		common.SetContextKey(c, constant.ContextKeyUserGroup, "default")
		common.SetContextKey(c, constant.ContextKeyUsingGroup, "default")
		common.SetContextKey(c, constant.ContextKeyTokenGroup, "default")
		common.SetContextKey(c, constant.ContextKeyTokenId, token.Id)
	}, Distribute(), func(c *gin.Context) {
		handlerCalled = true
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPost,
		"/v1/chat/completions",
		strings.NewReader(`{"model":"no-channel-chat","messages":[{"role":"user","content":"auto-ban-response-marker"}]}`),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept-Language", "en")
	engine.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusForbidden, recorder.Code)
	assert.Empty(t, recorder.Header().Get("Retry-After"))
	assert.False(t, handlerCalled)
	var response struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    string `json:"code"`
		} `json:"error"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, "permission_error", response.Error.Type)
	assert.Equal(t, "user_banned", response.Error.Code)
	assert.Equal(t, i18n.Translate(i18n.LangEn, i18n.MsgSensitiveWordAutoBanned), response.Error.Message)

	var storedUser model.User
	require.NoError(t, database.First(&storedUser, user.Id).Error)
	assert.Equal(t, common.UserStatusDisabled, storedUser.Status)
	assert.Equal(t, 1, storedUser.SensitiveWordViolationCount)
}

func TestWriteSensitiveWordRequestErrorUsesClaudePermissionEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Set(common.RequestIdKey, "req-sensitive-auto-ban")
	apiErr := newSensitiveWordRequestError(
		"account disabled", "permission_error", types.ErrorCodeUserBanned, http.StatusForbidden,
	)

	writeSensitiveWordRequestError(context, types.RelayFormatClaude, apiErr)

	require.Equal(t, http.StatusForbidden, recorder.Code)
	assert.Empty(t, recorder.Header().Get("Retry-After"))
	var response struct {
		Type      string `json:"type"`
		RequestID string `json:"request_id"`
		Error     struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, "error", response.Type)
	assert.Equal(t, "req-sensitive-auto-ban", response.RequestID)
	assert.Equal(t, "permission_error", response.Error.Type)
	assert.Equal(t, "account disabled", response.Error.Message)
}

func TestWriteSensitiveWordRequestErrorUsesGeminiPermissionEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	apiErr := newSensitiveWordRequestError(
		"account disabled", "permission_error", types.ErrorCodeUserBanned, http.StatusForbidden,
	)

	writeSensitiveWordRequestError(context, types.RelayFormatGemini, apiErr)

	require.Equal(t, http.StatusForbidden, recorder.Code)
	var response struct {
		Error struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
			Status  string `json:"status"`
		} `json:"error"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, http.StatusForbidden, response.Error.Code)
	assert.Equal(t, "PERMISSION_DENIED", response.Error.Status)
	assert.Equal(t, "account disabled", response.Error.Message)
}
