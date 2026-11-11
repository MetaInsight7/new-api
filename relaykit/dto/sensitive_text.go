package dto

import (
	"encoding/json"
	"sort"
	"strings"

	kitutil "github.com/QuantumNous/new-api/relaykit/relayconvert/kitutil"
	"github.com/QuantumNous/new-api/relaykit/types"
)

type SensitiveTextProvider interface {
	GetSensitiveTextFields() []types.SensitiveTextField
}

func ExtractSensitiveText(request Request) []types.SensitiveTextField {
	provider, ok := request.(SensitiveTextProvider)
	if !ok || provider == nil {
		return nil
	}
	return provider.GetSensitiveTextFields()
}

func SensitivePromptFields(prompt string) []types.SensitiveTextField {
	collector := newSensitiveTextCollector()
	collector.append(
		types.SensitiveTextRoleUser,
		types.SensitiveTextKindPrompt,
		collector.nextUserRound(),
		prompt,
	)
	return collector.fields
}

type sensitiveTextCollector struct {
	fields               []types.SensitiveTextField
	round                int
	userRoundSet         bool
	conversation         int
	conversationSequence *int
}

func newSensitiveTextCollector() *sensitiveTextCollector {
	sequence := 0
	return &sensitiveTextCollector{round: 0, conversationSequence: &sequence}
}

func (collector *sensitiveTextCollector) newConversation() *sensitiveTextCollector {
	(*collector.conversationSequence)++
	return &sensitiveTextCollector{
		round:                0,
		conversation:         *collector.conversationSequence,
		conversationSequence: collector.conversationSequence,
	}
}

func (collector *sensitiveTextCollector) appendConversation(fields []types.SensitiveTextField) {
	for _, field := range fields {
		field.Order = len(collector.fields)
		collector.fields = append(collector.fields, field)
	}
}

func (collector *sensitiveTextCollector) nextUserRound() int {
	if collector.userRoundSet {
		collector.round++
	} else {
		collector.userRoundSet = true
	}
	return collector.round
}

func (collector *sensitiveTextCollector) startUserRound() int {
	round := collector.nextUserRound()
	collector.fields = append(collector.fields, types.SensitiveTextField{
		Role: types.SensitiveTextRoleUser, Kind: types.SensitiveTextKindTurnBoundary,
		Conversation: collector.conversation, Round: round, Order: len(collector.fields),
	})
	return round
}

func (collector *sensitiveTextCollector) currentRound() int {
	return collector.round
}

func (collector *sensitiveTextCollector) append(role types.SensitiveTextRole, kind types.SensitiveTextKind, round int, text string) {
	if strings.TrimSpace(text) == "" {
		return
	}
	collector.fields = append(collector.fields, types.SensitiveTextField{
		Role: role, Kind: kind, Conversation: collector.conversation,
		Round: round, Order: len(collector.fields), Text: text,
	})
}

func sensitiveRole(role string) types.SensitiveTextRole {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "system":
		return types.SensitiveTextRoleSystem
	case "developer":
		return types.SensitiveTextRoleDeveloper
	case "assistant", "model":
		return types.SensitiveTextRoleAssistant
	case "tool", "function":
		return types.SensitiveTextRoleTool
	default:
		return types.SensitiveTextRoleUser
	}
}

func sensitiveRoundForRole(collector *sensitiveTextCollector, role types.SensitiveTextRole, startsUserRound bool) int {
	if role == types.SensitiveTextRoleSystem || role == types.SensitiveTextRoleDeveloper {
		return types.SensitiveTextContextRound
	}
	if role == types.SensitiveTextRoleUser && startsUserRound {
		return collector.startUserRound()
	}
	return collector.currentRound()
}

func decodeSensitiveJSON(raw []byte) (any, bool) {
	var value any
	if err := kitutil.Unmarshal(raw, &value); err != nil {
		return nil, false
	}
	return value, true
}

