# 上游 `main` 合入 Tokendazi 执行 Spec（2026-11-24）

## 1. 文档状态

- 状态：**双轴 review 修复完成；待最终门禁与复审清零后创建本地 merge commit**。
- 本 Spec 是本轮完整上游合并的实现、复核和验收基准。
- 任何冲突处理、自动合并语义审计、补充修复和测试结论都必须能回指本文件。
- 若实现需要偏离本 Spec，必须先更新本文件并记录原因，再修改代码。
- 本轮只允许本地合并、测试和 review；**不 push、不部署**。

### 1.1 补写说明

在本 Spec 建立前，临时集成 worktree 已开始处理冲突，这是执行顺序错误。此前手工处理的 22 个冲突文件全部视为“临时实现”，必须按本 Spec 重新复核，不能因已 `git add` 就视为验收通过。

截至补写时：

- 未创建 merge commit。
- 未 push。
- 未部署。
- 原始开发工作区未被清理或重置。
- 仓库仍处于未完成 merge 状态，仍有 unresolved paths，尚不能运行完整测试。

### 1.2 当前状态

- Tickets 01–11 与获批 Q6 方案均已实现并完成定向验证。
- unresolved paths 与非 Markdown conflict markers 均为 0，`git diff HEAD --check` 无错误。
- 根模块、`relaykit`、`tokenkit`、前端与 SQLite/MySQL/PostgreSQL 门禁均已完成；结果见第 14 节。
- 全量前端 lint/format 仍有父提交原样继承的基线问题，本轮修改文件与基线失败文件 0 重叠。
- 尚未创建 merge commit，未 push、未部署，原始开发工作区未被修改或清理。

## 2. 固定基线

| 项目 | 固定值 |
|---|---|
| Tokendazi 原分支 HEAD | `4c4dc34b0439f633fc6b2f97a817510882109ff9` |
| 固定上游 HEAD | `cb074d31e296b53ce252d402a7063079cbf7f990` |
| 共同祖先 | `7fb6e57a5ff671582131c74e00bdfaa867d4e985` |
| 集成分支 | `merge/upstream-cb074d31e` |
| 集成 worktree | `/tmp/newapi-upstream-merge-assessment-20261123` |
| 合并方式 | 完整 `merge`，不是选择性 cherry-pick |
| 上游新增提交 | 118 |
| Tokendazi 新增提交 | 161 |
| 双方重叠文件 | 150 |
| 初始冲突文件 | 58（56 content、1 add/add、1 modify/delete） |

本轮执行期间不得移动固定上游 HEAD。若官方 `main` 再更新，应在本轮完成后另做增量合并，不得把新提交混入当前验收范围。

## 3. 已确认决策

1. 完整合入固定上游，不做选择性 cherry-pick。
2. 行为等价时优先复用官方实现，减少长期分叉成本。
3. 双方能力部分重叠时，以官方架构为底座补齐 Tokendazi 已上线契约。
4. 官方实现缺少关键能力、破坏安全默认值、生产数据或既有契约时，保留 Tokendazi 实现，并补入官方新增能力。
5. 保留 Tokendazi 品牌、Workspace 信息架构、权限边界和生产数据兼容性。
6. Tokendazi 敏感词预检、观察/阻断、审计与独立面板继续作为权威体系。
7. 官方 request-plugin/request-filter 完整保留并与权威预检共存，但不把旧敏感词配置重复迁入 request-filter。
8. 不执行官方会删除旧 option 的 `MigrateSensitiveWords()`；继续使用并强化 Tokendazi `MigrateSensitiveWordData()`，旧 option 与已有插件实例均不得被覆盖或删除。
9. 敏感词迁移必须幂等、SQLite/MySQL/PostgreSQL 兼容，并通过数据库原子租约避免多实例启动时重复导入。

## 4. 目标

