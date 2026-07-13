package controller

import (
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

// 上游错误脱敏:按 ErrorOrigin 决定是否替换用户可见的错误消息。
// 用户自己的错误(参数/额度/敏感词)原样返回;
// 上游/平台错误替换为统一友好消息,管理员日志(在此之前已记录)保留原始信息。

// 友好消息(中英双份,按 gin context 语言选;无 context 时用中文)
var upstreamMessages = map[int]struct{ zh, en string }{
	429: {"请求频率过高，请稍后再试", "Rate limited, please try again later"},
	401: {"服务暂时不可用，请稍后再试", "Service temporarily unavailable, please try again later"},
	403: {"服务暂时不可用，请稍后再试", "Service temporarily unavailable, please try again later"},
	408: {"上游服务响应超时，请重试", "Upstream service timed out, please retry"},
	504: {"上游服务响应超时，请重试", "Upstream service timed out, please retry"},
	500: {"上游服务异常，请稍后再试", "Upstream service error, please try again later"},
	502: {"上游服务异常，请稍后再试", "Upstream service error, please try again later"},
	503: {"上游服务异常，请稍后再试", "Upstream service error, please try again later"},
}

var (
	defaultUpstreamMsg = "请求处理失败，请稍后再试"
	defaultPlatformMsg = "服务内部错误，请稍后重试"
)

func sanitizeErrorForUser(c *gin.Context, e *types.NewAPIError) {
	if e == nil {
		return
	}
	switch e.GetOrigin() {
	case types.ErrorOriginUser:
		return
	case types.ErrorOriginUpstream:
		if isUpstreamUserError(e.StatusCode) {
			return
		}
		e.ReplaceMessage(getUpstreamMessage(e.StatusCode))
	case types.ErrorOriginPlatform:
		e.ReplaceMessage(defaultPlatformMsg)
	}
}

func isUpstreamUserError(statusCode int) bool {
	return statusCode == 400 || statusCode == 413 || statusCode == 422
}

func getUpstreamMessage(statusCode int) string {
	if m, ok := upstreamMessages[statusCode]; ok {
		return m.zh
	}
	if statusCode >= 500 {
		return upstreamMessages[500].zh
	}
	return defaultUpstreamMsg
}
