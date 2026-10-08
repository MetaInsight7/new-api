package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestCodexLimitReportPreservesRealWindowsAndFailedUsage(t *testing.T) {
	channels := []*model.Channel{
		{Id: 1, Name: "Free", Status: 2},
		{Id: 2, Name: "Pro", Status: 1},
		{Id: 3, Name: "Failed", Status: 3},
		{Id: 4, Name: "Malformed", Status: 4},
	}
	fetcher := func(ctx context.Context, channel *model.Channel) (int, []byte, error) {
		switch channel.Id {
		case 1:
			return 200, []byte(`{"plan_type":"free","rate_limit":{"allowed":false,"limit_reached":true,"primary_window":{"used_percent":100,"limit_window_seconds":18000},"secondary_window":{"used_percent":23,"limit_window_seconds":604740}},"additional_rate_limits":[{"limit_name":"Images","primary_window":{"used_percent":0}}]}`), nil
		case 2:
			return 200, []byte(`{"email":"pro@example.test","account_id":"account-2","rate_limit":{"plan_type":"pro","primary_window":{"used_percent":15,"limit_window_seconds":43200},"secondary_window":{}},"additional_rate_limits":[{"metered_feature":"model-a","primary_window":{"used_percent":40,"limit_window_seconds":18000}}]}`), nil
		case 3:
			return 503, nil, errors.New("secret-access-token in upstream URL")
		default:
			return 200, []byte(`{"error":"unavailable"}`), nil
		}
	}
	report := BuildCodexLimitReportWithUsage(context.Background(), channels, fetcher,
		map[int]model.CodexChannelUsageStat{1: {TokenUsed: 30, Quota: 100}, 3: {TokenUsed: 12, Quota: 20}}, 100, 200)
	assert.Equal(t, 2, report.SuccessCount)
	assert.Equal(t, 2, report.FailureCount)
	assert.Equal(t, int64(42), report.TotalTokenUsed)
	assert.Equal(t, int64(120), report.TotalQuota)
	assert.Equal(t, int64(100), report.StartTimestamp)
	assert.Equal(t, int64(200), report.EndTimestamp)
	free := report.Rows[0]
	assert.Nil(t, free.BaseFiveHourWindow)
	assert.Equal(t, float64(23), *free.BaseWeeklyWindow.UsedPercent)
	require.Len(t, free.AdditionalLimits, 1)
	require.Len(t, free.AdditionalLimits[0].UnknownWindows, 1)
	assert.Equal(t, float64(0), *free.AdditionalLimits[0].UnknownWindows[0].UsedPercent)
	pro := report.Rows[1]
	assert.Nil(t, pro.Allowed)
	assert.Equal(t, "pro", pro.PlanType)
	assert.Equal(t, "account-2", pro.AccountID)
	assert.Nil(t, pro.BaseFiveHourWindow)
	assert.Nil(t, pro.BaseWeeklyWindow)
	require.Len(t, pro.UnknownWindows, 1)
	require.NotNil(t, pro.UnknownWindows[0].LimitWindowSeconds)
	assert.Equal(t, int64(43200), *pro.UnknownWindows[0].LimitWindowSeconds)
	require.Len(t, pro.AdditionalLimits, 1)
	assert.Equal(t, "model-a", pro.AdditionalLimits[0].Name)
	assert.NotNil(t, pro.AdditionalLimits[0].FiveHourWindow)
	assert.Equal(t, int64(12), report.Rows[2].RangeTokenUsed)
	assert.Equal(t, 503, report.Rows[2].UpstreamStatus)
	encoded, err := common.Marshal(report)
	require.NoError(t, err)
	assert.NotContains(t, string(encoded), "secret-access-token")
	assert.Equal(t, "Invalid Codex usage response", report.Rows[3].Message)
}

func TestCodexLimitReportPreservesExplicitResetZeroAndMissingUsage(t *testing.T) {
	report := BuildCodexLimitReportWithUsage(context.Background(), []*model.Channel{{Id: 1}},
		func(context.Context, *model.Channel) (int, []byte, error) {
			return 200, []byte(`{"rate_limit":{"primary_window":{"reset_after_seconds":0}}}`), nil
		}, nil, 100, 200)
	require.Len(t, report.Rows[0].UnknownWindows, 1)
	window := report.Rows[0].UnknownWindows[0]
	assert.Nil(t, window.UsedPercent)
	require.NotNil(t, window.ResetAfterSeconds)
	assert.Zero(t, *window.ResetAfterSeconds)
	encoded, err := common.Marshal(window)
	require.NoError(t, err)
	assert.Equal(t, `{"reset_after_seconds":0}`, string(encoded))
}

