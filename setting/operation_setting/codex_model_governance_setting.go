package operation_setting

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
)

const DefaultCodexUnsupportedPattern = `The '([^']+)' model is not supported when using Codex with a ChatGPT account\.`

type CodexModelGovernanceSetting struct {
	Enabled                    bool     `json:"enabled"`
	ProbeEnabled               bool     `json:"probe_enabled"`
	ProbeIntervalMinutes       int      `json:"probe_interval_minutes"`
	UnsupportedMessagePatterns []string `json:"unsupported_message_patterns"`
	OfficialSourceURLs         []string `json:"official_source_urls"`
	OfficialLifecycleTerms     []string `json:"official_lifecycle_terms"`
	AlertCooldownMinutes       int      `json:"alert_cooldown_minutes"`
}

var codexModelGovernanceSetting = CodexModelGovernanceSetting{
	Enabled:              false,
	ProbeEnabled:         false,
	ProbeIntervalMinutes: 1440,
	UnsupportedMessagePatterns: []string{
		DefaultCodexUnsupportedPattern,
	},
	OfficialSourceURLs: []string{},
	OfficialLifecycleTerms: []string{
		"deprecated",
		"retired",
		"sunset",
		"unavailable",
		"not supported",
	},
	AlertCooldownMinutes: 60,
}

func init() {
	config.GlobalConfig.Register("codex_model_governance_setting", &codexModelGovernanceSetting)
}

func GetCodexModelGovernanceSetting() *CodexModelGovernanceSetting {
	return &codexModelGovernanceSetting
}

func ValidateCodexModelGovernancePatterns(patterns []string) error {
	if len(patterns) == 0 {
		return fmt.Errorf("at least one Codex unsupported model pattern is required")
	}
	for index, pattern := range patterns {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			return fmt.Errorf("pattern #%d is empty", index+1)
		}
		if _, err := regexp.Compile(pattern); err != nil {
			return fmt.Errorf("pattern #%d is invalid: %w", index+1, err)
		}
	}
	return nil
}

func NormalizeCodexModelGovernanceOption(key string, value any) (string, error) {
	field := strings.TrimPrefix(key, "codex_model_governance_setting.")
	serialized := fmt.Sprint(value)
	switch field {
	case "unsupported_message_patterns", "official_source_urls", "official_lifecycle_terms":
		var values []string
		switch typed := value.(type) {
		case string:
			if strings.HasPrefix(strings.TrimSpace(typed), "[") {
				if err := common.UnmarshalJsonStr(typed, &values); err != nil {
					return "", fmt.Errorf("expected a JSON string array")
				}
			} else {
				values = strings.Split(typed, "\n")
			}
		default:
			data, err := common.Marshal(value)
			if err != nil {
				return "", err
			}
			if err := common.Unmarshal(data, &values); err != nil {
				return "", fmt.Errorf("expected a string array")
			}
		}
		normalized := make([]string, 0, len(values))
		for _, item := range values {
			if trimmed := strings.TrimSpace(item); trimmed != "" {
				normalized = append(normalized, trimmed)
			}
		}
		if field == "unsupported_message_patterns" {
			if err := ValidateCodexModelGovernancePatterns(normalized); err != nil {
				return "", err
			}
		}
		if len(normalized) > 64 {
			return "", fmt.Errorf("at most 64 entries are allowed")
		}
		if field == "official_source_urls" {
			for _, source := range normalized {
				if err := validateOperationHTTPURL(source); err != nil {
					return "", err
				}
			}
		}
		data, err := common.Marshal(normalized)
		return string(data), err
	case "enabled", "probe_enabled":
		parsed, err := strconv.ParseBool(serialized)
		if err != nil {
			return "", fmt.Errorf("expected a boolean")
		}
		return strconv.FormatBool(parsed), nil
	case "probe_interval_minutes", "alert_cooldown_minutes":
		parsed, err := strconv.Atoi(serialized)
		minimum := 1
		if field == "probe_interval_minutes" {
			minimum = 60
		}
		if err != nil || parsed < minimum || parsed > 525600 {
			return "", fmt.Errorf("minutes must be between %d and 525600", minimum)
		}
	default:
		return "", fmt.Errorf("unknown Codex governance setting")
	}
	return serialized, nil
}

func validateOperationHTTPURL(value string) error {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Hostname() == "" || parsed.User != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return fmt.Errorf("expected an HTTP(S) URL without embedded credentials")
	}
	return nil
}
