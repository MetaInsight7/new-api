package model

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	relaytypes "github.com/QuantumNous/new-api/relaykit/types"
	"gorm.io/gorm"
)

func expandSensitiveWordMessage(message string, threshold int) string {
	message = strings.TrimSpace(message)
	if message == "" {
		message = sensitiveWordBlockMessage
	}
	return strings.ReplaceAll(message, "{{threshold}}", fmt.Sprintf("%d", threshold))
}

func decodeSensitiveAuditSlice[T any](value string) []T {
	var result []T
	if value != "" {
		_ = common.UnmarshalJsonStr(value, &result)
	}
	return result
}

func sensitiveCheckResultFromAudit(event SensitiveWordAuditEvent) *SensitiveCheckResult {
	policy := GetSensitiveWordPolicy()
	var evidence SensitiveWordAuditEvidence
	if event.Evidence != "" {
		_ = common.UnmarshalJsonStr(string(event.Evidence), &evidence)
	}
	modes := make([]string, 0, len(evidence.Matches))
	seenModes := make(map[string]struct{}, len(evidence.Matches))
	for _, match := range evidence.Matches {
		if _, exists := seenModes[match.RuleMode]; exists {
			continue
		}
		seenModes[match.RuleMode] = struct{}{}
		modes = append(modes, match.RuleMode)
	}
	return &SensitiveCheckResult{
		Matched: true, Blocked: event.Blocked, Counted: event.Counted,
		Disposition:       event.Disposition,
		WhitelistBypassed: event.WhitelistBypassed,
		AutoBanned:        event.AutoBanned, ViolationCount: event.ViolationCount,
		MatchedWords:     decodeSensitiveAuditSlice[string](string(event.MatchedWords)),
		MatchedRuleIDs:   decodeSensitiveAuditSlice[int64](string(event.MatchedRuleIDs)),
		MatchedRuleNames: decodeSensitiveAuditSlice[string](string(event.MatchedRuleNames)),
		MatchedScope:     event.MatchedScope, AuditID: event.ID,
		Message:          expandSensitiveWordMessage(policy.BlockMessage, policy.BanThreshold),
		UserStatusBefore: event.UserStatusBefore, UserStatusAfter: event.UserStatusAfter,
		QuotaBefore: event.QuotaBefore, QuotaAfter: event.QuotaAfter,
		ObserveOnly: event.ObserveOnly, MatchedRuleModes: modes,
		Matches: evidence.Matches, EvidenceTruncated: event.EvidenceTruncated,
		PolicyGeneration: event.PolicyGeneration,
	}
}

func normalizeSensitiveCheckFields(input SensitiveCheckInput) []relaytypes.SensitiveTextField {
	fields := input.Fields
	if len(fields) == 0 && strings.TrimSpace(input.Prompt) != "" {
		fields = []relaytypes.SensitiveTextField{{
			Role: relaytypes.SensitiveTextRoleUser, Kind: relaytypes.SensitiveTextKindPrompt,
			Round: 0, Order: 0, Text: input.Prompt,
		}}
	}
	result := make([]relaytypes.SensitiveTextField, 0, len(fields))
	for _, field := range fields {
		field.Text = normalizeSensitivePrompt(field.Text)
		if field.Text == "" {
			continue
		}
		result = append(result, field)
	}
	return result
}

func CheckSensitiveRequest(input SensitiveCheckInput) (*SensitiveCheckResult, error) {
	return CheckSensitiveRequestForGroups(input, []string{input.GroupName})
}