func TestGetCodexLimitReportFreePlanPreservesAdditionalFiveHourWindow(t *testing.T) {
	database, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "report.db")), &gorm.Config{})
	require.NoError(t, err)
	connection, err := database.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, connection.Close()) })
	previousDB, previousLogDB := model.DB, model.LOG_DB
	model.DB, model.LOG_DB = database, database
	t.Cleanup(func() { model.DB, model.LOG_DB = previousDB, previousLogDB })
	require.NoError(t, database.AutoMigrate(&model.Channel{}, &model.Log{}))
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		assert.Equal(t, http.MethodGet, request.Method)
		assert.Equal(t, "/backend-api/wham/usage", request.URL.Path)
		assert.Equal(t, "Bearer fixture-access", request.Header.Get("Authorization"))
		writer.Header().Set("Content-Type", "application/json")
		_, writeErr := io.WriteString(writer, `{"plan_type":"free","rate_limit":{"primary_window":{"used_percent":100,"limit_window_seconds":18000}},"additional_rate_limits":[{"limit_name":"Images","rate_limit":{"primary_window":{"used_percent":37,"limit_window_seconds":18000}}}]}`)
		assert.NoError(t, writeErr)
	}))
	t.Cleanup(upstream.Close)
	baseURL := upstream.URL
	require.NoError(t, database.Create(&model.Channel{
		Id: 71, Type: constant.ChannelTypeCodex, Status: 1, BaseURL: &baseURL,
		Key: `{"access_token":"fixture-access","account_id":"fixture-account"}`,
	}).Error)

	report, err := GetCodexLimitReport(context.Background(), 100, 200)
	require.NoError(t, err)
	require.Len(t, report.Rows, 1)
	row := report.Rows[0]
	assert.True(t, row.Success)
	assert.Equal(t, "free", row.PlanType)
	assert.Nil(t, row.BaseFiveHourWindow)
	require.Len(t, row.AdditionalLimits, 1)
	assert.Equal(t, "Images", row.AdditionalLimits[0].Name)
	window := row.AdditionalLimits[0].FiveHourWindow
	require.NotNil(t, window)
	require.NotNil(t, window.UsedPercent)
	require.NotNil(t, window.LimitWindowSeconds)
	assert.Equal(t, float64(37), *window.UsedPercent)
	assert.Equal(t, int64(18000), *window.LimitWindowSeconds)
}

func TestCodexLimitReportBoundsConcurrencyAndHonorsCancellation(t *testing.T) {
	channels := make([]*model.Channel, 13)
	for index := range channels {
		channels[index] = &model.Channel{Id: index + 1}
	}
	started, release := make(chan struct{}, len(channels)), make(chan struct{})
	var active, peak atomic.Int32
	fetcher := func(ctx context.Context, channel *model.Channel) (int, []byte, error) {
		count := active.Add(1)
		defer active.Add(-1)
		for previous := peak.Load(); count > previous; previous = peak.Load() {
			if peak.CompareAndSwap(previous, count) {
				break
			}
		}
		started <- struct{}{}
		<-release
		return 200, []byte(`{"rate_limit":{"allowed":true}}`), nil
	}
	completed := make(chan CodexLimitReport, 1)
	go func() {
		completed <- BuildCodexLimitReportWithUsage(context.Background(), channels, fetcher, nil, 100, 200)
	}()
	for range 10 {
		<-started
	}
	close(release)
	report := <-completed
	assert.Equal(t, int32(10), peak.Load())
	assert.Equal(t, 13, report.SuccessCount)
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	var calls atomic.Int32
	report = BuildCodexLimitReportWithUsage(cancelled, channels, func(context.Context, *model.Channel) (int, []byte, error) {
		calls.Add(1)
		return 200, nil, nil
	}, nil, 100, 200)
	assert.Equal(t, int32(0), calls.Load())
	assert.Equal(t, 13, report.FailureCount)
}

func TestCodexLimitReportCancelsInFlightQueriesWithoutDroppingLocalUsage(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	channels := make([]*model.Channel, 13)
	for index := range channels {
		channels[index] = &model.Channel{Id: index + 1}
	}
	started := make(chan struct{}, len(channels))
	completed := make(chan CodexLimitReport, 1)
	go func() {
		completed <- BuildCodexLimitReportWithUsage(ctx, channels,
			func(ctx context.Context, channel *model.Channel) (int, []byte, error) {
				started <- struct{}{}
				<-ctx.Done()
				return 0, nil, ctx.Err()
			}, map[int]model.CodexChannelUsageStat{13: {TokenUsed: 9, Quota: 20}}, 100, 200)
	}()
	for range 10 {
		<-started
	}
	cancel()
	report := <-completed
	assert.Equal(t, 13, report.FailureCount)
	assert.Equal(t, int64(9), report.TotalTokenUsed)
	assert.Equal(t, int64(20), report.Rows[12].RangeQuota)
	for _, row := range report.Rows {
		assert.Equal(t, "Codex usage query cancelled or timed out", row.Message)
	}
}

