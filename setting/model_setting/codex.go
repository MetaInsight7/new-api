package model_setting

import "github.com/QuantumNous/new-api/setting/config"

type CodexSettings struct {
	ImageCarrierModel string `json:"image_carrier_model"`
}

var defaultCodexSettings = CodexSettings{}

var codexSettings = defaultCodexSettings

func init() {
	config.GlobalConfig.Register("codex", &codexSettings)
}

func GetCodexSettings() *CodexSettings {
	return &codexSettings
}
