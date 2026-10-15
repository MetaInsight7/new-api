package operation_setting

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/setting/config"
)

const MaxFirstResponseTimeoutSeconds = int64(math.MaxInt64) / int64(time.Second)

type StreamingSetting struct {
	FirstResponseTimeoutEnabled bool  `json:"first_response_timeout_enabled"`
	FirstResponseTimeoutSeconds int64 `json:"first_response_timeout_seconds"`
}

var streamingSetting = StreamingSetting{FirstResponseTimeoutSeconds: 600}
var streamingSettingMutex sync.RWMutex

func init() {
	config.GlobalConfig.Register("streaming_setting", &streamingSetting)
}

func (settings *StreamingSetting) LockConfig() {
	streamingSettingMutex.Lock()
}

func (settings *StreamingSetting) UnlockConfig() {
	streamingSettingMutex.Unlock()
}

func (settings *StreamingSetting) RLockConfig() {
	streamingSettingMutex.RLock()
}

func (settings *StreamingSetting) RUnlockConfig() {
	streamingSettingMutex.RUnlock()
}

func (settings *StreamingSetting) ValidateConfigUpdate(values map[string]string) error {
	for key, value := range values {
		switch key {
		case "first_response_timeout_enabled":
			if value != "true" && value != "false" {
				return fmt.Errorf("streaming setting %s must be a boolean", key)
			}
		case "first_response_timeout_seconds":
			seconds, err := strconv.ParseInt(value, 10, 64)
			if err != nil || seconds < 1 || seconds > MaxFirstResponseTimeoutSeconds {
				return fmt.Errorf("streaming setting %s must be an integer between 1 and %d", key, MaxFirstResponseTimeoutSeconds)
			}
		default:
			return fmt.Errorf("unknown streaming setting %s", key)
		}
	}
	return nil
}

func ValidateStreamingOption(key, value string) error {
	configKey, ok := strings.CutPrefix(key, "streaming_setting.")
	if !ok {
		return nil
	}
	return streamingSetting.ValidateConfigUpdate(map[string]string{configKey: value})
}

func GetStreamingSetting() StreamingSetting {
	streamingSettingMutex.RLock()
	defer streamingSettingMutex.RUnlock()
	return streamingSetting
}

func (settings StreamingSetting) FirstResponseTimeout() time.Duration {
	if !settings.FirstResponseTimeoutEnabled {
		return 0
	}
	return time.Duration(settings.FirstResponseTimeoutSeconds) * time.Second
}
