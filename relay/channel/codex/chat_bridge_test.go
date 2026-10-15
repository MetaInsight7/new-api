package codex

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	perfmetrics "github.com/QuantumNous/new-api/pkg/perf_metrics"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	hosttypes "github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newCodexChatBridgeFixture(stream bool, body io.ReadCloser) (*gin.Context, *httptest.ResponseRecorder, *relaycommon.RelayInfo, *http.Response) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Writer.Header().Set("X-Fixture", "unchanged")
	common.SetContextKey(ctx, constant.ContextKeyIsStream, stream)
	info := &relaycommon.RelayInfo{
		IsStream: true, RelayFormat: types.RelayFormatOpenAI, RelayMode: relayconstant.RelayModeChatCompletions,
		ChannelMeta: &relaycommon.ChannelMeta{ChannelType: constant.ChannelTypeCodex, UpstreamModelName: "gpt-5-codex"},
	}
	response := &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"text/event-stream"}}, Body: body}
	return ctx, recorder, info, response
}

func TestRelayChatOverCodexPreservesFailureUsageAndSendsOnlyStreamErrorAfterOutput(t *testing.T) {
	for _, output := range []string{
		`{"type":"response.output_text.delta","delta":"partial text"}`,
		`{"type":"response.reasoning_text.delta","delta":"partial reasoning"}`,
		`{"type":"response.output_item.added","output_index":0,"item":{"type":"function_call","call_id":"call_1","name":"lookup"}}`,
	} {
		t.Run(output, func(t *testing.T) {
			body := "data: " + output + "\n\n" +
				`data: {"type":"response.failed","response":{"status":"failed","error":{"message":"secret key https://private.example"},"usage":{"input_tokens":7,"output_tokens":2,"total_tokens":9,"input_tokens_details":{"cached_tokens":4},"output_tokens_details":{"reasoning_tokens":1}}}}` + "\n\n"
			ctx, recorder, info, response := newCodexChatBridgeFixture(true, io.NopCloser(strings.NewReader(body)))
			usage, apiErr := RelayChatOverCodex(ctx, info, response)
			require.NotNil(t, apiErr)
			assert.Equal(t, 7, usage.PromptTokens)
			assert.Equal(t, 2, usage.CompletionTokens)
			assert.Equal(t, 4, usage.PromptTokensDetails.CachedTokens)
			assert.Equal(t, 1, usage.CompletionTokenDetails.ReasoningTokens)
			assert.True(t, ctx.Writer.Written())
			assert.True(t, types.IsSkipRetryError(apiErr))
			assert.False(t, service.ShouldRetryRelayError(ctx, apiErr, 1))
			assert.Contains(t, recorder.Body.String(), `data: {"error":`)
			assert.NotContains(t, recorder.Body.String(), "secret")
			assert.NotContains(t, recorder.Body.String(), "[DONE]")
			assert.NotContains(t, recorder.Body.String(), `"finish_reason":"stop"`)
		})
	}
}

