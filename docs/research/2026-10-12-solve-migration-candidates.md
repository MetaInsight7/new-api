# `solve/main` 相对 `tokendazi_op` 的迁移候选研究

日期：2026-10-12
研究对象：`solve/main@762ce21c429a3aa9adc104bcd80eb7df3ad3bdbd`
当前分支：`tokendazi_op@3b76bf8836cadc705bf14348e2afa6ee21b8f1b3`
共同祖先：`4ca47ee236fd5f09ec71d732fbe3d4270f03bec3`

## 结论摘要

这不是按提交标题生成的清单。每项都至少核对了 `solve/main` final HEAD 的最终源码与测试，并与当前分支实现逐项比对；有设计文档、后续修复或下线记录时也一并核对。两个分支在共同祖先后都持续演进，因此以下建议均是“迁移行为与不变量”，不是整文件或整提交搬运。

最值得优先实施的不是新页面，而是三项会直接改变故障语义的修复：

1. **任务型请求的 `LocalError` 必须先于 HTTP 状态码终止重试**。当前分支仍会把本地 429/5xx 当作上游故障重试，这是已确认的现存缺陷。
2. **重试候选耗尽后保留最后一个真实上游错误**。当前分支维护了 `RelayInfo.LastError`，但最终仍可能返回合成的 `get_channel_failed` 500。
3. **流式首包看门狗**。当前普通流式超时会被注释、空行或 keep-alive 不断重置，无法约束“连接活着但永远没有首个有效事件”的上游。

随后建议迁移 MySQL I/O 超时、API Key 权限更新的 fail-closed 协议、模型级 Prometheus 指标、跨实例验证码、渠道测试告警和 Claude/Fable 流错误预检。通用渠道批量编辑、上游 response ID、SSE UTF-8 和白标清洗可作为独立小票。注册风控应拆分迁移，不应一次性照搬。

| 优先级 | 候选 | `solve/main` final HEAD | 当前分支 | 建议 |
| --- | --- | --- | --- | --- |
| P0 | 任务 `LocalError` 先行终止重试 | 仍存在并有回归测试 | **缺失，且当前顺序有误** | 立即修复 |
| P0 | 重试耗尽保留真实上游错误 | 仍存在并有 429 测试 | 缺失 | 立即迁移 |
| P0 | 流式首包看门狗 | 仍存在并接入指标 | 缺失 | 首批迁移 |
| P0 | MySQL DSN 超时与 flusher 生存性 | 仍存在并有测试 | 缺失 | 首批迁移 |
| P0/P1 | API Key 权限批量更新与 fail-closed 缓存协议 | 仍存在并有安全设计/测试 | **部分已有**：allowlist 与单条 mutation fence 已有；blacklist、批量更新、pending marker 缺失 | 适配迁移，不覆盖现有缓存代码 |
| P1 | 模型级 Prometheus 延迟/TTFT/错误指标 | 仍存在且有严格基数预算 | **部分已有**：渠道 Prometheus 和持久化模型健康已有，模型级直方图缺失 | 迁移指标层，不重复建看板 |
| P1 | 跨实例验证码 | 仍存在并有安全加固 | 缺失，仍为进程内 map | 多实例上线前迁移 |
| P1 | 定时渠道测试告警与分布式冷却 | 仍存在，原始设计已被后续实现增强 | 仅有 Codex 治理专用钉钉告警 | 分阶段迁移 |
| P1 | Claude/Fable 5 流式错误预检 gate | 仍存在并有大量回归测试 | 缺失 | 有对应上游时迁移 |
| P1 | Prometheus 无效鉴权探测限流 | 仍存在，最终只限失败请求 | 缺失 | 小改动迁移 |
| P1 | SSE UTF-8 Content-Type | 仍存在 | 缺失 | 小改动迁移 |
| P1/P2 | 请求类型错误脱敏与字段路径保护 | 仍存在并有回归测试 | 缺失 | 小改动迁移 |
| P1/P2 | 上游 response ID 全链路记录 | 仍存在 | 仅有 upstream request ID | 迁移 |
| P2 | 选中渠道通用批量编辑 | 仍存在且后续纳入并发/Codex 字段 | 缺失；当前 `PUT /api/channel/batch` 被 Codex 专用操作占用 | 重设计路由后迁移 |
| P2 | 注册风控套件 | 仍存在，多条路径均有测试 | 仅有基础白名单和 Turnstile | 拆票迁移 |
| P2 | 按渠道白标清洗成功响应 | 仍存在 | 仅有错误文案清洗 | 仅对外白标业务迁移 |
| P3 | 模型标签与展示权重排序 | 仍存在，硬编码徽标已删除 | tags 已有，display weight 缺失 | 低优先级 |

## 研究方法与判定口径

- **final HEAD 判定**：不以某个历史提交曾经存在为准；通过 `solve/main` 最终树中的符号、调用点和测试确认功能仍被使用。
- **当前差距判定**：不仅搜索同名符号，也检查当前分支是否存在等价行为。例如当前已有渠道 Prometheus，但没有 `newapi_model_request_duration_seconds` 等模型级指标；当前已有 API Key allowlist，但没有 final solve 的 blacklist 和权限更新 pending marker。
- **删除/TODO 判定**：核对删除提交、最终树和文档。历史实现若已从 final HEAD 删除，不列为迁移候选；源码明确写有未完成产品约束的功能列为条件项。
- **范围排除**：支付、钱包、订阅、营销活动、广告、SEO、官网、Recall、媒体商品化和特定供应商业务流不进入主清单。
- **报告位置**：当前仓库没有既有 `docs/research/` 约定；按用户指定路径新建该目录，并且本任务只写本报告文件。

