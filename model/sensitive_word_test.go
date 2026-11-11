package model

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	relaytypes "github.com/QuantumNous/new-api/relaykit/types"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// legacySensitiveWordV1* mirror the first P-30 tables, before rule names,
// modes, rule-word rows, user columns, and the policy singleton existed.
type legacySensitiveWordV1Rule struct {
	ID        int64     `gorm:"primaryKey"`
	Word      string    `gorm:"type:varchar(200);not null;index"`
	Scope     string    `gorm:"type:varchar(16);not null;index"`
	Enabled   bool      `gorm:"not null;default:true;index"`
	CreatedBy int       `gorm:"index"`
	Version   int64     `gorm:"not null;default:1"`
	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

func (legacySensitiveWordV1Rule) TableName() string { return "sensitive_word_rules" }

type legacySensitiveWordV1Audit struct {
	ID              int64     `gorm:"primaryKey"`
	RequestID       string    `gorm:"type:varchar(128);index"`
	UserID          int       `gorm:"index"`
	FullPrompt      string    `gorm:"type:text"`
	MatchedRuleIDs  string    `gorm:"type:text"`
	MatchedWords    string    `gorm:"type:text"`
	MatchedSnippets string    `gorm:"type:text"`
	CreatedAt       time.Time `gorm:"index"`
}

func (legacySensitiveWordV1Audit) TableName() string {
	return "sensitive_word_audit_events"
}

func setupSensitiveWordTest(t *testing.T) {
	t.Helper()
	require.NoError(t, DB.AutoMigrate(
		&Option{}, &User{}, &UserSession{}, &Log{},
		&SensitiveWordRule{}, &SensitiveWordRuleWord{}, &SensitiveWordRuleGroup{},
		&SensitiveWordPolicy{}, &SensitiveWordAuditEvent{},
	))
	require.NoError(t, DB.Exec("DELETE FROM sensitive_word_audit_events").Error)
	require.NoError(t, DB.Exec("DELETE FROM sensitive_word_rule_words").Error)
	require.NoError(t, DB.Exec("DELETE FROM sensitive_word_rule_groups").Error)
	require.NoError(t, DB.Exec("DELETE FROM sensitive_word_rules").Error)
	require.NoError(t, DB.Exec("DELETE FROM sensitive_word_policy").Error)
	require.NoError(t, DB.Where("key IN ?", []string{
		"SensitiveWords", "SensitiveWordConfig", sensitiveWordMigrationKey,
	}).Delete(&Option{}).Error)
	require.NoError(t, LOG_DB.Where("type = ?", LogTypeSensitiveWordBlock).Delete(&Log{}).Error)
	invalidateSensitiveWordRuntime()

	t.Cleanup(func() {
		_ = DB.Exec("DELETE FROM sensitive_word_audit_events").Error
		_ = DB.Exec("DELETE FROM sensitive_word_rule_words").Error
		_ = DB.Exec("DELETE FROM sensitive_word_rule_groups").Error
		_ = DB.Exec("DELETE FROM sensitive_word_rules").Error
		_ = DB.Exec("DELETE FROM sensitive_word_policy").Error
		_ = DB.Where("key IN ?", []string{
			"SensitiveWords", "SensitiveWordConfig", sensitiveWordMigrationKey,
		}).Delete(&Option{}).Error
		_ = LOG_DB.Where("type = ?", LogTypeSensitiveWordBlock).Delete(&Log{}).Error
		invalidateSensitiveWordRuntime()
	})
}

func saveSensitiveWordTestPolicy(t *testing.T, retain bool, threshold int) {
	t.Helper()
	require.NoError(t, SaveSensitiveWordPolicy(SensitiveWordPolicy{
		Enabled: true, CheckPrompt: true, RetainFullPrompt: retain,
		BlockMessage: sensitiveWordBlockMessage, BanThreshold: threshold,
		FullPromptRetentionDays: 180, MaxPromptRunes: SensitiveWordMaxPromptRunes,
	}, 1))
}

func createSensitiveWordTestUser(t *testing.T, quota int, whitelisted bool) *User {
	t.Helper()
	id := time.Now().UnixNano()
	user := &User{
		Username: fmt.Sprintf("sensitive-test-%d", id), Password: "unused-password",
		Status: common.UserStatusEnabled, Role: common.RoleCommonUser,
		Group: "default", Quota: quota, AffCode: fmt.Sprintf("sw-%d", id),
		AuthVersion: 1, SensitiveWordWhitelist: whitelisted,
	}
	require.NoError(t, DB.Create(user).Error)
	t.Cleanup(func() {
		_ = DB.Where("user_id = ?", user.Id).Delete(&UserSession{}).Error
		_ = DB.Delete(&User{}, user.Id).Error
	})
	return user
}

func addSensitiveWordTestRule(t *testing.T, name string, words []string, scope string, groups []string, mode string) *SensitiveWordRuleDetail {
	t.Helper()
	detail, err := UpsertSensitiveWordRuleWithMode(0, name, words, scope, groups, 1, mode)
	require.NoError(t, err)
	return detail
}

func sensitiveWordTestInput(user *User, requestID, prompt string) SensitiveCheckInput {
	return SensitiveCheckInput{
		RequestID: requestID, UserID: user.Id, Username: user.Username,
		TokenID: 99, TokenName: "sensitive-test-key", GroupName: "default",
		ModelName: "gpt-test", Endpoint: "/v1/chat/completions",
		Protocol: "openai", Prompt: prompt,
	}
}

func sensitiveWordTestAudit(t *testing.T, requestID string) SensitiveWordAuditEvent {
	t.Helper()
	var event SensitiveWordAuditEvent
	require.NoError(t, DB.Where("request_id = ?", requestID).First(&event).Error)
	return event
}

func TestSensitiveWordValidationAndDefaults(t *testing.T) {
	setupSensitiveWordTest(t)
	require.NoError(t, MigrateSensitiveWordData())
	policy := GetSensitiveWordPolicy()
	require.True(t, policy.Enabled)
	require.True(t, policy.CheckPrompt)
	require.False(t, policy.AutoBanEnabled)
	require.True(t, policy.RetainFullPrompt)
	require.Equal(t, 50, policy.BanThreshold)
	require.Equal(t, 180, policy.FullPromptRetentionDays)
	require.Equal(t, 65_536, policy.MaxPromptRunes)
	require.Equal(t, sensitiveWordBlockMessage, policy.BlockMessage)
	require.NotContains(t, policy.BlockMessage, "{{threshold}}")

	legacyPolicy := policy
	legacyPolicy.BlockMessage = legacySensitiveWordBlockMessage
	require.Equal(t, sensitiveWordBlockMessage, normalizeSensitiveWordPolicy(legacyPolicy).BlockMessage)
	customPolicy := policy
	customPolicy.BlockMessage = "custom policy message"
	require.Equal(t, customPolicy.BlockMessage, normalizeSensitiveWordPolicy(customPolicy).BlockMessage)

	words, err := normalizeSensitiveWords([]string{" Alpha ", "alpha", "Beta"})
	require.NoError(t, err)
	require.Equal(t, []string{"Alpha", "Beta"}, words)
	_, err = normalizeSensitiveWords([]string{strings.Repeat("x", SensitiveWordMaxRunes+1)})
	require.Error(t, err)
	_, err = UpsertSensitiveWordRuleWithMode(0, "empty", nil, SensitiveWordScopeGlobal, nil, 1, SensitiveWordModeBlock)
	require.Error(t, err)
	_, err = UpsertSensitiveWordRuleWithMode(0, "bad", []string{"word"}, "bad-scope", nil, 1, SensitiveWordModeBlock)
	require.Error(t, err)
	_, err = UpsertSensitiveWordRuleWithMode(0, "bad", []string{"word"}, SensitiveWordScopeGroup, []string{"auto"}, 1, SensitiveWordModeBlock)
	require.Error(t, err)
	_, err = UpsertSensitiveWordRuleWithMode(0, "bad", []string{"word"}, SensitiveWordScopeGlobal, nil, 1, "invalid")
	require.Error(t, err)

	detail := addSensitiveWordTestRule(t, "default observe", []string{"word"}, SensitiveWordScopeGlobal, nil, SensitiveWordModeObserve)
	require.Equal(t, SensitiveWordModeObserve, detail.Mode)
	require.Equal(t, 1, detail.WordCount)
}

func TestSensitiveWordCustomBlockMessageRemainsVerbatim(t *testing.T) {
	setupSensitiveWordTest(t)
	require.NoError(t, MigrateSensitiveWordData())
	policy := GetSensitiveWordPolicy()
	policy.BlockMessage = "  custom {{threshold}} policy message，清空余额  "
	require.NoError(t, SaveSensitiveWordPolicy(policy, 1))
	addSensitiveWordTestRule(t, "custom message", []string{"custom-message-marker"}, SensitiveWordScopeGlobal, nil, SensitiveWordModeBlock)
	user := createSensitiveWordTestUser(t, 1_000_000, false)

	result, err := CheckSensitiveRequest(sensitiveWordTestInput(user, "custom-message", "custom-message-marker"))
	require.NoError(t, err)
	require.True(t, result.Blocked)
	require.False(t, result.UsesDefaultMessage)
	require.Equal(t, "  custom {{threshold}} policy message，清空余额  ", result.Message)
}

func TestSensitiveWordPolicyVersionIsServerOwned(t *testing.T) {
	setupSensitiveWordTest(t)
	require.NoError(t, MigrateSensitiveWordData())
	before := GetSensitiveWordPolicy()
	updated := before
	updated.Version = 9_999_999
	updated.BanThreshold = before.BanThreshold + 1

	require.NoError(t, SaveSensitiveWordPolicy(updated, 42))
	after := GetSensitiveWordPolicy()
	require.Equal(t, before.Version+1, after.Version)
	require.Equal(t, before.BanThreshold+1, after.BanThreshold)
}

func TestSensitiveWordPolicyRowSelfHealsOnColdStartAndRuleMutation(t *testing.T) {
	setupSensitiveWordTest(t)
	require.NoError(t, MigrateSensitiveWordData())
	require.NoError(t, DB.Save(&Option{Key: "SensitiveWordConfig", Value: `{invalid`}).Error)
	require.NoError(t, DB.Delete(&SensitiveWordPolicy{}, 1).Error)
	invalidateSensitiveWordRuntime()

	require.NoError(t, MigrateSensitiveWordData())
	policy := GetSensitiveWordPolicy()
	require.Equal(t, int64(1), policy.Version)
	require.True(t, policy.Enabled)
	var legacy Option
	require.NoError(t, DB.Where(&Option{Key: "SensitiveWordConfig"}).First(&legacy).Error)
	require.Equal(t, `{invalid`, legacy.Value)

	require.NoError(t, DB.Delete(&SensitiveWordPolicy{}, 1).Error)
	_, err := UpsertSensitiveWordRuleWithMode(
		0, "policy repair", []string{"policy repair marker"},
		SensitiveWordScopeGlobal, nil, 7, SensitiveWordModeObserve,
	)
	require.NoError(t, err)
	require.NoError(t, DB.First(&policy, 1).Error)
	require.Equal(t, int64(1), policy.Version)
}

func TestSensitiveWordPolicyReadFailureIsNotSilentlyDisabled(t *testing.T) {
	previousDB, previousLogDB := DB, LOG_DB
	previousMain, previousLog := common.MainDatabaseType(), common.LogDatabaseType()
	previousUnavailable := sensitiveWordRuntimeUnavailable.Load()
	t.Cleanup(func() {
		DB, LOG_DB = previousDB, previousLogDB
		common.SetDatabaseTypes(previousMain, previousLog)
		setSensitiveWordRuntimeUnavailable(previousUnavailable)
	})

	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/closed-policy.db"), &gorm.Config{})
	require.NoError(t, err)
	DB, LOG_DB = db, db
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	setSensitiveWordRuntimeUnavailable(false)
	require.NoError(t, db.AutoMigrate(&Option{}, &SensitiveWordPolicy{}))
	require.NoError(t, db.Create(&Option{Key: sensitiveWordMigrationKey, Value: sensitiveWordMigrationValue}).Error)
	policy := defaultSensitiveWordPolicy()
	require.NoError(t, db.Create(&policy).Error)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	_, err = GetSensitiveWordPolicyWithError()
	require.ErrorIs(t, err, ErrSensitiveWordPolicyUnavailable)
}

func TestSensitiveWordRuleModePriorityGroupsAndHotReload(t *testing.T) {
	setupSensitiveWordTest(t)
	require.NoError(t, MigrateSensitiveWordData())
	saveSensitiveWordTestPolicy(t, true, SensitiveWordBanThreshold)
	user := createSensitiveWordTestUser(t, 7_310_000, false)

	result, err := CheckSensitiveRequest(sensitiveWordTestInput(user, "before-rule", "hot marker"))
	require.NoError(t, err)
	require.False(t, result.Matched)
	observe := addSensitiveWordTestRule(t, "observe", []string{"hot marker"}, SensitiveWordScopeGlobal, nil, SensitiveWordModeObserve)
	groupBlock := addSensitiveWordTestRule(t, "vip block", []string{"hot marker"}, SensitiveWordScopeGroup, []string{"vip"}, SensitiveWordModeBlock)

	result, err = CheckSensitiveRequestForGroups(
		sensitiveWordTestInput(user, "observe-default", "HOT MARKER"), []string{"default"},
	)
	require.NoError(t, err)
	require.True(t, result.Matched)
	require.True(t, result.ObserveOnly)
	require.False(t, result.Blocked)
	require.Equal(t, 0, result.ViolationCount)
	require.Equal(t, []int64{observe.ID}, result.MatchedRuleIDs)

	result, err = CheckSensitiveRequestForGroups(
		sensitiveWordTestInput(user, "block-vip", "hot marker"), []string{"default", "vip", "vip"},
	)
	require.NoError(t, err)
	require.True(t, result.Blocked, "block must win when block and observe rules both match")
	require.False(t, result.ObserveOnly)
	require.Equal(t, 1, result.ViolationCount, "one request must increment only once")
	require.ElementsMatch(t, []int64{observe.ID, groupBlock.ID}, result.MatchedRuleIDs)
	require.Equal(t, []string{"hot marker"}, result.MatchedWords)
	require.Len(t, result.Matches, 2)
	require.Equal(t, "vip", sensitiveWordTestAudit(t, "block-vip").GroupName)

	require.NoError(t, SetSensitiveWordRuleMode(groupBlock.ID, SensitiveWordModeOff, 1))
	result, err = CheckSensitiveRequestForGroups(
		sensitiveWordTestInput(user, "after-mode-hot-reload", "hot marker"), []string{"vip"},
	)
	require.NoError(t, err)
	require.True(t, result.ObserveOnly)
	require.False(t, result.Blocked)
	require.Equal(t, 1, result.ViolationCount)
	require.NotContains(t, ListSensitiveWordGroups(), "auto")
}

func TestSensitiveWordCollectsObserveMatchesUntilFirstBlock(t *testing.T) {
	setupSensitiveWordTest(t)
	require.NoError(t, MigrateSensitiveWordData())
	saveSensitiveWordTestPolicy(t, true, SensitiveWordBanThreshold)
	user := createSensitiveWordTestUser(t, 7_315_000, false)
	blockRule := addSensitiveWordTestRule(
		t, "block words", []string{"block marker", "later block marker"},
		SensitiveWordScopeGlobal, nil, SensitiveWordModeBlock,
	)
	observeRule := addSensitiveWordTestRule(
		t, "observe words", []string{"first observe marker", "second observe marker", "later observe marker"},
		SensitiveWordScopeGlobal, nil, SensitiveWordModeObserve,
	)

	result, err := CheckSensitiveRequest(sensitiveWordTestInput(
		user, "stop-at-first-block", "first observe marker, second observe marker, block marker, later observe marker, later block marker",
	))
	require.NoError(t, err)
	require.True(t, result.Blocked)
	require.ElementsMatch(t, []int64{blockRule.ID, observeRule.ID}, result.MatchedRuleIDs)
	require.Equal(t, []string{"first observe marker", "second observe marker", "block marker"}, result.MatchedWords)
	require.Len(t, result.Matches, 3)
	require.Equal(t, 1, result.ViolationCount)

	event := sensitiveWordTestAudit(t, "stop-at-first-block")
	var storedWords []string
	require.NoError(t, common.Unmarshal([]byte(event.MatchedWords), &storedWords))
	require.ElementsMatch(t, result.MatchedWords, storedWords)
	var evidence SensitiveWordAuditEvidence
	require.NoError(t, common.Unmarshal([]byte(event.Evidence), &evidence))
	require.Len(t, evidence.Matches, 3)
}

func TestSensitiveWordChecksLatestInputRound(t *testing.T) {
	setupSensitiveWordTest(t)
	require.NoError(t, MigrateSensitiveWordData())
	saveSensitiveWordTestPolicy(t, true, SensitiveWordBanThreshold)
	user := createSensitiveWordTestUser(t, 7_316_000, false)
	observeRule := addSensitiveWordTestRule(
		t, "observe latest user", []string{"observe marker"},
		SensitiveWordScopeGlobal, nil, SensitiveWordModeObserve,
	)
	blockRule := addSensitiveWordTestRule(
		t, "block latest user", []string{"block marker"},
		SensitiveWordScopeGlobal, nil, SensitiveWordModeBlock,
	)

	input := sensitiveWordTestInput(user, "latest-user-semantics", "")
	input.Fields = []relaytypes.SensitiveTextField{
		{Role: relaytypes.SensitiveTextRoleSystem, Kind: relaytypes.SensitiveTextKindContent, Round: -1, Order: 0, Text: "block marker in system"},
		{Role: relaytypes.SensitiveTextRoleDeveloper, Kind: relaytypes.SensitiveTextKindInstruction, Round: -1, Order: 1, Text: "block marker from developer"},
		{Role: relaytypes.SensitiveTextRoleUser, Kind: relaytypes.SensitiveTextKindContent, Round: 0, Order: 2, Text: "observe marker and block marker in history"},
		{Role: relaytypes.SensitiveTextRoleAssistant, Kind: relaytypes.SensitiveTextKindContent, Round: 0, Order: 3, Text: "observe marker and block marker from assistant"},
		{Role: relaytypes.SensitiveTextRoleTool, Kind: relaytypes.SensitiveTextKindToolResult, Round: 0, Order: 4, Text: "block marker from tool"},
		{Role: relaytypes.SensitiveTextRoleUser, Kind: relaytypes.SensitiveTextKindContent, Round: 1, Order: 5, Text: "latest clean input"},
		{Role: relaytypes.SensitiveTextRoleUser, Kind: relaytypes.SensitiveTextKindContent, Round: 1, Order: 6, Text: "latest clean second text block"},
		{Role: relaytypes.SensitiveTextRoleAssistant, Kind: relaytypes.SensitiveTextKindToolArgument, Round: 1, Order: 7, Text: "block marker in assistant tool arguments"},
		{Role: relaytypes.SensitiveTextRoleTool, Kind: relaytypes.SensitiveTextKindToolResult, Round: 1, Order: 8, Text: "latest clean tool result"},
	}
	result, err := CheckSensitiveRequest(input)
	require.NoError(t, err)
	require.False(t, result.Matched)

	input.RequestID = "latest-user-observe"
	input.Fields[6].Text = "observe marker in latest input"
	result, err = CheckSensitiveRequest(input)
	require.NoError(t, err)
	require.False(t, result.Blocked)
	require.True(t, result.ObserveOnly)
	require.Equal(t, []int64{observeRule.ID}, result.MatchedRuleIDs)

	input.RequestID = "latest-user-block"
	input.Fields[6].Text = "block marker in latest input"
	result, err = CheckSensitiveRequest(input)
	require.NoError(t, err)
	require.True(t, result.Blocked)
	require.Equal(t, []int64{blockRule.ID}, result.MatchedRuleIDs)

	input.RequestID = "latest-tool-result-block"
	input.Fields[6].Text = "latest clean second text block"
	input.Fields[8].Text = "block marker in latest tool result"
	result, err = CheckSensitiveRequest(input)
	require.NoError(t, err)
	require.True(t, result.Blocked)
	require.Equal(t, relaytypes.SensitiveTextKindToolResult, result.Matches[0].FieldKind)

	input.RequestID = "tool-result-only"
	input.Fields = []relaytypes.SensitiveTextField{{
		Role: relaytypes.SensitiveTextRoleTool, Kind: relaytypes.SensitiveTextKindToolResult,
		Round: 0, Order: 0, Text: "observe marker from a tool-only request",
	}}
	result, err = CheckSensitiveRequest(input)
	require.NoError(t, err)
	require.True(t, result.Matched)
	require.True(t, result.ObserveOnly)
	require.Equal(t, []int64{observeRule.ID}, result.MatchedRuleIDs)
}

func TestSensitiveWordTextFreeLatestUserRoundDoesNotRescanHistory(t *testing.T) {
	setupSensitiveWordTest(t)
	require.NoError(t, MigrateSensitiveWordData())
	saveSensitiveWordTestPolicy(t, true, SensitiveWordBanThreshold)
	user := createSensitiveWordTestUser(t, 7_316_250, false)
	addSensitiveWordTestRule(
		t, "media follow-up", []string{"historical block marker"},
		SensitiveWordScopeGlobal, nil, SensitiveWordModeBlock,
	)

	input := sensitiveWordTestInput(user, "text-free-latest-user", "")
	input.Fields = []relaytypes.SensitiveTextField{
		{Role: relaytypes.SensitiveTextRoleUser, Kind: relaytypes.SensitiveTextKindTurnBoundary, Round: 0, Order: 0},
		{Role: relaytypes.SensitiveTextRoleUser, Kind: relaytypes.SensitiveTextKindContent, Round: 0, Order: 1, Text: "historical block marker"},
		{Role: relaytypes.SensitiveTextRoleAssistant, Kind: relaytypes.SensitiveTextKindContent, Round: 0, Order: 2, Text: "prior response"},
		{Role: relaytypes.SensitiveTextRoleUser, Kind: relaytypes.SensitiveTextKindTurnBoundary, Round: 1, Order: 3},
	}

	result, err := CheckSensitiveRequest(input)
	require.NoError(t, err)
	require.False(t, result.Matched)
}

func TestSensitiveWordChecksLatestUserRoundInEveryConversation(t *testing.T) {
	setupSensitiveWordTest(t)
	require.NoError(t, MigrateSensitiveWordData())
	saveSensitiveWordTestPolicy(t, true, SensitiveWordBanThreshold)
	user := createSensitiveWordTestUser(t, 7_316_500, false)
	addSensitiveWordTestRule(
		t, "batch latest user", []string{"batch block marker"},
		SensitiveWordScopeGlobal, nil, SensitiveWordModeBlock,
	)

	input := sensitiveWordTestInput(user, "batch-latest-user", "")
	input.Fields = []relaytypes.SensitiveTextField{
		{Role: relaytypes.SensitiveTextRoleUser, Kind: relaytypes.SensitiveTextKindContent, Conversation: 1, Round: 0, Order: 0, Text: "batch block marker in history"},
		{Role: relaytypes.SensitiveTextRoleUser, Kind: relaytypes.SensitiveTextKindContent, Conversation: 1, Round: 1, Order: 1, Text: "first conversation latest is clean"},
		{Role: relaytypes.SensitiveTextRoleUser, Kind: relaytypes.SensitiveTextKindContent, Conversation: 2, Round: 0, Order: 2, Text: "batch block marker in second conversation"},
		{Role: relaytypes.SensitiveTextRoleUser, Kind: relaytypes.SensitiveTextKindContent, Conversation: 3, Round: 0, Order: 3, Text: "third conversation latest is clean"},
	}

	result, err := CheckSensitiveRequest(input)
	require.NoError(t, err)
	require.True(t, result.Blocked)
	require.Len(t, result.Matches, 1)
	require.Equal(t, 2, result.Matches[0].FieldConversation)
	require.Equal(t, 2, result.Matches[0].FieldOrder)
}

func TestSensitiveWordRejectedHistoryDoesNotPoisonNextUserTurn(t *testing.T) {
	setupSensitiveWordTest(t)
	require.NoError(t, MigrateSensitiveWordData())
	saveSensitiveWordTestPolicy(t, true, SensitiveWordBanThreshold)
	user := createSensitiveWordTestUser(t, 7_317_000, false)
	addSensitiveWordTestRule(
		t, "blocked input", []string{"block marker"},
		SensitiveWordScopeGlobal, nil, SensitiveWordModeBlock,
	)

	blockedInput := sensitiveWordTestInput(user, "initial-block", "")
	blockedInput.Fields = []relaytypes.SensitiveTextField{{
		Role: relaytypes.SensitiveTextRoleUser, Kind: relaytypes.SensitiveTextKindContent,
		Round: 0, Order: 0, Text: "block marker in rejected input",
	}}
	result, err := CheckSensitiveRequest(blockedInput)
	require.NoError(t, err)
	require.True(t, result.Blocked)
	require.Equal(t, 1, result.ViolationCount)

	nextInput := sensitiveWordTestInput(user, "clean-follow-up", "")
	nextInput.Fields = []relaytypes.SensitiveTextField{
		{
			Role: relaytypes.SensitiveTextRoleUser, Kind: relaytypes.SensitiveTextKindContent,
			Round: 0, Order: 0, Text: "block marker in rejected input",
		},
		{
			Role: relaytypes.SensitiveTextRoleUser, Kind: relaytypes.SensitiveTextKindContent,
			Round: 1, Order: 1, Text: "clean follow-up",
		},
	}
	result, err = CheckSensitiveRequest(nextInput)
	require.NoError(t, err)
	require.False(t, result.Matched)

	var storedUser User
	require.NoError(t, DB.First(&storedUser, user.Id).Error)
	require.Equal(t, 1, storedUser.SensitiveWordViolationCount)

	var auditCount int64
	require.NoError(t, LOG_DB.Model(&SensitiveWordAuditEvent{}).Where("user_id = ?", user.Id).Count(&auditCount).Error)
	require.Equal(t, int64(1), auditCount)
}

func TestSensitiveWordCapsMatchesAndRetainsBlockingEvidence(t *testing.T) {
	setupSensitiveWordTest(t)
	require.NoError(t, MigrateSensitiveWordData())
	saveSensitiveWordTestPolicy(t, true, SensitiveWordBanThreshold)
	user := createSensitiveWordTestUser(t, 7_318_000, false)
	words := make([]string, 0, SensitiveWordMaxMatches+1)
	for index := 0; index <= SensitiveWordMaxMatches; index++ {
		words = append(words, fmt.Sprintf("observe-marker-%03d", index))
	}
	addSensitiveWordTestRule(
		t, "many observe words", words,
		SensitiveWordScopeGlobal, nil, SensitiveWordModeObserve,
	)
	blockRule := addSensitiveWordTestRule(
		t, "late block", []string{"final-block-marker"},
		SensitiveWordScopeGlobal, nil, SensitiveWordModeBlock,
	)

	result, err := CheckSensitiveRequest(sensitiveWordTestInput(
		user, "match-cap", strings.Join(append(words, "final-block-marker"), " "),
	))
	require.NoError(t, err)
	require.True(t, result.Blocked)
	require.True(t, result.EvidenceTruncated)
	require.Len(t, result.Matches, SensitiveWordMaxMatches)
	require.Contains(t, result.MatchedRuleIDs, blockRule.ID)
	require.Condition(t, func() bool {
		for _, match := range result.Matches {
			if match.RuleID == blockRule.ID && match.RuleMode == SensitiveWordModeBlock {
				return true
			}
		}
		return false
	})
}

func TestSensitiveWordSkipsFirstWordOutsideCandidateGroups(t *testing.T) {
	setupSensitiveWordTest(t)
	require.NoError(t, MigrateSensitiveWordData())
	saveSensitiveWordTestPolicy(t, true, SensitiveWordBanThreshold)
	user := createSensitiveWordTestUser(t, 7_317_000, false)
	addSensitiveWordTestRule(
		t, "vip only", []string{"vip marker"},
		SensitiveWordScopeGroup, []string{"vip"}, SensitiveWordModeBlock,
	)
	defaultRule := addSensitiveWordTestRule(
		t, "default block", []string{"default marker"},
		SensitiveWordScopeGlobal, nil, SensitiveWordModeBlock,
	)

	result, err := CheckSensitiveRequestForGroups(
		sensitiveWordTestInput(user, "skip-inapplicable-word", "vip marker then default marker"),
		[]string{"default"},
	)
	require.NoError(t, err)
	require.True(t, result.Blocked)
	require.Equal(t, []int64{defaultRule.ID}, result.MatchedRuleIDs)
	require.Equal(t, []string{"default marker"}, result.MatchedWords)
}

func TestSensitiveWordPolicyVersionRefreshesRemoteRuntime(t *testing.T) {
	setupSensitiveWordTest(t)
	require.NoError(t, MigrateSensitiveWordData())
	saveSensitiveWordTestPolicy(t, false, SensitiveWordBanThreshold)
	user := createSensitiveWordTestUser(t, 7_420_000, false)
	rule := addSensitiveWordTestRule(t, "remote refresh", []string{"remote runtime marker"}, SensitiveWordScopeGlobal, nil, SensitiveWordModeObserve)

	result, err := CheckSensitiveRequest(sensitiveWordTestInput(user, "remote-observe", "remote runtime marker"))
	require.NoError(t, err)
	require.True(t, result.ObserveOnly)
	sensitiveRuntime.RLock()
	before := sensitiveRuntime.snapshot
	sensitiveRuntime.RUnlock()
	require.NotNil(t, before)

	// Simulate a successful rule mutation on another node: it changes durable
	// rows and the policy generation, but cannot clear this process's snapshot.
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&SensitiveWordRule{}).Where("id = ?", rule.ID).Updates(map[string]any{
			"mode": SensitiveWordModeBlock, "version": gorm.Expr("version + ?", 1),
		}).Error; err != nil {
			return err
		}
		return bumpSensitiveWordPolicyVersion(tx, 7)
	}))
	var durablePolicy SensitiveWordPolicy
	require.NoError(t, DB.First(&durablePolicy, 1).Error)
	require.Greater(t, durablePolicy.Version, before.policyVersion)
	require.Equal(t, before.policyVersion, GetSensitiveWordPolicy().Version, "a dropped notification keeps the local immutable snapshot until the periodic check")
	require.NoError(t, syncSensitiveWordRuntimeGeneration())
	require.Equal(t, durablePolicy.Version, GetSensitiveWordPolicy().Version)

	result, err = CheckSensitiveRequest(sensitiveWordTestInput(user, "remote-block", "remote runtime marker"))
	require.NoError(t, err)
	require.True(t, result.Blocked)
	var stored User
	require.NoError(t, DB.First(&stored, user.Id).Error)
	require.Equal(t, 1, stored.SensitiveWordViolationCount)
}

