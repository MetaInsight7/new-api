package service

import (
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/types"

	"github.com/bytedance/gopkg/util/gopool"
	"github.com/gin-gonic/gin"
)

// 违规内容检测与审计记录。
//
// CheckViolation 在内置敏感词之上做“分类 + 分级”匹配(复用 AC 自动机 AcSearch):
//   - 命中任一高危词 => Block(拦截),Hit=true
//   - 仅命中低危词   => 放行,Hit=true(仅记录)
//   - 自定义词库为空 => 回退到内置 SensitiveWords(全部视为高危),保持向后兼容
// RecordViolation 异步把命中记录写入 LOG_DB(不阻塞 relay 主链路)。

// ViolationMatch 单个命中词及其分类/分级。
type ViolationMatch struct {
	Word     string `json:"word"`
	Category string `json:"category"`
	Severity string `json:"severity"`
}

// ViolationResult 一次检测的结果。
type ViolationResult struct {
	Hit        bool             `json:"hit"`
	Block      bool             `json:"block"`
	Severity   string           `json:"severity"` // 命中最高分级:high|low
	Words      []string         `json:"words"`
	Categories []string         `json:"categories"`
	Matches    []ViolationMatch `json:"matches"`
}

// ShouldCheckViolation 是否需要在提示词阶段做违规检测。
// 只认违规审计总开关 ViolationAuditEnabled(已与旧内置 CheckSensitive* 开关解耦:
// 面板只暴露这一个开关,词库等配置在“违规审计”页面)。
func ShouldCheckViolation() bool {
	return setting.ViolationAuditEnabled
}

// CheckViolation 检测文本是否命中违规词库(仅以违规审计的分类分级词库为准)。
func CheckViolation(text string) ViolationResult {
	res := ViolationResult{}
	if len(text) == 0 {
		return res
	}
	lowerText := strings.ToLower(text)

	high, low, meta := setting.ViolationDicts()
	_, highWords := AcSearch(lowerText, high, false)
	_, lowWords := AcSearch(lowerText, low, false)

	if len(highWords) == 0 && len(lowWords) == 0 {
		return res
	}
	res.Hit = true
	res.Block = len(highWords) > 0
	if res.Block {
		res.Severity = "high"
	} else {
		res.Severity = "low"
	}

	catSet := map[string]struct{}{}
	seenWord := map[string]struct{}{}
	appendMatches := func(words []string, fallbackSeverity string) {
		for _, w := range words {
			key := strings.ToLower(strings.TrimSpace(w))
			if key == "" {
				continue
			}
			if _, dup := seenWord[key]; dup {
				continue
			}
			seenWord[key] = struct{}{}
			m := ViolationMatch{Word: key, Category: "other", Severity: fallbackSeverity}
			if info, ok := meta[key]; ok {
				m.Category = info.Category
				m.Severity = info.Severity
			}
			catSet[m.Category] = struct{}{}
			res.Words = append(res.Words, key)
			res.Matches = append(res.Matches, m)
		}
	}
	appendMatches(highWords, "high")
	appendMatches(lowWords, "low")

	res.Categories = make([]string, 0, len(catSet))
	for c := range catSet {
		res.Categories = append(res.Categories, c)
	}
	return res
}

// RecordViolation 异步记录一次违规命中(仅在 result.Hit 时调用)。
func RecordViolation(c *gin.Context, result ViolationResult, scan AuditScan) {
	if !result.Hit {
		return
	}
	userId := c.GetInt("id")
	username := c.GetString("username")
	tokenId := c.GetInt("token_id")
	tokenName := c.GetString("token_name")
	group := c.GetString("group")
	modelName := c.GetString("original_model")
	channelId := c.GetInt("channel_id")
	requestId := c.GetString(common.RequestIdKey)

	// 是否记录 IP:复用用户设置(与 RecordErrorLog 一致)
	ip := ""
	if settingMap, err := model.GetUserSetting(userId, false); err == nil && settingMap.RecordIpLog {
		ip = c.ClientIP()
	}

	action := model.ViolationActionAllowed
	// 命中处片段取自被判定的文本:拦截取全量文本,放行取最新 user 文本
	matchedText := scan.LatestUserText
	if result.Block {
		action = model.ViolationActionBlocked
		matchedText = scan.FullText
	}

	log := &model.ViolationLog{
		CreatedAt:    time.Now().Unix(),
		UserId:       userId,
		Username:     username,
		TokenId:      tokenId,
		TokenName:    tokenName,
		UserGroup:    group,
		ModelName:    modelName,
		ChannelId:    channelId,
		Ip:           ip,
		RequestId:    requestId,
		Category:     strings.Join(result.Categories, ","),
		Severity:     result.Severity,
		Action:       action,
		MatchedWords: strings.Join(result.Words, ","),
		Snippet:      violationSnippet(matchedText, result.Words),
		Context:      scan.ContextJSON,
	}
	gopool.Go(func() {
		if err := model.CreateViolationLog(log); err != nil {
			common.SysError("failed to record violation log: " + err.Error())
		}
	})
}