## P0：任务 `LocalError` 必须先于状态码终止重试

### 证据与 final HEAD 状态

- 修复提交：`30a7b1142`（`Preserve token quota failures instead of reporting channel exhaustion`）。
- final 实现：`solve/main:controller/relay.go` 的 `shouldRetryTaskRelay` 在检查 429、307 和 5xx 之前先判断 `taskErr.LocalError`。
- final 测试：`solve/main:controller/relay_task_test.go` 的 `TestShouldRetryTaskRelayDoesNotRetryLocalErrorsRegardlessOfStatus` 覆盖本地 500、429、400；同文件还验证额度预扣失败不会触发任何上游调用。
- 该提交仍是 `solve/main` final HEAD 的祖先，测试和先行判断均仍存在。

### 当前分支差距

当前 `controller/relay.go` 的 `decideTaskRetry` 顺序相反：先把 429/307 和 5xx 判为可重试，之后才进入 `taskErr.LocalError`。因此本地计费、定价、校验或配额错误如果恰好使用 429/5xx，会错误地：

- 排除当前渠道并尝试其他渠道；
- 产生误导性的渠道失败记录；
- 最终可能被覆盖成“无可用渠道”。

当前测试只覆盖本地 403 会停止，并明确写成“只要状态规则没有强制重试”；没有覆盖本地 429/5xx，不能防住该缺陷。

### 收益、复杂度与风险

- **收益**：避免无意义上游调用、渠道误封和真实本地错误丢失；保证计费/配额错误与渠道选择解耦。
- **复杂度**：低。调整一个决策函数的优先级并补参数化测试。
- **风险**：必须确认所有 `LocalError` 构造点确实表示“换渠道也不会改善”。不要仅按状态码推断 `LocalError`。
- **迁移边界**：保留当前 request-policy 的 `PolicyDecision` 结构和审计记录，只改变决策优先级，不搬运 solve 的旧重试函数。

## P0：重试候选耗尽时保留最后一个真实上游错误

### 证据与 final HEAD 状态

- 修复提交：`399228a78`（`Preserve upstream error when retry exhausts channel tiers`）。
- final 实现：`solve/main:controller/relay.go` 的 `getChannel` 在候选返回空且 `info.LastError != nil` 时，返回该错误并追加 skip-retry，而不是制造新的 `get_channel_failed`。
- final 测试：`solve/main:controller/relay_error_passthrough_test.go` 明确验证最后一个 HTTP 429 的状态码、错误码和消息被保留。
- final HEAD 仍保留实现和测试；后续任务额度修复 `30a7b1142` 进一步强化了“本地错误不得被渠道耗尽覆盖”的同一原则。

### 当前分支差距

当前 `controller/relay.go` 已在每次 relay/task 失败后更新 `relayInfo.LastError`，但 `getChannel` 在 `channel == nil` 时仍直接返回“分组下模型的可用渠道不存在（retry）”的合成错误。真实的 429、401、5xx 或协议错误因此可能在最后一步丢失。

### 收益、复杂度与风险

- **收益**：LiteLLM、SDK 和调用方能根据真实 429/5xx 正确退避、降级或告警，不会把限流误判为网关 500。
- **复杂度**：低，但要覆盖当前 auto-group、跨优先级、并发满载和 request-policy 分支。
- **风险**：只应复用已经过现有错误清洗链处理的 `NewAPIError`，不得直接暴露原始上游 body；重试开始时必须清理前一 attempt 的上下文状态。
- **建议测试**：普通 relay 429、task 本地错误、auto 分组耗尽、并发槽位全满、完全没有候选但从未调用上游五类场景分别断言。

## P0：流式首包看门狗

### 证据与 final HEAD 状态

- 修复提交：`e8c9da301`（`add first-response watchdog for streaming requests`）。
- final 配置：`STREAMING_FIRST_RESPONSE_TIMEOUT`，默认 180 秒，`0` 禁用；定义于 `solve/main:constant/env.go`，由 `solve/main:common/init.go` 读取。
- final 实现：`solve/main:relay/helper/stream_scanner.go` 并行运行不可重置的首包 timer 与原有 chunk idle timer；收到首个真实 SSE data 后才停止首包 timer。
- final 测试：`solve/main:relay/helper/stream_scanner_test.go` 覆盖静默上游超时、首个 data 后停表和禁用配置；Claude/Fable 测试也覆盖 gate 与看门狗组合。
- final 指标：`solve/main:pkg/perf_metrics/metrics.go` 将 `first_response_timeout` 计入超时类别，说明该状态在最终观测链中仍被使用。

### 当前分支差距

当前 `relay/helper/stream_scanner.go` 只有 `STREAMING_TIMEOUT`，并在每次 `scanner.Scan()` 后重置 ticker。上游持续发送空行、SSE 注释或 keep-alive 时，连接可以永久保持“活跃”，却没有任何有效首包，继续占用 HTTP 连接和渠道并发槽。

### 收益、复杂度与风险

