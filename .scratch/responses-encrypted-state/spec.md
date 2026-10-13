# Responses 加密推理状态有损续接 Spec

## Problem Statement

Responses 客户端会把上游返回的 reasoning item 放入后续请求，其中的 `encrypted_content` 与 provider-owned item ID 只在生成它们的上游凭证安全域内有效。渠道亲和过期、渠道故障或普通重试导致请求进入另一个安全域时，上游可能返回 `invalid_encrypted_content`、`thinking_signature_invalid`，或只在错误消息中说明 encrypted content 无法验证、解密、解析，最终使仍然包含可读对话和 reasoning summary 的长会话完全中断。

当前系统没有针对这种不兼容状态的恢复边界。管理员也无法在请求策略页面控制是否允许有损续接，无法从请求日志判断系统是否为了续接而删除了不可移植的隐藏推理状态。

## Solution

系统在 Responses 上游明确拒绝加密推理状态后，允许执行一次受控的有损续接：遍历完整 `input`，从 reasoning item 删除不可移植的 `encrypted_content` 与 provider-owned item ID，保留可读的 `summary` 或 `content`；没有任何可读内容的 reasoning item 整体删除。清理实际改变请求体后，系统在当前渠道额外重试一次，而且不消耗管理员配置的普通重试次数。

若额外重试仍失败，本次入站请求不再执行第二次清理，并把已经清理的请求交还普通重试和换渠策略。系统不清除 `previous_response_id` 或 `conversation`，不建立密文兼容缓存，也不尝试解密或重新加密隐藏推理状态。

该能力默认开启，由管理员在“系统设置 → 请求策略 → 会话与重试”通过全局开关控制。界面必须明确提示恢复会丢失隐藏推理连续性。恢复事实只进入管理员可见的请求日志 `admin_info`，不增加面向客户端的响应头、SSE 事件或 WebSocket 扩展事件。

## User Stories

1. 作为 Responses API 用户，我希望渠道安全域变化后长会话仍能继续，以免一段不可移植密文使全部可读上下文失效。
2. 作为 Responses API 用户，我希望系统保留 reasoning summary 和普通消息，以便有损恢复后仍保留尽可能多的可读语义。
3. 作为 Responses API 用户，我希望系统删除与密文绑定的 provider-owned item ID，以免残留 ID 继续引用原安全域状态。
4. 作为 Responses API 用户，我希望没有可读内容的 reasoning item 被整体删除，以免向新上游发送无意义或结构非法的空 reasoning item。
5. 作为非流式 Responses 用户，我希望 HTTP 400 中的加密状态拒绝能够自动恢复。
6. 作为 SSE Responses 用户，我希望首个可恢复的流内失败事件不会先泄漏给客户端再重试，以免一个逻辑请求产生相互矛盾的两个终态。
7. 作为 Responses Compact 用户，我希望压缩接口与普通 Responses 接口使用相同的有损续接规则。
8. 作为 Responses WebSocket 用户，我希望上游在接受请求前拒绝加密状态时，系统能够在同一逻辑请求内恢复，而不要求客户端重连并手工删字段。
9. 作为 API 用户，我希望恢复重试不消耗普通 `RetryTimes`，以免兼容性修复减少真正用于渠道故障切换的预算。
10. 作为 API 用户，我希望同一个入站请求最多执行一次恢复周期，以免持续错误形成清理和重试循环。
11. 作为 API 用户，我希望恢复后仍失败时，后续普通渠道尝试继续使用已清理请求，以免每个渠道重复接收已知不兼容的密文。
12. 作为管理员，我希望该能力默认开启，让开发期环境无需额外配置即可获得安全的连续性兜底。
13. 作为管理员，我希望能通过一个全局开关立即停用有损续接，以便严格保留原始请求语义或排查问题。
14. 作为管理员，我希望开关旁有醒目的有损说明，以便理解隐藏推理连续性会丢失，而不是把该能力误认为无损迁移。
15. 作为管理员，我希望请求日志记录恢复事件、原渠道和结果，但不记录密文本身，以便审计行为且不扩大敏感状态暴露面。
16. 作为渠道管理员，我希望一次可恢复的密文拒绝在恢复成功时不禁用、不冷却、不排除渠道，也不生成错误日志，以免把凭证安全域不兼容误判为渠道健康故障。
17. 作为渠道管理员，我希望恢复重试仍失败后重新进入既有普通重试与渠道健康策略，以便最终失败继续遵守平台已有治理规则。
18. 作为兼容上游用户，我希望系统识别直接的 `invalid_encrypted_content` 和中转归一化后的 `thinking_signature_invalid`，以便不同中转链路表现一致。
19. 作为兼容上游用户，我希望没有结构化错误码时，系统能识别 “encrypted content could not be verified/decrypted/parsed” 和 “invalid signature in thinking/reasoning block” 这类窄语义错误。
20. 作为安全负责人，我希望语义识别还要求请求确有可清理的加密 reasoning，并且清理必须实际改变请求体，以免普通 4xx 被错误重试。
21. 作为协议兼容维护者，我希望 pass-through 请求只对 `input` 做外科式改写并保留未知顶层字段，以免恢复功能破坏未来 Responses 字段。
22. 作为协议兼容维护者，我希望 `previous_response_id` 与 `conversation` 保持原样，以免功能在没有明确依据时扩大状态删除范围。

## Implementation Decisions

