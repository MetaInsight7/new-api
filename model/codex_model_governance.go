package model

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	CodexModelGovernanceStatusActive                   = "active"
	CodexModelGovernanceStatusUnsupportedPendingReview = "unsupported_pending_review"
	CodexModelGovernanceStatusUnsupportedDisabled      = "unsupported_disabled"
	CodexModelGovernanceStatusPendingReview            = CodexModelGovernanceStatusUnsupportedPendingReview
	CodexModelGovernanceStatusRemoved                  = "removed"
	CodexModelGovernanceStatusIgnored                  = "ignored"

	CodexModelGovernanceSourceProbe               = "probe"
	CodexModelGovernanceSourceOfficialCodexNotice = "official_codex_notice"
	CodexModelGovernanceSourceOfficialNotice      = CodexModelGovernanceSourceOfficialCodexNotice
	CodexModelGovernanceSourceManual              = "manual"
)

type CodexModelGovernanceRecord struct {
	ID                 int    `json:"id" gorm:"primaryKey"`
	ModelName          string `json:"model_name" gorm:"type:varchar(255);uniqueIndex;not null"`
	Status             string `json:"status" gorm:"type:varchar(32);index;not null"`
	Source             string `json:"source" gorm:"type:varchar(64);index;not null"`
	MatchedRule        string `json:"matched_rule" gorm:"type:varchar(255)"`
	LastError          string `json:"last_error" gorm:"type:text"`
	AffectedChannelIDs string `json:"affected_channel_ids" gorm:"type:text"`
	DisabledChannelIDs string `json:"disabled_channel_ids" gorm:"type:text"`
	AbilitiesDisabled  bool   `json:"abilities_disabled" gorm:"default:false"`
	DetectedAt         int64  `json:"detected_at" gorm:"type:bigint;index"`
	LastCheckedAt      int64  `json:"last_checked_at" gorm:"type:bigint;index"`
	LastAlertedAt      int64  `json:"last_alerted_at" gorm:"type:bigint;index"`
	ReviewedAt         int64  `json:"reviewed_at" gorm:"type:bigint"`
	ReviewedBy         int    `json:"reviewed_by" gorm:"index;default:0"`
	ReviewNote         string `json:"review_note" gorm:"type:text"`
	CreatedTime        int64  `json:"created_time" gorm:"type:bigint"`
	UpdatedTime        int64  `json:"updated_time" gorm:"type:bigint"`
}

type CodexModelGovernancePendingInput struct {
	ModelName          string
	Source             string
	MatchedRule        string
	LastError          string
	AffectedChannelIDs []int
	LastCheckedAt      int64
	DisableAbilities   bool
}

func GetCodexModelGovernanceChannels(ctx context.Context, withKey bool) ([]*Channel, error) {
	var channels []*Channel
	query := DB.WithContext(ctx).Where("type = ?", constant.ChannelTypeCodex).Order("id ASC")
	if !withKey {
		query = query.Omit("key")
	}
	err := query.Find(&channels).Error
	return channels, err
}

func encodeCodexModelGovernanceChannelIDs(channelIDs []int) string {
	channelIDs = normalizeCodexModelGovernanceChannelIDs(channelIDs)
	parts := make([]string, 0, len(channelIDs))
	for _, id := range channelIDs {
		parts = append(parts, strconv.Itoa(id))
	}
	return strings.Join(parts, ",")
}

func decodeCodexModelGovernanceChannelIDs(encoded string) []int {
	if strings.TrimSpace(encoded) == "" {
		return nil
	}
	parts := strings.Split(encoded, ",")
	ids := make([]int, 0, len(parts))
	for _, part := range parts {
		id, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil {
			continue
		}
		ids = append(ids, id)
	}
	return normalizeCodexModelGovernanceChannelIDs(ids)
}

func DecodeCodexModelGovernanceChannelIDs(encoded string) []int {
	return decodeCodexModelGovernanceChannelIDs(encoded)
}

func EncodeCodexModelGovernanceChannelIDsForDisplay(channelIDs []int) string {
	return encodeCodexModelGovernanceChannelIDs(channelIDs)
}

func CodexModelGovernanceDisabledChannelIDs(record CodexModelGovernanceRecord) []int {
	disabledChannelIDs := decodeCodexModelGovernanceChannelIDs(record.DisabledChannelIDs)
	if len(disabledChannelIDs) > 0 {
		return disabledChannelIDs
	}
	if record.AbilitiesDisabled {
		return decodeCodexModelGovernanceChannelIDs(record.AffectedChannelIDs)
	}
	return nil
}

