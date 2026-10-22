package controller

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func configureAdsSyncTest(t *testing.T) {
	t.Helper()
	for key, value := range map[string]string{
		"GOOGLE_ADS_DEVELOPER_TOKEN": "fixture-developer-token",
		"GOOGLE_ADS_CLIENT_ID":       "fixture-client",
		"GOOGLE_ADS_CLIENT_SECRET":   "fixture-secret",
		"GOOGLE_ADS_REFRESH_TOKEN":   "fixture-refresh",
		"GOOGLE_ADS_CUSTOMER_ID":     "1234567890",
		"GOOGLE_ADS_CAMPAIGN_PREFIX": "fixture",
		"GOOGLE_ADS_LANDING_HOST":    "example.test",
	} {
		t.Setenv(key, value)
	}

	opsAdsSyncMu.Lock()
	previousInFlight, previousLastTry, previousResult := opsAdsSyncInFlight, opsAdsSyncLastTry, opsAdsSyncLastResult
	previousSpend, previousDaily := opsSyncAdsSpendFunc, opsSyncAdsDailyFunc
	opsAdsSyncInFlight, opsAdsSyncLastTry, opsAdsSyncLastResult = false, time.Time{}, opsAdsSyncResult{}
	opsAdsSyncMu.Unlock()
	t.Cleanup(func() {
		opsAdsSyncMu.Lock()
		opsAdsSyncInFlight, opsAdsSyncLastTry, opsAdsSyncLastResult = previousInFlight, previousLastTry, previousResult
		opsSyncAdsSpendFunc, opsSyncAdsDailyFunc = previousSpend, previousDaily
		opsAdsSyncMu.Unlock()
	})
}

func waitForAdsSyncIdle(t *testing.T) {
	t.Helper()
	require.Eventually(t, func() bool {
		opsAdsSyncMu.Lock()
		defer opsAdsSyncMu.Unlock()
		return !opsAdsSyncInFlight
	}, time.Second, time.Millisecond)
}

func TestEnsureAdsSyncStartedDoesNotBlockReportRequests(t *testing.T) {
	configureAdsSyncTest(t)
	started := make(chan struct{})
	release := make(chan struct{})
	opsSyncAdsSpendFunc = func() error {
		close(started)
		<-release
		return nil
	}
	opsSyncAdsDailyFunc = func() error { return nil }

	start := time.Now()
	ensureAdsSyncStarted()
	assert.Less(t, time.Since(start), 100*time.Millisecond)
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("background ads sync did not start")
	}
	close(release)
	waitForAdsSyncIdle(t)
}

func TestAdsSyncStatusIsSanitizedAndDistinguishesFailure(t *testing.T) {
	configureAdsSyncTest(t)
	opsSyncAdsSpendFunc = func() error { return errors.New("https://secret.invalid?token=usable-secret") }
	opsSyncAdsDailyFunc = func() error { return nil }

	ensureAdsSyncStarted()
	waitForAdsSyncIdle(t)
	assert.Equal(t, "ads spend synchronization failed", currentAdsSyncError())
	assert.False(t, strings.Contains(currentAdsSyncError(), "secret"))
}

func TestAdsSyncStatusIsHealthyAfterSuccessfulAttempt(t *testing.T) {
	configureAdsSyncTest(t)
	opsSyncAdsSpendFunc = func() error { return nil }
	opsSyncAdsDailyFunc = func() error { return nil }

	ensureAdsSyncStarted()
	waitForAdsSyncIdle(t)
	assert.Empty(t, currentAdsSyncError())
}

func TestAdsSyncStatusStaysEmptyWhenCredentialsAreNotConfigured(t *testing.T) {
	configureAdsSyncTest(t)
	for _, key := range []string{
		"GOOGLE_ADS_DEVELOPER_TOKEN",
		"GOOGLE_ADS_CLIENT_ID",
		"GOOGLE_ADS_CLIENT_SECRET",
		"GOOGLE_ADS_REFRESH_TOKEN",
		"GOOGLE_ADS_CUSTOMER_ID",
	} {
		t.Setenv(key, "")
	}
	opsSyncAdsSpendFunc = func() error { return errors.New("must not run") }
	opsSyncAdsDailyFunc = func() error { return errors.New("must not run") }

	ensureAdsSyncStarted()

	opsAdsSyncMu.Lock()
	defer opsAdsSyncMu.Unlock()
	assert.False(t, opsAdsSyncInFlight)
	assert.True(t, opsAdsSyncLastTry.IsZero())
	assert.Empty(t, opsAdsSyncLastResult.LastError)
}
