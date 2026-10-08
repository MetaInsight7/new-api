package service

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type codexDiscoveryTransport func(*http.Request) (*http.Response, error)

func (transport codexDiscoveryTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return transport(request)
}

func TestCodexDiscoveryResponseLimitsAndErrorStatus(t *testing.T) {
	for _, scenario := range []string{"oversized", "unauthorized", "success"} {
		t.Run(scenario, func(t *testing.T) {
			status, body := http.StatusOK, `{"models":[{"slug":"gpt-model"},{"slug":"gpt-model"}]}`
			if scenario == "oversized" {
				body = strings.Repeat(" ", (4<<20)+1)
			}
			if scenario == "unauthorized" {
				status = http.StatusUnauthorized
				body = strings.Repeat(" ", (4<<20)+1)
			}
			client := &http.Client{Transport: codexDiscoveryTransport(func(request *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body))}, nil
			})}
			upstreamStatus, models, err := FetchCodexModels(t.Context(), client, "https://example.test", &CodexOAuthKey{AccessToken: "token", AccountID: "account"}, "0.144.0")
			require.Equal(t, status, upstreamStatus)
			if scenario == "oversized" {
				require.ErrorContains(t, err, "exceeds")
				return
			}
			require.NoError(t, err)
			if scenario == "success" {
				require.Equal(t, []string{"gpt-model"}, models)
			} else {
				require.Empty(t, models)
			}
		})
	}
}

func TestCodexDiscoveryVersionCacheCoalescesAndCachesColdFailures(t *testing.T) {
	var calls atomic.Int32
	cache := &codexClientVersionCache{}
	now := time.Now()
	client := &http.Client{Transport: codexDiscoveryTransport(func(request *http.Request) (*http.Response, error) {
		calls.Add(1)
		return &http.Response{StatusCode: http.StatusServiceUnavailable, Body: io.NopCloser(strings.NewReader("unavailable"))}, nil
	})}
	var workers sync.WaitGroup
	for range 10 {
		workers.Go(func() {
			_, err := cache.get(t.Context(), client, "https://example.test/release", now)
			require.Error(t, err)
		})
	}
	workers.Wait()
	require.Equal(t, int32(1), calls.Load())
	_, err := cache.get(t.Context(), client, "https://example.test/release", now.Add(31*time.Second))
	require.Error(t, err)
	require.Equal(t, int32(2), calls.Load())
}

func TestCodexDiscoveryVersionCacheDoesNotHoldLockDuringNetwork(t *testing.T) {
	cache := &codexClientVersionCache{version: "0.144.0", expiresAt: time.Now().Add(-time.Minute)}
	client := &http.Client{Transport: codexDiscoveryTransport(func(request *http.Request) (*http.Response, error) {
		if !cache.TryLock() {
			return nil, errors.New("cache lock held during network")
		}
		cache.Unlock()
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"tag_name":"rust-v0.145.0","draft":false,"prerelease":false}`))}, nil
	})}
	version, err := cache.get(t.Context(), client, "https://example.test/release", time.Now())
	require.NoError(t, err)
	require.Equal(t, "0.145.0", version)
}
