package dto

import (
	"encoding/json"
	"strings"
	"testing"

	kitutil "github.com/QuantumNous/new-api/relaykit/relayconvert/kitutil"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/stretchr/testify/require"
)

func sensitiveFieldTexts(fields []types.SensitiveTextField) []string {
	texts := make([]string, 0, len(fields))
	for index, field := range fields {
		if field.Order != index {
			panic("sensitive field order is not contiguous")
		}
		texts = append(texts, field.Text)
	}
	return texts
}

func findSensitiveField(t *testing.T, fields []types.SensitiveTextField, text string) types.SensitiveTextField {
	t.Helper()
	for _, field := range fields {
		if field.Text == text {
			return field
		}
	}
	t.Fatalf("sensitive text field %q not found", text)
	return types.SensitiveTextField{}
}

func sensitiveTurnBoundaries(fields []types.SensitiveTextField) []types.SensitiveTextField {
	boundaries := make([]types.SensitiveTextField, 0)
	for _, field := range fields {
		if field.Kind == types.SensitiveTextKindTurnBoundary {
			boundaries = append(boundaries, field)
		}
	}
	return boundaries
}

func TestOpenAISensitiveTextFieldsPreserveRoundsAndExcludeStructure(t *testing.T) {
	raw := []byte(`{
  "model":"gpt-test",
  "metadata":{"note":"ordinary-metadata-marker"},
  "response_format":{"type":"json_schema","json_schema":{"name":"structural-response-name","description":"response-schema-description-marker","schema":{"type":"object","properties":{"answer":{"description":"response-field-description-marker"}}}}},
  "tools":[{"type":"function","function":{"name":"structural-tool-name","description":"tool-description-marker","parameters":{"type":"object","properties":{"query":{"type":"string","description":"parameter-description-marker"}},"required":["structural-required-marker"]}}}],
  "messages":[
    {"role":"system","name":"structural-message-name","content":"system-marker"},
    {"role":"user","content":[{"type":"text","text":"first-user-marker"},{"type":"image_url","image_url":{"url":"data:image/png;base64,media-base64-marker"}}]},
    {"role":"assistant","content":"assistant-marker","tool_calls":[{"id":"structural-call-id-marker","type":"function","function":{"name":"structural-function-name","arguments":"{\"query\":\"tool-argument-marker\",\"id\":\"argument-id-marker\"}"}}]},
    {"role":"tool","tool_call_id":"structural-tool-call-id","content":"tool-result-marker"},
    {"role":"user","content":"second-user-marker"}
  ]
}`)
	var request GeneralOpenAIRequest
	require.NoError(t, kitutil.Unmarshal(raw, &request))
	beforeWire, err := kitutil.Marshal(request)
	require.NoError(t, err)
	beforeTokens := request.GetTokenCountMeta().CombineText

	fields := ExtractSensitiveText(&request)
	joined := strings.Join(sensitiveFieldTexts(fields), "\n")
	for _, marker := range []string{
		"response-schema-description-marker", "response-field-description-marker",
		"tool-description-marker", "parameter-description-marker", "system-marker",
		"first-user-marker", "assistant-marker", "tool-argument-marker",
		"tool-result-marker", "second-user-marker",
	} {
		require.Contains(t, joined, marker)
	}
	for _, marker := range []string{
		"ordinary-metadata-marker", "structural-tool-name", "structural-message-name",
		"structural-response-name", "structural-required-marker", "structural-call-id-marker", "structural-function-name",
		"argument-id-marker", "media-base64-marker",
	} {
		require.NotContains(t, joined, marker)
	}

	require.Equal(t, types.SensitiveTextContextRound, findSensitiveField(t, fields, "system-marker").Round)
	require.Equal(t, 0, findSensitiveField(t, fields, "first-user-marker").Round)
	require.Equal(t, 0, findSensitiveField(t, fields, "assistant-marker").Round)
	require.Equal(t, 0, findSensitiveField(t, fields, "tool-argument-marker").Round)
	toolResult := findSensitiveField(t, fields, "tool-result-marker")
	require.Equal(t, 0, toolResult.Round)
	require.Equal(t, types.SensitiveTextKindToolResult, toolResult.Kind)
	require.Equal(t, 1, findSensitiveField(t, fields, "second-user-marker").Round)

	afterWire, err := kitutil.Marshal(request)
	require.NoError(t, err)
	require.JSONEq(t, string(beforeWire), string(afterWire))
	require.Equal(t, beforeTokens, request.GetTokenCountMeta().CombineText)
}

