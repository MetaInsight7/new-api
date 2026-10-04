# Base 与二开最新版 Relay 对比

> 状态：分析文档，仅用于确定后续迁移范围；本文本身不代表已经完成任何代码迁移。
>
> 目标：以 Base `main` 为底座，评估二开最新版 `remotes/solve/main` 中 Codex、OpenAI、Claude、Gemini、Grok 相关 relay 能力，区分真正新增的能力、Base 已经具备的能力、Base 后续更优的实现，以及只应按需迁移的实现。

## 1. 结论先行

### 1.1 总体判断

二开最新版不是 Base 的“完整升级版”，而是一个时间点更早、在部分渠道上做了较深定制的分支。两边的改动方向不同：

- **Base 更适合作为底座**：当前已经使用 `relaykit`、统一的跨协议转换、统一的 Responses usage 累加器、较新的流式状态和计费处理，并且包含二开分支没有的后续主线能力。
- **二开在少数产品能力上更完整**：Codex Chat bridge、Codex 图像、Codex 指纹、Gemini TTS、Grok Subscription，以及若干上游异常边界处理。
- **不能按目录或整文件覆盖**：二开不少文件仍依赖旧的本地 DTO 和 `pkg/apicompat`；直接覆盖会退回 Base 已经淘汰的协议转换架构，并可能丢掉 Base 后续修复。
- **应按能力迁移**：先保留 Base 的 adaptor、`relaykit`、路由和计费边界，再把二开的独立能力改写或适配到 Base 当前接口上。

### 1.2 迁移决策矩阵

| Relay | 二开真正新增或更强的部分 | Base 已有或更优的部分 | 建议 |
|---|---|---|---|
| Codex | Chat → Responses bridge、`gpt-image-2` 图像生成/编辑、指纹收敛、部分 terminal failure 处理 | `relaykit`、Alpha Search、OAuth 401 自动刷新、较新的模型发现和请求安全 | **按能力迁移，优先级高** |
| OpenAI | Codex Responses 失败流专项修复、部分响应 ID/白标测试、即梦专用图片适配 | Responses/Chat/Realtime/Image/Audio 的较新架构、统一 usage 与计费、WebSocket、跨协议转换 | **不整体迁移，只验证并选择性吸收** |
| Claude | Fable 5 的 200 SSE 嵌入式错误探测、原生 web tool 保留细节、上游 response ID | `relaykit` 转换、OpenAI/Gemini/Responses 入口、thinking/reasoning/cache/billing | **以 Base 为主，局部补齐** |
| Gemini | OpenAI Audio Speech → Gemini TTS、PCM/WAV 输出、TTS usage；图片无真实 usage 时不合成 token 的策略 | Responses → Gemini、`relaykit`、原生/跨协议/Embedding/Imagen 路径 | **单独迁移 TTS，图片计费策略另行验证** |
| 原生 xAI/Grok | 没有可确认的独立产品能力；主要是旧 DTO/架构差异 | Base 已有相同的 Chat、Search、Reasoning、Image、Responses 能力，且实现较新 | **不迁移** |
| Grok Subscription | 完整新增的 OAuth/PKCE 订阅账号渠道、CLI proxy、媒体、billing evidence、刷新与视频任务 | Base 完全没有对应渠道 | **独立大功能评估，不与原生 xAI 混合迁移** |

### 1.3 推荐优先级

1. **必须保留 Base 实现**：`relaykit`、Base 的 OpenAI Realtime、Gemini Responses、Codex Alpha Search、Codex 动态模型发现和 OAuth 刷新。
2. **第一批按功能迁移**：Codex Chat bridge、Codex 图像、Codex 指纹。
3. **第二批按实际使用情况迁移**：Gemini TTS；Claude Fable 5 错误探测；Claude 原生 web tool 的缺失边界。
4. **单独讨论计费策略**：Gemini 图片“真实 usage 才计 token”与 Base 当前合成 usage 的差异。
5. **暂不迁移**：原生 xAI relay 的旧实现差异；OpenAI 的即梦专用适配，除非当前确实使用该渠道。
6. **单独立项**：Grok Subscription。它不是普通的 Grok API relay，而是一整套第三方订阅账号接入系统。

## 2. 对比范围与方法

### 2.1 对比基线

| 项目 | 基线 |
|---|---|
| Base | `main`，`56758edf95ec162033a6b73b553ac789404f87c9`，2026-09-30 |
| 二开最新版 | `remotes/solve/main`，`762ce21c429a3aa9adc104bcd80eb7df3ad3bdbd`，2026-09-09 |
| 共同祖先 | `4ca47ee236fd5f09ec71d732fbe3d4270f03bec3`，2026-06-06 |
| 当前迁移工作分支 | `tokendazi_op` |

本次文档比较的是 **Base `main` 与用户提供的二开最新版 `remotes/solve/main`**。当前 `tokendazi_op` 中已经迁移的代码不作为判断二开原始能力的依据，避免把之前的迁移结果误认为 Base 或二开原生实现。

### 2.2 为什么不能只看一次 `git diff`

共同祖先之后，两边都继续产生了提交：

- Base `main` 继续吸收了 `relaykit`、Responses WebSocket、模型能力、计费和安全修复。
- `remotes/solve/main` 则继续增加 Codex 指纹、Grok Subscription、Gemini TTS 等定制能力。

