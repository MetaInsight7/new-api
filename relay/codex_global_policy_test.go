package relay

import (
	"fmt"
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
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/model_setting"
	hosttypes "github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type globalChatPolicyUpstreamRequest struct {
	path string
	body []byte
}

type globalChatPolicyBilling struct{ settled []int }

func (billing *globalChatPolicyBilling) Settle(quota int) error {
	billing.settled = append(billing.settled, quota)
	return nil
}
func (billing *globalChatPolicyBilling) Refund(*gin.Context)      {}
func (billing *globalChatPolicyBilling) NeedsRefund() bool        { return len(billing.settled) == 0 }
func (billing *globalChatPolicyBilling) GetPreConsumedQuota() int { return 100 }
func (billing *globalChatPolicyBilling) Reserve(int) error        { return nil }

func newGlobalChatPolicyFixture(test *testing.T, channelType int, stream bool, upstreamBody string) (*gin.Context, *httptest.ResponseRecorder, *relaycommon.RelayInfo, *globalChatPolicyBilling, <-chan globalChatPolicyUpstreamRequest) {
	test.Helper()
	settings := model_setting.GetGlobalSettings()
	previousSettings := *settings
	previousBatch, previousLogs := common.BatchUpdateEnabled, common.LogConsumeEnabled
	previousTimeout := constant.StreamingTimeout
	settings.PassThroughRequestEnabled = false
	settings.ChatCompletionsToResponsesPolicy = model_setting.ChatCompletionsToResponsesPolicy{
		Enabled: true, AllChannels: true, ModelPatterns: []string{`^gpt-5-codex$`},
	}
	common.BatchUpdateEnabled, common.LogConsumeEnabled = true, false
	constant.StreamingTimeout = 30
	test.Cleanup(func() {
		*settings = previousSettings
		common.BatchUpdateEnabled, common.LogConsumeEnabled = previousBatch, previousLogs
		constant.StreamingTimeout = previousTimeout
	})

	captured := make(chan globalChatPolicyUpstreamRequest, 1)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, err := io.ReadAll(request.Body)
		if err != nil {
			http.Error(writer, "invalid request body", http.StatusBadRequest)
			return
		}
		captured <- globalChatPolicyUpstreamRequest{path: request.URL.Path, body: body}
		writer.Header().Set("Content-Type", "text/event-stream")
		_, _ = writer.Write([]byte(upstreamBody))
	}))
	test.Cleanup(server.Close)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Request.Header.Set("Content-Type", "application/json")
	common.SetContextKey(ctx, constant.ContextKeyOriginalModel, "gpt-5-codex")
	common.SetContextKey(ctx, constant.ContextKeyChannelType, channelType)
	common.SetContextKey(ctx, constant.ContextKeyChannelId, 123)
	common.SetContextKey(ctx, constant.ContextKeyChannelBaseUrl, server.URL)
	key := "local-api-key"
	if channelType == constant.ChannelTypeCodex {
		key = `{"access_token":"local-access-token","account_id":"local-account"}`
	}
	common.SetContextKey(ctx, constant.ContextKeyChannelKey, key)
	request := &dto.GeneralOpenAIRequest{
		Model: "gpt-5-codex", Stream: &stream,
		Messages: []dto.Message{{Role: "user", Content: "hello"}},
	}
	info := relaycommon.GenRelayInfoOpenAI(ctx, request)
	billing := &globalChatPolicyBilling{}
	info.Billing, info.IsPlayground, info.DisablePing = billing, true, true
	info.UserQuota, info.UserSetting.QuotaWarningThreshold = 1000, 1
	info.PriceData = hosttypes.PriceData{ModelRatio: 1, CompletionRatio: 1, CacheRatio: 1, GroupRatioInfo: hosttypes.GroupRatioInfo{GroupRatio: 1}}
	info.SetEstimatePromptTokens(13)
	require.True(test, service.ShouldChatCompletionsUseResponsesGlobal(123, channelType, "gpt-5-codex"))
	return ctx, recorder, info, billing, captured
}