type codexReportTestTransport func(*http.Request) (*http.Response, error)

func (transport codexReportTestTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return transport(request)
}

func TestGetCodexLimitReportRefreshesOnceAndKeepsPersistenceErrors(t *testing.T) {
	for _, scenario := range []string{"refresh succeeds", "forbidden refresh succeeds", "refresh rejected", "persistence fails", "retry rejected", "oversized usage", "upstream unavailable", "network fails", "concurrent OAuth replaces key", "OAuth replaces key during refresh", "OAuth changes destination before refresh", "OAuth changes destination during refresh"} {
		t.Run(scenario, func(t *testing.T) {
			database, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "report.db")), &gorm.Config{})
			require.NoError(t, err)
			sqlDB, err := database.DB()
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
			previousDB, previousLogDB, previousMemory := model.DB, model.LOG_DB, common.MemoryCacheEnabled
			model.DB, model.LOG_DB, common.MemoryCacheEnabled = database, database, true
			t.Cleanup(func() {
				if previousDB == nil {
					if !database.Migrator().HasTable(&model.Channel{}) {
						require.NoError(t, database.AutoMigrate(&model.Channel{}))
					}
					require.NoError(t, database.Where("1 = 1").Delete(&model.Channel{}).Error)
					model.InitChannelCache()
				}
				model.DB, model.LOG_DB, common.MemoryCacheEnabled = previousDB, previousLogDB, previousMemory
				if previousDB != nil {
					model.InitChannelCache()
				}
			})
			require.NoError(t, database.AutoMigrate(&model.Channel{}, &model.Log{}, &model.Ability{}))
			baseURL := "https://usage.example.test"
			channel := &model.Channel{Id: 41, Type: constant.ChannelTypeCodex, Status: 2, BaseURL: &baseURL,
				Key: `{"access_token":"old-access","refresh_token":"old-refresh","account_id":"account-41"}`}
			newOAuthKey := `{"access_token":"oauth-access","refresh_token":"oauth-refresh","account_id":"account-41"}`
			newConfiguration := map[string]any{"key": newOAuthKey}
			destinationChanged := scenario == "OAuth changes destination before refresh" || scenario == "OAuth changes destination during refresh"
			var newDestinationCalls atomic.Int32
			if destinationChanged {
				proxyURL := "http://report-refresh-race.example.test:8080"
				setting, settingErr := common.Marshal(map[string]string{"proxy": proxyURL})
				require.NoError(t, settingErr)
				newConfiguration["base_url"], newConfiguration["setting"] = "https://new-usage.example.test", string(setting)
				proxyClient, proxyErr := GetHttpClientWithProxy(proxyURL)
				require.NoError(t, proxyErr)
				previousProxyTransport := proxyClient.Transport
				t.Cleanup(func() {
					proxyClient.Transport = previousProxyTransport
					InvalidateProxyClient(proxyURL)
				})
				proxyClient.Transport = codexReportTestTransport(func(request *http.Request) (*http.Response, error) {
					newDestinationCalls.Add(1)
					assert.Equal(t, "new-usage.example.test", request.URL.Host)
					assert.Equal(t, "/backend-api/wham/usage", request.URL.Path)
					assert.Equal(t, "Bearer oauth-access", request.Header.Get("Authorization"))
					return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"rate_limit":{"allowed":true}}`)), Request: request}, nil
				})
			}
			require.NoError(t, database.Create(channel).Error)
			require.NoError(t, database.Create(&model.Log{ChannelId: 41, Type: model.LogTypeConsume, CreatedAt: 100, PromptTokens: 3, CompletionTokens: 2, Quota: 9}).Error)
			client, err := GetHttpClientWithProxy("")
			require.NoError(t, err)
			previousTransport := client.Transport
			t.Cleanup(func() { client.Transport = previousTransport })
			var usageCalls, refreshCalls atomic.Int32
			client.Transport = codexReportTestTransport(func(request *http.Request) (*http.Response, error) {
				status, body := 200, `{"rate_limit":{"allowed":true,"primary_window":{"used_percent":0,"limit_window_seconds":18000}}}`
				if request.URL.Path == "/backend-api/wham/usage" {
					usageCalls.Add(1)
					if destinationChanged {
						assert.Equal(t, "usage.example.test", request.URL.Host)
						assert.Equal(t, "Bearer old-access", request.Header.Get("Authorization"), "new credentials must never reach the old destination")
					}
					if scenario == "network fails" {
						return nil, errors.New("secret-access-token in network failure")
					} else if scenario == "upstream unavailable" {
						status, body = 503, `{}`
					} else if scenario == "oversized usage" {
						body = strings.Repeat("x", (1<<20)+1)
					} else if request.Header.Get("Authorization") == "Bearer old-access" || scenario == "retry rejected" {
						status, body = http.StatusUnauthorized, `{}`
						if scenario == "concurrent OAuth replaces key" || scenario == "OAuth changes destination before refresh" {
							if err := database.Model(&model.Channel{}).Where("id = ?", channel.Id).Updates(newConfiguration).Error; err != nil {
								return nil, err
							}
						}
						if scenario == "forbidden refresh succeeds" {
							status = http.StatusForbidden
						}
					}
				} else {
					refreshCalls.Add(1)
					body = `{"access_token":"new-access","refresh_token":"new-refresh","expires_in":3600}`
					if scenario == "refresh rejected" {
						status, body = 403, `{}`
					} else if scenario == "persistence fails" {
						if err := database.Migrator().DropTable(&model.Channel{}); err != nil {
							return nil, err
						}
					} else if scenario == "OAuth replaces key during refresh" || scenario == "OAuth changes destination during refresh" {
						if err := database.Model(&model.Channel{}).Where("id = ?", channel.Id).Updates(newConfiguration).Error; err != nil {
							return nil, err
						}
					}
				}
				return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: request}, nil
			})
			report, err := GetCodexLimitReport(context.Background(), 100, 200)
			require.NoError(t, err)
			require.Len(t, report.Rows, 1)
			assert.Equal(t, int64(5), report.TotalTokenUsed)
			assert.Equal(t, int64(9), report.TotalQuota)
			switch scenario {
			case "refresh succeeds", "forbidden refresh succeeds":
				assert.True(t, report.Rows[0].Success)
				assert.Equal(t, int32(2), usageCalls.Load())
				cached, cacheErr := model.CacheGetChannel(41)
				require.NoError(t, cacheErr)
				assert.Contains(t, cached.Key, "new-refresh")
			case "refresh rejected", "persistence fails":
				assert.False(t, report.Rows[0].Success)
				assert.Equal(t, "Failed to refresh Codex credential", report.Rows[0].Message)
				assert.Equal(t, int32(1), usageCalls.Load())
			case "retry rejected":
				assert.False(t, report.Rows[0].Success)
				assert.Equal(t, 401, report.Rows[0].UpstreamStatus)
				assert.Equal(t, int32(2), usageCalls.Load())
			case "oversized usage", "upstream unavailable", "network fails":
				assert.False(t, report.Rows[0].Success)
				assert.Equal(t, int32(1), usageCalls.Load())
				assert.Equal(t, int32(0), refreshCalls.Load())
			case "concurrent OAuth replaces key", "OAuth replaces key during refresh", "OAuth changes destination before refresh", "OAuth changes destination during refresh":
				assert.True(t, report.Rows[0].Success)
				if destinationChanged {
					assert.Equal(t, int32(1), usageCalls.Load())
					assert.Equal(t, int32(1), newDestinationCalls.Load())
				} else {
					assert.Equal(t, int32(2), usageCalls.Load())
				}
				if scenario == "concurrent OAuth replaces key" || scenario == "OAuth changes destination before refresh" {
					assert.Equal(t, int32(0), refreshCalls.Load())
				} else {
					assert.Equal(t, int32(1), refreshCalls.Load())
				}
				persisted, persistErr := model.GetChannelById(channel.Id, true)
				require.NoError(t, persistErr)
				assert.Equal(t, newOAuthKey, persisted.Key)
				assert.Equal(t, 2, persisted.Status)
				cached, cacheErr := model.CacheGetChannel(channel.Id)
				require.NoError(t, cacheErr)
				assert.Equal(t, newOAuthKey, cached.Key)
				if destinationChanged {
					assert.Equal(t, "https://new-usage.example.test", cached.GetBaseURL())
					assert.Equal(t, "http://report-refresh-race.example.test:8080", cached.GetSetting().Proxy)
				}
			}
			assert.LessOrEqual(t, refreshCalls.Load(), int32(1))
		})
	}
}

func TestGetCodexLimitReportRejectsInvalidTimeRanges(t *testing.T) {
	for _, timestamps := range [][2]int64{{0, 200}, {200, 100}, {100, 100 + 91*24*3600}} {
		_, err := GetCodexLimitReport(context.Background(), timestamps[0], timestamps[1])
		require.ErrorIs(t, err, ErrCodexLimitReportRange)
	}
}