- **收益**：将“半开且无有效输出”的请求从负载均衡器级长超时缩短到可配置窗口，并在尚未提交客户端响应时允许安全切换渠道。
- **复杂度**：中。通用 scanner、stream end reason、Responses/Claude 映射、指标与测试都要改。
- **风险**：一旦已向客户端写入 ping 或响应头，不能再透明重试；必须以“首个有效协议事件”而非任意字节停止 timer；大上下文模型可能合法地超过默认值，应允许按部署覆盖。
- **迁移边界**：适配当前 `relaykit` 和现有 `StreamStatus`，不要覆盖当前 Responses 计费、WebSocket 或加密状态恢复逻辑。

## P0：MySQL I/O 超时与 batch flusher 生存性

### 证据与 final HEAD 状态

- 修复提交：`de981f966`（`defend batchUpdate against MySQL TCP half-open hang`）。
- final 实现：`solve/main:model/dsn_defaults.go` 使用 MySQL driver 解析 DSN，在用户未显式设置时补 `timeout=10s`、`readTimeout=30s`、`writeTimeout=10s`，并支持 `SQL_DIAL_TIMEOUT`、`SQL_READ_TIMEOUT`、`SQL_WRITE_TIMEOUT`。
- final 测试：`solve/main:model/dsn_defaults_test.go` 覆盖默认值、用户覆盖、环境变量和非法 DSN；`solve/main:model/batch_update_test.go` 覆盖 panic containment。
- final `solve/main:model/utils.go` 仍在 `batchUpdate` 外层 recover 并记录 stack。

### 当前分支差距

- `model/main.go` 只用字符串方式补 `parseTime=true`，没有连接、读、写超时。
- `model/utils.go` 的后台批量落库没有本地 recover。
- 当前批量落库结构已不同于 solve，不能复制整文件。

### 收益、复杂度与风险

- **收益**：避免 MySQL 半开 TCP 让额度/用量更新永久阻塞；避免一次 panic 终止本轮或 worker 的后续工作。
- **复杂度**：低到中。DSN 默认值是独立小改；flusher 保护需适配当前合并更新逻辑。
- **风险**：过短 `readTimeout` 会误伤合法慢查询；recover 只能保活，不能自动恢复已从内存 map 取出但尚未处理的数据。迁移时应同时明确失败重入或持久化策略，不能把 recover 当成数据可靠性保证。

## P0/P1：API Key 权限规则与 fail-closed 缓存更新

### 证据与 final HEAD 状态

- 基础模型访问提交：`7e95b9425`，最终提供统一的用户/Token 模型访问解析和可视化。
- 批量管理提交：`017c573e1`，把模型 allowlist 纳入既有 API Key 批量编辑。
- 安全加固提交：`77f96541c`，加入独立 blacklist、规则标准化/上限和 Redis pending marker。
- 设计：`solve/main:docs/superpowers/specs/2026-08-06-api-key-review-fixes-design.md`。
- final 实现：
  - `solve/main:model/token.go` 有 `ModelBlacklistEnabled` / `ModelBlacklist`、最多 512 条和 32 KiB 的批量规则上限；
  - `BatchUpdateTokens` 在事务中锁定并验证所有 token 归属；
  - `solve/main:model/token_cache.go` 在授权敏感更新前原子写 per-token pending marker、旋转 fill fence 并删除旧 hash；marker 存在时读取 fail-closed，禁止 DB fallback 与旧快照回填；
  - `PUT /api/token/batch` 仍存在于 final router。
- final 测试：`model/token_cache_test.go` 覆盖 pending 读取、stale fill 和 marker 生命周期；`model/token_validation_test.go` 覆盖规则去重和上限；`controller/token_test.go` 覆盖 allowlist/blacklist 批量更新及缓存失效。

### 当前分支已有与缺失

当前已经具备：

- API Key 的 `ModelLimitsEnabled` / `ModelLimits` allowlist；
- `/api/user/model-access`、统一模型访问解析和前端模型范围预览；
- 单条 Token mutation 前的 10 秒 HMAC fence，避免并发读者把旧 DB 快照回填 Redis。

当前缺失：

- 独立 blacklist 以及“deny 优先于 allow”的统一规则；
- `PUT /api/token/batch` 和批量组、配额、allowlist/blacklist 更新；
- 批量授权变更的 per-token owned pending marker；
- Redis 清理不确定时阻断授权读取的 fail-closed 语义；
- 批量规则的 512 条/32 KiB 服务端边界。

### 收益、复杂度与风险

- **收益**：防止权限收紧后的短窗口内旧缓存继续授权；能批量撤销某模型访问，而不必重建 allowlist；大规模 API Key 运维成本显著下降。
- **复杂度**：中高，涉及 schema、缓存 Lua、事务、middleware、API 和前端。
- **风险**：Redis 故障时 fail-closed 会临时拒绝请求；marker TTL 和所有权必须正确，否则可能长期锁死 token。当前分支的 mutation fence 是更新版本的本地设计，不能被 solve 旧缓存实现整体替换。
- **迁移边界**：保留当前 HMAC key、配额原子预扣和 auto-group 字段；先迁移 blacklist/规则规范化，再为批量授权更新增加 owned pending marker。

## P1：模型级 Prometheus 延迟、TTFT 与错误指标

### 证据与 final HEAD 状态

