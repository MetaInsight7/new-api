package perfmetrics

import (
	"testing"

	"github.com/QuantumNous/new-api/setting/perf_metrics_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClassifyModelHealth(t *testing.T) {
	tests := []struct {
		name           string
		requestCount   int64
		successRate    float64
		expectedHealth ModelHealthState
	}{
		{"insufficient", 19, 100, ModelHealthInsufficient},
		{"healthy", 20, 99.9, ModelHealthHealthy},
		{"watch", 20, 99, ModelHealthWatch},
		{"degraded", 20, 98.9, ModelHealthDegraded},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := ClassifyModelHealth(test.requestCount, test.successRate); got != test.expectedHealth {
				t.Fatalf("health = %s, want %s", got, test.expectedHealth)
			}
		})
	}
}

func TestModelHealthMetadataDescribesIgnoredOutcomes(t *testing.T) {
	metadata := buildModelHealthMetadata(100, 24, perf_metrics_setting.PerfMetricsSetting{}, 3600, 0, 0, nil, nil)
	require.Len(t, metadata.DataQuality.Caveats, 2)
	assert.Equal(t, ModelHealthCaveatIgnoredOutcomes, metadata.DataQuality.Caveats[0].Code)
	assert.Equal(t, "Client cancellations and business rejections are excluded from health calculations.", metadata.DataQuality.Caveats[0].Description)
}
