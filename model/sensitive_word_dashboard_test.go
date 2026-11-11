package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sensitiveWordDashboardJSON(t *testing.T, value any) SensitiveWordAuditJSON {
	t.Helper()
	raw, err := marshalSensitiveAuditJSON(value)
	require.NoError(t, err)
	return raw
}

func TestSensitiveWordDashboardAggregatesOperationalView(t *testing.T) {
	setupSensitiveWordTest(t)
	require.NoError(t, MigrateSensitiveWordData())
	policy := GetSensitiveWordPolicy()
	policy.AutoBanEnabled = true
	require.NoError(t, SaveSensitiveWordPolicy(policy, 1))

	blockingRule := addSensitiveWordTestRule(
		t, "credential abuse", []string{"credential marker"},
		SensitiveWordScopeGlobal, nil, SensitiveWordModeBlock,
	)
	observeRule := addSensitiveWordTestRule(
		t, "observe context", []string{"observe marker"},
		SensitiveWordScopeGlobal, nil, SensitiveWordModeObserve,
	)
	firstUser := createSensitiveWordTestUser(t, 10_000, false)
	secondUser := createSensitiveWordTestUser(t, 10_000, false)
	now := time.Now()
	events := []SensitiveWordAuditEvent{
		{
			RequestID: "dashboard-block", UserID: firstUser.Id, UsernameSnapshot: firstUser.Username,
			ModelName: "gpt-dashboard", GroupName: "default", Disposition: "block",
			MatchedRuleIDs:   sensitiveWordDashboardJSON(t, []int64{blockingRule.ID}),
			MatchedRuleNames: sensitiveWordDashboardJSON(t, []string{blockingRule.Name}),
			Blocked:          true, Counted: true, AutoBanned: true, ViolationCount: 5,
			CreatedAt: now.Add(-30 * time.Minute),
		},
		{
			RequestID: "dashboard-observe", UserID: secondUser.Id, UsernameSnapshot: secondUser.Username,
			ModelName: "gpt-dashboard", GroupName: "premium", Disposition: "observe",
			MatchedRuleIDs:   sensitiveWordDashboardJSON(t, []int64{observeRule.ID}),
			MatchedRuleNames: sensitiveWordDashboardJSON(t, []string{observeRule.Name}),
			ObserveOnly:      true, ViolationCount: 0, CreatedAt: now.Add(-2 * time.Hour),
		},
		{
			RequestID: "dashboard-bypass", UserID: firstUser.Id, UsernameSnapshot: firstUser.Username,
			ModelName: "claude-dashboard", GroupName: "default", Disposition: "whitelist_bypass",
			MatchedRuleIDs:    sensitiveWordDashboardJSON(t, []int64{blockingRule.ID}),
			MatchedRuleNames:  sensitiveWordDashboardJSON(t, []string{blockingRule.Name}),
			WhitelistBypassed: true, ViolationCount: 5, CreatedAt: now.Add(-3 * time.Hour),
		},
		{
			RequestID: "dashboard-outside-window", UserID: firstUser.Id, UsernameSnapshot: firstUser.Username,
			ModelName: "old-model", GroupName: "default", Disposition: "block",
			MatchedRuleIDs:   sensitiveWordDashboardJSON(t, []int64{blockingRule.ID}),
			MatchedRuleNames: sensitiveWordDashboardJSON(t, []string{blockingRule.Name}),
			Blocked:          true, Counted: true, CreatedAt: now.Add(-8 * 24 * time.Hour),
		},
	}
	require.NoError(t, DB.Create(&events).Error)

	dashboard, err := GetSensitiveWordDashboard(168)
	require.NoError(t, err)
	assert.Equal(t, 168, dashboard.RequestedHours)
	assert.Equal(t, int64(24*60*60), dashboard.BucketSeconds)
	assert.Equal(t, "Asia/Shanghai", dashboard.Timezone)
	assert.True(t, dashboard.Policy.Enabled)
	assert.True(t, dashboard.Policy.AutoBanEnabled)
	assert.Equal(t, 2, dashboard.Policy.RuleCount)
	assert.Equal(t, 1, dashboard.Policy.BlockingRuleCount)
	assert.Equal(t, 1, dashboard.Policy.ObserveRuleCount)
	assert.Equal(t, SensitiveWordDashboardSummary{
		MatchedRequests: 3, BlockedRequests: 1, ObservedRequests: 1,
		WhitelistBypasses: 1, AffectedUsers: 2, AutomaticBans: 1,
	}, dashboard.Summary)
	require.Len(t, dashboard.Series, 7)
	var seriesTotal int64
	for _, point := range dashboard.Series {
		seriesTotal += point.MatchedRequests
	}
	assert.Equal(t, int64(3), seriesTotal)
	require.Len(t, dashboard.TopRules, 2)
	assert.Equal(t, blockingRule.ID, dashboard.TopRules[0].RuleID)
	assert.Equal(t, int64(2), dashboard.TopRules[0].MatchCount)
	require.Len(t, dashboard.TopUsers, 2)
	assert.Equal(t, firstUser.Id, dashboard.TopUsers[0].UserID)
	assert.Equal(t, int64(2), dashboard.TopUsers[0].MatchCount)
	assert.Equal(t, "gpt-dashboard", dashboard.TopModels[0].Name)
	assert.Equal(t, int64(2), dashboard.TopModels[0].Count)
	assert.Equal(t, "default", dashboard.TopGroups[0].Name)
	assert.Equal(t, int64(2), dashboard.TopGroups[0].Count)
	require.Len(t, dashboard.RecentEvents, 3)
	assert.Equal(t, "dashboard-block", dashboard.RecentEvents[0].RequestID)
	assert.Equal(t, []string{blockingRule.Name}, dashboard.RecentEvents[0].RuleNames)

	_, err = GetSensitiveWordDashboard(12)
	require.Error(t, err)
}