func CheckSensitiveRequestForGroups(input SensitiveCheckInput, candidateGroups []string) (*SensitiveCheckResult, error) {
	if strings.TrimSpace(input.RequestID) == "" {
		input.RequestID = common.NewRequestId()
	}
	snapshot, err := currentSensitiveRuntimeSnapshot()
	if err != nil {
		return nil, err
	}
	policy := snapshot.policy
	fields := normalizeSensitiveCheckFields(input)
	if !policy.Enabled || !policy.CheckPrompt || len(fields) == 0 {
		return &SensitiveCheckResult{}, nil
	}
	decision, err := matchSensitiveFieldsForGroups(snapshot, fields, candidateGroups, input.GroupName)
	if err != nil {
		return nil, err
	}
	if len(decision.Matches) == 0 {
		return &SensitiveCheckResult{}, nil
	}
	if decision.MatchedGroup != "" {
		input.GroupName = decision.MatchedGroup
	}

	ids := make([]int64, 0, len(decision.Matches))
	names := make([]string, 0, len(decision.Matches))
	words := make([]string, 0, len(decision.Matches))
	modes := make([]string, 0, 2)
	seenIDs := make(map[int64]struct{}, len(decision.Matches))
	seenNames := make(map[string]struct{}, len(decision.Matches))
	seenWords := make(map[string]struct{}, len(decision.Matches))
	seenModes := make(map[string]struct{}, 2)
	scopes := make(map[string]bool, 2)
	for _, match := range decision.Matches {
		if _, exists := seenIDs[match.RuleID]; !exists {
			seenIDs[match.RuleID] = struct{}{}
			ids = append(ids, match.RuleID)
		}
		if match.RuleName != "" {
			if _, exists := seenNames[match.RuleName]; !exists {
				seenNames[match.RuleName] = struct{}{}
				names = append(names, match.RuleName)
			}
		}
		wordKey := strings.ToLower(match.Word)
		if _, exists := seenWords[wordKey]; !exists {
			seenWords[wordKey] = struct{}{}
			words = append(words, match.Word)
		}
		if _, exists := seenModes[match.RuleMode]; !exists {
			seenModes[match.RuleMode] = struct{}{}
			modes = append(modes, match.RuleMode)
		}
		scopes[match.RuleScope] = true
	}
	matchedScope := SensitiveWordScopeGlobal
	if !scopes[SensitiveWordScopeGlobal] {
		matchedScope = SensitiveWordScopeGroup
	} else if scopes[SensitiveWordScopeGroup] {
		matchedScope = "global+group"
	}
	observeOnly := !decision.Blocking
	result := &SensitiveCheckResult{
		Matched: true, ObserveOnly: observeOnly,
		MatchedWords: words, MatchedRuleIDs: ids, MatchedRuleNames: names,
		MatchedScope: matchedScope, Message: expandSensitiveWordMessage(policy.BlockMessage, policy.BanThreshold),
		MatchedRuleModes: modes, Matches: decision.Matches,
		EvidenceTruncated: decision.MatchesTruncated, PolicyGeneration: policy.Version,
	}

	evidence, preview, snippets, evidenceTruncated, promptHash, evidenceErr := buildSensitiveAuditEvidence(fields, decision.Matches, policy, decision.MatchesTruncated)
	if evidenceErr != nil {
		result.Blocked = decision.Blocking
		return result, fmt.Errorf("%w: %v", ErrSensitiveWordAuditPersistence, evidenceErr)
	}
	result.EvidenceTruncated = evidenceTruncated
	idsRaw, err := marshalSensitiveAuditJSON(ids)
	if err != nil {
		result.Blocked = decision.Blocking
		return result, err
	}
	namesRaw, err := marshalSensitiveAuditJSON(names)
	if err != nil {
		result.Blocked = decision.Blocking
		return result, err
	}
	wordsRaw, err := marshalSensitiveAuditJSON(words)
	if err != nil {
		result.Blocked = decision.Blocking
		return result, err
	}
	snippetsRaw, err := marshalSensitiveAuditJSON(snippets)
	if err != nil {
		result.Blocked = decision.Blocking
		return result, err
	}

	var event SensitiveWordAuditEvent
	var existingEvent *SensitiveWordAuditEvent
	authStateChanged := false
	err = DB.Transaction(func(tx *gorm.DB) error {
		var user User
		if err := lockForUpdate(tx).First(&user, input.UserID).Error; err != nil {
			return err
		}
		var prior SensitiveWordAuditEvent
		if err := tx.Where("request_id = ? AND user_id = ?", input.RequestID, input.UserID).
			Order("id asc").First(&prior).Error; err == nil {
			existingEvent = &prior
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		result.UserStatusBefore = user.Status
		result.UserStatusAfter = user.Status
		result.QuotaBefore = user.Quota
		result.QuotaAfter = user.Quota
		result.WhitelistBypassed = user.SensitiveWordWhitelist
		result.Blocked = decision.Blocking && !user.SensitiveWordWhitelist
		result.Counted = result.Blocked
		result.Disposition = "observe"
		if user.SensitiveWordWhitelist {
			result.Disposition = "whitelist_bypass"
		} else if result.Blocked {
			result.Disposition = "block"
		}
		if result.Counted {
			result.ViolationCount = user.SensitiveWordViolationCount + 1
			updates := map[string]any{"sensitive_word_violation_count": result.ViolationCount}
			result.AutoBanned = policy.AutoBanEnabled && result.ViolationCount >= policy.BanThreshold &&
				user.Role != common.RoleRootUser && user.Status != common.UserStatusDisabled
			if result.AutoBanned {
				nextVersion, versionErr := IncrementUserAuthVersionWithTx(tx, input.UserID)
				if versionErr != nil {
					return versionErr
				}
				updates["status"] = common.UserStatusDisabled
				updates["auth_version"] = nextVersion
				result.UserStatusAfter = common.UserStatusDisabled
				authStateChanged = true
			}
			if err := tx.Model(&User{}).Where("id = ?", input.UserID).Updates(updates).Error; err != nil {
				return err
			}
		} else {
			result.ViolationCount = user.SensitiveWordViolationCount
		}

		event = SensitiveWordAuditEvent{
			RequestID: input.RequestID, UserID: input.UserID, UserRole: user.Role,
			UsernameSnapshot: input.Username, TokenID: input.TokenID,
			TokenNameSnapshot: input.TokenName, GroupName: input.GroupName,
			ModelName: input.ModelName, Endpoint: input.Endpoint, Protocol: input.Protocol,
			PromptHash: promptHash, RedactedPreview: preview, Evidence: evidence,
			EvidenceTruncated: evidenceTruncated,
			MatchedRuleIDs:    idsRaw, MatchedRuleNames: namesRaw,
			MatchedWords: wordsRaw, MatchedSnippets: snippetsRaw,
			MatchedScope: matchedScope, Disposition: result.Disposition,
			WhitelistBypassed: user.SensitiveWordWhitelist, Blocked: result.Blocked, Counted: result.Counted,
			ViolationCount: result.ViolationCount, AutoBanned: result.AutoBanned, ObserveOnly: observeOnly,
			UserStatusBefore: result.UserStatusBefore, UserStatusAfter: result.UserStatusAfter,
			QuotaBefore: result.QuotaBefore, QuotaAfter: result.QuotaAfter,
			RuleVersion: policy.Version, PolicyGeneration: policy.Version, CreatedAt: time.Now(),
		}
		if err := tx.Create(&event).Error; err != nil {
			return fmt.Errorf("%w: %v", ErrSensitiveWordAuditPersistence, err)
		}
		return nil
	})
	if err != nil {
		var prior SensitiveWordAuditEvent
		if lookupErr := DB.Where("request_id = ? AND user_id = ?", input.RequestID, input.UserID).
			Order("id asc").First(&prior).Error; lookupErr == nil {
			return sensitiveCheckResultFromAudit(prior), nil
		}
		result.AutoBanned = false
		result.Counted = false
		result.ViolationCount = 0
		result.AuditID = 0
		result.Blocked = decision.Blocking
		return result, err
	}
	if existingEvent != nil {
		return sensitiveCheckResultFromAudit(*existingEvent), nil
	}

	result.AuditID = event.ID
	if authStateChanged {
		refreshSensitiveWordAuthState(input.UserID, "sensitive_word_threshold")
	}
	writeSensitiveWordUsageLog(input, event, result)
	return result, nil
}

func marshalSensitiveAuditJSON(value any) (SensitiveWordAuditJSON, error) {
	raw, err := common.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrSensitiveWordAuditPersistence, err)
	}
	return SensitiveWordAuditJSON(raw), nil
}