func TestCodexGlobalChatResponsesPolicyRejectsMissingTerminal(test *testing.T) {
	for _, stream := range []bool{true, false} {
		for _, streamMarker := range []bool{true, false} {
			test.Run(fmt.Sprintf("stream_%t/marker_%t", stream, streamMarker), func(test *testing.T) {
				body := `data: {"type":"response.created","response":{"id":"resp_prelude"}}` + "\n\ndata: [DONE]\n\n"
				ctx, recorder, info, billing, captured := newGlobalChatPolicyFixture(test, constant.ChannelTypeCodex, stream, body)
				if !streamMarker {
					delete(ctx.Keys, string(constant.ContextKeyIsStream))
				}
				apiErr := TextHelper(ctx, info)
				require.NotNil(test, apiErr)
				assert.Equal(test, http.StatusBadGateway, apiErr.StatusCode)
				assert.False(test, types.IsSkipRetryError(apiErr))
				assert.True(test, service.ShouldRetryRelayError(ctx, apiErr, 1))
				assert.False(test, ctx.Writer.Written())
				assert.Empty(test, recorder.Body.String())
				assert.Empty(test, billing.settled)
				assert.False(test, common.GetContextKeyBool(ctx, constant.ContextKeyLocalCountTokens))
				assert.Equal(test, stream, info.IsStream)
				assert.Equal(test, relayconstant.RelayModeChatCompletions, info.RelayMode)
				assert.Equal(test, "/v1/chat/completions", info.RequestURLPath)
				assert.Equal(test, []types.RelayFormat{types.RelayFormatOpenAI, types.RelayFormatOpenAIResponses}, info.RequestConversionChain)
				require.Len(test, captured, 1)
				upstream := <-captured
				assert.Equal(test, "/backend-api/codex/responses", upstream.path)
				var request dto.OpenAIResponsesRequest
				require.NoError(test, common.Unmarshal(upstream.body, &request))
				require.NotNil(test, request.Stream)
				assert.True(test, *request.Stream)
			})
		}
	}
}

