package codex

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestFinalizeRequestRestoresTrustedCodexHeaders(t *testing.T) {
	previous := common.OptionMap
	common.OptionMapRWMutex.Lock()
	common.OptionMap = map[string]string{
		service.OptionKeyCodexClientVersion:         "0.145.6",
		service.OptionKeyCodexEnforceClientIdentity: "true",
	}
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = previous
		common.OptionMapRWMutex.Unlock()
	})

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ApiKey:      `{"access_token":"trusted-access","account_id":"trusted-account"}`,
			ChannelType: constant.ChannelTypeCodex,
			ChannelSetting: dto.ChannelSettings{
				CodexFingerprintMode: dto.CodexFingerprintModeOff,
			},
		},
	}
	req := httptest.NewRequest(http.MethodPost, "https://chatgpt.example/backend-api/codex/responses", nil)
	req.Header.Set("Authorization", "Bearer attacker")
	req.Header.Set("ChatGPT-Account-Id", "attacker-account")
	req.Header.Set("OpenAI-Beta", "attacker-beta")
	req.Header.Set("Originator", "attacker-originator")
	req.Header.Set("User-Agent", "attacker-agent")
	req.Header.Set("OpenAI-Client-Version", "attacker-version")
	req.Header.Set("Cookie", "session=secret")
	req.Header.Set("X-Codex-Installation-Id", "attacker-fingerprint")
	req.Header.Set("Traceparent", "attacker-trace")

	require.NoError(t, (&Adaptor{}).FinalizeRequest(c, info, req))
	require.Equal(t, "Bearer trusted-access", req.Header.Get("Authorization"))
	require.Equal(t, "trusted-account", req.Header.Get("chatgpt-account-id"))
	require.Equal(t, "responses=experimental", req.Header.Get("OpenAI-Beta"))
	require.Equal(t, "codex_cli_rs", req.Header.Get("originator"))
	require.Equal(t, "codex-cli/0.145.6", req.Header.Get("User-Agent"))
	require.Equal(t, "0.145.6", req.Header.Get("OpenAI-Client-Version"))
	require.Empty(t, req.Header.Get("Cookie"))
	require.Empty(t, req.Header.Get("X-Codex-Installation-Id"))
	require.Empty(t, req.Header.Get("Traceparent"))
}

func TestFinalizeCodexRequestBodyReappliesFingerprintAndRejectsDuplicateKeys(t *testing.T) {
	t.Setenv("CODEX_FINGERPRINT_DEPLOYMENT_NAMESPACE", "finalizer-test")
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			CodexFingerprintSeed: "018f89db-7792-7b5e-a360-7fd9279fd725",
			ChannelSetting: dto.ChannelSettings{
				CodexFingerprintMode: dto.CodexFingerprintModeFull,
			},
		},
	}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	fingerprint, err := ResolveCodexFingerprint(info, "attacker", time.Now())
	require.NoError(t, err)
	rewritten, err := SanitizeCodexRequestBody([]byte(`{"client_metadata":{"session_id":"attacker"},"prompt_cache_key":"attacker"}`), fingerprint, dto.CodexFingerprintModeFull)
	require.NoError(t, err)
	require.NotContains(t, string(rewritten), "attacker")
	require.Contains(t, string(rewritten), "x-codex-installation-id")

	_, err = SanitizeCodexRequestBody([]byte(`{"model":"a","model":"b"}`), fingerprint, dto.CodexFingerprintModeFull)
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "duplicate json key"))
}

func TestFinalizeRequestKillSwitchPreservesLegacySoftwareIdentity(t *testing.T) {
	previous := common.OptionMap
	common.OptionMapRWMutex.Lock()
	common.OptionMap = map[string]string{
		service.OptionKeyCodexEnforceClientIdentity: "false",
	}
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = previous
		common.OptionMapRWMutex.Unlock()
	})

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	info := &relaycommon.RelayInfo{
		IsStream: true,
		ChannelMeta: &relaycommon.ChannelMeta{
			ApiKey:         `{"access_token":"trusted-access","account_id":"trusted-account"}`,
			ChannelSetting: dto.ChannelSettings{CodexFingerprintMode: dto.CodexFingerprintModeOff},
		},
	}
	req := httptest.NewRequest(http.MethodPost, "https://chatgpt.example/backend-api/codex/responses", nil)
	req.Header.Set("Authorization", "Bearer attacker")
	req.Header.Set("OpenAI-Beta", "attacker-beta")
	req.Header.Set("User-Agent", "legacy-codex/1.2.3")
	req.Header.Set("originator", "legacy-originator")
	req.Header.Set("OpenAI-Client-Version", "legacy-version")
	req.Header.Set("Cookie", "secret")

	require.NoError(t, (&Adaptor{}).FinalizeRequest(c, info, req))
	require.Equal(t, "Bearer trusted-access", req.Header.Get("Authorization"))
	require.Equal(t, "trusted-account", req.Header.Get("chatgpt-account-id"))
	require.Equal(t, "responses=experimental", req.Header.Get("OpenAI-Beta"))
	require.Equal(t, "application/json", req.Header.Get("Content-Type"))
	require.Equal(t, "text/event-stream", req.Header.Get("Accept"))
	require.Equal(t, "legacy-codex/1.2.3", req.Header.Get("User-Agent"))
	require.Equal(t, "legacy-originator", req.Header.Get("originator"))
	require.Equal(t, "legacy-version", req.Header.Get("OpenAI-Client-Version"))
	require.Empty(t, req.Header.Get("Cookie"))
}

