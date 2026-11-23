package model

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

type SensitiveWordEventFilter struct {
	StartTimestamp int64
	EndTimestamp   int64
	Disposition    string
	Keyword        string
}

func ValidSensitiveWordEventDisposition(disposition string) bool {
	switch disposition {
	case "", "block", "observe", "whitelist_bypass":
		return true
	default:
		return false
	}
}

func ListSensitiveWordEvents(filter SensitiveWordEventFilter, offset, limit int) ([]SensitiveWordDashboardRecentEvent, int64, error) {
	if !ValidSensitiveWordMonitorTimestampRange(filter.StartTimestamp, filter.EndTimestamp) {
		return nil, 0, fmt.Errorf("invalid sensitive-word event range: %d-%d", filter.StartTimestamp, filter.EndTimestamp)
	}
	if !ValidSensitiveWordEventDisposition(filter.Disposition) {
		return nil, 0, fmt.Errorf("invalid sensitive-word event disposition: %s", filter.Disposition)
	}
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 || limit > 100 {
		limit = 100
	}

	query := DB.Model(&SensitiveWordAuditEvent{}).
		Where("created_at >= ? AND created_at <= ?", time.Unix(filter.StartTimestamp, 0), time.Unix(filter.EndTimestamp, 0))
	switch filter.Disposition {
	case "block":
		query = query.Where("blocked = ? OR disposition = ?", true, "block")
	case "whitelist_bypass":
		query = query.Where("whitelist_bypassed = ? OR disposition = ?", true, "whitelist_bypass")
	case "observe":
		query = query.Where("disposition = ? OR (disposition = ? AND observe_only = ? AND blocked = ? AND whitelist_bypassed = ?)", "observe", "", true, false, false)
	}
	keyword := strings.TrimSpace(filter.Keyword)
	if keyword != "" {
		pattern := sensitiveWordEventSearchPattern(keyword)
		condition := "(username_snapshot LIKE ? ESCAPE '!' OR model_name LIKE ? ESCAPE '!' OR group_name LIKE ? ESCAPE '!' OR request_id LIKE ? ESCAPE '!')"
		arguments := []any{pattern, pattern, pattern, pattern}
		if userID, err := strconv.Atoi(keyword); err == nil && userID > 0 {
			condition = "(" + condition + " OR user_id = ?)"
			arguments = append(arguments, userID)
		}
		query = query.Where(condition, arguments...)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var events []sensitiveWordDashboardEvent
	if err := query.
		Select("id", "request_id", "user_id", "username_snapshot", "model_name", "group_name", "matched_rule_names", "disposition", "whitelist_bypassed", "blocked", "observe_only", "auto_banned", "violation_count", "created_at").
		Order("created_at desc, id desc").
		Offset(offset).Limit(limit).
		Find(&events).Error; err != nil {
		return nil, 0, err
	}
	return sensitiveWordDashboardRecentEvents(events), total, nil
}

func RedactSensitiveWordEventMetadata(events []SensitiveWordDashboardRecentEvent) {
	for index := range events {
		events[index].AuditID = 0
		events[index].RuleNames = nil
	}
}

func sensitiveWordDashboardRecentEvents(events []sensitiveWordDashboardEvent) []SensitiveWordDashboardRecentEvent {
	result := make([]SensitiveWordDashboardRecentEvent, 0, len(events))
	for _, event := range events {
		result = append(result, SensitiveWordDashboardRecentEvent{
			AuditID: event.ID, RequestID: event.RequestID,
			UserID: event.UserID, Username: event.UsernameSnapshot,
			ModelName: event.ModelName, GroupName: event.GroupName,
			Disposition:    sensitiveWordDashboardDisposition(event),
			RuleNames:      decodeSensitiveAuditSlice[string](string(event.MatchedRuleNames)),
			ViolationCount: event.ViolationCount, AutomaticBan: event.AutoBanned,
			CreatedAt: event.CreatedAt.Unix(),
		})
	}
	return result
}

func sensitiveWordEventSearchPattern(keyword string) string {
	keyword = strings.ReplaceAll(keyword, "!", "!!")
	keyword = strings.ReplaceAll(keyword, "%", "!%")
	keyword = strings.ReplaceAll(keyword, "_", "!_")
	return "%" + keyword + "%"
}
