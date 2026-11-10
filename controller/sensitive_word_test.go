package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service/authz"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newSensitiveWordAuthorizationEngine() *gin.Engine {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		actorID, _ := strconv.Atoi(c.GetHeader("X-Test-Actor-ID"))
		actorRole, _ := strconv.Atoi(c.GetHeader("X-Test-Actor-Role"))
		c.Set("id", actorID)
		c.Set("role", actorRole)
		c.Set("username", fmt.Sprintf("actor-%d", actorID))
		c.Next()
	})
	engine.PATCH(
		"/api/sensitive-words/users/:id",
		middleware.RequirePermission(authz.SensitiveWordManage),
		UpdateSensitiveWordUserSafety,
	)
	engine.POST(
		"/api/sensitive-words/users/:id/unban",
		middleware.RequirePermission(authz.SensitiveWordManage),
		UnbanSensitiveWordUser,
	)
	engine.GET(
		"/api/log/sensitive-word-audit/:id",
		middleware.RequirePermission(authz.SensitiveWordEvidenceRead),
		GetSensitiveWordAudit,
	)
	engine.GET(
		"/api/sensitive-words/rules/:id",
		middleware.RequirePermission(authz.SensitiveWordManage),
		GetSensitiveWordRule,
	)
	engine.GET("/api/user/:id", GetUser)
	return engine
}

func performSensitiveWordAdminRequest(engine http.Handler, method, path, body string, actorID, actorRole int) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Test-Actor-ID", strconv.Itoa(actorID))
	request.Header.Set("X-Test-Actor-Role", strconv.Itoa(actorRole))
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)
	return recorder
}

func setupSensitiveWordControllerAuthorization(t *testing.T) (*gin.Engine, *model.User) {
	t.Helper()
	database := setupManageUserTestDB(t)
	previousMaster := common.IsMasterNode
	common.IsMasterNode = true
	t.Cleanup(func() { common.IsMasterNode = previousMaster })
	require.NoError(t, database.AutoMigrate(&model.Token{}, &model.SensitiveWordAuditEvent{}))
	require.NoError(t, authz.Init(database))

	target := &model.User{
		Username: "sensitive-target", Password: "password", Role: common.RoleCommonUser,
		Status: common.UserStatusDisabled, Group: "default", AuthVersion: 3,
		SensitiveWordViolationCount: 7, Quota: 8_000, UsedQuota: 900,
	}
	require.NoError(t, database.Create(target).Error)
	return newSensitiveWordAuthorizationEngine(), target
}

func TestSensitiveWordManagementPermissionAndDedicatedUnban(t *testing.T) {
	engine, target := setupSensitiveWordControllerAuthorization(t)
	now := time.Now().Unix()
	require.NoError(t, model.DB.Create(&model.UserSession{
		SID: "sensitive-unban-session", UserID: target.Id, Version: 1, UserAuthVersion: target.AuthVersion,
		Status: model.UserSessionStatusActive, RefreshHash: "sensitive-unban-refresh", LoginMethod: "password",
		LastActiveAt: now, ExpiresAt: now + 3600,
	}).Error)
	token := model.Token{
		UserId: target.Id, Key: "sensitive-unban-key", Name: "existing key",
		Status: common.TokenStatusEnabled, ExpiredTime: -1, UnlimitedQuota: true,
	}
	require.NoError(t, model.DB.Create(&token).Error)

	const managerID = 200
	patch := performSensitiveWordAdminRequest(
		engine, http.MethodPatch, fmt.Sprintf("/api/sensitive-words/users/%d", target.Id),
		`{"whitelist":true}`, managerID, common.RoleAdminUser,
	)
	require.Equal(t, http.StatusOK, patch.Code, patch.Body.String())

	var updated model.User
	require.NoError(t, model.DB.First(&updated, target.Id).Error)
	assert.True(t, updated.SensitiveWordWhitelist)
	assert.Equal(t, 7, updated.SensitiveWordViolationCount, "whitelist changes preserve history")

	require.NoError(t, authz.SetUserPermissions(managerID, authz.PermissionsMap{
		authz.ResourceSensitiveWord: {authz.ActionManage: false},
	}))
	denied := performSensitiveWordAdminRequest(
		engine, http.MethodPatch, fmt.Sprintf("/api/sensitive-words/users/%d", target.Id),
		`{"violation_count":0}`, managerID, common.RoleAdminUser,
	)
	assert.Equal(t, http.StatusForbidden, denied.Code)
	require.NoError(t, model.DB.First(&updated, target.Id).Error)
	assert.Equal(t, 7, updated.SensitiveWordViolationCount)

	require.NoError(t, authz.SetUserPermissions(managerID, authz.PermissionsMap{
		authz.ResourceSensitiveWord: {authz.ActionManage: true},
	}))
	unban := performSensitiveWordAdminRequest(
		engine, http.MethodPost, fmt.Sprintf("/api/sensitive-words/users/%d/unban", target.Id),
		"", managerID, common.RoleAdminUser,
	)
	require.Equal(t, http.StatusOK, unban.Code, unban.Body.String())
	require.NoError(t, model.DB.First(&updated, target.Id).Error)
	assert.Equal(t, common.UserStatusEnabled, updated.Status)
	assert.Zero(t, updated.SensitiveWordViolationCount)
	assert.True(t, updated.SensitiveWordWhitelist, "unban is independent from whitelist state")
	assert.Equal(t, 8_000, updated.Quota)
	assert.Equal(t, 900, updated.UsedQuota)
	assert.EqualValues(t, 4, updated.AuthVersion)

	var session model.UserSession
	require.NoError(t, model.DB.First(&session, "sid = ?", "sensitive-unban-session").Error)
	assert.Equal(t, model.UserSessionStatusRevoked, session.Status)
	assert.Equal(t, "sensitive_word_enable", session.RevokedReason)
	var storedToken model.Token
	require.NoError(t, model.DB.First(&storedToken, token.Id).Error)
	assert.Equal(t, common.TokenStatusEnabled, storedToken.Status)
	assert.Equal(t, "sensitive-unban-key", storedToken.Key)

	ordinary := performSensitiveWordAdminRequest(
		engine, http.MethodPatch, fmt.Sprintf("/api/sensitive-words/users/%d", target.Id),
		`{"whitelist":false}`, 201, common.RoleCommonUser,
	)
	assert.Equal(t, http.StatusForbidden, ordinary.Code)
}

