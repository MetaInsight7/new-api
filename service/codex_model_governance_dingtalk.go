package service

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

func BuildDingTalkCodexModelGovernanceAlertContent(record *model.CodexModelGovernanceRecord) string {
	affected := model.DecodeCodexModelGovernanceChannelIDs(record.AffectedChannelIDs)
	disabled := model.CodexModelGovernanceDisabledChannelIDs(*record)
	content := fmt.Sprintf("Codex model governance alert\nModel: %s\nStatus: %s\nSource: %s\nMatched Rule: %s\nAffected Channels: %v\nDisabled Channels: %v\nReason: %s\nReview in the Codex model governance page.", record.ModelName, record.Status, record.Source, record.MatchedRule, affected, disabled, record.LastError)
	if len(disabled) < len(affected) {
		content += "\nLinked channels are still serving user requests."
	}
	return model.SanitizeCodexModelGovernanceEvidence(content)
}

func NotifyDingTalkCodexModelGovernance(ctx context.Context, record *model.CodexModelGovernanceRecord) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	setting := operation_setting.GetMonitorAlertSetting()
	if !setting.DingTalkAlertEnabled || record == nil {
		return false, nil
	}
	if err := ValidateSSRFProtectedFetchURLContext(ctx, setting.DingTalkAlertWebhookURL); err != nil {
		return false, fmt.Errorf("DingTalk webhook rejected by fetch policy")
	}
	scope := record.ModelName + "\x00" + record.AffectedChannelIDs + "\x00" + model.EncodeCodexModelGovernanceChannelIDsForDisplay(model.CodexModelGovernanceDisabledChannelIDs(*record))
	key := fmt.Sprintf("codex-governance:%x", sha256.Sum256([]byte(scope)))
	token, err := common.GenerateRandomCharsKey(32)
	if err != nil {
		return false, err
	}
	cooldown := time.Duration(max(1, operation_setting.GetCodexModelGovernanceSetting().AlertCooldownMinutes)) * time.Minute
	reservation, allowed, err := model.ReserveCodexModelGovernanceAlertCooldown(key, cooldown, 30*time.Second, token)
	if err != nil || !allowed {
		return false, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = model.RollbackCodexModelGovernanceAlertCooldown(reservation)
		}
	}()
	if err := sendDingTalkText(ctx, setting.DingTalkAlertWebhookURL, setting.DingTalkAlertSecret, BuildDingTalkCodexModelGovernanceAlertContent(record)); err != nil {
		return false, err
	}
	committed = true
	if err := model.CommitCodexModelGovernanceAlertCooldown(reservation); err != nil {
		return true, err
	}
	return true, nil
}
