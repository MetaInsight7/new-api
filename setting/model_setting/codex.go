package model_setting

import (
	"sync"

	"github.com/QuantumNous/new-api/setting/config"
)

type CodexSettings struct {
	ImageCarrierModel string `json:"image_carrier_model"`
}

var defaultCodexSettings = CodexSettings{}

var codexSettings = defaultCodexSettings
var codexSettingsMu sync.RWMutex

func init() {
	config.GlobalConfig.Register("codex", &codexSettings)
}

func (settings *CodexSettings) LockConfig() {
	codexSettingsMu.Lock()
}

func (settings *CodexSettings) UnlockConfig() {
	codexSettingsMu.Unlock()
}

func (settings *CodexSettings) RLockConfig() {
	codexSettingsMu.RLock()
}

func (settings *CodexSettings) RUnlockConfig() {
	codexSettingsMu.RUnlock()
}

func GetCodexSettings() CodexSettings {
	codexSettingsMu.RLock()
	defer codexSettingsMu.RUnlock()
	return codexSettings
}