func TestSensitiveWordEvidencePermissionIsIndependentAndProtectsRoot(t *testing.T) {
	engine, target := setupSensitiveWordControllerAuthorization(t)
	legacyRoot := &model.User{
		Username: "legacy-root-evidence", Password: "password", Role: common.RoleRootUser,
		Status: common.UserStatusEnabled, Group: "default", AffCode: "legacy-root-evidence-aff",
	}
	require.NoError(t, model.DB.Create(legacyRoot).Error)
	commonEvent := model.SensitiveWordAuditEvent{
		RequestID: "common-evidence", UserID: target.Id, UserRole: common.RoleCommonUser,
		PromptHash: "common-secret-hash", Evidence: `{"version":1,"fields":[]}`,
	}
	rootEvent := model.SensitiveWordAuditEvent{
		RequestID: "root-evidence", UserID: legacyRoot.Id, UserRole: common.RoleRootUser,
		PromptHash: "root-secret-hash", Evidence: `{"version":1,"fields":[]}`,
	}
	legacyRootEvent := model.SensitiveWordAuditEvent{
		RequestID: "legacy-root-evidence", UserID: legacyRoot.Id,
		PromptHash: "legacy-root-secret-hash", Evidence: `{"version":1,"fields":[]}`,
	}
	require.NoError(t, model.DB.Create(&commonEvent).Error)
	require.NoError(t, model.DB.Create(&rootEvent).Error)
	require.NoError(t, model.DB.Create(&legacyRootEvent).Error)

	const readerID = 300
	manageOnly := performSensitiveWordAdminRequest(
		engine, http.MethodGet, fmt.Sprintf("/api/log/sensitive-word-audit/%d", commonEvent.ID),
		"", readerID, common.RoleAdminUser,
	)
	assert.Equal(t, http.StatusForbidden, manageOnly.Code)
	assert.NotContains(t, manageOnly.Body.String(), "common-secret-hash")

	require.NoError(t, authz.SetUserPermissions(readerID, authz.PermissionsMap{
		authz.ResourceSensitiveWord: {
			authz.ActionManage:       false,
			authz.ActionEvidenceRead: true,
		},
	}))
	readCommon := performSensitiveWordAdminRequest(
		engine, http.MethodGet, fmt.Sprintf("/api/log/sensitive-word-audit/%d", commonEvent.ID),
		"", readerID, common.RoleAdminUser,
	)
	require.Equal(t, http.StatusOK, readCommon.Code, readCommon.Body.String())
	assert.Contains(t, readCommon.Body.String(), "common-secret-hash")

	manageDenied := performSensitiveWordAdminRequest(
		engine, http.MethodPatch, fmt.Sprintf("/api/sensitive-words/users/%d", target.Id),
		`{"whitelist":true}`, readerID, common.RoleAdminUser,
	)
	assert.Equal(t, http.StatusForbidden, manageDenied.Code)

	readRoot := performSensitiveWordAdminRequest(
		engine, http.MethodGet, fmt.Sprintf("/api/log/sensitive-word-audit/%d", rootEvent.ID),
		"", readerID, common.RoleAdminUser,
	)
	assert.Equal(t, http.StatusForbidden, readRoot.Code)
	assert.Contains(t, readRoot.Body.String(), `"message_key":"No permission to view this audit evidence"`)
	assert.NotContains(t, readRoot.Body.String(), "root-secret-hash")
	readLegacyRoot := performSensitiveWordAdminRequest(
		engine, http.MethodGet, fmt.Sprintf("/api/log/sensitive-word-audit/%d", legacyRootEvent.ID),
		"", readerID, common.RoleAdminUser,
	)
	assert.Equal(t, http.StatusForbidden, readLegacyRoot.Code)
	assert.NotContains(t, readLegacyRoot.Body.String(), "legacy-root-secret-hash")

	rootRead := performSensitiveWordAdminRequest(
		engine, http.MethodGet, fmt.Sprintf("/api/log/sensitive-word-audit/%d", rootEvent.ID),
		"", 1, common.RoleRootUser,
	)
	require.Equal(t, http.StatusOK, rootRead.Code, rootRead.Body.String())
	assert.Contains(t, rootRead.Body.String(), "root-secret-hash")
}