func TestSensitiveTextFieldsPreserveTextFreeLatestUserTurns(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		request Request
	}{
		{
			name: "openai chat image-only turn",
			raw: `{"messages":[
  {"role":"user","content":"historical-sensitive-marker"},
  {"role":"assistant","content":"assistant-marker"},
  {"role":"user","content":[{"type":"image_url","image_url":{"url":"data:image/png;base64,latest-media-marker"}}]}
]}`,
			request: &GeneralOpenAIRequest{},
		},
		{
			name: "openai responses image-only turn",
			raw: `{"input":[
  {"type":"message","role":"user","content":[{"type":"input_text","text":"historical-sensitive-marker"}]},
  {"type":"message","role":"assistant","content":[{"type":"output_text","text":"assistant-marker"}]},
  {"type":"message","role":"user","content":[{"type":"input_image","image_url":"data:image/png;base64,latest-media-marker"}]}
]}`,
			request: &OpenAIResponsesRequest{},
		},
		{
			name: "claude image-only turn",
			raw: `{"messages":[
  {"role":"user","content":"historical-sensitive-marker"},
  {"role":"assistant","content":"assistant-marker"},
  {"role":"user","content":[{"type":"image","source":{"type":"base64","media_type":"image/png","data":"latest-media-marker"}}]}
]}`,
			request: &ClaudeRequest{},
		},
		{
			name: "gemini inline-data-only turn",
			raw: `{"contents":[
  {"role":"user","parts":[{"text":"historical-sensitive-marker"}]},
  {"role":"model","parts":[{"text":"assistant-marker"}]},
  {"role":"user","parts":[{"inlineData":{"mimeType":"image/png","data":"latest-media-marker"}}]}
]}`,
			request: &GeminiChatRequest{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.NoError(t, kitutil.Unmarshal([]byte(test.raw), test.request))
			fields := ExtractSensitiveText(test.request)
			boundaries := sensitiveTurnBoundaries(fields)
			require.Len(t, boundaries, 2)
			require.Equal(t, 1, boundaries[1].Round)
			require.Empty(t, boundaries[1].Text)
			require.Greater(t, boundaries[1].Order, findSensitiveField(t, fields, "historical-sensitive-marker").Order)
			require.NotContains(t, strings.Join(sensitiveFieldTexts(fields), "\n"), "latest-media-marker")
		})
	}
}

func TestSensitiveToolArgumentWithTrailingTextIsScannedWhole(t *testing.T) {
	collector := newSensitiveTextCollector()
	appendSensitiveArgument(
		collector,
		types.SensitiveTextRoleAssistant,
		types.SensitiveTextKindToolArgument,
		0,
		`{"query":"safe"} trailing-sensitive-marker`,
	)
	require.Len(t, collector.fields, 1)
	require.Equal(t, `{"query":"safe"} trailing-sensitive-marker`, collector.fields[0].Text)
}

