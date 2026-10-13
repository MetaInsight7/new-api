package service

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
)

type ResponsesEncryptedReasoningRecovery struct {
	enabled   bool
	attempted bool
}

func NewResponsesEncryptedReasoningRecovery(enabled bool) *ResponsesEncryptedReasoningRecovery {
	return &ResponsesEncryptedReasoningRecovery{enabled: enabled}
}

func (r *ResponsesEncryptedReasoningRecovery) Prepare(c *gin.Context, request any, upstreamErr *types.NewAPIError) (bool, error) {
	if r == nil || !r.enabled || r.attempted || c == nil || !IsResponsesEncryptedReasoningError(upstreamErr) {
		return false, nil
	}

	storage, err := common.GetBodyStorage(c)
	if err != nil {
		return false, err
	}
	raw, err := storage.Bytes()
	if err != nil {
		return false, err
	}
	rewritten, input, changed, hasEncryptedReasoning, err := sanitizeResponsesEncryptedReasoningBody(raw)
	if err != nil {
		return false, err
	}
	if !hasEncryptedReasoning || !changed {
		return false, nil
	}

	switch request.(type) {
	case *dto.OpenAIResponsesRequest, *dto.OpenAIResponsesCompactionRequest:
	default:
		return false, fmt.Errorf("unsupported Responses recovery request type %T", request)
	}

	newStorage, err := common.CreateBodyStorage(rewritten)
	if err != nil {
		return false, err
	}

	switch typed := request.(type) {
	case *dto.OpenAIResponsesRequest:
		typed.Input = append(typed.Input[:0], input...)
	case *dto.OpenAIResponsesCompactionRequest:
		typed.Input = append(typed.Input[:0], input...)
	}
	r.attempted = true
	_ = storage.Close()
	c.Set(common.KeyBodyStorage, newStorage)
	c.Set(common.KeyRequestBody, nil)
	if c.Request != nil {
		c.Request.Body = io.NopCloser(newStorage)
		c.Request.ContentLength = int64(len(rewritten))
	}
	RequestPolicy(c).AddEvent(PolicyEvent{
		ChannelID:   c.GetInt("channel_id"),
		Status:      upstreamErr.StatusCode,
		ErrorCode:   string(upstreamErr.GetErrorCode()),
		ErrorSource: "upstream",
		Decision: PolicyDecision{
			Action: "recover",
			Reason: "encrypted_reasoning_removed",
			Source: "responses_encrypted_state",
		},
		Health: "unchanged",
	})
	return true, nil
}

func IsResponsesEncryptedReasoningError(apiErr *types.NewAPIError) bool {
	if apiErr == nil {
		return false
	}
	code := strings.ToLower(strings.TrimSpace(string(apiErr.GetErrorCode())))
	if code == "invalid_encrypted_content" || code == "thinking_signature_invalid" {
		return true
	}
	if apiErr.StatusCode < 400 || apiErr.StatusCode >= 500 {
		return false
	}
	openAIError := apiErr.ToOpenAIError()
	errorType := strings.ToLower(strings.TrimSpace(openAIError.Type))
	if errorType != "" && errorType != "invalid_request_error" {
		return false
	}
	message := strings.ToLower(strings.TrimSpace(openAIError.Message))
	if strings.Contains(message, "encrypted content") &&
		(strings.Contains(message, "could not be verified") ||
			strings.Contains(message, "could not be decrypted") ||
			strings.Contains(message, "could not be parsed")) {
		return true
	}
	return strings.Contains(message, "invalid signature") &&
		(strings.Contains(message, "thinking block") || strings.Contains(message, "reasoning block"))
}

func sanitizeResponsesEncryptedReasoningBody(raw []byte) ([]byte, json.RawMessage, bool, bool, error) {
	var body map[string]json.RawMessage
	if err := common.Unmarshal(raw, &body); err != nil {
		return nil, nil, false, false, err
	}
	input, ok := body["input"]
	if !ok {
		return raw, nil, false, false, nil
	}
	rewrittenInput, changed, hasEncryptedReasoning, err := sanitizeResponsesEncryptedReasoningInput(input)
	if err != nil || !hasEncryptedReasoning || !changed {
		return raw, input, changed, hasEncryptedReasoning, err
	}
	body["input"] = rewrittenInput
	rewritten, err := common.Marshal(body)
	if err != nil {
		return nil, nil, false, false, err
	}
	return rewritten, rewrittenInput, true, true, nil
}

func sanitizeResponsesEncryptedReasoningInput(input json.RawMessage) (json.RawMessage, bool, bool, error) {
	var items []json.RawMessage
	if err := common.Unmarshal(input, &items); err != nil {
		return input, false, false, nil
	}

	rewrittenItems := make([]json.RawMessage, 0, len(items))
	changed := false
	hasEncryptedReasoning := false
	for _, rawItem := range items {
		var item map[string]json.RawMessage
		if err := common.Unmarshal(rawItem, &item); err != nil {
			rewrittenItems = append(rewrittenItems, rawItem)
			continue
		}
		var itemType string
		if err := common.Unmarshal(item["type"], &itemType); err != nil || itemType != "reasoning" {
			rewrittenItems = append(rewrittenItems, rawItem)
			continue
		}
		if encrypted, exists := item["encrypted_content"]; exists && hasJSONContent(encrypted) {
			hasEncryptedReasoning = true
		}
		if _, exists := item["encrypted_content"]; exists {
			delete(item, "encrypted_content")
			changed = true
		}
		if _, exists := item["id"]; exists {
			delete(item, "id")
			changed = true
		}
		if !hasJSONContent(item["summary"]) && !hasJSONContent(item["content"]) {
			changed = true
			continue
		}
		rewrittenItem, err := common.Marshal(item)
		if err != nil {
			return nil, false, false, err
		}
		rewrittenItems = append(rewrittenItems, rewrittenItem)
	}
	if !hasEncryptedReasoning || !changed {
		return input, changed, hasEncryptedReasoning, nil
	}
	rewritten, err := common.Marshal(rewrittenItems)
	if err != nil {
		return nil, false, false, err
	}
	return rewritten, true, true, nil
}

func hasJSONContent(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	var value any
	if err := common.Unmarshal(raw, &value); err != nil {
		return false
	}
	switch typed := value.(type) {
	case nil:
		return false
	case string:
		return strings.TrimSpace(typed) != ""
	case []any:
		return len(typed) > 0
	case map[string]any:
		return len(typed) > 0
	default:
		return true
	}
}
