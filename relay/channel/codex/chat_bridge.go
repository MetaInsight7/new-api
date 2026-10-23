package codex

import (
	"context"
	"errors"
	"math"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/relayconvert"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

const codexChatEventLimit = 1 << 20

type codexChatBridge struct {
	context       *gin.Context
	info          *relaycommon.RelayInfo
	stream        bool
	state         *relayconvert.ResponseStreamState
	accumulator   *relayconvert.ResponsesBufferedAccumulator
	usage         *dto.Usage
	authoritative bool
	terminal      *dto.OpenAIResponsesResponse
	pending       [][]byte
	pendingBytes  int
}

func RelayChatOverCodex(ctx *gin.Context, info *relaycommon.RelayInfo, response *http.Response) (*dto.Usage, *types.NewAPIError) {
	if ctx == nil || ctx.Request == nil || info == nil || info.ChannelMeta == nil || response == nil || response.Body == nil {
		return &dto.Usage{}, types.NewOpenAIError(errors.New("Invalid Codex Chat response"), types.ErrorCodeBadResponse, http.StatusBadGateway)
	}
	closeBody := sync.OnceFunc(func() { service.CloseResponseBodyGracefully(response) })
	defer closeBody()
	if response.StatusCode != http.StatusOK {
		return &dto.Usage{}, types.NewOpenAIError(errors.New("Codex upstream request failed"), types.ErrorCodeBadResponse, response.StatusCode)
	}
	bridge := &codexChatBridge{
		context: ctx, info: info, stream: codexClientWantsStream(ctx, info), usage: &dto.Usage{},
		accumulator: relayconvert.NewResponsesBufferedAccumulator(),
	}
	info.StreamStatus = relaycommon.NewStreamStatus()
	info.StreamStatus.RequireTerminal()
	info.SendResponseCount, info.ReceivedResponseCount = 0, 0
	common.SetContextKey(ctx, constant.ContextKeyResponseStreamStatus, info.StreamStatus)
	if bridge.stream {
		state, err := relayconvert.NewResponseStreamState(types.RelayFormatOpenAIResponses, types.RelayFormatOpenAI, relayconvert.ResponseStreamOptions{
			ID: helper.GetResponseID(ctx), Model: info.UpstreamModelName, Created: time.Now().Unix(), IncludeUsage: info.ShouldIncludeUsage,
		})
		if err != nil {
			return bridge.fail("Failed to convert Codex Chat response", http.StatusBadGateway)
		}
		bridge.state = state
	}
	requestContext := ctx.Request.Context()
	stopCancellation := context.AfterFunc(requestContext, closeBody)
	defer stopCancellation()
	var timedOut atomic.Bool
	idleTimeout := time.Duration(max(constant.StreamingTimeout, 1)) * time.Second
	if constant.StreamingTimeout <= 0 {
		idleTimeout = 300 * time.Second
	}
	timer := time.AfterFunc(idleTimeout, func() {
		timedOut.Store(true)
		closeBody()
	})
	defer timer.Stop()
	scanner := helper.NewStreamScanner(response.Body, codexChatEventLimit)
	var dataLines []string
	var dataBytes int
	var eventType string
	terminalSeen := false
	for scanner.Scan() {
		timer.Reset(idleTimeout)
		if requestContext.Err() != nil || timedOut.Load() {
			break
		}
		line := scanner.Text()
		if line == "" {
			if len(dataLines) > 0 {
				terminal, apiErr := bridge.consume(strings.Join(dataLines, "\n"), eventType)
				if apiErr != nil {
					return bridge.failWithAPIError(apiErr)
				}
				if terminal {
					terminalSeen = true
					break
				}
			}
			dataLines, dataBytes, eventType = nil, 0, ""
			continue
		}
		if data, ok := strings.CutPrefix(line, "data:"); ok {
			data = strings.TrimPrefix(data, " ")
			dataBytes += len(data) + 1
			if dataBytes > codexChatEventLimit {
				return bridge.fail("Codex Chat event exceeds the size limit", http.StatusBadGateway)
			}
			dataLines = append(dataLines, data)
		} else if name, ok := strings.CutPrefix(line, "event:"); ok {
			eventType = strings.TrimSpace(name)
		}
	}
	if requestContext.Err() != nil {
		info.StreamStatus.SetEndReason(relaycommon.StreamEndReasonClientGone, requestContext.Err())
		return bridge.fail("Codex Chat request cancelled", http.StatusBadGateway)
	}
	if timedOut.Load() {
		info.StreamStatus.SetEndReason(relaycommon.StreamEndReasonTimeout, context.DeadlineExceeded)
		return bridge.fail("Codex Chat stream timed out", http.StatusGatewayTimeout)
	}
	if scanErr := scanner.Err(); scanErr != nil {
		info.StreamStatus.SetEndReason(relaycommon.StreamEndReasonScannerErr, errors.New("Codex Chat stream read failed"))
		return bridge.fail("Codex Chat stream read failed", http.StatusBadGateway)
	}
	if !terminalSeen && len(dataLines) > 0 {
		terminal, apiErr := bridge.consume(strings.Join(dataLines, "\n"), eventType)
		if apiErr != nil {
			return bridge.failWithAPIError(apiErr)
		}
		terminalSeen = terminal
	}
	if !terminalSeen {
		info.StreamStatus.SetEndReason(relaycommon.StreamEndReasonEOF, nil)
		return bridge.fail("Codex Chat stream ended before a terminal event", http.StatusBadGateway)
	}
	info.StreamStatus.SetEndReason(relaycommon.StreamEndReasonDone, nil)
	if bridge.stream {
		if err := bridge.flushPrelude(); err != nil {
			return bridge.fail("Failed to write Codex Chat response", http.StatusBadGateway)
		}
		if err := bridge.write([]byte("[DONE]")); err != nil {
			return bridge.fail("Failed to write Codex Chat response", http.StatusBadGateway)
		}
		return bridge.usage, nil
	}
	bridge.accumulator.SupplementResponseOutput(bridge.terminal)
	bridge.terminal.ID = helper.GetResponseID(ctx)
	if bridge.terminal.Model == "" {
		bridge.terminal.Model = info.UpstreamModelName
	}
	if bridge.terminal.CreatedAt == 0 {
		bridge.terminal.CreatedAt = dto.IntValue(time.Now().Unix())
	}
	converted, err := service.ConvertResponse(ctx, info, types.RelayFormatOpenAI, bridge.terminal)
	if err != nil {
		return bridge.fail("Failed to convert Codex Chat response", http.StatusBadGateway)
	}
	chatResponse, ok := converted.Value.(*dto.OpenAITextResponse)
	if !ok {
		return bridge.fail("Invalid converted Codex Chat response", http.StatusBadGateway)
	}
	var responseText strings.Builder
	for _, choice := range chatResponse.Choices {
		responseText.WriteString(choice.Message.StringContent())
		responseText.WriteString(choice.Message.GetReasoningContent())
		for _, tool := range choice.Message.ParseToolCalls() {
			responseText.WriteString(tool.Function.Arguments)
			responseText.WriteString(tool.Function.Name)
		}
	}
	bridge.estimateCompletedUsage(responseText.String())
	chatResponse.Usage = *bridge.usage
	ctx.JSON(http.StatusOK, chatResponse)
	return bridge.usage, nil
}

func (bridge *codexChatBridge) consume(payload, eventType string) (bool, *types.NewAPIError) {
	var event dto.ResponsesStreamResponse
	if strings.TrimSpace(payload) == "[DONE]" || common.UnmarshalJsonStr(payload, &event) != nil {
		return false, types.NewOpenAIError(errors.New("Invalid Codex Chat stream event"), types.ErrorCodeBadResponse, http.StatusBadGateway)
	}
	if event.Type == "" {
		event.Type = eventType
	}
	if event.Type == "" {
		return false, types.NewOpenAIError(errors.New("Invalid Codex Chat stream event"), types.ErrorCodeBadResponse, http.StatusBadGateway)
	}
	bridge.info.ReceivedResponseCount++
	service.ObserveResponsesOutcome(bridge.info, &event)
	var responseStatus string
	if event.Response != nil {
		bridge.info.ObserveResponseModel(event.Response.Model)
		_ = common.Unmarshal(event.Response.Status, &responseStatus)
	}
	terminal := false
	switch event.Type {
	case "response.completed", "response.done", "response.incomplete", "response.failed", "response.error", "error", "response.cancelled", "response.canceled":
		terminal = true
	}
	failed := event.Type == "error" || event.Type == "response.error" || event.Type == "response.failed" || responseStatus == "failed" ||
		event.Type == "response.cancelled" || event.Type == "response.canceled" || responseStatus == "cancelled" || responseStatus == "canceled"
	if (terminal || failed) && event.Response != nil && event.Response.Usage != nil {
		upstreamUsage := event.Response.Usage
		rawUsage := &dto.Usage{InputTokens: upstreamUsage.InputTokens, OutputTokens: upstreamUsage.OutputTokens, TotalTokens: upstreamUsage.TotalTokens}
		if upstreamUsage.InputTokensDetails != nil {
			inputDetails := upstreamUsage.InputTokensDetails.Clone()
			rawUsage.InputTokensDetails = &inputDetails
		}
		if upstreamUsage.OutputTokensDetails != nil {
			outputDetails := *upstreamUsage.OutputTokensDetails
			rawUsage.OutputTokensDetails = &outputDetails
		}
		if !codexChatUsageIsValid(rawUsage) {
			return false, types.NewOpenAIError(errors.New("Invalid Codex Chat usage"), types.ErrorCodeBadResponse, http.StatusBadGateway)
		}
		rawUsage.TotalTokens = rawUsage.InputTokens + rawUsage.OutputTokens
		if rawUsage.OutputTokensDetails != nil {
			rawUsage.CompletionTokenDetails = *rawUsage.OutputTokensDetails
		}
		event.Response.Usage = rawUsage
		bridge.usage = relayconvert.UsageFromResponsesUsage(rawUsage)
		service.ApplyResponsesUsage(bridge.usage, rawUsage)
		bridge.authoritative = true
	}
	if failed {
		errorResponse := event.Response
		if errorResponse == nil || errorResponse.GetOpenAIError() == nil {
			var envelope struct {
				Error *types.OpenAIError `json:"error"`
			}
			if err := common.UnmarshalJsonStr(payload, &envelope); err != nil {
				return false, types.NewOpenAIError(errors.New("Invalid Codex Chat failure event"), types.ErrorCodeBadResponse, http.StatusBadGateway)
			}
			if envelope.Error == nil && event.Code != "" {
				envelope.Error = &types.OpenAIError{Code: event.Code, Type: "upstream_error"}
			}
			if envelope.Error != nil {
				errorResponse = &dto.OpenAIResponsesResponse{Error: envelope.Error}
			}
		}
		return false, openai.NewCodexResponsesTerminalError(event.Type, errorResponse, false)
	}
	if terminal {
		if event.Response == nil {
			return false, types.NewOpenAIError(errors.New("Invalid Codex Chat terminal event"), types.ErrorCodeBadResponse, http.StatusBadGateway)
		}
		bridge.terminal = event.Response
		if event.Type == "response.incomplete" {
			bridge.terminal.Status = []byte(`"incomplete"`)
		} else if responseStatus == "" {
			bridge.terminal.Status = []byte(`"completed"`)
		}
	}
	if !bridge.stream {
		bridge.accumulator.ProcessEvent(&event)
		return terminal, nil
	}
	results, err := service.ConvertStreamResponseChunk(bridge.context, bridge.info, bridge.state, &event)
	if err != nil {
		return false, types.NewOpenAIError(errors.New("Failed to convert Codex Chat stream event"), types.ErrorCodeBadResponse, http.StatusBadGateway)
	}
	if terminal {
		bridge.estimateCompletedUsage(bridge.state.UsageText())
		bridge.state.SetUsage(bridge.usage)
	}
	for _, result := range results {
		var chunk *dto.ChatCompletionsStreamResponse
		switch value := result.Value.(type) {
		case dto.ChatCompletionsStreamResponse:
			chunk = &value
		case *dto.ChatCompletionsStreamResponse:
			chunk = value
		}
		if chunk == nil {
			return false, types.NewOpenAIError(errors.New("Invalid converted Codex Chat chunk"), types.ErrorCodeBadResponse, http.StatusBadGateway)
		}
		if terminal && chunk.Usage != nil {
			chunk.Usage = bridge.usage
		}
		if err := bridge.sendChunk(chunk, terminal); err != nil {
			return false, types.NewOpenAIError(errors.New("Failed to write Codex Chat response"), types.ErrorCodeBadResponse, http.StatusBadGateway)
		}
	}
	return terminal, nil
}

func (bridge *codexChatBridge) estimateCompletedUsage(responseText string) {
	if bridge.authoritative || bridge.terminal == nil || responseText == "" {
		return
	}
	var status string
	if common.Unmarshal(bridge.terminal.Status, &status) != nil || status != "completed" {
		return
	}
	bridge.usage = service.ResponseText2Usage(bridge.context, responseText, bridge.info.UpstreamModelName, bridge.info.GetEstimatePromptTokens())
}

func codexChatUsageIsValid(usage *dto.Usage) bool {
	if usage.InputTokens < 0 || usage.OutputTokens < 0 || usage.OutputTokens > math.MaxInt32 ||
		usage.InputTokens > math.MaxInt32-usage.OutputTokens || usage.TotalTokens < 0 || usage.TotalTokens > math.MaxInt32 {
		return false
	}
	if usage.TotalTokens != 0 && usage.TotalTokens != usage.InputTokens+usage.OutputTokens {
		return false
	}
	if details := usage.InputTokensDetails; details != nil {
		for _, tokens := range []int{details.CachedTokens, details.CachedCreationTokens, details.CacheWriteTokens, details.TextTokens, details.AudioTokens, details.ImageTokens} {
			if tokens < 0 || tokens > usage.InputTokens {
				return false
			}
		}
		if cached := details.CachedTokensDetails; cached != nil {
			for _, tokens := range []*int{cached.TextTokens, cached.AudioTokens, cached.ImageTokens} {
				if tokens != nil && (*tokens < 0 || *tokens > details.CachedTokens) {
					return false
				}
			}
		}
	}
	if details := usage.OutputTokensDetails; details != nil {
		for _, tokens := range []int{details.TextTokens, details.AudioTokens, details.ImageTokens, details.ReasoningTokens} {
			if tokens < 0 || tokens > usage.OutputTokens {
				return false
			}
		}
	}
	return true
}

func (bridge *codexChatBridge) sendChunk(chunk *dto.ChatCompletionsStreamResponse, terminal bool) error {
	meaningful := false
	for _, choice := range chunk.Choices {
		if choice.Delta.GetContentString() != "" || choice.Delta.GetReasoningContent() != "" ||
			(len(choice.Delta.Annotations) > 0 && string(choice.Delta.Annotations) != "null" && string(choice.Delta.Annotations) != "[]") {
			meaningful = true
		}
		for _, tool := range choice.Delta.ToolCalls {
			if tool.ID != "" || tool.Function.Name != "" || tool.Function.Arguments != "" {
				meaningful = true
			}
		}
	}
	payload, err := common.Marshal(chunk)
	if err != nil {
		return err
	}
	if !bridge.context.Writer.Written() && !meaningful && !terminal {
		bridge.pendingBytes += len(payload)
		if bridge.pendingBytes > codexChatEventLimit {
			return errors.New("Codex Chat prelude exceeds the size limit")
		}
		bridge.pending = append(bridge.pending, payload)
		return nil
	}
	if meaningful {
		bridge.info.SetFirstResponseTime()
	}
	if err := bridge.flushPrelude(); err != nil {
		return err
	}
	return bridge.write(payload)
}

func (bridge *codexChatBridge) flushPrelude() error {
	for _, payload := range bridge.pending {
		if err := bridge.write(payload); err != nil {
			return err
		}
	}
	bridge.pending, bridge.pendingBytes = nil, 0
	return nil
}

func (bridge *codexChatBridge) write(payload []byte) error {
	if err := bridge.context.Request.Context().Err(); err != nil {
		return err
	}
	helper.SetEventStreamHeaders(bridge.context)
	if _, err := bridge.context.Writer.WriteString("data: " + string(payload) + "\n\n"); err != nil {
		return err
	}
	bridge.info.SendResponseCount++
	return helper.FlushWriter(bridge.context)
}

func (bridge *codexChatBridge) fail(message string, status int) (*dto.Usage, *types.NewAPIError) {
	return bridge.failWithAPIError(types.NewOpenAIError(errors.New(message), types.ErrorCodeBadResponse, status))
}

func (bridge *codexChatBridge) failWithAPIError(apiErr *types.NewAPIError) (*dto.Usage, *types.NewAPIError) {
	if bridge.context.Writer.Written() || bridge.usage.TotalTokens > 0 || bridge.context.Request.Context().Err() != nil {
		types.ErrOptionWithSkipRetry()(apiErr)
	}
	openAIError := apiErr.ToOpenAIError()
	bridge.info.StreamStatus.MarkFailed(common.Interface2String(openAIError.Code), openAIError.Type, apiErr.StatusCode)
	bridge.info.StreamStatus.RecordError(apiErr.Error())
	bridge.pending = nil
	if bridge.stream && bridge.context.Writer.Written() {
		if requestID := bridge.context.GetString(common.RequestIdKey); requestID != "" {
			openAIError.Message = common.MessageWithRequestId(openAIError.Message, requestID)
		}
		if payload, err := common.Marshal(gin.H{"error": openAIError}); err == nil {
			_ = bridge.write(payload)
		}
	}
	if bridge.authoritative && bridge.usage.TotalTokens > 0 && bridge.info.Billing != nil {
		if err := service.PostTextConsumeQuotaOnError(bridge.context, bridge.info, bridge.usage, []*common.Message{common.NewMessage("Codex Chat failure reported authoritative usage")}); err != nil {
			logger.LogError(bridge.context, common.LogText("failed to settle Codex Chat authoritative usage: %s", err.Error()))
		}
	}
	return bridge.usage, apiErr
}
