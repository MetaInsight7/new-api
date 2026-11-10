package controller

import (
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

func GetSensitiveWordPolicy(c *gin.Context) {
	policy, err := model.GetSensitiveWordPolicyWithError()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": policy})
}

func UpdateSensitiveWordPolicy(c *gin.Context) {
	var policy model.SensitiveWordPolicy
	if err := common.DecodeJson(c.Request.Body, &policy); err != nil {
		common.ApiError(c, err)
		return
	}
	previous, err := model.GetSensitiveWordPolicyWithError()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.SaveSensitiveWordPolicy(policy, c.GetInt("id")); err != nil {
		common.ApiError(c, err)
		return
	}
	updated, err := model.GetSensitiveWordPolicyWithError()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "sensitive_word.policy_update", map[string]any{
		"enabled_before": previous.Enabled, "enabled_after": updated.Enabled,
		"check_prompt":          updated.CheckPrompt,
		"auto_ban_enabled":      updated.AutoBanEnabled,
		"retain_full_prompt":    updated.RetainFullPrompt,
		"ban_threshold":         updated.BanThreshold,
		"retention_days":        updated.FullPromptRetentionDays,
		"max_prompt_runes":      updated.MaxPromptRunes,
		"block_message_changed": previous.BlockMessage != updated.BlockMessage,
	})
	c.JSON(http.StatusOK, gin.H{"success": true, "data": updated})
}

func GetSensitiveWordGroups(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true, "data": model.ListSensitiveWordGroups()})
}

func GetSensitiveWordRules(c *gin.Context) {
	rules, err := model.ListSensitiveWordRules()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": rules})
}

func GetSensitiveWordRule(c *gin.Context) {
	id, ok := sensitiveWordRuleID(c)
	if !ok {
		return
	}
	rule, err := model.GetSensitiveWordRuleDetail(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": rule})
}

type sensitiveRuleRequest struct {
	Name   string   `json:"name"`
	Words  []string `json:"words"`
	Scope  string   `json:"scope"`
	Groups []string `json:"groups"`
	Mode   string   `json:"mode"`
}

func decodeSensitiveRuleRequest(c *gin.Context) (sensitiveRuleRequest, bool) {
	var request sensitiveRuleRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiError(c, err)
		return request, false
	}
	if request.Mode == "" {
		request.Mode = model.SensitiveWordModeObserve
	}
	return request, true
}

func CreateSensitiveWordRule(c *gin.Context) {
	request, ok := decodeSensitiveRuleRequest(c)
	if !ok {
		return
	}
	rule, err := model.UpsertSensitiveWordRuleWithMode(
		0, request.Name, request.Words, request.Scope, request.Groups, c.GetInt("id"), request.Mode,
	)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "sensitive_word_rule.create", map[string]any{
		"rule_id": rule.ID, "name": rule.Name, "scope": rule.Scope,
		"mode": rule.Mode, "word_count": rule.WordCount,
	})
	c.JSON(http.StatusOK, gin.H{"success": true, "data": rule})
}

func UpdateSensitiveWordRule(c *gin.Context) {
	id, ok := sensitiveWordRuleID(c)
	if !ok {
		return
	}
	request, ok := decodeSensitiveRuleRequest(c)
	if !ok {
		return
	}
	rule, err := model.UpsertSensitiveWordRuleWithMode(
		id, request.Name, request.Words, request.Scope, request.Groups, c.GetInt("id"), request.Mode,
	)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "sensitive_word_rule.update", map[string]any{
		"rule_id": rule.ID, "name": rule.Name, "scope": rule.Scope,
		"mode": rule.Mode, "word_count": rule.WordCount,
	})
	c.JSON(http.StatusOK, gin.H{"success": true, "data": rule})
}

func SetSensitiveWordRuleMode(c *gin.Context) {
	id, ok := sensitiveWordRuleID(c)
	if !ok {
		return
	}
	var request struct {
		Mode string `json:"mode"`
	}
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.SetSensitiveWordRuleMode(id, request.Mode, c.GetInt("id")); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "sensitive_word_rule.mode", map[string]any{
		"rule_id": id, "mode": request.Mode,
	})
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func DeleteSensitiveWordRule(c *gin.Context) {
	id, ok := sensitiveWordRuleID(c)
	if !ok {
		return
	}
	if err := model.DeleteSensitiveWordRule(id, c.GetInt("id")); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "sensitive_word_rule.delete", map[string]any{"rule_id": id})
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func GetSensitiveWordAudit(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		common.ApiErrorT(c, "Invalid audit ID")
		return
	}
	var event model.SensitiveWordAuditEvent
	if err := model.DB.First(&event, id).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	if c.GetInt("role") != common.RoleRootUser {
		subjectRole := event.UserRole
		if subjectRole == 0 {
			var subject model.User
			if err := model.DB.Select("role").First(&subject, event.UserID).Error; err != nil {
				common.ApiErrorStatus(c, http.StatusForbidden, common.NewMessage("No permission to view this audit evidence"))
				return
			}
			subjectRole = subject.Role
		}
		if subjectRole == common.RoleRootUser {
			common.ApiErrorStatus(c, http.StatusForbidden, common.NewMessage("No permission to view this audit evidence"))
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": event})
}

func UpdateSensitiveWordUserSafety(c *gin.Context) {
	userID, ok := sensitiveWordUserID(c)
	if !ok {
		return
	}
	var request struct {
		ViolationCount *int  `json:"violation_count"`
		Whitelist      *bool `json:"whitelist"`
	}
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiError(c, err)
		return
	}
	if request.ViolationCount == nil && request.Whitelist == nil {
		common.ApiErrorT(c, "At least one sensitive-word safety field is required")
		return
	}
	user, err := model.GetUserById(userID, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !canManageTargetRole(c.GetInt("role"), user.Role) {
		common.ApiErrorStatus(c, http.StatusForbidden, common.NewMessage("No permission to update this user"))
		return
	}
	if err := model.UpdateSensitiveWordUserFields(userID, request.ViolationCount, request.Whitelist); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAuditFor(c, userID, "sensitive_word.user_update", map[string]any{
		"target_user_id":    userID,
		"count_updated":     request.ViolationCount != nil,
		"whitelist_updated": request.Whitelist != nil,
	})
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func UnbanSensitiveWordUser(c *gin.Context) {
	userID, ok := sensitiveWordUserID(c)
	if !ok {
		return
	}
	user, err := model.GetUserById(userID, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !canManageTargetRole(c.GetInt("role"), user.Role) {
		common.ApiErrorStatus(c, http.StatusForbidden, common.NewMessage("No permission to update this user"))
		return
	}
	result, err := model.EnableUserAndResetSensitiveWordViolations(userID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAuditFor(c, userID, "sensitive_word.unban", map[string]any{
		"target_user_id": userID,
		"status_changed": result.StatusChanged,
		"count_reset":    result.ViolationCountReset,
	})
	c.JSON(http.StatusOK, gin.H{"success": true, "data": result})
}

func sensitiveWordRuleID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		common.ApiErrorT(c, "Invalid rule ID")
		return 0, false
	}
	return id, true
}

func sensitiveWordUserID(c *gin.Context) (int, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorT(c, "Invalid user ID")
		return 0, false
	}
	return id, true
}