1. 完整吸收固定上游的功能、安全修复、插件体系、依赖和稳定性更新。
2. 保留 Tokendazi 已上线的认证缓存、渠道并发与冷却、分组路由、Codex 治理、计费保护、首事件超时、敏感信息默认保护和 Workspace 体验。
3. 对冲突和高风险自动合并文件做三方语义合并，而不是只消除 conflict marker。
4. 保证 SQLite、MySQL、PostgreSQL 兼容；不得以迁移便利为由破坏生产数据。
5. 完成本地后端、前端、数据库迁移和生成文件验证，形成可复核的 merge 结果。

## 5. 非目标与禁区

- 不执行官方破坏性 `MigrateSensitiveWords()`，也不把 Tokendazi 规则镜像成第二套 request-filter 规则。
- 不重构与本次合并无关的模块。
- 不修复与本次上游合并无关的既有失败。
- 不直接修改、清理或重置 `/root/newapi/new-api` 中用户已有未提交文件。
- 禁止 `git reset --hard`、`git clean`、merge abort 或覆盖原分支。
- 不提交凭据、生产数据、备份、日志样本中的敏感信息。
- 未完成全部门禁前不创建最终 merge commit；本轮不 push、不部署。

## 6. 通用冲突决策算法

每个冲突必须按以下顺序处理：

1. 比较共同祖先、Tokendazi、官方三个版本，而不是只看 conflict marker。
2. 追踪调用方、测试、数据库写入、缓存失效和用户可见行为。
3. 判断双方修改属于替代、互补还是相互排斥。
4. 等价能力选官方实现。
5. 互补能力组合实现，并明确执行顺序和单次副作用约束。
6. 官方能力较弱时保留 Tokendazi 契约，同时接入官方同步、插件或安全框架。
7. 为新增组合行为补回归测试。
8. 完成定向测试后才能把该冲突域标记为完成。

以下高风险文件禁止整文件选择 `ours` 或 `theirs`：认证缓存、渠道模型与缓存、Relay、计费、错误处理、迁移、请求策略、系统任务、路由生成和权限相关前端。

## 7. 核心行为不变量

### 7.1 认证缓存与 API Key

- Redis 认证缓存必须保留官方 MAC 完整性校验。
- Redis Cluster 使用的认证、并发 fence/version key 必须处于同一 hash slot，禁止引入跨槽事务错误。
- 缓存 MAC 必须覆盖 `MaxConcurrency` 与 `ConcurrencyVersion`，防止篡改或陈旧数据绕过并发限制。
- Tokendazi 用户并发版本栅栏必须保留。
- API Key 的 `AllowUserQuotaQuery` 继续默认关闭；只有用户明确开启才允许查询。
- API Key 创建与编辑不得因官方 UI 合并而放宽权限、过期时间或安全默认值。

### 7.2 渠道生命周期、路由与缓存

- 保留永久不可用状态 `ChannelStatusBanned`，不能被自动测试或周期任务恢复。
- 保留 Codex fingerprint seed、分组 priority/weight、渠道并发、等待与冷却语义。
- 渠道状态变化继续通过 Tokendazi Redis PubSub 即时通知，同时调用官方 `BumpSyncTopic(SyncTopicChannels)`。
- 渠道常规编辑不得覆盖密钥；密钥只能通过受保护的专用路径更新。
- `other_info` 更新必须使用官方乐观并发方案，禁止 stale snapshot 覆盖其他写入。
- 保留 `forceMaxConcurrency` 契约。
- 合入官方插件渠道配置、凭据校验、凭据刷新锁和余额详情。
- 多节点凭据刷新不能重复消费一次性 refresh token，且不能因单个等待者取消而丢失已轮换凭据。

### 7.3 Codex 凭据与治理

- Codex 刷新保留 Tokendazi 的 expected-key 检查、singleflight、代理刷新、缓存重置和 CAS 持久化。
- 凭据成功落库后必须触发官方渠道同步 topic。
- CAS 失败时读取并使用最新凭据，不得覆盖管理员或另一节点刚写入的值。
- 上游刷新成功后的持久化使用有界、与单个请求取消解耦的 context，避免 refresh token 已旋转却未保存。
- Codex 管理端 usage/reset 重试必须复用统一刷新服务，不允许另建较弱的直接写 Key 路径。
- 保留 Tokendazi Codex request finalizer、客户端身份治理和 fingerprint 行为。