func TestSensitiveWordRuntimeNotificationRefreshesSnapshot(t *testing.T) {
	setupSensitiveWordTest(t)
	require.NoError(t, MigrateSensitiveWordData())
	saveSensitiveWordTestPolicy(t, false, SensitiveWordBanThreshold)
	user := createSensitiveWordTestUser(t, 7_430_000, false)
	rule := addSensitiveWordTestRule(t, "notification refresh", []string{"notification marker"}, SensitiveWordScopeGlobal, nil, SensitiveWordModeObserve)

	before, err := CheckSensitiveRequest(sensitiveWordTestInput(user, "notification-observe", "notification marker"))
	require.NoError(t, err)
	require.True(t, before.ObserveOnly)
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&SensitiveWordRule{}).Where("id = ?", rule.ID).Updates(map[string]any{
			"mode": SensitiveWordModeBlock, "version": gorm.Expr("version + ?", 1),
		}).Error; err != nil {
			return err
		}
		return bumpSensitiveWordPolicyVersion(tx, 8)
	}))

	handleSensitiveWordRuntimeNotification()
	after, err := CheckSensitiveRequest(sensitiveWordTestInput(user, "notification-block", "notification marker"))
	require.NoError(t, err)
	require.True(t, after.Blocked)
}

func TestSensitiveWordRefreshFailureRetainsLastValidSnapshot(t *testing.T) {
	setupSensitiveWordTest(t)
	require.NoError(t, MigrateSensitiveWordData())
	saveSensitiveWordTestPolicy(t, false, SensitiveWordBanThreshold)
	user := createSensitiveWordTestUser(t, 7_440_000, false)
	addSensitiveWordTestRule(t, "retained snapshot", []string{"retained marker"}, SensitiveWordScopeGlobal, nil, SensitiveWordModeObserve)

	before, err := CheckSensitiveRequest(sensitiveWordTestInput(user, "retained-before", "retained marker"))
	require.NoError(t, err)
	require.True(t, before.ObserveOnly)
	require.NoError(t, DB.Migrator().RenameTable(&SensitiveWordPolicy{}, "sensitive_word_policy_unavailable"))
	t.Cleanup(func() {
		if DB.Migrator().HasTable("sensitive_word_policy_unavailable") {
			_ = DB.Migrator().RenameTable("sensitive_word_policy_unavailable", "sensitive_word_policy")
		}
	})

	handleSensitiveWordRuntimeNotification()
	after, err := CheckSensitiveRequest(sensitiveWordTestInput(user, "retained-after", "retained marker"))
	require.NoError(t, err)
	require.True(t, after.ObserveOnly)
	require.Equal(t, before.PolicyGeneration, after.PolicyGeneration)

	require.NoError(t, DB.Migrator().RenameTable("sensitive_word_policy_unavailable", "sensitive_word_policy"))
}

