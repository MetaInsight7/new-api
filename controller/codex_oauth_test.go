package controller

import (
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel/codex"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type codexOAuthTestTransport func(*http.Request) (*http.Response, error)

func (transport codexOAuthTestTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return transport(request)
}

func codexOAuthTestRequest(identity service.AuthIdentity, channelID int, body string, handler gin.HandlerFunc) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(response)
	context.Request = httptest.NewRequest(http.MethodPost, "/api/channel/codex/oauth", strings.NewReader(body))
	context.Request.Header.Set("Content-Type", "application/json")
	context.Set("id", identity.UserID)
	context.Set("role", common.RoleRootUser)
	context.Set("session_id", identity.SessionID)
	context.Set("auth_version", identity.UserAuthVersion)
	context.Set("session_version", identity.SessionVersion)
	if channelID > 0 {
		context.Params = gin.Params{{Key: "id", Value: strconv.Itoa(channelID)}}
	}
	handler(context)
	return response
}

func setupCodexOAuthTest(t *testing.T) service.AuthIdentity {
	t.Helper()
	user, identity := setupSecurityEnrollmentTest(t)
	require.NoError(t, model.DB.Model(user).Update("role", common.RoleRootUser).Error)
	for range 2 {
		require.NoError(t, model.DB.AutoMigrate(&model.Channel{}, &model.Ability{}))
	}
	return identity
}

func startCodexOAuthTestFlow(t *testing.T, identity service.AuthIdentity, channelID int) string {
	t.Helper()
	handler := StartCodexOAuth
	if channelID > 0 {
		handler = StartCodexOAuthForChannel
	}
	response := codexOAuthTestRequest(identity, channelID, "{}", handler)
	var result struct {
		Success bool `json:"success"`
		Data    struct {
			AuthorizeURL string `json:"authorize_url"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &result))
	require.True(t, result.Success, response.Body.String())
	authorizeURL, err := url.Parse(result.Data.AuthorizeURL)
	require.NoError(t, err)
	require.Equal(t, "https://auth.openai.com/oauth/authorize", authorizeURL.Scheme+"://"+authorizeURL.Host+authorizeURL.Path)
	require.Equal(t, "S256", authorizeURL.Query().Get("code_challenge_method"))
	require.Equal(t, "http://localhost:1455/auth/callback", authorizeURL.Query().Get("redirect_uri"))
	require.NotEmpty(t, authorizeURL.Query().Get("code_challenge"))
	return authorizeURL.Query().Get("state")
}

func completeCodexOAuthTestFlow(t *testing.T, identity service.AuthIdentity, channelID int, state string) *httptest.ResponseRecorder {
	t.Helper()
	body, err := common.Marshal(map[string]string{"input": "http://localhost:1455/auth/callback?code=authorization-code&state=" + url.QueryEscape(state)})
	require.NoError(t, err)
	handler := CompleteCodexOAuth
	if channelID > 0 {
		handler = CompleteCodexOAuthForChannel
	}
	return codexOAuthTestRequest(identity, channelID, string(body), handler)
}

func stubCodexOAuthExchange(t *testing.T, status int) *int {
	t.Helper()
	client, err := service.GetHttpClientWithProxy("")
	require.NoError(t, err)
	previousTransport := client.Transport
	t.Cleanup(func() { client.Transport = previousTransport })
	exchanges := new(int)
	client.Transport = codexOAuthTestTransport(func(request *http.Request) (*http.Response, error) {
		(*exchanges)++
		require.Equal(t, "https://auth.openai.com/oauth/token", request.URL.String())
		require.NoError(t, request.ParseForm())
		require.Equal(t, "authorization_code", request.Form.Get("grant_type"))
		require.Equal(t, "authorization-code", request.Form.Get("code"))
		require.NotEmpty(t, request.Form.Get("code_verifier"))
		require.Equal(t, "http://localhost:1455/auth/callback", request.Form.Get("redirect_uri"))
		claims, marshalErr := common.Marshal(map[string]any{"https://api.openai.com/auth": map[string]string{"chatgpt_account_id": "test-account"}, "email": "codex@example.com"})
		require.NoError(t, marshalErr)
		accessToken := "header." + base64.RawURLEncoding.EncodeToString(claims) + ".signature"
		payload, marshalErr := common.Marshal(map[string]any{"access_token": accessToken, "refresh_token": "test-refresh", "expires_in": 3600})
		require.NoError(t, marshalErr)
		return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(string(payload)))}, nil
	})
	return exchanges
}

func TestCodexOAuthNewCredentialIsBoundAndSingleUse(t *testing.T) {
	identity := setupCodexOAuthTest(t)
	exchanges := stubCodexOAuthExchange(t, http.StatusOK)
	state := startCodexOAuthTestFlow(t, identity, 0)
	require.NotEmpty(t, state)
	response := completeCodexOAuthTestFlow(t, identity, 0, state)
	var result struct {
		Success bool `json:"success"`
		Data    struct {
			Key string `json:"key"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &result))
	require.True(t, result.Success, response.Body.String())
	credential, err := codex.ParseOAuthKey(result.Data.Key)
	require.NoError(t, err)
	require.Equal(t, "test-account", credential.AccountID)
	require.Equal(t, "codex@example.com", credential.Email)
	require.Equal(t, "test-refresh", credential.RefreshToken)
	require.NotEmpty(t, credential.Expired)
	replay := completeCodexOAuthTestFlow(t, identity, 0, state)
	require.Contains(t, replay.Body.String(), `"success":false`)
	require.Equal(t, 1, *exchanges)
}

