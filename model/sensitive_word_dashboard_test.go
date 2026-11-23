package model

import (
	"strconv"
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

func TestSensitiveWordEventsSupportRangeFilteringSearchAndPagination(t *testing.T) {
	setupSensitiveWordTest(t)
	require.NoError(t, MigrateSensitiveWordData())
	now := time.Now().Truncate(time.Second)
	user := createSensitiveWordTestUser(t, 10_000, false)
	events := []SensitiveWordAuditEvent{
		{
			RequestID: "events-observe-new", UserID: user.Id, UsernameSnapshot: "searchable-user",
			ModelName: "observe-model", GroupName: "premium", Disposition: "observe", ObserveOnly: true,
			MatchedRuleNames: sensitiveWordDashboardJSON(t, []string{"observe rule"}), CreatedAt: now.Add(-time.Minute),
		},
		{
			RequestID: "events-observe-old", UserID: user.Id, UsernameSnapshot: "searchable-user",
			ModelName: "observe-model", GroupName: "default", Disposition: "observe", ObserveOnly: true,
			MatchedRuleNames: sensitiveWordDashboardJSON(t, []string{"observe rule"}), CreatedAt: now.Add(-2 * time.Hour),
		},
		{
			RequestID: "events-block", UserID: user.Id + 1, UsernameSnapshot: "blocked-user",
			ModelName: "block-model", GroupName: "default", Disposition: "block", Blocked: true,
			MatchedRuleNames: sensitiveWordDashboardJSON(t, []string{"block rule"}), CreatedAt: now.Add(-3 * time.Hour),
		},
		{
			RequestID: "events-outside", UserID: user.Id, UsernameSnapshot: "searchable-user",
			ModelName: "observe-model", GroupName: "default", Disposition: "observe", ObserveOnly: true,
			CreatedAt: now.Add(-48 * time.Hour),
		},
	}
	require.NoError(t, DB.Create(&events).Error)

	filter := SensitiveWordEventFilter{
		StartTimestamp: now.Add(-4 * time.Hour).Unix(), EndTimestamp: now.Unix(),
		Disposition: "observe", Keyword: "observe-model",
	}
	firstPage, total, err := ListSensitiveWordEvents(filter, 0, 1)
	require.NoError(t, err)
	require.Equal(t, int64(2), total)
	require.Len(t, firstPage, 1)
	require.Equal(t, "events-observe-new", firstPage[0].RequestID)
	require.Equal(t, []string{"observe rule"}, firstPage[0].RuleNames)

	secondPage, total, err := ListSensitiveWordEvents(filter, 1, 1)
	require.NoError(t, err)
	require.Equal(t, int64(2), total)
	require.Len(t, secondPage, 1)
	require.Equal(t, "events-observe-old", secondPage[0].RequestID)

	filter.Keyword = strconv.Itoa(user.Id)
	byUser, total, err := ListSensitiveWordEvents(filter, 0, 20)
	require.NoError(t, err)
	require.Equal(t, int64(2), total)
	require.Len(t, byUser, 2)

	filter.Keyword = "events-observe-old"
	byRequest, total, err := ListSensitiveWordEvents(filter, 0, 20)
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Equal(t, "events-observe-old", byRequest[0].RequestID)
}

func TestSensitiveWordDashboardSupportsCustomRange(t *testing.T) {
	setupSensitiveWordTest(t)
	require.NoError(t, MigrateSensitiveWordData())
	now := time.Now().Truncate(time.Second)
	require.NoError(t, DB.Create(&[]SensitiveWordAuditEvent{
		{RequestID: "custom-range-inside", UserID: 1, Disposition: "observe", ObserveOnly: true, CreatedAt: now.Add(-2 * time.Hour)},
		{RequestID: "custom-range-outside", UserID: 1, Disposition: "block", Blocked: true, CreatedAt: now.Add(-8 * time.Hour)},
	}).Error)

	dashboard, err := GetSensitiveWordDashboardForRange(now.Add(-4*time.Hour).Unix(), now.Unix())
	require.NoError(t, err)
	require.Equal(t, 4, dashboard.RequestedHours)
	require.Equal(t, now.Add(-4*time.Hour).Unix(), dashboard.WindowStart)
	require.Equal(t, now.Unix(), dashboard.WindowEnd)
	require.Equal(t, int64(1), dashboard.Summary.MatchedRequests)
	require.Equal(t, int64(1), dashboard.Summary.ObservedRequests)
	require.Len(t, dashboard.RecentEvents, 1)
	require.Equal(t, "custom-range-inside", dashboard.RecentEvents[0].RequestID)

	_, err = GetSensitiveWordDashboardForRange(now.Unix(), now.Add(-time.Hour).Unix())
	require.Error(t, err)
}