- 渠道级设计与实现：`docs/superpowers/specs/2026-07-13-channel-prometheus-health-design.md`、提交 `5c369ce16`。
- 模型级实现：`cad802e50`（`Expose trustworthy per-model latency and TTFT metrics`）。
- final 指标：
  - `newapi_model_request_duration_seconds`：最终成功请求的端到端延迟直方图；
  - `newapi_model_ttft_seconds`：成功流式请求 TTFT；
  - `newapi_model_stream_success_total`；
  - 固定错误分类和 dropped-sample 指标。
- final 基数保护：`solve/main:pkg/perf_metrics/prometheus_model.go` 默认最多 50 个活跃模型、1 小时 idle retirement，并与全局 series budget 协同；模型名之外不加入用户、Key、错误文本等标签。
- final 测试：`solve/main:pkg/perf_metrics/prometheus_test.go` 覆盖模型 admission 并发、50 模型上限、series budget、TTFT 完整组、drop 原因和输出顺序。

### 当前分支已有与缺失

当前已经有：

- `pkg/perf_metrics/channel_prometheus.go` 的渠道 attempt、渠道延迟、渠道 TTFT、渠道+模型请求/Token 指标；
- `RecordRelayResult` 的最终请求采样和持久化模型健康数据；
- 模型可用性与管理面板。

当前 `/metrics` 缺少上述 `newapi_model_*` 端到端直方图和 drop-health 指标。渠道 attempt 延迟不能替代模型最终体验：一次客户端请求可能先失败重试，再由另一渠道成功。

### 收益、复杂度与风险

- **收益**：直接回答“某模型经过重试后的真实 P95/P99 和 TTFT 是多少”，与现有渠道 attempt 视角互补。
- **复杂度**：中。可复用当前 `RecordRelayResult`，无需复制 solve 的整个 perf metrics 架构或重复创建模型健康页面。
- **风险**：Prometheus 基数和 scrape 输出大小。必须保留模型 admission cap、idle retirement、固定错误标签和全局 series limit；不能添加 group、channel、用户或动态错误文本到模型直方图。
- **明确排除**：历史 Redis 聚合方案后来被 `067dfe323` 用每实例 sidecar 抓取替代，不应迁移旧 Redis Prometheus rollup。

## P1：跨实例验证码存储

### 证据与 final HEAD 状态

- 提交：`9d8d84a93`；安全加固：`5585edb1d`。
- final 实现：`solve/main:common/verification.go` 在 Redis 可用时按 purpose 隔离存储验证码并设置 `VerificationValidMinutes` TTL；避免使用会在 debug 日志打印值的通用 Redis wrapper；Redis 未初始化时有 nil guard。
- final 测试：`solve/main:common/verification_test.go` 覆盖跨实例、purpose 隔离、TTL、Redis 宕机和 nil client。
- 该能力在 final HEAD 仍存在；同文件后续还增加了一次性邮件链接，但后者不是本候选的必要范围。

### 当前分支差距

当前 `common/verification.go` 完全使用进程内 map。多实例负载均衡下，发送验证码和验证请求落到不同节点就会失败。

### 收益、复杂度与风险

- **收益**：消除多实例随机验证码失败，是正式水平扩容前的基础条件。
- **复杂度**：中，核心实现小，但需要安全和故障策略测试。
- **风险**：solve 在 Redis 写失败时回退当前进程内存，只能保住单实例，可能让用户收到一个其他节点无法验证的验证码。多实例部署应明确选择 fail-closed、短时粘性或双写策略；Redis key 不应包含可直接枚举的明文邮箱，验证码不得进入 debug 日志。

## P1：定时渠道测试告警与分布式去重

### 证据与 final HEAD 状态

- 初始设计：`solve/main:docs/superpowers/specs/2026-06-02-dingtalk-channel-alert-design.md`。
- AI 摘要设计：`solve/main:docs/superpowers/specs/2026-07-02-dingtalk-channel-alert-ai-summary-design.md`。
- 实现与加固链：`1102f1679`、`2586b0cdb`、`802723477`、`f272e47c9`、`206dd156f`、`aee3f6540`、`b537c063e`、`94d88ee84`、`4408818b3`、`86def1282`。
- final 实现：
  - `controller/channel-test.go` 只在定时测试路径收集并批量发送；
  - `service/dingtalk_alert.go` 做脱敏、签名、超时、最多五条一批和错误处理；
  - `model/dingtalk_alert_cooldown.go` 使用 DB reservation/commit/rollback 实现跨实例冷却；发送失败回滚 reservation，发送成功后的 commit 失败只告警、不重复发送；
  - `monitor_setting` 支持 allowed/ignored channel types；
  - AI 中文摘要失败时仍发送原始告警。
- final 测试：`service/dingtalk_alert_test.go` 覆盖凭据脱敏、网络错误 URL 清洗、超时、批量、AI fallback、跨实例冷却和 commit/rollback；`model/dingtalk_alert_cooldown_test.go` 覆盖数据库状态机。

初始 spec 写的是“进程内 cooldown”，但 final 实现已经演进为 DB 分布式 reservation；应以 final 代码和测试为准。

### 当前分支已有与缺失

当前有 Codex model governance 专用钉钉告警，但没有通用的定时渠道测试失败告警、渠道类型过滤或跨实例通用告警冷却。当前 `MonitorSetting` 主要包含测试开关、模式和并发数。

