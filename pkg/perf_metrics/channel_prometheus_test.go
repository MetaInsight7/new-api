package perfmetrics

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/perf_metrics_setting"
	"github.com/prometheus/common/expfmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildPrometheusTextIncludesChannelAttemptAndEscapedLabels(t *testing.T) {
	isolatePrometheusMetrics(t)
	info := &relaycommon.RelayInfo{OriginModelName: "model\"name", IsStream: false}
	RecordChannelAttempt(info, 7, "channel\"name", time.Now().Add(-time.Second), nil)

	text, err := BuildPrometheusText(context.Background())

	require.NoError(t, err)
	require.Contains(t, text, `newapi_channel_attempts_total{channel_id="7",status="success",error_category="none"} 1`)
	require.Contains(t, text, `channel_name="channel\"name"`)
}

func TestClassifyChannelAttemptError(t *testing.T) {
	err := types.NewOpenAIError(context.DeadlineExceeded, types.ErrorCodeDoRequestFailed, 500)
	status, category := classifyChannelAttemptError(err)
	require.Equal(t, "error", status)
	require.Equal(t, "timeout", category)
}

func TestFinalModelRequestsCountSampledCancellations(testContext *testing.T) {
	isolatePrometheusMetrics(testContext)
	modelName := "final-model\"with\\labels\n"
	for range 9 {
		RecordRelayResult(context.Background(), &relaycommon.RelayInfo{
			OriginModelName: modelName, StartTime: time.Now().Add(-time.Second),
		}, nil)
	}
	RecordRelayResult(context.Background(), &relaycommon.RelayInfo{
		OriginModelName: modelName, StartTime: time.Now().Add(-time.Second),
	}, types.NewOpenAIError(context.Canceled, types.ErrorCodeDoRequestFailed, 499))
	text, err := BuildPrometheusText(context.Background())
	require.NoError(testContext, err)
	require.Contains(testContext, text, `newapi_model_requests_total{model="final-model\"with\\labels\n",status="success"} 9`)
	require.Contains(testContext, text, `newapi_model_requests_total{model="final-model\"with\\labels\n",status="error"} 1`)
	require.Contains(testContext, text, `newapi_model_request_duration_seconds_count{model="final-model\"with\\labels\n"} 9`)
	parser := expfmt.TextParser{}
	families, err := parser.TextToMetricFamilies(strings.NewReader(text))
	require.NoError(testContext, err)
	for _, metric := range families["newapi_model_requests_total"].GetMetric() {
		require.Equal(testContext, modelName, metric.GetLabel()[0].GetValue())
	}
}

func isolatePrometheusMetrics(testContext *testing.T) {
	testContext.Helper()
	testContext.Setenv("PROMETHEUS_MAX_SERIES_PER_SCRAPE", "5000")
	testContext.Setenv("PROMETHEUS_MAX_MODEL_HISTOGRAM_MODELS", "50")
	previous := perf_metrics_setting.GetSetting()
	require.NoError(testContext, config.GlobalConfig.LoadFromDB(map[string]string{
		"perf_metrics_setting.enabled": "true", "perf_metrics_setting.bucket_time": "hour",
		"perf_metrics_setting.flush_interval": "5", "perf_metrics_setting.retention_days": "0",
	}))
	testContext.Cleanup(func() {
		require.NoError(testContext, config.GlobalConfig.LoadFromDB(map[string]string{
			"perf_metrics_setting.enabled": fmt.Sprint(previous.Enabled), "perf_metrics_setting.bucket_time": previous.BucketTime,
			"perf_metrics_setting.flush_interval": fmt.Sprint(previous.FlushInterval), "perf_metrics_setting.retention_days": fmt.Sprint(previous.RetentionDays),
		}))
	})
	channelMetricBuckets.Clear()
	channelModelBuckets.Clear()
	hotBuckets.Clear()
	modelRequests.Lock()
	clear(modelRequests.values)
	modelRequests.Unlock()
	modelPerformance.Lock()
	clear(modelPerformance.values)
	clear(modelPerformance.dropped)
	modelPerformance.Unlock()
	testContext.Cleanup(func() {
		channelMetricBuckets.Clear()
		channelModelBuckets.Clear()
		hotBuckets.Clear()
		modelRequests.Lock()
		clear(modelRequests.values)
		modelRequests.Unlock()
		modelPerformance.Lock()
		clear(modelPerformance.values)
		clear(modelPerformance.dropped)
		modelPerformance.Unlock()
	})
}