func TestSensitiveWordRuntimeDoesNotCrossDatabaseInstances(t *testing.T) {
	setupSensitiveWordTest(t)
	require.NoError(t, MigrateSensitiveWordData())
	saveSensitiveWordTestPolicy(t, false, SensitiveWordBanThreshold)
	require.True(t, GetSensitiveWordPolicy().Enabled)

	previousDB := DB
	otherDB, err := gorm.Open(sqlite.Open("file:sensitive-runtime-other?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	DB = otherDB
	t.Cleanup(func() {
		DB = previousDB
		sqlDB, dbErr := otherDB.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
	})

	policy, err := GetSensitiveWordPolicyWithError()
	require.NoError(t, err)
	require.False(t, policy.Enabled)
}

func TestSensitiveWordWhitelistAndRequestIdempotency(t *testing.T) {
	setupSensitiveWordTest(t)
	require.NoError(t, MigrateSensitiveWordData())
	saveSensitiveWordTestPolicy(t, true, SensitiveWordBanThreshold)
	addSensitiveWordTestRule(t, "block", []string{"blocked marker"}, SensitiveWordScopeGlobal, nil, SensitiveWordModeBlock)

	whitelisted := createSensitiveWordTestUser(t, 1_900_000, true)
	result, err := CheckSensitiveRequest(sensitiveWordTestInput(whitelisted, "whitelist", "blocked marker"))
	require.NoError(t, err)
	require.True(t, result.WhitelistBypassed)
	require.False(t, result.Blocked)
	require.Zero(t, result.ViolationCount)
	require.NotEmpty(t, sensitiveWordTestAudit(t, "whitelist").Evidence)

	user := createSensitiveWordTestUser(t, 2_300_000, false)
	first, err := CheckSensitiveRequest(sensitiveWordTestInput(user, "same-request", "blocked marker"))
	require.NoError(t, err)
	second, err := CheckSensitiveRequest(sensitiveWordTestInput(user, "same-request", "blocked marker"))
	require.NoError(t, err)
	require.Equal(t, first.AuditID, second.AuditID)
	require.Equal(t, 1, second.ViolationCount)
	var auditCount int64
	require.NoError(t, DB.Model(&SensitiveWordAuditEvent{}).
		Where("request_id = ? AND user_id = ?", "same-request", user.Id).Count(&auditCount).Error)
	require.Equal(t, int64(1), auditCount)
	var stored User
	require.NoError(t, DB.First(&stored, user.Id).Error)
	require.Equal(t, 1, stored.SensitiveWordViolationCount)
}

func TestSensitiveWordUsageLogLinksEvidenceWithoutContentLeak(t *testing.T) {
	setupSensitiveWordTest(t)
	requestID := fmt.Sprintf("sensitive-log-link-%d", time.Now().UnixNano())
	writeSensitiveWordUsageLog(
		SensitiveCheckInput{RequestID: requestID, UserID: 12, Username: "subject", TokenID: 34, TokenName: "key"},
		SensitiveWordAuditEvent{ID: 56},
		&SensitiveCheckResult{
			Blocked: true, Counted: true, AutoBanned: false,
			MatchedWords: []string{"must-not-enter-log"},
		},
	)

	var stored Log
	require.NoError(t, LOG_DB.Where("request_id = ?", requestID).First(&stored).Error)
	adminLog := stored
	FormatAdminLogs([]*Log{&adminLog})
	require.Contains(t, adminLog.Other, `"audit_id":56`)
	require.Contains(t, adminLog.Other, `"action":"blocked"`)
	require.NotContains(t, adminLog.Other, "must-not-enter-log")
	require.NotContains(t, adminLog.Other, "matched_words")

	userLog := stored
	formatUserLogs([]*Log{&userLog}, 12)
	require.JSONEq(t, `{"action":"sensitive_word_block"}`, userLog.Other)
}

func TestSensitiveWordRoleDispositionKeepsRootRecoverable(t *testing.T) {
	setupSensitiveWordTest(t)
	require.NoError(t, MigrateSensitiveWordData())
	saveSensitiveWordTestPolicy(t, true, 1)
	policy := GetSensitiveWordPolicy()
	policy.AutoBanEnabled = true
	require.NoError(t, SaveSensitiveWordPolicy(policy, 1))
	addSensitiveWordTestRule(t, "role disposition", []string{"role marker"}, SensitiveWordScopeGlobal, nil, SensitiveWordModeBlock)

	root := createSensitiveWordTestUser(t, 1_000, false)
	require.NoError(t, DB.Model(root).Update("role", common.RoleRootUser).Error)
	root.Role = common.RoleRootUser
	rootResult, err := CheckSensitiveRequest(sensitiveWordTestInput(root, "root-role", "role marker"))
	require.NoError(t, err)
	require.True(t, rootResult.Blocked)
	require.True(t, rootResult.Counted)
	require.False(t, rootResult.AutoBanned)
	require.Equal(t, 1, rootResult.ViolationCount)
	var storedRoot User
	require.NoError(t, DB.First(&storedRoot, root.Id).Error)
	require.Equal(t, common.UserStatusEnabled, storedRoot.Status)
	require.Equal(t, 1, storedRoot.SensitiveWordViolationCount)

	admin := createSensitiveWordTestUser(t, 1_000, false)
	require.NoError(t, DB.Model(admin).Update("role", common.RoleAdminUser).Error)
	admin.Role = common.RoleAdminUser
	adminResult, err := CheckSensitiveRequest(sensitiveWordTestInput(admin, "admin-role", "role marker"))
	require.NoError(t, err)
	require.True(t, adminResult.Blocked)
	require.True(t, adminResult.Counted)
	require.True(t, adminResult.AutoBanned)
	require.Equal(t, 1, adminResult.ViolationCount)
	var storedAdmin User
	require.NoError(t, DB.First(&storedAdmin, admin.Id).Error)
	require.Equal(t, common.UserStatusDisabled, storedAdmin.Status)
	require.Equal(t, 1, storedAdmin.SensitiveWordViolationCount)
}

func TestSensitiveWordConcurrentSameRequestCountsOnce(t *testing.T) {
	setupSensitiveWordTest(t)
	require.NoError(t, MigrateSensitiveWordData())
	saveSensitiveWordTestPolicy(t, false, SensitiveWordBanThreshold)
	addSensitiveWordTestRule(t, "concurrent", []string{"concurrent marker"}, SensitiveWordScopeGlobal, nil, SensitiveWordModeBlock)
	user := createSensitiveWordTestUser(t, 3_000_000, false)

	const workers = 8
	start := make(chan struct{})
	results := make(chan *SensitiveCheckResult, workers)
	errorsCh := make(chan error, workers)
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			result, err := CheckSensitiveRequest(sensitiveWordTestInput(user, "concurrent-request", "concurrent marker"))
			results <- result
			errorsCh <- err
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	close(errorsCh)
	for err := range errorsCh {
		require.NoError(t, err)
	}
	var auditID int64
	for result := range results {
		require.NotNil(t, result)
		require.Equal(t, 1, result.ViolationCount)
		if auditID == 0 {
			auditID = result.AuditID
		}
		require.Equal(t, auditID, result.AuditID)
	}
	var stored User
	require.NoError(t, DB.First(&stored, user.Id).Error)
	require.Equal(t, 1, stored.SensitiveWordViolationCount)
}

func TestSensitiveWordThresholdBanAndEnableResetPreserveAccounting(t *testing.T) {
	setupSensitiveWordTest(t)
	require.NoError(t, MigrateSensitiveWordData())
	saveSensitiveWordTestPolicy(t, true, 3)
	policy := GetSensitiveWordPolicy()
	policy.AutoBanEnabled = true
	require.NoError(t, SaveSensitiveWordPolicy(policy, 1))
	addSensitiveWordTestRule(t, "threshold", []string{"threshold marker"}, SensitiveWordScopeGlobal, nil, SensitiveWordModeBlock)
	user := createSensitiveWordTestUser(t, 88_800_000, false)
	user.UsedQuota = 123_456
	require.NoError(t, DB.Model(&User{}).Where("id = ?", user.Id).Update("used_quota", user.UsedQuota).Error)

	now := time.Now().Unix()
	session := UserSession{
		SID: fmt.Sprintf("sensitive-session-%d", user.Id), UserID: user.Id,
		Version: 1, UserAuthVersion: 1, Status: UserSessionStatusActive,
		RefreshHash: "sensitive-refresh", LoginMethod: "password",
		CreatedAt: now, LastActiveAt: now, ExpiresAt: now + 3600,
	}
	require.NoError(t, DB.Create(&session).Error)

	for count := 1; count <= 3; count++ {
		result, err := CheckSensitiveRequest(sensitiveWordTestInput(
			user, fmt.Sprintf("threshold-%d", count), "threshold marker",
		))
		require.NoError(t, err)
		require.True(t, result.Blocked)
		require.Equal(t, count, result.ViolationCount)
		require.Equal(t, count == 3, result.AutoBanned)
	}
	var banned User
	require.NoError(t, DB.First(&banned, user.Id).Error)
	require.Equal(t, common.UserStatusDisabled, banned.Status)
	require.Equal(t, 3, banned.SensitiveWordViolationCount)
	require.True(t, banned.SensitiveWordAutoBanned)
	require.Equal(t, 88_800_000, banned.Quota)
	require.Equal(t, 123_456, banned.UsedQuota)
	require.Equal(t, int64(2), banned.AuthVersion)
	var revoked UserSession
	require.NoError(t, DB.First(&revoked, "sid = ?", session.SID).Error)
	require.Equal(t, UserSessionStatusRevoked, revoked.Status)
	require.Equal(t, "sensitive_word_threshold", revoked.RevokedReason)

	thresholdAudit := sensitiveWordTestAudit(t, "threshold-3")
	require.True(t, thresholdAudit.AutoBanned)
	require.Equal(t, 88_800_000, thresholdAudit.QuotaBefore)
	require.Equal(t, 88_800_000, thresholdAudit.QuotaAfter)
	var evidenceBefore int64
	require.NoError(t, DB.Model(&SensitiveWordAuditEvent{}).Where("user_id = ?", user.Id).Count(&evidenceBefore).Error)

	reset, err := EnableUserAndResetSensitiveWordViolations(user.Id)
	require.NoError(t, err)
	require.True(t, reset.StatusChanged)
	require.True(t, reset.ViolationCountReset)
	require.Equal(t, 3, reset.ViolationCountBefore)
	require.Zero(t, reset.ViolationCountAfter)
	require.Equal(t, int64(3), reset.AuthVersionAfter)
	require.Equal(t, 88_800_000, reset.QuotaAfter)
	require.Equal(t, 123_456, reset.UsedQuotaAfter)

	var enabled User
	require.NoError(t, DB.First(&enabled, user.Id).Error)
	require.Equal(t, common.UserStatusEnabled, enabled.Status)
	require.Zero(t, enabled.SensitiveWordViolationCount)
	require.False(t, enabled.SensitiveWordAutoBanned)
	require.Equal(t, banned.Quota, enabled.Quota)
	require.Equal(t, banned.UsedQuota, enabled.UsedQuota)
	var evidenceAfter int64
	require.NoError(t, DB.Model(&SensitiveWordAuditEvent{}).Where("user_id = ?", user.Id).Count(&evidenceAfter).Error)
	require.Equal(t, evidenceBefore, evidenceAfter)

	_, err = EnableUserAndResetSensitiveWordViolations(user.Id)
	require.ErrorContains(t, err, "Only accounts disabled by sensitive-word automatic enforcement can be unbanned here")
}

func TestSensitiveWordDedicatedUnbanRejectsEnabledUserAndCountResetStaysIndependent(t *testing.T) {
	setupSensitiveWordTest(t)
	user := createSensitiveWordTestUser(t, 2_000_000, false)
	require.NoError(t, DB.Model(&User{}).Where("id = ?", user.Id).
		Update("sensitive_word_violation_count", 2).Error)
	now := time.Now().Unix()
	session := UserSession{
		SID: fmt.Sprintf("sensitive-enabled-session-%d", user.Id), UserID: user.Id,
		Version: 1, UserAuthVersion: user.AuthVersion, Status: UserSessionStatusActive,
		RefreshHash: "sensitive-enabled-refresh", LoginMethod: "password",
		CreatedAt: now, LastActiveAt: now, ExpiresAt: now + 3600,
	}
	require.NoError(t, DB.Create(&session).Error)

	_, err := EnableUserAndResetSensitiveWordViolations(user.Id)
	require.ErrorContains(t, err, "Only accounts disabled by sensitive-word automatic enforcement can be unbanned here")
	count := 0
	require.NoError(t, UpdateSensitiveWordUserFields(user.Id, &count, nil))

	var stored User
	require.NoError(t, DB.First(&stored, user.Id).Error)
	require.Equal(t, common.UserStatusEnabled, stored.Status)
	require.Zero(t, stored.SensitiveWordViolationCount)
	require.Equal(t, user.AuthVersion, stored.AuthVersion)
	var active UserSession
	require.NoError(t, DB.First(&active, "sid = ?", session.SID).Error)
	require.Equal(t, UserSessionStatusActive, active.Status)
	require.Empty(t, active.RevokedReason)
}

func TestSensitiveWordAdministratorStatusDecisionSupersedesAutomaticBanSource(t *testing.T) {
	setupSensitiveWordTest(t)
	user := createSensitiveWordTestUser(t, 2_100_000, false)
	require.NoError(t, DB.Model(&User{}).Where("id = ?", user.Id).Updates(map[string]any{
		"status": common.UserStatusDisabled, "sensitive_word_violation_count": 4,
		"sensitive_word_auto_banned": true,
	}).Error)
	require.NoError(t, DB.First(user, user.Id).Error)

	require.NoError(t, user.SetStatusByAdministrator(common.UserStatusDisabled))
	require.False(t, user.SensitiveWordAutoBanned)
	_, err := EnableUserAndResetSensitiveWordViolations(user.Id)
	require.ErrorContains(t, err, "Only accounts disabled by sensitive-word automatic enforcement can be unbanned here")

	var stored User
	require.NoError(t, DB.First(&stored, user.Id).Error)
	require.Equal(t, common.UserStatusDisabled, stored.Status)
	require.Equal(t, 4, stored.SensitiveWordViolationCount)
}

func TestSensitiveWordAuditFailurePreservesBlockingDecision(t *testing.T) {
	setupSensitiveWordTest(t)
	require.NoError(t, MigrateSensitiveWordData())
	saveSensitiveWordTestPolicy(t, true, SensitiveWordBanThreshold)
	addSensitiveWordTestRule(t, "audit failure", []string{"audit failure marker"}, SensitiveWordScopeGlobal, nil, SensitiveWordModeBlock)
	user := createSensitiveWordTestUser(t, 4_200_000, false)

	triggerName := fmt.Sprintf("sensitive_audit_failure_%d", time.Now().UnixNano())
	require.NoError(t, DB.Exec(fmt.Sprintf(
		"CREATE TRIGGER %s BEFORE INSERT ON sensitive_word_audit_events BEGIN SELECT RAISE(ABORT, 'forced audit failure'); END",
		triggerName,
	)).Error)
	t.Cleanup(func() { _ = DB.Exec("DROP TRIGGER " + triggerName).Error })

	result, err := CheckSensitiveRequest(sensitiveWordTestInput(user, "audit-failure", "audit failure marker"))
	require.ErrorIs(t, err, ErrSensitiveWordAuditPersistence)
	require.NotNil(t, result)
	require.True(t, result.Matched)
	require.True(t, result.Blocked, "a blocking match must not reach upstream when its audit write fails")
	require.False(t, result.Counted, "rolled-back enforcement must not claim a committed count")
	require.Zero(t, result.ViolationCount)
	var stored User
	require.NoError(t, DB.First(&stored, user.Id).Error)
	require.Zero(t, stored.SensitiveWordViolationCount)
}

func TestSensitiveWordAuditFailurePreservesWhitelistBypass(t *testing.T) {
	setupSensitiveWordTest(t)
	require.NoError(t, MigrateSensitiveWordData())
	saveSensitiveWordTestPolicy(t, true, SensitiveWordBanThreshold)
	addSensitiveWordTestRule(t, "whitelist audit failure", []string{"whitelist failure marker"}, SensitiveWordScopeGlobal, nil, SensitiveWordModeBlock)
	user := createSensitiveWordTestUser(t, 4_250_000, true)

	triggerName := fmt.Sprintf("sensitive_whitelist_failure_%d", time.Now().UnixNano())
	require.NoError(t, DB.Exec(fmt.Sprintf(
		"CREATE TRIGGER %s BEFORE INSERT ON sensitive_word_audit_events BEGIN SELECT RAISE(ABORT, 'forced audit failure'); END",
		triggerName,
	)).Error)
	t.Cleanup(func() { _ = DB.Exec("DROP TRIGGER " + triggerName).Error })

	result, err := CheckSensitiveRequest(sensitiveWordTestInput(user, "whitelist-audit-failure", "whitelist failure marker"))
	require.ErrorIs(t, err, ErrSensitiveWordAuditPersistence)
	require.NotNil(t, result)
	require.True(t, result.Matched)
	require.True(t, result.WhitelistBypassed)
	require.False(t, result.Blocked)
	require.Equal(t, "whitelist_bypass", result.Disposition)
}

func TestSensitiveWordObserveAuditFailureIsTyped(t *testing.T) {
	setupSensitiveWordTest(t)
	require.NoError(t, MigrateSensitiveWordData())
	saveSensitiveWordTestPolicy(t, true, SensitiveWordBanThreshold)
	addSensitiveWordTestRule(t, "observe failure", []string{"observe failure marker"}, SensitiveWordScopeGlobal, nil, SensitiveWordModeObserve)
	user := createSensitiveWordTestUser(t, 4_300_000, false)

	triggerName := fmt.Sprintf("sensitive_observe_failure_%d", time.Now().UnixNano())
	require.NoError(t, DB.Exec(fmt.Sprintf(
		"CREATE TRIGGER %s BEFORE INSERT ON sensitive_word_audit_events BEGIN SELECT RAISE(ABORT, 'forced audit failure'); END",
		triggerName,
	)).Error)
	t.Cleanup(func() { _ = DB.Exec("DROP TRIGGER " + triggerName).Error })

	result, err := CheckSensitiveRequest(sensitiveWordTestInput(user, "observe-failure", "observe failure marker"))
	require.ErrorIs(t, err, ErrSensitiveWordAuditPersistence)
	require.True(t, result.Matched)
	require.True(t, result.ObserveOnly)
	require.False(t, result.Blocked)
	var stored User
	require.NoError(t, DB.First(&stored, user.Id).Error)
	require.Zero(t, stored.SensitiveWordViolationCount)
}

func TestSensitiveWordPromptEvidenceLimits(t *testing.T) {
	require.Equal(t, "abc…", truncateSensitivePrompt("abcdef", 3))
	require.Equal(t, "ab", truncateSensitivePrompt("a\x00b", 10))
	value := truncateSensitivePrompt(strings.Repeat("界", 200_000), 200_000)
	require.True(t, utf8.ValidString(value))
	require.LessOrEqual(t, len([]byte(value)), SensitiveWordMaxPromptBytes)
	require.True(t, strings.HasSuffix(value, "…"))
	require.Equal(t, "mediumtext", sensitiveWordAuditPromptDBType(string(common.DatabaseTypeMySQL)))
	require.Equal(t, "text", sensitiveWordAuditPromptDBType(string(common.DatabaseTypePostgreSQL)))
	require.Equal(t, "text", sensitiveWordAuditPromptDBType(string(common.DatabaseTypeSQLite)))

	fields := []relaytypes.SensitiveTextField{
		{Role: relaytypes.SensitiveTextRoleSystem, Kind: relaytypes.SensitiveTextKindContent, Round: -1, Order: 0, Text: "system"},
		{Role: relaytypes.SensitiveTextRoleUser, Kind: relaytypes.SensitiveTextKindContent, Round: 0, Order: 1, Text: "first user"},
		{Role: relaytypes.SensitiveTextRoleAssistant, Kind: relaytypes.SensitiveTextKindContent, Round: 0, Order: 2, Text: "assistant"},
		{Role: relaytypes.SensitiveTextRoleUser, Kind: relaytypes.SensitiveTextKindContent, Round: 1, Order: 3, Text: "matched user"},
		{Role: relaytypes.SensitiveTextRoleUser, Kind: relaytypes.SensitiveTextKindTurnBoundary, Round: 1, Order: 4},
	}
	matches := []SensitiveWordMatch{{
		RuleID: 1, RuleMode: SensitiveWordModeBlock, Word: "matched",
		FieldOrder: 3, FieldRole: relaytypes.SensitiveTextRoleUser,
		FieldKind: relaytypes.SensitiveTextKindContent, FieldRound: 1,
	}}
	evidenceJSON, _, _, _, _, err := buildSensitiveAuditEvidence(fields, matches, defaultSensitiveWordPolicy(), false)
	require.NoError(t, err)
	var evidence SensitiveWordAuditEvidence
	require.NoError(t, common.Unmarshal([]byte(evidenceJSON), &evidence))
	require.Equal(t, []int{0, 1, 2, 3}, []int{
		evidence.Fields[0].Order, evidence.Fields[1].Order,
		evidence.Fields[2].Order, evidence.Fields[3].Order,
	})
	for _, field := range evidence.Fields {
		require.NotEqual(t, relaytypes.SensitiveTextKindTurnBoundary, field.Kind)
	}
}

func TestSensitiveWordAuditEvidenceCentersRetainedMatch(t *testing.T) {
	word := "命中词"
	text := strings.Repeat("前", 160) + word
	matchStart := 160
	fields := []relaytypes.SensitiveTextField{{
		Role: relaytypes.SensitiveTextRoleUser, Kind: relaytypes.SensitiveTextKindContent,
		Round: 4, Order: 9, Text: text,
	}}
	matches := []SensitiveWordMatch{{
		RuleID: 1, RuleMode: SensitiveWordModeBlock, Word: word,
		FieldOrder: 9, FieldRole: relaytypes.SensitiveTextRoleUser,
		FieldKind: relaytypes.SensitiveTextKindContent, FieldRound: 4,
		StartRune: matchStart, EndRune: matchStart + len([]rune(word)),
	}}
	policy := defaultSensitiveWordPolicy()
	policy.MaxPromptRunes = 17
	evidenceJSON, _, _, truncated, _, err := buildSensitiveAuditEvidence(fields, matches, policy, false)
	require.NoError(t, err)
	require.True(t, truncated)
	require.LessOrEqual(t, len([]byte(evidenceJSON)), SensitiveWordMaxPromptBytes)

	var evidence SensitiveWordAuditEvidence
	require.NoError(t, common.Unmarshal([]byte(evidenceJSON), &evidence))
	require.Len(t, evidence.Fields, 1)
	require.Equal(t, 9, evidence.Fields[0].Order)
	require.LessOrEqual(t, len([]rune(evidence.Fields[0].Text)), policy.MaxPromptRunes)
	require.Contains(t, evidence.Fields[0].Text, word)
	require.True(t, strings.HasPrefix(evidence.Fields[0].Text, "…"))
	require.False(t, strings.HasSuffix(evidence.Fields[0].Text, "…"), "a match at the field end should retain the end of the field")
	require.Equal(t, matchStart, evidence.Matches[0].StartRune, "stored match offsets remain relative to the original normalized field")
}

func TestSensitiveWordAuditEvidenceStoresEachFieldOnceAndPrefersBlock(t *testing.T) {
	observeWord := "观察词"
	blockWord := "阻断词"
	middle := strings.Repeat("中", 200)
	text := observeWord + middle + blockWord
	blockStart := len([]rune(observeWord + middle))
	fields := []relaytypes.SensitiveTextField{{
		Role: relaytypes.SensitiveTextRoleUser, Kind: relaytypes.SensitiveTextKindContent,
		Round: 2, Order: 5, Text: text,
	}}
	matches := []SensitiveWordMatch{
		{
			RuleID: 1, RuleMode: SensitiveWordModeObserve, Word: observeWord,
			FieldOrder: 5, FieldRole: relaytypes.SensitiveTextRoleUser,
			FieldKind: relaytypes.SensitiveTextKindContent, FieldRound: 2,
			StartRune: 0, EndRune: len([]rune(observeWord)),
		},
		{
			RuleID: 2, RuleMode: SensitiveWordModeBlock, Word: blockWord,
			FieldOrder: 5, FieldRole: relaytypes.SensitiveTextRoleUser,
			FieldKind: relaytypes.SensitiveTextKindContent, FieldRound: 2,
			StartRune: blockStart, EndRune: blockStart + len([]rune(blockWord)),
		},
	}
	policy := defaultSensitiveWordPolicy()
	policy.MaxPromptRunes = 21
	evidenceJSON, _, _, truncated, _, err := buildSensitiveAuditEvidence(fields, matches, policy, false)
	require.NoError(t, err)
	require.True(t, truncated)

	var evidence SensitiveWordAuditEvidence
	require.NoError(t, common.Unmarshal([]byte(evidenceJSON), &evidence))
	require.Len(t, evidence.Fields, 1)
	require.Contains(t, evidence.Fields[0].Text, blockWord)
	require.NotContains(t, evidence.Fields[0].Text, observeWord)
	require.Len(t, evidence.Matches, 2)
}

func TestSensitiveWordAuditEvidenceKeepsTotalSerializedSizeBounded(t *testing.T) {
	fields := []relaytypes.SensitiveTextField{
		{Role: relaytypes.SensitiveTextRoleSystem, Kind: relaytypes.SensitiveTextKindContent, Round: -1, Order: 0, Text: strings.Repeat("系", SensitiveWordMaxPromptRunes)},
		{Role: relaytypes.SensitiveTextRoleUser, Kind: relaytypes.SensitiveTextKindContent, Round: 0, Order: 1, Text: strings.Repeat("甲", SensitiveWordMaxPromptRunes)},
		{Role: relaytypes.SensitiveTextRoleAssistant, Kind: relaytypes.SensitiveTextKindContent, Round: 0, Order: 2, Text: strings.Repeat("乙", SensitiveWordMaxPromptRunes)},
		{Role: relaytypes.SensitiveTextRoleUser, Kind: relaytypes.SensitiveTextKindContent, Round: 1, Order: 3, Text: strings.Repeat("丙", SensitiveWordMaxPromptRunes-3) + "命中词"},
	}
	matches := []SensitiveWordMatch{{
		RuleID: 1, RuleMode: SensitiveWordModeBlock, Word: "命中词",
		FieldOrder: 3, FieldRole: relaytypes.SensitiveTextRoleUser,
		FieldKind: relaytypes.SensitiveTextKindContent, FieldRound: 1,
		StartRune: SensitiveWordMaxPromptRunes - 3, EndRune: SensitiveWordMaxPromptRunes,
	}}
	evidenceJSON, _, _, truncated, _, err := buildSensitiveAuditEvidence(fields, matches, defaultSensitiveWordPolicy(), false)
	require.NoError(t, err)
	require.True(t, truncated)
	require.LessOrEqual(t, len([]byte(evidenceJSON)), SensitiveWordMaxPromptBytes)

	var evidence SensitiveWordAuditEvidence
	require.NoError(t, common.Unmarshal([]byte(evidenceJSON), &evidence))
	require.NotEmpty(t, evidence.Fields)
	var matchedField *relaytypes.SensitiveTextField
	for index := range evidence.Fields {
		if evidence.Fields[index].Order == 3 {
			matchedField = &evidence.Fields[index]
			break
		}
	}
	require.NotNil(t, matchedField)
	require.Contains(t, matchedField.Text, "命中词")
}

func TestSensitiveWordAuditEvidenceRetentionCleanup(t *testing.T) {
	setupSensitiveWordTest(t)
	require.NoError(t, MigrateSensitiveWordData())
	policy := GetSensitiveWordPolicy()
	policy.FullPromptRetentionDays = 1
	require.NoError(t, SaveSensitiveWordPolicy(policy, 1))
	user := createSensitiveWordTestUser(t, 5_000_000, false)

	old := SensitiveWordAuditEvent{
		RequestID: "expired-evidence", UserID: user.Id, FullPrompt: "expired prompt",
		RedactedPreview: "expired preview", MatchedSnippets: `["expired snippet"]`,
		CreatedAt: time.Now().Add(-48 * time.Hour),
	}
	fresh := SensitiveWordAuditEvent{
		RequestID: "fresh-evidence", UserID: user.Id, FullPrompt: "fresh prompt",
		RedactedPreview: "fresh preview", MatchedSnippets: `["fresh snippet"]`,
		CreatedAt: time.Now(),
	}
	require.NoError(t, DB.Create(&old).Error)
	require.NoError(t, DB.Create(&fresh).Error)

	cleared, err := CleanupSensitiveWordAudits()
	require.NoError(t, err)
	require.Equal(t, int64(1), cleared)
	var storedOld, storedFresh SensitiveWordAuditEvent
	require.NoError(t, DB.First(&storedOld, old.ID).Error)
	require.NoError(t, DB.First(&storedFresh, fresh.ID).Error)
	require.Empty(t, storedOld.FullPrompt)
	require.Empty(t, storedOld.RedactedPreview)
	require.Equal(t, "[]", string(storedOld.MatchedSnippets))
	require.Equal(t, SensitiveWordAuditPrompt("fresh prompt"), storedFresh.FullPrompt)
	require.Equal(t, "fresh preview", storedFresh.RedactedPreview)

	cleared, err = CleanupSensitiveWordAudits()
	require.NoError(t, err)
	require.Zero(t, cleared, "already-cleared evidence must not be rewritten every day")
}

func TestSensitiveWordAuditCleanupUsesBoundedBatches(t *testing.T) {
	setupSensitiveWordTest(t)
	require.NoError(t, MigrateSensitiveWordData())
	policy := GetSensitiveWordPolicy()
	policy.FullPromptRetentionDays = 1
	require.NoError(t, SaveSensitiveWordPolicy(policy, 1))
	user := createSensitiveWordTestUser(t, 5_100_000, false)
	require.NoError(t, DB.Model(&User{}).Where("id = ?", user.Id).
		Update("sensitive_word_violation_count", 9).Error)

	events := make([]SensitiveWordAuditEvent, 0, SensitiveWordAuditCleanupBatchSize+1)
	for index := range SensitiveWordAuditCleanupBatchSize + 1 {
		events = append(events, SensitiveWordAuditEvent{
			RequestID: fmt.Sprintf("expired-batch-%d", index), UserID: user.Id,
			Evidence: `{"version":1,"fields":[]}`, FullPrompt: "expired prompt",
			RedactedPreview: "expired preview", MatchedSnippets: `["expired snippet"]`,
			MatchedWords: `["marker"]`, Disposition: "block", Blocked: true,
			Counted: true, ViolationCount: 9, CreatedAt: time.Now().Add(-48 * time.Hour),
		})
	}
	require.NoError(t, DB.CreateInBatches(&events, 100).Error)

	cleared, err := CleanupSensitiveWordAudits()
	require.NoError(t, err)
	require.Equal(t, int64(SensitiveWordAuditCleanupBatchSize), cleared)
	var remaining int64
	require.NoError(t, DB.Model(&SensitiveWordAuditEvent{}).
		Where("evidence <> ''").Count(&remaining).Error)
	require.Equal(t, int64(1), remaining)

	cleared, err = CleanupSensitiveWordAudits()
	require.NoError(t, err)
	require.Equal(t, int64(1), cleared)
	cleared, err = CleanupSensitiveWordAudits()
	require.NoError(t, err)
	require.Zero(t, cleared)

	var storedEvent SensitiveWordAuditEvent
	require.NoError(t, DB.First(&storedEvent, events[0].ID).Error)
	require.Empty(t, storedEvent.Evidence)
	require.Empty(t, storedEvent.FullPrompt)
	require.Empty(t, storedEvent.RedactedPreview)
	require.Equal(t, SensitiveWordAuditJSON("[]"), storedEvent.MatchedSnippets)
	require.Equal(t, SensitiveWordAuditJSON(`["marker"]`), storedEvent.MatchedWords)
	require.True(t, storedEvent.Blocked)
	require.True(t, storedEvent.Counted)
	require.Equal(t, 9, storedEvent.ViolationCount)

	var storedUser User
	require.NoError(t, DB.First(&storedUser, user.Id).Error)
	require.Equal(t, 9, storedUser.SensitiveWordViolationCount)
}

func TestSensitiveWordUserSafetyFieldsRequireExplicitUpdate(t *testing.T) {
	setupSensitiveWordTest(t)
	user := createSensitiveWordTestUser(t, 5_500_000, true)
	require.NoError(t, DB.Model(&User{}).Where("id = ?", user.Id).
		Update("sensitive_word_violation_count", 7).Error)

	ordinaryEdit := *user
	ordinaryEdit.DisplayName = "edited user"
	ordinaryEdit.SensitiveWordViolationCount = 0
	ordinaryEdit.SensitiveWordWhitelist = false
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		return ordinaryEdit.EditWithTx(tx, false, nil)
	}))
	var stored User
	require.NoError(t, DB.First(&stored, user.Id).Error)
	require.Equal(t, 7, stored.SensitiveWordViolationCount)
	require.True(t, stored.SensitiveWordWhitelist)

	count := 0
	whitelist := false
	require.NoError(t, UpdateSensitiveWordUserFields(user.Id, &count, &whitelist))
	require.NoError(t, DB.First(&stored, user.Id).Error)
	require.Zero(t, stored.SensitiveWordViolationCount)
	require.False(t, stored.SensitiveWordWhitelist)

	negative := -1
	require.Error(t, UpdateSensitiveWordUserFields(user.Id, &negative, nil))
}

