package openai

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

func OaiResponsesHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	defer service.CloseResponseBodyGracefully(resp)

	// read response body
	var responsesResponse dto.OpenAIResponsesResponse
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}
	err = common.Unmarshal(responseBody, &responsesResponse)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	if oaiError := responsesResponse.GetOpenAIError(); oaiError != nil && oaiError.Type != "" {
		return nil, types.WithOpenAIError(*oaiError, resp.StatusCode)
	}

	info.ObserveResponseModel(responsesResponse.Model)
	responseBody = rewriteSGLangResponsesCreatedAt(info, responseBody, "created_at", responsesResponse.CreatedAt)

	// 写入新的 response body
	service.IOCopyBytesGracefully(c, resp, responseBody)

	// compute usage
	usage := &dto.Usage{}
	service.ApplyResponsesUsage(usage, responsesResponse.Usage)
	// Count actual tool invocations from Output (not tool declarations).
	for _, output := range responsesResponse.Output {
		switch output.Type {
		case dto.BuildInCallWebSearchCall:
			info.CountBillableToolCall(dto.BuildInCallWebSearchCall, "")
		case dto.BuildInCallFileSearchCall:
			info.CountBillableToolCall(dto.BuildInCallFileSearchCall, "")
		case dto.BuildInCallFunctionCall:
			info.CountBillableToolCall(dto.BuildInCallFunctionCall, output.Name)
		}
	}

	imageCounter := &relaycommon.ImageGenerationCallCounter{}
	if !relaycommon.IsNonBillableResponsesStatus(responsesResponse.Status) {
		for i := range responsesResponse.Output {
			idx := i
			imageCounter.Observe(&responsesResponse.Output[i], &idx)
		}
	}
	imageCounter.Commit(info)

	return usage, nil
}

func OaiResponsesStreamHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	if resp == nil || resp.Body == nil {
		logger.LogError(c, "invalid response or response body")
		return nil, types.NewError(fmt.Errorf("invalid response"), types.ErrorCodeBadResponse)
	}

	defer service.CloseResponseBodyGracefully(resp)

	accumulator := service.NewResponsesUsageAccumulator(info)
	var terminalErr *types.NewAPIError
	var responseHeaderSnapshot http.Header
	var eventStreamHeadersValue any
	var hadEventStreamHeaders bool
	if info.GetChannelType() == constant.ChannelTypeCodex {
		responseHeaderSnapshot = c.Writer.Header().Clone()
		eventStreamHeadersValue, hadEventStreamHeaders = c.Get("event_stream_headers_set")
	}

	helper.StreamScannerHandler(c, resp, info, func(data string, sr *helper.StreamResult) {

		// 检查当前数据是否包含 completed 状态和 usage 信息
		var streamResponse dto.ResponsesStreamResponse
		if err := common.UnmarshalJsonStr(data, &streamResponse); err != nil {
			logger.LogError(c, "failed to unmarshal stream response: "+err.Error())
			sr.Error(err)
			return
		}
		if streamResponse.Response != nil {
			data = string(rewriteSGLangResponsesCreatedAt(info, []byte(data), "response.created_at", streamResponse.Response.CreatedAt))
		}
		if isCodexResponsesTerminalFailure(info, streamResponse) {
			responseWritten := c.Writer.Written()
			accumulator.Observe(&streamResponse)
			terminalErr = NewCodexResponsesTerminalError(
				streamResponse.Type,
				streamResponse.Response,
				responseWritten || codexResponsesUsageIsAuthoritative(streamResponse.Response),
			)
			logCodexResponsesTerminalFailure(c, streamResponse.Type, streamResponse.Response)
			if responseWritten {
				sendResponsesStreamData(c, streamResponse, sanitizeCodexTerminalEvent(data, streamResponse.Type))
			}
			sr.Done()
			return
		}
		sendResponsesStreamData(c, streamResponse, data)
		accumulator.Observe(&streamResponse)
	})

	common.SetContextKey(c, constant.ContextKeyResponseStreamStatus, info.StreamStatus)
	info.StreamStatus.RequireTerminal()
	usage := accumulator.Finish()
	if terminalErr != nil {
		if !types.IsSkipRetryError(terminalErr) {
			restoreCodexResponsesStreamAttemptState(c, info, responseHeaderSnapshot, eventStreamHeadersValue, hadEventStreamHeaders)
		}
		return usage, terminalErr
	}
	return usage, nil
}

func codexResponsesUsageIsAuthoritative(response *dto.OpenAIResponsesResponse) bool {
	if response == nil || response.Usage == nil {
		return false
	}
	usage := response.Usage
	return usage.InputTokens > 0 || usage.OutputTokens > 0 || usage.TotalTokens > 0
}

func isCodexResponsesTerminalFailure(info *relaycommon.RelayInfo, event dto.ResponsesStreamResponse) bool {
	if info == nil || info.GetChannelType() != constant.ChannelTypeCodex {
		return false
	}
	switch event.Type {
	case "response.failed", "response.incomplete", "response.error", "error":
		return true
	}
	if event.Response == nil {
		return false
	}
	var status string
	if err := common.Unmarshal(event.Response.Status, &status); err != nil {
		return false
	}
	return status == "failed" || status == "incomplete"
}

