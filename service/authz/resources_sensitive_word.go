package authz

const (
	ResourceSensitiveWord = "sensitive_word"

	ActionManage       = "manage"
	ActionEvidenceRead = "evidence_read"
)

var (
	SensitiveWordManage       = Permission{Resource: ResourceSensitiveWord, Action: ActionManage}
	SensitiveWordEvidenceRead = Permission{Resource: ResourceSensitiveWord, Action: ActionEvidenceRead}
)

func init() {
	RegisterResource(ResourceDefinition{
		Resource: ResourceSensitiveWord,
		LabelKey: "Sensitive-word safety",
		Actions: []ActionDefinition{
			{
				Action:         ActionManage,
				LabelKey:       "Manage sensitive-word safety",
				DescriptionKey: "Manage policy, rules, user whitelists, violation counts, and dedicated unban actions.",
				DefaultRoles:   []string{BuiltInRoleAdmin},
			},
			{
				Action:         ActionEvidenceRead,
				LabelKey:       "Read sensitive-word evidence",
				DescriptionKey: "Read matched words, rules, hashes, and retained request context. Root-user evidence remains restricted to Root.",
			},
		},
	})
}
