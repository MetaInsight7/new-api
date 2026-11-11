# P-30 敏感词内容审计重构

## 范围与基线

本设计在官方 `QuantumNous/new-api` 的 `d04c118c8803f49e0c9bab74dcf5b5efeab9464a` 基线之上重建 P-30，不复用本 fork 既有 P-30 提交。它只涵盖请求进入 Relay 前的敏感词检查、审计、违规计数、阈值停用和管理员管理界面；不改变额度、已用额度、订阅、钱包、历史账务或上游重试策略。

旧 `SensitiveWords`、`SensitiveWordConfig` 和旧 `word/enabled` 规则列只作为一次性迁移输入。用户白名单只使用新的 `users.sensitive_word_whitelist` 字段。运行时不再读取旧 Option，也不再使用旧 `RequestPolicySnapshot.CheckText/TextKeywords` 或 `service/sensitive.go`。

## 数据流

```text
协议 DTO 提取可检查文本
        |
middleware.Distribute：有效分组解析完成、选渠道前预检
        |
候选分组 + Aho-Corasick 快照（单次扫描）
        |
主库事务：锁定用户、审计、计数、必要时禁用
        |
  400 invalid_request_error / sensitive_words_detected
  达到封禁阈值时 403 permission_error / user_banned
  或  缓存决定 -> 选渠道 -> 估算/预扣费/上游
```

`middleware.Distribute` 在协议请求已可解析、有效分组已确定但尚未调用 `SelectChannelForRequest` 时执行策略，因此本地阻断优先于无可用渠道或模型不存在。Responses WebSocket 不经过 `Distribute`，其已解析的 `response.create` DTO 会调用同一预检函数，位置同样位于渠道选择和上游握手之前。结果放入请求上下文；`PrepareRequestBilling` 复用该结果，以确保一个请求只匹配和审计一次，并在未经过分发中间件的调用场景保留同样的计费前兜底。普通阻断返回不可重试的 HTTP `400`，OpenAI 兼容错误使用 `invalid_request_error / sensitive_words_detected`；当次命中触发自动封禁时立即返回不可重试的 HTTP `403 permission_error / user_banned`，之后的令牌鉴权保持同一 `403` 语义。Claude 路径使用其原生错误外壳并返回请求 ID，Gemini 原生路径返回 `{error:{code,message,status}}`。敏感词响应不携带 `Retry-After`；该头只用于临时限流。系统默认的普通拦截及自动封禁文案按用户设置、`Accept-Language` 或站点默认语言返回简中、繁中或英文；管理员自定义普通拦截文案是单一文本，按配置原样返回。对外响应不暴露命中词、规则、证据、累计次数或准确封禁阈值，这些信息只在管理员审计面板提供。

受支持 DTO 通过独立的结构化文本提取接口进入检查，不再复用 Token 统计的 `CombineText`。提取器覆盖通用 Relay 中的 system、developer、user、assistant、tool、普通 Prompt、Embedding、Rerank、图片和音频文本字段；运行时对每个独立会话选择包含最新 user 输入或文本型工具结果的最高输入轮次，只匹配该轮的 user 文本和工具结果。没有新 user 文本的纯工具结果请求仍会被检查；历史轮次以及 system、developer、assistant、工具定义、工具参数和响应 Schema 只作为命中后的审计上下文候选。纯媒体或空白 user 消息通过不含文本的内部边界推进轮次，该边界不扫描、不留存。Gemini 批量聊天和批量 Embedding 中的每个子请求视为独立会话，分别检查各自的最新输入轮次，不能由批次中的后续子请求遮蔽。结构字段、ID、URL、媒体、Base64 和二进制不参与匹配。Responses 的 `function_call_output.output` 支持字符串、内容数组和 JSON 对象，且提取过程不改变请求原始 wire shape。

视频生成、通用异步任务、Midjourney、Suno、任务插件自定义字段和 OpenAI Realtime WebSocket 事件明确不进入敏感词风控。Responses WebSocket 的 `response.create` 属于 Responses 协议，仍进入相同检查链路。

## 规则与运行时

规则由 `sensitive_word_rules`、`sensitive_word_rule_words` 和 `sensitive_word_rule_groups` 组成：

| 约束 | 值 |
| --- | --- |
| 作用域 | `global` 或使用真实定价分组的 `group` |
| 模式 | `block`、`observe`、`off` |
| 新规则默认模式 | `observe` |
| 单词条长度 | 最多 200 个字符 |
| 单规则词条数 | 最多 10,000 |
| 匹配 | 大小写不敏感的 Aho-Corasick |

