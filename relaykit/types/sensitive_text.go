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
)

const SensitiveTextContextRound = -1

// SensitiveTextField is one ordered piece of model-visible request text.
// Round starts at zero for the first user input; system and developer context
// use SensitiveTextContextRound and tool activity stays in its user round.
type SensitiveTextField struct {
	Role  SensitiveTextRole `json:"role"`
	Kind  SensitiveTextKind `json:"kind"`
	Round int               `json:"round"`
	Order int               `json:"order"`
	Text  string            `json:"text"`
}