func sensitiveFieldsHash(fields []relaytypes.SensitiveTextField) (string, error) {
	raw, err := common.Marshal(fields)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(raw)
	return hex.EncodeToString(hash[:]), nil
}

func buildSensitiveAuditEvidence(fields []relaytypes.SensitiveTextField, matches []SensitiveWordMatch, policy SensitiveWordPolicy, alreadyTruncated bool) (SensitiveWordAuditJSON, string, []string, bool, string, error) {
	promptHash, err := sensitiveFieldsHash(fields)
	if err != nil {
		return "", "", nil, alreadyTruncated, "", err
	}
	snippets := make([]string, 0, len(matches))
	seenSnippets := make(map[string]struct{}, len(matches))
	for _, match := range matches {
		if match.Snippet == "" {
			continue
		}
		if _, exists := seenSnippets[match.Snippet]; exists {
			continue
		}
		seenSnippets[match.Snippet] = struct{}{}
		snippets = append(snippets, match.Snippet)
	}
	preview := ""
	if len(snippets) > 0 {
		preview = snippets[0]
	}
	if !policy.RetainFullPrompt {
		return "", preview, snippets, alreadyTruncated, promptHash, nil
	}

	evidence := SensitiveWordAuditEvidence{Version: 1, Matches: append([]SensitiveWordMatch(nil), matches...), Truncated: alreadyTruncated}
	for {
		raw, marshalErr := common.Marshal(evidence)
		if marshalErr != nil {
			return "", "", nil, evidence.Truncated, "", marshalErr
		}
		if len(raw) <= SensitiveWordMaxPromptBytes {
			break
		}
		if len(evidence.Matches) == 0 {
			return "", "", nil, true, "", errors.New("sensitive-word evidence metadata exceeds limit")
		}
		evidence.Matches = evidence.Matches[:len(evidence.Matches)-1]
		evidence.Truncated = true
	}

	matchedOrders := make(map[int]struct{}, len(matches))
	matchedRounds := make(map[int]struct{}, len(matches))
	for _, match := range matches {
		matchedOrders[match.FieldOrder] = struct{}{}
		matchedRounds[match.FieldRound] = struct{}{}
	}
	selected := make([]relaytypes.SensitiveTextField, 0, len(fields))
	seenOrders := make(map[int]struct{}, len(fields))
	appendField := func(field relaytypes.SensitiveTextField) {
		if _, exists := seenOrders[field.Order]; exists {
			return
		}
		seenOrders[field.Order] = struct{}{}
		selected = append(selected, field)
	}
	for _, field := range fields {
		if _, matched := matchedOrders[field.Order]; matched {
			appendField(field)
		}
	}
	for _, field := range fields {
		if field.Role == relaytypes.SensitiveTextRoleSystem || field.Role == relaytypes.SensitiveTextRoleDeveloper {
			appendField(field)
			continue
		}
		for currentRound := range matchedRounds {
			if currentRound >= 0 && field.Round >= max(0, currentRound-3) && field.Round <= currentRound {
				appendField(field)
				break
			}
		}
	}

	for _, field := range selected {
		candidate := field
		if policy.MaxPromptRunes > 0 && len([]rune(candidate.Text)) > policy.MaxPromptRunes {
			candidate.Text = string([]rune(candidate.Text)[:policy.MaxPromptRunes]) + "…"
			evidence.Truncated = true
		}
		evidence.Fields = append(evidence.Fields, candidate)
		raw, marshalErr := common.Marshal(evidence)
		if marshalErr != nil {
			return "", "", nil, evidence.Truncated, "", marshalErr
		}
		if len(raw) <= SensitiveWordMaxPromptBytes {
			continue
		}
		evidence.Fields = evidence.Fields[:len(evidence.Fields)-1]
		evidence.Truncated = true
		break
	}
	sort.SliceStable(evidence.Fields, func(left, right int) bool {
		return evidence.Fields[left].Order < evidence.Fields[right].Order
	})
	raw, err := common.Marshal(evidence)
	if err != nil {
		return "", "", nil, evidence.Truncated, "", err
	}
	return SensitiveWordAuditJSON(raw), preview, snippets, evidence.Truncated, promptHash, nil
}