func TestRelayChatOverCodexPreservesTerminalErrorClassification(t *testing.T) {
	for _, scenario := range []struct {
		name      string
		event     string
		code      string
		errorType string
		status    int
		outcome   perfmetrics.Outcome
	}{
		{"input rejection", `{"type":"response.failed","response":{"status":"failed","error":{"code":"context_length_exceeded","type":"invalid_request_error","message":"secret diagnostic"}}}`, "context_length_exceeded", "invalid_request_error", 400, perfmetrics.OutcomeFailure},
		{"credentials take precedence", `{"type":"response.failed","response":{"status":"failed","error":{"code":"invalid_api_key","type":"invalid_request_error","message":"secret diagnostic"}}}`, "invalid_api_key", "invalid_request_error", 401, perfmetrics.OutcomeFailure},
		{"server failure", `{"type":"response.failed","response":{"status":"failed","error":{"code":"server_error","type":"server_error","message":"secret diagnostic"}}}`, "server_error", "server_error", 500, perfmetrics.OutcomeFailure},
		{"flat input rejection", `{"type":"error","code":"context_length_exceeded","message":"secret diagnostic"}`, "context_length_exceeded", "upstream_error", 400, perfmetrics.OutcomeFailure},
	} {
		for _, stream := range []bool{false, true} {
			for _, afterOutput := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/stream_%t/after_output_%t", scenario.name, stream, afterOutput), func(t *testing.T) {
					body := "data: " + scenario.event + "\n\n"
					if afterOutput {
						body = "data: {\"type\":\"response.output_text.delta\",\"delta\":\"partial text\"}\n\n" + body
					}
					ctx, recorder, info, response := newCodexChatBridgeFixture(stream, io.NopCloser(strings.NewReader(body)))
					usage, apiErr := RelayChatOverCodex(ctx, info, response)
					require.NotNil(t, apiErr)
					assert.Zero(t, usage.TotalTokens)
					assert.Equal(t, scenario.code, string(apiErr.GetErrorCode()))
					assert.Equal(t, scenario.status, apiErr.StatusCode)
					assert.Equal(t, scenario.errorType, apiErr.ToOpenAIError().Type)
					assert.Equal(t, scenario.outcome, perfmetrics.ClassifyRelayOutcome(ctx.Request.Context(), info, apiErr))
					assert.Equal(t, stream && afterOutput, types.IsSkipRetryError(apiErr))
					assert.NotContains(t, apiErr.Error(), "secret")
					assert.NotContains(t, recorder.Body.String(), "secret")
					assert.NotContains(t, recorder.Body.String(), "[DONE]")
					if stream && afterOutput {
						assert.Contains(t, recorder.Body.String(), `"code":"`+scenario.code+`"`)
					}
				})
			}
		}
	}
}

func TestRelayChatOverCodexRejectsTruncationWithoutInventingUsage(t *testing.T) {
	for _, stream := range []bool{true, false} {
		t.Run(map[bool]string{true: "stream", false: "buffered"}[stream], func(t *testing.T) {
			body := "data: " + `{"type":"response.output_text.delta","delta":"partial text"}` + "\n\n"
			ctx, recorder, info, response := newCodexChatBridgeFixture(stream, io.NopCloser(strings.NewReader(body)))
			info.SetEstimatePromptTokens(999)
			usage, apiErr := RelayChatOverCodex(ctx, info, response)
			require.NotNil(t, apiErr)
			assert.Equal(t, http.StatusBadGateway, apiErr.StatusCode)
			assert.Zero(t, usage.TotalTokens)
			assert.Zero(t, usage.PromptTokens)
			assert.Zero(t, usage.CompletionTokens)
			assert.NotContains(t, recorder.Body.String(), "[DONE]")
			assert.NotContains(t, recorder.Body.String(), `"finish_reason":"stop"`)
			assert.Equal(t, stream, ctx.Writer.Written())
			assert.Equal(t, stream, types.IsSkipRetryError(apiErr))
			assert.Equal(t, !stream, service.ShouldRetryRelayError(ctx, apiErr, 1))
		})
	}
}

func TestRelayChatOverCodexPreservesUsageWhenResponseStatusFails(t *testing.T) {
	for _, stream := range []bool{true, false} {
		t.Run(map[bool]string{true: "stream", false: "buffered"}[stream], func(t *testing.T) {
			body := `data: {"type":"response.in_progress","response":{"status":"failed","usage":{"input_tokens":7,"output_tokens":2,"total_tokens":9}}}` + "\n\n"
			ctx, recorder, info, response := newCodexChatBridgeFixture(stream, io.NopCloser(strings.NewReader(body)))
			usage, apiErr := RelayChatOverCodex(ctx, info, response)
			require.NotNil(t, apiErr)
			assert.Equal(t, 9, usage.TotalTokens)
			assert.Equal(t, 7, usage.PromptTokens)
			assert.Equal(t, 2, usage.CompletionTokens)
			assert.True(t, types.IsSkipRetryError(apiErr))
			assert.False(t, ctx.Writer.Written())
			assert.Empty(t, recorder.Body.String())
		})
	}
}

