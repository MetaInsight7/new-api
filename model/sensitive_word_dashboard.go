package model

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
)

const (
	sensitiveWordDashboardTopLimit    = 5
	sensitiveWordDashboardRecentLimit = 12
)

var sensitiveWordDashboardLocation = time.FixedZone("Asia/Shanghai", 8*60*60)

type SensitiveWordDashboardPolicy struct {
	Enabled                 bool `json:"enabled"`
	CheckPrompt             bool `json:"check_prompt"`
	AutoBanEnabled          bool `json:"auto_ban_enabled"`
	BanThreshold            int  `json:"ban_threshold"`
	FullPromptRetentionDays int  `json:"full_prompt_retention_days"`
	RuleCount               int  `json:"rule_count"`
	BlockingRuleCount       int  `json:"blocking_rule_count"`
	ObserveRuleCount        int  `json:"observe_rule_count"`
	DisabledRuleCount       int  `json:"disabled_rule_count"`
}

type SensitiveWordDashboardSummary struct {
	MatchedRequests   int64 `json:"matched_requests"`
	BlockedRequests   int64 `json:"blocked_requests"`
	ObservedRequests  int64 `json:"observed_requests"`
	WhitelistBypasses int64 `json:"whitelist_bypasses"`
	AffectedUsers     int   `json:"affected_users"`
	AutomaticBans     int64 `json:"automatic_bans"`
}

type SensitiveWordDashboardSeriesPoint struct {
	Timestamp         int64 `json:"timestamp"`
	MatchedRequests   int64 `json:"matched_requests"`
	BlockedRequests   int64 `json:"blocked_requests"`
	ObservedRequests  int64 `json:"observed_requests"`
	WhitelistBypasses int64 `json:"whitelist_bypasses"`
	AutomaticBans     int64 `json:"automatic_bans"`
}

type SensitiveWordDashboardTopRule struct {
	RuleID       int64  `json:"rule_id"`
	RuleName     string `json:"rule_name"`
	MatchCount   int64  `json:"match_count"`
	BlockedCount int64  `json:"blocked_count"`
}

type SensitiveWordDashboardTopUser struct {
	UserID        int    `json:"user_id"`
	Username      string `json:"username"`
	MatchCount    int64  `json:"match_count"`
	BlockedCount  int64  `json:"blocked_count"`
	AutomaticBans int64  `json:"automatic_bans"`
	LastSeenAt    int64  `json:"last_seen_at"`
}

type SensitiveWordDashboardDimension struct {
	Name  string `json:"name"`
	Count int64  `json:"count"`
}

type SensitiveWordDashboardRecentEvent struct {
	AuditID        int64    `json:"audit_id,omitempty"`
	RequestID      string   `json:"request_id"`
	UserID         int      `json:"user_id"`
	Username       string   `json:"username"`
	ModelName      string   `json:"model_name"`
	GroupName      string   `json:"group_name"`
	Disposition    string   `json:"disposition"`
	RuleNames      []string `json:"rule_names,omitempty"`
	ViolationCount int      `json:"violation_count"`
	AutomaticBan   bool     `json:"automatic_ban"`
	CreatedAt      int64    `json:"created_at"`
}

type SensitiveWordDashboard struct {
	RequestedHours            int                                 `json:"requested_hours"`
	SensitiveMetadataIncluded bool                                `json:"sensitive_metadata_included"`
	BucketSeconds             int64                               `json:"bucket_seconds"`
	WindowStart               int64                               `json:"window_start"`
	GeneratedAt               int64                               `json:"generated_at"`
	Timezone                  string                              `json:"timezone"`
	Policy                    SensitiveWordDashboardPolicy        `json:"policy"`
	Summary                   SensitiveWordDashboardSummary       `json:"summary"`
	Series                    []SensitiveWordDashboardSeriesPoint `json:"series"`
	TopRules                  []SensitiveWordDashboardTopRule     `json:"top_rules"`
	TopUsers                  []SensitiveWordDashboardTopUser     `json:"top_users"`
	TopModels                 []SensitiveWordDashboardDimension   `json:"top_models"`
	TopGroups                 []SensitiveWordDashboardDimension   `json:"top_groups"`
	RecentEvents              []SensitiveWordDashboardRecentEvent `json:"recent_events"`
}

type sensitiveWordDashboardEvent struct {
	ID                int64
	RequestID         string
	UserID            int
	UsernameSnapshot  string
	ModelName         string
	GroupName         string
	MatchedRuleIDs    SensitiveWordAuditJSON
	MatchedRuleNames  SensitiveWordAuditJSON
	Disposition       string
	WhitelistBypassed bool
	Blocked           bool
	ObserveOnly       bool
	AutoBanned        bool
	ViolationCount    int
	CreatedAt         time.Time
}

func ValidSensitiveWordDashboardHours(hours int) bool {
	return hours == 24 || hours == 168 || hours == 720
}

