package model

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

// opsDayBucketExpr builds a portable SQL expression that maps created_at (epoch
// seconds) to the start epoch of the report-timezone day it falls in, using the
// explicit per-day UTC boundaries computed by the controller (DST-aware). This
// replaces a single fixed tz-offset FLOOR, which mis-buckets the hour near
// midnight whenever the window spans a DST transition. dayStarts holds n+1
// ascending boundaries for n days; the constants are integers, so inlining them
// is injection-safe and works on SQLite/MySQL/PostgreSQL alike.
func opsDayBucketExpr(dayStarts []int64) string {
	var b strings.Builder
	b.WriteString("CASE")
	for i := 0; i+1 < len(dayStarts); i++ {
		fmt.Fprintf(&b, " WHEN created_at >= %d AND created_at < %d THEN %d", dayStarts[i], dayStarts[i+1], dayStarts[i])
	}
	b.WriteString(" END")
	return b.String()
}

// Ops daily report data layer. All queries are read-only aggregates over
// ordinary customer accounts; administrator accounts are excluded from the
// marketing cohort.
//
// logs may live in a separate database (LOG_DB / LOG_SQL_DSN), so nothing here
// joins logs with users in SQL — user metadata is joined in memory by the
// controller. Per-user aggregates are fetched in id chunks through the
// user_id index, which keeps every query an index lookup instead of a scan.

const opsReportChunkSize = 500
const opsExternalAPIKeyLogPredicate = "token_id > 0 AND LOWER(COALESCE(token_name, '')) NOT LIKE 'playground%'"

type OpsUser struct {
	Id             int    `json:"id"`
	Username       string `json:"username"`
	DisplayName    string `json:"display_name"`
	Email          string `json:"email"`
	CreatedAt      int64  `json:"created_at"`
	AdsAttribution string `json:"ads_attribution"`
	OauthKind      string `json:"oauth_kind"`
	Quota          int64  `json:"quota"`
	UsedQuota      int64  `json:"used_quota"`
	RequestCount   int    `json:"request_count"`
	LastLoginAt    int64  `json:"last_login_at"`
	BrowserLang    string `json:"browser_lang"`
	LastLoginIp    string `json:"last_login_ip"`
	PayCountry     string `json:"pay_country"`
}

type OpsUserSubscriptionRemaining struct {
	UserId         int   `json:"user_id"`
	RemainingQuota int64 `json:"remaining_quota"`
}

type OpsUserSubscriptionSource struct {
	UserId      int    `json:"user_id"`
	Source      string `json:"source"`
	PaymentMode string `json:"payment_mode"`
}

type OpsUserLogStats struct {
	UserId            int   `json:"user_id"`
	FirstPlaygroundAt int64 `json:"first_playground_at"`
	PlaygroundCount   int   `json:"playground_count"`
	FirstApiKeyAt     int64 `json:"first_apikey_at"`
	ApiKeyCount       int   `json:"apikey_count"`
	LastRequestAt     int64 `json:"last_request_at"`
}

type OpsUserTokenStats struct {
	UserId           int   `json:"user_id"`
	ManualTokenCount int   `json:"manual_token_count"`
	FirstManualAt    int64 `json:"first_manual_at"`
	// CliTokenCount counts tokens created via the CLI device-authorization flow
	// (source = 'cli'); those fire the activate_success event too, so they must
	// count as activation regardless of when they were created.
	CliTokenCount int `json:"cli_token_count"`
	// AutoKeyUsedQuota is the quota burned through auto-provisioned tokens
	// (created within the auto window of signup) — the ops cost of giving
	// signup credit to bot/farm registrations that never create a manual key.
	AutoKeyUsedQuota int64 `json:"auto_key_used_quota"`
}

type OpsKeyDaily struct {
	UserId   int   `json:"user_id"`
	DayTs    int64 `json:"day_ts"`
	ReqCount int   `json:"req_count"`
	Quota    int64 `json:"quota"`
}

type OpsTopUp struct {
	UserId     int     `json:"user_id"`
	Money      float64 `json:"money"`
	Status     string  `json:"status"`
	CreateTime int64   `json:"create_time"`
	// PaymentProvider distinguishes user-initiated orders from system-generated
	// stripe_auto rows (threshold auto-charges; their failed rows are mere cooldown
	// markers) so intent stats can exclude the latter.
	PaymentProvider string `json:"payment_provider"`
	// Source distinguishes wallet top-ups from subscription orders so ops can
	// trace "paid amount" back to the payment flow that created it.
	Source string `json:"source"`
}