### 7.4 上游请求构建顺序

普通 HTTP、form 和 WebSocket 上游请求使用同一优先级：

1. adaptor/provider 构造 URL 与基础 header；
2. request plugin 产生的 header 效果；
3. 管理员配置的 Header Override；
4. Tokendazi provider finalizer 恢复不可被覆盖的协议/安全约束；
5. 官方 `RequestSigner` 对最终 URL/header 签名；
6. 发送请求。

签名必须最后执行，否则后续 header 或 query 修改会使签名失效。Codex finalizer 必须在签名前执行，防止用户覆盖授权和客户端身份头。

Responses JSON 请求继续支持官方 `ResponsesRequestFinalizer`；它与 HTTP request finalizer 是不同层次的接口，两者都必须保留。

### 7.5 Relay、重试与错误日志

- 已向客户端提交真实响应后，普通 HTTP/SSE 不得重试到另一渠道；WebSocket 按其独立状态机处理。
- affinity、渠道固定、`SkipRetry` 和状态码规则继续决定是否重试。
- 每次最终失败只记录一条错误日志；不得同时保留旧内联日志和官方抽取日志而产生重复记录。
- 错误日志必须保留官方 `request_plugins` 审计记录。
- request-plugin、本地策略拒绝等 `local` 错误不得触发渠道并发冷却或自动禁用，即使错误文本包含 `capacity` 或 `rate limit`。
- 真实上游 429/容量并发错误继续按 Tokendazi 规则进入渠道冷却；配额耗尽类错误不得误判为瞬时并发冷却。
- ingress 插件拒绝发生在预扣前，不退款；attempt/egress 失败由统一 billing session 处理退款，错误日志函数不得重复处理计费。
- 无可用渠道 503 与并发等待失败继续遵守既有日志口径，不在本次合并中扩大用户日志噪音。

### 7.6 计费与空回复

- 预扣、结算、退款、用户累计、渠道累计、消费日志和性能样本均只能执行一次。
- 非流式 OpenAI 兼容空输出继续使用 Tokendazi 的本地计数与空回复标记，使实际计费归零；不得因官方插件合并恢复扣费。
- 显式上游 `empty_response` 错误仍作为失败处理并最终退款。
- 流式请求继续按有效业务输出和终止状态处理，不把心跳、注释或伪 `[DONE]` 当作成功输出。
- 已获得可靠权威 usage 的失败请求按既有 BillingSession 契约结算；后续退款调用必须幂等。

### 7.7 Streaming 与 WebSocket

- 首响应超时只由第一个非心跳业务事件解除。
- `data == "[DONE]"` 才是终止帧；`[DONE]extra` 必须作为普通数据，禁止前缀匹配。
- 超时、取消、panic 和正常完成必须设置唯一且准确的 stream end reason。
- retry 时必须重置 attempt-scoped 的本地 token/空回复标记和 request-plugin header/record，同时保留 request-scoped 诊断与计费累积。
- Responses WebSocket 必须保留官方 request-plugin 路径与 Tokendazi 已有 provider 回归测试。

### 7.8 敏感词与 request-plugin 共存

- 渠道选择前的 Tokendazi 敏感词结果是观察、阻断、白名单与独立审计面板的唯一权威结果。
- 预检结果写入 `ContextKeySensitiveWordCheckResult`；计费层只能复用，不得重复匹配、重复审计或产生第二条敏感词日志。
- 官方 request-plugin/request-filter 可独立做通用请求变换、拒绝与回复检查；其审计记录继续写入普通 usage/error log，不替代 Tokendazi 敏感词审计。
- request-plugin 的本地拒绝与敏感词本地拒绝都不得冷却或禁用上游渠道。
- `MigrateSensitiveWordData()` 保留旧 option 原文，幂等导入 Tokendazi 规则，并用原子迁移租约保证多实例仅有一个执行者；迁移完成标记提交前不得释放租约。
- 已存在的 `RequestPluginInstances` 不得被敏感词迁移改写；官方旧敏感词到 request-filter 的删除式迁移明确不执行。
- 敏感词独立面板、sidebar 权限、usage-log 类型与七语言键必须与官方插件页面同时保留。