func TestRelayChatOverCodexRejectsInvalidTerminalUsage(t *testing.T) {
	for _, rawUsage := range []string{
		`{"input_tokens":-1,"output_tokens":2}`,
		`{"input_tokens":2147483647,"output_tokens":1}`,
		`{"input_tokens":7,"output_tokens":2,"input_tokens_details":{"cached_tokens":-1}}`,
		`{"input_tokens":7,"output_tokens":2,"input_tokens_details":{"cache_write_tokens":8}}`,
		`{"input_tokens":7,"output_tokens":2,"output_tokens_details":{"reasoning_tokens":-1}}`,
	} {
		for _, terminal := range []string{"completed", "failed"} {
			t.Run(terminal+rawUsage, func(t *testing.T) {
				body := `data: {"type":"response.` + terminal + `","response":{"status":"` + terminal + `","usage":` + rawUsage + `}}` + "\n\n"
				ctx, recorder, info, response := newCodexChatBridgeFixture(true, io.NopCloser(strings.NewReader(body)))
				info.SetEstimatePromptTokens(999)
				usage, apiErr := RelayChatOverCodex(ctx, info, response)
				require.NotNil(t, apiErr)
				assert.Zero(t, usage.TotalTokens)
				assert.False(t, ctx.Writer.Written())
				assert.Empty(t, recorder.Body.String())
			})
		}
	}
}

func TestRelayChatOverCodexRejectsPreludeOnlyDoneAndMalformedStreams(t *testing.T) {
	for _, tail := range []string{"", "data: [DONE]\n\n", "data: {not-json}\n\n", "data: " + strings.Repeat("x", (1<<20)+1)} {
		body := "data: " + `{"type":"response.created","response":{"id":"resp_1"}}` + "\n\n" + tail
		ctx, recorder, info, response := newCodexChatBridgeFixture(true, io.NopCloser(strings.NewReader(body)))
		_, apiErr := RelayChatOverCodex(ctx, info, response)
		require.NotNil(t, apiErr)
		assert.False(t, ctx.Writer.Written())
		assert.Empty(t, recorder.Body.String())
		assert.True(t, service.ShouldRetryRelayError(ctx, apiErr, 1))
	}
}

func TestRelayChatOverCodexConvertsCompletedAndIncompleteTerminals(t *testing.T) {
	for _, stream := range []bool{true, false} {
		for _, terminal := range []string{"completed", "incomplete"} {
			t.Run(map[bool]string{true: "stream-", false: "buffered-"}[stream]+terminal, func(t *testing.T) {
				body := "event: response.output_text.delta\ndata: {\n" +
					"data: \"delta\":\"hello\",\"output_index\":0}\n\n" +
					"data: {\"type\":\"response." + terminal + "\",\"response\":{\"status\":\"" + terminal + "\",\"incomplete_details\":{\"reason\":\"max_output_tokens\"},\"usage\":{\"input_tokens\":11,\"output_tokens\":7}}}"
				ctx, recorder, info, response := newCodexChatBridgeFixture(stream, io.NopCloser(strings.NewReader(body)))
				info.ShouldIncludeUsage = true
				usage, apiErr := RelayChatOverCodex(ctx, info, response)
				require.Nil(t, apiErr)
				assert.Equal(t, 11, usage.PromptTokens)
				assert.Equal(t, 7, usage.CompletionTokens)
				assert.Contains(t, recorder.Body.String(), `"model":"gpt-5-codex"`)
				assert.Contains(t, recorder.Body.String(), "hello")
				assert.Contains(t, recorder.Body.String(), `"usage":`)
				if terminal == "incomplete" {
					assert.Contains(t, recorder.Body.String(), `"finish_reason":"length"`)
				}
				if stream {
					assert.Equal(t, "text/event-stream", recorder.Header().Get("Content-Type"))
					assert.Contains(t, recorder.Body.String(), "[DONE]")
				} else {
					assert.Contains(t, recorder.Header().Get("Content-Type"), "application/json")
					assert.NotContains(t, recorder.Body.String(), "[DONE]")
				}
			})
		}
	}
}