func refreshSensitiveWordAuthState(userID int, reason string) int64 {
	if err := PublishUserAuthCache(userID); err != nil {
		common.SysLog(fmt.Sprintf("failed to publish sensitive-word auth cache for user %d: %s", userID, err.Error()))
	}
	revoked, err := RevokeAllUserSessions(userID, reason)
	if err != nil {
		common.SysLog(fmt.Sprintf("failed to revoke sensitive-word sessions for user %d: %s", userID, err.Error()))
	}
	if err := InvalidateUserTokensCache(userID); err != nil {
		common.SysLog(fmt.Sprintf("failed to invalidate sensitive-word token cache for user %d: %s", userID, err.Error()))
	}
	return revoked
}

func writeSensitiveWordUsageLog(input SensitiveCheckInput, event SensitiveWordAuditEvent, result *SensitiveCheckResult) {
	action := "blocked"
	if result.WhitelistBypassed {
		action = "whitelist_bypass"
	} else if result.ObserveOnly {
		action = "observe"
	}
	other := NewLogOther()
	other.SetPublic("action", SensitiveWordLogAction)
	other.SetAdmin("keyword_filter", map[string]any{
		"audit_id": event.ID, "action": action,
		"blocked": result.Blocked, "counted": result.Counted,
		"auto_banned": result.AutoBanned, "balance_changed": false,
	})
	log := &Log{
		UserId: input.UserID, Username: input.Username, CreatedAt: common.GetTimestamp(),
		Type: LogTypeSensitiveWordBlock, Content: "Sensitive word audit", TokenId: input.TokenID,
		TokenName: input.TokenName, ModelName: input.ModelName, Group: input.GroupName,
		RequestId: input.RequestID, Other: other.JSONString(),
	}
	if err := createLog(log); err != nil {
		common.SysLog("failed to record sensitive-word usage log: " + err.Error())
	}
}

