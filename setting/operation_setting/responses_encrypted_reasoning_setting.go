package operation_setting

import "github.com/QuantumNous/new-api/setting/config"

type ResponsesEncryptedReasoningSetting struct {
	RecoveryEnabled bool `json:"recovery_enabled"`
}

var responsesEncryptedReasoningSetting = ResponsesEncryptedReasoningSetting{
	RecoveryEnabled: true,
}

func init() {
	config.GlobalConfig.Register("responses_encrypted_reasoning_setting", &responsesEncryptedReasoningSetting)
}

func GetResponsesEncryptedReasoningSetting() *ResponsesEncryptedReasoningSetting {
	return &responsesEncryptedReasoningSetting
}
