package service

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResponsesEncryptedReasoningRecoveryPreparesSanitizedRetry(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	raw := []byte(`{
		"model":"gpt-5.6",
		"future_top_level":{"retained":true},
		"previous_response_id":"resp_previous",
		"conversation":{"id":"conv_1"},
		"output":[{"type":"message","id":"msg_output","content":[{"type":"output_text","text":"unchanged output"}]}],
		"input":[
			{"type":"message","role":"user","content":"keep me"},
			{"type":"reasoning","id":"rs_keep","encrypted_content":"cipher-one","summary":[{"type":"summary_text","text":"readable"}],"future_item":"retained"},
			{"type":"reasoning","id":"rs_content","encrypted_content":"cipher-two","content":[{"type":"reasoning_text","text":"readable content"}]},
			{"type":"reasoning","id":"rs_without_cipher","summary":[{"type":"summary_text","text":"also retained"}]},
			{"type":"reasoning","id":"rs_drop","encrypted_content":"cipher-three"}
		]
	}`)
	storage, err := common.CreateBodyStorage(raw)
	require.NoError(t, err)
	t.Cleanup(func() { common.CleanupBodyStorage(ctx) })
	ctx.Set(common.KeyBodyStorage, storage)
	ctx.Request = httptest.NewRequest("POST", "/v1/responses", io.NopCloser(storage))

	request := &dto.OpenAIResponsesRequest{}
	require.NoError(t, common.Unmarshal(raw, request))
	upstreamErr := types.WithOpenAIError(types.OpenAIError{
		Type:    "invalid_request_error",
		Code:    "invalid_encrypted_content",
		Message: "The encrypted content could not be verified.",
	}, 400)

	recovery := NewResponsesEncryptedReasoningRecovery(true)
	applied, err := recovery.Prepare(ctx, request, upstreamErr)
	require.NoError(t, err)
	require.True(t, applied)

	var input []map[string]any
	require.NoError(t, common.Unmarshal(request.Input, &input))
	require.Len(t, input, 4)
	assert.Equal(t, "message", input[0]["type"])
	assert.Equal(t, "reasoning", input[1]["type"])
	assert.NotContains(t, input[1], "id")
	assert.NotContains(t, input[1], "encrypted_content")
	assert.Equal(t, "retained", input[1]["future_item"])
	assert.Equal(t, "reasoning", input[2]["type"])
	assert.NotContains(t, input[2], "id")
	assert.NotContains(t, input[2], "encrypted_content")
	assert.Contains(t, input[2], "content")
	assert.Equal(t, "reasoning", input[3]["type"])
	assert.NotContains(t, input[3], "id")
	assert.Contains(t, input[3], "summary")

	rewrittenStorage, err := common.GetBodyStorage(ctx)
	require.NoError(t, err)
	rewritten, err := rewrittenStorage.Bytes()
	require.NoError(t, err)
	var rewrittenRequest struct {
		FutureTopLevel   map[string]bool  `json:"future_top_level"`
		PreviousResponse string           `json:"previous_response_id"`
		Conversation     json.RawMessage  `json:"conversation"`
		Output           json.RawMessage  `json:"output"`
		Input            []map[string]any `json:"input"`
	}
	require.NoError(t, common.Unmarshal(rewritten, &rewrittenRequest))
	assert.True(t, rewrittenRequest.FutureTopLevel["retained"])
	assert.Equal(t, "resp_previous", rewrittenRequest.PreviousResponse)
	assert.JSONEq(t, `{"id":"conv_1"}`, string(rewrittenRequest.Conversation))
	assert.JSONEq(t, `[{"type":"message","id":"msg_output","content":[{"type":"output_text","text":"unchanged output"}]}]`, string(rewrittenRequest.Output))
	assert.Len(t, rewrittenRequest.Input, 4)
}