// CleanupSensitiveWordAudits clears full-prompt evidence after the configured
// retention period while preserving the audit event and its enforcement facts.
func CleanupSensitiveWordAudits() (int64, error) {
	policy, err := GetSensitiveWordPolicyWithError()
	if err != nil {
		return 0, err
	}
	if policy.FullPromptRetentionDays <= 0 {
		return 0, nil
	}
	cutoff := time.Now().AddDate(0, 0, -policy.FullPromptRetentionDays)
	var ids []int64
	if err := DB.Model(&SensitiveWordAuditEvent{}).
		Where("created_at < ? AND (evidence <> '' OR full_prompt <> '' OR redacted_preview <> '' OR matched_snippets <> ?)", cutoff, "[]").
		Order("id asc").Limit(SensitiveWordAuditCleanupBatchSize).Pluck("id", &ids).Error; err != nil {
		return 0, err
	}
	if len(ids) == 0 {
		return 0, nil
	}
	result := DB.Model(&SensitiveWordAuditEvent{}).Where("id IN ?", ids).
		Updates(map[string]any{"evidence": "", "full_prompt": "", "redacted_preview": "", "matched_snippets": "[]"})
	return result.RowsAffected, result.Error
}

func truncateSensitivePrompt(value string, maxRunes int) string {
	value = strings.ReplaceAll(value, "\x00", "")
	runes := []rune(strings.TrimSpace(value))
	if maxRunes <= 0 {
		maxRunes = SensitiveWordMaxPromptRunes
	}
	truncated := len(runes) > maxRunes
	if truncated {
		runes = runes[:maxRunes]
	}
	value = string(runes)
	marker := "…"
	byteLimit := SensitiveWordMaxPromptBytes - len(marker)
	if len(value) > byteLimit {
		value = trimSensitivePromptBytes(value, byteLimit)
		truncated = true
	}
	if truncated {
		return value + marker
	}
	return value
}

