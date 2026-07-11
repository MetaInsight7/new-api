package types

import (
	"errors"
	"testing"
)

func TestErrorOriginDefault(t *testing.T) {
	e := NewError(errors.New("test"), ErrorCodeInvalidRequest)
	if !e.IsUser() {
		t.Fatalf("default origin should be user, got %d", e.GetOrigin())
	}
}

func TestErrorOriginOption(t *testing.T) {
	e := NewError(errors.New("upstream fail"), ErrorCodeDoRequestFailed, ErrOptionWithOrigin(ErrorOriginUpstream))
	if !e.IsUpstream() {
		t.Fatalf("expected upstream, got %d", e.GetOrigin())
	}
}

func TestSetOrigin(t *testing.T) {
	e := InitOpenAIError(ErrorCodeBadResponseStatusCode, 502)
	e.SetOrigin(ErrorOriginUpstream)
	if !e.IsUpstream() {
		t.Fatalf("expected upstream after SetOrigin")
	}
}

func TestReplaceMessageOpenAI(t *testing.T) {
	oai := OpenAIError{Message: "original upstream error with request_id: req_abc123", Type: "error", Code: "rate_limit"}
	e := WithOpenAIError(oai, 429)
	e.SetOrigin(ErrorOriginUpstream)

	e.ReplaceMessage("请求频率过高，请稍后再试")

	if e.Error() != "请求频率过高，请稍后再试" {
		t.Fatalf("Err not replaced: %s", e.Error())
	}
	result := e.ToOpenAIError()
	if result.Message != "请求频率过高，请稍后再试" {
		t.Fatalf("OpenAI message not replaced: %s", result.Message)
	}
}

func TestReplaceMessageClaude(t *testing.T) {
	claude := ClaudeError{Message: "overloaded_error from upstream", Type: "overloaded_error"}
	e := WithClaudeError(claude, 529)
	e.SetOrigin(ErrorOriginUpstream)

	e.ReplaceMessage("上游服务异常，请稍后再试")

	result := e.ToClaudeError()
	if result.Message != "上游服务异常，请稍后再试" {
		t.Fatalf("Claude message not replaced: %s", result.Message)
	}
}

func TestUserErrorNotReplaced(t *testing.T) {
	e := NewError(errors.New("invalid model parameter"), ErrorCodeInvalidRequest)
	// default origin = user, ReplaceMessage should NOT be called by sanitize logic
	// (this tests the type system, not the sanitize function)
	if !e.IsUser() {
		t.Fatalf("should be user origin")
	}
	if e.IsUpstream() || e.IsPlatform() {
		t.Fatalf("should not be upstream or platform")
	}
}