func TestPrepareRequestBodySanitizesCompactMetadataAtEgress(t *testing.T) {
	t.Setenv("CODEX_FINGERPRINT_DEPLOYMENT_NAMESPACE", "finalizer-test")
	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeResponsesCompact,
		ChannelMeta: &relaycommon.ChannelMeta{
			CodexFingerprintSeed: "018f89db-7792-7b5e-a360-7fd9279fd725",
			ChannelSetting:       dto.ChannelSettings{CodexFingerprintMode: dto.CodexFingerprintModeFull},
		},
	}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses/compact", nil)
	body, closer, err := (&Adaptor{}).prepareRequestBody(c, info, bytes.NewReader([]byte(`{"model":"gpt-5","client_metadata":{"session_id":"attacker"},"metadata":{"secret":"attacker"}}`)))
	require.NoError(t, err)
	require.NotNil(t, closer)
	defer closer.Close()
	rewritten, err := io.ReadAll(body)
	require.NoError(t, err)
	require.NotContains(t, string(rewritten), "attacker")
	require.NotContains(t, string(rewritten), "client_metadata")
	require.NotContains(t, string(rewritten), "\"metadata\"")
}

func TestCodexFinalOutboundConstraints(t *testing.T) {
	t.Setenv("CODEX_FINGERPRINT_DEPLOYMENT_NAMESPACE", "outbound-constraints-test")
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		raw, err := io.ReadAll(request.Body)
		if err != nil {
			http.Error(writer, "read request", http.StatusBadRequest)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write(raw)
	}))
	t.Cleanup(server.Close)
	for _, mode := range []int{relayconstant.RelayModeChatCompletions, relayconstant.RelayModeResponses, relayconstant.RelayModeResponsesCompact} {
		for _, fingerprintMode := range []string{dto.CodexFingerprintModeOff, dto.CodexFingerprintModeFull} {
			t.Run(fmt.Sprintf("mode=%d/fingerprint=%s", mode, fingerprintMode), func(t *testing.T) {
				info := fingerprintTestInfo(fingerprintMode)
				info.RelayMode = mode
				info.ChannelType = constant.ChannelTypeCodex
				info.ChannelBaseUrl = server.URL
				info.ApiKey = `{"access_token":"test-access","account_id":"test-account"}`
				info.ChannelSetting.PassThroughBodyEnabled = true
				info.ParamOverride = map[string]any{
					"temperature": 0, "max_completion_tokens": 32, "metadata": map[string]any{"tag": "private"},
					"store": true, "stream": false, "max_tool_calls": 0, "top_logprobs": 0,
					"client_metadata": map[string]any{"session_id": "client"},
				}
				raw := []byte(`{"model":"gpt-5.5","input":"hello","service_tier":"fast","parallel_tool_calls":false,"provider_extension":{"number":9007199254740993,"zero":0,"flag":false}}`)
				raw, err := relaycommon.ApplyParamOverrideWithRelayInfo(raw, info)
				require.NoError(t, err)
				context, _ := gin.CreateTestContext(httptest.NewRecorder())
				context.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
				context.Request.Header.Set("Content-Type", "application/json")
				result, err := (&Adaptor{}).DoRequest(context, info, bytes.NewReader(raw))
				require.NoError(t, err)
				response, ok := result.(*http.Response)
				require.True(t, ok)
				defer response.Body.Close()
				wire, err := io.ReadAll(response.Body)
				require.NoError(t, err)
				for _, field := range []string{"temperature", "max_completion_tokens", "metadata"} {
					assert.False(t, gjson.GetBytes(wire, field).Exists(), field)
				}
				assert.Equal(t, "fast", gjson.GetBytes(wire, "service_tier").String())
				assert.Equal(t, "false", gjson.GetBytes(wire, "parallel_tool_calls").Raw)
				assert.Equal(t, "9007199254740993", gjson.GetBytes(wire, "provider_extension.number").Raw, "wire=%s", wire)
				assert.Equal(t, "0", gjson.GetBytes(wire, "provider_extension.zero").Raw)
				assert.Equal(t, "false", gjson.GetBytes(wire, "provider_extension.flag").Raw)
				assert.Equal(t, `""`, gjson.GetBytes(wire, "instructions").Raw)
				if mode == relayconstant.RelayModeResponsesCompact {
					for _, field := range []string{"store", "stream", "max_tool_calls", "top_logprobs", "client_metadata"} {
						assert.False(t, gjson.GetBytes(wire, field).Exists(), field)
					}
				} else {
					for _, field := range []string{"store", "stream"} {
						assert.Equal(t, "false", gjson.GetBytes(wire, field).Raw, field)
					}
					for _, field := range []string{"max_tool_calls", "top_logprobs"} {
						assert.Equal(t, "0", gjson.GetBytes(wire, field).Raw, field)
					}
				}
			})
		}
	}
}