func TestFinalModelErrorCategories(testContext *testing.T) {
	isolatePrometheusMetrics(testContext)
	testContext.Setenv("PROMETHEUS_MAX_MODEL_HISTOGRAM_MODELS", "0")
	for _, test := range []struct {
		name, category string
		code           types.ErrorCode
		status         int
		cause          error
	}{
		{"cancel", "client_cancel", types.ErrorCodeDoRequestFailed, 500, context.Canceled},
		{"deadline", "timeout", types.ErrorCodeDoRequestFailed, 500, context.DeadlineExceeded},
		{"net-timeout", "timeout", types.ErrorCodeDoRequestFailed, 500, &net.DNSError{IsTimeout: true}},
		{"response-timeout", "timeout", types.ErrorCodeChannelResponseTimeExceeded, 400, nil},
		{"http-timeout", "timeout", "upstream", 504, nil},
		{"http-request-timeout", "timeout", "upstream", 408, nil},
		{"rate", "rate_limit", types.ErrorCodeDoRequestFailed, 429, nil},
		{"auth", "auth", types.ErrorCodeDoRequestFailed, 401, nil},
		{"forbidden", "auth", "upstream", 403, nil},
		{"key", "auth", types.ErrorCodeChannelInvalidKey, 400, nil},
		{"body-read", "bad_response", types.ErrorCodeReadResponseBodyFailed, 500, nil},
		{"bad-response", "bad_response", types.ErrorCodeBadResponse, 500, nil},
		{"bad-body", "bad_response", types.ErrorCodeBadResponseBody, 500, nil},
		{"empty", "bad_response", types.ErrorCodeEmptyResponse, 500, nil},
		{"transport", "network", types.ErrorCodeDoRequestFailed, 500, nil},
		{"aws-invoke", "network", types.ErrorCodeAwsInvokeError, 400, nil},
		{"aws-client", "network", types.ErrorCodeChannelAwsClientError, 500, nil},
		{"upstream-parameter", "upstream_4xx", "context_length_exceeded", 400, nil},
		{"upstream-failure", "upstream_5xx", "upstream", 500, nil},
		{"fallback", "other", "unexpected", 200, nil},
	} {
		testContext.Run(test.name, func(testContext *testing.T) {
			cause := test.cause
			if cause == nil {
				cause = errors.New("sensitive error must not be a label")
			}
			RecordRelayResult(context.Background(), &relaycommon.RelayInfo{
				OriginModelName: test.name, StartTime: time.Now().Add(-time.Second),
			}, types.NewErrorWithStatusCode(cause, test.code, test.status))
			text, err := BuildPrometheusText(context.Background())
			require.NoError(testContext, err)
			require.Contains(testContext, text, fmt.Sprintf(`newapi_model_errors_total{model=%q,error_category=%q} 1`, test.name, test.category))
			require.Contains(testContext, text, fmt.Sprintf(`newapi_model_requests_total{model=%q,status="error"} 1`, test.name))
			require.NotContains(testContext, text, "sensitive error")
		})
	}
}

func TestModelPerformanceAdmissionPreservesFinalRequestCounts(testContext *testing.T) {
	isolatePrometheusMetrics(testContext)
	testContext.Setenv("PROMETHEUS_MAX_MODEL_HISTOGRAM_MODELS", "1")
	var workers sync.WaitGroup
	start := make(chan struct{})
	for _, modelName := range []string{"admit-a", "admit-b", "admit-c"} {
		workers.Go(func() {
			<-start
			RecordRelayResult(context.Background(), &relaycommon.RelayInfo{OriginModelName: modelName}, types.InitOpenAIError("upstream", 500))
		})
	}
	close(start)
	workers.Wait()
	text, err := BuildPrometheusText(context.Background())
	require.NoError(testContext, err)
	require.Contains(testContext, text, "newapi_model_histogram_active_models 1\n")
	require.Contains(testContext, text, `newapi_model_histogram_dropped_samples_total{reason="model_limit"} 2`)
	require.Equal(testContext, 3, strings.Count(text, `newapi_model_requests_total{model="admit-`))
	require.Equal(testContext, 1, strings.Count(text, `newapi_model_errors_total{model="admit-`))
}

