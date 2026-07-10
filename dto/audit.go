package dto

import "strings"

// 违规审计用的结构化对话提取。
//
// 目的:把不同协议的请求统一抽成 [{role,text}] —— 剥掉 base64(图片/音频/文件只留标记),
// 供违规审计做「按最新 user 消息判低危」+「存当前及前若干轮上下文」。
// 已实现:OpenAI 兼容(含 Grok)、Claude、Gemini;其它协议不实现本接口,由上层回退到
// meta.CombineText 的文本拼接方式。

type AuditMessage struct {
	Role string `json:"role"`
	Text string `json:"text"`
}

// AuditContentProvider 可选接口:请求类型实现它即支持结构化审计提取。
type AuditContentProvider interface {
	GetAuditMessages() []AuditMessage
}

const auditMediaMark = "[非文本内容]"

// ---- OpenAI 兼容(GeneralOpenAIRequest)—— 同时覆盖 Grok/xAI 等 OpenAI 同构协议 ----

func (r *GeneralOpenAIRequest) GetAuditMessages() []AuditMessage {
	out := make([]AuditMessage, 0, len(r.Messages))
	for i := range r.Messages {
		m := &r.Messages[i]
		text := strings.TrimSpace(m.StringContent()) // 仅文本,天然剥离 base64 媒体
		if text == "" && m.Content != nil {
			text = auditMediaMark
		}
		out = append(out, AuditMessage{Role: m.Role, Text: text})
	}
	return out
}

// ---- Claude(ClaudeRequest)----

func (c *ClaudeRequest) GetAuditMessages() []AuditMessage {
	out := make([]AuditMessage, 0, len(c.Messages)+1)
	// system(可能是字符串或结构化)
	if c.System != nil {
		var sysText string
		if c.IsStringSystem() {
			sysText = c.GetStringSystem()
		} else {
			sysText = claudeMediaText(c.ParseSystem())
		}
		if strings.TrimSpace(sysText) != "" {
			out = append(out, AuditMessage{Role: "system", Text: strings.TrimSpace(sysText)})
		}
	}
	for i := range c.Messages {
		m := &c.Messages[i]
		var text string
		if m.IsStringContent() {
			if s, ok := m.Content.(string); ok {
				text = s
			}
		} else {
			content, _ := m.ParseContent()
			text = claudeMediaText(content)
		}
		text = strings.TrimSpace(text)
		if text == "" {
			text = auditMediaMark
		}
		out = append(out, AuditMessage{Role: m.Role, Text: text})
	}
	return out
}

// claudeMediaText 从 Claude 媒体块里抽取纯文本(忽略图片/文档等 base64)。
func claudeMediaText(items []ClaudeMediaMessage) string {
	parts := make([]string, 0, len(items))
	for i := range items {
		it := &items[i]
		if it.Type == "text" && it.Text != nil && *it.Text != "" {
			parts = append(parts, *it.Text)
		}
	}
	return strings.Join(parts, "\n")
}

// ---- Gemini(GeminiChatRequest)----

func (r *GeminiChatRequest) GetAuditMessages() []AuditMessage {
	out := make([]AuditMessage, 0, len(r.Contents))
	for i := range r.Contents {
		c := &r.Contents[i]
		parts := make([]string, 0, len(c.Parts))
		for j := range c.Parts {
			if t := c.Parts[j].Text; t != "" {
				parts = append(parts, t)
			}
		}
		text := strings.TrimSpace(strings.Join(parts, "\n"))
		if text == "" {
			text = auditMediaMark
		}
		// Gemini 角色为 user/model,归一到审计通用角色
		role := c.Role
		if role == "model" {
			role = "assistant"
		}
		out = append(out, AuditMessage{Role: role, Text: text})
	}
	return out
}
