package service

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func withCodexIdentityOptions(t *testing.T, options map[string]string) {
	t.Helper()
	common.OptionMapRWMutex.Lock()
	previous := common.OptionMap
	common.OptionMap = make(map[string]string, len(options))
	for key, value := range options {
		common.OptionMap[key] = value
	}
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = previous
		common.OptionMapRWMutex.Unlock()
	})
}

func TestNormalizeCodexClientVersion(t *testing.T) {
	for _, testCase := range []struct {
		name string
		raw  string
		want string
		ok   bool
	}{
		{name: "floor", raw: "0.144.0", want: "0.144.0", ok: true},
		{name: "newer", raw: " v0.145.2 ", want: "0.145.2", ok: true},
		{name: "below floor", raw: "0.143.9"},
		{name: "prerelease", raw: "0.144.1-beta.1"},
		{name: "control", raw: "0.144.\n1"},
		{name: "too long", raw: "0.144.1" + strings.Repeat("1", 65)},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			got, ok := NormalizeCodexClientVersion(testCase.raw)
			require.Equal(t, testCase.ok, ok)
			require.Equal(t, testCase.want, got)
		})
	}
}

func TestValidateCodexIdentityOption(t *testing.T) {
	require.NoError(t, ValidateCodexIdentityOption(OptionKeyCodexClientVersion, ""))
	require.NoError(t, ValidateCodexIdentityOption(OptionKeyCodexClientVersion, "0.145.2"))
	require.NoError(t, ValidateCodexIdentityOption(OptionKeyCodexClientUserAgent, "codex-cli/0.145.2 linux-x64"))
	require.NoError(t, ValidateCodexIdentityOption(OptionKeyCodexEnforceClientIdentity, "false"))
	require.Error(t, ValidateCodexIdentityOption(OptionKeyCodexClientVersion, "0.143.9"))
	require.Error(t, ValidateCodexIdentityOption(OptionKeyCodexClientUserAgent, "Mozilla/5.0"))
	require.Error(t, ValidateCodexIdentityOption(OptionKeyCodexAutoSyncClientVersion, "sometimes"))
	require.True(t, IsCodexIdentityReadOnlyOptionKey(OptionKeyCodexSyncedClientVersion))
	require.False(t, IsCodexIdentityReadOnlyOptionKey(OptionKeyCodexClientVersion))
}

func TestResolveCodexClientIdentityPrecedenceAndUserAgent(t *testing.T) {
	withCodexIdentityOptions(t, map[string]string{
		OptionKeyCodexClientUserAgent:     "codex-cli/0.100.1 linux-arm64",
		OptionKeyCodexClientVersion:       "0.146.0",
		OptionKeyCodexSyncedClientVersion: "0.145.0",
	})

	identity := ResolveCodexClientIdentity()
	require.Equal(t, "0.146.0", identity.Version)
	require.Equal(t, "codex-cli/0.146.0 linux-arm64", identity.UserAgent)
	require.Equal(t, "codex_cli_rs", identity.Originator)

	withCodexIdentityOptions(t, map[string]string{
		OptionKeyCodexClientVersion:       "0.143.9",
		OptionKeyCodexSyncedClientVersion: "0.145.0",
		OptionKeyCodexClientUserAgent:     "Mozilla/5.0 attacker",
	})

	identity = ResolveCodexClientIdentity()
	require.Equal(t, "0.145.0", identity.Version)
	require.Equal(t, "codex-cli/0.145.0", identity.UserAgent)
}

func TestApplyCodexIdentityHonorsKillSwitchAndCredentialVersionRule(t *testing.T) {
	withCodexIdentityOptions(t, map[string]string{
		OptionKeyCodexClientVersion: "0.145.4",
	})
	identity := ResolveCodexClientIdentity()

	inference := http.Header{}
	ApplyCodexInferenceIdentity(inference, identity)
	require.Equal(t, "codex-cli/0.145.4", inference.Get("User-Agent"))
	require.Equal(t, "codex_cli_rs", inference.Get("originator"))
	require.Equal(t, "0.145.4", inference.Get(codexClientVersionHeader))

	credential := http.Header{}
	ApplyCodexCredentialIdentity(credential, identity)
	require.Equal(t, "codex-cli/0.145.4", credential.Get("User-Agent"))
	require.Equal(t, "codex_cli_rs", credential.Get("originator"))
	require.Empty(t, credential.Get(codexClientVersionHeader))

	withCodexIdentityOptions(t, map[string]string{
		OptionKeyCodexClientVersion:         "0.145.4",
		OptionKeyCodexEnforceClientIdentity: "false",
	})

	disabled := http.Header{}
	ApplyCodexInferenceIdentity(disabled, identity)
	require.Empty(t, disabled)
}

func TestCodexServiceEgressUsesCanonicalIdentity(t *testing.T) {
	withCodexIdentityOptions(t, map[string]string{
		OptionKeyCodexClientVersion:   "0.145.6",
		OptionKeyCodexClientUserAgent: "codex-cli/0.100.1 usage-suite",
	})

	requestHeaders := make([]http.Header, 0, 3)
	requestURLs := make([]string, 0, 3)
	client := &http.Client{Transport: codexIdentityRoundTripper(func(req *http.Request) (*http.Response, error) {
		requestHeaders = append(requestHeaders, req.Header.Clone())
		requestURLs = append(requestURLs, req.URL.String())
		body := `{"models":[{"slug":"gpt-5-codex"}]}`
		if strings.Contains(req.URL.Path, "/wham/usage") {
			body = `{"usage":0}`
		} else if strings.Contains(req.URL.Path, "/token") {
			body = `{"access_token":"access","refresh_token":"refresh","expires_in":3600}`
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Request:    req,
		}, nil
	})}

	status, _, err := FetchCodexModels(
		context.Background(),
		client,
		"https://chatgpt.example",
		&CodexOAuthKey{AccessToken: "access", AccountID: "account"},
		"0.143.9",
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status)

	status, _, err = FetchCodexWhamUsage(context.Background(), client, "https://chatgpt.example", "access", "account")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status)

	_, err = refreshCodexOAuthToken(
		context.Background(),
		client,
		"https://auth.example/token",
		codexOAuthClientID,
		"refresh",
	)
	require.NoError(t, err)

	require.Len(t, requestHeaders, 3)
	for _, headers := range requestHeaders {
		require.Equal(t, "codex-cli/0.145.6 usage-suite", headers.Get("User-Agent"))
		require.Equal(t, "codex_cli_rs", headers.Get("originator"))
	}
	require.Equal(t, "0.145.6", requestHeaders[0].Get(codexClientVersionHeader))
	require.Equal(t, "0.145.6", requestHeaders[1].Get(codexClientVersionHeader))
	require.Empty(t, requestHeaders[2].Get(codexClientVersionHeader))
	require.Contains(t, requestURLs[0], "client_version=0.145.6")
}

type codexIdentityRoundTripper func(*http.Request) (*http.Response, error)

func (fn codexIdentityRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}
