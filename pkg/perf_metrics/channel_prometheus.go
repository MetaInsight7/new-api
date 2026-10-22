package perfmetrics

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/perf_metrics_setting"
)

const (
	prometheusMaxSeriesPerScrapeEnv = "PROMETHEUS_MAX_SERIES_PER_SCRAPE"
	defaultPrometheusMaxSeries      = 5000
	prometheusSeriesIdleRetention   = 6 * time.Hour
)

var channelDurationBuckets = []float64{0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60, 120}

type channelAttemptKey struct {
	status        string
	errorCategory string
}

type channelMetricBucket struct {
	mu              sync.Mutex
	channelName     string
	attempts        map[channelAttemptKey]int64
	durationBuckets []int64
	durationSum     float64
	durationCount   int64
	ttftSum         float64
	ttftCount       int64
	lastUpdated     time.Time
}

type channelMetricSnapshot struct {
	channelID       int
	channelName     string
	attempts        map[channelAttemptKey]int64
	durationBuckets []int64
	durationSum     float64
	durationCount   int64
	ttftSum         float64
	ttftCount       int64
}

type channelModelKey struct {
	channelID int
	model     string
}

type channelModelBucket struct {
	mu           sync.Mutex
	attempts     map[string]int64
	inputTokens  int64
	outputTokens int64
	lastUpdated  time.Time
}

type channelModelSnapshot struct {
	channelID    int
	model        string
	attempts     map[string]int64
	inputTokens  int64
	outputTokens int64
}

var channelMetricBuckets sync.Map
var channelModelBuckets sync.Map

// RecordChannelAttempt records one real upstream attempt, including attempts
// that are later retried. Labels deliberately contain only stable channel and
// error categories; request IDs, users, and raw upstream messages are excluded.
func RecordChannelAttempt(info *relaycommon.RelayInfo, channelID int, channelName string, startedAt time.Time, relayErr *types.NewAPIError) {
	if !perf_metrics_setting.GetSetting().Enabled || info == nil || channelID <= 0 {
		return
	}
	now := time.Now()
	duration := 0.0
	if !startedAt.IsZero() && now.After(startedAt) {
		duration = now.Sub(startedAt).Seconds()
	}
	status, category := classifyChannelAttemptError(relayErr)
	bucket := getChannelMetricBucket(channelID)
	bucket.mu.Lock()
	bucket.channelName = channelName
	bucket.attempts[channelAttemptKey{status: status, errorCategory: category}]++
	bucket.durationSum += duration
	bucket.durationCount++
	for index, upperBound := range channelDurationBuckets {
		if duration <= upperBound {
			bucket.durationBuckets[index]++
		}
	}
	if info.IsStream && info.HasSendResponse() && info.FirstResponseTime.After(startedAt) {
		ttft := info.FirstResponseTime.Sub(startedAt).Seconds()
		if ttft >= 0 {
			bucket.ttftSum += ttft
			bucket.ttftCount++
		}
	}
	bucket.lastUpdated = now
	bucket.mu.Unlock()
	recordChannelModelAttempt(channelID, info.OriginModelName, status)
}

// RecordChannelTokens records settled token totals separately from attempt
// counters. It is intentionally limited to channel and model labels.
func RecordChannelTokens(info *relaycommon.RelayInfo, inputTokens, outputTokens int64) {
	if !perf_metrics_setting.GetSetting().Enabled || info == nil || info.ChannelId <= 0 || info.OriginModelName == "" {
		return
	}
	if inputTokens <= 0 && outputTokens <= 0 {
		return
	}
	key := channelModelKey{channelID: info.ChannelId, model: info.OriginModelName}
	value, _ := channelModelBuckets.LoadOrStore(key, &channelModelBucket{attempts: make(map[string]int64)})
	bucket := value.(*channelModelBucket)
	bucket.mu.Lock()
	bucket.inputTokens += max(inputTokens, 0)
	bucket.outputTokens += max(outputTokens, 0)
	bucket.lastUpdated = time.Now()
	bucket.mu.Unlock()
}

