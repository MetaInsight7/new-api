package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCleanupRetiredCodexModelGovernanceOptions(t *testing.T) {
	db := useFrontendOptionMigrationDB(t)
	retiredKeys := []string{
		"codex_model_governance_setting.dingtalk_enabled",
		"codex_model_governance_setting.dingtalk_webhook_url",
		"codex_model_governance_setting.dingtalk_secret",
		"codex_model_governance_setting.ai_analysis_api_key",
		"codex_model_governance_setting.ai_analysis_base_url",
		"codex_model_governance_setting.ai_analysis_model",
	}
	retained := []Option{
		{Key: "codex_model_governance_setting.alert_cooldown_minutes", Value: "60"},
		{Key: "monitor_setting.dingtalk_alert_enabled", Value: "true"},
		{Key: "monitor_setting.dingtalk_alert_webhook_url", Value: "https://example.com/robot"},
		{Key: "monitor_setting.dingtalk_alert_secret", Value: "shared-secret"},
		{Key: "monitor_setting.ai_analysis_api_key", Value: "shared-api-key"},
		{Key: "monitor_setting.ai_analysis_base_url", Value: "https://example.com/v1"},
		{Key: "monitor_setting.ai_analysis_model", Value: "analysis-model"},
		{Key: "unrelated.option", Value: "preserved"},
	}
	options := make([]Option, 0, len(retiredKeys)+len(retained))
	for _, key := range retiredKeys {
		options = append(options, Option{Key: key, Value: "obsolete"})
	}
	options = append(options, retained...)
	require.NoError(t, db.Create(&options).Error)

	deleted, err := CleanupRetiredCodexModelGovernanceOptions()
	require.NoError(t, err)
	assert.Equal(t, int64(len(retiredKeys)), deleted)
	for _, key := range retiredKeys {
		requireOptionMissing(t, db, key)
	}
	for _, option := range retained {
		assert.Equal(t, option.Value, requireOptionValue(t, db, option.Key))
	}

	deleted, err = CleanupRetiredCodexModelGovernanceOptions()
	require.NoError(t, err)
	assert.Zero(t, deleted)
}
