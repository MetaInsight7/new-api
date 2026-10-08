package controller

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestCodexResetCreditRetainsRequestIDThroughAuthenticationRetry(t *testing.T) {
	identity := setupCodexOAuthTest(t)
	channel := &model.Channel{Type: constant.ChannelTypeCodex, Key: `{"access_token":"old","refresh_token":"refresh","account_id":"account"}`, Name: "reset", Status: common.ChannelStatusEnabled}
	require.NoError(t, model.DB.Create(channel).Error)
	client, err := service.GetHttpClientWithProxy("")
	require.NoError(t, err)
	previousTransport := client.Transport
	t.Cleanup(func() { client.Transport = previousTransport })
	var requestIDs []string
	refreshes := 0
	client.Transport = codexOAuthTestTransport(func(request *http.Request) (*http.Response, error) {
		status, body := http.StatusOK, `{"windows_reset":1}`
		if request.URL.Host == "auth.openai.com" {
			refreshes++
			body = `{"access_token":"new","refresh_token":"rotated","expires_in":3600}`
		} else {
			var payload map[string]string
			require.NoError(t, common.DecodeJson(request.Body, &payload))
			require.NotEmpty(t, payload["redeem_request_id"])
			requestIDs = append(requestIDs, payload["redeem_request_id"])
			if len(requestIDs) == 1 {
				status, body = http.StatusUnauthorized, `{}`
			}
		}
		return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	response := codexOAuthTestRequest(identity, channel.Id, "{}", ResetCodexChannelUsage)
	require.Contains(t, response.Body.String(), `"success":true`)
	require.Len(t, requestIDs, 2)
	require.Equal(t, requestIDs[0], requestIDs[1])
	require.Equal(t, 1, refreshes)
}

func TestCodexResetCreditRetriesUsingCurrentChannelConfiguration(t *testing.T) {
	identity := setupCodexOAuthTest(t)
	originalURL := "https://original.example.com"
	channel := &model.Channel{Type: constant.ChannelTypeCodex, BaseURL: &originalURL, Key: `{"access_token":"old","refresh_token":"refresh","account_id":"original-account"}`, Name: "reset", Status: common.ChannelStatusEnabled}
	require.NoError(t, model.DB.Create(channel).Error)
	client, err := service.GetHttpClientWithProxy("")
	require.NoError(t, err)
	previousTransport := client.Transport
	t.Cleanup(func() { client.Transport = previousTransport })
	var requestIDs []string
	client.Transport = codexOAuthTestTransport(func(request *http.Request) (*http.Response, error) {
		var payload map[string]string
		require.NoError(t, common.DecodeJson(request.Body, &payload))
		requestIDs = append(requestIDs, payload["redeem_request_id"])
		if len(requestIDs) == 1 {
			require.Equal(t, "original.example.com", request.URL.Host)
			require.NoError(t, model.DB.Model(channel).Updates(map[string]any{
				"key":      `{"access_token":"replacement","refresh_token":"rotated","account_id":"replacement-account"}`,
				"base_url": "https://replacement.example.com",
			}).Error)
			return &http.Response{StatusCode: http.StatusUnauthorized, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{}`))}, nil
		}
		require.Equal(t, "replacement.example.com", request.URL.Host)
		require.Equal(t, "Bearer replacement", request.Header.Get("Authorization"))
		require.Equal(t, "replacement-account", request.Header.Get("chatgpt-account-id"))
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"windows_reset":1}`))}, nil
	})
	response := codexOAuthTestRequest(identity, channel.Id, "{}", ResetCodexChannelUsage)
	require.Contains(t, response.Body.String(), `"success":true`)
	require.Len(t, requestIDs, 2)
	require.Equal(t, requestIDs[0], requestIDs[1])
}

func TestCodexResetCreditDoesNotRetryUncertainFailureOrClaimZeroReset(t *testing.T) {
	for _, scenario := range []string{"network", "oversized", "zero", "missing", "positive"} {
		t.Run(scenario, func(t *testing.T) {
			identity := setupCodexOAuthTest(t)
			channel := &model.Channel{Type: constant.ChannelTypeCodex, Key: `{"access_token":"old","refresh_token":"refresh","account_id":"account"}`, Name: "reset", Status: common.ChannelStatusEnabled}
			require.NoError(t, model.DB.Create(channel).Error)
			client, err := service.GetHttpClientWithProxy("")
			require.NoError(t, err)
			previousTransport := client.Transport
			t.Cleanup(func() { client.Transport = previousTransport })
			calls := 0
			client.Transport = codexOAuthTestTransport(func(request *http.Request) (*http.Response, error) {
				calls++
				body := `{"windows_reset":1}`
				switch scenario {
				case "network":
					return nil, errors.New("uncertain failure with sensitive-token")
				case "oversized":
					body = strings.Repeat("x", (1<<20)+1)
				case "zero":
					body = `{"windows_reset":0}`
				case "missing":
					body = `{}`
				}
				return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
			})
			response := codexOAuthTestRequest(identity, channel.Id, "{}", ResetCodexChannelUsage)
			var result gin.H
			require.NoError(t, common.Unmarshal(response.Body.Bytes(), &result))
			require.Equal(t, scenario == "positive", result["success"], response.Body.String())
			require.NotContains(t, response.Body.String(), "sensitive-token")
			require.Equal(t, 1, calls)
		})
	}
}