### 7.9 请求策略、插件与系统任务

- 官方 request-plugin/provider-plugin/task-plugin 架构完整合入。
- Tokendazi 已上线 request policy 契约、默认值与已有数据必须保持可读。
- 请求策略采用官方 MoeForge regexp 与跨节点 option sync，同时保留 Tokendazi `ResponsesEncryptedReasoningRecoveryEnabled` 字段、默认值、校验和原子 snapshot。
- option 全量加载只允许一次数据库读取；按 `requestPolicy → passkey` 顺序持锁，数据库读取失败时不得修改当前运行时配置。
- 官方跨节点 `SyncTopicOptions` 与 Tokendazi `streamingOptionRevision` 必须同时保留：前者同步其他节点，后者防止本节点陈旧快照覆盖刚写入的流式设置。
- option 批量更新成功后，流式设置应用、revision 递增和单次 sync topic 通知缺一不可；数据库写失败不得发出虚假同步通知。
- 插件安装、更新、凭据刷新和系统同步任务必须幂等，并在多节点环境避免重复副作用。
- 系统任务类型、handler 注册和持久化标识不得因枚举重排而改变已有任务含义。
- 最终 handler 集合必须包含 channel test、channel balance、model update、midjourney poll、async task poll、model availability、system task cleanup 与敏感词审计清理。
- 系统任务历史清理只删除超过保留期的已完成任务，不得删除 pending/running 任务。
- `model/option.go` 的 option key、默认值和同步 topic 必须做双向清单审计，禁止静默丢键。
- 插件路由采用官方 `RouteGroup` 多候选结构；查询路由不得占满用户并发槽，真实提交仍必须经过用户并发限制。

### 7.10 前端、权限与信息架构

- 保留 Tokendazi 品牌、Workspace 导航和角色权限过滤。
- 官方新页面、插件入口和 request-processing 能力应接入现有信息架构，不能绕过权限组件。
- 注册条款、登录/注册表单必须复用现有 auth 组件，并符合现有 OWASP 认证/会话约束。
- `LegalConsent` 已提供强制法律确认时，不恢复仅展示文本且无调用方的旧 `TermsFooter`。
- 注册页以 Tokendazi 品牌化表单为底座，吸收官方密码强度、密码确认反馈、`aria-describedby` 与 `autocomplete=new-password`；不得丢失广告归因、OAuth 位置、Turnstile 或法律勾选。
- API Key drawer 保留 Tokendazi 安全默认值、小时级过期体验和权限字段，同时合入官方新增字段。
- API Key 编辑切换记录时必须先禁用保存并等待详情加载，再按当前记录动态 reset，防止上一把密钥的额度查询权限、过期时间或模型限制串入下一把。
- 新 API Key 的 `allow_user_quota_query` 继续默认 `false`；编辑已有 key 时回填其真实存量值。
- i18n 使用七语言英文源键；不得用删除本地键解决冲突。
- `static-keys.ts` 按两边新增键集合合并；只有确认已无调用方的旧密码提示键可以随官方删除，不顺手清理共同基线中的重复键。
- `routeTree.gen.ts` 等生成文件最后由对应生成器重建，不手工拼接为最终结果。

### 7.11 数据与迁移安全

- 所有 schema 变更必须支持 SQLite、MySQL、PostgreSQL。
- 不允许 drop/rename 生产字段或覆盖已有 option，除非有显式迁移与回滚证明。
- 敏感词旧数据只导入 Tokendazi 规则；旧 option 和已有 request-plugin 配置保持原值，迁移锁异常退出后可安全重试。
- 自动迁移必须可重复执行；多实例同时启动不得产生重复数据或破坏唯一约束。
- 正式部署前必须另做生产备份和恢复校验；本轮不部署。