func TestCodexOAuthRejectsExpiredCrossSessionAndRevokedFlows(t *testing.T) {
	for _, scenario := range []string{"expired", "session", "revoked", "version", "permission"} {
		t.Run(scenario, func(t *testing.T) {
			identity := setupCodexOAuthTest(t)
			exchanges := stubCodexOAuthExchange(t, http.StatusOK)
			state := startCodexOAuthTestFlow(t, identity, 0)
			switch scenario {
			case "expired":
				require.NoError(t, model.DB.Model(&model.AuthFlow{}).Where("purpose = ?", codexChannelOAuthPurpose).Update("expires_at", time.Now().Add(-time.Minute)).Error)
			case "session":
				identity.SessionID = "another-session"
			case "revoked":
				require.NoError(t, model.DB.Model(&model.UserSession{}).Where("sid = ?", identity.SessionID).Update("status", model.UserSessionStatusRevoked).Error)
			case "version":
				require.NoError(t, model.DB.Model(&model.UserSession{}).Where("sid = ?", identity.SessionID).Update("version", identity.SessionVersion+1).Error)
			case "permission":
				require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", identity.UserID).Update("role", common.RoleCommonUser).Error)
			}
			response := completeCodexOAuthTestFlow(t, identity, 0, state)
			require.Contains(t, response.Body.String(), `"success":false`)
			require.Equal(t, 0, *exchanges)
		})
	}
}

func TestCodexOAuthDoesNotOverwriteConcurrentCredentialChange(t *testing.T) {
	identity := setupCodexOAuthTest(t)
	channel := &model.Channel{Type: constant.ChannelTypeCodex, Key: `{"access_token":"old","account_id":"old-account"}`, Name: "codex", Status: common.ChannelStatusEnabled, Models: "gpt-5-codex", Group: "default"}
	require.NoError(t, model.DB.Create(channel).Error)
	stubCodexOAuthExchange(t, http.StatusOK)
	state := startCodexOAuthTestFlow(t, identity, channel.Id)
	newKey := `{"access_token":"changed","account_id":"changed-account"}`
	require.NoError(t, model.DB.Model(channel).Update("key", newKey).Error)
	response := completeCodexOAuthTestFlow(t, identity, channel.Id, state)
	require.Contains(t, response.Body.String(), `"success":false`)
	require.NotContains(t, response.Body.String(), "test-refresh")
	updated, err := model.GetChannelById(channel.Id, true)
	require.NoError(t, err)
	require.Equal(t, newKey, updated.Key)
}