func TestSensitiveWordMigrationIsOneWayAndIdempotent(t *testing.T) {
	setupSensitiveWordTest(t)
	checkPrompt := true
	legacyPolicy, err := common.Marshal(legacySensitiveWordConfig{
		Enabled: true, CheckPrompt: &checkPrompt, AuditEnabled: false,
		BlockMessage: "legacy {{threshold}}", BanThreshold: 9,
		FullPromptRetentionDays: 30, MaxPromptRunes: 4096, RuleVersion: 7,
	})
	require.NoError(t, err)
	require.NoError(t, DB.Create(&Option{Key: "SensitiveWordConfig", Value: string(legacyPolicy)}).Error)
	require.NoError(t, DB.Create(&Option{Key: "SensitiveWords", Value: "legacy-one\nlegacy-two\nLEGACY-ONE"}).Error)
	user := createSensitiveWordTestUser(t, 6_800_000, false)

	require.NoError(t, MigrateSensitiveWordData())
	require.NoError(t, MigrateSensitiveWordData())
	policy := GetSensitiveWordPolicy()
	require.True(t, policy.Enabled)
	require.False(t, policy.RetainFullPrompt)
	require.Equal(t, 9, policy.BanThreshold)
	require.Equal(t, 4096, policy.MaxPromptRunes)
	rules, err := ListSensitiveWordRules()
	require.NoError(t, err)
	require.Len(t, rules, 1)
	require.Equal(t, 2, rules[0].WordCount)
	detail, err := GetSensitiveWordRuleDetail(rules[0].ID)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"legacy-one", "legacy-two"}, detail.Words)
	var migrated User
	require.NoError(t, DB.First(&migrated, user.Id).Error)
	require.False(t, migrated.SensitiveWordWhitelist)
	var marker Option
	require.NoError(t, DB.Where(&Option{Key: sensitiveWordMigrationKey}).First(&marker).Error)
	require.Equal(t, sensitiveWordMigrationValue, marker.Value)

	require.NoError(t, DeleteSensitiveWordRule(detail.ID, 1))
	require.NoError(t, MigrateSensitiveWordData())
	rules, err = ListSensitiveWordRules()
	require.NoError(t, err)
	require.Empty(t, rules, "committed migration must never resurrect legacy options")
}

