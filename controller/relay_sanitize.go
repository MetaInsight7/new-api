package controller

import (
	"net/http"
	"regexp"

	taskdto "github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
)

var userFacingErrorCodes = map[types.ErrorCode]struct{}{
	types.ErrorCodeInvalidRequest:             {},
	types.ErrorCodeSensitiveWordsDetected:     {},
	types.ErrorCodeViolationFeeGrokCSAM:       {},
	types.ErrorCodeReadRequestBodyFailed:      {},
	types.ErrorCodeInsufficientUserQuota:      {},
	types.ErrorCodePreConsumeTokenQuotaFailed: {},
	types.ErrorCodeAccessDenied:               {},
	types.ErrorCodeModelNotFound:              {},
	types.ErrorCodeBadRequestBody:             {},
}

var upstreamUserPassthroughCodes = map[int]struct{}{
	400: {},
	413: {},
	422: {},
}

var friendlyMessages = map[int]string{
	429: "请求频率过高，请稍后再试",
	401: "服务暂时不可用，请稍后再试",
	403: "服务暂时不可用，请稍后再试",
	408: "上游服务响应超时，请重试",
	500: "上游服务异常，请稍后再试",
	502: "上游服务异常，请稍后再试",
	503: "上游服务异常，请稍后再试",
	504: "上游服务响应超时，请重试",
}

var upstreamRequestIDPattern = regexp.MustCompile(
	`\s*\(request id: [^)]+\)` +
		`|,?\s*RequestID:\s*[a-zA-Z0-9_-]+` +
		`|,?\s*request[_-]?id[=:]\s*[a-zA-Z0-9_-]+` +
		`|,?\s*x-amzn-requestid[=:]\s*[a-zA-Z0-9_-]+` +
		`|,?\s*x-request-id[=:]\s*[a-zA-Z0-9_-]+`,
)

func sanitizeErrorForUser(err *types.NewAPIError) {
	if err == nil {
		return
	}
	if _, ok := userFacingErrorCodes[err.GetErrorCode()]; ok {
		err.SetMessage(upstreamRequestIDPattern.ReplaceAllString(err.Error(), ""))
		return
	}
	if _, ok := upstreamUserPassthroughCodes[err.StatusCode]; ok {
		err.SetMessage(upstreamRequestIDPattern.ReplaceAllString(err.Error(), ""))
		return
	}
	message, ok := friendlyMessages[err.StatusCode]
	if err.StatusCode == http.StatusInternalServerError && err.GetErrorType() == types.ErrorTypeNewAPIError {
		message = "请求处理失败，请稍后再试"
		ok = true
	}
	if !ok && err.StatusCode >= 500 {
		message = friendlyMessages[500]
	}
	if message == "" {
		message = "请求处理失败，请稍后再试"
	}
	err.SetMessage(message)
}

func sanitizeTaskErrorForUser(taskErr *taskdto.TaskError) {
	if taskErr == nil {
		return
	}
	if taskErr.Code == "task_timeout" {
		taskErr.Message = upstreamRequestIDPattern.ReplaceAllString(taskErr.Message, "")
		return
	}
	statusCode := taskErr.StatusCode
	if message, ok := friendlyMessages[statusCode]; ok {
		taskErr.Message = message
		return
	}
	if statusCode >= 500 {
		taskErr.Message = friendlyMessages[500]
		return
	}
	taskErr.Message = upstreamRequestIDPattern.ReplaceAllString(taskErr.Message, "")
}