func TestRelayChatOverCodexBillsOnlyNativeResponsesUsage(test *testing.T) {
	for _, extra := range []string{
		`"prompt_tokens":1,"completion_tokens":1`,
		`"billing_usage":{"source":"oai_chat","semantic":"openai","openai_usage":{"prompt_tokens":-10,"completion_tokens":-5,"total_tokens":-15}}`,
		`"prompt_tokens_details":{"cached_tokens":20},"usage_semantic":"anthropic","cost":-100`,
	} {
		for _, stream := range []bool{true, false} {
			test.Run(fmt.Sprintf("%s/stream_%t", extra, stream), func(test *testing.T) {
				body := `data: {"type":"response.output_text.delta","delta":"hello"}` + "\n\n" +
					`data: {"type":"response.completed","response":{"status":"completed","usage":{"input_tokens":20,"output_tokens":5,"total_tokens":25,"input_tokens_details":{"cached_tokens":4},` + extra + `}}}` + "\n\n"
				ctx, recorder, info, response := newCodexChatBridgeFixture(stream, io.NopCloser(strings.NewReader(body)))
				info.ShouldIncludeUsage = true
				usage, apiErr := (&Adaptor{}).DoResponse(ctx, response, info)
				require.Nil(test, apiErr)
				actualUsage := usage.(*dto.Usage)
				require.NotNil(test, actualUsage.BillingUsage)
				canonical, present := actualUsage.BillingUsage.CanonicalUsage()
				require.True(test, present)
				assert.Equal(test, 20, canonical.PromptTokens)
				assert.Equal(test, 5, canonical.CompletionTokens)
				assert.Equal(test, 25, canonical.TotalTokens)
				assert.Equal(test, 4, canonical.PromptTokensDetails.CachedTokens)
				assert.Equal(test, dto.BillingUsageSemanticOpenAI, canonical.UsageSemantic)
				assert.Contains(test, recorder.Body.String(), `"prompt_tokens":20`)
				assert.Contains(test, recorder.Body.String(), `"completion_tokens":5`)
			})
		}
	}
}

func TestRelayChatOverCodexCompletedUsageFallbackRequiresContent(test *testing.T) {
	previousBatch, previousLogs := common.BatchUpdateEnabled, common.LogConsumeEnabled
	common.BatchUpdateEnabled, common.LogConsumeEnabled = true, false
	test.Cleanup(func() { common.BatchUpdateEnabled, common.LogConsumeEnabled = previousBatch, previousLogs })
	for _, stream := range []bool{true, false} {
		for _, scenario := range []struct {
			name       string
			prelude    string
			output     string
			wantTokens int
		}{
			{name: "text delta", prelude: `data: {"type":"response.output_text.delta","delta":"hello"}` + "\n\n", wantTokens: 15},
			{name: "terminal text", output: `,"output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hello"}]}]`, wantTokens: 15},
			{name: "reasoning delta", prelude: `data: {"type":"response.reasoning_text.delta","delta":"hello"}` + "\n\n", wantTokens: 15},
			{name: "terminal reasoning", output: `,"output":[{"type":"reasoning","summary":[{"type":"summary_text","text":"hello"}]}]`, wantTokens: 15},
			{name: "tool deltas", prelude: `data: {"type":"response.output_item.added","output_index":0,"item":{"type":"function_call","id":"fc_1","call_id":"call_1","name":"hello"}}` + "\n\n" + `data: {"type":"response.function_call_arguments.delta","output_index":0,"delta":"{}"}` + "\n\n", wantTokens: 15},
			{name: "terminal tool", output: `,"output":[{"type":"function_call","id":"fc_1","call_id":"call_1","name":"hello","arguments":"{}"}]`, wantTokens: 15},
			{name: "empty terminal"},
			{name: "role prelude", prelude: `data: {"type":"response.created","response":{"id":"resp_empty"}}` + "\n\n"},
		} {
			test.Run(fmt.Sprintf("%s/stream_%t", scenario.name, stream), func(test *testing.T) {
				body := scenario.prelude + `data: {"type":"response.completed","response":{"status":"completed"` + scenario.output + `}}` + "\n\n"
				ctx, recorder, info, response := newCodexChatBridgeFixture(stream, io.NopCloser(strings.NewReader(body)))
				billing := &codexChatBillingFixture{}
				info.Billing, info.IsPlayground, info.StartTime, info.OriginModelName = billing, true, time.Now(), "gpt-5-codex"
				info.UserQuota, info.UserSetting.QuotaWarningThreshold = 1000, 1
				info.PriceData = hosttypes.PriceData{ModelRatio: 1, CompletionRatio: 1, CacheRatio: 1, GroupRatioInfo: hosttypes.GroupRatioInfo{GroupRatio: 1}}
				info.ShouldIncludeUsage = true
				info.SetEstimatePromptTokens(13)
				usage, apiErr := RelayChatOverCodex(ctx, info, response)
				require.Nil(test, apiErr)
				require.NotNil(test, usage)
				assert.Equal(test, scenario.wantTokens, usage.TotalTokens)
				assert.Equal(test, scenario.wantTokens > 0, common.GetContextKeyBool(ctx, constant.ContextKeyLocalCountTokens))
				if scenario.wantTokens > 0 {
					assert.Equal(test, 13, usage.PromptTokens)
					assert.Equal(test, 2, usage.CompletionTokens)
					assert.Contains(test, recorder.Body.String(), `"prompt_tokens":13`)
					assert.Contains(test, recorder.Body.String(), `"completion_tokens":2`)
					assert.Contains(test, recorder.Body.String(), "hello")
					if stream {
						assert.Equal(test, 1, strings.Count(recorder.Body.String(), `"usage":{`))
					}
				} else {
					assert.Zero(test, usage.PromptTokens)
					assert.Zero(test, usage.CompletionTokens)
				}
				service.PostTextConsumeQuota(ctx, info, usage, nil)
				assert.Equal(test, []int{scenario.wantTokens}, billing.settled)
			})
		}
	}
}

