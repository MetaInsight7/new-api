package model

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	relaytypes "github.com/QuantumNous/new-api/relaykit/types"
	"gorm.io/gorm"
)

type sensitiveRuntimeRule struct {
	ID      int64
	Name    string
	Scope   string
	Mode    string
	Version int64
	Groups  map[string]struct{}
}

type sensitiveRuntimeSnapshot struct {
	machine           *sensitiveWordAutomaton
	words             map[string][]sensitiveRuntimeRule
	policy            SensitiveWordPolicy
	policyVersion     int64
	migrationComplete bool
}

var sensitiveRuntime struct {
	sync.RWMutex
	database *gorm.DB
	snapshot *sensitiveRuntimeSnapshot
}

var sensitiveRuntimeRefreshMu sync.Mutex
var sensitiveRuntimeSyncOnce sync.Once

// The migration can intentionally leave legacy data unresolved. Keep request
// handling disabled until the one-way migration has completed successfully.
var sensitiveWordRuntimeUnavailable atomic.Bool

var sensitiveSecretPattern = regexp.MustCompile(`(?i)(sk-[a-z0-9_-]{12,}|bearer\s+[a-z0-9._-]{12,}|https?://[^\s]+)`)

func invalidateSensitiveWordRuntime() {
	sensitiveRuntime.Lock()
	sensitiveRuntime.snapshot = nil
	sensitiveRuntime.Unlock()
}

func setSensitiveWordRuntimeUnavailable(unavailable bool) {
	sensitiveWordRuntimeUnavailable.Store(unavailable)
	sensitiveRuntime.Lock()
	sensitiveRuntime.snapshot = nil
	if unavailable {
		sensitiveRuntime.database = nil
	} else {
		sensitiveRuntime.database = DB
	}
	sensitiveRuntime.Unlock()
}

func normalizeSensitivePrompt(value string) string {
	value = strings.ReplaceAll(value, "\x00", "")
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	return strings.TrimSpace(value)
}

func redactedSensitivePreview(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	text = sensitiveSecretPattern.ReplaceAllString(text, "[已脱敏]")
	runes := []rune(text)
	if len(runes) > 96 {
		runes = runes[:96]
	}
	return string(runes)
}

func buildSensitiveRuntimeSnapshot(db *gorm.DB, policy SensitiveWordPolicy) (*sensitiveRuntimeSnapshot, error) {
	if db == nil {
		return nil, ErrSensitiveWordPolicyUnavailable
	}
	var rules []SensitiveWordRule
	if err := db.Preload("Groups").Preload("Words").
		Where("mode IN ?", []string{SensitiveWordModeBlock, SensitiveWordModeObserve}).
		Order("id asc").Find(&rules).Error; err != nil {
		return nil, err
	}

	words := make([]string, 0)
	metadata := make(map[string][]sensitiveRuntimeRule)
	seenWords := make(map[string]struct{})
	for _, rule := range rules {
		groups := make(map[string]struct{}, len(rule.Groups))
		for _, group := range rule.Groups {
			groups[group.GroupName] = struct{}{}
		}
		for _, word := range ruleWords(rule) {
			normalized := strings.ToLower(word)
			if _, exists := seenWords[normalized]; !exists {
				seenWords[normalized] = struct{}{}
				words = append(words, normalized)
			}
			metadata[normalized] = append(metadata[normalized], sensitiveRuntimeRule{
				ID: rule.ID, Name: rule.Name, Scope: rule.Scope,
				Mode: normalizeSensitiveWordRuleMode(rule.Mode), Version: rule.Version,
				Groups: groups,
			})
		}
	}
	snapshot := &sensitiveRuntimeSnapshot{
		words: metadata, policy: policy, policyVersion: policy.Version, migrationComplete: true,
	}
	if len(words) == 0 {
		return snapshot, nil
	}
	snapshot.machine = newSensitiveWordAutomaton(words)
	return snapshot, nil
}

func loadSensitiveRuntimeSnapshot(db *gorm.DB) (*sensitiveRuntimeSnapshot, error) {
	if db == nil {
		return nil, ErrSensitiveWordPolicyUnavailable
	}
	var loaded *sensitiveRuntimeSnapshot
	err := db.Transaction(func(tx *gorm.DB) error {
		var marker Option
		markerResult := tx.Select("key", "value").Where(&Option{Key: sensitiveWordMigrationKey}).Limit(1).Find(&marker)
		if markerResult.Error != nil {
			return fmt.Errorf("read migration marker: %w", markerResult.Error)
		}
		if markerResult.RowsAffected == 0 || marker.Value != sensitiveWordMigrationValue {
			policy := defaultSensitiveWordPolicy()
			policy.Enabled = false
			loaded = &sensitiveRuntimeSnapshot{policy: policy, policyVersion: policy.Version}
			return nil
		}
		var stored SensitiveWordPolicy
		if err := tx.First(&stored, 1).Error; err != nil {
			return fmt.Errorf("read policy: %w", err)
		}
		policy := normalizeSensitiveWordPolicy(stored)
		snapshot, err := buildSensitiveRuntimeSnapshot(tx, policy)
		if err != nil {
			return fmt.Errorf("build rule snapshot: %w", err)
		}
		loaded = snapshot
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSensitiveWordPolicyUnavailable, err)
	}
	return loaded, nil
}