### 收益、复杂度与风险

- **收益**：在渠道被自动禁用之前或即使未禁用时及时暴露异常；多节点下不重复轰炸。
- **复杂度**：中高，涉及设置、前后端、sender、数据库表和定时测试整合。
- **风险**：webhook SSRF、secret/渠道 key 泄漏、告警风暴、DB 时间一致性。必须复用当前 URL 安全校验与 secret 存储；不要覆盖当前更丰富的渠道测试模式和并发配置。
- **建议阶段**：一期仅原始告警、类型过滤、分布式冷却；AI 摘要二期可选。

## P1：Claude/Fable 5 流式错误预检 gate

### 证据与 final HEAD 状态

- 提交：`a67dcf68f`、`cfd724d56`。
- final 实现：`solve/main:relay/channel/claude/relay-claude.go` 仅对 Fable 5 启用预检；识别正式 error event，以及被包装成 text delta 的 prompt-limit 错误。
- 通用 gate：`solve/main:relay/helper/stream_scanner.go` 的 `StreamScannerHandlerWithDataGateAndEnd` 在确认成功前不提交 SSE，并按序回放缓冲事件。
- 边界：最少观察 4 个事件，最多 32 个事件或 1 MiB；到自然 EOF 时由 gate-end 决定释放或报错。
- final 测试：`relay/channel/claude/stream_error_test.go` 和 `relay/helper/stream_scanner_test.go` 覆盖正式错误、拆分文本错误、正常流、EOF、取消和有界缓冲。

### 当前分支差距

当前 `ClaudeStreamHandler` 直接调用普通 `StreamScannerHandler`。若上游先返回 HTTP 200/SSE，再在首批事件中表达校验失败，网关可能已经向客户端提交 200，无法再返回正确 HTTP 400。

### 收益、复杂度与风险

- **收益**：保留真实错误语义，避免“200 + 错误文本”被当作正常生成内容和正常计费。
- **复杂度**：高，必须适配当前 `relaykit`，不能覆盖 solve 的旧 DTO/转换层。
- **风险**：增加首包延迟和最多 1 MiB 内存；错误文本匹配过宽会误杀正常内容。必须保持 provider/model opt-in 和有界 gate。

## P1：Prometheus 无效鉴权探测限流

### 证据与 final HEAD 状态

- 初始提交 `2fbfdd66f` 曾对整个 `/metrics` 做限流；该方案后来被 `d32460331` 明确修正为**只在鉴权失败分支限流**，避免同 NAT 下的恶意探测耗尽合法 scrape 配额。
- final 实现：`solve/main:middleware/prometheus_metrics_auth.go` 在 bearer 缺失或错误时调用 `PrometheusMetricsRateLimit`；合法 scrape 不消耗该 bucket。
- final 测试：`middleware/prometheus_metrics_auth_test.go` 的 `TestPrometheusMetricsAuthFailureRateLimit` 和配置 clamp 测试。

### 当前分支差距

当前已有常量时间 token 比较和 loopback 免鉴权，但无效 bearer 请求没有独立限流；token 未配置也统一返回 401，而 solve final 返回 503，便于运维区分配置错误与凭据错误。

### 收益、复杂度与风险

- **收益**：抑制公网 token 探测和日志噪声，不影响正常监控。
- **复杂度**：低。可直接复用当前更完善的 fixed-window `rateLimitFactory` 和 `Retry-After`。
- **风险**：不能重新采用早期“所有 scrape 共用 pre-auth bucket”的方案；Redis client 尚未初始化时需回退内存 limiter。

## P1：SSE UTF-8 Content-Type 标准化

### 证据与 final HEAD 状态

- 提交：`b58195c20`。
- final 实现将通用 helper、自定义 event 和 Ollama 流改为 `text/event-stream; charset=utf-8`。
- 测试：`common/custom-event_test.go`、`relay/helper/common_test.go`，并将原来精确等于 `text/event-stream` 的测试改为前缀判断。

### 当前分支差距

当前 `common/custom-event.go` 和 `relay/helper/common.go` 仍写纯 `text/event-stream`。

### 收益、复杂度与风险

- **收益**：减少中文、日文及供应商 Unicode 增量被代理或客户端错误解码的兼容问题。
- **复杂度**：低。
- **风险**：主要是已有测试或客户端错误地做 Content-Type 精确字符串比较；应统一改为媒体类型解析或前缀判断。

## P1/P2：请求类型错误脱敏与字段路径保护

### 证据与 final HEAD 状态

- 提交链：`3f90b7234`、`1c00546ec`、`7b96e93ab`、`ec2ad10fb`、`e48845c22`。
- final `solve/main:common/gin.go` 将 `json.UnmarshalTypeError` 改写为只包含 JSON 字段、期望类型与收到的类型，不再向客户端暴露 `GeneralOpenAIRequest` 等 Go 结构名。
- final `solve/main:common/str.go` 还收窄了裸域名脱敏：仅识别保守 TLD 集，并跳过含下划线、纯数字段的 dotted path，避免把 `messages.0.content`、`prompt_cache_key` 等字段路径误判成域名。
- final 测试覆盖内存与磁盘请求体、真实 `UnmarshalBodyReusable` 路径、字段路径、ccTLD 和 URL/IP/API key 脱敏。

### 当前分支差距