func TestResponsesSensitiveTextFieldsUseCurrentRequestOnly(t *testing.T) {
	raw := []byte(`{
  "model":"gpt-test",
  "instructions":"developer-marker",
  "previous_response_id":"remote-history-id-marker",
  "metadata":{"note":"responses-metadata-marker"},
	"text":{"format":{"type":"json_schema","name":"responses-structural-schema-name","description":"responses-schema-description","schema":{"type":"object","properties":{"answer":{"description":"responses-field-description"}}}},"verbosity":"responses-structural-verbosity"},
  "tools":[{"type":"function","name":"structural-tool-name","description":"responses-tool-description","parameters":{"type":"object","properties":{"query":{"description":"responses-parameter-description"}}}}],
  "input":[
    {"type":"message","role":"user","content":[{"type":"input_text","text":"responses-user-one"},{"type":"input_image","image_url":"data:image/png;base64,responses-media-marker"}]},
    {"type":"message","role":"assistant","content":[{"type":"output_text","text":"responses-assistant"}]},
    {"type":"function_call","call_id":"responses-call-id-marker","name":"structural-function-name","arguments":"{\"query\":\"responses-tool-argument\",\"id\":\"responses-argument-id-marker\"}"},
    {"type":"function_call_output","call_id":"responses-call-id-marker","output":"{\"result\":\"responses-tool-result\",\"id\":\"responses-result-id-marker\"}"},
    {"type":"message","role":"assistant","content":{"ordinary":"ordinary-content-object-marker"}},
    {"type":"message","role":"user","content":"responses-user-two"}
  ]
}`)
	var request OpenAIResponsesRequest
	require.NoError(t, kitutil.Unmarshal(raw, &request))
	fields := ExtractSensitiveText(&request)
	joined := strings.Join(sensitiveFieldTexts(fields), "\n")
	for _, marker := range []string{
		"developer-marker", "responses-schema-description", "responses-field-description",
		"responses-tool-description", "responses-parameter-description",
		"responses-user-one", "responses-assistant", "responses-tool-argument",
		"responses-tool-result", "responses-user-two",
	} {
		require.Contains(t, joined, marker)
	}
	for _, marker := range []string{
		"remote-history-id-marker", "responses-metadata-marker", "structural-tool-name",
		"responses-structural-schema-name", "responses-structural-verbosity",
		"responses-media-marker", "responses-call-id-marker", "structural-function-name",
		"responses-argument-id-marker", "responses-result-id-marker", "ordinary-content-object-marker",
	} {
		require.NotContains(t, joined, marker)
	}
	require.Equal(t, types.SensitiveTextContextRound, findSensitiveField(t, fields, "developer-marker").Round)
	require.Equal(t, 0, findSensitiveField(t, fields, "responses-user-one").Round)
	require.Equal(t, 0, findSensitiveField(t, fields, "responses-tool-result").Round)
	require.Equal(t, 1, findSensitiveField(t, fields, "responses-user-two").Round)
}

func TestResponsesCompactionSensitiveTextFieldsIncludeOutputSchema(t *testing.T) {
	request := &OpenAIResponsesCompactionRequest{
		Text: json.RawMessage(`{"format":{"type":"json_schema","name":"structural-schema-name","schema":{"properties":{"answer":{"description":"compact-schema-description"}}}}}`),
	}
	fields := ExtractSensitiveText(request)
	require.Equal(t, types.SensitiveTextKindResponseSchema, findSensitiveField(t, fields, "compact-schema-description").Kind)
	require.NotContains(t, strings.Join(sensitiveFieldTexts(fields), "\n"), "structural-schema-name")
}