func TestCodexGlobalChatResponsesPolicyPreservesChatContract(test *testing.T) {
	textDelta := `data: {"type":"response.output_text.delta","delta":"hello"}` + "\n\n"
	for _, stream := range []bool{true, false} {
		for _, scenario := range []struct {
			name           string
			prelude        string
			terminal       string
			wantPrompt     int
			wantCompletion int
			wantStatus     int
			wantFinish     string
			wantEstimate   bool
			wantSettlement bool
			withoutUsage   bool
			withoutMarker  bool
			staleStream    bool
			wantDetails    bool
		}{
			{
				name:       "native usage ignores billing sidecar",
				terminal:   `data: {"type":"response.completed","response":{"status":"completed","usage":{"input_tokens":20,"output_tokens":5,"total_tokens":25,"input_tokens_details":{"cached_tokens":4},"output_tokens_details":{"reasoning_tokens":2},"prompt_tokens":999,"completion_tokens":999,"billing_usage":{"source":"oai_chat","semantic":"openai","openai_usage":{"prompt_tokens":999,"completion_tokens":999,"total_tokens":1998}}}}}` + "\n\n",
				wantPrompt: 20, wantCompletion: 5, wantFinish: "stop", wantSettlement: true, wantDetails: true,
			},
			{
				name:       "completed without usage",
				terminal:   `data: {"type":"response.completed","response":{"status":"completed"}}` + "\n\n",
				wantPrompt: 13, wantCompletion: 2, wantFinish: "stop", wantEstimate: true, wantSettlement: true,
			},
			{
				name:       "completed without usage chunk",
				terminal:   `data: {"type":"response.completed","response":{"status":"completed"}}` + "\n\n",
				wantPrompt: 13, wantCompletion: 2, wantFinish: "stop", wantEstimate: true, wantSettlement: true, withoutUsage: true,
			},
			{
				name:       "completed without stream marker",
				terminal:   `data: {"type":"response.completed","response":{"status":"completed"}}` + "\n\n",
				wantPrompt: 13, wantCompletion: 2, wantFinish: "stop", wantEstimate: true, wantSettlement: true, withoutMarker: true,
			},
			{
				name:       "client marker overrides stale stream info",
				terminal:   `data: {"type":"response.completed","response":{"status":"completed"}}` + "\n\n",
				wantPrompt: 13, wantCompletion: 2, wantFinish: "stop", wantEstimate: true, wantSettlement: true, staleStream: true,
			},
			{
				name:       "native zero never estimates",
				prelude:    `data: {"type":"response.created","response":{"usage":{"input_tokens":999,"output_tokens":999,"total_tokens":1998}}}` + "\n\n",
				terminal:   `data: {"type":"response.completed","response":{"status":"completed","usage":{"input_tokens":0,"output_tokens":0,"total_tokens":0,"prompt_tokens":999,"completion_tokens":999,"billing_usage":{"source":"oai_chat","semantic":"openai","openai_usage":{"prompt_tokens":999,"completion_tokens":999,"total_tokens":1998}}}}}` + "\n\n",
				wantFinish: "stop", wantSettlement: true,
			},
			{
				name:       "incomplete never estimates",
				terminal:   `data: {"type":"response.incomplete","response":{"status":"incomplete","incomplete_details":{"reason":"max_output_tokens"}}}` + "\n\n",
				wantFinish: "length", wantSettlement: true,
			},
			{
				name:       "input error never estimates",
				terminal:   `data: {"type":"response.failed","response":{"status":"failed","error":{"code":"context_length_exceeded","type":"invalid_request_error","message":"private upstream message"}}}` + "\n\n",
				wantStatus: http.StatusBadRequest,
			},
			{
				name:       "auth error settles only native usage",
				terminal:   `data: {"type":"response.failed","response":{"status":"failed","usage":{"input_tokens":20,"output_tokens":5,"total_tokens":25},"error":{"code":"invalid_api_key","type":"authentication_error","message":"private upstream message"}}}` + "\n\n",
				wantPrompt: 20, wantCompletion: 5, wantStatus: http.StatusUnauthorized, wantSettlement: true,
			},
			{name: "truncated never estimates", wantStatus: http.StatusBadGateway},
		} {
			test.Run(fmt.Sprintf("%s/stream_%t", scenario.name, stream), func(test *testing.T) {
				ctx, recorder, info, billing, captured := newGlobalChatPolicyFixture(test, constant.ChannelTypeCodex, stream, scenario.prelude+textDelta+scenario.terminal)
				if scenario.withoutUsage {
					info.Request.(*dto.GeneralOpenAIRequest).StreamOptions = &dto.StreamOptions{IncludeUsage: false}
				}
				if scenario.withoutMarker {
					delete(ctx.Keys, string(constant.ContextKeyIsStream))
				}
				if scenario.staleStream {
					info.IsStream = !stream
				}
				apiErr := TextHelper(ctx, info)
				if scenario.wantStatus != 0 {
					require.NotNil(test, apiErr)
					assert.Equal(test, scenario.wantStatus, apiErr.StatusCode)
					switch scenario.wantStatus {
					case http.StatusBadRequest:
						assert.Equal(test, "context_length_exceeded", apiErr.ToOpenAIError().Code)
						assert.Equal(test, "invalid_request_error", apiErr.ToOpenAIError().Type)
					case http.StatusUnauthorized:
						assert.Equal(test, "invalid_api_key", apiErr.ToOpenAIError().Code)
						assert.Equal(test, "authentication_error", apiErr.ToOpenAIError().Type)
					}
					assert.Equal(test, stream || scenario.wantPrompt > 0, types.IsSkipRetryError(apiErr))
					assert.Equal(test, stream, ctx.Writer.Written())
					assert.NotContains(test, recorder.Body.String(), "[DONE]")
					assert.NotContains(test, recorder.Body.String(), `"finish_reason":"stop"`)
					assert.NotContains(test, recorder.Body.String(), "private upstream message")
					if stream {
						assert.Contains(test, recorder.Body.String(), "hello")
						assert.Contains(test, recorder.Body.String(), `"error":`)
						assert.NotContains(test, recorder.Body.String(), `"usage":{`)
					} else {
						assert.Empty(test, recorder.Body.String())
					}
				} else {
					require.Nil(test, apiErr)
					assert.Contains(test, recorder.Body.String(), "hello")
					assert.Contains(test, recorder.Body.String(), `"finish_reason":"`+scenario.wantFinish+`"`)
					if scenario.wantDetails {
						assert.Contains(test, recorder.Body.String(), `"cached_tokens":4`)
						assert.Contains(test, recorder.Body.String(), `"reasoning_tokens":2`)
					}
					if stream {
						assert.Equal(test, "text/event-stream", recorder.Header().Get("Content-Type"))
						assert.Contains(test, recorder.Body.String(), "[DONE]")
						if scenario.withoutUsage {
							assert.NotContains(test, recorder.Body.String(), `"usage":{`)
						} else {
							assert.Equal(test, 1, strings.Count(recorder.Body.String(), `"usage":{`))
							assert.Contains(test, recorder.Body.String(), fmt.Sprintf(`"prompt_tokens":%d`, scenario.wantPrompt))
							assert.Contains(test, recorder.Body.String(), fmt.Sprintf(`"completion_tokens":%d`, scenario.wantCompletion))
						}
					} else {
						assert.Contains(test, recorder.Header().Get("Content-Type"), "application/json")
						assert.NotContains(test, recorder.Body.String(), "[DONE]")
						var response dto.OpenAITextResponse
						require.NoError(test, common.Unmarshal(recorder.Body.Bytes(), &response))
						assert.Equal(test, scenario.wantPrompt, response.Usage.PromptTokens)
						assert.Equal(test, scenario.wantCompletion, response.Usage.CompletionTokens)
					}
				}
				assert.Equal(test, scenario.wantEstimate, common.GetContextKeyBool(ctx, constant.ContextKeyLocalCountTokens))
				assert.Equal(test, stream, info.IsStream)
				if scenario.wantSettlement {
					assert.Equal(test, []int{scenario.wantPrompt + scenario.wantCompletion}, billing.settled)
				} else {
					assert.Empty(test, billing.settled)
				}
				require.Len(test, captured, 1)
				upstream := <-captured
				assert.Equal(test, "/backend-api/codex/responses", upstream.path)
				var request dto.OpenAIResponsesRequest
				require.NoError(test, common.Unmarshal(upstream.body, &request))
				require.NotNil(test, request.Stream)
				assert.True(test, *request.Stream)
			})
		}
	}
}

