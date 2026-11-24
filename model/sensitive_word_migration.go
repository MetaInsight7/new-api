package model

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"iter"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	sensitiveWordMigrationMySQLLockName  = "new-api:sensitive-word-migration:v2"
	sensitiveWordMigrationPostgresLockID = int64(0x4e41504953574d47)
	sensitiveWordMigrationLockWait       = 5 * time.Minute
	sensitiveWordMigrationLockPoll       = 50 * time.Millisecond
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

// withSensitiveWordMigrationLease pins one database session for the whole
// migration. Server databases use session advisory locks, which disappear if
// the process or connection dies. SQLite holds one immediate transaction, so
// another process cannot enter the callback until the owner commits or exits.
func withSensitiveWordMigrationLease(db *gorm.DB, migrate func(*gorm.DB) error) error {
	if db == nil {
		return nil
	}
	return db.Connection(func(connection *gorm.DB) error {
		connPool := connection.Statement.ConnPool
		migrationDB := connection.Session(&gorm.Session{NewDB: true})
		switch connection.Dialector.Name() {
		case "mysql":
			var acquired sql.NullInt64
			if err := connPool.QueryRowContext(
				connection.Statement.Context,
				"SELECT GET_LOCK(?, ?)",
				sensitiveWordMigrationMySQLLockName,
				int(sensitiveWordMigrationLockWait/time.Second),
			).Scan(&acquired); err != nil {
				return err
			}
			if !acquired.Valid || acquired.Int64 != 1 {
				return fmt.Errorf("timed out acquiring sensitive-word migration lease")
			}
			defer func() {
				var released sql.NullInt64
				releaseErr := connPool.QueryRowContext(
					context.Background(),
					"SELECT RELEASE_LOCK(?)",
					sensitiveWordMigrationMySQLLockName,
				).Scan(&released)
				if releaseErr != nil {
					common.SysError(common.LogText("failed to release sensitive-word migration lease: %s", releaseErr.Error()))
				} else if !released.Valid || released.Int64 != 1 {
					common.SysError(common.LogText("failed to release sensitive-word migration lease: lock was not owned by this database session"))
				}
			}()
			return migrate(migrationDB)
		case "postgres":
			var acquired any
			if err := connPool.QueryRowContext(
				connection.Statement.Context,
				"SELECT pg_advisory_lock($1)",
				sensitiveWordMigrationPostgresLockID,
			).Scan(&acquired); err != nil {
				return err
			}
			defer func() {
				var released bool
				releaseErr := connPool.QueryRowContext(
					context.Background(),
					"SELECT pg_advisory_unlock($1)",
					sensitiveWordMigrationPostgresLockID,
				).Scan(&released)
				if releaseErr != nil {
					common.SysError(common.LogText("failed to release sensitive-word migration lease: %s", releaseErr.Error()))
				} else if !released {
					common.SysError(common.LogText("failed to release sensitive-word migration lease: lock was not owned by this database session"))
				}
			}()
			return migrate(migrationDB)
		case "sqlite":
			for {
				beginErr := connection.Exec("BEGIN IMMEDIATE").Error
				if beginErr == nil {
					break
				}
				message := strings.ToLower(beginErr.Error())
				if !strings.Contains(message, "database is locked") && !strings.Contains(message, "database table is locked") {
					return beginErr
				}
				time.Sleep(sensitiveWordMigrationLockPoll)
			}
			committed := false
			defer func() {
				if !committed {
					_ = connection.Exec("ROLLBACK").Error
				}
			}()
			sqliteMigrationDB := connection.Session(&gorm.Session{NewDB: true, SkipDefaultTransaction: true})
			if err := migrate(sqliteMigrationDB); err != nil {
				return err
			}
			if err := connection.Exec("COMMIT").Error; err != nil {
				return err
			}
			committed = true
			return nil
		default:
			return fmt.Errorf("unsupported database dialect %q for sensitive-word migration lease", connection.Dialector.Name())
		}
	})
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
		completed = true
		return nil
	}
	err = withSensitiveWordMigrationLease(DB, func(db *gorm.DB) error {
		if migrationErr := prepareSensitiveWordAuditIdentityMigration(db); migrationErr != nil {
			return migrationErr
		}
		var marker Option
		markerErr := db.Where(&Option{Key: sensitiveWordMigrationKey}).First(&marker).Error
		if markerErr == nil && marker.Value == sensitiveWordMigrationValue {
			var policy SensitiveWordPolicy
			policyErr := db.First(&policy, 1).Error
			if errors.Is(policyErr, gorm.ErrRecordNotFound) {
				policy = defaultSensitiveWordPolicy()
				policyErr = db.Create(&policy).Error
			}
			completed = policyErr == nil
			return policyErr
		}
		if markerErr != nil && !errors.Is(markerErr, gorm.ErrRecordNotFound) {
			return markerErr
		}

		if migrationErr := migrateSensitiveWordPolicy(db); migrationErr != nil {
			return migrationErr
		}
		if migrationErr := migrateSensitiveWordRules(db); migrationErr != nil {
			return migrationErr
		}
		if migrationErr := migrateLegacySensitiveWordOption(db); migrationErr != nil {
			return migrationErr
		}
		if markerErr := db.Save(&Option{Key: sensitiveWordMigrationKey, Value: sensitiveWordMigrationValue}).Error; markerErr != nil {
			return markerErr
		}
		completed = true
		return nil
	})
	return err
}

