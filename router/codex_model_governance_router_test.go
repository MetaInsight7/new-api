package router

import (
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCodexGovernanceRoutesDeclareActionSpecificPATScopes(t *testing.T) {
	engine := gin.New()
	RegisterCodexModelGovernanceRoutes(engine.Group("/api"))
	for _, route := range []struct{ method, path, scope string }{
		{http.MethodGet, "/api/codex_model_governance/", "channel:read"},
		{http.MethodPost, "/api/codex_model_governance/rules/test", "channel:operate"},
		{http.MethodPost, "/api/codex_model_governance/trigger", "channel:operate"},
		{http.MethodPost, "/api/codex_model_governance/:id/review/disable", "channel:operate"},
		{http.MethodPost, "/api/codex_model_governance/:id/review/restore", "channel:operate"},
		{http.MethodPost, "/api/codex_model_governance/:id/review/ignore", "channel:operate"},
		{http.MethodPost, "/api/codex_model_governance/:id/review/confirm_remove", "channel:write"},
	} {
		rule, declared := middleware.AccessTokenRouteRule(route.method + " " + route.path)
		require.True(t, declared, route.path)
		assert.Equal(t, route.scope, rule.Scope(), route.path)
	}
	for _, route := range engine.Routes() {
		assert.NotEqual(t, "/api/codex_model_governance/:id/review", route.Path)
	}
}
