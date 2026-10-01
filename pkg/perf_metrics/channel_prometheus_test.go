package perfmetrics

import (
	"context"
	"testing"
	"time"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/stretchr/testify/require"
)

func TestBuildPrometheusTextIncludesChannelAttemptAndEscapedLabels(t *testing.T) {
	channelMetricBuckets.Range(func(key, _ any) bool {
		channelMetricBuckets.Delete(key)
		return true
	})
	channelModelBuckets.Range(func(key, _ any) bool {
		channelModelBuckets.Delete(key)
		return true
	})
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