func TestRelayChatOverCodexIncompleteWithoutUsageDoesNotEstimate(t *testing.T) {
	for _, stream := range []bool{true, false} {
		t.Run(map[bool]string{true: "stream", false: "buffered"}[stream], func(t *testing.T) {
			body := `data: {"type":"response.output_text.delta","delta":"partial text"}` + "\n\n" +
				`data: {"type":"response.incomplete","response":{"status":"incomplete","incomplete_details":{"reason":"max_output_tokens"}}}` + "\n\n"
			ctx, recorder, info, response := newCodexChatBridgeFixture(stream, io.NopCloser(strings.NewReader(body)))
			info.SetEstimatePromptTokens(999)
			usage, apiErr := RelayChatOverCodex(ctx, info, response)
			require.Nil(t, apiErr)
			assert.Zero(t, usage.TotalTokens)
			assert.Zero(t, usage.PromptTokens)
			assert.Zero(t, usage.CompletionTokens)
			assert.False(t, common.GetContextKeyBool(ctx, constant.ContextKeyLocalCountTokens))
			assert.Contains(t, recorder.Body.String(), `"finish_reason":"length"`)
		})
	}
}

func TestCodexChatAdaptorRejectsPreludeOnlyDoneWithoutCommitting(t *testing.T) {
	for _, stream := range []bool{true, false} {
		t.Run(map[bool]string{true: "stream", false: "buffered"}[stream], func(t *testing.T) {
			body := "data: " + `{"type":"response.created","response":{"id":"resp_dispatch"}}` + "\n\ndata: [DONE]\n\n"
			ctx, recorder, info, response := newCodexChatBridgeFixture(stream, io.NopCloser(strings.NewReader(body)))
			reportedUsage, apiErr := (&Adaptor{}).DoResponse(ctx, response, info)
			require.NotNil(t, apiErr)
			usage, validUsage := reportedUsage.(*dto.Usage)
			require.True(t, validUsage)
			require.NotNil(t, usage)
			assert.Zero(t, usage.TotalTokens)
			assert.Equal(t, http.StatusBadGateway, apiErr.StatusCode)
			assert.False(t, ctx.Writer.Written())
			assert.Empty(t, recorder.Body.String())
			assert.Empty(t, recorder.Header().Get("Content-Type"))
			assert.False(t, types.IsSkipRetryError(apiErr))
			assert.True(t, service.ShouldRetryRelayError(ctx, apiErr, 1))
		})
	}
}

type codexChatBlockedBody struct {
	started   chan struct{}
	closed    chan struct{}
	startOnce sync.Once
	closeOnce sync.Once
}

func (body *codexChatBlockedBody) Read(buffer []byte) (int, error) {
	body.startOnce.Do(func() { close(body.started) })
	<-body.closed
	return 0, io.ErrClosedPipe
}

func (body *codexChatBlockedBody) Close() error {
	body.closeOnce.Do(func() { close(body.closed) })
	return nil
}