### 7.12 定价可见性与性能口径

- 模型定价使用官方认证中间件提供的 `ContextKeyUserGroup`，不在控制器内重新查询用户。
- 先按调用者可用分组过滤官方 pricing 响应，禁止通过 `EnableGroup` 泄露不可见私有分组。
- 过滤后再生成 Tokendazi `group_model_ratio`；支持通配模型与显式倍率 `0`，且不得暴露调用者不可见分组。
- 性能统计保持 Tokendazi 二态口径：所有已进入平台处理的真实请求最终只计为 success 或 failure。
- 客户端取消、业务拒绝、参数拒绝、敏感策略拒绝均计入真实 failure；本轮不恢复官方 `OutcomeIgnored`。
- 已有明确正常完成信号的流式请求，即使随后 context 取消，仍计 success；非流式不能只因 `apiErr == nil` 就推断“已完成后取消”。
- 官方即时任务成功采样与 hot bucket 合并能力应保留，但不得改变上述二态口径。

## 8. 冲突域与阶段

### Phase A：Spec 与现场冻结

1. 固定 hashes、worktree 和冲突清单。
2. 建立本 Spec。
3. 对 Spec 前已处理的 22 个文件逐个回审。

**Gate A：** Spec 已存在；基线与禁区准确；无 push/deploy；此前实现均未被误标为已验收。

### Phase B：认证缓存与渠道系统复核

范围：认证缓存、用户并发版本、渠道模型/cache/status、渠道选择与渠道前端。

**Gate B：** MAC、Redis hash slot、并发 fence、永久禁用、分组路由、渠道同步、密钥更新与插件凭据测试通过。

### Phase C：Relay、Codex 与 Streaming

范围：Codex usage/refresh、请求 finalizer/signer、Relay error、Volcengine TTS、attempt reset、stream scanner、Responses WebSocket。

**Gate C：** 请求顺序、CAS 刷新、单日志、本地错误不冷却、空回复计费、精确 `[DONE]`、首非心跳事件和 WS 回归测试通过。

### Phase D：定价、任务、Option 与请求策略

范围：pricing、system task handlers、option、request policy、perf metrics、plugin router。

**Gate D：** option 双向清单无丢失，任务类型不漂移，策略旧数据兼容，metrics 口径一致，插件路由权限测试通过。

### Phase E：非敏感词前端

范围：terms footer、sign-up form、API Key drawer、非敏感词 static keys。

**Gate E：** auth/API Key 定向测试、i18n、typecheck 和涉及文件 lint 通过；权限与安全默认值不回退。

### Phase F：非 Q6 阶段检查点（已完成）

1. 列出只剩敏感词 Q6 的 unresolved files。
2. 运行所有不被 Q6 conflict marker 阻断的定向检查。
3. 向用户报告已处理项、待决项与风险。

**Gate F：** 非 Q6 冲突完成语义合并并有定向验证；进入已批准的 Q6 实现前不创建 merge commit。

### Phase G：敏感词 Q6（已确认并实现）

- Tokendazi 敏感词体系继续作为权威，官方 request-plugin/request-filter 并存。
- 官方 `MigrateSensitiveWords()` 不执行；`MigrateSensitiveWordData()` 保留旧源并增加多实例原子租约。
- 合并独立面板、sidebar、usage-log 类型、七语言 locale 与生成路由，并运行敏感词、插件和数据库迁移回归测试。

**Gate G：** 旧 option/request-plugin 实例无覆盖或删除；并发启动只导入一份规则；敏感词与插件各自只产生其合同规定的审计记录。

### Phase H：全量验证与本地 merge commit

仅在 Q6 解决后执行：清除全部冲突、生成文件、全量后端/前端/数据库门禁、双轴 review，最后创建本地 merge commit。

**Gate H：** 所有必需检查有最新结果；无 unresolved path/marker；不 push、不部署。

## 9. Spec 前临时实现复核清单

以下文件已在 Spec 前手工解冲突，必须按对应不变量复核：

### 9.1 认证缓存

