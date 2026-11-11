package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSanitizeErrorForUserPreservesActionableRequestErrors(t *testing.T) {
	err := types.NewError(nil, types.ErrorCodeInvalidRequest)
	err.SetMessage("messages cannot be empty (request id: req_upstream_123)")

	sanitizeErrorForUser(err, false)

	assert.Equal(t, "messages cannot be empty", err.Error())
}

func TestSanitizeErrorForUserHidesUpstreamDetails(t *testing.T) {
	err := types.WithOpenAIError(types.OpenAIError{
		Message: "dial tcp api.example.com:443: connection refused",
		Type:    "server_error",
		Code:    "internal_error",
	}, http.StatusBadGateway)

	sanitizeErrorForUser(err, false)

	assert.Equal(t, "上游服务异常，请稍后再试", err.Error())
	assert.NotContains(t, err.Error(), "api.example.com")
}

func TestSanitizeErrorForUserHidesInternalChannelDetails(t *testing.T) {
	err := types.NewError(fmt.Errorf("group private-group has no channel for secret-model"), types.ErrorCodeGetChannelFailed)

	sanitizeErrorForUser(err, false)

	assert.Equal(t, "请求处理失败，请稍后再试", err.Error())
}

func TestSanitizeErrorForUserHidesUpstreamUserBannedMessage(t *testing.T) {
	err := types.WithOpenAIError(types.OpenAIError{
		Message: "upstream account disabled: internal tenant secret",
		Type:    "permission_error",
		Code:    types.ErrorCodeUserBanned,
	}, http.StatusForbidden)

	sanitizeErrorForUser(err, false)

	assert.Equal(t, "服务暂时不可用，请稍后再试", err.Error())
	assert.NotContains(t, err.Error(), "tenant secret")
}

func TestClaudeSensitiveWordBillingFallbackUsesNativeErrorType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	require.NoError(t, i18n.Init())
	tests := []struct {
		name       string
		autoBanned bool
		statusCode int
		errorType  string
		errorCode  types.ErrorCode
		message    string
	}{
		{name: "blocked", statusCode: http.StatusBadRequest, errorType: "invalid_request_error", errorCode: types.ErrorCodeSensitiveWordsDetected, message: "local policy message"},
		{name: "auto banned", autoBanned: true, statusCode: http.StatusForbidden, errorType: "permission_error", errorCode: types.ErrorCodeUserBanned, message: i18n.Translate(i18n.LangEn, i18n.MsgSensitiveWordAutoBanned)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			context, _ := gin.CreateTestContext(httptest.NewRecorder())
			context.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
			context.Request.Header.Set("Accept-Language", "en")
			apiErr := service.NewSensitiveWordRequestError(context, &model.SensitiveCheckResult{
				Matched: true, Blocked: true, AutoBanned: test.autoBanned,
				Message: "local policy message",
			})
			require.NotNil(t, apiErr)
			require.Equal(t, test.statusCode, apiErr.StatusCode)
			require.Equal(t, test.errorCode, apiErr.GetErrorCode())

			sanitizeErrorForUser(apiErr, true)
			claudeError := toClaudeRelayError(apiErr, true)

			assert.Equal(t, test.errorType, claudeError.Type)
			assert.Equal(t, test.message, claudeError.Message)
		})
	}
}