因此，单纯把二开分支覆盖到 Base，或者只查看某一侧的三点 diff，都可能产生误判。本次判断采用以下方式：

1. 以共同祖先为参照，分别检查 Base 一侧和二开一侧的提交演进。
2. 对每个 relay 比较请求入口、上游 URL、认证、请求转换、响应转换、流式状态、usage/计费、错误处理和测试。
3. 对同名功能检查提交时间和当前代码，判断是二开新增、Base 后续已经实现，还是两边对同一问题的不同实现。
4. 只把“用户可用能力”与“实现架构差异”分开记录，不把旧 DTO 重写误认为产品能力。

### 2.3 结论标记

本文使用以下判断：

- **Base 优势**：Base 当前已有，或架构/安全/维护性明显更适合作为底座。
- **二开新增**：Base `main` 中没有，二开提供了实际用户能力。
- **重叠实现**：两边都支持，但实现路径不同；需要用 Base 代码重做或补测试，不能直接复制。
- **按需迁移**：只有在当前运营确实使用该渠道/能力时才值得开发。
- **不迁移**：二开没有形成可接受的新增能力，或者 Base 已经有等价/更优实现。

## 3. 总体架构差异

### 3.1 Base 的当前架构

Base 的 relay 已经将协议 DTO 和跨协议转换逐步抽到独立的 `relaykit` 模块：

- `relaykit/dto`：OpenAI、Claude、Gemini、Responses 等协议结构。
- `relaykit/relayconvert`：协议之间的请求、响应、tool、reasoning、usage 转换。
- Host 项目中的 `relay/channel/*`：负责渠道 URL、认证、HTTP/WebSocket 传输、渠道特有策略和计费上下文。
- `service` 与 `relay/common`：负责 usage、流式状态、渠道路由和结算。

这使得同一套 Responses/Chat/Claude/Gemini 转换可以被多个渠道复用，也减少了各个 provider 各自维护一套 DTO 的风险。

### 3.2 二开常见的实现方式

二开最新版仍保留较多旧结构：

- 直接依赖项目本地 `dto`。
- Codex Chat bridge 依赖 `pkg/apicompat` 做 Chat → Responses 转换。
- Claude、xAI 等部分 adaptor 保留较完整的本地转换函数。
- OpenAI、Gemini 的部分响应处理仍然在渠道目录内维护。

这些代码并不等于功能有问题，但不能直接覆盖到当前 Base：

- DTO 字段可能不完整，存在 typed round-trip 丢字段风险。
- 同一个协议的 usage 和 error 可能绕开 Base 统一状态机。
- 直接复制会重新引入 Base 已经解决的流式、计费或安全问题。

### 3.3 迁移总原则

后续每个能力独立提交时，应遵循：

1. 先在 Base 当前 adaptor 上确认入口和响应处理位置。
2. 复用 Base `relaykit` 与公共 usage/stream 组件。
3. 只搬运二开新增的行为、校验和测试场景。
4. 用 Base 的 DTO 和错误类型重写适配层，不复制旧文件的整体结构。
5. 对涉及计费、认证、重试和媒体下载的功能，必须增加失败路径测试。

## 4. Codex Relay

### 4.1 能力对比

| 能力/入口 | Base `main` | 二开最新版 | 判断 |
|---|---|---|---|
| `/v1/responses` | 支持，使用当前 Responses/`relaykit` 处理 | 支持，保留更多 Codex 特有字段与边界修复 | 两边重叠，Base 架构优先 |
| `/v1/responses/compact` | 支持 | 支持 | Base 保留，二开 compact 字段处理可作为测试参考 |
| `/v1/alpha/search` | 支持 | 未完整保留 | **Base 独有能力，必须保留** |
| `/v1/chat/completions` | adaptor 原本不支持 | Chat → Responses bridge | **二开新增，建议迁移** |
| Codex 图像生成 | adaptor 原本不支持 | 通过 Responses image generation 支持 `gpt-image-2` | **二开新增，建议迁移** |
| Codex 图像编辑 | adaptor 原本不支持 | 支持 multipart，并兼容 Playground JSON 附件路径 | 二开新增，按实际使用迁移 |
| Codex 指纹 | 无完整指纹收敛能力 | `off/device/session/full` 四种策略，注入 header/body/client metadata | **二开新增，建议迁移** |
| 动态模型发现 | 有官方客户端版本获取、模型请求和 401 自动刷新 | 有模型发现，但 401 更偏向直接报告过期 | **Base 逻辑更优** |
| `response.failed` | 当前 Base 有通用 Responses usage/terminal 状态处理 | 有 Codex 专项错误、重试恢复、fallback usage | 需要对齐行为，不覆盖 Base |

### 4.2 Base 的 Codex 实现

Base 当前重点在“安全、可维护的 Responses relay”：

- `relay/channel/codex/adaptor.go` 支持 Responses、Compact 和 Alpha Search 路由。
- 请求头使用 Codex OAuth JSON 中的 `access_token` 和 `account_id`。
- `service/codex_channel_models.go` 与 `service/codex_models.go` 负责动态模型发现。
- 动态模型发现会获取 Codex 官方客户端版本；上游返回 401 时可以刷新渠道 OAuth 凭证后再试一次。
- Base 的请求和响应处理已经接入当前 `relaykit`、统一的 Responses handler 和流式状态。
- Codex adaptor 明确拒绝 Chat、Audio、Embedding、Rerank 和 Image 等当前未实现入口，而不是静默走错误的兼容路径。

