package codex

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestFullFingerprintValidatesOriginalTypedMetadata(t *testing.T) {
	t.Setenv("CODEX_FINGERPRINT_DEPLOYMENT_NAMESPACE", "typed-metadata-test")
	for _, mode := range []int{relayconstant.RelayModeResponses, relayconstant.RelayModeResponsesCompact} {
		for _, testCase := range []struct {
			name string
			raw  string
		}{
			{name: "malformed", raw: `{"session_id":`},
			{name: "null", raw: `null`},
			{name: "array", raw: `[]`},
			{name: "scalar", raw: `false`},
			{name: "oversized", raw: `{"tag":"` + strings.Repeat("x", 64*1024) + `"}`},
			{name: "oversized original whitespace", raw: `{` + strings.Repeat(" ", 64*1024) + `}`},
			{name: "overdepth", raw: strings.Repeat(`{"nested":`, 11) + `false` + strings.Repeat(`}`, 11)},
			{name: "duplicate identity", raw: `{"session_id":"one","session_id":"two"}`},
		} {
			t.Run(fmt.Sprintf("mode=%d/%s", mode, testCase.name), func(t *testing.T) {
				info := fingerprintTestInfo(dto.CodexFingerprintModeFull)
				info.RelayMode = mode
				converted, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(nil, info, dto.OpenAIResponsesRequest{
					Model: "gpt-5.5", Input: json.RawMessage(`"hello"`), ClientMetadata: json.RawMessage(testCase.raw),
				})
				require.Error(t, err)
				require.Nil(t, converted)
				require.NotContains(t, err.Error(), fingerprintTestSeed)
			})
		}
	}
}

func TestFullFingerprintTypedMetadataBoundaries(t *testing.T) {
	t.Setenv("CODEX_FINGERPRINT_DEPLOYMENT_NAMESPACE", "typed-metadata-test")
	for _, mode := range []int{relayconstant.RelayModeResponses, relayconstant.RelayModeResponsesCompact} {
		for _, testCase := range []struct {
			name       string
			metadata   string
			cacheKey   string
			rewriteKey bool
		}{
			{name: "missing", cacheKey: "independent-cache"},
			{name: "session cache", metadata: `{"session_id":" client ","tag":"private"}`, cacheKey: "client", rewriteKey: true},
			{name: "independent cache", metadata: `{"session_id":"client","tag":"private"}`, cacheKey: "independent-cache"},
			{name: "64 KiB", metadata: `{"tag":"` + strings.Repeat("x", 64*1024-len(`{"tag":""}`)) + `"}`, cacheKey: "independent-cache"},
			{name: "depth 10", metadata: strings.Repeat(`{"nested":`, 10) + `false` + strings.Repeat(`}`, 10), cacheKey: "independent-cache"},
		} {
			t.Run(fmt.Sprintf("mode=%d/%s", mode, testCase.name), func(t *testing.T) {
				info := fingerprintTestInfo(dto.CodexFingerprintModeFull)
				info.RelayMode = mode
				cacheKey, err := common.Marshal(testCase.cacheKey)
				require.NoError(t, err)
				converted, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(nil, info, dto.OpenAIResponsesRequest{
					Model: "gpt-5.5", Input: json.RawMessage(`"hello"`),
					ClientMetadata: json.RawMessage(testCase.metadata), PromptCacheKey: cacheKey,
				})
				require.NoError(t, err)
				wire, err := common.Marshal(converted)
				require.NoError(t, err)
				if mode == relayconstant.RelayModeResponsesCompact {
					assert.False(t, gjson.GetBytes(wire, "client_metadata").Exists())
					assert.Equal(t, testCase.cacheKey, gjson.GetBytes(wire, "prompt_cache_key").String())
					return
				}
				fingerprint, err := ResolveCodexFingerprint(info, "client", time.Unix(1700000000, 0))
				require.NoError(t, err)
				metadata := gjson.GetBytes(wire, "client_metadata")
				assert.Len(t, metadata.Map(), 6)
				assert.Equal(t, fingerprint.InstallationID, metadata.Get("x-codex-installation-id").String())
				assert.Equal(t, fingerprint.SessionID, metadata.Get("session_id").String())
				assert.Equal(t, fingerprint.ThreadID, metadata.Get("thread_id").String())
				assert.False(t, metadata.Get("tag").Exists())
				assert.False(t, metadata.Get("nested").Exists())
				if testCase.rewriteKey {
					assert.Equal(t, fingerprint.SessionID, gjson.GetBytes(wire, "prompt_cache_key").String())
				} else {
					assert.Equal(t, testCase.cacheKey, gjson.GetBytes(wire, "prompt_cache_key").String())
				}
			})
		}
	}
}