func normalizeCodexModelGovernanceChannelIDs(channelIDs []int) []int {
	seen := make(map[int]struct{}, len(channelIDs))
	normalized := make([]int, 0, len(channelIDs))
	for _, id := range channelIDs {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		normalized = append(normalized, id)
	}
	slices.Sort(normalized)
	return normalized
}

func codexModelGovernanceChannelIDSet(channelIDs []int) map[int]struct{} {
	result := make(map[int]struct{}, len(channelIDs))
	for _, channelID := range normalizeCodexModelGovernanceChannelIDs(channelIDs) {
		result[channelID] = struct{}{}
	}
	return result
}

func codexModelGovernanceChannelIDContains(channelIDs []int, channelID int) bool {
	if channelID <= 0 {
		return false
	}
	_, ok := codexModelGovernanceChannelIDSet(channelIDs)[channelID]
	return ok
}

func removeCodexModelGovernanceChannelID(channelIDs []int, channelID int) []int {
	channelIDs = normalizeCodexModelGovernanceChannelIDs(channelIDs)
	if channelID <= 0 || len(channelIDs) == 0 {
		return channelIDs
	}
	next := make([]int, 0, len(channelIDs))
	for _, id := range channelIDs {
		if id != channelID {
			next = append(next, id)
		}
	}
	return next
}

func codexModelGovernanceRecordAffectsChannel(record CodexModelGovernanceRecord, channelID int) bool {
	affectedChannelIDs := decodeCodexModelGovernanceChannelIDs(record.AffectedChannelIDs)
	return codexModelGovernanceChannelIDContains(affectedChannelIDs, channelID)
}

func codexModelGovernanceRecordDisablesChannel(record CodexModelGovernanceRecord, channelID int) bool {
	if !record.AbilitiesDisabled {
		return false
	}
	disabledChannelIDs := CodexModelGovernanceDisabledChannelIDs(record)
	return codexModelGovernanceChannelIDContains(disabledChannelIDs, channelID)
}

func resolveCodexModelGovernancePendingScopes(modelName string, inputChannelIDs []int, disableAbilities bool) ([]int, []int, error) {
	inputChannelIDs = normalizeCodexModelGovernanceChannelIDs(inputChannelIDs)
	affectedChannelIDs, err := FindAffectedCodexChannelIDs(modelName)
	if err != nil {
		return nil, nil, err
	}
	affectedChannelIDs = normalizeCodexModelGovernanceChannelIDs(append(affectedChannelIDs, inputChannelIDs...))
	if len(affectedChannelIDs) == 0 {
		affectedChannelIDs = inputChannelIDs
	}

	disabledChannelIDs := []int(nil)
	if disableAbilities {
		if len(inputChannelIDs) > 0 {
			disabledChannelIDs, err = filterCodexChannelIDs(DB, inputChannelIDs)
			if err != nil {
				return nil, nil, err
			}
		} else {
			disabledChannelIDs = affectedChannelIDs
		}
	}
	return affectedChannelIDs, normalizeCodexModelGovernanceChannelIDs(disabledChannelIDs), nil
}

