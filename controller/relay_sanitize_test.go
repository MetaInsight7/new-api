package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/types"
)

func TestSanitize_UserError_PassThrough(t *testing.T) {
	e := types.NewError(nil, types.ErrorCodeInvalidRequest)
	e.SetMessage("invalid model parameter: messages cannot be empty")
	sanitizeErrorForUser(nil, e) // nil context → i18n falls back to key
	msg := e.Error()
	if msg != "invalid model parameter: messages cannot be empty" {
		t.Fatalf("user error should pass through unchanged, got: %s", msg)
	}
}

func TestSanitize_UpstreamError_Replaced(t *testing.T) {
	oai := types.OpenAIError{
		Message: "Rate limit exceeded on tokens per min. Limit: 10000, Used: 9500 (request id: req_abc123)",
		Type:    "rate_limit_error",
		Code:    "rate_limit_exceeded",
	}
	e := types.WithOpenAIError(oai, 429)
	e.SetOrigin(types.ErrorOriginUpstream)

	sanitizeErrorForUser(nil, e)

	msg := e.Error()
	if contains(msg, "Rate limit") || contains(msg, "req_abc123") || contains(msg, "10000") {
		t.Fatalf("upstream details should be stripped, got: %s", msg)
	}
	if msg != "请求频率过高，请稍后再试" {
		t.Fatalf("expected friendly message, got: %s", msg)
	}

	result := e.ToOpenAIError()
	if contains(result.Message, "Rate limit") || contains(result.Message, "req_abc123") {
		t.Fatalf("OpenAI message should be replaced, got: %s", result.Message)
	}
}

func TestSanitize_UpstreamError_500(t *testing.T) {
	oai := types.OpenAIError{
		Message: "Internal server error at https://api.openai.com/v1/chat/completions",
		Type:    "server_error",
		Code:    "internal_error",
	}
	e := types.WithOpenAIError(oai, 500)
	e.SetOrigin(types.ErrorOriginUpstream)

	sanitizeErrorForUser(nil, e)

	if contains(e.Error(), "openai.com") {
		t.Fatalf("upstream URL should be stripped, got: %s", e.Error())
	}
}

func TestSanitize_PlatformError_Replaced(t *testing.T) {
	e := types.NewError(nil, types.ErrorCodeGetChannelFailed, types.ErrOptionWithOrigin(types.ErrorOriginPlatform))
	e.SetMessage("获取分组 default 下模型 gpt-4o 的可用渠道不存在")

	sanitizeErrorForUser(nil, e)

	if contains(e.Error(), "default") || contains(e.Error(), "gpt-4o") {
		t.Fatalf("platform details should be stripped, got: %s", e.Error())
	}
}

func TestSanitize_MultiRequestId_Gone(t *testing.T) {
	oai := types.OpenAIError{
		Message: "Error processing request (request id: req_upstream_123)",
		Type:    "error",
		Code:    "server_error",
	}
	e := types.WithOpenAIError(oai, 502)
	e.SetOrigin(types.ErrorOriginUpstream)

	sanitizeErrorForUser(nil, e)

	if contains(e.Error(), "req_upstream_123") {
		t.Fatalf("upstream request_id should be gone, got: %s", e.Error())
	}
}

func TestSanitize_UpstreamUserError_PassThrough(t *testing.T) {
	cases := []struct {
		status int
		msg    string
	}{
		{400, "This model does not support assistant message prefill"},
		{413, "Request body too large"},
		{422, "Unprocessable entity: invalid field type"},
	}
	for _, tc := range cases {
		oai := types.OpenAIError{
			Message: tc.msg,
			Type:    "invalid_request_error",
		}
		e := types.WithOpenAIError(oai, tc.status)
		e.SetOrigin(types.ErrorOriginUpstream)

		sanitizeErrorForUser(nil, e)

		if e.Error() != tc.msg {
			t.Fatalf("upstream %d should pass through, want %q, got %q", tc.status, tc.msg, e.Error())
		}
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