func refreshSensitiveWordRuntime() error {
	sensitiveRuntimeRefreshMu.Lock()
	defer sensitiveRuntimeRefreshMu.Unlock()

	database := DB
	loaded, err := loadSensitiveRuntimeSnapshot(database)
	if err != nil {
		sensitiveRuntime.Lock()
		if sensitiveRuntime.database != database {
			sensitiveRuntime.database = database
			sensitiveRuntime.snapshot = nil
		}
		sensitiveRuntime.Unlock()
		return err
	}
	sensitiveRuntime.Lock()
	sensitiveRuntime.database = database
	sensitiveRuntime.snapshot = loaded
	sensitiveRuntime.Unlock()
	return nil
}

func currentSensitiveRuntimeSnapshot() (*sensitiveRuntimeSnapshot, error) {
	if sensitiveWordRuntimeUnavailable.Load() || DB == nil {
		policy := defaultSensitiveWordPolicy()
		policy.Enabled = false
		return &sensitiveRuntimeSnapshot{policy: policy, policyVersion: policy.Version}, nil
	}
	sensitiveRuntime.RLock()
	database := sensitiveRuntime.database
	snapshot := sensitiveRuntime.snapshot
	sensitiveRuntime.RUnlock()
	if database != DB {
		policy := defaultSensitiveWordPolicy()
		policy.Enabled = false
		return &sensitiveRuntimeSnapshot{policy: policy, policyVersion: policy.Version}, nil
	}
	if snapshot != nil {
		return snapshot, nil
	}
	if err := refreshSensitiveWordRuntime(); err != nil {
		return nil, err
	}
	sensitiveRuntime.RLock()
	snapshot = sensitiveRuntime.snapshot
	sensitiveRuntime.RUnlock()
	if snapshot == nil {
		return nil, ErrSensitiveWordPolicyUnavailable
	}
	return snapshot, nil
}

func sensitiveWordRuntimeGeneration() (bool, int64, error) {
	if DB == nil {
		return false, 0, ErrSensitiveWordPolicyUnavailable
	}
	var marker Option
	markerResult := DB.Select("key", "value").Where(&Option{Key: sensitiveWordMigrationKey}).Limit(1).Find(&marker)
	if markerResult.Error != nil {
		return false, 0, markerResult.Error
	}
	if markerResult.RowsAffected == 0 || marker.Value != sensitiveWordMigrationValue {
		return false, 0, nil
	}
	var policy SensitiveWordPolicy
	if err := DB.Select("version").First(&policy, 1).Error; err != nil {
		return true, 0, err
	}
	return true, policy.Version, nil
}

func syncSensitiveWordRuntimeGeneration() error {
	if sensitiveWordRuntimeUnavailable.Load() {
		return nil
	}
	complete, generation, err := sensitiveWordRuntimeGeneration()
	if err != nil {
		return fmt.Errorf("%w: check generation: %v", ErrSensitiveWordPolicyUnavailable, err)
	}
	sensitiveRuntime.RLock()
	snapshot := sensitiveRuntime.snapshot
	sensitiveRuntime.RUnlock()
	if snapshot == nil || snapshot.migrationComplete != complete || (complete && snapshot.policyVersion != generation) {
		if snapshot != nil && snapshot.migrationComplete && !complete {
			return fmt.Errorf("%w: migration marker disappeared", ErrSensitiveWordPolicyUnavailable)
		}
		return refreshSensitiveWordRuntime()
	}
	return nil
}

func handleSensitiveWordRuntimeNotification() {
	if err := refreshSensitiveWordRuntime(); err != nil {
		common.SysError(common.LogText("failed to refresh sensitive-word runtime; retaining last valid snapshot: %s", err.Error()))
	}
}

func sensitiveWordRuntimeChanged() {
	handleSensitiveWordRuntimeNotification()
	if err := common.PublishConfigChanged(context.Background(), common.ConfigScopeSensitiveWords); err != nil {
		common.SysError(common.LogText("failed to publish sensitive-word runtime change: %s", err.Error()))
	}
}

func StartSensitiveWordRuntimeSync(ctx context.Context, frequency int) {
	if ctx == nil {
		ctx = context.Background()
	}
	sensitiveRuntimeSyncOnce.Do(func() {
		handleSensitiveWordRuntimeNotification()
		if common.RedisEnabled && common.RDB != nil {
			go common.SubscribeConfigChanged(ctx, func(scope string) {
				if scope == common.ConfigScopeSensitiveWords {
					handleSensitiveWordRuntimeNotification()
				}
			})
		}
		if frequency <= 0 {
			return
		}
		go func() {
			ticker := time.NewTicker(time.Duration(frequency) * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					if err := syncSensitiveWordRuntimeGeneration(); err != nil {
						common.SysError(common.LogText("failed to sync sensitive-word runtime generation; retaining last valid snapshot: %s", err.Error()))
					}
				}
			}
		}()
	})
}