规则、策略或模式更新在同一事务中递增 `sensitive_word_policy.version`。提交后当前节点立即重建不可变本机快照，并通过 Redis `new-api:config-changed` 的 `sensitive_words` scope 通知其他节点；遗漏通知的节点按 `SYNC_FREQUENCY` 周期检查数据库代次并收敛。刷新失败会记录运维错误并继续使用最后一个有效快照，不在每个请求读取数据库版本。自动分组按候选顺序评估，单次 automaton 扫描收集候选分组可用的全局和局部规则，避免按候选分组重复扫描提示词。

## 策略、审计与账号状态

单例 `sensitive_word_policy` 保存策略。新库默认启用请求文本检查、关闭自动封禁，阈值为 50，完整证据留存 180 天。`block` 和 `observe` 规则都只扫描最新输入轮次中的 user 与工具结果字段，历史轮次和其他角色不触发匹配；`observe` 命中会继续收集，首个适用的 `block` 命中被保留后立即停止扫描剩余文本。命中按“规则 ID + 词条”去重并最多保留 256 个，同一次请求只写一条审计并最多累计一次违规。`max_prompt_runes` 是单个证据字段的字符上限，不是当前轮、全部轮次或扫描文本的总上限：命中字段超过该值时，优先围绕首个 `block` 命中截取窗口，没有 `block` 时围绕首个 `observe` 命中截取；其他命中通过位置和短片段保留，非命中上下文字段仍从开头截断。每个选中字段最多保存一次，整份结构化证据序列化后最多 512 KiB。证据优先保留命中字段、system/developer 和命中轮次之前三轮，超过任一证据上限时记录截断状态。每日系统任务每批最多处理 500 条，到期清空结构化证据、兼容完整 Prompt、脱敏预览和命中片段，但永久保留审计事实、处置结果和账号违规次数。

审计写入 `sensitive_word_audit_events`，包含请求、协议、分组、规则版本、命中信息、提示词哈希、证据留存状态、违规次数和前后账号状态。请求 ID 与用户 ID 共同保证重复请求不会重复计数。

所有会改变用户安全状态的路径都在同一事务内使用 `lockForUpdate(tx)`：

- 阻断命中：仅增加 `sensitive_word_violation_count`。
- 达到阈值：仅更新 `status`、违规次数、`auth_version` 和敏感词自动封禁来源标记。
- 普通账号状态管理：显式启用或禁用账号时接管状态、清除自动封禁来源标记并保留敏感词违规次数。
- 专用敏感词解封：只接受当前确由敏感词自动封禁的禁用账号，启用账号、清除来源标记并清零敏感词违规次数。

提交后，自动封禁和专用解封都会依次尝试 `PublishUserAuthCache`、`RevokeAllUserSessions`、`InvalidateUserTokensCache`。这些异步失效步骤任一失败不会回滚已提交的安全状态，也不会阻止剩余步骤执行；系统日志会记录失败以便运维补偿。封禁和解封均不修改余额、额度、订阅、历史账务或 API Key 的持久状态。

## 日志与隐私

命中事件写入使用日志类型 `8`。新日志的公开部分仅写入 `other.action=sensitive_word_block`；管理员投影只额外携带用于打开独立审计记录的 `audit_id` 和不含内容证据的处置摘要。规则、命中词、Prompt 哈希和结构化上下文只保存在独立审计表，不写入使用日志。普通用户投影会移除整个管理员字段，并继续清理历史记录中的顶层 `audit_id` 和 `keyword_filter`。

完整证据只由 `GET /api/log/sensitive-word-audit/:id` 返回。该接口除 `AdminAuth` 外还要求独立的 `sensitive_word.evidence_read` 权限；非 Root 即使拥有此权限，也不能读取 Root 的证据。用户使用日志及普通用户 API 不返回证据、命中词或审计 ID。

## 管理接口与界面

保持以下接口：

- `GET|PUT /api/sensitive-words/policy`
- `GET /api/sensitive-words/groups`
- `GET|POST /api/sensitive-words/rules`
- `GET|PUT|DELETE /api/sensitive-words/rules/:id`
- `PATCH /api/sensitive-words/rules/:id/mode`
- `PATCH /api/sensitive-words/users/:id`
- `POST /api/sensitive-words/users/:id/unban`
- `GET /api/sensitive-words/dashboard`
- `GET /api/log/sensitive-word-audit/:id`

