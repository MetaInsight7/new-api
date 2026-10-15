package service

import (
	"context"
	"crypto/hmac"
	cryptorand "crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"maps"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/iotest"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupCodexGovernanceServiceTest(t *testing.T, handler http.HandlerFunc) *gorm.DB {
	t.Helper()
	database, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "governance.db")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := database.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, database.AutoMigrate(&model.Option{}, &model.Channel{}, &model.Ability{}, &model.CodexModelGovernanceRecord{}, &model.CodexModelGovernanceProbeState{}, &model.CodexModelGovernanceAlertCooldownRecord{}))
	previousDB, previousType, previousCache := model.DB, common.MainDatabaseType(), common.MemoryCacheEnabled
	setting := operation_setting.GetCodexModelGovernanceSetting()
	previousSetting := *setting
	previousMonitor := *operation_setting.GetMonitorSetting()
	previousOptions := common.OptionMap
	channelAlertLocalCooldown.Lock()
	previousCooldowns := maps.Clone(channelAlertLocalCooldown.records)
	channelAlertLocalCooldown.records = make(map[int]localChannelAlertCooldown)
	channelAlertLocalCooldown.Unlock()
	common.OptionMap = make(map[string]string)
	previousClient := ssrfProtectedHTTPClient
	model.DB = database
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	common.MemoryCacheEnabled = false
	configureSSRFTestFetchSetting(t)
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	ssrfProtectedHTTPClient = newProtectedFetchHTTPClientWithProxy(nil, func(ctx context.Context, network, address string) (net.Conn, error) {
		assert.Equal(t, "93.184.216.34:80", address)
		return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
	}, currentFetchProtection, func(*http.Request) (*url.URL, error) { return nil, nil })
	t.Cleanup(func() {
		channelAlertLocalCooldown.Lock()
		channelAlertLocalCooldown.records = previousCooldowns
		channelAlertLocalCooldown.Unlock()
		ssrfProtectedHTTPClient.CloseIdleConnections()
		ssrfProtectedHTTPClient = previousClient
		*setting = previousSetting
		*operation_setting.GetMonitorSetting() = previousMonitor
		common.OptionMap = previousOptions
		model.DB = previousDB
		common.SetMainDatabaseType(previousType)
		common.MemoryCacheEnabled = previousCache
	})
	return database
}

func TestCodexGovernanceRulesAndSettingsRejectNoise(t *testing.T) {
	setting := operation_setting.GetCodexModelGovernanceSetting()
	assert.False(t, setting.Enabled)
	assert.False(t, setting.ProbeEnabled)
	assert.False(t, operation_setting.GetMonitorAlertSetting().DingTalkAlertEnabled)
	assert.Empty(t, operation_setting.GetMonitorAIAnalysisAPIKey())
	patterns := []string{operation_setting.DefaultCodexUnsupportedPattern}
	for _, message := range []string{"model not found", "rate limit exceeded", "context deadline exceeded", "The 'gpt-5.4' model is not supported by this endpoint", "invalid API key"} {
		assert.False(t, ClassifyCodexUnsupportedMessage(message, patterns).Matched, message)
	}
	match := ClassifyCodexUnsupportedMessage("The 'gpt-5.4' model is not supported when using Codex with a ChatGPT account.", patterns)
	assert.True(t, match.Matched)
	assert.Equal(t, "gpt-5.4", match.ModelName)
	findings := ExtractCodexOfficialNoticeFindings("gpt-5.4 was retired. gpt-5.4-mini remains available.", []string{"gpt-5.4", "gpt-5.4", "gpt-5.4-mini"}, []string{"retired"})
	require.Len(t, findings, 1)
	assert.Equal(t, "gpt-5.4", findings[0].ModelName)
	for _, option := range []struct {
		field string
		value any
	}{
		{"unsupported_message_patterns", []string{"["}}, {"probe_interval_minutes", 59}, {"enabled", "maybe"},
		{"official_source_urls", []string{"file:///etc/passwd"}}, {"ai_analysis_base_url", "https://user:password@example.com"}, {"unknown_field", true},
	} {
		_, err := operation_setting.NormalizeCodexModelGovernanceOption("codex_model_governance_setting."+option.field, option.value)
		assert.Error(t, err, option.field)
	}
	value, err := operation_setting.NormalizeCodexModelGovernanceOption("codex_model_governance_setting.official_source_urls", "https://example.com/notice\n\n")
	require.NoError(t, err)
	assert.Equal(t, `["https://example.com/notice"]`, value)
}