Base 的静态模型列表相对精简，包含当前 Base 维护的 Codex 文本模型与 `codex-auto-review`。动态发现和渠道模型配置才是实际路由时应优先考虑的来源，不应直接用二开的旧静态列表替换。

### 4.3 二开的 Chat bridge

二开新增 `relay/channel/codex/chat_bridge.go`，核心流程是：

1. 接收统一的 OpenAI Chat Completions 请求。
2. 使用 `ToCompatChatRequest` 和 `apicompat.ChatCompletionsToResponses` 转成 Responses 请求。
3. 补充 Codex 必需的 `instructions` 字段。
4. 清理 Codex 不接受的参数，例如非 Compact 路径上的 `store`、temperature、部分 token 参数。
5. 复用 Responses 上游地址。
6. 将 Responses 流或非流响应重新聚合成 Chat Completions 格式，并补 usage、finish reason、tool call 和 reasoning。

这是明显的用户能力扩展：很多 OpenAI SDK 只会调用 `/v1/chat/completions`，没有 Chat bridge 就无法使用 Codex 渠道。

但是二开实现依赖旧的 `dto`/`apicompat` 结构。特别是直接经过 typed DTO 时，可能丢失当前 Responses DTO 中的新字段；因此应在 Base 当前 Chat/Responses 转换层上实现，而不是直接复制整个 `chat_bridge.go`。

### 4.4 二开的图像能力

二开新增：

- `relay/channel/codex/image.go`
- `relay/channel/codex/image_usage.go`
- `gpt-image-2` 模型注册。

请求侧行为：

- 只接受 `gpt-image-*` 模型。
- 生成请求转为 Codex Responses 的 image generation tool。
- 编辑请求支持标准 multipart 图片，并兼容 Playground 传来的 JSON data URL 附件。
- 上游强制使用 SSE，adapter 再根据客户端期望输出格式处理。
- 对输入数量、字段、大小和 `response_format` 做校验。

响应和计费侧行为：

- 识别图片输出和图片 usage。
- 避免把大图片数据本身当作 token 计费依据。
- 在流式或终止失败时保留可结算的 usage。

该能力迁移时必须同时检查：

- Base 图片入口对 multipart/JSON 的分流。
- `relay/image_handler.go` 的客户端断开、重试和 quota 预扣。
- 图像按次价格与 token usage 是否叠加。
- `response.failed`、无图片结果和部分成功时的结算。

### 4.5 二开的 Codex 指纹

二开新增的指纹不是普通的请求 ID，而是为了让同一个渠道在多个 Codex 订阅请求中呈现一致的客户端身份。主要内容包括：

- `relay/channel/codex/fingerprint.go`：指纹生成、解析和 request body 改写。
- `relay/channel/codex/request_policy.go`：最终出口的 header 清洗和服务端身份覆盖。
- `Channel.CodexFingerprintSeed`：渠道级 seed。
- `CODEX_FINGERPRINT_DEPLOYMENT_NAMESPACE`：部署级命名空间。
- `off/device/session/full`：从不注入，到只注入设备/会话/完整身份的不同策略。
- 请求头中的 window/thread/session 信息。
- Responses body 的 `client_metadata`、turn metadata 等字段。

二开还做了以下安全加固：

- 拒绝或清洗重复 JSON key，避免客户端通过重复字段绕过策略。
- 限制 JSON 深度和请求体大小。
- 只在可信模式下保留 turn metadata。
- 最终 egress 阶段删除客户端伪造的 Codex identity header，再写入服务端值。

该能力有实际价值，但不能把“指纹”理解为普通随机 UUID：它会影响上游对同一账号的身份识别。迁移必须保留关闭开关、按渠道隔离、并发一致性和失败回滚语义，不能只复制几个 header。

### 4.6 401、流式失败和 usage 差异

二开在 2026-08-11 左右集中处理了 Codex Responses terminal failure：

- 识别 `response.failed` 和 `response.incomplete`。
- 读取 terminal event 中可能存在的 usage。
- upstream 在客户端尚未收到任何数据时失败，可以恢复本次 stream 状态并让渠道重试。
- 已经向客户端写出内容后失败，则不能再透明重试，需保留错误和已产生 usage。
- terminal event 没有 usage 时，对已经产生的文本、tool argument 或 reasoning 做有界 fallback 估算。
- 重试前恢复 response header、event-stream 状态和计费上下文。

Base 当前已经有通用 `ResponsesUsageAccumulator`、流式结束状态和失败事件处理，因此迁移时需要先做行为对照：

- Base 是否已经覆盖同一场景。
- Base 的通用逻辑是否会对 Codex 产生错误重试。
- `response.failed` 在写出首字节前后是否遵守同一计费规则。
- fallback 是否与用户此前确定的“确实发生消耗才扣费”原则冲突。

推荐把二开专项测试迁移为 Base 的回归测试，而不是直接覆盖 Base 的 `relay/channel/openai/relay_responses.go`。

### 4.7 Codex 迁移建议

建议拆成以下独立提交：