- `model/token.go`
- `model/user_auth_cache.go`
- `model/user_cache.go`
- `model/user_cache_auth_version_test.go`

### 9.2 渠道系统

- `controller/channel.go`
- `controller/channel_authz.go`
- `controller/channel_multi_key_test.go`
- `model/channel.go`
- `model/channel_cache.go`
- `model/channel_status_test.go`
- `service/channel_select.go`
- `web/src/features/channels/components/channels-columns.tsx`
- `web/src/features/channels/components/data-table-row-actions.tsx`
- `web/src/features/channels/hooks/use-channel-mutate-form.ts`

### 9.3 Relay / Codex / Streaming

- `controller/codex_usage.go`
- `service/codex_credential_refresh.go`
- `relay/channel/adapter.go`
- `relay/channel/api_request.go`
- `relay/channel/volcengine/tts.go`
- `relay/common/relay_info.go`
- `relay/helper/stream_scanner.go`
- `relay/responses_websocket_test.go`

这些文件当前只代表 conflict marker 已消除，不代表代码已通过编译、测试或 review。

## 10. Q6 语义复核边界

以下文件包含敏感词迁移、审计、导航或与 Q6 强耦合的生成内容，均须按 7.8 的最终合同复核：

- `constant/context_key.go`
- `controller/relay.go`
- `i18n/locales/zh-CN.yaml`
- `i18n/locales/zh-TW.yaml`
- `main.go`
- `middleware/distributor.go`
- `model/sensitive_word_migration.go`
- `model/system_task.go`
- `pkg/perf_metrics/outcome.go`
- `relay/request_billing.go`
- `relay/responses_websocket.go`
- `relaykit/types/error.go`
- `web/src/features/system-settings/request-policies/section-registry.tsx`
- `web/src/features/usage-logs/types.ts`
- `web/src/hooks/__tests__/sidebar-config.test.tsx`
- `web/src/hooks/use-sidebar-data.ts`
- `web/src/i18n/locales/en.json`
- `web/src/i18n/locales/fr.json`
- `web/src/i18n/locales/ja.json`
- `web/src/i18n/locales/ru.json`
- `web/src/i18n/locales/vi.json`
- `web/src/i18n/locales/zh-TW.json`
- `web/src/i18n/locales/zh.json`
- `web/src/routeTree.gen.ts`

这些文件不得整侧取舍；Tokendazi 权威敏感词能力、官方插件能力和非敏感词增量必须同时保留。

## 11. 验证矩阵

### 11.1 合并完整性

```bash
git diff --name-only --diff-filter=U
git diff --check
rg -n '^(<<<<<<<|=======|>>>>>>>)' --glob '!*.md' .
```

最终验收：0 unresolved path、0 conflict marker、0 whitespace error。

### 11.2 后端定向测试

冲突域完成后至少运行：

```bash
GOTOOLCHAIN=auto GOWORK=off go test ./model
GOTOOLCHAIN=auto GOWORK=off go test ./service
GOTOOLCHAIN=auto GOWORK=off go test ./relay/channel ./relay/common ./relay/helper
GOTOOLCHAIN=auto GOWORK=off go test ./relay/channel/codex ./relay/channel/openai ./relay/channel/volcengine
GOTOOLCHAIN=auto GOWORK=off go test ./controller ./pkg/perf_metrics ./router
```

重点覆盖：

- 认证缓存完整性、Redis hash slot、用户并发版本；
- 渠道永久禁用、并发冷却、分组优先级、多 key；
- Codex CAS 刷新、取消后持久化、缓存与 sync topic；
- finalizer/signature 顺序；
- 本地错误不冷却、错误日志单次写入、request-plugin 审计；
- 非流式空回复零计费；
- 首非心跳事件超时与精确 `[DONE]`；
- Responses WebSocket request-plugin/provider 行为；
- request policy、system task、option 与 perf outcome。

### 11.3 后端全量门禁