// GetOpsUsers returns ordinary customer accounts that back the daily report's
// funnel metrics. Base uses Group for model routing and pricing, not as a
// marketing cohort marker, so the report intentionally filters by role.
// Base currently has no persisted ads attribution, browser-language, login-IP,
// or payment-country columns; those response fields remain empty until a
// separate tracking migration is approved.
// When includeDisabled is false (the default reporting denominator),
// disabled accounts — manually banned and honeypot-created — are excluded: they
// are not real signups and would inflate registration/activation/paid counts.
func GetOpsUsers(includeDisabled bool) ([]*OpsUser, error) {
	var users []*OpsUser
	query := DB.Table("users").
		Select(`id, username, display_name, email, created_at,
			'' AS ads_attribution,
			quota, used_quota, request_count, last_login_at,
			'' AS browser_lang, '' AS last_login_ip, '' AS pay_country,
			CASE WHEN github_id IS NOT NULL AND github_id <> '' THEN 'github'
			     WHEN discord_id IS NOT NULL AND discord_id <> '' THEN 'discord'
			     WHEN oidc_id IS NOT NULL AND oidc_id <> '' THEN 'oidc'
			     WHEN wechat_id IS NOT NULL AND wechat_id <> '' THEN 'wechat'
			     WHEN telegram_id IS NOT NULL AND telegram_id <> '' THEN 'telegram'
				 ELSE 'email' END AS oauth_kind`).
		Where("role = ? AND deleted_at IS NULL", common.RoleCommonUser)
	if !includeDisabled {
		query = query.Where("status = ?", common.UserStatusEnabled)
	}
	err := query.Find(&users).Error
	return users, err
}

// GetOpsUserSubscriptionRemaining returns the active subscription entitlement
// balance for the given users. tokendazi_op uses direct subscriptions rather
// than the newer contract-backed subscription schema.
func GetOpsUserSubscriptionRemaining(userIds []int) (map[int]int64, error) {
	if len(userIds) == 0 {
		return map[int]int64{}, nil
	}
	now := common.GetTimestamp()
	result := make(map[int]int64, len(userIds))

	var legacyRows []OpsUserSubscriptionRemaining
	legacySQL := `
		SELECT s.user_id AS user_id,
		       COALESCE(SUM(CASE
		         WHEN s.amount_total > s.amount_used THEN s.amount_total - s.amount_used
		         ELSE 0
		       END), 0) AS remaining_quota
		FROM user_subscriptions s
		INNER JOIN users u ON u.id = s.user_id
		WHERE s.user_id IN ?
		  AND u.deleted_at IS NULL
		  AND s.status = ?
		  AND s.end_time > ?
		GROUP BY s.user_id`
	if err := DB.Raw(legacySQL, userIds, "active", now).Scan(&legacyRows).Error; err != nil {
		return nil, err
	}
	for _, row := range legacyRows {
		result[row.UserId] = row.RemainingQuota
	}

	return result, nil
}

// GetOpsUserSubscriptionSources returns the active subscription source for each
// user from the direct subscription rows available in tokendazi_op.
func GetOpsUserSubscriptionSources(userIds []int) (map[int]OpsUserSubscriptionSource, error) {
	if len(userIds) == 0 {
		return map[int]OpsUserSubscriptionSource{}, nil
	}
	now := common.GetTimestamp()
	result := make(map[int]OpsUserSubscriptionSource, len(userIds))

	var legacyRows []OpsUserSubscriptionSource
	legacySQL := `
		SELECT s.user_id AS user_id,
		       COALESCE(NULLIF(TRIM(s.source), ''), '') AS source,
		       '' AS payment_mode
		FROM user_subscriptions s
		INNER JOIN users u ON u.id = s.user_id
		WHERE s.user_id IN ?
		  AND u.deleted_at IS NULL
		  AND s.status = ?
		  AND s.end_time > ?
		ORDER BY s.end_time ASC, s.id ASC`
	if err := DB.Raw(legacySQL, userIds, "active", now).Scan(&legacyRows).Error; err != nil {
		return nil, err
	}
	for _, row := range legacyRows {
		if _, ok := result[row.UserId]; !ok {
			result[row.UserId] = row
		}
	}

	return result, nil
}

