package service

import (
	"net/http"

	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
)

type GeminiError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Status  string `json:"status"`
}

func GeminiErrorFromNewAPIError(apiErr *types.NewAPIError) GeminiError {
	openAIError := apiErr.ToOpenAIError()
	status := "UNKNOWN"
	switch apiErr.StatusCode {
	case http.StatusBadRequest:
		status = "INVALID_ARGUMENT"
	case http.StatusUnauthorized:
		status = "UNAUTHENTICATED"
	case http.StatusForbidden:
		status = "PERMISSION_DENIED"
	case http.StatusNotFound:
		status = "NOT_FOUND"
	case http.StatusTooManyRequests:
		status = "RESOURCE_EXHAUSTED"
	case http.StatusServiceUnavailable:
		status = "UNAVAILABLE"
	case http.StatusInternalServerError:
		status = "INTERNAL"
	}
	return GeminiError{Code: apiErr.StatusCode, Message: openAIError.Message, Status: status}
}

// NewSensitiveWordRequestError builds the public error contract shared by
// pre-routing enforcement and the billing fallback.
func NewSensitiveWordRequestError(c *gin.Context, result *model.SensitiveCheckResult) *types.NewAPIError {
	if result == nil || !result.Matched || !result.Blocked {
		return nil
	}
	message := result.Message
	errorType := "invalid_request_error"
	errorCode := types.ErrorCodeSensitiveWordsDetected
	statusCode := http.StatusBadRequest
	if result.UsesDefaultMessage {
		message = i18n.T(c, i18n.MsgSensitiveWordBlocked)
	}
	if result.AutoBanned {
		message = i18n.T(c, i18n.MsgSensitiveWordAutoBanned)
		errorType = "permission_error"
		errorCode = types.ErrorCodeUserBanned
		statusCode = http.StatusForbidden
	}
	return types.WithOpenAIError(types.OpenAIError{
		Message: message,
		Type:    errorType,
		Code:    errorCode,
	}, statusCode, types.ErrOptionWithSkipRetry(), types.ErrOptionWithNoRecordErrorLog())
}