```bash
GOTOOLCHAIN=auto GOWORK=off go test ./...
GOTOOLCHAIN=auto GOWORK=off go vet ./...
GOTOOLCHAIN=auto GOWORK=off go build ./...
(cd relaykit && GOWORK=off go test ./... && GOWORK=off go vet ./... && GOWORK=off go build ./...)
(cd tokenkit && GOTOOLCHAIN=auto GOWORK=off go test ./... && GOTOOLCHAIN=auto GOWORK=off go vet ./... && GOTOOLCHAIN=auto GOWORK=off go build ./...)
```

完整门禁必须在 0 unresolved path、0 conflict marker 的真实合并工作树运行，不再使用 Q6 overlay 代替最终验证。

### 11.4 前端定向测试

根据最终冲突实现运行对应 Vitest，至少覆盖：

- channel table、row actions、编辑表单；
- sign-up 与 terms footer；
- API Key drawer；
- sidebar/Workspace 权限；
- request policy/request-processing；
- usage log request-plugin record；
- 敏感词独立面板、sidebar 权限与审计详情；
- request-processing、request-plugin 日志与敏感词页面共存。

### 11.5 前端全量门禁

```bash
cd web
bun install --frozen-lockfile
bun run i18n:sync
bun check -b
bun run lint
bun run format:check
bun run test
bun run build
```

若 `package.json` 中脚本名与上述命令不同，以合并后的实际脚本为准，并在 verification 中记录真实命令。

### 11.6 数据库兼容

- SQLite：必须本地实际运行迁移与相关测试。
- MySQL：使用测试 DSN 验证迁移、唯一约束、锁与 option/system task 行为。
- PostgreSQL：使用测试 DSN验证迁移、advisory lock、CAS 与 option/system task 行为。
- 不连接生产数据库做合并验证。
- 外部 DSN 缺失时明确记录“未运行”，不得标记通过。

### 11.7 双轴 Review

最终提交前分别执行：

1. Standards review：是否符合 `AGENTS.md`、`web/AGENTS.md`、安全、i18n、测试与数据库规范。
2. Spec review：是否逐项满足本文件的不变量、阶段 Gate 和用户已确认决策。

两个 review 的阻断项清零后才能创建本地 merge commit。

## 12. 完成定义

本次上游合并只有同时满足以下条件才算完成：

1. Q6 经单独决策并写入更新后的 Spec。
2. 所有冲突及高风险自动合并文件完成语义审计。
3. 所有 conflict marker 与 unresolved path 清零。
4. 定向、全量、前端、独立模块和可用数据库测试有最新结果。
5. 双轴 review 无未解决阻断项。
6. 创建本地 merge commit，并准确记录验证结果。
7. 未 push、未部署；后续 push/部署必须由用户另行确认。

## 13. 执行记录

| 时间 | 事件 | 结果 |
|---|---|---|
| 2026-11-23 | 固定上游并创建临时集成 worktree | 完成 |
| 2026-11-23 至 2026-11-24 | 预演并开始处理认证、渠道、Relay 冲突 | 在 Spec 前发生，全部待复核 |
| 2026-11-24 | 用户要求 Q6 暂缓、其余按推荐方案继续 | 已记录 |
| 2026-11-24 | 用户指出应先写 Spec | 已暂停代码修改并补写本文件 |
| 2026-11-24 | 用户确认 Q6 按推荐方案继续并要求完成全部 tickets | 已写入 7.8，进入实现与全量验证 |
| 2026-11-24 | Tickets 05–11 完成实现与定向验证 | 进入全量门禁与双轴 review；未 push、未部署 |
| 2026-11-24 | controller 三库整包与前端全量测试完成 | SQLite/MySQL/PostgreSQL 与 298 个前端测试文件全部通过 |
| 2026-11-24 | 根模块最终门禁发现测试集成问题 | 修复 SQLite immediate 事务测试屏障、插件测试漏导入和错误脱敏断言；未修改对应生产安全策略 |
| 2026-11-24 | 根模块 test/vet/build 最终复跑 | 全部通过；进入双轴 review |
| 2026-11-24 | 双轴 review 返回三项阻断 | 常见密码筛查、DeepSeek Tools 前置敏感词检查、单渠道复制/删除原子性 |
| 2026-11-24 | 三项 review 阻断完成根因修复与定向回归 | 进入最终全量门禁与双轴复审；未 push、未部署 |
| 2026-11-24 | Spec 复审发现删除 no-op 仍发布同步事件 | 改为按 Channel/Ability 实际变更发布；三库重复/不存在删除回归通过 |
| 2026-11-25 | Standards 与 Spec 最终复审 | 两轴 blockers 均为 0；进入最终暂存与本地 merge commit |