func TestModelPerformanceBudgetKeepsRecentGroupsWhole(testContext *testing.T) {
	isolatePrometheusMetrics(testContext)
	for _, modelName := range []string{"older", "recent"} {
		RecordRelayResult(context.Background(), &relaycommon.RelayInfo{OriginModelName: modelName}, types.InitOpenAIError("upstream", 500))
	}
	testContext.Setenv("PROMETHEUS_MAX_SERIES_PER_SCRAPE", "8")
	text, err := BuildPrometheusText(context.Background())
	require.NoError(testContext, err)
	require.Contains(testContext, text, `newapi_model_errors_total{model="recent",error_category="upstream_5xx"} 1`)
	require.NotContains(testContext, text, `newapi_model_errors_total{model="older"`)
	require.Contains(testContext, text, "newapi_perf_metrics_series 8\n")
	require.Contains(testContext, text, "newapi_model_histogram_active_models 2\n")
	require.Contains(testContext, text, `newapi_model_histogram_dropped_samples_total{reason="series_limit"} 0`)
	testContext.Setenv("PROMETHEUS_MAX_SERIES_PER_SCRAPE", "6")
	text, err = BuildPrometheusText(context.Background())
	require.NoError(testContext, err)
	require.Contains(testContext, text, `newapi_model_requests_total{model="older",status="error"} 1`)
	require.NotContains(testContext, text, "newapi_model_histogram_active_models ")
	require.NotContains(testContext, text, `newapi_model_errors_total{`)
	testContext.Setenv("PROMETHEUS_MAX_SERIES_PER_SCRAPE", "1")
	_, err = BuildPrometheusText(context.Background())
	require.ErrorContains(testContext, err, "series limit exceeded")
}

func TestPrometheusChannelSeriesBudgetIncludesInfiniteBucket(testContext *testing.T) {
	isolatePrometheusMetrics(testContext)
	RecordChannelAttempt(&relaycommon.RelayInfo{OriginModelName: "model"}, 1, "channel", time.Now().Add(-time.Second), nil)
	testContext.Setenv("PROMETHEUS_MAX_SERIES_PER_SCRAPE", "15")
	_, err := BuildPrometheusText(context.Background())
	require.ErrorContains(testContext, err, "series limit exceeded")
	testContext.Setenv("PROMETHEUS_MAX_SERIES_PER_SCRAPE", "16")
	text, err := BuildPrometheusText(context.Background())
	require.NoError(testContext, err)
	require.Contains(testContext, text, "newapi_perf_metrics_series 16\n")
}

