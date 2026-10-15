package operation_setting

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
)

type MonitorSetting struct {
	AutoTestChannelEnabled       bool    `json:"auto_test_channel_enabled"`
	AutoTestChannelMinutes       float64 `json:"auto_test_channel_minutes"`
	ChannelTestMode              string  `json:"channel_test_mode"`
	ChannelTestConcurrency       int     `json:"channel_test_concurrency"`
	DingTalkAlertEnabled         bool    `json:"dingtalk_alert_enabled"`
	DingTalkAlertWebhookURL      string  `json:"dingtalk_alert_webhook_url"`
	DingTalkAlertSecret          string  `json:"dingtalk_alert_secret"`
	DingTalkAlertCooldownMinutes int     `json:"dingtalk_alert_cooldown_minutes"`
	AIAnalysisAPIKey             string  `json:"ai_analysis_api_key"`
	AIAnalysisBaseURL            string  `json:"ai_analysis_base_url"`
	AIAnalysisModel              string  `json:"ai_analysis_model"`
	aiAnalysisBaseURLConfigured  bool
	aiAnalysisModelConfigured    bool
}

const (
	ChannelTestModeScheduledAll    = "scheduled_all"
	ChannelTestModeAutoBanOnly     = "auto_ban_only"
	ChannelTestModePassiveRecovery = "passive_recovery"

	ChannelTestConcurrencyOptionKey   = "monitor_setting.channel_test_concurrency"
	DefaultChannelTestConcurrency     = 1
	MaxChannelTestConcurrency         = 32
	DefaultMonitorAIAnalysisBaseURL   = "https://api.openai.com/v1"
	DefaultMonitorAIAnalysisModelName = "gpt-5.4-mini"
)

// 默认配置
var monitorSetting = MonitorSetting{
	AutoTestChannelEnabled:       false,
	AutoTestChannelMinutes:       10,
	ChannelTestMode:              ChannelTestModeScheduledAll,
	ChannelTestConcurrency:       DefaultChannelTestConcurrency,
	DingTalkAlertCooldownMinutes: 60,
	AIAnalysisBaseURL:            DefaultMonitorAIAnalysisBaseURL,
	AIAnalysisModel:              DefaultMonitorAIAnalysisModelName,
}

func init() {
	// 注册到全局配置管理器
	config.GlobalConfig.Register("monitor_setting", &monitorSetting)
}

func GetMonitorSetting() *MonitorSetting {
	if os.Getenv("CHANNEL_TEST_FREQUENCY") != "" {
		frequency, err := strconv.Atoi(os.Getenv("CHANNEL_TEST_FREQUENCY"))
		if err == nil && frequency > 0 {
			monitorSetting.AutoTestChannelEnabled = true
			monitorSetting.AutoTestChannelMinutes = float64(frequency)
			monitorSetting.ChannelTestMode = ChannelTestModeScheduledAll
		}
	}
	if enabled, ok := os.LookupEnv("CHANNEL_TEST_ENABLED"); ok {
		parsed, err := strconv.ParseBool(enabled)
		if err == nil {
			monitorSetting.AutoTestChannelEnabled = parsed
		}
	}
	switch monitorSetting.ChannelTestMode {
	case ChannelTestModeAutoBanOnly, ChannelTestModePassiveRecovery:
	default:
		monitorSetting.ChannelTestMode = ChannelTestModeScheduledAll
	}
	monitorSetting.ChannelTestConcurrency = NormalizeChannelTestConcurrency(monitorSetting.ChannelTestConcurrency)
	return &monitorSetting
}

func NormalizeChannelTestConcurrency(concurrency int) int {
	if concurrency < 1 {
		return DefaultChannelTestConcurrency
	}
	if concurrency > MaxChannelTestConcurrency {
		return MaxChannelTestConcurrency
	}
	return concurrency
}

func ValidateChannelTestConcurrency(value string) error {
	concurrency, err := strconv.Atoi(value)
	if err != nil || concurrency < 1 || concurrency > MaxChannelTestConcurrency {
		return fmt.Errorf("channel test concurrency must be between 1 and %d", MaxChannelTestConcurrency)
	}
	return nil
}