## 14. 提交前验证记录

### 14.1 合并完整性

- `git diff --name-only --diff-filter=U`：0 unresolved path。
- 非 Markdown `<<<<<<<` / `=======` / `>>>>>>>`：0 conflict marker。
- `git diff HEAD --check`：通过。
- 本轮新增 Go 测试代码中无原生 `t.Fatal` / `t.Fatalf` / `t.Error` / `t.Errorf`。

### 14.2 后端与数据库

- 根模块最终状态的 `go test ./... -count=1`、`go vet ./...`、`go build ./...`：全部通过。
- `controller` 整包：SQLite 3.50.4（142.414s）、MySQL 8.0.46（283.509s）、PostgreSQL 16.15（343.462s）全部通过。
- `model` 与 `service` 整包在 SQLite/MySQL/PostgreSQL 全部通过；provider controller 三库矩阵连续 10 轮、兑换码隔离数据库矩阵连续 3 轮通过。
- `relaykit` 与 `tokenkit` 的 test/vet/build：通过。
- SQLite 裸路径统一补 WAL、30s busy timeout 与 immediate transaction；相应 DSN 定向测试通过。
- 全量门禁发现原并发额度测试用双事务查询屏障与 SQLite immediate transaction 互相等待；测试夹具现按数据库锁语义释放屏障，生产事务策略保持不变，三库定向测试均通过。

### 14.3 前端

- 使用官方 Bun 1.4.4 执行 frozen install，无 lockfile 或依赖树变更。
- i18n sync、TypeScript typecheck：通过。
- 本轮 21 个前端修改文件的定向 oxlint 与 `oxfmt --check`：通过。
- Vitest 最终复跑：298 个文件、3273 项测试全部通过，耗时 2099.77s；`scrollTo` 与 canvas 信息为既有 jsdom 环境提示。
- 生产构建：通过，Rsbuild 2.2.0 耗时 21.7s；路由测试文件未导出 Route 的提示为既有警告。
- 全量 lint 基线为 141 errors / 84 files，84 个文件均与两个父提交逐字节相同；全量 format 基线为 29 files，均逐字节来自父提交；两者与本轮 21 个前端修正文件 0 重叠。

### 14.4 安全语义复核

- 上游 provider-plugin 集成测试原先直接期待向客户端返回插件或供应商内部错误，与 Tokendazi 既有错误脱敏策略冲突。
- 生产脱敏逻辑保持不变；测试现同时证明客户端响应不含内部诊断，而同请求的内部 error log 保留原始错误，相关 9 组 router 回归与 router 整包均通过。
- 新密码写入统一执行 10001 项常见口令词表与明显可预测模式筛查，历史密码登录不追溯阻断；前端提供即时反馈但服务端完整词表保持权威。
- DeepSeek `/v1/tools/search` 与 `/v1/tools/fetch` 现映射为 `RelayFormatDeepSeekTools`，敏感文本在渠道选择与计费前完成一次权威检查；无可用渠道时仍先审计或阻断。
- 单渠道 `Insert`/`Delete` 的 Channel 与 Ability 写入现处于同一事务，Ability 失败会整体回滚；删除只在 Channel 或 Ability 实际变化后发布 channels sync topic，重复/不存在删除不发布虚假通知。

### 14.5 待完成

- Standards review 与 Spec review 的首轮阻断项及 no-op 复审项均已修复，最终复审 blockers 均为 0。
- 执行最终 `git add -A`、暂存区完整性检查和唯一的本地 merge commit。
- 不 push、不部署。