func TestSuccessfulAsyncModelDurationDistribution(testContext *testing.T) {
	isolatePrometheusMetrics(testContext)
	now := time.Now().Unix()
	for _, task := range []model.Task{
		{Status: model.TaskStatusSuccess, SubmitTime: now - 2, FinishTime: now},
		{Status: model.TaskStatusSuccess, SubmitTime: now - 4, FinishTime: now},
		{Status: model.TaskStatusFailure, SubmitTime: now - 1, FinishTime: now},
		{Status: model.TaskStatusInProgress, SubmitTime: now - 1},
	} {
		task.Properties.OriginModelName = "async-distribution"
		RecordTaskResult(&task, nil)
	}
	text, err := BuildPrometheusText(context.Background())
	require.NoError(testContext, err)
	for _, expected := range []string{
		`newapi_model_requests_total{model="async-distribution",status="success"} 2`,
		`newapi_model_requests_total{model="async-distribution",status="error"} 1`,
		`newapi_model_errors_total{model="async-distribution",error_category="other"} 1`,
		`newapi_model_request_duration_seconds_bucket{model="async-distribution",le="1"} 0`,
		`newapi_model_request_duration_seconds_bucket{model="async-distribution",le="2"} 1`,
		`newapi_model_request_duration_seconds_bucket{model="async-distribution",le="3"} 1`,
		`newapi_model_request_duration_seconds_bucket{model="async-distribution",le="5"} 2`,
		`newapi_model_request_duration_seconds_bucket{model="async-distribution",le="+Inf"} 2`,
		`newapi_model_request_duration_seconds_count{model="async-distribution"} 2`,
		`newapi_model_request_duration_seconds_sum{model="async-distribution"} 6`,
	} {
		require.Contains(testContext, text, expected+"\n")
	}
	require.NotContains(testContext, text, `newapi_model_ttft_seconds_count{model="async-distribution"}`)
	require.NotContains(testContext, text, `newapi_model_stream_success_total{model="async-distribution"}`)
	parser := expfmt.TextParser{}
	families, err := parser.TextToMetricFamilies(strings.NewReader(text))
	require.NoError(testContext, err)
	metrics := families["newapi_model_request_duration_seconds"].GetMetric()
	require.Len(testContext, metrics, 1)
	histogram := metrics[0].GetHistogram()
	require.Equal(testContext, uint64(2), histogram.GetSampleCount())
	require.Equal(testContext, 6.0, histogram.GetSampleSum())
	var bounds []float64
	for _, bucket := range histogram.GetBucket() {
		bounds = append(bounds, bucket.GetUpperBound())
	}
	require.Equal(testContext, []float64{0.25, 0.5, 1, 2, 3, 5, 10, 15, 30, 60, 120, 300, 600}, bounds[:len(bounds)-1])
}

func TestSuccessfulStreamTTFTDistribution(testContext *testing.T) {
	isolatePrometheusMetrics(testContext)
	started := time.Now().Add(-2 * time.Second)
	RecordRelayResult(context.Background(), &relaycommon.RelayInfo{
		OriginModelName: "stream-distribution", IsStream: true,
		StartTime: started, FirstResponseTime: started.Add(300 * time.Millisecond),
	}, nil)
	text, err := BuildPrometheusText(context.Background())
	require.NoError(testContext, err)
	for _, expected := range []string{
		`newapi_model_ttft_seconds_bucket{model="stream-distribution",le="0.25"} 0`,
		`newapi_model_ttft_seconds_bucket{model="stream-distribution",le="0.5"} 1`,
		`newapi_model_ttft_seconds_bucket{model="stream-distribution",le="+Inf"} 1`,
		`newapi_model_ttft_seconds_sum{model="stream-distribution"} 0.3`,
		`newapi_model_ttft_seconds_count{model="stream-distribution"} 1`,
		`newapi_model_stream_success_total{model="stream-distribution"} 1`,
	} {
		require.Contains(testContext, text, expected+"\n")
	}
	parser := expfmt.TextParser{}
	families, err := parser.TextToMetricFamilies(strings.NewReader(text))
	require.NoError(testContext, err)
	metrics := families["newapi_model_ttft_seconds"].GetMetric()
	require.Len(testContext, metrics, 1)
	var bounds []float64
	for _, bucket := range metrics[0].GetHistogram().GetBucket() {
		bounds = append(bounds, bucket.GetUpperBound())
	}
	require.Equal(testContext, []float64{0.1, 0.25, 0.5, 1, 2, 3, 5, 10, 15, 30, 60}, bounds[:len(bounds)-1])
}