// logsForceIndexHint keeps the optimizer on the user_id index; with large IN
// lists MySQL has been observed to fall back to a full scan of the logs table.
// Decided by LOG_DB's own dialect, not the main DB's: with LOG_SQL_DSN the two
// can differ, and this MySQL-only hint is a syntax error elsewhere.
func logsForceIndexHint() string {
	if LOG_DB != nil && LOG_DB.Dialector.Name() == "mysql" {
		return " FORCE INDEX (idx_logs_user_id)"
	}
	return ""
}

func chunkInts(ids []int, size int) [][]int {
	var chunks [][]int
	for i := 0; i < len(ids); i += size {
		end := i + size
		if end > len(ids) {
			end = len(ids)
		}
		chunks = append(chunks, ids[i:end])
	}
	return chunks
}

// GetOpsUserLogStats returns per-user playground/API-key usage aggregates.
// Prefer the ops_user_log_stats aggregate table (maintained incrementally in
// the background) so the report never scans the whole logs history; fall back
// to the direct logs scan only until the first backfill has populated the
// table (e.g. right after a fresh deploy).
func GetOpsUserLogStats(userIds []int) ([]*OpsUserLogStats, error) {
	if opsUserLogStatsReady() {
		var all []*OpsUserLogStats
		for _, chunk := range chunkInts(userIds, opsReportChunkSize) {
			var batch []*OpsUserLogStats
			if err := LOG_DB.Table("ops_user_log_stats").
				Select("user_id, first_playground_at, playground_count, first_api_key_at, api_key_count, last_request_at").
				Where("user_id IN ?", chunk).
				Scan(&batch).Error; err != nil {
				return nil, err
			}
			all = append(all, batch...)
		}
		return all, nil
	}
	return getOpsUserLogStatsFromLogs(userIds)
}

// getOpsUserLogStatsFromLogs is the original direct logs scan, kept as the
// fallback until the aggregate table is populated.
func getOpsUserLogStatsFromLogs(userIds []int) ([]*OpsUserLogStats, error) {
	var all []*OpsUserLogStats
	for _, chunk := range chunkInts(userIds, opsReportChunkSize) {
		var batch []*OpsUserLogStats
		sql := fmt.Sprintf(`
			SELECT user_id,
			       COALESCE(MIN(CASE WHEN LOWER(COALESCE(token_name, '')) LIKE 'playground%%' THEN created_at END), 0) AS first_playground_at,
			       COALESCE(SUM(CASE WHEN LOWER(COALESCE(token_name, '')) LIKE 'playground%%' THEN 1 ELSE 0 END), 0) AS playground_count,
			       COALESCE(MIN(CASE WHEN %s THEN created_at END), 0) AS first_api_key_at,
			       COALESCE(SUM(CASE WHEN %s THEN 1 ELSE 0 END), 0) AS api_key_count,
			       COALESCE(MAX(created_at), 0) AS last_request_at
			FROM logs%s
			WHERE type = ? AND user_id IN ?
			GROUP BY user_id`, opsExternalAPIKeyLogPredicate, opsExternalAPIKeyLogPredicate, logsForceIndexHint())
		if err := LOG_DB.Raw(sql, LogTypeConsume, chunk).Scan(&batch).Error; err != nil {
			return nil, err
		}
		all = append(all, batch...)
	}
	return all, nil
}

// GetOpsKeyDailyUsage returns per-user-per-day API-key request aggregates over
// the window defined by dayStarts (the "key used" DAU series source). dayStarts
// holds n+1 ascending UTC epoch boundaries — the real report-timezone midnights
// for the n days — so buckets stay correct across DST transitions. day_ts
// values are the per-day start epochs.
func GetOpsKeyDailyUsage(userIds []int, dayStarts []int64) ([]*OpsKeyDaily, error) {
	if len(dayStarts) < 2 {
		return nil, nil
	}
	dayExpr := opsDayBucketExpr(dayStarts)
	startTs := dayStarts[0]
	var all []*OpsKeyDaily
	for _, chunk := range chunkInts(userIds, opsReportChunkSize) {
		var batch []*OpsKeyDaily
		sql := fmt.Sprintf(`
			SELECT user_id,
			       %s AS day_ts,
			       COUNT(*) AS req_count,
			       COALESCE(SUM(quota), 0) AS quota
			FROM logs%s
			WHERE type = ? AND %s AND created_at >= ? AND user_id IN ?
			GROUP BY user_id, %s`, dayExpr, logsForceIndexHint(), opsExternalAPIKeyLogPredicate, dayExpr)
		if err := LOG_DB.Raw(sql, LogTypeConsume, startTs, chunk).Scan(&batch).Error; err != nil {
			return nil, err
		}
		all = append(all, batch...)
	}
	return all, nil
}