1. **支持 Codex Chat Completions 到 Responses 的双向桥接**。
2. **支持 Codex `gpt-image-2` 生成与编辑**。
3. **增加 Codex 可配置指纹收敛及最终出口清洗**。
4. **补齐 Codex terminal failure 的重试、错误和 usage 回归测试**。

必须保留 Base 的：

- Alpha Search。
- 动态模型发现和 OAuth 401 自动刷新。
- 当前 `relaykit` DTO 和 Responses handler。
- Base 的 SSRF、响应体大小、请求头和媒体安全限制。

相关代码和提交：

- Base：`relay/channel/codex/adaptor.go`、`service/codex_channel_models.go`、`service/codex_models.go`。
- 二开：`relay/channel/codex/chat_bridge.go`、`image.go`、`image_usage.go`、`fingerprint.go`、`request_policy.go`。
- Chat bridge：`f66a193a4`、`f15053e0f`、`9df670074`。
- 图像：`6b34f941c`、`c1db4787c`、`4e1bd44f9`、`7c39cf6fb`。
- 指纹：`af6078d72`、`173ad6e45`、`aae51578a`、`92471bee1`。
- terminal failure：`3ed7b3f35`、`702918e4f`、`63f775a96`、`9456ec8c6`。

## 5. OpenAI Relay

### 5.1 能力面

| 能力 | Base `main` | 二开最新版 | 判断 |
|---|---|---|---|
| Chat Completions | 支持 | 支持 | 两边重叠 |
| Responses | 支持 | 支持 | Base 当前实现更现代 |
| Responses Compact | 支持 | 支持 | 保留 Base 入口和状态处理 |
| Realtime WebSocket | 支持，Base 有独立 `relay_realtime.go` | 仍有 adaptor 路由和处理，但代码布局不同 | Base 优先，不要用二开文件覆盖 |
| Image generation/edit | 支持流式、非流式和图片计费 | 支持，但部分响应/图片处理被合并到旧文件 | Base 优先 |
| Audio | 支持 TTS/STT | 支持 | Base 优先 |
| Claude/Gemini/Responses 转换 | 使用 `relaykit` 和共享服务 | 更多本地转换逻辑 | Base 优先 |
| Built-in tools billing | 有统一处理 | 有部分专项修复 | 迁移测试，不迁移整文件 |

### 5.2 Base 的优势

Base `main` 在共同祖先之后继续完成了多项结构性更新：

- `relaykit` 成为请求和响应转换的共享层。
- Responses usage 使用统一累加器，处理 completed、incomplete、failed、取消和流中断。
- built-in tool 调用、图片生成调用和 cache usage 进入统一计费上下文。
- 增加 Responses WebSocket relay。
- 保留独立的 Realtime 双向 WebSocket 处理与增量计费。
- 对模型能力、reasoning effort、hosted tools、tool result media 做了更细的规范化。
- 处理 SGLang `created_at` 等兼容性问题，并持续修正图片 usage 和流中断。

这意味着二开的 `relay/channel/openai` 目录即使新增了不少测试，也不能视为可直接替换的新版。直接替换会删除或回退 Base 的 `relay_realtime.go`、图片响应处理、usage 辅助和 Responses 跨协议入口等文件布局或能力。

### 5.3 二开值得检查的部分

#### 5.3.1 Codex Responses terminal failure

二开在 OpenAI Responses handler 中增加了 Codex 专项分支：

- `response.failed` 到统一 API error 的映射。
- 没有输出数据时允许上游渠道重试。
- 已输出数据时禁止透明重试，并保留 usage。
- terminal usage 缺失时对已收到的文本和工具参数做有界 fallback。
- retry 前恢复 header 和 stream attempt state。

Base 当前有通用的 terminal event 和 usage accumulator，但需要针对 Codex 做一次回归对照。若 Base 的通用实现已经满足行为，应只移植测试；若有差异，应在 Base 的当前层次补一个 Codex policy，而不是把二开的旧 handler 整体搬过来。

#### 5.3.2 Realtime GA/preview header

二开和 Base 都已经处理 OpenAI Realtime GA 与 preview 模型的差异：

- GA 模型不能携带已经下线的 beta 标识。
- `-realtime-preview` 模型仍需要旧 beta header/protocol。
- WebSocket `Sec-WebSocket-Protocol` 与 HTTP header 要保持一致。

这不是二开需要重复迁移的功能。以 Base `relay/channel/openai/adaptor.go` 当前实现和 Base 的 WebSocket relay 为准。

#### 5.3.3 上游 response ID 与白标

二开增加了若干 response ID 捕获和白标测试，目标是：

- 记录上游 Responses 或 Chat response ID。
- 不把上游品牌、内部错误细节或敏感 header 透传给用户。
- 保持公开 response 的模型、ID 和 error 结构稳定。

Base 已有 request-boundary、response model mismatch、upstream response ID 和错误封装能力。应将二开的测试场景转写到 Base，而不是复制旧的 response scrubber。

#### 5.3.4 即梦图片适配

二开新增 `relay/channel/openai/jimeng_zhizinan_image.go`，它是特定即梦反代服务的请求/响应适配，不是 OpenAI relay 的通用能力。只有当前确实部署该渠道时才值得迁移；迁移时应放在即梦渠道边界，不应污染 OpenAI 通用 adaptor。

### 5.4 OpenAI 迁移结论

