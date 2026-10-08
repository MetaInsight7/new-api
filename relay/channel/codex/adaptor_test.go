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
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
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

func TestConvertResponsesAppliesCodexConstraints(t *testing.T) {
	for _, testCase := range []struct {
		name    string
		mode    int
		compact bool
	}{
		{name: "responses", mode: relayconstant.RelayModeResponses},
		{name: "compact", mode: relayconstant.RelayModeResponsesCompact, compact: true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			var request dto.OpenAIResponsesRequest
			require.NoError(t, common.Unmarshal([]byte(`{
				"model":"gpt-5.5","input":"hello","include":[],"conversation":"conv_1",
				"context_management":[{"type":"compaction","compact_threshold":0}],
				"truncation":"disabled","prompt":{"id":"prompt_1"},"text":{"format":{"type":"text"}},
				"tools":[{"type":"web_search"}],"tool_choice":"auto","reasoning":{"effort":"high"},
				"prompt_cache_key":"cache","service_tier":"fast","parallel_tool_calls":false,
				"max_tool_calls":0,"top_logprobs":0,"stream":false,"store":true,
				"frequency_penalty":0,"presence_penalty":0,"user":"user_1","metadata":{"tag":"private"},
				"stream_options":{"include_usage":true},"prompt_cache_options":{},
				"prompt_cache_retention":"24h","safety_identifier":"user_1",
				"max_output_tokens":0,"temperature":0,"top_p":0,"client_metadata":{"session_id":"client"}
			}`), &request))
			request.ReasoningConversion = &dto.ReasoningConversionState{}
			info := &relaycommon.RelayInfo{RelayMode: testCase.mode, ChannelMeta: &relaycommon.ChannelMeta{}}
			converted, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(nil, info, request)
			require.NoError(t, err)
			responsesRequest, ok := converted.(dto.OpenAIResponsesRequest)
			require.True(t, ok)
			assert.Same(t, request.ReasoningConversion, responsesRequest.ReasoningConversion)
			wire, err := common.Marshal(converted)
			require.NoError(t, err)
			for _, field := range []string{
				"frequency_penalty", "presence_penalty", "user", "metadata", "stream_options",
				"prompt_cache_options", "prompt_cache_retention", "safety_identifier",
				"max_output_tokens", "max_completion_tokens", "temperature", "top_p",
			} {
				assert.False(t, gjson.GetBytes(wire, field).Exists(), field)
			}
			for field, expected := range map[string]string{
				"input": `"hello"`, "include": `[]`, "conversation": `"conv_1"`,
				"context_management": `[{"type":"compaction","compact_threshold":0}]`,
				"truncation":         `"disabled"`, "prompt": `{"id":"prompt_1"}`, "text": `{"format":{"type":"text"}}`,
				"tools": `[{"type":"web_search"}]`, "tool_choice": `"auto"`, "reasoning": `{"effort":"high"}`,
				"prompt_cache_key": `"cache"`, "service_tier": `"fast"`, "parallel_tool_calls": `false`,
				"instructions": `""`,
			} {
				assert.JSONEq(t, expected, gjson.GetBytes(wire, field).Raw, field)
			}
			if testCase.compact {
				for _, field := range []string{"store", "stream", "max_tool_calls", "top_logprobs", "client_metadata"} {
					assert.False(t, gjson.GetBytes(wire, field).Exists(), field)
				}
			} else {
				for _, field := range []string{"store", "stream"} {
					assert.Equal(t, "false", gjson.GetBytes(wire, field).Raw, field)
				}
				for _, field := range []string{"max_tool_calls", "top_logprobs"} {
					assert.Equal(t, "0", gjson.GetBytes(wire, field).Raw, field)
				}
				assert.JSONEq(t, `{"session_id":"client"}`, gjson.GetBytes(wire, "client_metadata").Raw)
			}
		})
	}
}

func TestChatFastUsesExistingServiceTierGate(t *testing.T) {
	settings := model_setting.GetGlobalSettings()
	previous := settings.PassThroughRequestEnabled
	settings.PassThroughRequestEnabled = false
	t.Cleanup(func() { settings.PassThroughRequestEnabled = previous })
	for _, modelName := range []string{"gpt-5.4", "gpt-5.5"} {
		for _, testCase := range []struct {
			name    string
			tier    json.RawMessage
			allowed bool
			want    string
		}{
			{name: "explicit fast allowed", tier: json.RawMessage(`"fast"`), allowed: true, want: "fast"},
			{name: "explicit fast disabled", tier: json.RawMessage(`"fast"`)},
			{name: "absent is not injected", allowed: true},
		} {
			t.Run(modelName+"/"+testCase.name, func(t *testing.T) {
				info := &relaycommon.RelayInfo{
					RelayMode: relayconstant.RelayModeChatCompletions,
					ChannelMeta: &relaycommon.ChannelMeta{
						ChannelType:          constant.ChannelTypeCodex,
						ChannelOtherSettings: dto.ChannelOtherSettings{AllowServiceTier: testCase.allowed},
					},
				}
				converted, err := (&Adaptor{}).ConvertOpenAIRequest(nil, info, &dto.GeneralOpenAIRequest{
					Model: modelName, ServiceTier: testCase.tier,
					Messages: []dto.Message{{Role: "user", Content: "hello"}},
				})
				require.NoError(t, err)
				raw, err := common.Marshal(converted)
				require.NoError(t, err)
				raw, err = relaycommon.RemoveDisabledFields(raw, info.ChannelOtherSettings, info.ChannelSetting.PassThroughBodyEnabled)
				require.NoError(t, err)
				raw, err = ApplyCodexRequestConstraints(raw, false)
				require.NoError(t, err)
				if testCase.want == "" {
					assert.False(t, gjson.GetBytes(raw, "service_tier").Exists())
				} else {
					assert.Equal(t, testCase.want, gjson.GetBytes(raw, "service_tier").String())
				}
			})
		}
	}
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
	assert.Nil(t, request.Stream)
}