- 使用一个共享的 Responses 加密推理状态恢复模块统一完成错误分类、JSON 清理、单次恢复状态和管理员事件记录；HTTP、Compact、SSE 与 WebSocket 只负责各自的传输边界。
- 全局配置键为 `responses_encrypted_reasoning_setting.recovery_enabled`，默认值为 `true`，纳入现有请求策略读取、校验和原子更新模型。
- 精确错误码白名单包括 `invalid_encrypted_content` 与 `thinking_signature_invalid`。
- 结构化错误码缺失或被中转改写时，仅把 4xx `invalid_request_error` 的窄语义组合视为候选：`encrypted content` 与 `could not be verified`、`could not be decrypted` 或 `could not be parsed` 同时出现；或者 `invalid signature` 与 `thinking block` 或 `reasoning block` 同时出现。比较不区分大小写。
- 即使错误信号命中，也必须确认请求包含加密 reasoning，且清理结果实际改变请求，才允许启动恢复重试。
- 清理遍历 `input` 中全部 reasoning item，删除 `encrypted_content` 和 provider-owned `id`，保留其他未知字段以及非空 `summary`、`content`。
- reasoning item 在清理后没有非空 `summary` 或 `content` 时整体删除；普通消息、工具项及非 reasoning item 原样保留。
- pass-through 模式改写原始 JSON 请求存储，保留所有未知顶层字段；已解析请求模型同步更新相同的 `input`，保证 pass-through 与转换路径一致。
- 不删除或改写 `previous_response_id`、`conversation`、普通 `output` 字段，也不处理上游响应中的 output。
- 每个入站请求持有独立的恢复状态；状态最多从“未尝试”进入一次“已尝试”，不会因第二个渠道再次命中相同错误而重置。
- 首次命中后在当前渠道额外重试一次，不增加普通重试索引，不预先排除当前渠道，也不调用渠道处罚、冷却或错误日志路径。
- 额外重试成功时正常完成请求；额外重试失败时，只把最终错误交给既有普通重试决策，且普通重试读取已清理请求。
- SSE 只有在尚未向客户端提交响应数据时才能恢复；若客户端已收到业务事件，则保持既有单终态和不可重试边界。
- WebSocket 只在上游接受当前 `response.create` 前执行恢复；已经接受并开始输出后不重放。恢复失败后的渠道切换遵守现有普通重试预算和选择策略。
- 恢复事件进入现有管理员请求策略诊断数据，记录动作、原因、渠道、状态和错误码，不记录密文、密文哈希、summary 正文或用户输入。
- 前端在“会话与重试”区域提供单一开关，旁边显示有损恢复警告；不新增独立页面、统计面板或逐渠道开关。
- 不向客户端增加恢复响应头、SSE 事件、WebSocket 事件或错误扩展字段，避免改变公开协议。

## Testing Decisions

- 最高测试接缝是完整的请求恢复状态机：输入原始请求、上游错误和连续尝试结果，断言实际发送次数、同渠道额外重试、普通重试预算、渠道处罚边界和最终请求体，而不是断言内部函数调用。
- JSON 清理器使用纯行为测试覆盖多轮 `input`、多个 reasoning item、未知字段、非 reasoning item、空/非空 summary 与 content、provider-owned ID、字符串 input、畸形 JSON 和无变化输入。
- 错误分类测试覆盖两个精确代码、两组窄消息模式、大小写变化、CPA 归一化结果，以及相似但不满足成对关键词的负例。
- HTTP 集成测试覆盖非流式和 Compact：恢复成功不消耗普通重试、不处罚渠道；恢复后仍失败只清理一次并进入普通策略；开关关闭时原错误原样进入既有路径。
- SSE 集成测试覆盖首事件即失败、错误码缺失但消息命中、已经输出后失败、恢复成功及恢复失败，确保客户端不会看到被恢复掉的失败终态。
- WebSocket 集成测试沿用现有真实客户端/伪上游接缝，覆盖接受前拒绝、同连接清理重试、恢复后普通换渠、接受后错误不重放和每个 `response.create` 独立的一次性状态。
- 请求日志测试从管理员日志输出断言恢复事件可见且不含密文；普通用户日志继续依赖既有 `admin_info` 剥离边界。
- 前端组件测试覆盖默认开启、加载已关闭配置、切换后保存最小 delta、醒目有损说明和保存失败后保留编辑值。
- 定向测试通过后运行受影响 Go 包测试、前端 Vitest、TypeScript 检查与 lint；不修复与本功能无关的既有失败。

## Out of Scope

- 不实现 LiteLLM 的 security-domain 来源标记或基于来源的预清理。
- 不延长渠道亲和 TTL，不建立密文、会话或渠道不兼容缓存。
- 不尝试解密、验证、转换或重新加密 `encrypted_content`。
- 不把 API Key、Codex subscription、Azure deployment 或 organization 自动拆分为不同路由池。
- 不清理 `previous_response_id`、`conversation` 或普通输出历史。
- 不在每次请求发送前主动删除 reasoning 状态。
- 不新增客户端可见恢复标志、独立监控页、恢复统计卡片或渠道级配置。
- 不改变普通重试次数、状态码规则、渠道优先级、权重、并发或自动分组算法。

## Further Notes

- `thinking_signature_invalid` 是 CLIProxyAPI 等中转对 `invalid_encrypted_content` 或 “Invalid signature in thinking block” 的归一化错误码，不应描述为 OpenAI 固定原始错误码。
- 有损续接保护的是可读会话连续性，不保证隐藏推理连续性；管理员界面和文档必须使用“有损”术语。
- 一个新的客户端请求拥有新的恢复周期。如果客户端再次携带旧密文，该新请求仍可清理一次；同一个入站请求内不会重复清理。