func GetSensitiveWordDashboard(hours int) (*SensitiveWordDashboard, error) {
	if !ValidSensitiveWordDashboardHours(hours) {
		return nil, fmt.Errorf("invalid sensitive-word dashboard window: %d", hours)
	}
	policy, err := GetSensitiveWordPolicyWithError()
	if err != nil {
		return nil, err
	}
	now := time.Now().In(sensitiveWordDashboardLocation)
	windowStart, bucketDuration, bucketCount := sensitiveWordDashboardWindow(hours, now)
	dashboard := &SensitiveWordDashboard{
		RequestedHours:            hours,
		SensitiveMetadataIncluded: true,
		BucketSeconds:             int64(bucketDuration / time.Second),
		WindowStart:               windowStart.Unix(),
		GeneratedAt:               now.Unix(),
		Timezone:                  sensitiveWordDashboardLocation.String(),
		Policy: SensitiveWordDashboardPolicy{
			Enabled:                 policy.Enabled,
			CheckPrompt:             policy.CheckPrompt,
			AutoBanEnabled:          policy.AutoBanEnabled,
			BanThreshold:            policy.BanThreshold,
			FullPromptRetentionDays: policy.FullPromptRetentionDays,
		},
		Series: make([]SensitiveWordDashboardSeriesPoint, bucketCount),
	}
	for index := range bucketCount {
		dashboard.Series[index].Timestamp = windowStart.Add(time.Duration(index) * bucketDuration).Unix()
	}
	if err := populateSensitiveWordDashboardRuleCounts(&dashboard.Policy); err != nil {
		return nil, err
	}

	users := make(map[int]*SensitiveWordDashboardTopUser)
	rules := make(map[string]*SensitiveWordDashboardTopRule)
	models := make(map[string]int64)
	groups := make(map[string]int64)
	rows, err := DB.Model(&SensitiveWordAuditEvent{}).
		Select("id", "request_id", "user_id", "username_snapshot", "model_name", "group_name", "matched_rule_ids", "matched_rule_names", "disposition", "whitelist_bypassed", "blocked", "observe_only", "auto_banned", "violation_count", "created_at").
		Where("created_at >= ? AND created_at <= ?", windowStart, now).
		Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var event sensitiveWordDashboardEvent
		if err := DB.ScanRows(rows, &event); err != nil {
			return nil, err
		}
		bucketIndex := int(event.CreatedAt.Sub(windowStart) / bucketDuration)
		if bucketIndex < 0 || bucketIndex >= len(dashboard.Series) {
			continue
		}
		disposition := sensitiveWordDashboardDisposition(event)
		point := &dashboard.Series[bucketIndex]
		point.MatchedRequests++
		dashboard.Summary.MatchedRequests++
		switch disposition {
		case "block":
			point.BlockedRequests++
			dashboard.Summary.BlockedRequests++
		case "whitelist_bypass":
			point.WhitelistBypasses++
			dashboard.Summary.WhitelistBypasses++
		default:
			point.ObservedRequests++
			dashboard.Summary.ObservedRequests++
		}
		if event.AutoBanned {
			point.AutomaticBans++
			dashboard.Summary.AutomaticBans++
		}

		if event.UserID > 0 {
			user := users[event.UserID]
			if user == nil {
				user = &SensitiveWordDashboardTopUser{UserID: event.UserID}
				users[event.UserID] = user
			}
			user.MatchCount++
			if disposition == "block" {
				user.BlockedCount++
			}
			if event.AutoBanned {
				user.AutomaticBans++
			}
			if event.CreatedAt.Unix() >= user.LastSeenAt {
				user.LastSeenAt = event.CreatedAt.Unix()
				user.Username = event.UsernameSnapshot
			}
		}
		if modelName := strings.TrimSpace(event.ModelName); modelName != "" {
			models[modelName]++
		}
		if groupName := strings.TrimSpace(event.GroupName); groupName != "" {
			groups[groupName]++
		}
		accumulateSensitiveWordDashboardRules(rules, event, disposition == "block")
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	dashboard.Summary.AffectedUsers = len(users)
	dashboard.TopUsers = topSensitiveWordDashboardUsers(users)
	dashboard.TopRules = topSensitiveWordDashboardRules(rules)
	dashboard.TopModels = topSensitiveWordDashboardDimensions(models)
	dashboard.TopGroups = topSensitiveWordDashboardDimensions(groups)
	dashboard.RecentEvents, err = getSensitiveWordDashboardRecentEvents(windowStart, now)
	if err != nil {
		return nil, err
	}
	return dashboard, nil
}

func (dashboard *SensitiveWordDashboard) RedactSensitiveMetadata() {
	if dashboard == nil {
		return
	}
	dashboard.SensitiveMetadataIncluded = false
	dashboard.TopRules = []SensitiveWordDashboardTopRule{}
	for index := range dashboard.RecentEvents {
		dashboard.RecentEvents[index].AuditID = 0
		dashboard.RecentEvents[index].RuleNames = nil
	}
}