const fingerprintTestSeed = "018f89db-7792-7b5e-a360-7fd9279fd725"

func fingerprintTestInfo(mode string) *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{
		CodexFingerprintSeed: fingerprintTestSeed,
		ChannelSetting:       dto.ChannelSettings{CodexFingerprintMode: mode},
	}}
}

func TestCodexFingerprintDefaultsToOff(t *testing.T) {
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{}}
	require.Equal(t, dto.CodexFingerprintModeOff, fingerprintMode(info))
	ids, err := resolveFingerprintIDsForRequest(info, "client", time.Now())
	require.NoError(t, err)
	require.Nil(t, ids)
}

func TestCodexFingerprintRequiresDeploymentNamespace(t *testing.T) {
	t.Setenv("CODEX_FINGERPRINT_DEPLOYMENT_NAMESPACE", "")
	_, err := ResolveCodexFingerprint(fingerprintTestInfo(dto.CodexFingerprintModeDevice), "client", time.Unix(1700000000, 0))
	require.Error(t, err)
	require.NotContains(t, err.Error(), fingerprintTestSeed)
}

func TestCodexFingerprintModesAreStableAndRequestScoped(t *testing.T) {
	t.Setenv("CODEX_FINGERPRINT_DEPLOYMENT_NAMESPACE", "test-deployment")
	now := time.Unix(1700000000, 0)
	first, err := ResolveCodexFingerprint(fingerprintTestInfo(dto.CodexFingerprintModeSession), "client-a", now)
	require.NoError(t, err)
	second, err := ResolveCodexFingerprint(fingerprintTestInfo(dto.CodexFingerprintModeSession), "client-b", now)
	require.NoError(t, err)
	require.Equal(t, first.InstallationID, second.InstallationID)
	require.Equal(t, first.SessionID, second.SessionID)
	require.NotEqual(t, first.ThreadID, second.ThreadID)
	require.Equal(t, now.UnixMilli(), first.StartedAtMS)
	require.NotEqual(t, first.TurnID, second.TurnID)
	require.True(t, uuid.Validate(first.TurnID) == nil)
}

func TestCodexFingerprintBodyAndHeadersUseSameIdentity(t *testing.T) {
	t.Setenv("CODEX_FINGERPRINT_DEPLOYMENT_NAMESPACE", "test-deployment")
	info := fingerprintTestInfo(dto.CodexFingerprintModeFull)
	ids, err := resolveFingerprintIDsForRequest(info, "client", time.Unix(1700000000, 0))
	require.NoError(t, err)

	body := map[string]any{"model": "gpt-5", "client_metadata": map[string]any{"session_id": "client"}}
	require.True(t, applyFingerprintBody(body, ids))
	metadata := body["client_metadata"].(map[string]any)
	require.Equal(t, ids.installationID, metadata["x-codex-installation-id"])
	require.Equal(t, ids.threadID, metadata["thread_id"])

	headers := http.Header{}
	applyFingerprintHeaders(headers, ids)
	require.Equal(t, ids.installationID, headers.Get("x-codex-installation-id"))
	require.Equal(t, ids.threadID, headers.Get("thread-id"))
	require.Equal(t, ids.sessionID, headers.Get("session-id"))
}

