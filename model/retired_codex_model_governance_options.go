package model

import "errors"

var retiredCodexModelGovernanceOptionKeys = []string{
	"codex_model_governance_setting.dingtalk_enabled",
	"codex_model_governance_setting.dingtalk_webhook_url",
	"codex_model_governance_setting.dingtalk_secret",
	"codex_model_governance_setting.ai_analysis_api_key",
	"codex_model_governance_setting.ai_analysis_base_url",
	"codex_model_governance_setting.ai_analysis_model",
}

func CleanupRetiredCodexModelGovernanceOptions() (int64, error) {
	if DB == nil {
		return 0, errors.New("database is not initialized")
	}

	result := DB.Where(map[string]any{"key": retiredCodexModelGovernanceOptionKeys}).Delete(&Option{})
	return result.RowsAffected, result.Error
}