func getChannelMetricBucket(channelID int) *channelMetricBucket {
	if value, ok := channelMetricBuckets.Load(channelID); ok {
		return value.(*channelMetricBucket)
	}
	candidate := &channelMetricBucket{
		attempts:        make(map[channelAttemptKey]int64),
		durationBuckets: make([]int64, len(channelDurationBuckets)),
	}
	actual, _ := channelMetricBuckets.LoadOrStore(channelID, candidate)
	return actual.(*channelMetricBucket)
}

func recordChannelModelAttempt(channelID int, model, status string) {
	if channelID <= 0 || model == "" {
		return
	}
	key := channelModelKey{channelID: channelID, model: model}
	value, _ := channelModelBuckets.LoadOrStore(key, &channelModelBucket{attempts: make(map[string]int64)})
	bucket := value.(*channelModelBucket)
	bucket.mu.Lock()
	bucket.attempts[status]++
	bucket.lastUpdated = time.Now()
	bucket.mu.Unlock()
}

func classifyChannelAttemptError(relayErr *types.NewAPIError) (string, string) {
	if relayErr == nil {
		return "success", "none"
	}
	message := strings.ToLower(relayErr.Error())
	if errors.Is(relayErr, context.Canceled) {
		return "error", "client_cancel"
	}
	if errors.Is(relayErr, context.DeadlineExceeded) || relayErr.StatusCode == http.StatusRequestTimeout || relayErr.StatusCode == http.StatusGatewayTimeout {
		return "error", "timeout"
	}
	if strings.Contains(message, "context deadline exceeded") || strings.Contains(message, "timeout") {
		return "error", "timeout"
	}
	if relayErr.StatusCode == http.StatusTooManyRequests {
		return "error", "rate_limit"
	}
	if relayErr.StatusCode == http.StatusUnauthorized || relayErr.StatusCode == http.StatusForbidden {
		return "error", "auth"
	}
	var networkError net.Error
	if errors.As(relayErr, &networkError) {
		if networkError.Timeout() {
			return "error", "timeout"
		}
		return "error", "network"
	}
	if relayErr.StatusCode >= 400 && relayErr.StatusCode < 500 {
		return "error", "upstream_4xx"
	}
	if relayErr.StatusCode >= 500 && relayErr.StatusCode <= 599 {
		return "error", "upstream_5xx"
	}
	if strings.Contains(message, "dial ") || strings.Contains(message, "connection") || strings.Contains(message, "timeout") {
		return "error", "network"
	}
	return "error", "other"
}