func TestSuccessfulStreamsDoNotFabricateTimingSamples(testContext *testing.T) {
	started := time.Now().Add(-2 * time.Second)
	for _, test := range []struct {
		name           string
		start, first   time.Time
		invalidLatency int
		invalidTTFT    int
	}{
		{name: "missing", start: started},
		{name: "sentinel", start: started, first: started.Add(-time.Second)},
		{name: "before-start", start: started, first: started.Add(-time.Millisecond), invalidTTFT: 1},
		{name: "at-start", start: started, first: started, invalidTTFT: 1},
		{name: "after-end", start: started, first: started.Add(time.Hour), invalidTTFT: 1},
		{name: "no-start", first: started, invalidLatency: 1, invalidTTFT: 1},
		{name: "future-start", start: started.Add(time.Hour), invalidLatency: 1},
	} {
		testContext.Run(test.name, func(testContext *testing.T) {
			isolatePrometheusMetrics(testContext)
			RecordRelayResult(context.Background(), &relaycommon.RelayInfo{
				OriginModelName: "timing", IsStream: true, StartTime: test.start, FirstResponseTime: test.first,
			}, nil)
			text, err := BuildPrometheusText(context.Background())
			require.NoError(testContext, err)
			require.Contains(testContext, text, `newapi_model_requests_total{model="timing",status="success"} 1`)
			require.Contains(testContext, text, `newapi_model_stream_success_total{model="timing"} 1`)
			require.NotContains(testContext, text, `newapi_model_ttft_seconds_count{model="timing"}`)
			require.Contains(testContext, text, fmt.Sprintf(`newapi_model_histogram_dropped_samples_total{reason="invalid_latency"} %d`, test.invalidLatency))
			require.Contains(testContext, text, fmt.Sprintf(`newapi_model_histogram_dropped_samples_total{reason="invalid_ttft"} %d`, test.invalidTTFT))
			if test.invalidLatency == 0 {
				require.Contains(testContext, text, `newapi_model_request_duration_seconds_count{model="timing"} 1`)
			} else {
				require.NotContains(testContext, text, `newapi_model_request_duration_seconds_count{model="timing"}`)
			}
		})
	}
}

func TestModelHistogramBudgetCountsOnlyNewOmittedObservations(testContext *testing.T) {
	isolatePrometheusMetrics(testContext)
	started := time.Now().Add(-2 * time.Second)
	info := &relaycommon.RelayInfo{OriginModelName: "budget-stream", IsStream: true, StartTime: started, FirstResponseTime: started.Add(300 * time.Millisecond)}
	RecordRelayResult(context.Background(), info, nil)
	RecordRelayResult(context.Background(), info, types.InitOpenAIError("upstream", 500))
	testContext.Setenv("PROMETHEUS_MAX_SERIES_PER_SCRAPE", "38")
	for range 2 {
		text, err := BuildPrometheusText(context.Background())
		require.NoError(testContext, err)
		require.Contains(testContext, text, `newapi_model_histogram_dropped_samples_total{reason="series_limit"} 2`)
		require.NotContains(testContext, text, `newapi_model_request_duration_seconds_count{model="budget-stream"}`)
		require.NotContains(testContext, text, `newapi_model_errors_total{model="budget-stream"`)
	}
	info.FirstResponseTime = time.Time{}
	RecordRelayResult(context.Background(), info, nil)
	text, err := BuildPrometheusText(context.Background())
	require.NoError(testContext, err)
	require.Contains(testContext, text, `newapi_model_histogram_dropped_samples_total{reason="series_limit"} 3`)
	testContext.Setenv("PROMETHEUS_MAX_SERIES_PER_SCRAPE", "0")
	text, err = BuildPrometheusText(context.Background())
	require.NoError(testContext, err)
	require.Contains(testContext, text, `newapi_model_request_duration_seconds_count{model="budget-stream"} 2`)
	require.Contains(testContext, text, `newapi_model_ttft_seconds_count{model="budget-stream"} 1`)
	require.Contains(testContext, text, `newapi_model_stream_success_total{model="budget-stream"} 2`)
	require.Contains(testContext, text, `newapi_model_histogram_dropped_samples_total{reason="series_limit"} 3`)
	info.FirstResponseTime = started.Add(300 * time.Millisecond)
	RecordRelayResult(context.Background(), info, nil)
	testContext.Setenv("PROMETHEUS_MAX_SERIES_PER_SCRAPE", "38")
	text, err = BuildPrometheusText(context.Background())
	require.NoError(testContext, err)
	require.Contains(testContext, text, `newapi_model_histogram_dropped_samples_total{reason="series_limit"} 5`)
}