func UpsertCodexModelGovernancePending(input CodexModelGovernancePendingInput) (*CodexModelGovernanceRecord, error) {
	modelName := strings.TrimSpace(input.ModelName)
	if modelName == "" {
		return nil, errors.New("model name is required")
	}
	if len([]rune(modelName)) > 255 {
		return nil, errors.New("model name exceeds 255 characters")
	}
	source := strings.TrimSpace(input.Source)
	if source == "" {
		source = CodexModelGovernanceSourceProbe
	}
	if len([]rune(source)) > 64 {
		return nil, errors.New("source exceeds 64 characters")
	}
	matchedRule := []rune(SanitizeCodexModelGovernanceEvidence(input.MatchedRule))
	if len(matchedRule) > 255 {
		matchedRule = matchedRule[:255]
	}
	now := common.GetTimestamp()
	lastCheckedAt := input.LastCheckedAt
	if lastCheckedAt == 0 {
		lastCheckedAt = now
	}
	affectedChannelIDs, disabledChannelIDs, err := resolveCodexModelGovernancePendingScopes(modelName, input.AffectedChannelIDs, input.DisableAbilities)
	if err != nil {
		return nil, err
	}
	requestedDisabledChannelIDs := disabledChannelIDs

	var record CodexModelGovernanceRecord
	abilitiesChanged := false
	err = DB.Transaction(func(tx *gorm.DB) error {
		seed := CodexModelGovernanceRecord{
			ModelName: modelName, Status: CodexModelGovernanceStatusUnsupportedPendingReview,
			Source: source, DetectedAt: now, CreatedTime: now, UpdatedTime: now,
		}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&seed).Error; err != nil {
			return err
		}
		err := lockForUpdate(tx).First(&record, "model_name = ?", modelName).Error
		if err != nil {
			return err
		} else {
			if record.DetectedAt == 0 {
				record.DetectedAt = now
			}
			if record.Status == CodexModelGovernanceStatusRemoved {
				affectedChannelIDs = normalizeCodexModelGovernanceChannelIDs(
					append(decodeCodexModelGovernanceChannelIDs(record.AffectedChannelIDs), affectedChannelIDs...),
				)
				changed, err := removeCodexModelFromChannelsWithDB(tx, modelName, affectedChannelIDs)
				if err != nil {
					return err
				}
				abilitiesChanged = abilitiesChanged || changed
				if err := tx.Model(&CodexModelGovernanceRecord{}).Where("id = ?", record.ID).Updates(map[string]any{
					"source":               source,
					"matched_rule":         string(matchedRule),
					"last_error":           SanitizeCodexModelGovernanceEvidence(input.LastError),
					"affected_channel_ids": encodeCodexModelGovernanceChannelIDs(affectedChannelIDs),
					"disabled_channel_ids": encodeCodexModelGovernanceChannelIDs(affectedChannelIDs),
					"abilities_disabled":   len(affectedChannelIDs) > 0,
					"detected_at":          record.DetectedAt,
					"last_checked_at":      lastCheckedAt,
					"updated_time":         now,
				}).Error; err != nil {
					return err
				}
				requestedDisabledChannelIDs = nil
				return tx.First(&record, "id = ?", record.ID).Error
			}
			currentDisabledChannelIDs := CodexModelGovernanceDisabledChannelIDs(record)
			affectedChannelIDs = normalizeCodexModelGovernanceChannelIDs(
				append(decodeCodexModelGovernanceChannelIDs(record.AffectedChannelIDs), affectedChannelIDs...),
			)
			nextStatus := CodexModelGovernanceStatusUnsupportedPendingReview
			if record.Status == CodexModelGovernanceStatusUnsupportedDisabled {
				nextStatus = CodexModelGovernanceStatusUnsupportedDisabled
			} else if record.Status == CodexModelGovernanceStatusIgnored && !input.DisableAbilities {
				nextStatus = CodexModelGovernanceStatusIgnored
			}
			updates := map[string]any{
				"status":               nextStatus,
				"source":               source,
				"matched_rule":         string(matchedRule),
				"last_error":           SanitizeCodexModelGovernanceEvidence(input.LastError),
				"affected_channel_ids": encodeCodexModelGovernanceChannelIDs(affectedChannelIDs),
				"disabled_channel_ids": encodeCodexModelGovernanceChannelIDs(currentDisabledChannelIDs),
				"abilities_disabled":   len(currentDisabledChannelIDs) > 0,
				"detected_at":          record.DetectedAt,
				"last_checked_at":      lastCheckedAt,
				"updated_time":         now,
			}
			if nextStatus == CodexModelGovernanceStatusUnsupportedPendingReview && record.Status != CodexModelGovernanceStatusUnsupportedPendingReview {
				updates["reviewed_at"] = int64(0)
				updates["reviewed_by"] = 0
				updates["review_note"] = ""
			}
			if err := tx.Model(&CodexModelGovernanceRecord{}).Where("id = ?", record.ID).Updates(updates).Error; err != nil {
				return err
			}
			if err := tx.First(&record, "id = ?", record.ID).Error; err != nil {
				return err
			}
		}
		if len(requestedDisabledChannelIDs) > 0 {
			changed, err := setCodexModelAbilityEnabledWithDB(tx, modelName, requestedDisabledChannelIDs, false)
			if err != nil {
				return err
			}
			abilitiesChanged = abilitiesChanged || changed
			disabledChannelIDs := normalizeCodexModelGovernanceChannelIDs(
				append(CodexModelGovernanceDisabledChannelIDs(record), requestedDisabledChannelIDs...),
			)
			if err := tx.Model(&CodexModelGovernanceRecord{}).Where("id = ?", record.ID).Updates(map[string]any{
				"disabled_channel_ids": encodeCodexModelGovernanceChannelIDs(disabledChannelIDs),
				"abilities_disabled":   len(disabledChannelIDs) > 0,
				"updated_time":         now,
			}).Error; err != nil {
				return err
			}
			return tx.First(&record, "id = ?", record.ID).Error
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if abilitiesChanged {
		RefreshCodexModelGovernanceCache()
	}
	return &record, nil
}

func MarkCodexModelGovernanceRecordAlerted(id int, alertedAt int64) error {
	if id <= 0 {
		return gorm.ErrRecordNotFound
	}
	if alertedAt == 0 {
		alertedAt = common.GetTimestamp()
	}
	return DB.Model(&CodexModelGovernanceRecord{}).
		Where("id = ?", id).
		Select("last_alerted_at").
		Update("last_alerted_at", alertedAt).Error
}

func ListCodexModelGovernanceRecords(status string) ([]CodexModelGovernanceRecord, error) {
	var records []CodexModelGovernanceRecord
	query := DB.Model(&CodexModelGovernanceRecord{})
	if strings.TrimSpace(status) != "" {
		query = query.Where("status = ?", strings.TrimSpace(status))
	}
	err := query.Order("updated_time DESC").Order("id DESC").Find(&records).Error
	return records, err
}

func GetCodexModelGovernanceRecord(id int) (*CodexModelGovernanceRecord, error) {
	if id <= 0 {
		return nil, gorm.ErrRecordNotFound
	}
	var record CodexModelGovernanceRecord
	if err := DB.First(&record, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &record, nil
}

func GetCodexModelGovernanceRecordByModelName(modelName string) (*CodexModelGovernanceRecord, error) {
	modelName = strings.TrimSpace(modelName)
	if modelName == "" {
		return nil, gorm.ErrRecordNotFound
	}
	var record CodexModelGovernanceRecord
	if err := DB.First(&record, "model_name = ?", modelName).Error; err != nil {
		return nil, err
	}
	return &record, nil
}

func FindAffectedCodexChannelIDs(modelName string) ([]int, error) {
	modelName = strings.TrimSpace(modelName)
	if modelName == "" {
		return nil, nil
	}
	var channels []Channel
	if err := DB.Model(&Channel{}).
		Where("type = ?", constant.ChannelTypeCodex).
		Find(&channels).Error; err != nil {
		return nil, err
	}
	ids := make([]int, 0, len(channels))
	for _, channel := range channels {
		if codexGovernanceChannelHasModel(channel.Models, modelName) {
			ids = append(ids, channel.Id)
		}
	}
	return normalizeCodexModelGovernanceChannelIDs(ids), nil
}

func DisableCodexModelAbilities(modelName string, channelIDs []int) error {
	return setCodexModelAbilityEnabled(modelName, channelIDs, false)
}

func RestoreCodexModelAbilities(modelName string, channelIDs []int) error {
	changed, err := restoreCodexModelAbilitiesWithDB(DB, modelName, channelIDs)
	if err != nil {
		return err
	}
	if changed {
		RefreshCodexModelGovernanceCache()
	}
	return nil
}

func RestoreCodexModelGovernanceAfterProbeSuccess(modelName string, channelID int) error {
	modelName = strings.TrimSpace(modelName)
	if modelName == "" || channelID <= 0 || DB == nil {
		return nil
	}

	abilitiesChanged := false
	err := DB.Transaction(func(tx *gorm.DB) error {
		var record CodexModelGovernanceRecord
		err := lockForUpdate(tx).First(&record, "model_name = ?", modelName).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		if record.Status == CodexModelGovernanceStatusRemoved {
			return nil
		}

		disabledChannelIDs := CodexModelGovernanceDisabledChannelIDs(record)
		nextDisabledChannelIDs := disabledChannelIDs
		if codexModelGovernanceChannelIDContains(disabledChannelIDs, channelID) {
			changed, err := restoreCodexModelAbilitiesWithDB(tx, modelName, []int{channelID})
			if err != nil {
				return err
			}
			abilitiesChanged = abilitiesChanged || changed
			nextDisabledChannelIDs = removeCodexModelGovernanceChannelID(disabledChannelIDs, channelID)
		}

		nextStatus := record.Status
		if codexModelGovernanceChannelIDContains(disabledChannelIDs, channelID) && len(nextDisabledChannelIDs) == 0 {
			nextStatus = CodexModelGovernanceStatusActive
		}
		now := common.GetTimestamp()
		updates := map[string]any{
			"status":               nextStatus,
			"disabled_channel_ids": encodeCodexModelGovernanceChannelIDs(nextDisabledChannelIDs),
			"abilities_disabled":   len(nextDisabledChannelIDs) > 0,
			"last_checked_at":      now,
			"updated_time":         now,
		}
		if nextStatus == CodexModelGovernanceStatusActive && record.Status != CodexModelGovernanceStatusActive {
			updates["reviewed_at"] = int64(0)
			updates["reviewed_by"] = 0
			updates["review_note"] = ""
		}
		return tx.Model(&CodexModelGovernanceRecord{}).Where("id = ?", record.ID).Updates(updates).Error
	})
	if err != nil {
		return err
	}
	if abilitiesChanged {
		RefreshCodexModelGovernanceCache()
	}
	return nil
}

func restoreCodexModelAbilitiesWithDB(db *gorm.DB, modelName string, channelIDs []int) (bool, error) {
	modelName = strings.TrimSpace(modelName)
	if modelName == "" {
		return false, errors.New("model name is required")
	}
	codexChannelIDs, err := filterEnabledCodexChannelIDsWithModel(db, modelName, channelIDs)
	if err != nil {
		return false, err
	}
	if len(codexChannelIDs) == 0 {
		return false, nil
	}
	var channels []Channel
	if err := db.Where("id IN ?", codexChannelIDs).Find(&channels).Error; err != nil {
		return false, err
	}
	changed := false
	for _, channel := range channels {
		for _, group := range strings.Split(channel.Group, ",") {
			ability := Ability{Group: group, Model: modelName, ChannelId: channel.Id, Enabled: true,
				Priority: channel.Priority, Weight: uint(channel.GetWeight()), Tag: channel.Tag}
			result := db.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "group"}, {Name: "model"}, {Name: "channel_id"}},
				DoUpdates: clause.AssignmentColumns([]string{"enabled"}),
			}).Create(&ability)
			if result.Error != nil {
				return changed, result.Error
			}
			changed = changed || result.RowsAffected > 0
		}
	}
	return changed, nil
}