// BuildPrometheusText returns the current in-memory channel metrics snapshot.
// Idle channels are removed to keep long-running instances from accumulating
// series forever. The series limit is a hard safety bound for scrapes.
func BuildPrometheusText(ctx context.Context) (string, error) {
	if ctx != nil {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}
	}
	now := time.Now()
	cutoff := now.Add(-prometheusSeriesIdleRetention)
	snapshots := make([]channelMetricSnapshot, 0)
	channelMetricBuckets.Range(func(key, value any) bool {
		channelID := key.(int)
		bucket := value.(*channelMetricBucket)
		bucket.mu.Lock()
		if bucket.lastUpdated.Before(cutoff) {
			bucket.mu.Unlock()
			channelMetricBuckets.CompareAndDelete(channelID, value)
			return true
		}
		snapshot := channelMetricSnapshot{
			channelID:       channelID,
			channelName:     bucket.channelName,
			attempts:        make(map[channelAttemptKey]int64, len(bucket.attempts)),
			durationBuckets: append([]int64(nil), bucket.durationBuckets...),
			durationSum:     bucket.durationSum,
			durationCount:   bucket.durationCount,
			ttftSum:         bucket.ttftSum,
			ttftCount:       bucket.ttftCount,
		}
		for attempt, count := range bucket.attempts {
			snapshot.attempts[attempt] = count
		}
		bucket.mu.Unlock()
		snapshots = append(snapshots, snapshot)
		return true
	})
	sort.Slice(snapshots, func(i, j int) bool { return snapshots[i].channelID < snapshots[j].channelID })
	modelSnapshots := snapshotChannelModels(cutoff)
	requestSnapshots := snapshotModelRequests(cutoff)

	series := len(requestSnapshots)
	for _, snapshot := range snapshots {
		series += 1 + len(snapshot.attempts) + len(channelDurationBuckets) + 3
		if snapshot.ttftCount > 0 {
			series += 2
		}
	}
	for _, snapshot := range modelSnapshots {
		series += len(snapshot.attempts) + 2
		if snapshot.inputTokens == 0 {
			series--
		}
		if snapshot.outputTokens == 0 {
			series--
		}
	}
	maxSeries := common.GetEnvOrDefault(prometheusMaxSeriesPerScrapeEnv, defaultPrometheusMaxSeries)
	if maxSeries > 0 && series > maxSeries {
		return "", fmt.Errorf("prometheus series limit exceeded: %d > %d", series, maxSeries)
	}
	includeModelMetadata := maxSeries <= 0 || series+5 <= maxSeries
	remaining := -1
	if maxSeries > 0 {
		remaining = max(0, maxSeries-series-5)
	}
	performanceSnapshots, activeModels, dropped := snapshotModelPerformance(now, remaining)
	if includeModelMetadata {
		series += 5
		for _, snapshot := range performanceSnapshots {
			series += snapshot.seriesCount()
		}
	}

	var output strings.Builder
	fmt.Fprintf(&output, "# HELP newapi_perf_metrics_series Number of application metric series exposed by this endpoint.\n# TYPE newapi_perf_metrics_series gauge\nnewapi_perf_metrics_series %d\n", series)
	output.WriteString("# HELP newapi_model_requests_total Total model requests by model and status.\n# TYPE newapi_model_requests_total counter\n")
	for _, snapshot := range requestSnapshots {
		fmt.Fprintf(&output, "newapi_model_requests_total{model=\"%s\",status=\"%s\"} %d\n", escapePrometheusLabel(snapshot.model), snapshot.status, snapshot.count)
	}
	if includeModelMetadata {
		fmt.Fprintf(&output, "# HELP newapi_model_histogram_active_models Current admitted model performance groups.\n# TYPE newapi_model_histogram_active_models gauge\nnewapi_model_histogram_active_models %d\n", activeModels)
		output.WriteString("# HELP newapi_model_histogram_dropped_samples_total Model observations dropped by reason.\n# TYPE newapi_model_histogram_dropped_samples_total counter\n")
		for _, reason := range modelDropReasons {
			fmt.Fprintf(&output, "newapi_model_histogram_dropped_samples_total{reason=\"%s\"} %d\n", reason, dropped[reason])
		}
		output.WriteString("# HELP newapi_model_errors_total Total final model errors by category.\n# TYPE newapi_model_errors_total counter\n")
		output.WriteString("# HELP newapi_model_request_duration_seconds Duration of successful final model operations.\n# TYPE newapi_model_request_duration_seconds histogram\n")
		output.WriteString("# HELP newapi_model_ttft_seconds Time to first response of successful streaming model operations.\n# TYPE newapi_model_ttft_seconds histogram\n")
		output.WriteString("# HELP newapi_model_stream_success_total Total successful streaming model operations.\n# TYPE newapi_model_stream_success_total counter\n")
		for _, snapshot := range performanceSnapshots {
			writeModelHistogram(&output, "newapi_model_request_duration_seconds", snapshot.model, snapshot.latency, modelLatencyBounds)
			writeModelHistogram(&output, "newapi_model_ttft_seconds", snapshot.model, snapshot.ttft, modelTTFTBounds)
			if snapshot.streamSuccess > 0 {
				fmt.Fprintf(&output, "newapi_model_stream_success_total{model=\"%s\"} %d\n", escapePrometheusLabel(snapshot.model), snapshot.streamSuccess)
			}
			categories := make([]string, 0, len(snapshot.errors))
			for category := range snapshot.errors {
				categories = append(categories, category)
			}
			sort.Strings(categories)
			for _, category := range categories {
				fmt.Fprintf(&output, "newapi_model_errors_total{model=\"%s\",error_category=\"%s\"} %d\n", escapePrometheusLabel(snapshot.model), category, snapshot.errors[category])
			}
		}
	}
	output.WriteString("# HELP newapi_channel_info Current display name for a stable channel ID.\n")
	output.WriteString("# TYPE newapi_channel_info gauge\n")
	for _, snapshot := range snapshots {
		output.WriteString("newapi_channel_info{channel_id=\"")
		output.WriteString(strconv.Itoa(snapshot.channelID))
		output.WriteString("\",channel_name=\"")
		output.WriteString(escapePrometheusLabel(snapshot.channelName))
		output.WriteString("\"} 1\n")
	}
	output.WriteString("# HELP newapi_channel_model_attempts_total Total channel attempts by model and outcome.\n")
	output.WriteString("# TYPE newapi_channel_model_attempts_total counter\n")
	for _, snapshot := range modelSnapshots {
		statuses := make([]string, 0, len(snapshot.attempts))
		for status := range snapshot.attempts {
			statuses = append(statuses, status)
		}
		sort.Strings(statuses)
		for _, status := range statuses {
			writeChannelModelLabels(&output, "newapi_channel_model_attempts_total", snapshot.channelID, snapshot.model, status)
			output.WriteByte(' ')
			output.WriteString(strconv.FormatInt(snapshot.attempts[status], 10))
			output.WriteByte('\n')
		}
		if snapshot.inputTokens > 0 {
			writeChannelModelLabels(&output, "newapi_channel_model_input_tokens_total", snapshot.channelID, snapshot.model, "")
			output.WriteByte(' ')
			output.WriteString(strconv.FormatInt(snapshot.inputTokens, 10))
			output.WriteByte('\n')
		}
		if snapshot.outputTokens > 0 {
			writeChannelModelLabels(&output, "newapi_channel_model_output_tokens_total", snapshot.channelID, snapshot.model, "")
			output.WriteByte(' ')
			output.WriteString(strconv.FormatInt(snapshot.outputTokens, 10))
			output.WriteByte('\n')
		}
	}
	output.WriteString("# HELP newapi_channel_attempts_total Total real upstream channel attempts, including retries.\n")
	output.WriteString("# TYPE newapi_channel_attempts_total counter\n")
	for _, snapshot := range snapshots {
		keys := make([]channelAttemptKey, 0, len(snapshot.attempts))
		for key := range snapshot.attempts {
			keys = append(keys, key)
		}
		sort.Slice(keys, func(i, j int) bool {
			if keys[i].status != keys[j].status {
				return keys[i].status < keys[j].status
			}
			return keys[i].errorCategory < keys[j].errorCategory
		})
		for _, key := range keys {
			writeChannelLabels(&output, snapshot.channelID, key.status, key.errorCategory)
			output.WriteByte(' ')
			output.WriteString(strconv.FormatInt(snapshot.attempts[key], 10))
			output.WriteByte('\n')
		}
	}
	output.WriteString("# HELP newapi_channel_request_duration_seconds Total duration of real upstream channel attempts.\n")
	output.WriteString("# TYPE newapi_channel_request_duration_seconds histogram\n")
	for _, snapshot := range snapshots {
		for index, upperBound := range channelDurationBuckets {
			writeChannelDuration(&output, snapshot.channelID, formatPrometheusFloat(upperBound), snapshot.durationBuckets[index])
		}
		writeChannelDuration(&output, snapshot.channelID, "+Inf", snapshot.durationCount)
		writeChannelLabelsSuffix(&output, "newapi_channel_request_duration_seconds_sum", snapshot.channelID)
		output.WriteByte(' ')
		output.WriteString(formatPrometheusFloat(snapshot.durationSum))
		output.WriteByte('\n')
		writeChannelLabelsSuffix(&output, "newapi_channel_request_duration_seconds_count", snapshot.channelID)
		output.WriteByte(' ')
		output.WriteString(strconv.FormatInt(snapshot.durationCount, 10))
		output.WriteByte('\n')
	}
	output.WriteString("# HELP newapi_channel_ttft_seconds Time to first token for streaming channel attempts.\n")
	output.WriteString("# TYPE newapi_channel_ttft_seconds summary\n")
	for _, snapshot := range snapshots {
		if snapshot.ttftCount == 0 {
			continue
		}
		writeChannelLabelsSuffix(&output, "newapi_channel_ttft_seconds_sum", snapshot.channelID)
		output.WriteByte(' ')
		output.WriteString(formatPrometheusFloat(snapshot.ttftSum))
		output.WriteByte('\n')
		writeChannelLabelsSuffix(&output, "newapi_channel_ttft_seconds_count", snapshot.channelID)
		output.WriteByte(' ')
		output.WriteString(strconv.FormatInt(snapshot.ttftCount, 10))
		output.WriteByte('\n')
	}
	return output.String(), nil
}

