package router

import (
	"net/http"

	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/service/authz"
	"github.com/gin-gonic/gin"
)

func RegisterCodexModelGovernanceRoutes(apiRouter *gin.RouterGroup) {
	routes := apiRouter.Group("/codex_model_governance", middleware.AdminAuth(), middleware.DisableCache())
	handlePermissionRoute(routes, http.MethodGet, "/", authz.ChannelRead, controller.ListCodexModelGovernanceRecords)
	handlePermissionRoute(routes, http.MethodPost, "/rules/test", authz.ChannelOperate, controller.TestCodexModelGovernanceRule)
	handlePermissionRoute(routes, http.MethodPost, "/trigger", authz.ChannelOperate, controller.TriggerCodexModelGovernanceTask)
	for _, action := range []string{service.CodexModelGovernanceReviewActionDisable, service.CodexModelGovernanceReviewActionRestore, service.CodexModelGovernanceReviewActionIgnore, service.CodexModelGovernanceReviewActionConfirmRemove} {
		permission := authz.ChannelOperate
		if action == service.CodexModelGovernanceReviewActionConfirmRemove {
			permission = authz.ChannelWrite
		}
		handlePermissionRoute(routes, http.MethodPost, "/:id/review/"+action, permission, controller.ReviewCodexModelGovernanceAction(action))
	}
}
