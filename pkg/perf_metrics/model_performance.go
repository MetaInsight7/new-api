package perfmetrics

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"maps"
	"net"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/perf_metrics_setting"
)

const modelPerformanceIdleRetention = time.Hour

var modelDropReasons = []string{"model_limit", "invalid_latency", "invalid_ttft", "series_limit"}

var modelLatencyBounds = []float64{0.25, 0.5, 1, 2, 3, 5, 10, 15, 30, 60, 120, 300, 600}
var modelTTFTBounds = []float64{0.1, 0.25, 0.5, 1, 2, 3, 5, 10, 15, 30, 60}

type modelHistogram struct {
	buckets []int64
	sum     float64
	count   int64
}

func (histogram *modelHistogram) add(value float64, bounds []float64) {
	if histogram.buckets == nil {
		histogram.buckets = make([]int64, len(bounds))
	}
	for index, bound := range bounds {
		if value <= bound {
			histogram.buckets[index]++
		}
	}
	histogram.sum += value
	histogram.count++
}

type modelPerformanceBucket struct {
	latency         modelHistogram
	ttft            modelHistogram
	streamSuccess   int64
	errors          map[string]int64
	lastUpdated     time.Time
	reportedLatency int64
	reportedTTFT    int64
}

func (bucket modelPerformanceBucket) seriesCount() int {
	count := len(bucket.errors)
	if bucket.latency.count > 0 {
		count += len(modelLatencyBounds) + 3
	}
	if bucket.ttft.count > 0 {
		count += len(modelTTFTBounds) + 3
	}
	if bucket.streamSuccess > 0 {
		count++
	}
	return count
}

type modelPerformanceSnapshot struct {
	model string
	modelPerformanceBucket
}

var modelPerformance = struct {
	sync.Mutex
	values  map[string]*modelPerformanceBucket
	dropped map[string]int64
}{values: make(map[string]*modelPerformanceBucket), dropped: make(map[string]int64)}

func recordModelPerformance(ctx context.Context, info *relaycommon.RelayInfo, success bool, apiErr *types.NewAPIError, now time.Time) {
	if !perf_metrics_setting.GetSetting().Enabled || info == nil || info.OriginModelName == "" {
		return
	}
	category := ""
	if !success {
		category = classifyFinalModelError(ctx, info, apiErr)
	}
	modelPerformance.Lock()
	defer modelPerformance.Unlock()
	validLatency := !info.StartTime.IsZero() && info.StartTime.Before(now)
	hasFirstResponse := info.IsStream && !info.FirstResponseTime.IsZero() && !info.FirstResponseTime.Equal(info.StartTime.Add(-time.Second))
	validTTFT := hasFirstResponse && validLatency && info.FirstResponseTime.After(info.StartTime) && !info.FirstResponseTime.After(now)
	if success {
		if !validLatency {
			modelPerformance.dropped["invalid_latency"]++
		}
		if hasFirstResponse && !validTTFT {
			modelPerformance.dropped["invalid_ttft"]++
		}
	}
	bucket := modelPerformance.values[info.OriginModelName]
	if bucket == nil {
		pruneIdleModelPerformance(time.Now())
		maxModels := common.GetEnvOrDefault("PROMETHEUS_MAX_MODEL_HISTOGRAM_MODELS", 50)
		if maxModels > 0 && len(modelPerformance.values) >= maxModels {
			modelPerformance.dropped["model_limit"]++
			return
		}
		bucket = &modelPerformanceBucket{errors: make(map[string]int64)}
		modelPerformance.values[info.OriginModelName] = bucket
	}
	if success {
		if validLatency {
			bucket.latency.add(now.Sub(info.StartTime).Seconds(), modelLatencyBounds)
		}
		if validTTFT {
			bucket.ttft.add(info.FirstResponseTime.Sub(info.StartTime).Seconds(), modelTTFTBounds)
		}
		if info.IsStream {
			bucket.streamSuccess++
		}
	} else {
		bucket.errors[category]++
	}
	bucket.lastUpdated = time.Now()
}