func snapshotChannelModels(cutoff time.Time) []channelModelSnapshot {
	snapshots := make([]channelModelSnapshot, 0)
	channelModelBuckets.Range(func(key, value any) bool {
		modelKey := key.(channelModelKey)
		bucket := value.(*channelModelBucket)
		bucket.mu.Lock()
		if bucket.lastUpdated.Before(cutoff) {
			bucket.mu.Unlock()
			channelModelBuckets.CompareAndDelete(modelKey, value)
			return true
		}
		snapshot := channelModelSnapshot{
			channelID: modelKey.channelID, model: modelKey.model,
			attempts:    make(map[string]int64, len(bucket.attempts)),
			inputTokens: bucket.inputTokens, outputTokens: bucket.outputTokens,
		}
		for status, count := range bucket.attempts {
			snapshot.attempts[status] = count
		}
		bucket.mu.Unlock()
		snapshots = append(snapshots, snapshot)
		return true
	})
	sort.Slice(snapshots, func(i, j int) bool {
		if snapshots[i].channelID != snapshots[j].channelID {
			return snapshots[i].channelID < snapshots[j].channelID
		}
		return snapshots[i].model < snapshots[j].model
	})
	return snapshots
}

func writeChannelLabels(output *strings.Builder, channelID int, status, category string) {
	output.WriteString("newapi_channel_attempts_total{channel_id=\"")
	output.WriteString(strconv.Itoa(channelID))
	output.WriteString("\",status=\"")
	output.WriteString(escapePrometheusLabel(status))
	output.WriteString("\",error_category=\"")
	output.WriteString(escapePrometheusLabel(category))
	output.WriteString("\"}")
}