func TestCodexOAuthExistingChannelRejectsWrongTargetAndUpdatesCredential(t *testing.T) {
	identity := setupCodexOAuthTest(t)
	channel := &model.Channel{Type: constant.ChannelTypeCodex, Key: `{"access_token":"old","account_id":"old-account"}`, Name: "codex", Status: common.ChannelStatusEnabled, Models: "gpt-5-codex", Group: "default"}
	require.NoError(t, model.DB.Create(channel).Error)
	exchanges := stubCodexOAuthExchange(t, http.StatusOK)
	state := startCodexOAuthTestFlow(t, identity, channel.Id)
	wrongTarget := completeCodexOAuthTestFlow(t, identity, 0, state)
	require.Contains(t, wrongTarget.Body.String(), `"success":false`)
	require.Equal(t, 0, *exchanges)
	response := completeCodexOAuthTestFlow(t, identity, channel.Id, state)
	require.Contains(t, response.Body.String(), `"success":true`)
	require.NotContains(t, response.Body.String(), "test-refresh")
	updated, err := model.GetChannelById(channel.Id, true)
	require.NoError(t, err)
	credential, err := codex.ParseOAuthKey(updated.Key)
	require.NoError(t, err)
	require.Equal(t, "test-account", credential.AccountID)
}

func TestCodexOAuthFailedExchangeCannotBeReplayed(t *testing.T) {
	identity := setupCodexOAuthTest(t)
	exchanges := stubCodexOAuthExchange(t, http.StatusUnauthorized)
	state := startCodexOAuthTestFlow(t, identity, 0)
	response := completeCodexOAuthTestFlow(t, identity, 0, state)
	require.Contains(t, response.Body.String(), `"success":false`)
	require.NotContains(t, response.Body.String(), "test-refresh")
	response = completeCodexOAuthTestFlow(t, identity, 0, state)
	require.Contains(t, response.Body.String(), `"success":false`)
	require.Equal(t, 1, *exchanges)
}

func TestCodexOAuthAuthorizationInput(t *testing.T) {
	for _, input := range []string{"http://localhost:1455/auth/callback?code=code&state=state", "code=code&state=state", "code#state"} {
		code, state, err := parseCodexAuthorizationInput(input)
		require.NoError(t, err)
		require.Equal(t, "code", code)
		require.Equal(t, "state", state)
	}
	for _, input := range []string{"", "code", "https://evil.example/auth/callback?code=code&state=state", "code=code&state=one&state=two"} {
		_, _, err := parseCodexAuthorizationInput(input)
		require.Error(t, err)
	}
}

func TestCodexOAuthCredentialRefreshPreservesNewerCredentials(t *testing.T) {
	for _, scenario := range []string{"expected_key_changed", "changed_during_exchange"} {
		t.Run(scenario, func(t *testing.T) {
			setupCodexOAuthTest(t)
			oldKey := `{"access_token":"old","refresh_token":"old-refresh","account_id":"old-account"}`
			newKey := `{"access_token":"other","refresh_token":"other-refresh","account_id":"other-account"}`
			channel := &model.Channel{Type: constant.ChannelTypeCodex, Key: oldKey, Name: "refresh", Status: common.ChannelStatusEnabled}
			require.NoError(t, model.DB.Create(channel).Error)
			client, err := service.GetHttpClientWithProxy("")
			require.NoError(t, err)
			previousTransport := client.Transport
			t.Cleanup(func() { client.Transport = previousTransport })
			exchanges := 0
			client.Transport = codexOAuthTestTransport(func(request *http.Request) (*http.Response, error) {
				exchanges++
				require.NoError(t, model.DB.Model(channel).Update("key", newKey).Error)
				return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"access_token":"refreshed","refresh_token":"rotated","expires_in":3600}`))}, nil
			})
			if scenario == "expected_key_changed" {
				require.NoError(t, model.DB.Model(channel).Update("key", newKey).Error)
			}
			credential, _, err := service.RefreshCodexChannelCredential(t.Context(), channel.Id, service.CodexCredentialRefreshOptions{ExpectedKey: oldKey})
			require.NoError(t, err)
			require.Equal(t, "other-account", credential.AccountID)
			updated, err := model.GetChannelById(channel.Id, true)
			require.NoError(t, err)
			require.Equal(t, newKey, updated.Key)
			expectedExchanges := 1
			if scenario == "expected_key_changed" {
				expectedExchanges = 0
			}
			require.Equal(t, expectedExchanges, exchanges)
		})
	}
}