func TestUserSafetyStateProjectionRequiresManagementPermission(t *testing.T) {
	engine, target := setupSensitiveWordControllerAuthorization(t)
	const actorID = 400

	visible := performSensitiveWordAdminRequest(
		engine, http.MethodGet, fmt.Sprintf("/api/user/%d", target.Id), "", actorID, common.RoleAdminUser,
	)
	require.Equal(t, http.StatusOK, visible.Code, visible.Body.String())
	assert.Contains(t, visible.Body.String(), `"sensitive_word_violation_count":7`)
	assert.Contains(t, visible.Body.String(), `"sensitive_word_whitelist":false`)

	require.NoError(t, authz.SetUserPermissions(actorID, authz.PermissionsMap{
		authz.ResourceSensitiveWord: {authz.ActionManage: false},
	}))
	hidden := performSensitiveWordAdminRequest(
		engine, http.MethodGet, fmt.Sprintf("/api/user/%d", target.Id), "", actorID, common.RoleAdminUser,
	)
	require.Equal(t, http.StatusOK, hidden.Code, hidden.Body.String())
	assert.Contains(t, hidden.Body.String(), `"sensitive_word_violation_count":0`)
	assert.Contains(t, hidden.Body.String(), `"sensitive_word_whitelist":false`)
	assert.NotContains(t, hidden.Body.String(), `"sensitive_word_violation_count":7`)
}

func TestSensitiveWordManagementErrorsExposeTranslationKeys(t *testing.T) {
	engine, _ := setupSensitiveWordControllerAuthorization(t)
	const managerID = 500

	invalidAudit := performSensitiveWordAdminRequest(
		engine, http.MethodGet, "/api/log/sensitive-word-audit/not-an-id", "", 1, common.RoleRootUser,
	)
	assert.Contains(t, invalidAudit.Body.String(), `"message_key":"Invalid audit ID"`)

	invalidRule := performSensitiveWordAdminRequest(
		engine, http.MethodGet, "/api/sensitive-words/rules/not-an-id", "", managerID, common.RoleAdminUser,
	)
	assert.Contains(t, invalidRule.Body.String(), `"message_key":"Invalid rule ID"`)

	emptyUpdate := performSensitiveWordAdminRequest(
		engine, http.MethodPatch, "/api/sensitive-words/users/1", `{}`, managerID, common.RoleAdminUser,
	)
	assert.Contains(t, emptyUpdate.Body.String(), `"message_key":"At least one sensitive-word safety field is required"`)

	peer := &model.User{
		Username: "sensitive-peer-admin", Password: "password", Role: common.RoleAdminUser,
		Status: common.UserStatusDisabled, Group: "default", AffCode: "sensitive-peer-aff",
	}
	require.NoError(t, model.DB.Create(peer).Error)
	updatePeer := performSensitiveWordAdminRequest(
		engine, http.MethodPatch, fmt.Sprintf("/api/sensitive-words/users/%d", peer.Id),
		`{"whitelist":true}`, managerID, common.RoleAdminUser,
	)
	require.Equal(t, http.StatusForbidden, updatePeer.Code, updatePeer.Body.String())
	assert.Contains(t, updatePeer.Body.String(), `"message_key":"No permission to update this user"`)

	unbanPeer := performSensitiveWordAdminRequest(
		engine, http.MethodPost, fmt.Sprintf("/api/sensitive-words/users/%d/unban", peer.Id),
		"", managerID, common.RoleAdminUser,
	)
	require.Equal(t, http.StatusForbidden, unbanPeer.Code, unbanPeer.Body.String())
	assert.Contains(t, unbanPeer.Body.String(), `"message_key":"No permission to update this user"`)
}

func TestSensitiveWordAuditActionMappings(t *testing.T) {
	assert.Equal(
		t,
		"Updated sensitive-word safety state for user 42",
		auditContentEN("sensitive_word.user_update", map[string]any{"target_user_id": 42}),
	)
	assert.Equal(
		t,
		"Unbanned user 42 and reset sensitive-word violations",
		auditContentEN("sensitive_word.unban", map[string]any{"target_user_id": 42}),
	)
}
