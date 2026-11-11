package types

type SensitiveTextRole string

const (
	SensitiveTextRoleSystem    SensitiveTextRole = "system"
	SensitiveTextRoleDeveloper SensitiveTextRole = "developer"
	SensitiveTextRoleUser      SensitiveTextRole = "user"
	SensitiveTextRoleAssistant SensitiveTextRole = "assistant"
	SensitiveTextRoleTool      SensitiveTextRole = "tool"
)

type SensitiveTextKind string

const (
	SensitiveTextKindContent         SensitiveTextKind = "content"
	SensitiveTextKindPrompt          SensitiveTextKind = "prompt"
	SensitiveTextKindInstruction     SensitiveTextKind = "instruction"
	SensitiveTextKindInput           SensitiveTextKind = "input"
	SensitiveTextKindQuery           SensitiveTextKind = "query"
	SensitiveTextKindDocument        SensitiveTextKind = "document"
	SensitiveTextKindToolDescription SensitiveTextKind = "tool_description"
	SensitiveTextKindToolArgument    SensitiveTextKind = "tool_argument"
	SensitiveTextKindToolResult      SensitiveTextKind = "tool_result"
	SensitiveTextKindResponseSchema  SensitiveTextKind = "response_schema"
	SensitiveTextKindTurnBoundary    SensitiveTextKind = "turn_boundary"
)

const SensitiveTextContextRound = -1

// SensitiveTextField is one ordered piece of model-visible request text.
// Conversation is zero for ordinary requests and separates independent items
// in a batch. Round starts at zero for each conversation's first input;
// system and developer context use SensitiveTextContextRound and tool results
// stay in the user round that caused them. Turn-boundary fields carry no
// request text; they preserve text-free user turns for latest-round selection
// and are never scanned or retained as audit evidence.
type SensitiveTextField struct {
	Role         SensitiveTextRole `json:"role"`
	Kind         SensitiveTextKind `json:"kind"`
	Conversation int               `json:"conversation,omitempty"`
	Round        int               `json:"round"`
	Order        int               `json:"order"`
	Text         string            `json:"text"`
}