func normalizeSensitiveGroups(groups []string, fallback string) []string {
	seen := make(map[string]struct{}, len(groups)+1)
	result := make([]string, 0, len(groups)+1)
	for _, group := range append(groups, fallback) {
		group = strings.TrimSpace(group)
		if group == "" {
			continue
		}
		if _, exists := seen[group]; exists {
			continue
		}
		seen[group] = struct{}{}
		result = append(result, group)
	}
	return result
}

type sensitiveMatchDecision struct {
	Matches          []SensitiveWordMatch
	MatchedGroup     string
	Blocking         bool
	MatchesTruncated bool
}

func matchSensitiveFieldsForGroups(snapshot *sensitiveRuntimeSnapshot, fields []relaytypes.SensitiveTextField, groups []string, fallbackGroup string) (sensitiveMatchDecision, error) {
	decision := sensitiveMatchDecision{}
	candidates := normalizeSensitiveGroups(groups, fallbackGroup)
	if len(candidates) == 0 {
		candidates = []string{""}
	}
	if snapshot == nil {
		return decision, ErrSensitiveWordPolicyUnavailable
	}
	if snapshot == nil || snapshot.machine == nil {
		return decision, nil
	}

	candidateIndex := make(map[string]int, len(candidates))
	for index, group := range candidates {
		candidateIndex[group] = index
	}
	latestUserRounds := make(map[int]int)
	for _, field := range fields {
		if field.Role != relaytypes.SensitiveTextRoleUser {
			continue
		}
		latestRound, exists := latestUserRounds[field.Conversation]
		if !exists || field.Round > latestRound {
			latestUserRounds[field.Conversation] = field.Round
		}
	}
	seenMatches := make(map[string]struct{})
	matchedGroup := ""
	matchedGroupIndex := len(candidates)

scanFields:
	for _, field := range fields {
		latestRound, hasUserRound := latestUserRounds[field.Conversation]
		if !hasUserRound || field.Role != relaytypes.SensitiveTextRoleUser || field.Round != latestRound {
			continue
		}
		if field.Kind == relaytypes.SensitiveTextKindTurnBoundary {
			continue
		}
		text := normalizeSensitivePrompt(field.Text)
		if text == "" {
			continue
		}
		originalRunes := []rune(text)
		snapshot.machine.scan(originalRunes, func(word string, start, end int) bool {
			for _, metadata := range snapshot.words[word] {
				mode := normalizeSensitiveWordRuleMode(metadata.Mode)
				if metadata.Scope == SensitiveWordScopeGroup {
					matchedIndex := len(candidates)
					for group := range metadata.Groups {
						if index, exists := candidateIndex[group]; exists && index < matchedIndex {
							matchedIndex = index
						}
					}
					if matchedIndex == len(candidates) {
						continue
					}
					if matchedIndex < matchedGroupIndex {
						matchedGroupIndex = matchedIndex
						matchedGroup = candidates[matchedIndex]
					}
				}
				blockingMatch := mode == SensitiveWordModeBlock
				key := fmt.Sprintf("%d:%s", metadata.ID, word)
				if _, exists := seenMatches[key]; exists {
					continue
				}
				seenMatches[key] = struct{}{}
				end = min(len(originalRunes), end)
				start = min(max(start, 0), end)
				from := max(0, start-24)
				to := min(len(originalRunes), end+24)
				match := SensitiveWordMatch{
					RuleID: metadata.ID, RuleName: metadata.Name,
					RuleScope: metadata.Scope, RuleMode: mode, RuleVersion: metadata.Version,
					Word: word, FieldOrder: field.Order, FieldRole: field.Role,
					FieldKind: field.Kind, FieldConversation: field.Conversation, FieldRound: field.Round,
					StartRune: start, EndRune: end,
					Snippet: redactedSensitivePreview(string(originalRunes[from:to])),
				}
				if len(decision.Matches) >= SensitiveWordMaxMatches {
					decision.MatchesTruncated = true
					if blockingMatch {
						decision.Matches[len(decision.Matches)-1] = match
						decision.Blocking = true
						return false
					}
					continue
				}
				decision.Matches = append(decision.Matches, match)
				if blockingMatch {
					decision.Blocking = true
					return false
				}
			}
			return true
		})
		if decision.Blocking {
			break scanFields
		}
	}
	if matchedGroup == "" && len(candidates) > 0 {
		matchedGroup = candidates[0]
	}
	decision.MatchedGroup = matchedGroup
	return decision, nil
}