// GetOpsAllKeyDailyUsage returns day-level usage across ALL users since
// startTs, aggregated from quota_data (hourly per-user-per-model rollups,
// ~500 rows/day) instead of raw logs: a 30-day window covers nearly the whole
// logs table, so the optimizer full-scans ~45M rows there (measured 100s+ on
// prod). Trade-off: quota_data counts all consumption including playground,
// not only token_id>0 API-key calls.
func GetOpsAllKeyDailyUsage(dayStarts []int64) ([]*OpsDauDay, error) {
	if len(dayStarts) < 2 {
		return nil, nil
	}
	dayExpr := opsDayBucketExpr(dayStarts)
	startTs := dayStarts[0]
	var rows []*OpsDauDay
	err := DB.Raw(fmt.Sprintf(`
		SELECT %s AS day_ts,
		       COUNT(DISTINCT user_id) AS active_users,
		       COALESCE(SUM(count), 0) AS req_count,
		       COALESCE(SUM(quota), 0) AS quota
		FROM quota_data
		WHERE created_at >= ?
		GROUP BY %s`, dayExpr, dayExpr), startTs).Scan(&rows).Error
	return rows, err
}

type OpsDauDay struct {
	DayTs       int64 `json:"day_ts"`
	ActiveUsers int   `json:"active_users"`
	ReqCount    int   `json:"req_count"`
	Quota       int64 `json:"quota"`
}

// GetOpsUserTokenStats returns per-user counts of manually created tokens.
// Tokens created within autoWindowSec of registration are auto-provisioned by
// signup integrations (main-key/auto/default) and are excluded.
func GetOpsUserTokenStats(autoWindowSec int64) ([]*OpsUserTokenStats, error) {
	var stats []*OpsUserTokenStats
	sql := fmt.Sprintf(`
		SELECT t.user_id,
		       COALESCE(SUM(CASE WHEN t.created_time - u.created_at >= ? THEN 1 ELSE 0 END), 0) AS manual_token_count,
		       COALESCE(MIN(CASE WHEN t.created_time - u.created_at >= ? THEN t.created_time END), 0) AS first_manual_at,
		       0 AS cli_token_count,
		       COALESCE(SUM(CASE WHEN t.created_time - u.created_at < ? THEN t.used_quota ELSE 0 END), 0) AS auto_key_used_quota
		FROM tokens t
		INNER JOIN users u ON u.id = t.user_id
		WHERE u.role = ? AND u.deleted_at IS NULL
		GROUP BY t.user_id`)
	err := DB.Raw(sql, autoWindowSec, autoWindowSec, autoWindowSec, common.RoleCommonUser).Scan(&stats).Error
	return stats, err
}

// GetOpsTopUps returns all top-up orders belonging to ordinary customer users.
func GetOpsTopUps() ([]*OpsTopUp, error) {
	var topUps []*OpsTopUp
	sql := fmt.Sprintf(`
		SELECT t.user_id, t.money, t.status, t.create_time, t.payment_provider, '' AS source
		FROM top_ups t
		INNER JOIN users u ON u.id = t.user_id
		WHERE u.role = ? AND u.deleted_at IS NULL
		  AND NOT EXISTS (SELECT 1 FROM subscription_orders so WHERE so.trade_no = t.trade_no)
		ORDER BY t.create_time`)
	err := DB.Raw(sql, common.RoleCommonUser).Scan(&topUps).Error
	return topUps, err
}

// OpsSubscriptionOrder carries one customer subscription order plus the USD list
// price of its plan, so the ops report can fold plan purchases into the
// top-up-based revenue aggregates.
type OpsSubscriptionOrder struct {
	UserId          int     `json:"user_id"`
	Money           float64 `json:"money"`
	Status          string  `json:"status"`
	CreateTime      int64   `json:"create_time"`
	PaymentProvider string  `json:"payment_provider"`
	PlanUSD         float64 `json:"plan_usd"`
}