func TestRelayChatOverCodexCancellationClosesBlockedUpstreamWithoutPrelude(t *testing.T) {
	body := &codexChatBlockedBody{started: make(chan struct{}), closed: make(chan struct{})}
	ctx, recorder, info, response := newCodexChatBridgeFixture(true, body)
	requestContext, cancel := context.WithCancel(ctx.Request.Context())
	defer cancel()
	ctx.Request = ctx.Request.WithContext(requestContext)
	finished := make(chan *types.NewAPIError, 1)
	go func() {
		_, apiErr := RelayChatOverCodex(ctx, info, response)
		finished <- apiErr
	}()
	<-body.started
	cancel()
	apiErr := <-finished
	require.NotNil(t, apiErr)
	assert.True(t, types.IsSkipRetryError(apiErr))
	assert.Equal(t, relaycommon.StreamEndReasonClientGone, info.StreamStatus.EndReason)
	assert.False(t, ctx.Writer.Written())
	assert.Empty(t, recorder.Body.String())
}

type codexChatBillingFixture struct{ settled []int }

func (billing *codexChatBillingFixture) Settle(quota int) error {
	billing.settled = append(billing.settled, quota)
	return nil
}
func (billing *codexChatBillingFixture) Refund(*gin.Context)      {}
func (billing *codexChatBillingFixture) NeedsRefund() bool        { return len(billing.settled) == 0 }
func (billing *codexChatBillingFixture) GetPreConsumedQuota() int { return 100 }
func (billing *codexChatBillingFixture) Reserve(int) error        { return nil }

func TestRelayChatOverCodexSettlesOnlyAuthoritativeFailureUsage(t *testing.T) {
	previousBatch, previousLogs := common.BatchUpdateEnabled, common.LogConsumeEnabled
	common.BatchUpdateEnabled, common.LogConsumeEnabled = true, false
	t.Cleanup(func() { common.BatchUpdateEnabled, common.LogConsumeEnabled = previousBatch, previousLogs })
	for _, authoritative := range []bool{true, false} {
		t.Run(map[bool]string{true: "authoritative", false: "missing"}[authoritative], func(t *testing.T) {
			body := "data: " + `{"type":"response.output_text.delta","delta":"partial text"}` + "\n\n"
			if authoritative {
				body += `data: {"type":"response.failed","response":{"status":"failed","usage":{"input_tokens":20,"output_tokens":5}}}` + "\n\n"
			}
			ctx, _, info, response := newCodexChatBridgeFixture(true, io.NopCloser(strings.NewReader(body)))
			billing := &codexChatBillingFixture{}
			info.Billing, info.IsPlayground, info.StartTime, info.OriginModelName = billing, true, time.Now(), "gpt-5-codex"
			info.UserQuota, info.UserSetting.QuotaWarningThreshold = 1000, 1
			info.SetEstimatePromptTokens(999)
			info.PriceData = hosttypes.PriceData{ModelRatio: 1, CompletionRatio: 1, CacheRatio: 1, GroupRatioInfo: hosttypes.GroupRatioInfo{GroupRatio: 1}}
			usage, apiErr := RelayChatOverCodex(ctx, info, response)
			require.NotNil(t, apiErr)
			if authoritative {
				assert.Equal(t, []int{25}, billing.settled)
				assert.Equal(t, 25, usage.TotalTokens)
				assert.NotNil(t, usage.BillingUsage)
			} else {
				assert.Empty(t, billing.settled)
				assert.Equal(t, &dto.Usage{}, usage)
			}
		})
	}
}

func TestRelayChatOverCodexPreludeDoesNotCommitFailedAttempt(t *testing.T) {
	for _, stream := range []bool{true, false} {
		t.Run(map[bool]string{true: "stream", false: "buffered"}[stream], func(t *testing.T) {
			body := "data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_fixture\"}}\n\n" +
				"data: {\"type\":\"response.failed\",\"response\":{\"status\":\"failed\",\"error\":{\"message\":\"secret token at https://private.example\"}}}\n\n"
			ctx, recorder, info, response := newCodexChatBridgeFixture(stream, io.NopCloser(strings.NewReader(body)))
			usage, apiErr := RelayChatOverCodex(ctx, info, response)
			require.NotNil(t, apiErr)
			require.NotNil(t, usage)
			assert.Zero(t, usage.TotalTokens)
			assert.False(t, ctx.Writer.Written())
			assert.Empty(t, recorder.Body.String())
			assert.Empty(t, recorder.Header().Get("Content-Type"))
			assert.Equal(t, "unchanged", recorder.Header().Get("X-Fixture"))
			assert.NotContains(t, apiErr.Error(), "secret")
			assert.False(t, types.IsSkipRetryError(apiErr))
			assert.True(t, service.ShouldRetryRelayError(ctx, apiErr, 1))
		})
	}
}