func TestClaudeSensitiveTextFieldsKeepToolActivityInUserRound(t *testing.T) {
	raw := []byte(`{
  "model":"claude-test",
  "system":[{"type":"text","text":"claude-system"}],
  "metadata":{"user_id":"claude-metadata-id"},
	"output_config":{"effort":"claude-structural-effort","format":{"type":"json_schema","name":"claude-structural-schema-name","schema":{"type":"object","properties":{"answer":{"description":"claude-output-description"}}}}},
  "tools":[{"name":"structural-tool-name","description":"claude-tool-description","input_schema":{"type":"object","properties":{"query":{"description":"claude-parameter-description"}}}}],
  "messages":[
    {"role":"user","content":[{"type":"text","text":"claude-user-one"},{"type":"image","source":{"type":"base64","media_type":"image/png","data":"claude-media-marker"}}]},
    {"role":"assistant","content":[{"type":"text","text":"claude-assistant"},{"type":"tool_use","id":"claude-tool-id","name":"structural-tool-name","input":{"query":"claude-tool-argument","id":"claude-argument-id"}}]},
    {"role":"user","content":[{"type":"tool_result","tool_use_id":"claude-tool-id","content":[{"type":"text","text":"claude-tool-result"},{"type":"image","source":{"type":"base64","data":"claude-result-media-marker"}}]}]},
    {"role":"user","content":"claude-user-two"}
  ]
}`)
	var request ClaudeRequest
	require.NoError(t, kitutil.Unmarshal(raw, &request))
	fields := ExtractSensitiveText(&request)
	joined := strings.Join(sensitiveFieldTexts(fields), "\n")
	for _, marker := range []string{
		"claude-system", "claude-output-description", "claude-tool-description", "claude-parameter-description",
		"claude-user-one", "claude-assistant", "claude-tool-argument",
		"claude-tool-result", "claude-user-two",
	} {
		require.Contains(t, joined, marker)
	}
	for _, marker := range []string{
		"claude-metadata-id", "structural-tool-name", "claude-media-marker",
		"claude-structural-effort", "claude-structural-schema-name",
		"claude-tool-id", "claude-argument-id", "claude-result-media-marker",
	} {
		require.NotContains(t, joined, marker)
	}
	require.Equal(t, 0, findSensitiveField(t, fields, "claude-tool-result").Round)
	require.Equal(t, 1, findSensitiveField(t, fields, "claude-user-two").Round)
	require.Len(t, sensitiveTurnBoundaries(fields), 2, "a pure tool-result message must stay in the active user round")
}

func TestGeminiSensitiveTextFieldsKeepFunctionResponseInUserRound(t *testing.T) {
	raw := []byte(`{
  "systemInstruction":{"parts":[{"text":"gemini-system"}]},
  "generationConfig":{"responseSchema":{"type":"object","properties":{"answer":{"description":"gemini-output-description"}}}},
  "tools":[{"functionDeclarations":[{"name":"structural-tool-name","description":"gemini-tool-description","parameters":{"type":"object","properties":{"query":{"description":"gemini-parameter-description"}}}}]}],
  "contents":[
    {"role":"user","parts":[{"text":"gemini-user-one"},{"inlineData":{"mimeType":"image/png","data":"gemini-media-marker"}}]},
    {"role":"model","parts":[{"text":"gemini-assistant"},{"functionCall":{"id":"gemini-call-id","name":"structural-function-name","args":{"query":"gemini-tool-argument","id":"gemini-argument-id"}}}]},
    {"role":"user","parts":[{"functionResponse":{"name":"structural-function-name","id":"gemini-response-id","response":{"result":"gemini-tool-result","id":"gemini-result-id"}}}]},
    {"role":"user","parts":[{"text":"gemini-user-two"}]}
  ]
}`)
	var request GeminiChatRequest
	require.NoError(t, kitutil.Unmarshal(raw, &request))
	fields := ExtractSensitiveText(&request)
	joined := strings.Join(sensitiveFieldTexts(fields), "\n")
	for _, marker := range []string{
		"gemini-system", "gemini-output-description", "gemini-tool-description", "gemini-parameter-description",
		"gemini-user-one", "gemini-assistant", "gemini-tool-argument",
		"gemini-tool-result", "gemini-user-two",
	} {
		require.Contains(t, joined, marker)
	}
	for _, marker := range []string{
		"structural-tool-name", "structural-function-name", "gemini-media-marker",
		"gemini-call-id", "gemini-argument-id", "gemini-response-id", "gemini-result-id",
	} {
		require.NotContains(t, joined, marker)
	}
	require.Equal(t, 0, findSensitiveField(t, fields, "gemini-tool-result").Round)
	require.Equal(t, 1, findSensitiveField(t, fields, "gemini-user-two").Round)
	require.Len(t, sensitiveTurnBoundaries(fields), 2, "a pure function response must stay in the active user round")
}