当前 `common.UnmarshalBodyReusable` 直接返回底层 JSON 类型错误；`controller/relay.go` 会把 invalid-request 文案作为用户可见 400 透传，因此 Go 结构名仍可能出现在响应中。当前 `relaykit/relayconvert/kitutil/mask.go` 的通用裸域名正则也没有字段路径保护，可能把 dotted path 错误掩码。

### 收益、复杂度与风险

- **收益**：减少内部实现名泄露，并让参数类型错误保持可读；避免排障日志和用户错误文案中的字段路径被脱敏器破坏。
- **复杂度**：低，适合独立小票。
- **风险**：裸域名与字段路径无法仅靠结构完美区分，应保留 final 的保守策略；完整 URL、IP 和凭据仍必须继续脱敏。

## P1/P2：上游 response ID 全链路追踪

### 证据与 final HEAD 状态

- 提交：`d1ee968a6`。
- final 捕获位置包括 OpenAI Responses、Chat-via-Responses、OpenAI Chat、Claude 和 BlockRun；写入日志 `other.upstream_response_id`，并在用量交易和详情页展示。
- final 在每次重试前删除 context 中旧 response ID，避免前一渠道的 ID 污染后一 attempt。
- 测试：`controller/relay_upstream_id_test.go`、`model/log_upstream_response_id_test.go`、`relay/channel/openai/responses_upstream_id_test.go`、Claude/BlockRun 测试。

### 当前分支差距

当前已经完整捕获、索引和筛选 `upstream_request_id` 响应头，但没有 `UpstreamResponseIdKey`，无法用 `resp_*`、`chatcmpl_*` 或 `msg_*` 反查网关日志。

### 收益、复杂度与风险

- **收益**：供应商工单和协议排障常只提供响应 body ID；补齐后可从上游 ID 反查渠道、请求和计费记录。
- **复杂度**：中，涉及多适配器、日志 metadata 和前端。
- **风险**：必须 attempt-scoped 清理；部分供应商 ID 可能包含租户信息，不应默认加入高基数 Prometheus label。是否建数据库索引应由真实查询量决定。

## P2：按选中渠道做通用批量编辑

### 证据与 final HEAD 状态

- 设计：`solve/main:docs/superpowers/specs/2026-06-29-channel-batch-edit-by-ids-design.md`。文档头写“待编写实现计划”，但该状态已过时；实现随后由 `b57b836d7`、`427fdc8b3`、`2d08a80b0`、`67a290364`、`e0c20c683` 完成。
- final 实现：`solve/main:controller/channel.go` 的 `ChannelBatchEdit` / `EditChannelBatch` 和 `model/channel.go` 的 `EditChannelsByIds`。
- final 字段已扩展为 models、mapping、groups、priority、weight、max concurrency 和 Codex fingerprint mode。
- final 测试：`model/channel_batch_edit_test.go` 覆盖模型更新与 ability 重建、priority/weight 定向同步、mapping 不触碰 ability、并发值设置/清零和空 ID no-op。

### 当前分支已有与缺失

当前已有批量启停、删除、设置标签、批量读取 key，以及 Codex fingerprint 专用的 `PUT /api/channel/batch`；没有通用 `ChannelBatchEdit` 或 `EditChannelsByIds`。因此 final solve 能力仍是缺口，但路由不能直接照搬：当前同一个 method/path 已被专用操作占用。

### 收益、复杂度与风险

- **收益**：无需先统一 tag，即可对任意选中渠道批量调整调度参数和并发限制，运维价值明确。
- **复杂度**：中，后端更新/ability 同步、路由合并、前端表单和审计都要改。
- **风险**：solve 明确采用“DB 先提交、ability 后重建且失败不回滚”的非事务模式，存在短暂不一致；当前分支还有 authz、Redis Pub/Sub 和 WebSocket 关闭语义，迁移时必须统一。建议把 Codex 专用字段并入一个显式 patch contract，或使用新路由，避免 method/path 冲突。

## P2：注册风控套件应拆分迁移

### 证据与 final HEAD 状态

- 设计：`docs/superpowers/specs/2026-07-10-registration-domain-risk-control-design.md`、`2026-07-15-registration-email-verification-design.md`。
- final 保留的能力包括：
  - 可配置邮箱域名黑名单：`407135d0c`；
  - 事务化域名注册频率阈值、封禁事件和精确恢复：`4293d52f2`、`a1d1b3a85`、`85bbcf138`、`714ee10ed`、`cca5a8492`；
  - DNS/MX/一次性邮箱校验：`99c028806`；
  - 注册 honeypot：`5ceb1b8be`；
  - 国家策略：`18eb7f19d`；
  - 服务端图形验证码：`9730abed7`。
- final 测试分布在 `controller/registration_*_test.go`、`model/registration_domain_risk_test.go`、`service/registration_security_test.go`、middleware/router 和前端注册测试中。

### 当前分支已有与缺失

当前有基础邮箱域名白名单、邮箱验证码和 Turnstile，但没有 final solve 的域名事件表、跨注册路径事务风控、DNS/MX/一次性邮箱校验、honeypot、国家策略或服务端图形验证码。

### 收益、复杂度与风险

- **收益**：在公开注册前降低批量养号、一次性邮箱和自动化注册成本，并提供可恢复的管理员事件记录。
- **复杂度**：高，必须拆票：
  1. 黑名单与域名解析规范化；
  2. DNS/MX/一次性邮箱与超时缓存；
  3. honeypot/服务端验证码；
  4. 事务化阈值、incident 与恢复；
  5. 国家策略仅在确有合规需求时启用。
