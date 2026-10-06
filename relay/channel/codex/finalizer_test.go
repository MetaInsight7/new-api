package codex

import (
	"bytes"
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
	"github.com/stretchr/testify/require"
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
