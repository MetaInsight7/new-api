package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

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
	require.NoError(t, database.AutoMigrate(&model.Channel{}, &model.Ability{}, &model.CodexModelGovernanceRecord{}, &model.CodexModelGovernanceProbeState{}, &model.CodexModelGovernanceAlertCooldownRecord{}))
	previousDB, previousType, previousCache := model.DB, common.MainDatabaseType(), common.MemoryCacheEnabled
	setting := operation_setting.GetCodexModelGovernanceSetting()
	previousSetting := *setting
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
		ssrfProtectedHTTPClient.CloseIdleConnections()
		ssrfProtectedHTTPClient = previousClient
		*setting = previousSetting
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
	assert.False(t, setting.DingTalkEnabled)
	assert.Empty(t, setting.AIAnalysisAPIKey)
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
	setting := operation_setting.GetCodexModelGovernanceSetting()
	setting.DingTalkEnabled = false
	setting.DingTalkWebhookURL = "http://93.184.216.34/dingtalk?access_token=fake-webhook-token"
	setting.DingTalkSecret = "fake-signing-secret"
	finding := CodexModelUnsupportedFinding{ModelName: "gpt-5.4", Source: model.CodexModelGovernanceSourceOfficialNotice, LastError: "api_key=sk-sensitive-evidence"}
	record, err := MoveCodexModelToPendingReview(finding)
	require.NoError(t, err)
	assert.Zero(t, record.LastAlertedAt)
	assert.Zero(t, deliveries.Load())
	setting.DingTalkEnabled = true
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
	setting.DingTalkWebhookURL = "http://127.0.0.1/internal"
	_, err = NotifyDingTalkCodexModelGovernance(context.Background(), record)
	assert.Error(t, err)
	assert.True(t, system_setting.GetFetchSetting().EnableSSRFProtection)
}