- 不迁移二开的整个 `relay/channel/openai` 目录。
- 先运行 Base 当前 OpenAI、Responses、Realtime、图片和计费测试。
- 对二开 Codex terminal failure、upstream ID、白标边界做差异测试。
- 若行为确实缺失，按“单个 policy/测试/渠道适配”提交。
- 即梦图片列为按需功能。

相关代码和提交：

- Base：`relay/channel/openai/adaptor.go`、`relay-openai.go`、`relay_responses.go`、`relay_realtime.go`、`relay_responses_compact.go`、`usage.go`。
- Base 结构性更新：`86ac0f774`、`9fe0457ee`、`0ed497f06`、`335ebcff7`。
- 二开 Codex terminal failure：`3ed7b3f35`、`702918e4f`、`63f775a96`、`9456ec8c6`。
- 二开 response ID：`d1ee968a6`。
- 二开即梦图片：`5cdd3d57d`。

## 6. Claude Relay

### 6.1 能力对比

| 能力 | Base `main` | 二开最新版 | 判断 |
|---|---|---|---|
| 原生 `/v1/messages` | 支持 | 支持 | Base 作为底座 |
| OpenAI Chat → Claude | 通过 `service.ConvertRequest`/`relaykit` | 本地完整转换函数 | Base 架构优先 |
| Gemini → Claude | 支持 | 未完整保留同等入口 | **Base 优势** |
| OpenAI Responses → Claude | 支持 | 未完整保留同等入口 | **Base 优势** |
| thinking/adaptive thinking | 支持 | 支持 | Base 统一转换优先 |
| hosted tools、tool result media | 支持 | 有 web tool 专项保留 | 先比较语义 |
| Fable 5 200 SSE 嵌入式错误 | 当前未见同等专项 gate | 支持 | **二开新增，建议补齐** |
| 上游 response ID | Base 核心 handler 未见同等设置 | 流式/非流式捕获 | 可选择迁移 |

### 6.2 Base 的 Claude 架构

Base 的 `relay/channel/claude/relay-claude.go` 已经明显变薄，主要职责是：

- 调用 `relaykit/relayconvert` 完成 Chat、Claude、Gemini、Responses 之间的转换。
- 处理 Claude thinking、adaptive thinking、reasoning effort 和 model suffix。
- 处理 Claude stream state、message delta usage、cache usage 和 tool billing。
- 将 Claude 原生响应和 OpenAI/Responses 输出交给共享转换层。

Base 还保留了 `aws`、`ali`、`blockrun` 等适配器需要调用的 Claude 核心处理函数。修改 Claude 核心函数时，需要同时检查这些渠道。

### 6.3 二开的原生 web tool 处理

二开在 OpenAI → Claude 转换中强化了原生 web tool：

- 保留 `web_search_202*` 的带日期版本，而不是统一改成旧 shorthand。
- 保留 `web_fetch_202*` 及其限制字段。
- 客户端明确声明原生 web search 时，优先原生声明，不再重复注入 shorthand search。
- 只有 fetch、没有 search 时，仍可按 OpenAI `web_search_options` 规则补 search。
- 保留 `max_uses=0`、allowed/blocked domains、citations、allowed callers 等零值和限制信息。

Base `relaykit/dto/claude.go` 和转换层已经具备 Claude web tool 类型与处理能力，所以二开逻辑不能直接复制。迁移前需要逐个运行以下契约：

1. dated web search 原样保留。
2. dated web fetch 原样保留。
3. native search 覆盖 shorthand，不重复计费。
4. fetch 不错误地抑制 shorthand search。
5. 普通 function 与 native web tool 共存。
6. `max_uses=0` 等显式零值不被 `omitempty` 逻辑错误覆盖。

如果 Base 已满足这些契约，只迁移测试或不做代码变更。

### 6.4 Fable 5 的流式错误 gate

二开处理了一类容易导致错误计费和错误重试的上游行为：HTTP 状态为 200，但 SSE 内容里实际返回：

```text
[Error: The model returned the following errors: ... prompt is too long ...]
```

实现特征：

- 在响应尚未提交给客户端前观察有限数量的 SSE event。
- 限制预检事件数和累计字节数，避免正常前导内容被无限等待。
- 识别错误标记跨多个 delta 的情况。
- 在 header/首字节提交前返回可分类的 relay error。
- 区分可重试的临时错误和客户端请求校验错误。

这是值得迁移的健壮性能力，但应嵌入 Base 当前的 stream commit/`StreamStatus` 机制，不能把二开旧 handler 直接复制进来。尤其要验证：

- 不会延迟正常 Claude 流的首 token 过久。
- 错误没有写给客户端之前，可以正确切换渠道。
- 已经写出真实内容后，不会错误地重复请求。
- 该错误的 usage/预扣/结算符合“未真正产生内容不扣费”的规则。

### 6.5 上游 response ID

二开在 Claude `message_start` 和非流式 response 中写入 `common.UpstreamResponseIdKey`，用于日志和请求关联。Base 已有上游 ID 的公共能力，但 Claude 当前核心路径不一定覆盖所有事件。可作为一个小的独立补丁评估，优先级低于 Fable gate。

### 6.6 Claude 迁移建议

1. 先用 Base relaykit 转换层跑完 web tool 契约，不要复制二开完整 `RequestOpenAI2ClaudeMessage`。
2. 若有缺失，只补 `relayconvert` 的最小语义和测试。
3. 将 Fable 5 preflight 错误 gate 适配到 Base 的 stream commit boundary。
4. 需要日志关联时，再单独补 response ID。