func RemoveCodexModelFromChannels(modelName string, channelIDs []int) error {
	changed, err := removeCodexModelFromChannelsWithDB(DB, modelName, channelIDs)
	if err != nil {
		return err
	}
	if changed {
		RefreshCodexModelGovernanceCache()
	}
	return nil
}

func removeCodexModelFromChannelsWithDB(db *gorm.DB, modelName string, channelIDs []int) (bool, error) {
	modelName = strings.TrimSpace(modelName)
	channelIDs = normalizeCodexModelGovernanceChannelIDs(channelIDs)
	if modelName == "" {
		return false, errors.New("model name is required")
	}
	if len(channelIDs) == 0 {
		return false, nil
	}

	ownsTx := db == DB
	tx := db
	if ownsTx {
		tx = db.Begin()
		if tx.Error != nil {
			return false, tx.Error
		}
	}
	defer func() {
		if r := recover(); r != nil {
			if ownsTx {
				tx.Rollback()
			}
			panic(r)
		}
	}()

	var channels []Channel
	if err := lockForUpdate(tx).Where("id IN ? AND type = ?", channelIDs, constant.ChannelTypeCodex).Order("id ASC").Find(&channels).Error; err != nil {
		if ownsTx {
			tx.Rollback()
		}
		return false, err
	}
	changedAny := false
	for _, channel := range channels {
		nextModels, changed := removeCodexGovernanceModel(channel.Models, modelName)
		if !changed {
			continue
		}
		changedAny = true
		channel.Models = nextModels
		if err := tx.Model(&Channel{}).Where("id = ?", channel.Id).Update("models", nextModels).Error; err != nil {
			if ownsTx {
				tx.Rollback()
			}
			return false, err
		}
		if err := tx.Where("channel_id = ? AND model = ?", channel.Id, modelName).Delete(&Ability{}).Error; err != nil {
			if ownsTx {
				tx.Rollback()
			}
			return false, err
		}
	}
	if ownsTx {
		if err := tx.Commit().Error; err != nil {
			return false, err
		}
	}
	return changedAny, nil
}