func TestCodexGovernanceOfficialFetchUsesProtectionAndBounds(t *testing.T) {
	setupCodexGovernanceServiceTest(t, func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/notice":
			_, _ = io.WriteString(writer, "gpt-5.4 was retired.")
		case "/large":
			_, _ = io.WriteString(writer, strings.Repeat("a", int(codexOfficialSourceMaxBodyBytes)+1))
		case "/redirect":
			http.Redirect(writer, request, "http://127.0.0.1/secret", http.StatusFound)
		default:
			writer.WriteHeader(http.StatusBadGateway)
		}
	})
	body, err := FetchCodexOfficialSource(context.Background(), "http://93.184.216.34/notice")
	require.NoError(t, err)
	assert.Equal(t, "gpt-5.4 was retired.", body)
	for _, source := range []string{"http://127.0.0.1/notice", "http://93.184.216.34/large", "http://93.184.216.34/redirect", "http://93.184.216.34/failure"} {
		_, err := FetchCodexOfficialSource(context.Background(), source)
		assert.Error(t, err)
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = FetchCodexOfficialSource(cancelled, "http://93.184.216.34/notice")
	assert.Error(t, err)
}

func TestCodexGovernanceAIProducesReviewOnlyAndFallsBack(t *testing.T) {
	var requests atomic.Int32
	fail := false
	database := setupCodexGovernanceServiceTest(t, func(writer http.ResponseWriter, request *http.Request) {
		requests.Add(1)
		assert.Equal(t, "/v1/responses", request.URL.Path)
		assert.Equal(t, "Bearer fake-analysis-key", request.Header.Get("Authorization"))
		var payload codexOfficialNoticeAIRequest
		require.NoError(t, common.DecodeJson(request.Body, &payload))
		assert.Equal(t, "test-analysis-model", payload.Model)
		assert.Equal(t, "json_schema", payload.Text.Format.Type)
		assert.True(t, payload.Text.Format.Strict)
		if fail {
			writer.WriteHeader(http.StatusServiceUnavailable)
			_, _ = io.WriteString(writer, `{"error":"sk-sensitive-response"}`)
			return
		}
		result := codexOfficialNoticeAIResult{Findings: []codexOfficialNoticeAIModelFinding{
			{ModelName: "gpt-5.4", LifecycleTerm: "retired", Evidence: "gpt-5.4 retired; api_key=sk-sensitive-evidence"},
			{ModelName: "gpt-5.4", LifecycleTerm: "retired", Evidence: "duplicate"},
			{ModelName: "other-model", LifecycleTerm: "retired", Evidence: "not a candidate"},
		}}
		output, err := common.Marshal(result)
		require.NoError(t, err)
		body, err := common.Marshal(map[string]string{"output_text": string(output)})
		require.NoError(t, err)
		_, _ = writer.Write(body)
	})
	channel := model.Channel{Id: 21, Type: constant.ChannelTypeCodex, Status: 1, Models: "gpt-5.4", Group: "default"}
	require.NoError(t, database.Create(&channel).Error)
	require.NoError(t, channel.AddAbilities(nil))
	options := CodexOfficialNoticeAIOptions{APIKey: "fake-analysis-key", BaseURL: "http://93.184.216.34/v1", Model: "test-analysis-model"}
	findings, usedAI, err := ExtractCodexOfficialNoticeFindingsWithOptionalAIWithOptions(context.Background(), "gpt-5.4 was retired.", []string{"gpt-5.4"}, []string{"retired"}, "https://official.example/notice", options)
	require.NoError(t, err)
	assert.True(t, usedAI)
	require.Len(t, findings, 1)
	assert.NotContains(t, findings[0].LastError, "sk-sensitive-evidence")
	record, err := MoveCodexModelToPendingReview(findings[0])
	require.NoError(t, err)
	assert.False(t, record.AbilitiesDisabled)
	assert.Zero(t, record.LastAlertedAt)
	var ability model.Ability
	require.NoError(t, database.First(&ability).Error)
	assert.True(t, ability.Enabled)
	fail = true
	findings, usedAI, err = ExtractCodexOfficialNoticeFindingsWithOptionalAIWithOptions(context.Background(), "gpt-5.4 was retired.", []string{"gpt-5.4"}, []string{"retired"}, "", options)
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "sk-sensitive-response")
	assert.True(t, usedAI)
	require.Len(t, findings, 1)
	options.APIKey = ""
	_, usedAI, err = ExtractCodexOfficialNoticeFindingsWithOptionalAIWithOptions(context.Background(), "gpt-5.4 was retired.", []string{"gpt-5.4"}, []string{"retired"}, "", options)
	require.NoError(t, err)
	assert.False(t, usedAI)
	assert.EqualValues(t, 2, requests.Load())
}