- **风险**：OAuth 首次注册路径遗漏、三数据库事务差异、DNS 阻塞、公共后缀解析、误杀与恢复、账号枚举和无障碍问题。所有认证变更必须按当前仓库 OWASP 要求重新设计，不能照搬旧 controller。

## P2：按渠道白标清洗成功响应

### 证据与 final HEAD 状态

- 提交：`1e6ee424a`；响应头补丁：`1cfb43a85`。
- final `WhitelabelUpstream` 可将 OpenAI 兼容流式/非流式响应的 model 改回客户端请求名，并删除 `system_fingerprint`；特定上游身份 header 也被过滤。
- `relay/channel/openai/whitelabel_test.go` 覆盖成功响应和流式 usage chunk。

### 当前分支差距

当前有全局错误文案清洗，但不会清洗成功响应中的上游 model、fingerprint 或供应商身份 header。

### 收益、复杂度与风险

- **收益**：对外白标售卖时避免暴露底层引擎或转售供应商。
- **复杂度**：中。
- **风险**：修改成功响应会影响客户端诊断和缓存键；header 不能靠不断追加供应商硬编码。应定义清晰的 allow/deny contract，默认关闭，并保留真实上游信息在管理员日志。

## P3：模型标签与展示权重排序

### 证据与 final HEAD 状态

- 提交链：`cd2d9761f`、`b1b7ed4d9`、`226222958`、`4ca6b5a11`、`69a35515a`。
- final 状态不是硬编码推广徽标：`7a86e6362` 已删除硬编码 promotion badges，最终由 metadata tags 驱动徽标，所有有标签模型优先，再按 `display_weight` 排序。

### 当前分支差距

当前 `Model` 和模型目录已有 tags，但没有 `display_weight` 及其 API/表单/模型访问投影。

### 收益、复杂度与风险

- **收益**：管理员可统一控制可用模型页和 Playground 的排序，不再在前端写死模型名单。
- **复杂度**：中，涉及 schema、同步、API 和多个前端入口。
- **风险**：这是展示能力而非可靠性能力；要避免把标签兼作授权或计费信号。低于前述基础设施项。

## 条件候选与明确 TODO

### Claude 原生版本化 Web Tool 保真：按流量迁移

- final 提交 `4d2ad9cd5` 仍存在：Claude 请求保留 `web_search_YYYYMMDD`、`web_fetch_YYYYMMDD` 工具的原始 JSON，避免转换时丢失未知供应商选项或显式零值；同时识别 OpenAI 日期化 web-search 类型。
- 当前分支没有 `ClaudeWebTool` 原始 JSON 保留机制，也没有等价的 `service/channel_web_tools.go` 路径。
- **收益**：Claude/Claude Code 联网工具演进时，网关无需先认识每个新字段也能保持请求语义。
- **复杂度/风险**：中。必须限制在已知 Web Tool 类型，不能把“保留未知 JSON”扩大成所有工具的无校验透传。只有线上确有 Claude 原生联网工具流量时才优先。

### 亲和渠道 5xx fallback 且不改写 affinity：仅 TODO

- `solve/main:docs/TODO.md` 明确记录了理想行为：命中亲和规则后，软错误仍按 `SkipRetryOnFailure` 决定；上游 5xx 可选择 fallback；fallback 成功不得覆盖原 affinity cache，使原渠道恢复后能自动回归。
- final HEAD 仍没有文档建议的 `RetryOnUpstreamFailure` 字段，`MarkChannelAffinityUsed` 也没有“fallback 不改写 cache”的实现；当前分支同样缺失。因此这不是可直接迁移的二开功能，而是来自生产事故记录的独立设计线索。
- 对 Responses/Codex 流量还必须额外考虑加密 reasoning 的渠道兼容域：跨不同订阅/API key fallback 可能触发 `invalid_encrypted_content`，不能只实现 affinity fallback 而忽略已在当前分支开发中的加密状态清理兜底。
- **结论**：值得另立设计票，但不能列为“对齐最新版”；应先定义可 fallback 的错误类别、兼容渠道池和 cache 写回规则。

### 管理员用户模拟登录：条件迁移

- final 提交 `539d76968` 仍存在；`ca9211998` 将进入/退出事件记在真实管理员审计中并从被模拟用户日志隐藏。
- 当前分支没有该能力。
- **收益**：客服可复现用户控制台问题。
- **复杂度/风险**：高。当前分支已经有更严格的登录 session、auth version、MFA/Passkey 和敏感操作 step-up，不能直接修改 session 字段照搬。若要做，至少要求 root/admin 边界、禁止模拟管理员、短时会话、明显 banner、真实操作者审计、再次认证和一键退出。

### 脱敏请求采样：仍有产品 TODO，不建议直接迁移

- 提交链：`d4c0e338a`、`eb7c99559`、`24f4d72d2`、`094a55b93`、`57119a692`。
- final 功能默认关闭、默认不存文本内容，具备字段名/凭据模式脱敏、URL 与二进制丢弃、body/string/depth 上限和异步写入。
- 但 `solve/main:model/log_request_sample.go` final HEAD 仍明确写有：`TODO(product): define a retention or manual deletion policy`，并注明不会被 `DeleteOldLog` 删除。
- 当前分支没有请求采样。
- **结论**：它有排障价值，但目前属于“实现存在、生命周期不完整”。只有补齐保留期、手动删除、访问审计、加密/备份策略和隐私告知后才应迁移。