func migrateSensitiveWordPolicy(db *gorm.DB) error {
	var policy SensitiveWordPolicy
	err := db.First(&policy, 1).Error
	if err == nil {
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	policy = defaultSensitiveWordPolicy()
	var legacy Option
	if err := db.Where(&Option{Key: "SensitiveWordConfig"}).First(&legacy).Error; err == nil {
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
	return db.Create(&policy).Error
}

func migrateSensitiveWordRules(db *gorm.DB) error {
	var rules []SensitiveWordRule
	if err := db.Find(&rules).Error; err != nil {
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
			if err := db.Model(&SensitiveWordRule{}).Where("id = ?", rule.ID).Updates(updates).Error; err != nil {
				return err
			}
		}

		var count int64
		if err := db.Model(&SensitiveWordRuleWord{}).Where("rule_id = ?", rule.ID).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 || strings.TrimSpace(rule.Word) == "" {
			continue
		}
		words := normalizeLegacySensitiveWords(strings.SplitSeq(rule.Word, "\n"))
		if len(words) == 0 {
			continue
		}
		writeWords := func(tx *gorm.DB) error {
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
		}
		var err error
		if db.Dialector.Name() == "sqlite" {
			err = writeWords(db)
		} else {
			err = db.Transaction(writeWords)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func migrateLegacySensitiveWordOption(db *gorm.DB) error {
	var legacy Option
	err := db.Where(&Option{Key: "SensitiveWords"}).First(&legacy).Error
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
	if err := db.Model(&SensitiveWordRule{}).
		Where("name = ? AND scope = ? AND created_by = ?", "旧配置导入", SensitiveWordScopeGlobal, 0).
		Count(&importedCount).Error; err != nil {
		return err
	}
	if importedCount == 0 {
		now := time.Now()
		rule := SensitiveWordRule{
			Name: "旧配置导入", Word: words[0], Scope: SensitiveWordScopeGlobal,
			Mode: SensitiveWordModeBlock, Enabled: true, CreatedBy: 0, Version: 1,
			CreatedAt: now, UpdatedAt: now,
		}
		insertRule := func(tx *gorm.DB) error {
			if err := tx.Create(&rule).Error; err != nil {
				return err
			}
			for _, word := range words {
				item := SensitiveWordRuleWord{
					RuleID: rule.ID, Word: word,
					NormalizedHash: sensitiveWordHash(word), CreatedAt: now,
				}
				if err := tx.Create(&item).Error; err != nil {
					return err
				}
			}
			return bumpSensitiveWordPolicyVersion(tx, 0)
		}
		if db.Dialector.Name() == "sqlite" {
			err = insertRule(db)
		} else {
			err = db.Transaction(insertRule)
		}
		if err != nil {
			return err
		}
	}
	return nil
}