func GetMonitorAlertSetting() MonitorSetting {
	common.OptionMapRWMutex.RLock()
	defer common.OptionMapRWMutex.RUnlock()
	return MonitorSetting{
		DingTalkAlertEnabled:         monitorSetting.DingTalkAlertEnabled,
		DingTalkAlertWebhookURL:      strings.TrimSpace(monitorSetting.DingTalkAlertWebhookURL),
		DingTalkAlertSecret:          strings.TrimSpace(monitorSetting.DingTalkAlertSecret),
		DingTalkAlertCooldownMinutes: monitorSetting.DingTalkAlertCooldownMinutes,
		AIAnalysisAPIKey:             strings.TrimSpace(monitorSetting.AIAnalysisAPIKey),
		AIAnalysisBaseURL:            strings.TrimSpace(monitorSetting.AIAnalysisBaseURL),
		AIAnalysisModel:              strings.TrimSpace(monitorSetting.AIAnalysisModel),
		aiAnalysisBaseURLConfigured:  monitorSetting.aiAnalysisBaseURLConfigured,
		aiAnalysisModelConfigured:    monitorSetting.aiAnalysisModelConfigured,
	}
}

func ResetMonitorAIAnalysisConfiguration() {
	monitorSetting.aiAnalysisBaseURLConfigured = false
	monitorSetting.aiAnalysisModelConfigured = false
}

func MarkMonitorAIAnalysisOption(key string) {
	switch key {
	case "monitor_setting.ai_analysis_base_url":
		monitorSetting.aiAnalysisBaseURLConfigured = true
	case "monitor_setting.ai_analysis_model":
		monitorSetting.aiAnalysisModelConfigured = true
	}
}

func GetMonitorAIAnalysisAPIKey() string {
	return GetMonitorAlertSetting().AIAnalysisAPIKey
}

func GetMonitorAIAnalysisBaseURL() string {
	return GetMonitorAIAnalysisConfig().BaseURL
}

func GetMonitorAIAnalysisModel() string {
	return GetMonitorAIAnalysisConfig().Model
}

type MonitorAIAnalysisConfig struct {
	APIKey  string
	BaseURL string
	Model   string
}

func GetMonitorAIAnalysisConfig() MonitorAIAnalysisConfig {
	return GetMonitorAlertSetting().AIAnalysisConfig()
}

func (setting MonitorSetting) AIAnalysisConfig() MonitorAIAnalysisConfig {
	baseURL, modelName := setting.AIAnalysisBaseURL, setting.AIAnalysisModel
	if !setting.aiAnalysisBaseURLConfigured {
		baseURL = strings.TrimSpace(os.Getenv("MONITOR_AI_ANALYSIS_BASE_URL"))
	}
	if baseURL == "" {
		baseURL = DefaultMonitorAIAnalysisBaseURL
	}
	if !setting.aiAnalysisModelConfigured {
		modelName = strings.TrimSpace(os.Getenv("MONITOR_AI_ANALYSIS_MODEL"))
	}
	if modelName == "" {
		modelName = DefaultMonitorAIAnalysisModelName
	}
	return MonitorAIAnalysisConfig{APIKey: setting.AIAnalysisAPIKey, BaseURL: baseURL, Model: modelName}
}

func ValidateMonitorOption(key, value string) error {
	field, ok := strings.CutPrefix(key, "monitor_setting.")
	if !ok {
		return nil
	}
	switch field {
	case "dingtalk_alert_enabled":
		if _, err := strconv.ParseBool(value); err != nil {
			return fmt.Errorf("expected a boolean")
		}
	case "dingtalk_alert_cooldown_minutes":
		minutes, err := strconv.ParseInt(value, 10, 64)
		if err != nil || minutes < 1 || minutes > int64((1<<63-1)/time.Minute) {
			return fmt.Errorf("DingTalk cooldown must be positive whole minutes within the supported duration")
		}
	case "dingtalk_alert_webhook_url", "ai_analysis_base_url":
		if value = strings.TrimSpace(value); value != "" {
			return validateOperationHTTPURL(value)
		}
	case "dingtalk_alert_secret", "ai_analysis_api_key", "ai_analysis_model":
	case "auto_test_channel_enabled", "auto_test_channel_minutes", "channel_test_mode", "channel_test_concurrency":
	default:
		return fmt.Errorf("unknown monitoring setting")
	}
	return nil
}

func ValidateMonitorOptions(values map[string]string) error {
	value, enabledChanged := values["monitor_setting.dingtalk_alert_enabled"]
	webhook, webhookChanged := values["monitor_setting.dingtalk_alert_webhook_url"]
	if !enabledChanged && !webhookChanged {
		return nil
	}
	current := GetMonitorAlertSetting()
	enabled := current.DingTalkAlertEnabled
	if enabledChanged {
		enabled, _ = strconv.ParseBool(value)
	}
	if !enabled {
		return nil
	}
	if !webhookChanged {
		webhook = current.DingTalkAlertWebhookURL
	}
	if strings.TrimSpace(webhook) == "" {
		return fmt.Errorf("configure the DingTalk webhook before enabling alerts")
	}
	return validateOperationHTTPURL(strings.TrimSpace(webhook))
}