## 已删除、当前已有或不应迁移的项目

| 项目 | final HEAD 核对 | 结论 |
| --- | --- | --- |
| Codex 团队邀请 | `0f7f58f96` 加入、`044021068` 加固；`f94fdfaf9` 删除服务/UI/测试。`docs/upstream-pr-merge-status-2026-07-20.md` 明确写“Codex 邀请功能下线”；final 树无相关实现 | **已删除，不迁移** |
| 旧 Redis 聚合 Prometheus | 早期 `27c325069` 至 `2fbfdd66f` 实现；`067dfe323` 改为每实例 Google Managed Prometheus sidecar并删除聚合路径 | **已下线的架构，不迁移**；仅迁移 final 的模型指标与失败鉴权限流 |
| 并发调度、实时并发与 Redis 压力优化 | 当前已有 `service/channel_concurrency.go`、wait/cooldown/load cache、实时状态、auto-group 和更完整测试 | **当前已有**，不再作为 solve 候选 |
| 渠道封禁状态 | 当前有 `ChannelStatusBanned=4`、服务端保护、审计和前端动作 | **当前已有** |
| 配置 Redis Pub/Sub | 当前 `common/redis_pubsub.go` 支持 scope 通知及指定渠道会话关闭，测试覆盖跨实例 | **当前已有，且更强** |
| 渠道 Prometheus health | 当前 `pkg/perf_metrics/channel_prometheus.go` 已有 channel attempt、duration、TTFT、channel+model counter/token | **当前已有**；缺的是模型最终体验直方图 |
| 模型可用性/Codex governance | 当前已有 model availability、Codex governance、探测与专用钉钉告警 | **当前已有** |
| API Key 基础模型 allowlist 与模型范围预览 | 当前有 `ModelLimits`、统一 model-access service、`/api/user/model-access` 和前端预览 | **当前已有**；候选只包含 blacklist、批量规则和 fail-closed pending marker |
| upstream request ID | 当前有字段、索引、筛选、详情页和 header 捕获 | **当前已有**；不要与缺失的 response body ID 混淆 |
| Session cookie 安全 | 当前 `common/session_cookie.go` 和 origin guard 已比 solve 的早期 cookie 修复更完整 | **当前已有/更强** |
| MySQL bool default AutoMigrate shim `7622857a0` | solve 仍使用 GORM 1.25.2；当前使用 GORM 1.25.12，已包含上游修复 | **误报，不迁移 shim** |
| Google OAuth 专用 claim table | 当前已有通用 verified-email、冲突处理和更现代的身份绑定机制 | **不直接倒灌旧专用设计** |
| 硬编码模型推广徽标 | final 已由 `7a86e6362` 删除，替换成 metadata tags/weight | **旧方案已删除**，只考虑最终 metadata 方案 |
| GCP TLS 到期告警、Managed Prometheus sidecar、Browser QA | final 中存在但绑定 solve 的 GCP/Cloud Run 部署 | **部署定制，不作为通用代码迁移** |
| GitHub Copilot、Grok Subscription、Apodex、特定媒体渠道 | final 中存在但属于供应商接入或业务产品 | **不属于本报告的通用可靠性范围** |
| 支付、钱包、订阅、邀请奖励、Recall、广告、SEO、官网、Prompt Gallery、Compute | 大量提交存在 | **按要求排除** |

## 推荐实施顺序

### 第一批：错误语义与小型可靠性修复

1. `LocalError` 决策优先级与参数化测试。
2. 候选耗尽时返回 `LastError`。
3. SSE UTF-8。
4. 请求类型错误脱敏与字段路径保护。
5. Prometheus 鉴权失败限流。
6. 首包看门狗及指标。

### 第二批：状态与存储可靠性

1. MySQL DSN 超时。
2. batch flusher panic/失败重入边界。
3. Redis 验证码与多实例故障策略。
4. upstream response ID。

### 第三批：权限与可观测性

1. API Key blacklist 和规则规范化。
2. API Key 批量更新与 owned pending marker。
3. 模型级 Prometheus 指标及 series budget。
4. 通用渠道测试告警、类型过滤和分布式冷却。

### 第四批：协议与运维增强

1. Claude/Fable gate。
2. Claude 原生版本化 Web Tool 保真（有对应流量时）。
3. 通用渠道批量编辑。
4. 注册风控分票。
5. 按业务需要评估白标和展示权重。

## 总体迁移原则

- 不 cherry-pick 大提交或覆盖文件；当前分支的 request-policy、auto-group、并发调度、authz、session、`relaykit` 和缓存协议都比 solve 对应基线不同。
- 每项先写当前架构下的行为测试，再移植不变量。
- 对所有“失败后重试”功能区分：本地失败、上游失败、尚未提交响应、已提交响应、结果可能未知五类状态。
- 对 Redis 权限/验证码功能先定义故障语义：fail-open、fail-closed、fallback 或短时不可用必须是显式决策。
- 对 Prometheus 和日志严格控制高基数与敏感数据；response ID 进入日志，不进入指标标签。
