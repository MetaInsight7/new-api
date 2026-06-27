package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// runGroupOverride builds a gin context that simulates TokenAuth having already
// set the default group, applies the optional X-Api-Group header, runs the
// GroupOverride middleware, and returns the context and response recorder.
func runGroupOverride(t *testing.T, headerVal string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	if headerVal != "" {
		c.Request.Header.Set(common.GroupHeaderName, headerVal)
	}
	common.SetContextKey(c, constant.ContextKeyUsingGroup, "default")
	common.SetContextKey(c, constant.ContextKeyTokenGroup, "default")
	GroupOverride()(c)
	return c, w
}

func TestGroupOverride(t *testing.T) {
	origEnabled := common.GroupHeaderOverrideEnabled
	origName := common.GroupHeaderName
	origAllow := common.GroupHeaderAllowedGroups
	t.Cleanup(func() {
		common.GroupHeaderOverrideEnabled = origEnabled
		common.GroupHeaderName = origName
		common.GroupHeaderAllowedGroups = origAllow
	})
	common.GroupHeaderName = "X-Api-Group"

	// "default" and "vip" are seeded by ratio_setting's package init, so
	// ContainsGroupRatio returns true for them without extra setup.

	t.Run("disabled: header is ignored", func(t *testing.T) {
		common.GroupHeaderOverrideEnabled = false
		common.GroupHeaderAllowedGroups = map[string]bool{}
		c, w := runGroupOverride(t, "vip")
		require.False(t, c.IsAborted())
		require.Equal(t, http.StatusOK, w.Code)
		require.Equal(t, "default", common.GetContextKeyString(c, constant.ContextKeyUsingGroup))
	})

	t.Run("enabled, empty header: group unchanged", func(t *testing.T) {
		common.GroupHeaderOverrideEnabled = true
		common.GroupHeaderAllowedGroups = map[string]bool{}
		c, _ := runGroupOverride(t, "")
		require.False(t, c.IsAborted())
		require.Equal(t, "default", common.GetContextKeyString(c, constant.ContextKeyUsingGroup))
	})

	t.Run("enabled, valid group, no allowlist: pins both group keys", func(t *testing.T) {
		common.GroupHeaderOverrideEnabled = true
		common.GroupHeaderAllowedGroups = map[string]bool{}
		c, _ := runGroupOverride(t, "vip")
		require.False(t, c.IsAborted())
		require.Equal(t, "vip", common.GetContextKeyString(c, constant.ContextKeyUsingGroup))
		require.Equal(t, "vip", common.GetContextKeyString(c, constant.ContextKeyTokenGroup))
	})

	t.Run("enabled, group not in allowlist: 403, group unchanged", func(t *testing.T) {
		common.GroupHeaderOverrideEnabled = true
		common.GroupHeaderAllowedGroups = map[string]bool{"image": true}
		c, w := runGroupOverride(t, "vip")
		require.True(t, c.IsAborted())
		require.Equal(t, http.StatusForbidden, w.Code)
		require.Equal(t, "default", common.GetContextKeyString(c, constant.ContextKeyUsingGroup))
	})

	t.Run("enabled, nonexistent group: 403", func(t *testing.T) {
		common.GroupHeaderOverrideEnabled = true
		common.GroupHeaderAllowedGroups = map[string]bool{}
		c, w := runGroupOverride(t, "does-not-exist")
		require.True(t, c.IsAborted())
		require.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("enabled, allow-listed and valid: pinned", func(t *testing.T) {
		common.GroupHeaderOverrideEnabled = true
		common.GroupHeaderAllowedGroups = map[string]bool{"vip": true}
		c, _ := runGroupOverride(t, "vip")
		require.False(t, c.IsAborted())
		require.Equal(t, "vip", common.GetContextKeyString(c, constant.ContextKeyUsingGroup))
	})
}
