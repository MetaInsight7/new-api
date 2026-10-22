package model

import (
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// OpsUserLogStatsRow is the pre-aggregated per-user consume-log statistics that
// back GetOpsUserLogStats. The alternative — scanning the whole logs history
// per user on every report rebuild — full-scans a ~45M-row table on prod
// (measured 100s+), so the report now reads this small table instead.
//
// The table lives in LOG_DB next to logs (a separate database when
// LOG_SQL_DSN is set) and is maintained by StartOpsUserLogStatsSyncTask:
// an incremental pass aggregates every consume log newer than the stored
// cursor and upserts per-user rows. The first run backfills from the whole
// logs history (one-time, in the background, never on the request path).
//
// Semantics mirror GetOpsUserLogStats exactly:
//   - playground = token_name LIKE 'playground%' (auto-fired onboarding call)
//   - api key    = token_id > 0 AND NOT playground (opsExternalAPIKeyLogPredicate)
//
// Deleting old logs (DeleteOldLog) does not retroactively shrink these rows:
// the cursor only moves forward, and first_* values were captured from the
// earliest matching log seen so far.

type OpsUserLogStatsRow struct {
	UserId            int   `gorm:"column:user_id;primaryKey;autoIncrement:false"`
	FirstPlaygroundAt int64 `gorm:"column:first_playground_at;default:0"`
	PlaygroundCount   int   `gorm:"column:playground_count;default:0"`
	FirstApiKeyAt     int64 `gorm:"column:first_api_key_at;default:0"`
	ApiKeyCount       int   `gorm:"column:api_key_count;default:0"`
	LastRequestAt     int64 `gorm:"column:last_request_at;default:0"`
	UpdatedAt         int64 `gorm:"column:updated_at;default:0"`
}

func (OpsUserLogStatsRow) TableName() string {
	return "ops_user_log_stats"
}

// OpsUserLogStatsMeta is the single-row sync cursor for the aggregation task:
// LastLogId is the highest logs.id already folded into ops_user_log_stats.
// Backfilled flips true only after the first full pass has caught up with the
// log tail — until then the report must keep using the slow direct scan rather
// than reading a half-populated table.
type OpsUserLogStatsMeta struct {
	Id           int   `gorm:"primaryKey;autoIncrement:false"` // always 1
	LastLogId    int64 `gorm:"column:last_log_id;default:0"`
	PendingLogId int64 `gorm:"column:pending_log_id;default:0"`
	PendingSince int64 `gorm:"column:pending_since;default:0"`
	Backfilled   bool  `gorm:"column:backfilled;default:false"`
	UpdatedAt    int64 `gorm:"column:updated_at;default:0"`
}

func (OpsUserLogStatsMeta) TableName() string {
	return "ops_user_log_stats_meta"
}

const (
	opsUserLogStatsSyncBatch = 50000
	opsUserLogStatsSyncEvery = 5 * time.Minute
	opsUserLogStatsGapWait   = 5 * time.Minute

	// Each stats row uses seven bind parameters. Keeping inserts at 500 rows
	// stays well below MySQL and PostgreSQL's 65,535-parameter protocol limit.
	opsUserLogStatsUpsertChunk = 500
)

// getOpsUserLogStatsMeta reads the single cursor row, creating it on first use.
func getOpsUserLogStatsMeta() (*OpsUserLogStatsMeta, error) {
	var meta OpsUserLogStatsMeta
	err := LOG_DB.Where("id = 1").First(&meta).Error
	if err == nil {
		return &meta, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, err
	}
	meta = OpsUserLogStatsMeta{Id: 1}
	if err := LOG_DB.Create(&meta).Error; err != nil {
		return nil, err
	}
	return &meta, nil
}

// opsUserLogStatsReady reports whether the aggregate table is fully usable:
// the cursor has advanced past the first backfill (a full pass that caught up
// with the log tail). Single-row primary-key read, cheap.
func opsUserLogStatsReady() bool {
	if common.UsingLogDatabase(common.DatabaseTypeClickHouse) {
		return false
	}
	meta, err := getOpsUserLogStatsMeta()
	return err == nil && meta.Backfilled
}

// SyncOpsUserLogStats runs one incremental aggregation pass. It scans every log
// ID so an invisible lower ID cannot be skipped when a higher transaction
// commits first, while only consume logs contribute to the aggregate rows.
func SyncOpsUserLogStats() error {
	meta, err := getOpsUserLogStatsMeta()
	if err != nil {
		return err
	}
	cursor := meta.LastLogId
	pendingLogId := meta.PendingLogId
	pendingSince := meta.PendingSince
	if pendingLogId <= cursor {
		pendingLogId = 0
		pendingSince = 0
	}
	reachedTail := false
	for {
		var logs []*Log
		query := LOG_DB.Select("id", "user_id", "created_at", "type", "token_name", "token_id").
			Where("id > ?", cursor)
		boundedByPendingGap := pendingLogId > cursor
		if boundedByPendingGap {
			query = query.Where("id <= ?", pendingLogId)
		}
		if err := query.
			Order("id").
			Limit(opsUserLogStatsSyncBatch).
			Find(&logs).Error; err != nil {
			return err
		}
		if len(logs) == 0 {
			if boundedByPendingGap {
				now := common.GetTimestamp()
				if pendingSince == 0 || now-pendingSince < int64(opsUserLogStatsGapWait/time.Second) {
					return nil
				}
				cursor = pendingLogId
				pendingLogId = 0
				pendingSince = 0
				if err := LOG_DB.Model(&OpsUserLogStatsMeta{}).Where("id = 1").Updates(map[string]interface{}{
					"last_log_id":    cursor,
					"pending_log_id": 0,
					"pending_since":  0,
					"updated_at":     now,
				}).Error; err != nil {
					return err
				}
				continue
			}
			reachedTail = true
			break
		}

		hasIdGap := int64(logs[0].Id) != cursor+1
		for index := 1; !hasIdGap && index < len(logs); index++ {
			hasIdGap = logs[index].Id != logs[index-1].Id+1
		}
		if hasIdGap {
			now := common.GetTimestamp()
			if pendingLogId == 0 {
				pendingLogId = int64(logs[len(logs)-1].Id)
				pendingSince = now
				return LOG_DB.Model(&OpsUserLogStatsMeta{}).Where("id = 1").Updates(map[string]interface{}{
					"pending_log_id": pendingLogId,
					"pending_since":  pendingSince,
					"updated_at":     now,
				}).Error
			}
			if pendingSince == 0 {
				pendingSince = now
				return LOG_DB.Model(&OpsUserLogStatsMeta{}).Where("id = 1").Updates(map[string]interface{}{
					"pending_since": pendingSince,
					"updated_at":    now,
				}).Error
			}
			if now-pendingSince < int64(opsUserLogStatsGapWait/time.Second) {
				return nil
			}
		}

		agg := map[int]*OpsUserLogStatsRow{}
		for _, l := range logs {
			if l.Type != LogTypeConsume {
				continue
			}
			row, ok := agg[l.UserId]
			if !ok {
				row = &OpsUserLogStatsRow{UserId: l.UserId}
				agg[l.UserId] = row
			}
			if strings.HasPrefix(strings.ToLower(l.TokenName), "playground") {
				row.PlaygroundCount++
				if row.FirstPlaygroundAt == 0 || l.CreatedAt < row.FirstPlaygroundAt {
					row.FirstPlaygroundAt = l.CreatedAt
				}
			} else if l.TokenId > 0 {
				row.ApiKeyCount++
				if row.FirstApiKeyAt == 0 || l.CreatedAt < row.FirstApiKeyAt {
					row.FirstApiKeyAt = l.CreatedAt
				}
			}
			if l.CreatedAt > row.LastRequestAt {
				row.LastRequestAt = l.CreatedAt
			}
		}

		// Merge with the rows already in the table (the same user can appear
		// across batches): counts accumulate, first_* keep the earliest
		// non-zero value, last_request_at takes the max. Then overwrite the
		// whole row so the upsert is portable across SQLite/MySQL/PostgreSQL.
		ids := make([]int, 0, len(agg))
		for uid := range agg {
			ids = append(ids, uid)
		}
		var existing []OpsUserLogStatsRow
		for _, chunk := range chunkInts(ids, opsReportChunkSize) {
			var batch []OpsUserLogStatsRow
			if err := LOG_DB.Where("user_id IN ?", chunk).Find(&batch).Error; err != nil {
				return err
			}
			existing = append(existing, batch...)
		}
		byId := map[int]*OpsUserLogStatsRow{}
		for i := range existing {
			byId[existing[i].UserId] = &existing[i]
		}
		now := common.GetTimestamp()
		rows := make([]OpsUserLogStatsRow, 0, len(agg))
		for _, row := range agg {
			if old, ok := byId[row.UserId]; ok {
				row.PlaygroundCount += old.PlaygroundCount
				row.ApiKeyCount += old.ApiKeyCount
				if old.FirstPlaygroundAt > 0 && (row.FirstPlaygroundAt == 0 || old.FirstPlaygroundAt < row.FirstPlaygroundAt) {
					row.FirstPlaygroundAt = old.FirstPlaygroundAt
				}
				if old.FirstApiKeyAt > 0 && (row.FirstApiKeyAt == 0 || old.FirstApiKeyAt < row.FirstApiKeyAt) {
					row.FirstApiKeyAt = old.FirstApiKeyAt
				}
				if old.LastRequestAt > row.LastRequestAt {
					row.LastRequestAt = old.LastRequestAt
				}
			}
			row.UpdatedAt = now
			rows = append(rows, *row)
		}
		cursor = int64(logs[len(logs)-1].Id)
		nextPendingLogId := pendingLogId
		nextPendingSince := pendingSince
		if nextPendingLogId > 0 && cursor >= nextPendingLogId {
			nextPendingLogId = 0
			nextPendingSince = 0
		}
		// Persist stats and cursor atomically. If either write fails, the next
		// pass must replay the batch without double-counting it.
		if err := LOG_DB.Transaction(func(tx *gorm.DB) error {
			for start := 0; start < len(rows); start += opsUserLogStatsUpsertChunk {
				end := min(start+opsUserLogStatsUpsertChunk, len(rows))
				if err := tx.Clauses(clause.OnConflict{
					Columns: []clause.Column{{Name: "user_id"}},
					DoUpdates: clause.AssignmentColumns([]string{
						"first_playground_at", "playground_count", "first_api_key_at",
						"api_key_count", "last_request_at", "updated_at",
					}),
				}).Create(rows[start:end]).Error; err != nil {
					return err
				}
			}
			return tx.Model(&OpsUserLogStatsMeta{}).Where("id = 1").Updates(map[string]interface{}{
				"last_log_id":    cursor,
				"pending_log_id": nextPendingLogId,
				"pending_since":  nextPendingSince,
				"updated_at":     now,
			}).Error
		}); err != nil {
			return err
		}
		pendingLogId = nextPendingLogId
		pendingSince = nextPendingSince
		if len(logs) < opsUserLogStatsSyncBatch {
			if boundedByPendingGap {
				continue
			}
			reachedTail = true
			break
		}
	}
	// Only a pass that actually reached the log tail completes the backfill;
	// a partial first pass must not mark the table ready.
	if reachedTail && !meta.Backfilled {
		if err := LOG_DB.Model(&OpsUserLogStatsMeta{}).Where("id = 1").
			Update("backfilled", true).Error; err != nil {
			return err
		}
	}
	return nil
}

var opsUserLogStatsTaskOnce sync.Once

// StartOpsUserLogStatsSyncTask runs the incremental aggregation every
// opsUserLogStatsSyncEvery on the master node (single writer per Rule 11; the
// report reads are multi-node safe because the table is shared and upserts are
// idempotent). The first pass is a full backfill, so the aggregate table
// becomes available within one batch cycle of startup.
func StartOpsUserLogStatsSyncTask() {
	if !common.IsMasterNode || common.UsingLogDatabase(common.DatabaseTypeClickHouse) {
		return
	}
	opsUserLogStatsTaskOnce.Do(func() {
		go func() {
			for {
				if err := SyncOpsUserLogStats(); err != nil {
					common.SysError("ops user log stats sync: " + err.Error())
				}
				time.Sleep(opsUserLogStatsSyncEvery)
			}
		}()
	})
}