func TestSensitiveWordAuditIdentityMigrationDeduplicatesBeforeUniqueIndex(t *testing.T) {
	database, err := gorm.Open(sqlite.Open(t.TempDir()+"/audit-identity.db"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, database.AutoMigrate(&legacySensitiveWordV1Audit{}))

	first := legacySensitiveWordV1Audit{RequestID: "duplicate-request", UserID: 7, CreatedAt: time.Now()}
	second := legacySensitiveWordV1Audit{RequestID: "duplicate-request", UserID: 7, CreatedAt: time.Now()}
	missing := legacySensitiveWordV1Audit{RequestID: "", UserID: 8, CreatedAt: time.Now()}
	require.NoError(t, database.Create(&first).Error)
	require.NoError(t, database.Create(&second).Error)
	require.NoError(t, database.Create(&missing).Error)

	require.NoError(t, prepareSensitiveWordAuditIdentityMigration(database))
	require.True(t, database.Migrator().HasIndex(&SensitiveWordAuditEvent{}, "idx_sensitive_word_audit_identity"))
	var duplicateCount int64
	require.NoError(t, database.Model(&legacySensitiveWordV1Audit{}).
		Where("user_id = ? AND request_id = ?", 7, "duplicate-request").Count(&duplicateCount).Error)
	require.Equal(t, int64(1), duplicateCount)
	var migratedMissing legacySensitiveWordV1Audit
	require.NoError(t, database.First(&migratedMissing, missing.ID).Error)
	require.Equal(t, fmt.Sprintf("legacy-sensitive-audit-%d", missing.ID), migratedMissing.RequestID)

	conflict := legacySensitiveWordV1Audit{RequestID: "duplicate-request", UserID: 7, CreatedAt: time.Now()}
	require.Error(t, database.Create(&conflict).Error)
}

func TestSensitiveWordMigrationImportsValidLegacyWordsAndRetainsInvalidSource(t *testing.T) {
	setupSensitiveWordTest(t)
	legacyWord := strings.Repeat("超长旧词", 80)
	require.Greater(t, len([]rune(legacyWord)), SensitiveWordMaxRunes)
	legacyValue := "legacy-valid-one\n" + legacyWord + "\nlegacy-valid-two"
	require.NoError(t, DB.Create(&Option{Key: "SensitiveWords", Value: legacyValue}).Error)
	require.NoError(t, MigrateSensitiveWordData())
	var marker Option
	require.NoError(t, DB.Where(&Option{Key: sensitiveWordMigrationKey}).First(&marker).Error)
	require.Equal(t, sensitiveWordMigrationValue, marker.Value)
	require.True(t, GetSensitiveWordPolicy().Enabled)
	rules, err := ListSensitiveWordRules()
	require.NoError(t, err)
	require.Len(t, rules, 1)
	detail, err := GetSensitiveWordRuleDetail(rules[0].ID)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"legacy-valid-one", "legacy-valid-two"}, detail.Words)
	var legacy Option
	require.NoError(t, DB.Where(&Option{Key: "SensitiveWords"}).First(&legacy).Error)
	require.Equal(t, legacyValue, legacy.Value)
}