func snapshotModelPerformance(now time.Time, remaining int) ([]modelPerformanceSnapshot, int, map[string]int64) {
	modelPerformance.Lock()
	defer modelPerformance.Unlock()
	pruneIdleModelPerformance(now)
	snapshots := make([]modelPerformanceSnapshot, 0, len(modelPerformance.values))
	for modelName, bucket := range modelPerformance.values {
		snapshot := modelPerformanceSnapshot{model: modelName, modelPerformanceBucket: *bucket}
		snapshot.errors = maps.Clone(bucket.errors)
		snapshot.latency.buckets = slices.Clone(bucket.latency.buckets)
		snapshot.ttft.buckets = slices.Clone(bucket.ttft.buckets)
		snapshots = append(snapshots, snapshot)
	}
	slices.SortFunc(snapshots, func(left, right modelPerformanceSnapshot) int {
		if order := right.lastUpdated.Compare(left.lastUpdated); order != 0 {
			return order
		}
		return cmp.Compare(left.model, right.model)
	})
	activeModels := len(snapshots)
	selected := make([]modelPerformanceSnapshot, 0, activeModels)
	exhausted := false
	for _, snapshot := range snapshots {
		seriesCount := snapshot.seriesCount()
		bucket := modelPerformance.values[snapshot.model]
		if !exhausted && (remaining < 0 || seriesCount <= remaining) {
			selected = append(selected, snapshot)
			if remaining >= 0 {
				remaining -= seriesCount
			}
		} else {
			exhausted = true
			modelPerformance.dropped["series_limit"] += max(0, snapshot.latency.count-bucket.reportedLatency) + max(0, snapshot.ttft.count-bucket.reportedTTFT)
		}
		bucket.reportedLatency = max(bucket.reportedLatency, snapshot.latency.count)
		bucket.reportedTTFT = max(bucket.reportedTTFT, snapshot.ttft.count)
	}
	return selected, activeModels, maps.Clone(modelPerformance.dropped)
}

func writeModelHistogram(output *strings.Builder, name, modelName string, histogram modelHistogram, bounds []float64) {
	if histogram.count == 0 {
		return
	}
	label := escapePrometheusLabel(modelName)
	for index, bound := range bounds {
		fmt.Fprintf(output, "%s_bucket{model=\"%s\",le=\"%s\"} %d\n", name, label, formatPrometheusFloat(bound), histogram.buckets[index])
	}
	fmt.Fprintf(output, "%s_bucket{model=\"%s\",le=\"+Inf\"} %d\n", name, label, histogram.count)
	fmt.Fprintf(output, "%s_sum{model=\"%s\"} %s\n", name, label, formatPrometheusFloat(histogram.sum))
	fmt.Fprintf(output, "%s_count{model=\"%s\"} %d\n", name, label, histogram.count)
}

func pruneIdleModelPerformance(now time.Time) {
	cutoff := now.Add(-modelPerformanceIdleRetention)
	for modelName, bucket := range modelPerformance.values {
		if bucket.lastUpdated.Before(cutoff) {
			delete(modelPerformance.values, modelName)
		}
	}
}

func classifyFinalModelError(ctx context.Context, info *relaycommon.RelayInfo, apiErr *types.NewAPIError) string {
	if apiErr == nil {
		if ctx != nil && ctx.Err() != nil {
			apiErr = types.NewError(ctx.Err(), types.ErrorCodeDoRequestFailed)
		} else if info != nil && info.StreamStatus != nil {
			stream := info.StreamStatus.OutcomeSnapshot()
			switch stream.EndReason {
			case relaycommon.StreamEndReasonClientGone:
				return "client_cancel"
			case relaycommon.StreamEndReasonTimeout, relaycommon.StreamEndReasonFirstResponseTimeout:
				return "timeout"
			}
			if stream.Response == relaycommon.ResponseOutcomeCancelled {
				return "client_cancel"
			}
			apiErr = types.InitOpenAIError(types.ErrorCode(stream.ErrorCode), stream.ErrorStatus)
		} else {
			return "other"
		}
	}
	if errors.Is(apiErr, context.Canceled) {
		return "client_cancel"
	}
	errorCode := apiErr.GetErrorCode()
	if errors.Is(apiErr, context.DeadlineExceeded) || errorCode == types.ErrorCodeChannelResponseTimeExceeded ||
		apiErr.StatusCode == http.StatusRequestTimeout || apiErr.StatusCode == http.StatusGatewayTimeout {
		return "timeout"
	}
	var networkError net.Error
	if errors.As(apiErr, &networkError) && networkError.Timeout() {
		return "timeout"
	}
	if apiErr.StatusCode == http.StatusTooManyRequests {
		return "rate_limit"
	}
	if apiErr.StatusCode == http.StatusUnauthorized || apiErr.StatusCode == http.StatusForbidden || errorCode == types.ErrorCodeChannelInvalidKey {
		return "auth"
	}
	switch errorCode {
	case types.ErrorCodeReadResponseBodyFailed, types.ErrorCodeBadResponse, types.ErrorCodeBadResponseBody, types.ErrorCodeEmptyResponse:
		return "bad_response"
	case types.ErrorCodeDoRequestFailed, types.ErrorCodeAwsInvokeError, types.ErrorCodeChannelAwsClientError:
		return "network"
	}
	if apiErr.StatusCode >= 400 && apiErr.StatusCode < 500 {
		return "upstream_4xx"
	}
	if apiErr.StatusCode >= 500 && apiErr.StatusCode <= 599 {
		return "upstream_5xx"
	}
	return "other"
}
