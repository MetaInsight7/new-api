package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/gin-gonic/gin"
)

// GroupOverride pins the using-group of a relay request based on a request
// header (default "X-Api-Group"). The header is expected to be injected by a
// trusted reverse proxy in front of the backend, so that different exposed
// URLs/domains can be locked to different channel groups (e.g. one URL only for
// image generation, another only for coding).
//
// Security model: the header is only honored when GROUP_HEADER_OVERRIDE_ENABLED
// is set, and the backend must not be publicly reachable except through the
// reverse proxies that always overwrite the header. A client-supplied value is
// therefore replaced by the proxy and cannot be used to escalate. As an extra
// guard, GROUP_HEADER_ALLOWED_GROUPS bounds which groups a header may select
// (empty = any existing group), so even a request that bypasses the proxy can
// only reach allow-listed groups — never sensitive ones like a billing tier.
//
// This middleware must run AFTER TokenAuth (which sets the default group) and
// BEFORE Distribute (which selects a channel from the group). It overrides both
// the using-group and the token-group context keys so that channel selection,
// model listing and group rate limiting all follow the pinned group.
func GroupOverride() func(c *gin.Context) {
	return func(c *gin.Context) {
		if !common.GroupHeaderOverrideEnabled {
			c.Next()
			return
		}
		group := strings.TrimSpace(c.GetHeader(common.GroupHeaderName))
		if group == "" {
			c.Next()
			return
		}
		// Only allow groups the operator explicitly permits via header.
		// An empty allowlist means any existing group is allowed.
		if len(common.GroupHeaderAllowedGroups) > 0 && !common.GroupHeaderAllowedGroups[group] {
			abortWithOpenAiMessage(c, http.StatusForbidden, fmt.Sprintf("分组 %s 不允许通过请求头覆盖 / group %q is not allowed via header", group, group))
			return
		}
		// "auto" is a special multi-group selector resolved by the distributor.
		if group != "auto" && !ratio_setting.ContainsGroupRatio(group) {
			abortWithOpenAiMessage(c, http.StatusForbidden, fmt.Sprintf("分组 %s 不存在或未配置 / group %q does not exist", group, group))
			return
		}
		common.SetContextKey(c, constant.ContextKeyUsingGroup, group)
		common.SetContextKey(c, constant.ContextKeyTokenGroup, group)
		c.Next()
	}
}