func sanitizeCodexTerminalEvent(data, eventType string) string {
	if eventType != "response.failed" && eventType != "response.error" && eventType != "error" {
		return data
	}
	var payload map[string]any
	if err := common.UnmarshalJsonStr(data, &payload); err != nil {
		return data
	}
	payload["message"] = "Codex upstream response failed"
	if response, ok := payload["response"].(map[string]any); ok {
		if _, exists := response["error"]; exists {
			response["error"] = map[string]any{
				"type":    "server_error",
				"message": "Codex upstream response failed",
				"code":    "server_error",
			}
		}
	}
	sanitized, err := common.Marshal(payload)
	if err != nil {
		return data
	}
	return string(sanitized)
}

func restoreCodexResponsesStreamAttemptState(
	c *gin.Context,
	info *relaycommon.RelayInfo,
	headerSnapshot http.Header,
	eventStreamHeadersValue any,
	hadEventStreamHeaders bool,
) {
	header := c.Writer.Header()
	clear(header)
	for key, values := range headerSnapshot {
		header[key] = append([]string(nil), values...)
	}
	if hadEventStreamHeaders {
		c.Set("event_stream_headers_set", eventStreamHeadersValue)
	} else if c.Keys != nil {
		delete(c.Keys, "event_stream_headers_set")
	}
	info.SendResponseCount = 0
	info.ReceivedResponseCount = 0
}

func logCodexResponsesTerminalFailure(c *gin.Context, eventType string, response *dto.OpenAIResponsesResponse) {
	upstreamType := ""
	upstreamCode := ""
	if response != nil {
		if openAIError := response.GetOpenAIError(); openAIError != nil {
			upstreamType = openAIError.Type
			upstreamCode = strings.TrimSpace(common.Interface2String(openAIError.Code))
		}
	}
	logger.LogWarn(c, "codex Responses terminal failure event=%s upstream_type=%s upstream_code=%s", eventType, upstreamType, upstreamCode)
}

func NewCodexResponsesTerminalError(eventType string, response *dto.OpenAIResponsesResponse, skipRetry bool) *types.NewAPIError {
	options := make([]types.NewAPIErrorOptions, 0, 1)
	if skipRetry {
		options = append(options, types.ErrOptionWithSkipRetry())
	}
	var openAIError *types.OpenAIError
	if response != nil {
		openAIError = response.GetOpenAIError()
	}
	statusCode := codexResponsesFailedStatus(openAIError)
	message := "Codex upstream response failed"
	if eventType == "response.incomplete" {
		message = "Codex upstream response incomplete"
	}
	if openAIError == nil {
		return types.NewOpenAIError(errors.New(message), types.ErrorCodeBadResponse, statusCode, options...)
	}
	errorType := strings.TrimSpace(openAIError.Type)
	if errorType == "" {
		errorType = "upstream_error"
	}
	errorCode := strings.TrimSpace(common.Interface2String(openAIError.Code))
	if errorCode == "" {
		errorCode = string(types.ErrorCodeBadResponse)
	}
	return types.WithOpenAIError(types.OpenAIError{
		Type:    errorType,
		Code:    errorCode,
		Message: message,
	}, statusCode, options...)
}

func codexResponsesFailedStatus(openAIError *types.OpenAIError) int {
	if openAIError == nil {
		return http.StatusInternalServerError
	}
	errorType := strings.ToLower(strings.TrimSpace(openAIError.Type))
	errorCode := strings.ToLower(strings.TrimSpace(common.Interface2String(openAIError.Code)))
	if statusCode := codexResponsesFailedIdentifierStatus(errorCode); statusCode != 0 {
		return statusCode
	}
	if statusCode := codexResponsesFailedIdentifierStatus(errorType); statusCode != 0 {
		return statusCode
	}
	return http.StatusInternalServerError
}

func codexResponsesFailedIdentifierStatus(identifier string) int {
	switch identifier {
	case "insufficient_quota", "credit_balance_exhausted", "organization_spend_limit_exceeded", "project_spend_limit_exceeded", "organization_usage_limit_exceeded", "project_usage_limit_exceeded", "rate_limit_error", "rate_limit_exceeded":
		return http.StatusTooManyRequests
	case "permission_error", "permission_denied", "unsupported_country_region_territory":
		return http.StatusForbidden
	case "authentication_error", "invalid_api_key":
		return http.StatusUnauthorized
	case "invalid_request_error", "invalid_request", "context_length_exceeded", "bad_request", "invalid_prompt", "content_policy_violation", "data_residency_mismatch", "bio_policy", "invalid_image", "invalid_image_format", "invalid_base64_image", "invalid_image_url", "image_too_large", "image_too_small", "image_parse_error", "image_content_policy_violation", "invalid_image_mode", "image_file_too_large", "unsupported_image_media_type", "empty_image_file", "failed_to_download_image", "image_file_not_found":
		return http.StatusBadRequest
	case "overloaded_error", "overloaded", "service_unavailable":
		return http.StatusServiceUnavailable
	case "server_error":
		return http.StatusInternalServerError
	default:
		return 0
	}
}

func rewriteSGLangResponsesCreatedAt(info *relaycommon.RelayInfo, payload []byte, path string, createdAt dto.IntValue) []byte {
	if info.GetChannelType() != constant.ChannelTypeSGLang {
		return payload
	}
	if !gjson.GetBytes(payload, path).Exists() {
		return payload
	}
	patched, err := sjson.SetBytes(payload, path, int(createdAt))
	if err != nil {
		return payload
	}
	return patched
}