func TestCodexPassThroughBodyAppliesConstraints(t *testing.T) {
	t.Setenv("CODEX_FINGERPRINT_DEPLOYMENT_NAMESPACE", "passthrough-constraints-test")
	for _, mode := range []int{relayconstant.RelayModeResponses, relayconstant.RelayModeResponsesCompact} {
		for _, fingerprintMode := range []string{dto.CodexFingerprintModeOff, dto.CodexFingerprintModeFull} {
			t.Run(fmt.Sprintf("mode=%d/fingerprint=%s", mode, fingerprintMode), func(t *testing.T) {
				info := fingerprintTestInfo(fingerprintMode)
				info.RelayMode = mode
				info.ChannelSetting.PassThroughBodyEnabled = true
				context, _ := gin.CreateTestContext(httptest.NewRecorder())
				context.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
				raw := []byte(`{"model":"gpt-5.5","input":"hello","service_tier":"fast","client_metadata":{"session_id":"client"},"prompt_cache_key":"client","max_completion_tokens":1,"metadata":{"tag":"private"},"stream":false,"store":true}`)
				wire, changed, err := (&Adaptor{}).RewritePassThroughBody(context, info, raw)
				require.NoError(t, err)
				assert.True(t, changed)
				for _, field := range []string{"max_completion_tokens", "metadata"} {
					assert.False(t, gjson.GetBytes(wire, field).Exists(), field)
				}
				assert.Equal(t, "fast", gjson.GetBytes(wire, "service_tier").String())
				if mode == relayconstant.RelayModeResponsesCompact {
					for _, field := range []string{"client_metadata", "stream", "store"} {
						assert.False(t, gjson.GetBytes(wire, field).Exists(), field)
					}
					assert.Equal(t, "client", gjson.GetBytes(wire, "prompt_cache_key").String())
				} else {
					assert.Equal(t, "false", gjson.GetBytes(wire, "stream").Raw)
					assert.Equal(t, "false", gjson.GetBytes(wire, "store").Raw)
				}
				constrainedAgain, err := ApplyCodexRequestConstraints(wire, mode == relayconstant.RelayModeResponsesCompact)
				require.NoError(t, err)
				assert.Equal(t, string(wire), string(constrainedAgain))
			})
		}
	}
}

func TestCodexConstraintsRejectInvalidOriginalBody(t *testing.T) {
	for _, compact := range []bool{false, true} {
		for _, testCase := range []struct {
			name string
			raw  string
		}{
			{name: "empty"},
			{name: "malformed", raw: `{"metadata":`},
			{name: "root null", raw: `null`},
			{name: "root array", raw: `[]`},
			{name: "root string", raw: `"hello"`},
			{name: "duplicate filtered field", raw: `{"metadata":{},"metadata":{}}`},
			{name: "client metadata null", raw: `{"client_metadata":null}`},
			{name: "client metadata array", raw: `{"client_metadata":[]}`},
			{name: "client metadata oversized", raw: `{"client_metadata":{"tag":"` + strings.Repeat("x", 64*1024) + `"}}`},
			{name: "client metadata overdepth", raw: `{"client_metadata":` + strings.Repeat(`{"nested":`, 11) + `false` + strings.Repeat(`}`, 11) + `}`},
		} {
			t.Run(fmt.Sprintf("compact=%t/%s", compact, testCase.name), func(t *testing.T) {
				wire, err := ApplyCodexRequestConstraints([]byte(testCase.raw), compact)
				require.Error(t, err)
				assert.Nil(t, wire)
			})
		}
	}
}