func ReviewCodexModelGovernanceRecord(id int, status string, reviewerID int, note string) error {
	if id <= 0 {
		return gorm.ErrRecordNotFound
	}
	tx := DB.Begin()
	if tx.Error != nil {
		return tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			panic(r)
		}
	}()

	var record CodexModelGovernanceRecord
	err := lockForUpdate(tx).First(&record, "id = ?", id).Error
	if err != nil {
		tx.Rollback()
		return err
	}
	nextStatus := strings.TrimSpace(status)
	channelIDs := decodeCodexModelGovernanceChannelIDs(record.AffectedChannelIDs)
	changed := false
	disabledChannelIDs := CodexModelGovernanceDisabledChannelIDs(record)
	abilitiesDisabled := len(disabledChannelIDs) > 0
	switch nextStatus {
	case CodexModelGovernanceStatusActive:
		if record.Status == CodexModelGovernanceStatusRemoved {
			tx.Rollback()
			return fmt.Errorf("model %s has been removed from channel configuration; re-add it to the affected channels manually before restoring", record.ModelName)
		}
		if len(disabledChannelIDs) > 0 {
			actionChanged, err := restoreCodexModelAbilitiesWithDB(tx, record.ModelName, disabledChannelIDs)
			if err != nil {
				tx.Rollback()
				return err
			}
			if record.AbilitiesDisabled && !actionChanged {
				tx.Rollback()
				return fmt.Errorf("no Codex abilities were restored for model %s; ensure affected Codex channels are enabled and still include the model", record.ModelName)
			}
			changed = changed || actionChanged
		}
		disabledChannelIDs = nil
		abilitiesDisabled = false
	case CodexModelGovernanceStatusRemoved:
		actionChanged, err := removeCodexModelFromChannelsWithDB(tx, record.ModelName, channelIDs)
		if err != nil {
			tx.Rollback()
			return err
		}
		changed = changed || actionChanged
		disabledChannelIDs = channelIDs
		abilitiesDisabled = true
	case CodexModelGovernanceStatusIgnored:
		if record.AbilitiesDisabled || len(disabledChannelIDs) > 0 {
			tx.Rollback()
			return fmt.Errorf("model %s already has routing disabled; restore or remove it instead of ignoring the finding", record.ModelName)
		}
	case CodexModelGovernanceStatusUnsupportedPendingReview, CodexModelGovernanceStatusUnsupportedDisabled:
		actionChanged, err := setCodexModelAbilityEnabledWithDB(tx, record.ModelName, channelIDs, false)
		if err != nil {
			tx.Rollback()
			return err
		}
		changed = changed || actionChanged
		disabledChannelIDs = channelIDs
		abilitiesDisabled = true
	default:
		tx.Rollback()
		return fmt.Errorf("unsupported codex model governance status: %s", status)
	}

	now := common.GetTimestamp()
	if err := tx.Model(&CodexModelGovernanceRecord{}).Where("id = ?", id).Updates(map[string]any{
		"status":               nextStatus,
		"disabled_channel_ids": encodeCodexModelGovernanceChannelIDs(disabledChannelIDs),
		"abilities_disabled":   abilitiesDisabled,
		"reviewed_at":          now,
		"reviewed_by":          reviewerID,
		"review_note":          SanitizeCodexModelGovernanceEvidence(note),
		"updated_time":         now,
	}).Error; err != nil {
		tx.Rollback()
		return err
	}
	if err := tx.Commit().Error; err != nil {
		return err
	}
	if changed {
		RefreshCodexModelGovernanceCache()
	}
	return nil
}

