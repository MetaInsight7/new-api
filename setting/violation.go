package setting

import (
	"sort"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
)

// 违规审计(内容安全)配置层。
//
// 设计:在内置敏感词(SensitiveWords)之上,提供“分类 + 分级”的违规词库。
//   - 分类:politics/porn/abuse/other(涉政/涉黄/辱骂/其他)
//   - 分级:high/low(高危拦截、低危放行但记录)
// 词库以 JSON 存 options 表(面板可配、免重启),这里持有解析后的结构与
// 高危/低危两份词典(供 service 层复用现成 AC 自动机匹配)。词库为空时,
// service 层会回退到内置 SensitiveWords(全部视为高危),保持向后兼容。

// ViolationAuditEnabled 违规审计总开关(默认关闭)。
var ViolationAuditEnabled = false

// ViolationWord 单条违规词条。
type ViolationWord struct {
	Word     string `json:"word"`
	Category string `json:"category"` // politics|porn|abuse|other
	Severity string `json:"severity"` // high|low
}

// 合法取值(前端下拉/校验用)。
var ViolationCategories = []string{"politics", "porn", "abuse", "other"}
var ViolationSeverities = []string{"high", "low"}

var (
	violationMu    sync.RWMutex
	violationWords []ViolationWord
	violationHigh  []string                 // 高危词典(小写)
	violationLow   []string                 // 低危词典(小写)
	violationMeta  map[string]ViolationWord // lower(word) -> 词条元信息
)

func normalizeCategory(c string) string {
	c = strings.ToLower(strings.TrimSpace(c))
	for _, v := range ViolationCategories {
		if v == c {
			return c
		}
	}
	return "other"
}

func normalizeSeverity(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "low" {
		return "low"
	}
	return "high"
}

// rebuildViolationIndex 由 violationWords 重建高危/低危词典与元信息映射。
// 调用方需持有 violationMu 写锁。
func rebuildViolationIndex() {
	high := make([]string, 0, len(violationWords))
	low := make([]string, 0, len(violationWords))
	meta := make(map[string]ViolationWord, len(violationWords))
	for _, w := range violationWords {
		key := strings.ToLower(strings.TrimSpace(w.Word))
		if key == "" {
			continue
		}
		if _, exists := meta[key]; exists {
			continue
		}
		meta[key] = w
		if w.Severity == "high" {
			high = append(high, key)
		} else {
			low = append(low, key)
		}
	}
	violationHigh = high
	violationLow = low
	violationMeta = meta
}

// SetViolationWords 从 JSON 字符串加载词库并重建索引(option 更新时调用)。
func SetViolationWords(jsonStr string) {
	words := make([]ViolationWord, 0)
	jsonStr = strings.TrimSpace(jsonStr)
	if jsonStr != "" {
		var parsed []ViolationWord
		if err := common.UnmarshalJsonStr(jsonStr, &parsed); err == nil {
			for _, w := range parsed {
				word := strings.TrimSpace(w.Word)
				if word == "" {
					continue
				}
				words = append(words, ViolationWord{
					Word:     word,
					Category: normalizeCategory(w.Category),
					Severity: normalizeSeverity(w.Severity),
				})
			}
		}
	}
	violationMu.Lock()
	violationWords = words
	rebuildViolationIndex()
	violationMu.Unlock()
}

// ViolationWordsToString 序列化当前词库为 JSON(存 option / 返回前端)。
func ViolationWordsToString() string {
	violationMu.RLock()
	defer violationMu.RUnlock()
	if len(violationWords) == 0 {
		return "[]"
	}
	b, err := common.Marshal(violationWords)
	if err != nil {
		return "[]"
	}
	return string(b)
}

// GetViolationWords 返回词库副本(按分类、分级、词排序,便于前端展示)。
func GetViolationWords() []ViolationWord {
	violationMu.RLock()
	defer violationMu.RUnlock()
	out := make([]ViolationWord, len(violationWords))
	copy(out, violationWords)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Category != out[j].Category {
			return out[i].Category < out[j].Category
		}
		if out[i].Severity != out[j].Severity {
			return out[i].Severity < out[j].Severity
		}
		return out[i].Word < out[j].Word
	})
	return out
}

// ViolationDicts 返回高危词典、低危词典与词->元信息映射(供 service 匹配)。
func ViolationDicts() (high []string, low []string, meta map[string]ViolationWord) {
	violationMu.RLock()
	defer violationMu.RUnlock()
	high = make([]string, len(violationHigh))
	copy(high, violationHigh)
	low = make([]string, len(violationLow))
	copy(low, violationLow)
	meta = make(map[string]ViolationWord, len(violationMeta))
	for k, v := range violationMeta {
		meta[k] = v
	}
	return high, low, meta
}

// ViolationLibraryEmpty 自定义词库是否为空(为空时 service 回退到内置敏感词)。
func ViolationLibraryEmpty() bool {
	violationMu.RLock()
	defer violationMu.RUnlock()
	return len(violationWords) == 0
}