func TestCodexFingerprintPassThroughReusesAttemptIdentity(t *testing.T) {
	t.Setenv("CODEX_FINGERPRINT_DEPLOYMENT_NAMESPACE", "test-deployment")
	gin.SetMode(gin.TestMode)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	context.Request.Header.Set("session-id", "client-session")
	info := fingerprintTestInfo(dto.CodexFingerprintModeFull)

	first, err := fingerprintIDsForRequest(context, info)
	require.NoError(t, err)
	second, err := fingerprintIDsForRequest(context, info)
	require.NoError(t, err)
	require.Same(t, first, second)
	require.NotEmpty(t, first.turnID)
}

func TestCodexFingerprintPassThroughStartsNewAttemptIdentity(t *testing.T) {
	t.Setenv("CODEX_FINGERPRINT_DEPLOYMENT_NAMESPACE", "test-deployment")
	gin.SetMode(gin.TestMode)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	info := fingerprintTestInfo(dto.CodexFingerprintModeFull)
	adaptor := &Adaptor{}
	raw := []byte(`{"model":"gpt-5"}`)

	_, _, err := adaptor.RewritePassThroughBody(context, info, raw)
	require.NoError(t, err)
	first, err := fingerprintIDsForRequest(context, info)
	require.NoError(t, err)
	_, _, err = adaptor.RewritePassThroughBody(context, info, raw)
	require.NoError(t, err)
	second, err := fingerprintIDsForRequest(context, info)
	require.NoError(t, err)
	require.NotEqual(t, first.turnID, second.turnID)
}

func TestCodexFingerprintRejectsDuplicateKeysInFullBody(t *testing.T) {
	t.Setenv("CODEX_FINGERPRINT_DEPLOYMENT_NAMESPACE", "test-deployment")
	info := fingerprintTestInfo(dto.CodexFingerprintModeFull)
	fingerprint, err := ResolveCodexFingerprint(info, "client", time.Unix(1700000000, 0))
	require.NoError(t, err)
	_, err = SanitizeCodexRequestBody([]byte(`{"model":"gpt-5","model":"gpt-5-mini"}`), fingerprint, dto.CodexFingerprintModeFull)
	require.Error(t, err)
}

func TestCodexFingerprintFinalizerDoesNotReuseAnotherChannelIdentity(t *testing.T) {
	t.Setenv("CODEX_FINGERPRINT_DEPLOYMENT_NAMESPACE", "channel-isolation-test")
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	first := fingerprintTestInfo(dto.CodexFingerprintModeFull)
	first.ChannelId = 1
	converted, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(context, first, dto.OpenAIResponsesRequest{
		Model: "gpt-5.5", Input: json.RawMessage(`"hello"`),
	})
	require.NoError(t, err)
	wire, err := common.Marshal(converted)
	require.NoError(t, err)
	firstInstallation := gjson.GetBytes(wire, "client_metadata.x-codex-installation-id").String()
	require.NotEmpty(t, firstInstallation)
	second := fingerprintTestInfo(dto.CodexFingerprintModeFull)
	second.ChannelId = 2
	second.CodexFingerprintSeed = "018f89db-7792-7b5e-a360-7fd9279fd726"
	second.ApiKey = `{"access_token":"test-access","account_id":"test-account"}`
	request := httptest.NewRequest(http.MethodPost, "https://codex.test/backend-api/codex/responses", nil)
	require.NoError(t, (&Adaptor{}).FinalizeRequest(context, second, request))
	fingerprint, err := ResolveCodexFingerprint(second, "", time.Unix(1700000000, 0))
	require.NoError(t, err)
	assert.Equal(t, fingerprint.InstallationID, request.Header.Get("x-codex-installation-id"))
	assert.NotEqual(t, firstInstallation, request.Header.Get("x-codex-installation-id"))
}