相关代码和提交：

- Base：`relay/channel/claude/adaptor.go`、`relay/channel/claude/relay-claude.go`、`relaykit/relayconvert`、`relaykit/dto/claude.go`。
- 二开：`relay/channel/claude/relay-claude.go`、`stream_error_test.go`、`web_tool_conversion_test.go`、`web_tool_http_test.go`。
- Fable gate：`cfd724d56`。
- 原生 web tool：`4d2ad9cd5`。
- response ID：`d1ee968a6`。

## 7. Gemini Relay

### 7.1 能力对比

| 能力/入口 | Base `main` | 二开最新版 | 判断 |
|---|---|---|---|
| 原生 Gemini generateContent | 支持 | 支持 | Base 架构优先 |
| OpenAI Chat → Gemini | 支持 | 支持 | Base `relaykit` 优先 |
| Claude → Gemini | 支持 | 支持或经旧链路转换 | Base 优先 |
| OpenAI Responses → Gemini | 支持 | 当前版本没有同等实现/为 TODO | **Base 独有，必须保留** |
| Gemini → OpenAI Responses | 支持 | 未完整保留 | **Base 独有，必须保留** |
| Embedding | 支持 | 支持 | Base 优先 |
| Imagen | 支持 | 支持 | 计费策略需比较 |
| OpenAI `/v1/audio/speech` → Gemini TTS | 未实现 | 支持 | **二开新增，建议迁移** |

### 7.2 Base 的 Gemini 能力

Base `relay/channel/gemini` 当前使用 `relaykit` 与共享 stream conversion：

- 原生 Gemini 请求可保持原生格式返回。
- OpenAI Chat、Claude 和 Responses 可以转换到 Gemini generateContent。
- Gemini stream 可以转换为 OpenAI Chat 或 Responses stream。
- Imagen、Embedding 和 native embedding 有独立路由。
- usage metadata 可区分 prompt、tool use、cached、thoughts、image、audio、text 等模态。
- thinking budget、thinking level 和模型后缀在共享转换层处理。

二开分支中的 `relay/channel/gemini/relay_responses.go` 不存在，`ConvertOpenAIResponsesRequest` 也没有保留 Base 的完整实现。因此 Gemini 迁移绝不能以二开 adaptor 覆盖 Base。

### 7.3 二开 Gemini TTS

二开新增 `relay/channel/gemini/tts.go`，把统一 OpenAI Audio Speech 请求映射到 Gemini generateContent：

1. 输入 `input` 和可选 `instructions` 合并为 Gemini user text content。
2. 设置 `responseModalities=["AUDIO"]`。
3. 将 OpenAI voice 映射到 Gemini prebuilt voice：
   - `alloy` → `Kore`
   - `echo` → `Puck`
   - `fable` → `Charon`
   - `onyx` → `Fenrir`
   - `nova` → `Aoede`
   - `shimmer` → `Leda`
4. 对模型名含 `tts` 的请求在 adaptor 中进入 `GeminiTTSHandler`。
5. 从 Gemini response 的 inline data 中解码 base64 音频。
6. Gemini 返回 PCM 时包装为 WAV；其他支持的音频类型按内容类型识别。
7. 通过真实 Gemini usage，或在无 usage 时按音频时长估算输出 usage。

该实现还明确限制了 Gemini 不支持的 OpenAI 参数：

- 不支持 SSE 音频流。
- 输出格式仅允许可处理的 WAV/音频格式。
- 数字 speed 只能为默认速度；非默认速度应通过 instructions 表达。
- 空 input、非法 speed、不可识别的音频内容会被拒绝。

这是独立且清晰的 provider 能力，适合在 Base 当前 `Adaptor.ConvertAudioRequest`/`DoResponse` 上新增，不需要改动现有 Chat/Responses 转换。

### 7.4 Gemini 图片 usage 的关键差异

二开新增 `relay/channel/gemini/usage_image.go`，配套修改图片 usage 行为：

- 如果上游提供 authoritative usage，使用上游 usage。
- 如果图片响应没有真实 usage，不再合成固定的 1400/258 token。
- 让图片按次价格或模型配置决定最终计费，而不是凭空增加 token。

Base 当前某些 Imagen 路径仍可能在缺少 usage 时按图片数量合成 token，例如流式图片缺 usage 时的合成值，以及非流式 Imagen 的固定 token 估算。这是一个重要的业务差异，尤其与项目已经确定的“真的花了才扣费”原则有关。

但不能直接把二开的 `usage_image.go` 覆盖过来。迁移前要确认：

1. Base 图片 quota 是否在请求开始前预扣。
2. 无 authoritative usage 时，图片按次价格是否已经能完整结算。
3. 上游返回图片但没有 token usage 时，使用零 token 是否会漏掉实际按次成本。
4. 上游失败、部分图片成功、客户端中断时应退回多少预扣。
5. OpenAI、Codex、Gemini 三类图片是否应采用同一计费政策。

建议把它单独做成“图片真实 usage 与按次计费策略对齐”功能，由 Base 的统一 billing/settlement 层实现，而不是只修改 Gemini handler。

### 7.5 Gemini 迁移建议