func TestCodexGovernanceDingTalkPersistsOnlyConfirmedDelivery(t *testing.T) {
	var deliveries atomic.Int32
	response := `{"errcode":500}`
	database := setupCodexGovernanceServiceTest(t, func(writer http.ResponseWriter, request *http.Request) {
		deliveries.Add(1)
		query := request.URL.Query()
		mac := hmac.New(sha256.New, []byte("fake-signing-secret"))
		_, _ = mac.Write([]byte(query.Get("timestamp") + "\n" + "fake-signing-secret"))
		assert.Equal(t, base64.StdEncoding.EncodeToString(mac.Sum(nil)), query.Get("sign"))
		body, err := io.ReadAll(request.Body)
		require.NoError(t, err)
		assert.NotContains(t, string(body), "sk-sensitive-evidence")
		_, _ = io.WriteString(writer, response)
	})
	require.NoError(t, model.UpdateOptionsBulk(map[string]string{
		"monitor_setting.dingtalk_alert_enabled":     "false",
		"monitor_setting.dingtalk_alert_webhook_url": "http://93.184.216.34/dingtalk?access_token=fake-webhook-token",
		"monitor_setting.dingtalk_alert_secret":      "fake-signing-secret",
	}))
	finding := CodexModelUnsupportedFinding{ModelName: "gpt-5.4", Source: model.CodexModelGovernanceSourceOfficialNotice, LastError: "api_key=sk-sensitive-evidence"}
	record, err := MoveCodexModelToPendingReview(finding)
	require.NoError(t, err)
	assert.Zero(t, record.LastAlertedAt)
	assert.Zero(t, deliveries.Load())
	require.NoError(t, model.UpdateOption("monitor_setting.dingtalk_alert_enabled", "true"))
	record, err = MoveCodexModelToPendingReview(finding)
	require.NoError(t, err)
	assert.Zero(t, record.LastAlertedAt)
	assert.EqualValues(t, 1, deliveries.Load())
	response = `{"errcode":0}`
	record, err = MoveCodexModelToPendingReview(finding)
	require.NoError(t, err)
	assert.Positive(t, record.LastAlertedAt)
	assert.EqualValues(t, 2, deliveries.Load())
	record, err = MoveCodexModelToPendingReview(finding)
	require.NoError(t, err)
	assert.Positive(t, record.LastAlertedAt)
	sent, err := NotifyDingTalkCodexModelGovernance(context.Background(), record)
	require.NoError(t, err)
	assert.False(t, sent)
	assert.EqualValues(t, 2, deliveries.Load())
	var stored model.CodexModelGovernanceRecord
	require.NoError(t, database.First(&stored, record.ID).Error)
	assert.Equal(t, record.LastAlertedAt, stored.LastAlertedAt)
	require.NoError(t, model.UpdateOption("monitor_setting.dingtalk_alert_webhook_url", "http://127.0.0.1/internal"))
	_, err = NotifyDingTalkCodexModelGovernance(context.Background(), record)
	assert.Error(t, err)
	assert.True(t, system_setting.GetFetchSetting().EnableSSRFProtection)
}

func TestChannelTestAlertFallbackAndTokenFailure(t *testing.T) {
	var deliveries atomic.Int32
	database := setupCodexGovernanceServiceTest(t, func(writer http.ResponseWriter, request *http.Request) {
		deliveries.Add(1)
		_, _ = io.WriteString(writer, `{"errcode":0}`)
	})
	require.False(t, database.Migrator().HasTable(&model.ChannelTestAlertCooldown{}))
	require.NoError(t, model.UpdateOptionsBulk(map[string]string{
		"monitor_setting.dingtalk_alert_enabled":     "true",
		"monitor_setting.dingtalk_alert_webhook_url": "http://93.184.216.34/dingtalk",
	}))
	operation_setting.GetMonitorSetting().DingTalkAlertCooldownMinutes = 0
	for range 2 {
		require.NoError(t, NotifyDingTalkChannelTestFailures(context.Background(), []ChannelTestFailure{{ChannelID: 801, Message: "unavailable"}}))
	}
	assert.EqualValues(t, 1, deliveries.Load(), "database failure must retain process-local cooldown; zero must not disable it")
	previousRandom := cryptorand.Reader
	cryptorand.Reader = iotest.ErrReader(errors.New("random source unavailable"))
	t.Cleanup(func() { cryptorand.Reader = previousRandom })
	require.Error(t, NotifyDingTalkChannelTestFailures(context.Background(), []ChannelTestFailure{{ChannelID: 802, Message: "unavailable"}}))
	assert.EqualValues(t, 1, deliveries.Load(), "token failure must not bypass reservations")
}