规则、策略、白名单、违规次数和专用解封接口要求 `sensitive_word.manage`；该权限默认授予内置管理员角色。完整证据读取使用独立的 `sensitive_word.evidence_read`，默认不授予非 Root。普通管理员没有管理权限时，用户列表和详情会隐藏真实白名单与违规次数。

管理界面位于 `/system-settings/request-policies/filtering`。旧 `/system-settings/security/sensitive-words` 路由保持重定向。运营看板位于 `/sensitive-word-monitor`；管理权限允许查看汇总、趋势、用户、模型、分组和处置事实，只有同时具备 `sensitive_word.evidence_read` 时，Dashboard API 才返回规则排行、最近事件的规则名和审计 ID，前端才展示证据入口。最近事件可直接打开受独立证据读取权限保护的多轮上下文，并复用既有接口执行白名单切换、违规次数清零和敏感词专用解封；完整使用日志跳转仍作为追踪入口保留。规则弹窗支持一行一个词、TXT 导入、大小写不敏感草稿搜索、首次命中选择、Enter/Shift+Enter 循环命中，并在关闭时清空搜索状态。用户编辑抽屉可维护白名单和违规次数；专用解封只在具有权限且目标账号当前由敏感词自动封禁时显示。普通启用账号不会隐式清零。

## 单向迁移

启动先自动迁移新表与用户字段，再运行 `MigrateSensitiveWordData`。迁移先为历史审计补齐请求身份、去重，再建立 `(user_id, request_id)` 唯一索引；随后迁移策略、旧规则词条和旧 Option。合规词条立即导入；单词超过 200 个字符或单规则去重后超过 10,000 个词时跳过超限项、记录运维告警并保留旧数据源原值，不因单个坏词关闭全部保护。数据库或结构性迁移步骤成功后写入当前 `SensitiveWordRulesMigrationVersion` 标记。所有节点都要求该标记存在才启用新运行时，避免从节点在主节点导入旧数据期间使用部分规则。标记存在后旧值永不重新成为运行时权威；数据库或结构性失败使新运行时保持未启用，服务本身继续启动。

迁移矩阵覆盖全新库、RC40 升级和初版 P-30 结构，并要求连续执行两次结果一致。MySQL 的完整提示词及结构化证据列为 `MEDIUMTEXT`，PostgreSQL 与 SQLite 为 `TEXT`。

## 部署与故障语义

单机不需要新增环境变量。多实例必须共享主数据库；建议共享 Redis 以获得即时刷新，并保持各节点 `SYNC_FREQUENCY` 大于 0，以便通知丢失时仍通过数据库代次收敛。滚动发布时先备份数据库，由 master 节点完成迁移并确认迁移标记，再启动其余同版本节点；slave 不执行迁移，只在标记完整后加载规则。

运行时遵守以下故障边界：

- 从未取得有效快照且迁移已完成时，策略加载失败返回不可重试 `503`；已有有效快照的节点刷新失败继续使用最后有效版本并告警。
- 迁移尚未完成或迁移明确失败时，敏感词运行时保持未启用，不阻止服务启动。
- `observe` 命中的审计持久化失败时放行并告警；`block` 命中的审计持久化失败时仍返回 `400 invalid_request_error / sensitive_words_detected`，但不伪造未提交的审计 ID、计数或封禁。
- 普通使用日志写入失败不回滚已提交的审计和账号处置；认证缓存发布、会话撤销或 Token 缓存失效的单项失败也不会阻止其余失效步骤。
- Redis 不可用不会让已有快照失效；恢复后即时通知继续工作，周期代次检查负责最终一致。

## 安全控制与排障

设计对应 OWASP ASVS 5.0.0 的 V7.4.1、V7.4.2、V16.2.5、V16.3.3：状态转换经行锁与事务保护，认证版本和会话/令牌缓存失效，敏感证据按角色隔离，审计失败不会静默放行。

出现意外放行时，依次检查：策略是否启用、迁移标记和本机快照是否可用、规则模式/作用域、令牌的实际或自动候选分组、DTO 是否实现结构化文本提取、主库审计写入及类型 8 日志。对于 Responses WebSocket，还要确认 `response.create` 已进入渠道选择前的共享 DTO 预检。视频、通用异步任务、MJ、Suno、插件自定义字段和 Realtime 不应出现在审计中。出现 `503` 时优先检查策略表、迁移标记、数据库连接和首次快照加载；出现 `403 user_banned` 时检查用户状态、`auth_version`、会话与令牌缓存失效日志。