// violationSnippet 取命中处上下文片段并截断(避免存整段 prompt,降隐私风险)。
func violationSnippet(text string, words []string) string {
	const window = 60
	const maxRunes = 500
	if text == "" {
		return ""
	}
	lower := strings.ToLower(text)
	pos := -1
	for _, w := range words {
		if w == "" {
			continue
		}
		if i := strings.Index(lower, strings.ToLower(w)); i >= 0 && (pos < 0 || i < pos) {
			pos = i
		}
	}
	if pos < 0 {
		pos = 0
	}
	start := pos - window
	if start < 0 {
		start = 0
	}
	end := pos + window
	if end > len(text) {
		end = len(text)
	}
	snippet := strings.ToValidUTF8(text[start:end], "")
	if start > 0 {
		snippet = "…" + snippet
	}
	if end < len(text) {
		snippet = snippet + "…"
	}
	if r := []rune(snippet); len(r) > maxRunes {
		snippet = string(r[:maxRunes])
	}
	return snippet
}

// ---- 审计扫描范围构建 ----

// AuditScan 一次请求的审计输入:
//   - FullText:全部消息文本(不分角色),用于高危拦截判定(堵伪造角色绕过)
//   - LatestUserText:最新一条 user 消息文本,用于低危记录(避免历史重复命中)
//   - ContextJSON:当前 + 前若干轮的结构化对话([{role,text}]),用于存证展示
type AuditScan struct {
	FullText       string
	LatestUserText string
	ContextJSON    string
}

const (
	violationContextMaxBytes = 256 * 1024 // 上下文存储封顶(PostgreSQL 无限,这里做安全阀)
	violationContextTurns    = 3          // 保留当前 + 前 N 轮
)

// BuildAuditScan 从请求构建审计扫描输入。
// 支持结构化提取的协议(OpenAI/Grok、Claude、Gemini)走 AuditContentProvider;
// 其余协议回退到 meta.CombineText 的文本拼接(不区分轮次/角色)。
func BuildAuditScan(request dto.Request, meta *types.TokenCountMeta) AuditScan {
	provider, ok := request.(dto.AuditContentProvider)
	if !ok {
		// 回退:整坨文本既作全量、也作“最新”(该协议不做轮次拆分)
		text := ""
		if meta != nil {
			text = meta.CombineText
		}
		return AuditScan{FullText: text, LatestUserText: text, ContextJSON: ""}
	}
	msgs := provider.GetAuditMessages()

	// 全量文本(不分角色)
	fullParts := make([]string, 0, len(msgs))
	for _, m := range msgs {
		if m.Text != "" {
			fullParts = append(fullParts, m.Text)
		}
	}
	fullText := strings.Join(fullParts, "\n")

	// 最新一条 user 消息
	latestUser := ""
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == "user" {
			latestUser = msgs[i].Text
			break
		}
	}

	// 上下文:system(若有) + 末尾「当前 + 前 N 轮」
	ctxMsgs := selectContextMessages(msgs, violationContextTurns)
	ctxJSON := marshalContextCapped(ctxMsgs, violationContextMaxBytes)

	return AuditScan{FullText: fullText, LatestUserText: latestUser, ContextJSON: ctxJSON}
}

// selectContextMessages 取 system 消息 + 末尾若干轮(1 轮≈user+assistant,故取末尾 2*turns+1 条非 system)。
func selectContextMessages(msgs []dto.AuditMessage, turns int) []dto.AuditMessage {
	sys := make([]dto.AuditMessage, 0, 1)
	rest := make([]dto.AuditMessage, 0, len(msgs))
	for _, m := range msgs {
		if m.Role == "system" {
			sys = append(sys, m)
		} else {
			rest = append(rest, m)
		}
	}
	keep := turns*2 + 1
	if len(rest) > keep {
		rest = rest[len(rest)-keep:]
	}
	return append(sys, rest...)
}

// marshalContextCapped 序列化上下文;超出上限则从最早的非首条逐步丢弃,直到达标。
func marshalContextCapped(msgs []dto.AuditMessage, maxBytes int) string {
	if len(msgs) == 0 {
		return ""
	}
	for {
		b, err := common.Marshal(msgs)
		if err != nil {
			return ""
		}
		if len(b) <= maxBytes || len(msgs) <= 1 {
			return string(b)
		}
		// 丢弃最早的一条(保留最后一条,通常是命中的最新 user)
		msgs = msgs[1:]
	}
}