func writeChannelDuration(output *strings.Builder, channelID int, upperBound string, count int64) {
	output.WriteString("newapi_channel_request_duration_seconds_bucket{channel_id=\"")
	output.WriteString(strconv.Itoa(channelID))
	output.WriteString("\",le=\"")
	output.WriteString(upperBound)
	output.WriteString("\"} ")
	output.WriteString(strconv.FormatInt(count, 10))
	output.WriteByte('\n')
}

func writeChannelLabelsSuffix(output *strings.Builder, metricName string, channelID int) {
	output.WriteString(metricName)
	output.WriteString("{channel_id=\"")
	output.WriteString(strconv.Itoa(channelID))
	output.WriteString("\"}")
}

func writeChannelModelLabels(output *strings.Builder, metricName string, channelID int, model, status string) {
	output.WriteString(metricName)
	output.WriteString("{channel_id=\"")
	output.WriteString(strconv.Itoa(channelID))
	output.WriteString("\",model=\"")
	output.WriteString(escapePrometheusLabel(model))
	output.WriteByte('"')
	if status != "" {
		output.WriteString(",status=\"")
		output.WriteString(escapePrometheusLabel(status))
		output.WriteByte('"')
	}
	output.WriteString("}")
}

func formatPrometheusFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func escapePrometheusLabel(value string) string {
	value = strings.Map(func(r rune) rune {
		if r != '\n' && unicode.IsControl(r) {
			return -1
		}
		return r
	}, value)
	if runes := []rune(value); len(runes) > 256 {
		value = string(runes[:256])
	}
	return strings.NewReplacer("\\", "\\\\", "\n", "\\n", "\"", "\\\"").Replace(value)
}
