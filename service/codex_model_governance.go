package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"gorm.io/gorm"
)

const (
	CodexModelGovernanceReviewActionConfirmRemove = "confirm_remove"
	CodexModelGovernanceReviewActionRestore       = "restore"
	CodexModelGovernanceReviewActionIgnore        = "ignore"
	CodexModelGovernanceReviewActionDisable       = "disable"
)

type CodexModelUnsupportedFinding struct {
	ModelName          string
	Source             string
	MatchedRule        string
	LastError          string
	AffectedChannelIDs []int
}

var notifyDingTalkCodexModelGovernance = NotifyDingTalkCodexModelGovernance

func codexFindingShouldAutoDisable(source string) bool {
	return source == model.CodexModelGovernanceSourceProbe
}

func MoveCodexModelToPendingReview(finding CodexModelUnsupportedFinding) (*model.CodexModelGovernanceRecord, error) {
	return MoveCodexModelToPendingReviewWithContext(context.Background(), finding)
}

func MoveCodexModelToPendingReviewWithContext(ctx context.Context, finding CodexModelUnsupportedFinding) (*model.CodexModelGovernanceRecord, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	modelName := strings.TrimSpace(finding.ModelName)
	if modelName == "" {
		return nil, fmt.Errorf("model name is required")
	}
	var previous *model.CodexModelGovernanceRecord
	previousRecord, err := model.GetCodexModelGovernanceRecordByModelName(modelName)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	} else {
		previous = previousRecord
	}
	source := strings.TrimSpace(finding.Source)
	disableAbilities := codexFindingShouldAutoDisable(source)
	record, err := model.UpsertCodexModelGovernancePending(model.CodexModelGovernancePendingInput{
		ModelName:          modelName,
		Source:             source,
		MatchedRule:        strings.TrimSpace(finding.MatchedRule),
		LastError:          strings.TrimSpace(finding.LastError),
		AffectedChannelIDs: finding.AffectedChannelIDs,
		DisableAbilities:   disableAbilities,
	})
	if err != nil {
		return record, err
	}
	if shouldNotifyCodexModelGovernanceFinding(record, previous, disableAbilities) {
		sent, notifyErr := notifyDingTalkCodexModelGovernance(ctx, record)
		if notifyErr != nil {
			common.SysError("Codex model governance alert failed: " + model.SanitizeCodexModelGovernanceEvidence(notifyErr.Error()))
		}
		if sent {
			alertedAt := common.GetTimestamp()
			if err := model.MarkCodexModelGovernanceRecordAlerted(record.ID, alertedAt); err != nil {
				return record, err
			}
			record.LastAlertedAt = alertedAt
		}
	}
	return record, nil
}

func shouldNotifyCodexModelGovernanceFinding(record *model.CodexModelGovernanceRecord, previous *model.CodexModelGovernanceRecord, disableAbilities bool) bool {
	if record == nil {
		return false
	}
	if previous == nil {
		return record.Status == model.CodexModelGovernanceStatusUnsupportedPendingReview ||
			record.Status == model.CodexModelGovernanceStatusUnsupportedDisabled
	}
	if hasNewCodexGovernanceDisabledChannels(record, previous) {
		return true
	}
	if hasNewCodexGovernanceAffectedChannels(record, previous) {
		return true
	}
	if record.Status != model.CodexModelGovernanceStatusUnsupportedPendingReview {
		return false
	}
	if previous.Status == model.CodexModelGovernanceStatusIgnored && disableAbilities {
		return true
	}
	cooldownMinutes := 60
	if setting := operation_setting.GetCodexModelGovernanceSetting(); setting != nil && setting.AlertCooldownMinutes > 0 {
		cooldownMinutes = setting.AlertCooldownMinutes
	}
	if record.LastAlertedAt <= 0 {
		return true
	}
	cooldownSeconds := int64(time.Duration(cooldownMinutes) * time.Minute / time.Second)
	return common.GetTimestamp()-record.LastAlertedAt >= cooldownSeconds
}

func hasNewCodexGovernanceDisabledChannels(record *model.CodexModelGovernanceRecord, previous *model.CodexModelGovernanceRecord) bool {
	if record == nil || previous == nil {
		return false
	}
	return hasNewCodexGovernanceChannelIDs(
		model.CodexModelGovernanceDisabledChannelIDs(*record),
		model.CodexModelGovernanceDisabledChannelIDs(*previous),
	)
}

func hasNewCodexGovernanceAffectedChannels(record *model.CodexModelGovernanceRecord, previous *model.CodexModelGovernanceRecord) bool {
	if record == nil || previous == nil {
		return false
	}
	return hasNewCodexGovernanceChannelIDs(
		model.DecodeCodexModelGovernanceChannelIDs(record.AffectedChannelIDs),
		model.DecodeCodexModelGovernanceChannelIDs(previous.AffectedChannelIDs),
	)
}

func hasNewCodexGovernanceChannelIDs(currentChannelIDs []int, previousChannelIDs []int) bool {
	if len(currentChannelIDs) == 0 {
		return false
	}
	seen := make(map[int]struct{}, len(previousChannelIDs))
	for _, channelID := range previousChannelIDs {
		seen[channelID] = struct{}{}
	}
	for _, channelID := range currentChannelIDs {
		if _, ok := seen[channelID]; !ok {
			return true
		}
	}
	return false
}

func ReviewCodexModelGovernance(recordID int, action string, reviewerID int, note string) error {
	status, err := codexModelGovernanceStatusForReviewAction(action)
	if err != nil {
		return err
	}
	return model.ReviewCodexModelGovernanceRecord(recordID, status, reviewerID, note)
}

func codexModelGovernanceStatusForReviewAction(action string) (string, error) {
	switch strings.TrimSpace(action) {
	case CodexModelGovernanceReviewActionConfirmRemove:
		return model.CodexModelGovernanceStatusRemoved, nil
	case CodexModelGovernanceReviewActionRestore:
		return model.CodexModelGovernanceStatusActive, nil
	case CodexModelGovernanceReviewActionIgnore:
		return model.CodexModelGovernanceStatusIgnored, nil
	case CodexModelGovernanceReviewActionDisable:
		return model.CodexModelGovernanceStatusUnsupportedDisabled, nil
	default:
		return "", fmt.Errorf("unsupported Codex model governance review action: %s", action)
	}
}
