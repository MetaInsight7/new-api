package codex

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	relaytypes "github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetRequestURLAlphaSearch(t *testing.T) {
	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:    constant.ChannelTypeCodex,
			ChannelBaseUrl: "https://chatgpt.com",
		},
		RelayMode: relayconstant.RelayModeAlphaSearch,
	}

	url, err := adaptor.GetRequestURL(info)
	require.NoError(t, err)
	assert.Equal(t, "https://chatgpt.com/backend-api/codex/alpha/search", url)
}

func TestGetRequestURLChatCompletionsUsesCodexResponsesEndpoint(t *testing.T) {
	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:    constant.ChannelTypeCodex,
			ChannelBaseUrl: "https://chatgpt.com",
		},
		RelayMode: relayconstant.RelayModeChatCompletions,
	}

	url, err := adaptor.GetRequestURL(info)

	require.NoError(t, err)
	assert.Equal(t, "https://chatgpt.com/backend-api/codex/responses", url)
}

func TestConvertOpenAIRequestBuildsCodexResponsesRequest(t *testing.T) {
	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{ChannelType: constant.ChannelTypeCodex},
		RelayMode:   relayconstant.RelayModeChatCompletions,
	}
	stream := false
	maxTokens := uint(128)
	temperature := 0.7
	topP := 0.8

	converted, err := adaptor.ConvertOpenAIRequest(nil, info, &dto.GeneralOpenAIRequest{
		Model:       "gpt-5.6-sol",
		Stream:      &stream,
		MaxTokens:   &maxTokens,
		Temperature: &temperature,
		TopP:        &topP,
		Messages: []dto.Message{
			{Role: "system", Content: "You are concise."},
			{Role: "user", Content: "Hello"},
		},
	})

	require.NoError(t, err)
	responsesRequest, ok := converted.(dto.OpenAIResponsesRequest)
	require.True(t, ok)
	require.NotNil(t, responsesRequest.Stream)
	assert.True(t, *responsesRequest.Stream)
	assert.Equal(t, json.RawMessage("false"), responsesRequest.Store)
	assert.Nil(t, responsesRequest.MaxOutputTokens)
	assert.Nil(t, responsesRequest.Temperature)
	assert.Nil(t, responsesRequest.TopP)
	assert.Nil(t, responsesRequest.FrequencyPenalty)
	assert.Nil(t, responsesRequest.PresencePenalty)
	assert.Equal(t, json.RawMessage(`"You are concise."`), responsesRequest.Instructions)

	var input []map[string]any
	require.NoError(t, common.Unmarshal(responsesRequest.Input, &input))
	require.Len(t, input, 1)
	assert.Equal(t, "Hello", input[0]["content"])
}

func TestDoResponseChatAggregatesUpstreamSSEForNonStreamingClient(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	common.SetContextKey(c, constant.ContextKeyIsStream, false)

	info := &relaycommon.RelayInfo{
		RelayMode:   relayconstant.RelayModeChatCompletions,
		RelayFormat: relaytypes.RelayFormatOpenAI,
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "gpt-5.6-sol"},
	}
	response := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body: io.NopCloser(strings.NewReader(strings.Join([]string{
			`data: {"type":"response.created","response":{"id":"resp_1","model":"gpt-5.6-sol","created_at":1710000000}}`,
			`data: {"type":"response.output_text.delta","delta":"hello"}`,
			`data: {"type":"response.completed","response":{"status":"completed","usage":{"input_tokens":2,"output_tokens":3,"total_tokens":5}}}`,
			"",
		}, "\n"))),
	}

	usage, apiErr := (&Adaptor{}).DoResponse(c, response, info)

	require.Nil(t, apiErr)
	require.IsType(t, &dto.Usage{}, usage)
	assert.Equal(t, 5, usage.(*dto.Usage).TotalTokens)
	var chatResponse dto.OpenAITextResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &chatResponse))
	require.Len(t, chatResponse.Choices, 1)
	assert.Equal(t, "hello", chatResponse.Choices[0].Message.StringContent())
}

// The Codex backend rejects these fields, so the adaptor clears them rather
// than forwarding what the client sent.
func TestConvertOpenAIResponsesRequestDropsPenalties(t *testing.T) {
	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{ChannelType: constant.ChannelTypeCodex},
		RelayMode:   relayconstant.RelayModeResponses,
	}

	converted, err := adaptor.ConvertOpenAIResponsesRequest(nil, info, dto.OpenAIResponsesRequest{
		Model:            "gpt-5-codex",
		Input:            json.RawMessage(`"hello"`),
		MaxOutputTokens:  lo.ToPtr(uint(128)),
		Temperature:      lo.ToPtr(1.0),
		FrequencyPenalty: json.RawMessage(`1.5`),
		PresencePenalty:  json.RawMessage(`1.5`),
	})
	require.NoError(t, err)

	request, ok := converted.(dto.OpenAIResponsesRequest)
	require.True(t, ok)
	assert.Nil(t, request.MaxOutputTokens)
	assert.Nil(t, request.Temperature)
	assert.Nil(t, request.FrequencyPenalty)
	assert.Nil(t, request.PresencePenalty)
}