// GetOpsSubscriptionOrders returns successful paid subscription orders. The
// current base mirrors completed subscriptions into top_ups, so GetOpsTopUps
// excludes those rows and this query supplies the subscription side exactly
// once. Balance-paid orders are excluded because their funds were already
// counted when the wallet was topped up.
func GetOpsSubscriptionOrders() ([]*OpsSubscriptionOrder, error) {
	var orders []*OpsSubscriptionOrder
	sql := fmt.Sprintf(`
		SELECT o.user_id, o.money, o.status, o.create_time, o.payment_provider,
		       COALESCE(p.price_amount, 0) AS plan_usd
		FROM subscription_orders o
		INNER JOIN users u ON u.id = o.user_id
		LEFT JOIN subscription_plans p ON p.id = o.plan_id
		WHERE u.role = ? AND u.deleted_at IS NULL AND o.status = ? AND o.payment_provider <> ?
		ORDER BY o.create_time`)
	err := DB.Raw(sql, common.RoleCommonUser, common.TopUpStatusSuccess, PaymentProviderBalance).Scan(&orders).Error
	return orders, err
}

type OpsTopUpTradeUser struct {
	TradeNo string `json:"trade_no"`
	UserId  int    `json:"user_id"`
}

// GetOpsTopUpUsersByTradeNos maps checkout trade_nos back to local user ids, so the
// Stripe conversion report can attribute sessions via client_reference_id instead of
// relying on the (often missing) checkout email.
func GetOpsTopUpUsersByTradeNos(tradeNos []string) ([]*OpsTopUpTradeUser, error) {
	var all []*OpsTopUpTradeUser
	for i := 0; i < len(tradeNos); i += opsReportChunkSize {
		end := i + opsReportChunkSize
		if end > len(tradeNos) {
			end = len(tradeNos)
		}
		var batch []*OpsTopUpTradeUser
		err := DB.Table("top_ups").
			Select("trade_no, user_id").
			Where("trade_no IN ?", tradeNos[i:end]).
			Scan(&batch).Error
		if err != nil {
			return nil, err
		}
		all = append(all, batch...)
	}
	return all, nil
}

type OpsUserLastIP struct {
	UserId int    `json:"user_id"`
	Ip     string `json:"ip"`
}

// GetOpsUsersLastIP returns the most recent non-empty request IP per user,
// restricted to logs at or after since (the report window start). One indexed
// MAX(id) pass plus one primary-key lookup; used for the full plg user set
// (~thousands) by the ops report region funnel. The time bound keeps the MAX
// pass off the oldest logs, which otherwise grows without bound.
func GetOpsUsersLastIP(userIds []int, since int64) ([]*OpsUserLastIP, error) {
	if len(userIds) == 0 {
		return nil, nil
	}
	var maxIds []int64
	sql := fmt.Sprintf(`
		SELECT MAX(id) FROM logs%s
		WHERE user_id IN ? AND ip <> '' AND created_at >= ?
		GROUP BY user_id`, logsForceIndexHint())
	if err := LOG_DB.Raw(sql, userIds, since).Scan(&maxIds).Error; err != nil {
		return nil, err
	}
	if len(maxIds) == 0 {
		return nil, nil
	}
	var rows []*OpsUserLastIP
	err := LOG_DB.Raw(`SELECT user_id, ip FROM logs WHERE id IN ?`, maxIds).Scan(&rows).Error
	return rows, err
}

type OpsUserModelUsage struct {
	UserId    int    `json:"user_id"`
	ModelName string `json:"model_name"`
	Count     int    `json:"count"`
}

// GetOpsUsersModelUsage returns per-model request counts for a small user set
// (top payers, <=~20 ids), aggregated from quota_data rollups.
func GetOpsUsersModelUsage(userIds []int) ([]*OpsUserModelUsage, error) {
	if len(userIds) == 0 {
		return nil, nil
	}
	var rows []*OpsUserModelUsage
	err := DB.Raw(`
		SELECT user_id, model_name, COALESCE(SUM(count), 0) AS count
		FROM quota_data
		WHERE user_id IN ?
		GROUP BY user_id, model_name`, userIds).Scan(&rows).Error
	return rows, err
}