func TestInvalidTimingsRemainVisibleWhenAdmissionRejectsModel(testContext *testing.T) {
	isolatePrometheusMetrics(testContext)
	testContext.Setenv("PROMETHEUS_MAX_MODEL_HISTOGRAM_MODELS", "1")
	for _, modelName := range []string{"invalid-a", "invalid-b"} {
		RecordRelayResult(context.Background(), &relaycommon.RelayInfo{OriginModelName: modelName}, nil)
	}
	text, err := BuildPrometheusText(context.Background())
	require.NoError(testContext, err)
	require.Contains(testContext, text, `newapi_model_histogram_dropped_samples_total{reason="invalid_latency"} 2`)
	require.Contains(testContext, text, `newapi_model_histogram_dropped_samples_total{reason="model_limit"} 1`)
	require.Contains(testContext, text, `newapi_model_requests_total{model="invalid-b",status="success"} 1`)
}

func TestModelMetricIdleLifetimesAndReadmission(testContext *testing.T) {
	isolatePrometheusMetrics(testContext)
	testContext.Setenv("PROMETHEUS_MAX_MODEL_HISTOGRAM_MODELS", "1")
	RecordRelayResult(context.Background(), &relaycommon.RelayInfo{OriginModelName: "idle"}, types.InitOpenAIError("upstream", 500))
	modelPerformance.Lock()
	modelPerformance.values["idle"].lastUpdated = time.Now().Add(-2 * time.Hour)
	modelPerformance.Unlock()
	var workers sync.WaitGroup
	start := make(chan struct{})
	workers.Go(func() {
		<-start
		_, err := BuildPrometheusText(context.Background())
		assert.NoError(testContext, err)
	})
	workers.Go(func() {
		<-start
		RecordRelayResult(context.Background(), &relaycommon.RelayInfo{OriginModelName: "current"}, types.InitOpenAIError("upstream", 500))
	})
	close(start)
	workers.Wait()
	RecordRelayResult(context.Background(), &relaycommon.RelayInfo{OriginModelName: "current"}, types.InitOpenAIError("upstream", 500))
	text, err := BuildPrometheusText(context.Background())
	require.NoError(testContext, err)
	require.Contains(testContext, text, `newapi_model_requests_total{model="idle",status="error"} 1`)
	require.NotContains(testContext, text, `newapi_model_errors_total{model="idle"`)
	require.Contains(testContext, text, `newapi_model_errors_total{model="current",error_category="upstream_5xx"} 2`)
	modelRequests.Lock()
	key := modelRequestKey{model: "idle", status: "error"}
	counter := modelRequests.values[key]
	counter.lastUpdated = time.Now().Add(-7 * time.Hour)
	modelRequests.values[key] = counter
	modelRequests.Unlock()
	text, err = BuildPrometheusText(context.Background())
	require.NoError(testContext, err)
	require.NotContains(testContext, text, `newapi_model_requests_total{model="idle"`)
}

