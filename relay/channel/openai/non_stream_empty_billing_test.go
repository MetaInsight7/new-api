package openai

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenaiHandlerMarksLocallyEstimatedEmptyNonStreamResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	info := &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatOpenAI,
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "compatible-model"},
	}
	info.SetEstimatePromptTokens(37)

	responseBody := `{"choices":null,"usage":{"prompt_tokens":0,"completion_tokens":0,"total_tokens":0}}`
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(responseBody)),
	}

	usage, apiErr := OpenaiHandler(ctx, info, resp)
	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	assert.Equal(t, 37, usage.PromptTokens)
	assert.Zero(t, usage.CompletionTokens)
	assert.True(t, common.GetContextKeyBool(ctx, constant.ContextKeyLocalCountTokens))
	assert.True(t, common.GetContextKeyBool(ctx, constant.ContextKeyNonStreamEmptyResponse))
	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"choices":null`)
	assert.NotContains(t, recorder.Body.String(), `"error"`)
}

func TestOpenaiHandlerKeepsStructuredNonStreamOutputBillable(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	info := &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatOpenAI,
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "compatible-model"},
	}
	info.SetEstimatePromptTokens(37)

	responseBody := `{"choices":[{"index":0,"message":{"role":"assistant","content":null,"tool_calls":[{"id":"call_1","type":"function","function":{"name":"lookup","arguments":"{}"}}]},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":0,"completion_tokens":0,"total_tokens":0}}`
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(responseBody)),
	}

	usage, apiErr := OpenaiHandler(ctx, info, resp)
	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	assert.Equal(t, 37, usage.PromptTokens)
	assert.Zero(t, usage.CompletionTokens)
	assert.True(t, common.GetContextKeyBool(ctx, constant.ContextKeyLocalCountTokens))
	assert.False(t, common.GetContextKeyBool(ctx, constant.ContextKeyNonStreamEmptyResponse))
}

func TestOpenAIProtocolConversionsMarkLocallyEstimatedEmptyNonStreamResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("chat to responses", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

		info := &relaycommon.RelayInfo{
			RelayFormat: types.RelayFormatOpenAIResponses,
			ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "compatible-model"},
		}
		info.SetEstimatePromptTokens(37)

		resp := &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"choices":null,"usage":{"prompt_tokens":0,"completion_tokens":0,"total_tokens":0}}`)),
		}

		usage, apiErr := OaiChatToResponsesHandler(ctx, info, resp)
		require.Nil(t, apiErr)
		require.NotNil(t, usage)
		assert.Equal(t, 37, usage.PromptTokens)
		assert.True(t, common.GetContextKeyBool(ctx, constant.ContextKeyLocalCountTokens))
		assert.True(t, common.GetContextKeyBool(ctx, constant.ContextKeyNonStreamEmptyResponse))
		assert.Equal(t, http.StatusOK, recorder.Code)
	})

	t.Run("responses to chat", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

		info := &relaycommon.RelayInfo{
			RelayFormat: types.RelayFormatOpenAI,
			ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "compatible-model"},
		}
		info.SetEstimatePromptTokens(37)

		resp := &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"id":"resp_1","object":"response","model":"compatible-model","status":"completed","output":[]}`)),
		}

		usage, apiErr := OaiResponsesToChatHandler(ctx, info, resp)
		require.Nil(t, apiErr)
		require.NotNil(t, usage)
		assert.Equal(t, 37, usage.PromptTokens)
		assert.True(t, common.GetContextKeyBool(ctx, constant.ContextKeyLocalCountTokens))
		assert.True(t, common.GetContextKeyBool(ctx, constant.ContextKeyNonStreamEmptyResponse))
		assert.Equal(t, http.StatusOK, recorder.Code)
	})
}
