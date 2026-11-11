package model

import (
	"errors"
	"fmt"
	"iter"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

func normalizeLegacySensitiveWords(words iter.Seq[string]) []string {
	result := make([]string, 0)
	seen := make(map[string]struct{})
	invalidCount := 0
	for raw := range words {
		word, err := normalizeSensitiveWord(raw)
		if err != nil {
			if strings.TrimSpace(raw) != "" {
				invalidCount++
			}
			continue
		}
		key := strings.ToLower(word)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		if len(result) >= SensitiveWordMaxRules {
			invalidCount++
			continue
		}
		result = append(result, word)
	}
	if invalidCount > 0 {
		common.SysLog(common.LogText("sensitive-word migration skipped %d over-limit legacy entries; the legacy source remains unchanged and valid entries were imported normally", invalidCount))
	}
	return result
}

func prepareSensitiveWordAutoBanOwnershipMigration(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	statement := &gorm.Statement{DB: db}
	if err := statement.Parse(&User{}); err != nil {
		return err
	}
	tableName := statement.Table
	if db.Statement != nil && db.Statement.Table != "" {
		tableName = db.Statement.Table
	}
	if !db.Migrator().HasTable(tableName) {
		return nil
	}

	columnName := "sensitive_word_auto_banned"
	if !db.Migrator().HasColumn(tableName, columnName) {
		if err := db.Exec("ALTER TABLE " + statement.Quote(tableName) + " ADD COLUMN sensitive_word_auto_banned BOOLEAN").Error; err != nil {
			return err
		}
	} else {
		columns, err := db.Migrator().ColumnTypes(tableName)
		if err != nil {
			return err
		}
		for _, column := range columns {
			if !strings.EqualFold(column.Name(), columnName) {
				continue
			}
			if nullable, known := column.Nullable(); known && !nullable {
				return nil
			}
			break
		}
	}

	return db.Table(tableName).Where("sensitive_word_auto_banned IS NULL").
		UpdateColumn("sensitive_word_auto_banned", false).Error
}

func prepareSensitiveWordAuditIdentityMigration(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable(&SensitiveWordAuditEvent{}) {
		return nil
	}
	if db.Migrator().HasIndex(&SensitiveWordAuditEvent{}, "idx_sensitive_word_audit_identity") {
		return nil
	}

	var missingIDs []int64
	if err := db.Model(&SensitiveWordAuditEvent{}).
		Where("request_id IS NULL OR request_id = ?", "").
		Order("id asc").Pluck("id", &missingIDs).Error; err != nil {
		return err
	}
	for _, id := range missingIDs {
		requestID := fmt.Sprintf("legacy-sensitive-audit-%d", id)
		if err := db.Model(&SensitiveWordAuditEvent{}).Where("id = ?", id).
			Update("request_id", requestID).Error; err != nil {
			return err
		}
	}

	type duplicateIdentity struct {
		UserID    int
		RequestID string
		KeepID    int64
	}
	var duplicates []duplicateIdentity
	if err := db.Model(&SensitiveWordAuditEvent{}).
		Select("user_id, request_id, MIN(id) AS keep_id").
		Group("user_id, request_id").Having("COUNT(*) > 1").
		Scan(&duplicates).Error; err != nil {
		return err
	}
	for _, duplicate := range duplicates {
		if err := db.Where(
			"user_id = ? AND request_id = ? AND id <> ?",
			duplicate.UserID, duplicate.RequestID, duplicate.KeepID,
		).Delete(&SensitiveWordAuditEvent{}).Error; err != nil {
			return err
		}
	}
	return db.Migrator().CreateIndex(&SensitiveWordAuditEvent{}, "idx_sensitive_word_audit_identity")
}

// MigrateSensitiveWordData performs an idempotent, one-way import. Once the
// marker is committed, legacy options and compatibility columns are never
// consulted by request handling or future migrations.
func MigrateSensitiveWordData() (err error) {
	completed := false
	defer func() {
		setSensitiveWordRuntimeUnavailable(err != nil || !completed)
	}()

	if DB == nil {
		return nil
	}
	if err := prepareSensitiveWordAuditIdentityMigration(DB); err != nil {
		return err
	}
	var marker Option
	markerErr := DB.Where(&Option{Key: sensitiveWordMigrationKey}).First(&marker).Error
	if markerErr == nil && marker.Value == sensitiveWordMigrationValue {
		var policy SensitiveWordPolicy
		policyErr := DB.First(&policy, 1).Error
		if errors.Is(policyErr, gorm.ErrRecordNotFound) {
			policy = defaultSensitiveWordPolicy()
			policyErr = DB.Create(&policy).Error
		}
		if policyErr != nil {
			return policyErr
		}
		completed = true
		return nil
	}
	if markerErr != nil && !errors.Is(markerErr, gorm.ErrRecordNotFound) {
		return markerErr
	}

	if err := migrateSensitiveWordPolicy(); err != nil {
		return err
	}
	if err := migrateSensitiveWordRules(); err != nil {
		return err
	}
	if err := migrateLegacySensitiveWordOption(); err != nil {
		return err
	}
	if err := DB.Save(&Option{Key: sensitiveWordMigrationKey, Value: sensitiveWordMigrationValue}).Error; err != nil {
		return err
	}
	completed = true
	return nil
}

func migrateSensitiveWordPolicy() error {
	var policy SensitiveWordPolicy
	err := DB.First(&policy, 1).Error
	if err == nil {
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	policy = defaultSensitiveWordPolicy()
	var legacy Option
	if err := DB.Where(&Option{Key: "SensitiveWordConfig"}).First(&legacy).Error; err == nil {
		var config legacySensitiveWordConfig
		if err := common.UnmarshalJsonStr(legacy.Value, &config); err != nil {
			return fmt.Errorf("decode legacy sensitive-word policy: %w", err)
		}
		policy.Enabled = config.Enabled
		// The first P-30 schema had no check_prompt field. Keep the new
		// policy default in that case; a present false remains an explicit
		// administrator choice from later versions.
		if config.CheckPrompt != nil {
			policy.CheckPrompt = *config.CheckPrompt
		}
		policy.AutoBanEnabled = config.AutoBanEnabled
		policy.RetainFullPrompt = config.AuditEnabled
		policy.BlockMessage = config.BlockMessage
		policy.BanThreshold = config.BanThreshold
		policy.FullPromptRetentionDays = config.FullPromptRetentionDays
		policy.MaxPromptRunes = config.MaxPromptRunes
		policy.Version = config.RuleVersion
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	policy = normalizeSensitiveWordPolicy(policy)
	return DB.Create(&policy).Error
}

func migrateSensitiveWordRules() error {
	var rules []SensitiveWordRule
	if err := DB.Find(&rules).Error; err != nil {
		return err
	}
	for _, rule := range rules {
		mode := migrateSensitiveWordRuleMode(rule.Mode, rule.Enabled)
		updates := make(map[string]any)
		if rule.Mode != mode || rule.Enabled != (mode != SensitiveWordModeOff) {
			updates["mode"] = mode
			updates["enabled"] = mode != SensitiveWordModeOff
		}
		if strings.TrimSpace(rule.Name) == "" {
			updates["name"] = fmt.Sprintf("迁移规则 #%d", rule.ID)
		}
		if rule.Version <= 0 {
			updates["version"] = 1
		}
		if len(updates) > 0 {
			if err := DB.Model(&SensitiveWordRule{}).Where("id = ?", rule.ID).Updates(updates).Error; err != nil {
				return err
			}
		}

		var count int64
		if err := DB.Model(&SensitiveWordRuleWord{}).Where("rule_id = ?", rule.ID).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 || strings.TrimSpace(rule.Word) == "" {
			continue
		}
		words := normalizeLegacySensitiveWords(strings.SplitSeq(rule.Word, "\n"))
		if len(words) == 0 {
			continue
		}
		if err := DB.Transaction(func(tx *gorm.DB) error {
			for _, word := range words {
				item := SensitiveWordRuleWord{
					RuleID: rule.ID, Word: word,
					NormalizedHash: sensitiveWordHash(word), CreatedAt: time.Now(),
				}
				if err := tx.Create(&item).Error; err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}

func migrateLegacySensitiveWordOption() error {
	var legacy Option
	err := DB.Where(&Option{Key: "SensitiveWords"}).First(&legacy).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	words := normalizeLegacySensitiveWords(strings.SplitSeq(legacy.Value, "\n"))
	if len(words) == 0 {
		return nil
	}
	var importedCount int64
	if err := DB.Model(&SensitiveWordRule{}).
		Where("name = ? AND scope = ? AND created_by = ?", "旧配置导入", SensitiveWordScopeGlobal, 0).
		Count(&importedCount).Error; err != nil {
		return err
	}
	if importedCount == 0 {
		if _, err := UpsertSensitiveWordRuleWithMode(
			0, "旧配置导入", words, SensitiveWordScopeGlobal, nil, 0, SensitiveWordModeBlock,
		); err != nil {
			return err
		}
	}
	return nil
}