1. 保留 Base 的 Responses 双向转换和 `relaykit`。
2. 单独迁移 Gemini TTS 请求、响应、音频格式和 usage 测试。
3. 图片计费策略先出独立 spec，再决定是否迁移二开零合成 token 行为。
4. 为 TTS 增加模型列表、可用模型、Audio Speech 路由和计费回归测试。

相关代码和提交：

- Base：`relay/channel/gemini/adaptor.go`、`relay-gemini.go`、`relay-gemini-native.go`、`relay_responses.go`。
- 二开：`relay/channel/gemini/tts.go`、`tts_test.go`、`usage_image.go`、`usage_image_test.go`。
- TTS：`d12da036c`、`218ce5279`、`c1fca7bfe`。
- 图片 usage：`11e264699`。

## 8. Grok Relay

Grok 必须拆成两个完全不同的概念：

1. **原生 xAI/Grok relay**：使用用户配置的 xAI API key，渠道类型是 `ChannelTypeXai`。
2. **Grok Subscription relay**：使用第三方 xAI/Grok 订阅账号的 OAuth/PKCE 凭证，渠道类型是 `ChannelTypeGrokSubscription`。

把两者混为“Grok relay”会导致迁移范围和安全评估完全错误。

### 8.1 原生 xAI/Grok

两边都具备以下能力：

- Chat Completions。
- 模型名 `-search` 后缀转为 search parameters。
- `grok-3-mini-high/low` reasoning effort 处理。
- 图片生成/编辑相关入口。
- Responses 委托 OpenAI Responses handler。
- xAI usage 与流式响应转换。

二开 `relay/channel/xai` 的主要差异是依赖本地 `dto`，而 Base 已使用 `relaykit/dto`；Base 还保留了更近期的 thinking suffix 保留策略和统一 `ReasoningEffort` 设置。它没有形成值得单独迁移的用户功能。

结论：原生 xAI 以 Base 为准，不把二开的旧 xAI 文件复制到 Base。

### 8.2 Grok Subscription 的新增范围

Base `main` 没有 `relay/channel/groksubscription`。二开新增的是一套完整的渠道产品，包含：

#### 渠道与路由

- `ChannelTypeGrokSubscription = 113`。
- 独立 `APITypeGrokSubscription`。
- `relay/channel/groksubscription/adaptor.go`。
- 文本请求固定到 `https://cli-chat-proxy.grok.com/v1/responses`。
- 图片生成/编辑使用 `api.x.ai/v1/images/generations` 和 `api.x.ai/v1/images/edits`。
- Responses、Chat → Responses、Responses Compact 和 Claude 兼容入口。
- 独立的 Grok Subscription 视频 task adaptor。

#### CLI proxy 身份

文本请求除了 OAuth Bearer，还带有订阅 CLI 所需的服务端 headers：

- `X-XAI-Token-Auth: xai-grok-cli`。
- Grok CLI version。
- Grok client identifier。
- CLI user agent。
- 固定 host guard，禁止把 token 发往非允许 host。

#### OAuth/PKCE

- `auth.x.ai` 授权端点和 token 交换。
- PKCE verifier 不以明文落库，只存加密值。
- state 只存 hash，complete 后 consume，防重放。
- 支持未绑定渠道的 OAuth completion，后续再写回渠道。
- 支持 refresh token import、refresh、CAS 写回和并发 refresh lease。
- 认证状态包含 `pending`、`active`、`needs_reauth` 等投影。
- 错误信息脱敏，不向前端返回 token、verifier、lease 或上游敏感 body。

#### 文本、Compact 与刷新

- Chat 请求转换为 Grok Responses，再转换回客户端格式。
- Compact 走专门的 summary turn 构造和清洗。
- 401 时最多进行一次安全 refresh/replay。
- 使用 refresh lease 和版本 CAS，避免多节点同时覆盖 credential。
- 上游已经产生语义输出后不再盲目重试，避免重复计费或重复回答。

#### 媒体与订阅资格

- 图片生成与编辑有输入大小、数量和字段校验。
- 媒体访问前探测月度/周度 billing 和订阅 tier。
- 只有确认存在 paid evidence 才允许媒体能力。
- evidence 过期、探测失败或订阅资格不明确时 fail closed。
- 视频走独立异步提交、轮询、媒体代理和 token refresh。

#### 安全与可观测性

- 固定 host pin，降低凭证误发和 SSRF 风险。
- tool 使用 strict parser；Compact 对工具字段做清洗。
- 限制 upstream response body。
- 统一脱敏错误。
- 渠道详情只返回非秘密认证状态。
- 对 account status、package quota、media eligibility 有测试覆盖。

### 8.3 Grok Subscription 的成本和风险

它不是普通的“新增一个上游 URL”：

- 需要新增渠道类型、API 类型、路由、控制器、模型/状态表、加密凭证存储和前端 OAuth 对话框。
- 需要处理数据库 migration、secret key 配置、跨节点 refresh CAS 和 lease。
- 需要验证 xAI 订阅资格、CLI proxy 合同、媒体权限和视频异步任务。
- OAuth 和订阅账号存在账号封禁、服务条款、凭证生命周期等运营风险。
- 迁移后还要做完整的安全测试，不能只通过一个文本请求判断完成。

这套能力与“本项目的钱包/应用订阅功能”不是同一个概念：它是**第三方 Grok 订阅账号作为上游渠道**。是否迁移必须独立确认，不能因为用户暂时不做本项目订阅，就默认不需要它。