func TestSanitizeResponsesEncryptedReasoningRequiresNonEmptyEncryptedState(t *testing.T) {
	for _, test := range []struct {
		name string
		raw  string
	}{
		{name: "no input", raw: `{"model":"gpt-5.6","output":[]}`},
		{name: "string input", raw: `{"model":"gpt-5.6","input":"hello"}`},
		{name: "ordinary messages", raw: `{"model":"gpt-5.6","input":[{"type":"message","role":"user","content":"hello"}]}`},
		{name: "reasoning without encrypted content", raw: `{"model":"gpt-5.6","input":[{"type":"reasoning","id":"rs_1","summary":[{"type":"summary_text","text":"readable"}]}]}`},
		{name: "empty encrypted content", raw: `{"model":"gpt-5.6","input":[{"type":"reasoning","id":"rs_1","encrypted_content":"","summary":[{"type":"summary_text","text":"readable"}]}]}`},
		{name: "null encrypted content", raw: `{"model":"gpt-5.6","input":[{"type":"reasoning","id":"rs_1","encrypted_content":null,"summary":[{"type":"summary_text","text":"readable"}]}]}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			rewritten, _, _, hasEncryptedReasoning, err := sanitizeResponsesEncryptedReasoningBody([]byte(test.raw))
			require.NoError(t, err)
			assert.False(t, hasEncryptedReasoning)
			assert.JSONEq(t, test.raw, string(rewritten))
		})
	}
}

func TestResponsesEncryptedReasoningRecoveryRecognizesOnlySupportedErrors(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		errorType  string
		code       any
		message    string
		want       bool
	}{
		{name: "OpenAI code", statusCode: 400, errorType: "invalid_request_error", code: "invalid_encrypted_content", message: "rejected", want: true},
		{name: "proxy normalized code", statusCode: 400, errorType: "invalid_request_error", code: "thinking_signature_invalid", message: "rejected", want: true},
		{name: "verification message", statusCode: 400, errorType: "invalid_request_error", message: "The encrypted content could not be verified", want: true},
		{name: "decryption message", statusCode: 422, errorType: "invalid_request_error", message: "Encrypted content could not be decrypted or parsed", want: true},
		{name: "thinking block signature", statusCode: 400, errorType: "invalid_request_error", message: "Invalid signature in thinking block", want: true},
		{name: "reasoning block signature case insensitive", statusCode: 400, errorType: "invalid_request_error", message: "INVALID SIGNATURE IN REASONING BLOCK", want: true},
		{name: "unpaired encrypted wording", statusCode: 400, errorType: "invalid_request_error", message: "encrypted content is not available"},
		{name: "unrelated signature block", statusCode: 400, errorType: "invalid_request_error", message: "invalid signature in tool block"},
		{name: "server error message", statusCode: 500, errorType: "server_error", message: "encrypted content could not be verified"},
		{name: "wrong four hundred type", statusCode: 400, errorType: "authentication_error", message: "encrypted content could not be verified"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, request := newEncryptedReasoningRecoveryRequest(t)
			upstreamErr := types.WithOpenAIError(types.OpenAIError{
				Type:    test.errorType,
				Code:    test.code,
				Message: test.message,
			}, test.statusCode)
			applied, err := NewResponsesEncryptedReasoningRecovery(true).Prepare(ctx, request, upstreamErr)
			require.NoError(t, err)
			assert.Equal(t, test.want, applied)
		})
	}
}

func TestResponsesEncryptedReasoningRecoveryRunsOncePerIncomingRequest(t *testing.T) {
	ctx, request := newEncryptedReasoningRecoveryRequest(t)
	upstreamErr := types.WithOpenAIError(types.OpenAIError{
		Type:    "invalid_request_error",
		Code:    "invalid_encrypted_content",
		Message: "rejected",
	}, http.StatusBadRequest)
	recovery := NewResponsesEncryptedReasoningRecovery(true)

	first, err := recovery.Prepare(ctx, request, upstreamErr)
	require.NoError(t, err)
	second, err := recovery.Prepare(ctx, request, upstreamErr)
	require.NoError(t, err)

	assert.True(t, first)
	assert.False(t, second)
}

func newEncryptedReasoningRecoveryRequest(t *testing.T) (*gin.Context, *dto.OpenAIResponsesRequest) {
	t.Helper()
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	raw := []byte(`{"model":"gpt-5.6","input":[{"type":"reasoning","id":"rs_1","encrypted_content":"cipher","summary":[{"type":"summary_text","text":"readable"}]}]}`)
	storage, err := common.CreateBodyStorage(raw)
	require.NoError(t, err)
	ctx.Set(common.KeyBodyStorage, storage)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", io.NopCloser(storage))
	t.Cleanup(func() { common.CleanupBodyStorage(ctx) })
	request := &dto.OpenAIResponsesRequest{}
	require.NoError(t, common.Unmarshal(raw, request))
	return ctx, request
}