func sensitiveWordDashboardWindow(hours int, now time.Time) (time.Time, time.Duration, int) {
	if hours == 24 {
		start := now.Truncate(time.Hour).Add(-23 * time.Hour)
		return start, time.Hour, 24
	}
	days := hours / 24
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, sensitiveWordDashboardLocation)
	return today.AddDate(0, 0, -(days - 1)), 24 * time.Hour, days
}

func populateSensitiveWordDashboardRuleCounts(policy *SensitiveWordDashboardPolicy) error {
	var modes []string
	if err := DB.Model(&SensitiveWordRule{}).Pluck("mode", &modes).Error; err != nil {
		return err
	}
	policy.RuleCount = len(modes)
	for _, mode := range modes {
		switch mode {
		case SensitiveWordModeBlock:
			policy.BlockingRuleCount++
		case SensitiveWordModeObserve:
			policy.ObserveRuleCount++
		default:
			policy.DisabledRuleCount++
		}
	}
	return nil
}

func sensitiveWordDashboardDisposition(event sensitiveWordDashboardEvent) string {
	if event.WhitelistBypassed || event.Disposition == "whitelist_bypass" {
		return "whitelist_bypass"
	}
	if event.Blocked || event.Disposition == "block" {
		return "block"
	}
	return "observe"
}

func accumulateSensitiveWordDashboardRules(rules map[string]*SensitiveWordDashboardTopRule, event sensitiveWordDashboardEvent, blocked bool) {
	ids := decodeSensitiveAuditSlice[int64](string(event.MatchedRuleIDs))
	names := decodeSensitiveAuditSlice[string](string(event.MatchedRuleNames))
	seen := make(map[string]struct{}, max(len(ids), len(names)))
	for index := range max(len(ids), len(names)) {
		var id int64
		var name string
		if index < len(ids) {
			id = ids[index]
		}
		if index < len(names) {
			name = strings.TrimSpace(names[index])
		}
		if id == 0 && name == "" {
			continue
		}
		key := strconv.FormatInt(id, 10)
		if id == 0 {
			key = "name:" + name
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		rule := rules[key]
		if rule == nil {
			rule = &SensitiveWordDashboardTopRule{RuleID: id, RuleName: name}
			rules[key] = rule
		}
		rule.MatchCount++
		if blocked {
			rule.BlockedCount++
		}
	}
}

func topSensitiveWordDashboardRules(values map[string]*SensitiveWordDashboardTopRule) []SensitiveWordDashboardTopRule {
	result := make([]SensitiveWordDashboardTopRule, 0, len(values))
	for _, value := range values {
		result = append(result, *value)
	}
	slices.SortFunc(result, func(left, right SensitiveWordDashboardTopRule) int {
		if order := cmp.Compare(right.MatchCount, left.MatchCount); order != 0 {
			return order
		}
		return cmp.Compare(left.RuleName, right.RuleName)
	})
	return result[:min(len(result), sensitiveWordDashboardTopLimit)]
}

func topSensitiveWordDashboardUsers(values map[int]*SensitiveWordDashboardTopUser) []SensitiveWordDashboardTopUser {
	result := make([]SensitiveWordDashboardTopUser, 0, len(values))
	for _, value := range values {
		result = append(result, *value)
	}
	slices.SortFunc(result, func(left, right SensitiveWordDashboardTopUser) int {
		if order := cmp.Compare(right.MatchCount, left.MatchCount); order != 0 {
			return order
		}
		return cmp.Compare(right.LastSeenAt, left.LastSeenAt)
	})
	return result[:min(len(result), sensitiveWordDashboardTopLimit)]
}

func topSensitiveWordDashboardDimensions(values map[string]int64) []SensitiveWordDashboardDimension {
	result := make([]SensitiveWordDashboardDimension, 0, len(values))
	for name, count := range values {
		result = append(result, SensitiveWordDashboardDimension{Name: name, Count: count})
	}
	slices.SortFunc(result, func(left, right SensitiveWordDashboardDimension) int {
		if order := cmp.Compare(right.Count, left.Count); order != 0 {
			return order
		}
		return cmp.Compare(left.Name, right.Name)
	})
	return result[:min(len(result), sensitiveWordDashboardTopLimit)]
}

func getSensitiveWordDashboardRecentEvents(start, end time.Time) ([]SensitiveWordDashboardRecentEvent, error) {
	var events []sensitiveWordDashboardEvent
	if err := DB.Model(&SensitiveWordAuditEvent{}).
		Select("id", "request_id", "user_id", "username_snapshot", "model_name", "group_name", "matched_rule_names", "disposition", "whitelist_bypassed", "blocked", "observe_only", "auto_banned", "violation_count", "created_at").
		Where("created_at >= ? AND created_at <= ?", start, end).
		Order("created_at desc, id desc").
		Limit(sensitiveWordDashboardRecentLimit).
		Find(&events).Error; err != nil {
		return nil, err
	}
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
	return result, nil
}
