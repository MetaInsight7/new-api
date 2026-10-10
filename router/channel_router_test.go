package router

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service/authz"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestChannelDefaultBaseURLsRequireReadPermission(t *testing.T) {
	assertChannelRoutePermission(t, http.MethodGet, "/default_base_urls", authz.ChannelRead, controller.GetChannelDefaultBaseURLs)

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	registerChannelRoutes(engine.Group("/api"))
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/channel/default_base_urls", nil))
	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
}

func TestChannelConcurrencySnapshotRequiresReadPermission(t *testing.T) {
	assertChannelRoutePermission(t, http.MethodGet, "/concurrency", authz.ChannelRead, controller.GetChannelConcurrencyStatus)

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	registerChannelRoutes(engine.Group("/api"))

	registered := false
	for _, route := range engine.Routes() {
		if route.Method == http.MethodGet && route.Path == "/api/channel/concurrency" {
			registered = true
			break
		}
	}
	require.True(t, registered)

	rule, declared := middleware.AccessTokenRouteRule("GET /api/channel/concurrency")
	require.True(t, declared)
	assert.Equal(t, "channel:read", rule.Scope())

	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/channel/concurrency?ids=1", nil))
	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
}

func TestChannelConcurrencySnapshotAuthorizesReadScopeAndPermission(t *testing.T) {
	engine, admin, channelReadToken, profileReadToken := newChannelConcurrencyAuthorizationEngine(t)

	authorized := channelConcurrencyBearerRequest(engine, channelReadToken)
	require.Equal(t, http.StatusOK, authorized.Code, authorized.Body.String())
	assert.Contains(t, authorized.Body.String(), `"success":true`)

	wrongScope := channelConcurrencyBearerRequest(engine, profileReadToken)
	require.Equal(t, http.StatusForbidden, wrongScope.Code, wrongScope.Body.String())
	assert.Contains(t, wrongScope.Body.String(), `"code":"ACCESS_TOKEN_SCOPE_DENIED"`)

	require.NoError(t, authz.SetUserPermissions(admin.Id, authz.PermissionsMap{
		authz.ResourceChannel: {authz.ActionRead: false},
	}))
	missingPermission := channelConcurrencyBearerRequest(engine, channelReadToken)
	require.Equal(t, http.StatusForbidden, missingPermission.Code, missingPermission.Body.String())
	assert.NotContains(t, missingPermission.Body.String(), "ACCESS_TOKEN_SCOPE_DENIED")
}

func TestChannelStatusRoutesUseExpectedPermissions(t *testing.T) {
	assertChannelRoutePermission(t, http.MethodGet, "/:id/vllm/status", authz.ChannelRead, controller.GetVLLMChannelStatus)
	assertChannelRoutePermission(t, http.MethodGet, "/:id/sglang/status", authz.ChannelRead, controller.GetSGLangChannelStatus)
	assertChannelRoutePermission(t, http.MethodPost, "/:id/status", authz.ChannelOperate, controller.UpdateChannelStatus)
	assertChannelRoutePermission(t, http.MethodPost, "/status/batch", authz.ChannelOperate, controller.BatchUpdateChannelStatus)
	assertChannelRoutePermission(t, http.MethodPut, "/", authz.ChannelWrite, controller.UpdateChannel)
}

func TestChannelDeleteRoutesUseSensitiveWritePermission(t *testing.T) {
	assertChannelRoutePermission(t, http.MethodDelete, "/:id", authz.ChannelSensitiveWrite, controller.DeleteChannel)
	assertChannelRoutePermission(t, http.MethodPost, "/batch", authz.ChannelSensitiveWrite, controller.DeleteChannelBatch)
	assertChannelRoutePermission(t, http.MethodDelete, "/disabled", authz.ChannelSensitiveWrite, controller.DeleteDisabledChannel)
	assertChannelRoutePermission(t, http.MethodPut, "/", authz.ChannelWrite, controller.UpdateChannel)
	assertChannelRoutePermission(t, http.MethodPut, "/tag", authz.ChannelWrite, controller.EditTagChannels)
	assertChannelRoutePermission(t, http.MethodPost, "/batch/tag", authz.ChannelWrite, controller.BatchSetChannelTag)
}