func TestGeminiBatchSensitiveTextFieldsKeepIndependentConversations(t *testing.T) {
	request := &GeminiChatRequest{Requests: []GeminiChatRequest{
		{Contents: []GeminiChatContent{
			{Role: "user", Parts: []GeminiPart{{Text: "batch-one-history"}}},
			{Role: "model", Parts: []GeminiPart{{Text: "batch-one-response"}}},
			{Role: "user", Parts: []GeminiPart{{Text: "batch-one-latest"}}},
		}},
		{Contents: []GeminiChatContent{
			{Role: "user", Parts: []GeminiPart{{Text: "batch-two-latest"}}},
		}},
	}}

	fields := ExtractSensitiveText(request)
	firstHistory := findSensitiveField(t, fields, "batch-one-history")
	firstLatest := findSensitiveField(t, fields, "batch-one-latest")
	secondLatest := findSensitiveField(t, fields, "batch-two-latest")
	require.Equal(t, firstHistory.Conversation, firstLatest.Conversation)
	require.NotEqual(t, firstLatest.Conversation, secondLatest.Conversation)
	require.Equal(t, 0, firstHistory.Round)
	require.Equal(t, 1, firstLatest.Round)
	require.Equal(t, 0, secondLatest.Round)
}

func TestGeminiBatchEmbeddingSensitiveTextFieldsKeepIndependentConversations(t *testing.T) {
	request := &GeminiBatchEmbeddingRequest{Requests: []*GeminiEmbeddingRequest{
		{Content: GeminiChatContent{Parts: []GeminiPart{{Text: "embedding-one"}}}},
		{Content: GeminiChatContent{Parts: []GeminiPart{{Text: "embedding-two"}}}},
	}}

	fields := ExtractSensitiveText(request)
	first := findSensitiveField(t, fields, "embedding-one")
	second := findSensitiveField(t, fields, "embedding-two")
	require.NotEqual(t, first.Conversation, second.Conversation)
	require.Equal(t, 0, first.Round)
	require.Equal(t, 0, second.Round)
}

func TestSensitiveTextFieldsCoverPromptProtocolsWithoutMedia(t *testing.T) {
	tests := []struct {
		name    string
		request Request
		want    []string
		reject  []string
	}{
		{name: "image", request: &ImageRequest{Prompt: "image-prompt"}, want: []string{"image-prompt"}},
		{name: "audio", request: &AudioRequest{Input: "audio-input", Instructions: "audio-instruction", RefAudio: json.RawMessage(`"data:audio/wav;base64,audio-media"`), RefText: json.RawMessage(`"reference-text"`)}, want: []string{"audio-input", "audio-instruction", "reference-text"}, reject: []string{"audio-media"}},
		{name: "embedding", request: &EmbeddingRequest{Input: []any{"embedding-text", map[string]any{"type": "image_url", "image_url": map[string]any{"url": "embedding-media"}}}}, want: []string{"embedding-text"}, reject: []string{"embedding-media"}},
		{name: "rerank", request: &RerankRequest{Query: "rerank-query", Documents: []any{"rerank-document", map[string]any{"text": "rerank-object-document", "id": "rerank-id"}}}, want: []string{"rerank-query", "rerank-document", "rerank-object-document"}, reject: []string{"rerank-id"}},
		{name: "deepseek search", request: &DeepSeekToolsRequest{RawBody: json.RawMessage(`{"query":"deepseek-query","url":"https://example.com/deepseek-url"}`)}, want: []string{"deepseek-query"}, reject: []string{"deepseek-url"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			joined := strings.Join(sensitiveFieldTexts(ExtractSensitiveText(test.request)), "\n")
			for _, marker := range test.want {
				require.Contains(t, joined, marker)
			}
			for _, marker := range test.reject {
				require.NotContains(t, joined, marker)
			}
		})
	}

}