func sortedSensitiveKeys(value map[string]any) []string {
	keys := make([]string, 0, len(value))
	for key := range value {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func appendSensitiveTextLeaves(collector *sensitiveTextCollector, role types.SensitiveTextRole, kind types.SensitiveTextKind, round int, value any) {
	switch value := value.(type) {
	case string:
		collector.append(role, kind, round, value)
	case []any:
		for _, item := range value {
			appendSensitiveTextLeaves(collector, role, kind, round, item)
		}
	case map[string]any:
		for _, key := range sortedSensitiveKeys(value) {
			normalized := strings.ToLower(strings.TrimSpace(key))
			switch normalized {
			case "id", "call_id", "tool_call_id", "metadata", "client_metadata":
				continue
			}
			appendSensitiveTextLeaves(collector, role, kind, round, value[key])
		}
	}
}

func appendSensitiveArgument(collector *sensitiveTextCollector, role types.SensitiveTextRole, kind types.SensitiveTextKind, round int, value string) {
	decoded, ok := decodeSensitiveJSON([]byte(value))
	if !ok {
		collector.append(role, kind, round, value)
		return
	}
	appendSensitiveTextLeaves(collector, role, kind, round, decoded)
}

func appendSensitiveRaw(collector *sensitiveTextCollector, role types.SensitiveTextRole, kind types.SensitiveTextKind, round int, raw json.RawMessage) {
	value, ok := decodeSensitiveJSON(raw)
	if !ok {
		return
	}
	appendSensitiveTextLeaves(collector, role, kind, round, value)
}

func appendSensitiveSchemaStrings(collector *sensitiveTextCollector, role types.SensitiveTextRole, kind types.SensitiveTextKind, round int, value any, parentKey string) {
	switch value := value.(type) {
	case string:
		switch strings.ToLower(parentKey) {
		case "type", "format", "name", "$schema", "$id", "$ref", "required":
			return
		}
		collector.append(role, kind, round, value)
	case []any:
		if strings.EqualFold(parentKey, "required") {
			return
		}
		for _, item := range value {
			appendSensitiveSchemaStrings(collector, role, kind, round, item, parentKey)
		}
	case map[string]any:
		for _, key := range sortedSensitiveKeys(value) {
			appendSensitiveSchemaStrings(collector, role, kind, round, value[key], key)
		}
	}
}

func appendSensitiveRawSchema(collector *sensitiveTextCollector, raw json.RawMessage) {
	value, ok := decodeSensitiveJSON(raw)
	if !ok {
		return
	}
	appendSensitiveSchemaStrings(
		collector,
		types.SensitiveTextRoleSystem,
		types.SensitiveTextKindResponseSchema,
		types.SensitiveTextContextRound,
		value,
		"schema",
	)
}

func appendSensitiveNestedSchema(collector *sensitiveTextCollector, raw json.RawMessage, keys ...string) {
	value, ok := decodeSensitiveJSON(raw)
	if !ok {
		return
	}
	object, ok := value.(map[string]any)
	if !ok {
		return
	}
	for _, key := range keys {
		if schema, exists := object[key]; exists {
			appendSensitiveSchemaStrings(
				collector,
				types.SensitiveTextRoleSystem,
				types.SensitiveTextKindResponseSchema,
				types.SensitiveTextContextRound,
				schema,
				key,
			)
		}
	}
}

func appendSensitiveToolDefinitions(collector *sensitiveTextCollector, raw any) {
	if bytesValue, ok := raw.(json.RawMessage); ok {
		decoded, valid := decodeSensitiveJSON(bytesValue)
		if !valid {
			return
		}
		raw = decoded
	}
	var walk func(any)
	walk = func(value any) {
		switch value := value.(type) {
		case []any:
			for _, item := range value {
				walk(item)
			}
		case map[string]any:
			for _, key := range sortedSensitiveKeys(value) {
				item := value[key]
				switch strings.ToLower(key) {
				case "description":
					if text, ok := item.(string); ok {
						collector.append(types.SensitiveTextRoleSystem, types.SensitiveTextKindToolDescription, types.SensitiveTextContextRound, text)
					}
				case "parameters", "input_schema", "inputschema":
					appendSensitiveSchemaStrings(collector, types.SensitiveTextRoleSystem, types.SensitiveTextKindToolDescription, types.SensitiveTextContextRound, item, key)
				case "function", "functions", "function_declarations", "functiondeclarations", "tools":
					walk(item)
				}
			}
		}
	}
	walk(raw)
}

func appendSensitiveAnyStrings(collector *sensitiveTextCollector, role types.SensitiveTextRole, kind types.SensitiveTextKind, round int, value any) {
	switch value := value.(type) {
	case string:
		collector.append(role, kind, round, value)
	case []string:
		for _, item := range value {
			collector.append(role, kind, round, item)
		}
	case []any:
		for _, item := range value {
			if text, ok := item.(string); ok {
				collector.append(role, kind, round, text)
			}
		}
	}
}

func (request *GeneralOpenAIRequest) GetSensitiveTextFields() []types.SensitiveTextField {
	collector := newSensitiveTextCollector()
	appendSensitiveToolDefinitions(collector, request.Functions)
	if request.ResponseFormat != nil {
		appendSensitiveRawSchema(collector, request.ResponseFormat.JsonSchema)
	}
	for _, tool := range request.Tools {
		if tool.Function.Description != "" {
			collector.append(types.SensitiveTextRoleSystem, types.SensitiveTextKindToolDescription, types.SensitiveTextContextRound, tool.Function.Description)
		}
		if value, ok := decodeSensitiveJSON(tool.Function.Parameters); ok {
			appendSensitiveSchemaStrings(collector, types.SensitiveTextRoleSystem, types.SensitiveTextKindToolDescription, types.SensitiveTextContextRound, value, "parameters")
		}
		appendSensitiveToolDefinitions(collector, tool.Custom)
	}
	if request.Prompt != nil {
		round := collector.nextUserRound()
		appendSensitiveAnyStrings(collector, types.SensitiveTextRoleUser, types.SensitiveTextKindPrompt, round, request.Prompt)
	}
	if request.Input != nil {
		round := collector.nextUserRound()
		appendSensitiveAnyStrings(collector, types.SensitiveTextRoleUser, types.SensitiveTextKindInput, round, request.Input)
	}
	if request.Instruction != "" {
		collector.append(types.SensitiveTextRoleUser, types.SensitiveTextKindInstruction, collector.nextUserRound(), request.Instruction)
	}
	if request.Prefix != nil || request.Suffix != nil {
		round := collector.nextUserRound()
		appendSensitiveAnyStrings(collector, types.SensitiveTextRoleUser, types.SensitiveTextKindInput, round, request.Prefix)
		appendSensitiveAnyStrings(collector, types.SensitiveTextRoleUser, types.SensitiveTextKindInput, round, request.Suffix)
	}
	for _, message := range request.Messages {
		role := sensitiveRole(message.Role)
		content := message.ParseContent()
		round := sensitiveRoundForRole(collector, role, role == types.SensitiveTextRoleUser)
		contentKind := types.SensitiveTextKindContent
		if role == types.SensitiveTextRoleTool {
			contentKind = types.SensitiveTextKindToolResult
		}
		for _, part := range content {
			if part.Type == ContentTypeText {
				collector.append(role, contentKind, round, part.Text)
			}
		}
		if message.ReasoningContent != nil {
			collector.append(role, types.SensitiveTextKindContent, round, *message.ReasoningContent)
		}
		if message.Reasoning != nil {
			collector.append(role, types.SensitiveTextKindContent, round, *message.Reasoning)
		}
		for _, toolCall := range message.ParseToolCalls() {
			if toolCall.Function.Arguments != "" {
				appendSensitiveArgument(collector, types.SensitiveTextRoleAssistant, types.SensitiveTextKindToolArgument, round, toolCall.Function.Arguments)
			}
			appendSensitiveToolCallRaw(collector, round, toolCall.Custom)
		}
		appendSensitiveToolDefinitions(collector, message.Tools)
	}
	return collector.fields
}

func appendSensitiveToolCallRaw(collector *sensitiveTextCollector, round int, raw json.RawMessage) {
	value, ok := decodeSensitiveJSON(raw)
	if !ok {
		return
	}
	object, ok := value.(map[string]any)
	if !ok {
		return
	}
	for _, key := range []string{"arguments", "input"} {
		if argument, exists := object[key]; exists {
			appendSensitiveTextLeaves(collector, types.SensitiveTextRoleAssistant, types.SensitiveTextKindToolArgument, round, argument)
		}
	}
	if output, exists := object["output"]; exists {
		appendSensitiveTextLeaves(collector, types.SensitiveTextRoleTool, types.SensitiveTextKindToolResult, round, output)
	}
}

func (request *OpenAIResponsesRequest) GetSensitiveTextFields() []types.SensitiveTextField {
	collector := newSensitiveTextCollector()
	appendSensitiveResponsesInstructions(collector, request.Instructions)
	appendSensitiveToolDefinitions(collector, request.Tools)
	appendSensitiveNestedSchema(collector, request.Text, "format")
	appendSensitiveResponsesInput(collector, request.Input)
	if len(request.Prompt) > 0 {
		appendSensitiveResponsesInput(collector, request.Prompt)
	}
	return collector.fields
}

func appendSensitiveResponsesInstructions(collector *sensitiveTextCollector, raw json.RawMessage) {
	value, ok := decodeSensitiveJSON(raw)
	if !ok {
		return
	}
	appendSensitiveTextLeaves(collector, types.SensitiveTextRoleDeveloper, types.SensitiveTextKindInstruction, types.SensitiveTextContextRound, value)
}

func appendSensitiveResponsesInput(collector *sensitiveTextCollector, raw json.RawMessage) {
	value, ok := decodeSensitiveJSON(raw)
	if !ok {
		return
	}
	if text, ok := value.(string); ok {
		collector.append(types.SensitiveTextRoleUser, types.SensitiveTextKindInput, collector.nextUserRound(), text)
		return
	}
	items, ok := value.([]any)
	if !ok {
		return
	}
	for _, itemValue := range items {
		item, ok := itemValue.(map[string]any)
		if !ok {
			continue
		}
		itemType, _ := item["type"].(string)
		switch itemType {
		case "message":
			role := sensitiveRole(stringValue(item["role"]))
			parts := sensitiveResponsesTextParts(item["content"])
			round := sensitiveRoundForRole(collector, role, role == types.SensitiveTextRoleUser)
			for _, text := range parts {
				collector.append(role, types.SensitiveTextKindContent, round, text)
			}
		case "input_text", "output_text", "text":
			if text := stringValue(item["text"]); text != "" {
				collector.append(types.SensitiveTextRoleUser, types.SensitiveTextKindInput, collector.nextUserRound(), text)
			}
		case "function_call", "custom_tool_call", "computer_call", "local_shell_call":
			for _, key := range []string{"arguments", "input"} {
				if argument, exists := item[key]; exists {
					if text, ok := argument.(string); ok {
						appendSensitiveArgument(collector, types.SensitiveTextRoleAssistant, types.SensitiveTextKindToolArgument, collector.currentRound(), text)
					} else {
						appendSensitiveTextLeaves(collector, types.SensitiveTextRoleAssistant, types.SensitiveTextKindToolArgument, collector.currentRound(), argument)
					}
				}
			}
		case "function_call_output", "custom_tool_call_output", "computer_call_output", "local_shell_call_output":
			if output, exists := item["output"]; exists {
				appendSensitiveResponsesOutput(collector, collector.currentRound(), output)
			}
		}
	}
}

func sensitiveResponsesTextParts(value any) []string {
	if text, ok := value.(string); ok {
		return []string{text}
	}
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	texts := make([]string, 0, len(items))
	for _, itemValue := range items {
		item, ok := itemValue.(map[string]any)
		if !ok {
			continue
		}
		switch stringValue(item["type"]) {
		case "input_text", "output_text", "text":
			if text := stringValue(item["text"]); text != "" {
				texts = append(texts, text)
			}
		}
	}
	return texts
}

func appendSensitiveResponsesOutput(collector *sensitiveTextCollector, round int, output any) {
	if text, ok := output.(string); ok {
		appendSensitiveArgument(collector, types.SensitiveTextRoleTool, types.SensitiveTextKindToolResult, round, text)
		return
	}
	if items, ok := output.([]any); ok {
		for _, itemValue := range items {
			item, ok := itemValue.(map[string]any)
			if !ok {
				continue
			}
			switch stringValue(item["type"]) {
			case "input_text", "output_text", "text":
				collector.append(types.SensitiveTextRoleTool, types.SensitiveTextKindToolResult, round, stringValue(item["text"]))
			}
		}
		return
	}
	appendSensitiveTextLeaves(collector, types.SensitiveTextRoleTool, types.SensitiveTextKindToolResult, round, output)
}

func stringValue(value any) string {
	text, _ := value.(string)
	return text
}

func (request *OpenAIResponsesCompactionRequest) GetSensitiveTextFields() []types.SensitiveTextField {
	collector := newSensitiveTextCollector()
	appendSensitiveResponsesInstructions(collector, request.Instructions)
	appendSensitiveToolDefinitions(collector, request.Tools)
	appendSensitiveNestedSchema(collector, request.Text, "format")
	appendSensitiveResponsesInput(collector, request.Input)
	return collector.fields
}

func (request *ClaudeRequest) GetSensitiveTextFields() []types.SensitiveTextField {
	collector := newSensitiveTextCollector()
	appendSensitiveClaudeSystem(collector, request.System)
	appendSensitiveToolDefinitions(collector, request.Tools)
	appendSensitiveNestedSchema(collector, request.OutputConfig, "format", "output_format")
	appendSensitiveRawSchema(collector, request.OutputFormat)
	if request.Prompt != "" {
		collector.append(types.SensitiveTextRoleUser, types.SensitiveTextKindPrompt, collector.nextUserRound(), request.Prompt)
	}
	for _, message := range request.Messages {
		role := sensitiveRole(message.Role)
		parts, _ := message.ParseContent()
		startsUserRound := false
		if role == types.SensitiveTextRoleUser {
			startsUserRound = true
			for _, part := range parts {
				if part.Type != "tool_result" && part.Type != "web_search_tool_result" {
					startsUserRound = true
					break
				}
				startsUserRound = false
			}
		}
		round := sensitiveRoundForRole(collector, role, startsUserRound)
		if text, ok := message.Content.(string); ok {
			collector.append(role, types.SensitiveTextKindContent, round, text)
		}
		for _, part := range parts {
			switch part.Type {
			case ContentTypeText:
				collector.append(role, types.SensitiveTextKindContent, round, part.GetText())
			case "document":
				if part.Source != nil && part.Source.Type == "text" {
					appendSensitiveAnyStrings(collector, role, types.SensitiveTextKindDocument, round, part.Source.Data)
				}
			case "tool_use", "server_tool_use":
				appendSensitiveTextLeaves(collector, types.SensitiveTextRoleAssistant, types.SensitiveTextKindToolArgument, round, part.Input)
			case "tool_result", "web_search_tool_result":
				appendSensitiveClaudeToolResult(collector, round, part.Content)
			}
		}
	}
	return collector.fields
}

func appendSensitiveClaudeSystem(collector *sensitiveTextCollector, value any) {
	if text, ok := value.(string); ok {
		collector.append(types.SensitiveTextRoleSystem, types.SensitiveTextKindContent, types.SensitiveTextContextRound, text)
		return
	}
	raw, err := kitutil.Marshal(value)
	if err != nil {
		return
	}
	decoded, ok := decodeSensitiveJSON(raw)
	if !ok {
		return
	}
	if items, ok := decoded.([]any); ok {
		for _, itemValue := range items {
			item, ok := itemValue.(map[string]any)
			if !ok || stringValue(item["type"]) != ContentTypeText {
				continue
			}
			collector.append(types.SensitiveTextRoleSystem, types.SensitiveTextKindContent, types.SensitiveTextContextRound, stringValue(item["text"]))
		}
	}
}

func appendSensitiveClaudeToolResult(collector *sensitiveTextCollector, round int, value any) {
	if text, ok := value.(string); ok {
		collector.append(types.SensitiveTextRoleTool, types.SensitiveTextKindToolResult, round, text)
		return
	}
	raw, err := kitutil.Marshal(value)
	if err != nil {
		return
	}
	decoded, ok := decodeSensitiveJSON(raw)
	if !ok {
		return
	}
	if items, ok := decoded.([]any); ok {
		for _, itemValue := range items {
			item, ok := itemValue.(map[string]any)
			if !ok || stringValue(item["type"]) != ContentTypeText {
				continue
			}
			collector.append(types.SensitiveTextRoleTool, types.SensitiveTextKindToolResult, round, stringValue(item["text"]))
		}
		return
	}
	appendSensitiveTextLeaves(collector, types.SensitiveTextRoleTool, types.SensitiveTextKindToolResult, round, decoded)
}

func (request *GeminiChatRequest) GetSensitiveTextFields() []types.SensitiveTextField {
	collector := newSensitiveTextCollector()
	appendSensitiveGeminiRequest(collector, request)
	return collector.fields
}

func appendSensitiveGeminiRequest(collector *sensitiveTextCollector, request *GeminiChatRequest) {
	if request == nil {
		return
	}
	if request.SystemInstructions != nil {
		for _, part := range request.SystemInstructions.Parts {
			collector.append(types.SensitiveTextRoleSystem, types.SensitiveTextKindContent, types.SensitiveTextContextRound, part.Text)
		}
	}
	appendSensitiveToolDefinitions(collector, request.Tools)
	appendSensitiveSchemaStrings(
		collector,
		types.SensitiveTextRoleSystem,
		types.SensitiveTextKindResponseSchema,
		types.SensitiveTextContextRound,
		request.GenerationConfig.ResponseSchema,
		"responseSchema",
	)
	appendSensitiveRawSchema(collector, request.GenerationConfig.ResponseJsonSchema)
	for _, content := range request.Contents {
		role := sensitiveRole(content.Role)
		startsUserRound := role == types.SensitiveTextRoleUser
		if startsUserRound && len(content.Parts) > 0 {
			startsUserRound = false
			for _, part := range content.Parts {
				if part.FunctionResponse == nil && part.CodeExecutionResult == nil ||
					part.Text != "" || part.InlineData != nil || part.FileData != nil ||
					part.FunctionCall != nil || part.ExecutableCode != nil {
					startsUserRound = true
					break
				}
			}
		}
		round := sensitiveRoundForRole(collector, role, startsUserRound)
		for _, part := range content.Parts {
			collector.append(role, types.SensitiveTextKindContent, round, part.Text)
			if part.FunctionCall != nil {
				appendSensitiveTextLeaves(collector, types.SensitiveTextRoleAssistant, types.SensitiveTextKindToolArgument, round, part.FunctionCall.Arguments)
				for _, argument := range part.FunctionCall.PartialArgs {
					if argument.StringValue != nil {
						collector.append(types.SensitiveTextRoleAssistant, types.SensitiveTextKindToolArgument, round, *argument.StringValue)
					}
				}
			}
			if part.FunctionResponse != nil {
				appendSensitiveTextLeaves(collector, types.SensitiveTextRoleTool, types.SensitiveTextKindToolResult, round, part.FunctionResponse.Response)
				appendSensitiveGeminiResponseParts(collector, round, part.FunctionResponse.Parts)
			}
			if part.ExecutableCode != nil {
				collector.append(types.SensitiveTextRoleAssistant, types.SensitiveTextKindToolArgument, round, part.ExecutableCode.Code)
			}
			if part.CodeExecutionResult != nil {
				collector.append(types.SensitiveTextRoleTool, types.SensitiveTextKindToolResult, round, part.CodeExecutionResult.Output)
			}
		}
	}
	for index := range request.Requests {
		conversation := collector.newConversation()
		appendSensitiveGeminiRequest(conversation, &request.Requests[index])
		collector.appendConversation(conversation.fields)
	}
}

func appendSensitiveGeminiResponseParts(collector *sensitiveTextCollector, round int, raw json.RawMessage) {
	value, ok := decodeSensitiveJSON(raw)
	if !ok {
		return
	}
	items, ok := value.([]any)
	if !ok {
		return
	}
	for _, itemValue := range items {
		item, ok := itemValue.(map[string]any)
		if !ok {
			continue
		}
		collector.append(types.SensitiveTextRoleTool, types.SensitiveTextKindToolResult, round, stringValue(item["text"]))
	}
}

func (request *EmbeddingRequest) GetSensitiveTextFields() []types.SensitiveTextField {
	collector := newSensitiveTextCollector()
	round := collector.nextUserRound()
	for _, text := range request.ParseInput() {
		collector.append(types.SensitiveTextRoleUser, types.SensitiveTextKindInput, round, text)
	}
	if items, ok := request.Input.([]any); ok {
		for _, item := range items {
			object, ok := item.(map[string]any)
			if !ok || stringValue(object["type"]) != ContentTypeText {
				continue
			}
			collector.append(types.SensitiveTextRoleUser, types.SensitiveTextKindInput, round, stringValue(object["text"]))
		}
	}
	return collector.fields
}

func (request *GeminiEmbeddingRequest) GetSensitiveTextFields() []types.SensitiveTextField {
	collector := newSensitiveTextCollector()
	appendSensitiveGeminiEmbedding(collector, request)
	return collector.fields
}

func (request *GeminiBatchEmbeddingRequest) GetSensitiveTextFields() []types.SensitiveTextField {
	collector := newSensitiveTextCollector()
	for _, item := range request.Requests {
		conversation := collector.newConversation()
		appendSensitiveGeminiEmbedding(conversation, item)
		collector.appendConversation(conversation.fields)
	}
	return collector.fields
}

func appendSensitiveGeminiEmbedding(collector *sensitiveTextCollector, request *GeminiEmbeddingRequest) {
	if request == nil {
		return
	}
	round := collector.nextUserRound()
	for _, part := range request.Content.Parts {
		collector.append(types.SensitiveTextRoleUser, types.SensitiveTextKindInput, round, part.Text)
	}
}

func (request *ImageRequest) GetSensitiveTextFields() []types.SensitiveTextField {
	return SensitivePromptFields(request.Prompt)
}

func (request *AudioRequest) GetSensitiveTextFields() []types.SensitiveTextField {
	collector := newSensitiveTextCollector()
	round := collector.nextUserRound()
	collector.append(types.SensitiveTextRoleUser, types.SensitiveTextKindInput, round, request.Input)
	collector.append(types.SensitiveTextRoleUser, types.SensitiveTextKindInstruction, round, request.Instructions)
	appendSensitiveRaw(collector, types.SensitiveTextRoleUser, types.SensitiveTextKindInput, round, request.RefText)
	return collector.fields
}

func (request *RerankRequest) GetSensitiveTextFields() []types.SensitiveTextField {
	collector := newSensitiveTextCollector()
	round := collector.nextUserRound()
	collector.append(types.SensitiveTextRoleUser, types.SensitiveTextKindQuery, round, request.Query)
	for _, document := range request.Documents {
		switch document := document.(type) {
		case string:
			collector.append(types.SensitiveTextRoleUser, types.SensitiveTextKindDocument, round, document)
		case map[string]any:
			appendSensitiveAnyStrings(collector, types.SensitiveTextRoleUser, types.SensitiveTextKindDocument, round, document["text"])
		}
	}
	return collector.fields
}

func (request *AlphaSearchRequest) GetSensitiveTextFields() []types.SensitiveTextField {
	return sensitiveKnownRawPromptFields(request.RawBody)
}

func (request *DeepSeekToolsRequest) GetSensitiveTextFields() []types.SensitiveTextField {
	return sensitiveKnownRawPromptFields(request.RawBody)
}

func sensitiveKnownRawPromptFields(raw json.RawMessage) []types.SensitiveTextField {
	collector := newSensitiveTextCollector()
	value, ok := decodeSensitiveJSON(raw)
	if !ok {
		return nil
	}
	object, ok := value.(map[string]any)
	if !ok {
		return nil
	}
	round := collector.nextUserRound()
	for _, key := range []string{"query", "prompt", "text"} {
		appendSensitiveAnyStrings(collector, types.SensitiveTextRoleUser, types.SensitiveTextKindQuery, round, object[key])
	}
	return collector.fields
}