func TestFinalStreamErrorsExcludeSuccessfulDistributions(testContext *testing.T) {
	for _, test := range []struct {
		name, category string
		end            relaycommon.StreamEndReason
		mark           func(*relaycommon.StreamStatus)
	}{
		{name: "first-response-timeout", category: "timeout", end: relaycommon.StreamEndReasonFirstResponseTimeout},
		{name: "read-timeout", category: "timeout", end: relaycommon.StreamEndReasonTimeout},
		{name: "disconnect", category: "client_cancel", end: relaycommon.StreamEndReasonClientGone},
		{name: "soft-error", category: "other", end: relaycommon.StreamEndReasonDone, mark: func(stream *relaycommon.StreamStatus) { stream.RecordError("secret detail") }},
		{name: "terminal-rate-limit", category: "rate_limit", end: relaycommon.StreamEndReasonEOF, mark: func(stream *relaycommon.StreamStatus) { stream.MarkFailed("rate_limit", "upstream", 429) }},
		{name: "missing-terminal", category: "other", end: relaycommon.StreamEndReasonEOF, mark: (*relaycommon.StreamStatus).RequireTerminal},
	} {
		testContext.Run(test.name, func(testContext *testing.T) {
			isolatePrometheusMetrics(testContext)
			stream := relaycommon.NewStreamStatus()
			stream.SetEndReason(test.end, nil)
			if test.mark != nil {
				test.mark(stream)
			}
			started := time.Now().Add(-time.Second)
			RecordRelayResult(context.Background(), &relaycommon.RelayInfo{
				OriginModelName: "stream-error", IsStream: true, StreamStatus: stream,
				StartTime: started, FirstResponseTime: started.Add(100 * time.Millisecond),
			}, nil)
			text, err := BuildPrometheusText(context.Background())
			require.NoError(testContext, err)
			require.Contains(testContext, text, `newapi_model_requests_total{model="stream-error",status="error"} 1`)
			require.Contains(testContext, text, `newapi_model_errors_total{model="stream-error",error_category="`+test.category+`"} 1`)
			require.NotContains(testContext, text, `newapi_model_request_duration_seconds_count{model="stream-error"}`)
			require.NotContains(testContext, text, `newapi_model_ttft_seconds_count{model="stream-error"}`)
			require.NotContains(testContext, text, `newapi_model_stream_success_total{model="stream-error"}`)
			require.NotContains(testContext, text, "secret detail")
		})
	}
}

func TestModelPerformanceDefaultAndUnlimitedAdmission(testContext *testing.T) {
	for _, test := range []struct {
		limit   string
		active  int
		dropped int
	}{
		{"", 50, 1}, {"0", 51, 0}, {"-1", 51, 0},
	} {
		testContext.Run("limit="+test.limit, func(testContext *testing.T) {
			isolatePrometheusMetrics(testContext)
			testContext.Setenv("PROMETHEUS_MAX_MODEL_HISTOGRAM_MODELS", test.limit)
			testContext.Setenv("PROMETHEUS_MAX_SERIES_PER_SCRAPE", "-1")
			for index := range 51 {
				RecordRelayResult(context.Background(), &relaycommon.RelayInfo{OriginModelName: fmt.Sprintf("limit-%d", index)}, types.InitOpenAIError("upstream", 500))
			}
			text, err := BuildPrometheusText(context.Background())
			require.NoError(testContext, err)
			require.Contains(testContext, text, fmt.Sprintf("newapi_model_histogram_active_models %d\n", test.active))
			require.Contains(testContext, text, fmt.Sprintf(`newapi_model_histogram_dropped_samples_total{reason="model_limit"} %d`, test.dropped))
			require.Equal(testContext, 51, strings.Count(text, `newapi_model_requests_total{model="limit-`))
		})
	}
}

func TestDisablingCollectionPreservesExistingModelMetrics(testContext *testing.T) {
	isolatePrometheusMetrics(testContext)
	previous := perf_metrics_setting.GetSetting().Enabled
	testContext.Cleanup(func() {
		require.NoError(testContext, config.GlobalConfig.LoadFromDB(map[string]string{"perf_metrics_setting.enabled": fmt.Sprint(previous)}))
	})
	require.NoError(testContext, config.GlobalConfig.LoadFromDB(map[string]string{"perf_metrics_setting.enabled": "true"}))
	info := &relaycommon.RelayInfo{OriginModelName: "collection-switch", StartTime: time.Now().Add(-time.Second)}
	RecordRelayResult(context.Background(), info, nil)
	before, err := BuildPrometheusText(context.Background())
	require.NoError(testContext, err)
	require.NoError(testContext, config.GlobalConfig.LoadFromDB(map[string]string{"perf_metrics_setting.enabled": "false"}))
	RecordRelayResult(context.Background(), info, types.InitOpenAIError("upstream", 500))
	RecordTaskResult(&model.Task{Status: model.TaskStatusSuccess, Properties: model.Properties{OriginModelName: "collection-switch"}}, nil)
	Record(Sample{Model: "collection-switch", Success: true})
	RecordChannelAttempt(info, 1, "channel", time.Now(), nil)
	after, err := BuildPrometheusText(context.Background())
	require.NoError(testContext, err)
	require.Equal(testContext, before, after)
}