func trimSensitivePromptBytes(value string, maxBytes int) string {
	if maxBytes <= 0 {
		return ""
	}
	if len(value) <= maxBytes {
		return value
	}
	value = value[:maxBytes]
	for len(value) > 0 && !utf8.ValidString(value) {
		value = value[:len(value)-1]
	}
	return value
}

// EnableUserAndResetSensitiveWordViolations is the only unban path owned by
// this feature. It never touches quota, usage, subscriptions, wallets, or
// historical audit rows.
func EnableUserAndResetSensitiveWordViolations(userID int) (*SensitiveWordEnableResetResult, error) {
	if userID <= 0 {
		return nil, common.NewMessage("Invalid user ID")
	}
	result := &SensitiveWordEnableResetResult{UserID: userID}
	err := DB.Transaction(func(tx *gorm.DB) error {
		var current User
		if err := lockForUpdate(tx).First(&current, userID).Error; err != nil {
			return err
		}
		result.StatusBefore = current.Status
		result.StatusAfter = common.UserStatusEnabled
		result.ViolationCountBefore = current.SensitiveWordViolationCount
		result.ViolationCountAfter = 0
		result.AuthVersionBefore = current.AuthVersion
		result.AuthVersionAfter = current.AuthVersion
		result.QuotaBefore = current.Quota
		result.QuotaAfter = current.Quota
		result.UsedQuotaBefore = current.UsedQuota
		result.UsedQuotaAfter = current.UsedQuota
		result.StatusChanged = current.Status != common.UserStatusEnabled
		result.ViolationCountReset = current.SensitiveWordViolationCount != 0

		updates := map[string]any{
			"status": common.UserStatusEnabled, "sensitive_word_violation_count": 0,
		}
		if result.StatusChanged {
			nextVersion, err := IncrementUserAuthVersionWithTx(tx, userID)
			if err != nil {
				return err
			}
			result.AuthVersionAfter = nextVersion
			updates["auth_version"] = nextVersion
		}
		if result.StatusChanged || result.ViolationCountReset {
			return tx.Model(&User{}).Where("id = ?", userID).Updates(updates).Error
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if result.StatusChanged {
		// The state transition is already committed. Attempt every cache and
		// session invalidation so a transient transport failure cannot prevent
		// later invalidation work or misreport the persisted outcome to the
		// administrator.
		result.SessionsRevoked = refreshSensitiveWordAuthState(userID, "sensitive_word_enable")
	}
	return result, nil
}

// UpdateSensitiveWordUserFields applies only fields explicitly supplied by an
// administrator. Keeping this separate from User.EditWithTx prevents ordinary
// profile edits from writing Go zero values over the safety state.
func UpdateSensitiveWordUserFields(userID int, violationCount *int, whitelist *bool) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		return UpdateSensitiveWordUserFieldsWithTx(tx, userID, violationCount, whitelist)
	})
}

// UpdateSensitiveWordUserFieldsWithTx keeps an administrative user edit and
// safety-state edit atomic when the caller already owns a transaction.
func UpdateSensitiveWordUserFieldsWithTx(tx *gorm.DB, userID int, violationCount *int, whitelist *bool) error {
	if violationCount == nil && whitelist == nil {
		return nil
	}
	if violationCount != nil && *violationCount < 0 {
		return common.NewMessage("Sensitive-word violation count cannot be negative")
	}
	var current User
	if err := lockForUpdate(tx).First(&current, userID).Error; err != nil {
		return err
	}
	updates := make(map[string]any, 2)
	if violationCount != nil {
		updates["sensitive_word_violation_count"] = *violationCount
	}
	if whitelist != nil {
		updates["sensitive_word_whitelist"] = *whitelist
	}
	return tx.Model(&current).Updates(updates).Error
}