type channelAlertBlockingRandomReader struct {
	original io.Reader
	blocked  atomic.Bool
	started  chan struct{}
	release  chan struct{}
}

func (reader *channelAlertBlockingRandomReader) Read(buffer []byte) (int, error) {
	if reader.blocked.CompareAndSwap(false, true) {
		close(reader.started)
		<-reader.release
	}
	return reader.original.Read(buffer)
}

func TestChannelTestAlertKeepsAnalysisConnectionSnapshot(t *testing.T) {
	var analyses atomic.Int32
	database := setupCodexGovernanceServiceTest(t, func(writer http.ResponseWriter, request *http.Request) {
		if strings.HasSuffix(request.URL.Path, "/chat/completions") {
			analyses.Add(1)
			assert.Equal(t, "/old/chat/completions", request.URL.Path)
			assert.Equal(t, "Bearer old-key", request.Header.Get("Authorization"))
			body, err := io.ReadAll(request.Body)
			require.NoError(t, err)
			assert.Contains(t, string(body), `"model":"old-model"`)
			_, _ = io.WriteString(writer, `{"choices":[{"message":{"content":"- 上游不可用"}}]}`)
			return
		}
		_, _ = io.WriteString(writer, `{"errcode":0}`)
	})
	require.NoError(t, database.AutoMigrate(&model.ChannelTestAlertCooldown{}))
	require.NoError(t, model.UpdateOptionsBulk(map[string]string{
		"monitor_setting.dingtalk_alert_enabled": "true", "monitor_setting.dingtalk_alert_webhook_url": "http://93.184.216.34/robot",
		"monitor_setting.ai_analysis_api_key": "old-key", "monitor_setting.ai_analysis_base_url": "http://93.184.216.34/old", "monitor_setting.ai_analysis_model": "old-model",
	}))
	reader := &channelAlertBlockingRandomReader{original: cryptorand.Reader, started: make(chan struct{}), release: make(chan struct{})}
	cryptorand.Reader = reader
	t.Cleanup(func() { cryptorand.Reader = reader.original })
	done := make(chan error, 1)
	go func() {
		done <- NotifyDingTalkChannelTestFailures(context.Background(), []ChannelTestFailure{{ChannelID: 809, Message: "unavailable"}})
		close(done)
	}()
	defer func() { close(reader.release); <-done }()
	select {
	case <-reader.started:
	case <-time.After(5 * time.Second):
		t.Fatal("reservation token generation did not start")
	}
	require.NoError(t, model.UpdateOptionsBulk(map[string]string{
		"monitor_setting.ai_analysis_api_key": "new-key", "monitor_setting.ai_analysis_base_url": "http://93.184.216.34/new", "monitor_setting.ai_analysis_model": "new-model",
	}))
	reader.release <- struct{}{}
	require.NoError(t, <-done)
	assert.EqualValues(t, 1, analyses.Load())
}

func TestChannelTestAlertCancellationDuringDNSPreflight(t *testing.T) {
	for _, stage := range []string{"analysis", "robot"} {
		t.Run(stage, func(t *testing.T) {
			var requests atomic.Int32
			database := setupCodexGovernanceServiceTest(t, func(writer http.ResponseWriter, request *http.Request) { requests.Add(1) })
			require.NoError(t, database.AutoMigrate(&model.ChannelTestAlertCooldown{}))
			options := map[string]string{"monitor_setting.dingtalk_alert_enabled": "true", "monitor_setting.dingtalk_alert_webhook_url": "http://slow-robot.example/robot"}
			if stage == "analysis" {
				options["monitor_setting.ai_analysis_api_key"] = "analysis-key"
				options["monitor_setting.ai_analysis_base_url"] = "http://slow-analysis.example/v1"
			}
			require.NoError(t, model.UpdateOptionsBulk(options))
			started, release := make(chan struct{}), make(chan struct{})
			var first sync.Once
			previousResolver := net.DefaultResolver
			net.DefaultResolver = &net.Resolver{PreferGo: true, Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				first.Do(func() { close(started) })
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-release:
					return nil, errors.New("DNS fixture released")
				}
			}}
			t.Cleanup(func() { net.DefaultResolver = previousResolver })
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan error, 1)
			go func() {
				done <- NotifyDingTalkChannelTestFailures(ctx, []ChannelTestFailure{{ChannelID: 810, Message: "unavailable"}})
				close(done)
			}()
			defer func() { close(release); <-done }()
			select {
			case <-started:
			case <-time.After(5 * time.Second):
				t.Fatal("DNS preflight did not start")
			}
			cancel()
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Error("task cancellation did not abort DNS preflight")
			}
			assert.Zero(t, requests.Load())
		})
	}
}
