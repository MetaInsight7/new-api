package perfmetrics

import "testing"

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