### 8.4 Grok 迁移建议

- 原生 xAI：不迁移二开旧实现，保持 Base。
- Grok Subscription：若当前没有实际订阅账号和运营需求，暂不迁移。
- 若确认需要，必须先建立独立 spec，拆分为：渠道注册、凭证加密、PKCE、文本 relay、刷新与并发、媒体 eligibility、图片、视频 task、前端管理、完整安全验证。
- 不建议把 Grok Subscription 做成普通“复制 relay 文件”的小提交。

相关代码和提交：

- 原生 xAI：`relay/channel/xai/adaptor.go`、`text.go`、`dto.go`。
- Subscription relay：`relay/channel/groksubscription/`。
- OAuth 控制器：`controller/grok_auth.go`。
- 状态和凭证：`model/grok_auth_flow.go`、`model/grok_channel_state.go`、`service/grok_credential_cipher.go`。
- 图片/视频：`relay/channel/groksubscription/image.go`、`relay/channel/task/groksubscription/`、`controller/video_proxy_grok_subscription.go`。
- 文本入口：`72e04ef0d`、`5f3c998ce`、`d86598e9a`、`21558fd8a`。
- OAuth：`d5dc37c0a`、`0dde9f4ad`、`51dc4149c`、`76fc600ee`。
- 媒体：`599defdea`、`1acbb3cd8`、`1eda71aef`。

## 9. 最终迁移清单

### 9.1 必须避免回退的 Base 能力

- [ ] `relaykit/dto` 和 `relaykit/relayconvert`。
- [ ] Base OpenAI Realtime WebSocket 与 GA/preview header 处理。
- [ ] Base Gemini Responses 双向转换。
- [ ] Base Codex Alpha Search。
- [ ] Base Codex 动态模型发现和 OAuth 401 自动刷新。
- [ ] Base 统一 Responses usage accumulator 和 stream status。
- [ ] Base 图片、tool、cache usage 的统一结算边界。
- [ ] Base 的 SSRF、请求体/响应体大小和媒体安全校验。

### 9.2 建议按独立 commit 开发的能力

- [ ] 支持 Codex Chat Completions → Responses → Chat。
- [ ] 支持 Codex `gpt-image-2` 生成/编辑。
- [ ] 增加 Codex 指纹收敛、seed、开关和 egress 清洗。
- [ ] 补齐 Codex terminal failure 回归行为。
- [ ] 增加 Gemini TTS。
- [ ] 增加 Claude Fable 5 嵌入式 SSE 错误 gate。
- [ ] 根据 Base web tool 契约补齐 Claude dated native tools。
- [ ] 评估并单独实现图片真实 usage/按次计费策略。

### 9.3 暂不迁移或按需迁移

- [ ] 原生 xAI/Grok 旧 adaptor：不迁移。
- [ ] OpenAI 即梦图片：只有使用该渠道时迁移。
- [ ] OpenAI 白标/response ID：先用 Base 现有能力和测试验证，缺失再补。
- [ ] Grok Subscription：独立立项，不并入普通 Grok relay 迁移。

## 10. 后续验证标准

任何一个功能进入实现前，应至少准备以下验证：

### 协议和路由

- OpenAI Chat、Responses、Compact、Realtime 的入口和上游路径。
- Claude 原生 Messages、OpenAI/Gemini/Responses 转换。
- Gemini 原生、Chat、Responses、Embedding、Imagen、Audio Speech。
- Codex Responses、Compact、Alpha Search、Chat bridge、Image。
- xAI 原生与 Grok Subscription 的渠道类型不可混淆。

### 流式和错误

- 首个 event 之前失败是否可切换渠道。
- 已提交 header/数据之后失败是否禁止重复重试。
- terminal failure、incomplete、客户端断开和上游超时是否有不同处理。
- 200 SSE 内嵌错误是否能在 commit 前识别。
- 非 JSON、超大 body、重复 key、恶意 header 是否 fail closed。

### 计费

- authoritative usage 优先于估算。
- 缺少真实 usage 时不凭空增加 token，除非对应能力明确按次计费。
- 失败、部分成功、重试和客户端中断不能重复扣费。
- tool、cache、image、audio、reasoning usage 分项正确。
- 对 Codex/Gemini 图片单独检查预扣和最终结算。

### 安全

- OAuth token、refresh token、PKCE verifier、指纹 seed 不进入客户端响应和普通日志。
- host guard、代理、下载和媒体轮询不能产生 SSRF。
- 客户端不能覆盖服务端维护的认证、身份和 Codex 指纹 header。
- refresh 使用 lease/CAS，不能因并发请求互相覆盖凭证。

## 11. 文档结论

这次对比的核心结论不是“把二开最新版合并进 Base”，而是：

> **Base `main` 作为协议和计费底座，二开最新版作为功能需求来源；每个功能在 Base 当前架构上重新落位。**

当前最明确的可迁移能力是 Codex Chat bridge、Codex 图像、Codex 指纹和 Gemini TTS。Claude 的 Fable gate 值得补齐，但要先确认 Base 的 web tool/stream commit 语义。Gemini 图片 usage 是计费策略问题，应单独讨论。原生 xAI 不需要迁移，Grok Subscription 则是独立的大型渠道项目。