func TestSensitiveWordMigratesInitialP30Schema(t *testing.T) {
	require.NoError(t, DB.Migrator().DropTable(
		&SensitiveWordAuditEvent{}, &SensitiveWordPolicy{}, &SensitiveWordRuleWord{},
		&SensitiveWordRuleGroup{}, &SensitiveWordRule{},
	))
	require.NoError(t, DB.AutoMigrate(
		&Option{}, &User{}, &legacySensitiveWordV1Rule{}, &SensitiveWordRuleGroup{},
		&legacySensitiveWordV1Audit{},
	))
	require.NoError(t, DB.Where("key IN ?", []string{
		"SensitiveWords", "SensitiveWordConfig", sensitiveWordMigrationKey,
	}).Delete(&Option{}).Error)
	setSensitiveWordRuntimeUnavailable(false)

	user := createSensitiveWordTestUser(t, 7_100_000, false)
	legacyRule := legacySensitiveWordV1Rule{
		Word: "initial-p30-rule", Scope: SensitiveWordScopeGlobal, Enabled: true,
		CreatedBy: 7, Version: 3, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	require.NoError(t, DB.Create(&legacyRule).Error)
	require.NoError(t, DB.Create(&legacySensitiveWordV1Audit{
		RequestID: "initial-p30-audit", UserID: user.Id, FullPrompt: "legacy audit prompt",
		MatchedRuleIDs: "[1]", MatchedWords: `["initial-p30-rule"]`, MatchedSnippets: "[]",
		CreatedAt: time.Now(),
	}).Error)
	legacyConfig, err := common.Marshal(map[string]any{
		"enabled": true, "audit_enabled": true, "block_message": "legacy policy",
		"ban_threshold": 9, "full_prompt_retention_days": 30,
		"max_prompt_runes": 4096, "rule_version": 2,
	})
	require.NoError(t, err)
	require.NoError(t, DB.Create(&Option{Key: "SensitiveWordConfig", Value: string(legacyConfig)}).Error)
	require.NoError(t, DB.Create(&Option{Key: "SensitiveWords", Value: "initial-p30-option"}).Error)

	// This is the same AutoMigrate phase production performs before the
	// one-way import. In particular, old rows receive a NULL mode rather than
	// being silently converted to observe by a schema default.
	require.NoError(t, DB.AutoMigrate(
		&User{}, &SensitiveWordRule{}, &SensitiveWordRuleWord{}, &SensitiveWordRuleGroup{},
		&SensitiveWordPolicy{}, &SensitiveWordAuditEvent{},
	))
	require.NoError(t, MigrateSensitiveWordData())

	policy := GetSensitiveWordPolicy()
	require.True(t, policy.Enabled)
	require.True(t, policy.CheckPrompt, "missing v1 check_prompt must preserve the secure default")
	require.Equal(t, 9, policy.BanThreshold)

	detail, err := GetSensitiveWordRuleDetail(legacyRule.ID)
	require.NoError(t, err)
	require.Equal(t, SensitiveWordModeBlock, detail.Mode)
	require.Equal(t, []string{"initial-p30-rule"}, detail.Words)
	rules, err := ListSensitiveWordRules()
	require.NoError(t, err)
	require.Len(t, rules, 2, "legacy global option must not disappear beside an old global rule")

	var migratedUser User
	require.NoError(t, DB.First(&migratedUser, user.Id).Error)
	require.False(t, migratedUser.SensitiveWordWhitelist)
	var audit SensitiveWordAuditEvent
	require.NoError(t, DB.Where("request_id = ?", "initial-p30-audit").First(&audit).Error)
	require.Equal(t, SensitiveWordAuditPrompt("legacy audit prompt"), audit.FullPrompt)
	var marker Option
	require.NoError(t, DB.Where(&Option{Key: sensitiveWordMigrationKey}).First(&marker).Error)
	require.Equal(t, sensitiveWordMigrationValue, marker.Value)

	t.Cleanup(func() {
		_ = DB.AutoMigrate(
			&SensitiveWordRule{}, &SensitiveWordRuleWord{}, &SensitiveWordRuleGroup{},
			&SensitiveWordPolicy{}, &SensitiveWordAuditEvent{},
		)
		_ = DB.Where("key IN ?", []string{
			"SensitiveWords", "SensitiveWordConfig", sensitiveWordMigrationKey,
		}).Delete(&Option{}).Error
		invalidateSensitiveWordRuntime()
	})
}

func TestSensitiveWordMigrationFailureDisablesRuntimeWithoutBlockingStartup(t *testing.T) {
	setupSensitiveWordTest(t)
	require.NoError(t, DB.Create(&Option{Key: "SensitiveWordConfig", Value: "not-json"}).Error)
	require.Error(t, MigrateSensitiveWordData())

	require.NoError(t, DB.Create(&SensitiveWordPolicy{
		ID: 1, Enabled: true, CheckPrompt: true, RetainFullPrompt: true,
		BlockMessage: sensitiveWordBlockMessage, BanThreshold: SensitiveWordBanThreshold,
		FullPromptRetentionDays: 180, MaxPromptRunes: SensitiveWordMaxPromptRunes, Version: 1,
	}).Error)
	require.False(t, GetSensitiveWordPolicy().Enabled, "failed migration must not activate partial state")

	require.NoError(t, DB.Where(&Option{Key: "SensitiveWordConfig"}).Delete(&Option{}).Error)
	require.NoError(t, MigrateSensitiveWordData())
	require.True(t, GetSensitiveWordPolicy().Enabled)
}

func TestSensitiveWordRuntimeRequiresCompletedMigrationMarker(t *testing.T) {
	setupSensitiveWordTest(t)
	require.NoError(t, DB.Create(&SensitiveWordPolicy{
		ID: 1, Enabled: true, CheckPrompt: true, RetainFullPrompt: true,
		BlockMessage: sensitiveWordBlockMessage, BanThreshold: SensitiveWordBanThreshold,
		FullPromptRetentionDays: 180, MaxPromptRunes: SensitiveWordMaxPromptRunes, Version: 1,
	}).Error)
	setSensitiveWordRuntimeUnavailable(false)

	policy := GetSensitiveWordPolicy()
	require.False(t, policy.Enabled, "a slave must not use partially migrated rules before the marker exists")

	require.NoError(t, DB.Create(&Option{
		Key: sensitiveWordMigrationKey, Value: sensitiveWordMigrationValue,
	}).Error)
	require.NoError(t, syncSensitiveWordRuntimeGeneration())
	require.True(t, GetSensitiveWordPolicy().Enabled)
}