func TestOpenAIGlobalChatResponsesPolicyPreservesGenericBridge(test *testing.T) {
	for _, stream := range []bool{true, false} {
		test.Run(fmt.Sprintf("stream_%t", stream), func(test *testing.T) {
			body := `data: {"type":"response.output_text.delta","delta":"hello"}` + "\n\n" +
				`data: {"type":"response.completed","response":{"status":"completed","usage":{"input_tokens":3,"output_tokens":2,"total_tokens":5}}}` + "\n\n"
			ctx, recorder, info, billing, captured := newGlobalChatPolicyFixture(test, constant.ChannelTypeOpenAI, stream, body)
			require.Nil(test, TextHelper(ctx, info))
			assert.Equal(test, []int{5}, billing.settled)
			assert.Contains(test, recorder.Body.String(), "hello")
			assert.Contains(test, recorder.Body.String(), `"prompt_tokens":3`)
			assert.Contains(test, recorder.Body.String(), `"completion_tokens":2`)
			if stream {
				assert.Contains(test, recorder.Body.String(), "[DONE]")
				assert.Equal(test, "text/event-stream", recorder.Header().Get("Content-Type"))
			} else {
				assert.NotContains(test, recorder.Body.String(), "[DONE]")
				var response dto.OpenAITextResponse
				require.NoError(test, common.Unmarshal(recorder.Body.Bytes(), &response))
				assert.Equal(test, 5, response.Usage.TotalTokens)
			}
			assert.False(test, common.GetContextKeyBool(ctx, constant.ContextKeyLocalCountTokens))
			assert.Equal(test, []types.RelayFormat{types.RelayFormatOpenAI, types.RelayFormatOpenAIResponses}, info.RequestConversionChain)
			require.Len(test, captured, 1)
			upstream := <-captured
			assert.Equal(test, "/v1/responses", upstream.path)
			var request dto.OpenAIResponsesRequest
			require.NoError(test, common.Unmarshal(upstream.body, &request))
			require.NotNil(test, request.Stream)
			assert.Equal(test, stream, *request.Stream)
		})
	}
}
