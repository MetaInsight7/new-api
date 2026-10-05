package codex

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

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