func TestChannelStatusRoutesRegisterWithoutConflict(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	api := engine.Group("/api")

	require.NotPanics(t, func() {
		registerChannelRoutes(api)
	})
}

func TestCodexLimitsAuthenticatesBeforeCheckingChannelPermission(test *testing.T) {
	engine := newAccessTokenScopeTestEngine(test)
	rule, declared := middleware.AccessTokenRouteRule("GET /api/data/codex/limits")
	require.True(test, declared)
	assert.Equal(test, "channel:read", rule.Scope())
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/data/codex/limits", nil))
	assert.Equal(test, http.StatusUnauthorized, recorder.Code)
}

func assertChannelRoutePermission(t *testing.T, method string, path string, permission authz.Permission, handler any) {
	t.Helper()
	for _, route := range channelPermissionRoutes {
		if route.method == method && route.path == path {
			assert.Equal(t, permission, route.permission)
			assert.Equal(t, reflect.ValueOf(handler).Pointer(), reflect.ValueOf(route.handler).Pointer())
			return
		}
	}
	t.Fatalf("route %s %s not found", method, path)
}

func newChannelConcurrencyAuthorizationEngine(t *testing.T) (*gin.Engine, *model.User, string, string) {
	t.Helper()
	previousDB := model.DB
	previousLogDB := model.LOG_DB
	previousDatabaseType := common.MainDatabaseType()
	previousLogDatabaseType := common.LogDatabaseType()
	previousRedisEnabled := common.RedisEnabled
	previousMasterNode := common.IsMasterNode

	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDatabase, err := database.DB()
	require.NoError(t, err)
	sqlDatabase.SetMaxOpenConns(1)
	require.NoError(t, database.AutoMigrate(
		&model.User{},
		&model.UserAccessToken{},
		&model.AuditLog{},
		&model.CasbinRule{},
		&model.AuthzRole{},
	))
	model.DB = database
	model.LOG_DB = database
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	common.RedisEnabled = false
	common.IsMasterNode = true
	require.NoError(t, authz.Init(database))
	t.Cleanup(func() {
		model.DB = previousDB
		model.LOG_DB = previousLogDB
		common.SetDatabaseTypes(previousDatabaseType, previousLogDatabaseType)
		common.RedisEnabled = previousRedisEnabled
		common.IsMasterNode = previousMasterNode
		require.NoError(t, sqlDatabase.Close())
	})

	admin := &model.User{
		Username:    "concurrency-reader",
		Password:    "password-placeholder",
		Role:        common.RoleAdminUser,
		Status:      common.UserStatusEnabled,
		Group:       "default",
		AffCode:     "concurrency-reader",
		AuthVersion: 1,
	}
	require.NoError(t, database.Create(admin).Error)
	channelReadToken := createChannelConcurrencyScopedToken(t, database, admin.Id, "channel:read")
	profileReadToken := createChannelConcurrencyScopedToken(t, database, admin.Id, "profile:read")

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	registerChannelRoutes(engine.Group("/api"))
	return engine, admin, channelReadToken, profileReadToken
}

func createChannelConcurrencyScopedToken(t *testing.T, database *gorm.DB, userID int, scope string) string {
	t.Helper()
	suffix, err := common.GenerateRandomCharsKey(43)
	require.NoError(t, err)
	raw := model.AccessTokenPrefix + suffix
	token := &model.UserAccessToken{
		UserId:    userID,
		Name:      scope,
		TokenHash: model.AccessTokenFingerprint(raw),
		TokenHint: model.AccessTokenHint(raw),
	}
	require.NoError(t, token.SetScopes([]string{scope}))
	require.NoError(t, database.Create(token).Error)
	return raw
}

func channelConcurrencyBearerRequest(engine *gin.Engine, token string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodGet, "/api/channel/concurrency", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)
	return recorder
}
