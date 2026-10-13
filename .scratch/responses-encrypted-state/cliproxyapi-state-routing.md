# CLIProxyAPI：Codex / OpenAI Responses 加密状态与多凭证路由调查

> 调研截止：2026-10-11（Asia/Shanghai）
> 第一方来源：`router-for-me/CLIProxyAPI` 官方源码、Issues、PR、Releases 与仓库文档。
> 已发布基线：[`v8.0.23`](https://github.com/router-for-me/CLIProxyAPI/releases/tag/v8.0.23)，commit [`d318bcc3afb9ea8782862f5e8cdb541afdc9dfa5`](https://github.com/router-for-me/CLIProxyAPI/commit/d318bcc3afb9ea8782862f5e8cdb541afdc9dfa5)。
> 当前 `main` 基线：[`f1d4a94ef4398349716f2db5bbc3a27624c174a7`](https://github.com/router-for-me/CLIProxyAPI/commit/f1d4a94ef4398349716f2db5bbc3a27624c174a7)。

## 结论

CLIProxyAPI **没有真正解决“产生密文的原账号不可用后，跨账号无损恢复 Responses 会话”**，当前 `main` 也没有自动的有损 `strip + retry` 恢复：

1. `routing.session-affinity` 只是可选的、进程内的 `session -> auth ID` 滑动 TTL 绑定；默认关闭、默认 TTL 为 1 小时。绑定账号不可用时会自动选择新凭证并覆盖绑定，而不是为有状态 Responses fail closed。[配置说明](https://github.com/router-for-me/CLIProxyAPI/blob/d318bcc3afb9ea8782862f5e8cdb541afdc9dfa5/config.example.yaml#L143-L166)、[selector 实现](https://github.com/router-for-me/CLIProxyAPI/blob/d318bcc3afb9ea8782862f5e8cdb541afdc9dfa5/sdk/cliproxy/auth/selector.go#L1030-L1111)
2. `encrypted_content` sanitizer 只能检查 GPT/Codex signature 的格式；结构合法但由另一个账号生成的 opaque blob 会原样保留。它不知道密文来自哪个凭证，也不能判断新凭证能否解密。[sanitizer](https://github.com/router-for-me/CLIProxyAPI/blob/d318bcc3afb9ea8782862f5e8cdb541afdc9dfa5/internal/runtime/executor/openai_responses_signature.go#L112-L239)
3. 上游 `invalid_encrypted_content` 被映射为 HTTP 400 `thinking_signature_invalid / invalid_request_error`；400 被视为请求错误并停止凭证轮换。[错误映射](https://github.com/router-for-me/CLIProxyAPI/blob/d318bcc3afb9ea8782862f5e8cdb541afdc9dfa5/internal/runtime/executor/codex_executor_terminal.go#L354-L375)、[停止轮换](https://github.com/router-for-me/CLIProxyAPI/blob/d318bcc3afb9ea8782862f5e8cdb541afdc9dfa5/sdk/cliproxy/auth/conductor_execution.go#L628-L690)
4. 此错误只会清除 CLIProxyAPI 自己的 reasoning replay cache，不会删除客户端当前请求中的 reasoning/`encrypted_content`，也不会重写后自动重试。[清缓存函数](https://github.com/router-for-me/CLIProxyAPI/blob/d318bcc3afb9ea8782862f5e8cdb541afdc9dfa5/internal/runtime/executor/codex_executor_reasoning.go#L817-L825)
5. 官方维护者在 [Issue #6420](https://github.com/router-for-me/CLIProxyAPI/issues/6420) 明确确认：混用 API key 与 subscription account，或混用不同 organization 的凭证时会发生该问题；代理无法安全转换 opaque `encrypted_content`，当前必要做法是拆分路由池，并以 `not planned` 关闭。[维护者原文](https://github.com/router-for-me/CLIProxyAPI/issues/6420#issuecomment-6017464221)

因此：

- **原账号仍健康且 affinity 未丢失**：通常可无损继续。
- **原账号永久失效、限流后改绑、TTL 过期、服务重启或绑定被清除**：不能保证无损恢复。
- **删掉密文后继续**：可以作为显式有损恢复，但当前发布版和 `main` 都未自动实现；模型只能看到保留下来的 summary/可读历史，而不是完整旧 reasoning。Issue #6420 的报告者也确认这种客户端 workaround 仅支持 HTTP、每次切换仍先失败一次，且 WebSocket 不支持。[workaround 说明](https://github.com/router-for-me/CLIProxyAPI/issues/6420#issuecomment-6021035757)

## 版本判定

### 已发布稳定行为

- 最新稳定版为 [`v8.0.23`](https://github.com/router-for-me/CLIProxyAPI/releases/tag/v8.0.23)，对应 `d318bcc3...`。
- 已合并 [PR #3615](https://github.com/router-for-me/CLIProxyAPI/pull/3615) 的能力是“按 provider/signature 格式过滤不兼容 signature”；它解决跨 provider 的格式错误，不会识别同为 GPT 格式但属于另一个账号的不可解密密文。
- 默认选路是 round-robin，也可配置 weighted-round-robin 或 fill-first；session affinity 是这些 selector 外的一层可选包装。[selector 构造](https://github.com/router-for-me/CLIProxyAPI/blob/d318bcc3afb9ea8782862f5e8cdb541afdc9dfa5/sdk/cliproxy/service_config.go#L36-L84)

### 当前 `main`

从 `v8.0.23` 到本次 `main@f1d4a94...`，session affinity、session cache、Responses sanitizer、terminal error classification、reasoning replay clear 等核心文件没有引入跨账号 encrypted-state 恢复。相关固定比较见 [`v8.0.23...main` compare](https://github.com/router-for-me/CLIProxyAPI/compare/d318bcc3afb9ea8782862f5e8cdb541afdc9dfa5...f1d4a94ef4398349716f2db5bbc3a27624c174a7)。发布后涉及 Codex WebSocket 的 [`5dbd92a0`](https://github.com/router-for-me/CLIProxyAPI/commit/5dbd92a0) 只调整错误先交给 conductor、再通知断线的顺序，以保留 cooldown；没有实现密文迁移或 strip-and-retry。

以下提案均不能视为稳定版或当前 `main` 行为：

| 提案 | 目标 | 状态 |
|---|---|---|
| [PR #1796](https://github.com/router-for-me/CLIProxyAPI/pull/1796) | 用 `previous_response_id`/密文 hash 绑定 auth；失败时 strip reasoning 后重试 | Closed，未合并 |
| [PR #3546](https://github.com/router-for-me/CLIProxyAPI/pull/3546) | Responses 的绑定 auth 不可用时 fail closed 503，不跨账号 | Closed，未合并 |
| [PR #5109](https://github.com/router-for-me/CLIProxyAPI/pull/5109) | 暂时 fallback 时保留主 affinity，恢复后切回 | Closed，未合并 |
| [PR #5220](https://github.com/router-for-me/CLIProxyAPI/pull/5220) | 把 provider-bound history 投影为中性历史 | Closed，未合并 |
| [PR #5410](https://github.com/router-for-me/CLIProxyAPI/pull/5410) | #5220 的替代实现 | Open，尚未进入 `main`/Release |

## 选账号与 Session Affinity

### 普通账号选择

- 默认 round-robin 按 `provider + canonical model` 维护上次 auth ID，并在当前可用候选中选择后继；候选变化时仍按 ID 环继续。[RoundRobinSelector](https://github.com/router-for-me/CLIProxyAPI/blob/d318bcc3afb9ea8782862f5e8cdb541afdc9dfa5/sdk/cliproxy/auth/selector.go#L27-L37)、[Pick](https://github.com/router-for-me/CLIProxyAPI/blob/d318bcc3afb9ea8782862f5e8cdb541afdc9dfa5/sdk/cliproxy/auth/selector.go#L613-L636)
- 只向 selector 提供当前最高可用优先级层；同层候选按 ID 稳定排序。[候选分层](https://github.com/router-for-me/CLIProxyAPI/blob/d318bcc3afb9ea8782862f5e8cdb541afdc9dfa5/sdk/cliproxy/auth/selector.go#L523-L579)
- weighted-round-robin 使用凭证 weight 做 smooth WRR；非正权重被排除。[WRR](https://github.com/router-for-me/CLIProxyAPI/blob/d318bcc3afb9ea8782862f5e8cdb541afdc9dfa5/sdk/cliproxy/auth/selector.go#L662-L706)

### Affinity key 与生命周期

- selector 从 Codex/Claude/OpenCode/pi headers、`prompt_cache_key`、Responses `conversation.id`、其他 body ID、execution metadata 和消息历史派生 session ID；`prompt_cache_key` 优先于 `conversation`。[提取优先级](https://github.com/router-for-me/CLIProxyAPI/blob/d318bcc3afb9ea8782862f5e8cdb541afdc9dfa5/sdk/cliproxy/auth/selector.go#L1587-L1604)、[body 提取](https://github.com/router-for-me/CLIProxyAPI/blob/d318bcc3afb9ea8782862f5e8cdb541afdc9dfa5/sdk/cliproxy/session/info.go#L783-L826)
- 实际缓存 key 是 `provider + session ID + canonical model`，value 是具体 `auth.ID`。[绑定 key](https://github.com/router-for-me/CLIProxyAPI/blob/d318bcc3afb9ea8782862f5e8cdb541afdc9dfa5/sdk/cliproxy/auth/selector.go#L1038-L1057)
- cache hit 且 auth 可用时复用并刷新 TTL；auth 不可用时立即调用 fallback selector、选新 auth 并覆盖绑定；cache miss/过期也按普通 selector 冷启动再绑定。[命中、改绑与冷绑定](https://github.com/router-for-me/CLIProxyAPI/blob/d318bcc3afb9ea8782862f5e8cdb541afdc9dfa5/sdk/cliproxy/auth/selector.go#L1059-L1111)
- `SessionCache` 是进程内 map，默认最多 65,536 aliases；`GetAndRefresh` 是滑动 TTL。代码没有持久化接口，因此 standalone 重启会丢失绑定。[缓存结构](https://github.com/router-for-me/CLIProxyAPI/blob/d318bcc3afb9ea8782862f5e8cdb541afdc9dfa5/sdk/cliproxy/auth/session_cache.go#L10-L59)、[滑动续期](https://github.com/router-for-me/CLIProxyAPI/blob/d318bcc3afb9ea8782862f5e8cdb541afdc9dfa5/sdk/cliproxy/auth/session_cache.go#L77-L130)
- request-scoped 错误会保留绑定；普通失败会删除当前 session/auth binding。429/5xx 会使当前 binding 被移除，后续 retry/call 可重新选账号。[OnResult](https://github.com/router-for-me/CLIProxyAPI/blob/d318bcc3afb9ea8782862f5e8cdb541afdc9dfa5/sdk/cliproxy/auth/selector.go#L1457-L1551)
- additional retry rounds 只适用于 403、408、429、500、502、503、504 和 transient transport；400 `invalid_encrypted_content` 不在其中。[retry 状态](https://github.com/router-for-me/CLIProxyAPI/blob/d318bcc3afb9ea8782862f5e8cdb541afdc9dfa5/sdk/cliproxy/auth/conductor_selection.go#L1455-L1475)、[配置说明](https://github.com/router-for-me/CLIProxyAPI/blob/d318bcc3afb9ea8782862f5e8cdb541afdc9dfa5/config.example.yaml#L171-L193)

## Responses 改写与密文处理

### 请求

Codex Responses translator 会强制：

- `stream: true`
- `store: false`
- `parallel_tool_calls: true`
- `include: ["reasoning.encrypted_content"]`

来源：[request translator](https://github.com/router-for-me/CLIProxyAPI/blob/d318bcc3afb9ea8782862f5e8cdb541afdc9dfa5/internal/translator/codex/openai/responses/codex_openai-responses_request.go#L15-L30)、[`include` 改写](https://github.com/router-for-me/CLIProxyAPI/blob/d318bcc3afb9ea8782862f5e8cdb541afdc9dfa5/internal/translator/codex/openai/responses/codex_openai-responses_request.go#L118-L147)。

HTTP executor 删除 `previous_response_id`，把完整 `input` 作为 stateless replay 发往上游；随后运行 signature sanitizer。[非流式路径](https://github.com/router-for-me/CLIProxyAPI/blob/d318bcc3afb9ea8782862f5e8cdb541afdc9dfa5/internal/runtime/executor/codex_executor_execute.go#L39-L85)、[流式路径](https://github.com/router-for-me/CLIProxyAPI/blob/d318bcc3afb9ea8782862f5e8cdb541afdc9dfa5/internal/runtime/executor/codex_executor_stream.go#L45-L95)。原请求的 `prompt_cache_key` 会写回上游 body，并同时作为 `Session-Id` header，但它只是缓存/会话提示，不携带密文来源凭证。[cache helper](https://github.com/router-for-me/CLIProxyAPI/blob/d318bcc3afb9ea8782862f5e8cdb541afdc9dfa5/internal/runtime/executor/codex_executor_request.go#L95-L147)

### 响应

native Codex Responses 事件基本透传，只在需要时补 model；响应里没有 CLIProxyAPI 自己的“来源 auth/security-domain”包装，因此后续请求无法从 reasoning item 自身恢复原凭证身份。[response translator](https://github.com/router-for-me/CLIProxyAPI/blob/d318bcc3afb9ea8782862f5e8cdb541afdc9dfa5/internal/translator/codex/openai/responses/codex_openai-responses_response.go#L15-L39)

### Sanitizer 的能力边界

sanitizer 会：

- 将非 compat 的明文 `reasoning_text` 提升为 summary，并清空 `reasoning.content`；
- 检查 `encrypted_content` 是否为无首尾空白的 GPT reasoning signature；
- 删除格式非法/null/类型错误的 `encrypted_content`；
- `store:false` 时删除失去可用密文的 orphan reasoning `id`，避免 item lookup 错误。

来源：[完整分支](https://github.com/router-for-me/CLIProxyAPI/blob/d318bcc3afb9ea8782862f5e8cdb541afdc9dfa5/internal/runtime/executor/openai_responses_signature.go#L112-L239)。

它**不会**：

- 记录哪个 auth 生成该 blob；
- 判断两个 auth 是否属于同一 OpenAI organization/encryption boundary；
- 在 auth 切换时删除一个“格式合法但新账号不可解密”的 blob；
- 收到 400 后修改原请求并重试。

## Reasoning Replay Cache 不是跨账号恢复

- replay cache 只为 Claude source format 启用，不是 native OpenAI Responses 的通用状态库。[启用条件](https://github.com/router-for-me/CLIProxyAPI/blob/d318bcc3afb9ea8782862f5e8cdb541afdc9dfa5/internal/runtime/executor/codex_executor_reasoning.go#L40-L74)
- standalone 默认是进程内缓存，TTL 1 小时、最多 10,240 entries；Home mode 才可用 KV。[缓存常量与存储](https://github.com/router-for-me/CLIProxyAPI/blob/d318bcc3afb9ea8782862f5e8cdb541afdc9dfa5/internal/cache/codex_reasoning_replay_cache.go#L18-L112)
- `invalid_encrypted_content` 时只删除这份内部缓存项；客户端已经携带的 foreign reasoning item 仍在当前和后续请求里，所以不会自动恢复。[删除函数](https://github.com/router-for-me/CLIProxyAPI/blob/d318bcc3afb9ea8782862f5e8cdb541afdc9dfa5/internal/runtime/executor/codex_executor_reasoning.go#L817-L825)

## HTTP / WebSocket 场景行为

| 场景 | 实际行为 | 连续性判断 |
|---|---|---|
| 首轮、affinity 关闭 | 普通 RR/WRR/fill-first 选 auth；返回密文不带来源 marker | 后续轮可能换账号 |
| 首轮、affinity 开启 | 从 session/thread/`prompt_cache_key` 等提取 ID，冷选 auth 并绑定 | 健康期内通常稳定 |
| 后续轮、绑定 auth 可用 | 命中同 auth，滑动刷新 TTL | 通常无损 |
| TTL 过期/进程重启 | 绑定不存在，按普通路由重新选 auth | 可能跨安全域失败 |
| 绑定 auth 429/5xx/不可用 | 清除或绕过旧绑定，fallback 到新 auth 并改绑 | 旧密文可能立即 400 |
| 新 auth 返回 `invalid_encrypted_content` | 映射为 request fault 400，停止轮换；只清内部 replay cache | 会话仍 poisoned，客户端需改历史 |
| 多账号同模型 | affinity 健康时固定具体 auth；失效后仍可在同模型账号间切换 | 模型相同不代表密文安全域相同 |
| WebSocket 已建立 | downstream handler 按 provider 保存 `pinnedAuthID`，请求通过 pinned auth 执行 | socket 生命周期内更强固定 |
| WebSocket pinned auth 401/429 | 不在原连接静默换账号；关闭并要求新 socket 做 full replay | 不是无损透明恢复 |

WebSocket 证据：handler 保存 per-provider pinned auth 并注入执行上下文，[pin 状态](https://github.com/router-for-me/CLIProxyAPI/blob/d318bcc3afb9ea8782862f5e8cdb541afdc9dfa5/sdk/api/handlers/openai/openai_responses_websocket.go#L353-L413)、[执行与 replay close](https://github.com/router-for-me/CLIProxyAPI/blob/d318bcc3afb9ea8782862f5e8cdb541afdc9dfa5/sdk/api/handlers/openai/openai_responses_websocket.go#L690-L809)。只把 401/429 视为 pinned-auth replay trigger，400 不触发，[判定函数](https://github.com/router-for-me/CLIProxyAPI/blob/d318bcc3afb9ea8782862f5e8cdb541afdc9dfa5/sdk/api/handlers/openai/openai_responses_websocket_forward.go#L345-L381)。native Codex duplex 明确不 redial、不重新选 credential、不 replay；只有首帧 bootstrap 尚未消费 follow-up input 时才可能由 conductor 换凭证重试。[duplex contract](https://github.com/router-for-me/CLIProxyAPI/blob/d318bcc3afb9ea8782862f5e8cdb541afdc9dfa5/internal/runtime/executor/codex_websockets_duplex.go#L23-L37)、[bootstrap 边界](https://github.com/router-for-me/CLIProxyAPI/blob/d318bcc3afb9ea8782862f5e8cdb541afdc9dfa5/internal/runtime/executor/codex_websockets_duplex.go#L98-L100)。

## 官方 Issues 与检索审计

按 exact terms `invalid_encrypted_content`、`encrypted_content`、`reasoning`、`prompt_cache_key`、`session affinity`、`sticky`、`account switch`、`codex`、`responses`，以及近义词 `thinking_signature_invalid`、`could not be decrypted`、`provider switch`、`auth unavailable`、`failover`、`fallback` 检索官方 Issues/PR 和源码。宽泛词结果很多，和本问题直接相关的主证据为：

- [Issue #6420](https://github.com/router-for-me/CLIProxyAPI/issues/6420)：429 failover 或 affinity expiry 后把旧账号 encrypted reasoning 发给新账号；closed as `not planned`。维护者要求 API/subscription、不同 org 分池。[维护者结论](https://github.com/router-for-me/CLIProxyAPI/issues/6420#issuecomment-6017464221)
- [Issue #1793](https://github.com/router-for-me/CLIProxyAPI/issues/1793)：OpenAI 切 Azure 后 `400 invalid_encrypted_content`。维护者确认密文绑定上游 account/provider；affinity 只能在原凭证仍可用时保持，同源不可用后客户端可能需要 fresh continuation。[维护者评论](https://github.com/router-for-me/CLIProxyAPI/issues/1793#issuecomment-4366472863)
- [Issue #3189](https://github.com/router-for-me/CLIProxyAPI/issues/3189)：route/auth 改变后会话 poisoned；报告日志中的 `cache hit but auth unavailable, reselected` 精确对应 selector 的自动改绑分支。[问题分析](https://github.com/router-for-me/CLIProxyAPI/issues/3189#issuecomment-4534355488)
- [PR #1796](https://github.com/router-for-me/CLIProxyAPI/pull/1796)、[#3546](https://github.com/router-for-me/CLIProxyAPI/pull/3546)、[#5220](https://github.com/router-for-me/CLIProxyAPI/pull/5220)：分别尝试 auth affinity + strip/retry、fail closed、provider-bound history normalization，均未合并。

## 与 LiteLLM、new-api 对比

以下对比使用各项目官方仓库的既有第一方调研基线，不把其行为误记为 CLIProxyAPI 官方行为。

| 项目 | encrypted-state 路由载体 | 原身份不可用 | TTL/重启 | 当前根本性 |
|---|---|---|---|---|
| CLIProxyAPI | 可选 `session -> auth.ID` 内存绑定；reasoning item 不带来源 | 自动改绑；合法 foreign blob 原样转发；无自动 strip/retry | 默认 1h 滑动 TTL；standalone 重启丢失 | 仅降低换号概率，不根治 |
| LiteLLM | 把来源 deployment ID 编入返回 item ID/密文 wrapper；下一轮从请求自描述 marker 解码 | 优先原 deployment/同 encryption-boundary peer；否则可 strip 密文、保留 summary 后有损继续 | encrypted affinity 本身无 Redis/TTL | 对已标记 item 更完整，但 marker、凭证轮换及混合来源仍有风险；见 [官方实现](https://github.com/BerriAI/litellm/blob/246c0e126b6ef09654af058a9f0cb966ad78c18d/litellm/router_utils/pre_call_checks/encrypted_content_affinity_check.py#L1-L74) |
| new-api | `prompt_cache_key -> channel_id` 的限时渠道亲和，不绑定具体 key/security domain | miss/失效后普通选渠；当前无已合并自动 strip/retry | 默认 3600 秒，依赖 affinity cache | 与 CLIProxyAPI 同类概率缓解；见 [selector 入口](https://github.com/QuantumNous/new-api/blob/93ea55f3c3102f7bc11cd005570c7965f0f22a03/middleware/distributor.go#L124-L129)、[affinity 实现](https://github.com/QuantumNous/new-api/blob/93ea55f3c3102f7bc11cd005570c7965f0f22a03/service/channel_affinity.go#L726-L755) |

CLIProxyAPI Issue #6420 自身也把 LiteLLM 的思路概括为：优先原 deployment，否则删除不可解密 reasoning 的 `encrypted_content` 和 item `id`、保留可读 summary；但维护者没有接受该实现进入 CLIProxyAPI。[Issue 正文](https://github.com/router-for-me/CLIProxyAPI/issues/6420)

## 可迁移设计

建议 new-api/OP 不直接复制 CLIProxyAPI 的短 TTL session affinity，而采用分层设计：

1. **encrypted-state affinity 优先于普通路由。** 在权重、优先级、`prompt_cache_key` affinity 和重试前先解析可信来源 marker。
2. **绑定具体安全域。** 至少绑定 `provider + channel + credential/key identity + generation`；仅 channel ID 不足以覆盖多 Key、多 org、多 subscription/API 混池。
3. **状态自描述或持久化。** 使用服务端签名/加密的 opaque routing token 随 reasoning item 返回，或使用多实例共享的 durable mapping；TTL 只能用于清理，不能成为正确性边界。
4. **默认 fail closed。** 原 security domain 不可用时，只允许已证明同域的 peer；否则返回明确可重试错误，保留原绑定，避免静默把 foreign ciphertext 发给新账号。
5. **显式有损恢复。** 可选策略删除 `encrypted_content` 和 provider-owned item `id`，保留 summary/可读 content；无可读内容时删除整个 reasoning item。必须标记 continuity degraded，并仅在首字节输出前重试。
6. **WebSocket 不静默迁移。** 已开始的 socket/session 出错后断开，要求客户端新连接并发送完整、已中性化历史；不要在增量 `previous_response_id` 链中原地换凭证。
7. **分池仍是立即可用的安全措施。** 在上述机制上线前，API key 与 subscription、不同 org、不能证明同 encryption boundary 的凭证必须拆池。

## 迁移风险

- **Marker 伪造与信息泄露**：不能把内部 auth/channel ID 明文 base64 后交给客户端；应使用 HMAC/AEAD，并校验租户、模型、版本和过期策略。
- **凭证轮换**：同一个逻辑 auth ID 换 key/org 后，旧密文仍可能失效；需要 credential generation/security-domain version。
- **多实例一致性**：进程内 cache 会在重启、滚动升级、负载均衡后失效；共享状态需处理 TTL、并发覆盖和灾备。
- **客户端可控 key 冲突**：`prompt_cache_key` 不是可信身份凭据，必须按租户/API token 隔离命名空间。
- **工具调用副作用**：strip 后自动重试可能重复 tool/hosted-tool 行为；只能在确认上游未产生可见输出或外部副作用前重试。
- **历史结构破坏**：function/custom tool call-output 必须保持配对；不能只机械删除 reasoning/id 后假设 transcript 始终安全。
- **有损恢复语义**：保留 summary 只能维持语义上下文，不能恢复模型私有 reasoning state；产品与审计必须明确标记降级。

## 最终判断

CLIProxyAPI 当前成熟之处是普通多凭证选择、优先级、cooldown/retry 和可选 session stickiness；但这些机制与“opaque encrypted reasoning 的来源身份”没有形成完整闭环。`v8.0.23` 与本次 `main` 都只能在原 auth 健康且绑定未丢失时维持连续性，**不能在原账号挂掉后跨不兼容账号无损恢复，也没有内建有损 strip-and-retry**。官方当前明确建议是按 API/subscription 与 organization 拆分路由池，而不是依赖自动 failover 修复密文状态。