func setCodexModelAbilityEnabled(modelName string, channelIDs []int, enabled bool) error {
	changed, err := setCodexModelAbilityEnabledWithDB(DB, modelName, channelIDs, enabled)
	if err != nil {
		return err
	}
	if changed {
		RefreshCodexModelGovernanceCache()
	}
	return nil
}

func setCodexModelAbilityEnabledWithDB(db *gorm.DB, modelName string, channelIDs []int, enabled bool) (bool, error) {
	modelName = strings.TrimSpace(modelName)
	channelIDs = normalizeCodexModelGovernanceChannelIDs(channelIDs)
	if modelName == "" {
		return false, errors.New("model name is required")
	}
	if len(channelIDs) == 0 {
		return false, nil
	}
	codexChannelIDs, err := filterCodexChannelIDs(db, channelIDs)
	if err != nil {
		return false, err
	}
	if len(codexChannelIDs) == 0 {
		return false, nil
	}
	result := db.Model(&Ability{}).
		Where("model = ? AND channel_id IN ?", modelName, codexChannelIDs).
		Where("enabled <> ?", enabled).
		Select("enabled").
		Update("enabled", enabled)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func reapplyCodexModelGovernanceDisabledAbilitiesWithDB(db *gorm.DB, channelIDs []int) (bool, error) {
	channelIDs = normalizeCodexModelGovernanceChannelIDs(channelIDs)
	if len(channelIDs) == 0 {
		return false, nil
	}
	codexChannelIDs, err := filterCodexChannelIDs(db, channelIDs)
	if err != nil {
		return false, err
	}
	if len(codexChannelIDs) == 0 {
		return false, nil
	}
	schemaPresent, err := codexModelGovernanceSchemaPresent(db)
	if err != nil {
		return false, err
	}
	if !schemaPresent {
		return false, nil
	}
	var records []CodexModelGovernanceRecord
	if err := db.Model(&CodexModelGovernanceRecord{}).
		Where("status IN ?", []string{
			CodexModelGovernanceStatusUnsupportedPendingReview,
			CodexModelGovernanceStatusUnsupportedDisabled,
			CodexModelGovernanceStatusRemoved,
		}).
		Find(&records).Error; err != nil {
		return false, err
	}
	if len(records) == 0 {
		return false, nil
	}
	requested := codexModelGovernanceChannelIDSet(codexChannelIDs)
	changed := false
	for _, record := range records {
		targetChannelIDs := []int(nil)
		if record.Status == CodexModelGovernanceStatusRemoved {
			targetChannelIDs = decodeCodexModelGovernanceChannelIDs(record.AffectedChannelIDs)
		} else {
			targetChannelIDs = CodexModelGovernanceDisabledChannelIDs(record)
		}
		targetChannelIDs = normalizeCodexModelGovernanceChannelIDs(targetChannelIDs)
		if len(targetChannelIDs) == 0 {
			continue
		}
		matchedChannelIDs := make([]int, 0, len(targetChannelIDs))
		for _, channelID := range targetChannelIDs {
			if _, ok := requested[channelID]; ok {
				matchedChannelIDs = append(matchedChannelIDs, channelID)
			}
		}
		if len(matchedChannelIDs) == 0 {
			continue
		}
		if record.Status == CodexModelGovernanceStatusRemoved {
			result := db.Where("model = ? AND channel_id IN ?", record.ModelName, matchedChannelIDs).Delete(&Ability{})
			if result.Error != nil {
				return changed, result.Error
			}
			changed = changed || result.RowsAffected > 0
			continue
		}
		result := db.Model(&Ability{}).
			Where("model = ? AND channel_id IN ?", record.ModelName, matchedChannelIDs).
			Where("enabled <> ?", false).
			Select("enabled").
			Update("enabled", false)
		if result.Error != nil {
			return changed, result.Error
		}
		changed = changed || result.RowsAffected > 0
	}
	return changed, nil
}

func filterCodexChannelIDs(db *gorm.DB, channelIDs []int) ([]int, error) {
	channelIDs = normalizeCodexModelGovernanceChannelIDs(channelIDs)
	if len(channelIDs) == 0 {
		return nil, nil
	}
	var channels []Channel
	if err := lockForUpdate(db).Model(&Channel{}).
		Where("id IN ? AND type = ?", channelIDs, constant.ChannelTypeCodex).
		Order("id").
		Find(&channels).Error; err != nil {
		return nil, err
	}
	ids := make([]int, 0, len(channels))
	for _, channel := range channels {
		ids = append(ids, channel.Id)
	}
	return normalizeCodexModelGovernanceChannelIDs(ids), nil
}

func filterEnabledCodexChannelIDsWithModel(db *gorm.DB, modelName string, channelIDs []int) ([]int, error) {
	channelIDs = normalizeCodexModelGovernanceChannelIDs(channelIDs)
	if len(channelIDs) == 0 {
		return nil, nil
	}
	var channels []Channel
	if err := lockForUpdate(db).Model(&Channel{}).
		Where("id IN ? AND type = ?", channelIDs, constant.ChannelTypeCodex).
		Order("id").
		Find(&channels).Error; err != nil {
		return nil, err
	}
	ids := make([]int, 0, len(channels))
	for _, channel := range channels {
		if channel.Status == common.ChannelStatusEnabled && codexGovernanceChannelHasModel(channel.Models, modelName) {
			ids = append(ids, channel.Id)
		}
	}
	return normalizeCodexModelGovernanceChannelIDs(ids), nil
}

func codexGovernanceChannelHasModel(models string, modelName string) bool {
	for _, model := range splitCodexGovernanceModels(models) {
		if model == modelName {
			return true
		}
	}
	return false
}

func removeCodexGovernanceModel(models string, modelName string) (string, bool) {
	existing := splitCodexGovernanceModels(models)
	next := make([]string, 0, len(existing))
	removed := false
	for _, model := range existing {
		if model == modelName {
			removed = true
			continue
		}
		next = append(next, model)
	}
	return strings.Join(next, ","), removed
}

func splitCodexGovernanceModels(models string) []string {
	if strings.TrimSpace(models) == "" {
		return nil
	}
	parts := strings.Split(models, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		model := strings.TrimSpace(part)
		if model == "" {
			continue
		}
		result = append(result, model)
	}
	return result
}

var codexGovernanceCredentialPattern = regexp.MustCompile(`(?i)(["']?(?:access_token|refresh_token|id_token|api[_-]?key|authorization|cookie|secret)["']?\s*[:=]\s*)(?:"[^"]*"|'[^']*'|bearer\s+[^\s,;}]+|[^\s,;}]+)`)
var codexGovernanceTokenPattern = regexp.MustCompile(`(?:sk-[A-Za-z0-9_-]+|eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+)`)

func SanitizeCodexModelGovernanceEvidence(value string) string {
	value = common.MaskSensitiveInfo(strings.TrimSpace(value))
	value = codexGovernanceCredentialPattern.ReplaceAllString(value, `${1}***`)
	value = codexGovernanceTokenPattern.ReplaceAllString(value, "***")
	runes := []rune(value)
	if len(runes) > 8192 {
		value = string(runes[:8192])
	}
	return value
}

func RefreshCodexModelGovernanceCache() {
	InitChannelCache()
	if err := common.PublishConfigChanged(context.Background(), common.ConfigScopeChannels); err != nil {
		common.SysError("failed to publish Codex governance channel-cache change; periodic cache sync remains available: " + err.Error())
	}
}

func ReapplyCodexModelGovernanceWithDB(db *gorm.DB, channelIDs []int) error {
	_, err := reapplyCodexModelGovernanceDisabledAbilitiesWithDB(db, channelIDs)
	return err
}

type codexAbilityGovernanceState struct {
	Removed  bool
	Disabled bool
}

func codexModelGovernanceSchemaPresent(db *gorm.DB) (bool, error) {
	tables, err := db.Migrator().GetTables()
	if err != nil {
		return false, err
	}
	statement := &gorm.Statement{DB: db}
	if err := statement.Parse(&CodexModelGovernanceRecord{}); err != nil {
		return false, err
	}
	return slices.Contains(tables, statement.Schema.Table), nil
}

func (channel *Channel) codexAbilityGovernanceByModel(db *gorm.DB, modelNames []string) (map[string]codexAbilityGovernanceState, error) {
	result := make(map[string]codexAbilityGovernanceState)
	if channel.Type != constant.ChannelTypeCodex {
		return result, nil
	}
	if db == nil {
		db = DB
	}
	schemaPresent, err := codexModelGovernanceSchemaPresent(db)
	if err != nil {
		return nil, err
	}
	if !schemaPresent {
		return result, nil
	}
	var records []CodexModelGovernanceRecord
	err = db.Where("model_name IN ?", modelNames).Find(&records).Error
	if err != nil {
		return nil, err
	}
	for _, record := range records {
		state := codexAbilityGovernanceState{}
		if record.Status == CodexModelGovernanceStatusRemoved {
			state.Removed = codexModelGovernanceRecordAffectsChannel(record, channel.Id)
		} else if record.Status == CodexModelGovernanceStatusUnsupportedPendingReview || record.Status == CodexModelGovernanceStatusUnsupportedDisabled {
			state.Disabled = codexModelGovernanceRecordDisablesChannel(record, channel.Id)
		}
		result[record.ModelName] = state
	}
	return result, nil
}
