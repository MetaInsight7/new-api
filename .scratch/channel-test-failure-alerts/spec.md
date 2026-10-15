# 渠道测试失败钉钉告警与可选 AI 摘要

Status: resolved

Type: task

Branch: tokendazi_op

Created: 2026-10-15

OP baseline: 8b89ed6db061d8fd70ba02f77735ad2f6bef18d0

Solve baseline: 762ce21c429a3aa9adc104bcd80eb7df3ad3bdbd

## Problem Statement

OP 已有自动渠道测试、系统任务调度、受控测试并发、自动禁用和恢复，但没有二开最新版的通用渠道测试失败钉钉告警。管理员如果只依赖已有渠道状态通知，会遗漏“测试已失败，但尚未触发禁用”的故障；批量故障也缺少可选的中文摘要。

OP 之前迁入的 Codex 模型治理使用独立的钉钉和 AI 接入配置，而二开最新版由通用测试告警与 Codex 治理共用“监控与警报”配置。用户明确认定原独立结构源于此前分析不完全，要求直接修正，不增加旧配置迁移、别名、回退等兼容代码。

用户需要对齐二开的告警能力，同时保留 OP 已完成的系统任务、测试模式、并发控制和治理规则。告警不能变成一次额外测试，不能改变现有 API 请求重试，也不能被误解为渠道已被禁用或消息必然可靠送达。

## Solution

在现有 OP 管理后台“监控与警报”中提供共用的钉钉机器人、签名 Secret、渠道告警冷却及 AI 分析 API Key / Base URL / 模型配置。钉钉告警默认关闭，AI 摘要在配置分析密钥后按需启用。

现有自动测试产生失败结果时，复用同一次测试的错误和渠道信息构建渠道测试失败告警；每收集 5 条失败候选发送一批，正常结束时发送尾批。先执行渠道告警冷却筛选，再为实际待发送的告警可选生成 AI 中文摘要，最后发送钉钉。没有分析密钥或分析失败时，保留不含 AI 摘要的基础告警。

通用测试告警与 Codex 治理共用机器人和分析服务接入配置，但保留各自的业务开关、触发条件和冷却含义；共用钉钉开关不等于启停 Codex 治理、主动探测或公告判定。原有 New API 管理员渠道禁用 / 恢复通知继续保留，不与新增测试失败告警合并。

本期采用二开的尽力投递语义：发送失败可观测，但不持久化批次、不自动补发；后续测试仍失败且符合告警条件时，可以产生新的告警。AI 输入和输出均做规则式脱敏，但不承诺完整匿名化或零泄漏。唯一已明确调整的分析配置优先级是：管理员明确保存的接口 / 模型值优先，即使值恰好等于内置默认值。

## User Stories

1. As an operator, I want scheduled channel-test failures reported to DingTalk, so that I can notice faults without opening the administration console.
2. As an operator, I want alerts for failed tests that do not disable a channel, so that a missing status-change notification does not hide a fault.
3. As an operator, I want alerts to reuse existing test results, so that enabling notifications does not create additional upstream test calls.
4. As an operator, I want Codex channels included when they participate in automatic tests, so that ordinary test failures are not excluded by provider type.
5. As an operator, I want manual tests to retain their existing notification behavior, so that troubleshooting does not unexpectedly generate scheduled-failure alerts.
6. As an operator, I want all existing test-selection modes preserved, so that adding alerts does not change which channels are tested or recovered.
7. As an operator, I want bounded test concurrency and system-task ownership preserved, so that notifications do not bypass OP's execution controls.
8. As an operator, I want existing channel-disable and recovery notifications retained, so that status changes outside scheduled tests remain visible.
9. As an operator, I want a failed test and a successful channel disablement distinguished, so that an alert does not falsely prove that a channel was disabled.
10. As an operator, I want one shared DingTalk configuration for generic alerts and Codex governance, so that I do not maintain conflicting notification destinations.
11. As an operator, I want one shared analysis-service configuration, so that both features use the intended credentials, endpoint and model.
12. As an operator, I want the obsolete independent Codex notification settings removed, so that the console no longer offers two competing configuration sources.
13. As an operator, I want the obsolete settings replaced without compatibility fallback, so that incorrect earlier design does not become permanent architecture.
14. As an operator, I want new DingTalk alerts disabled by default, so that installing the feature does not silently start sending operational data.
15. As an operator, I want to configure a signed DingTalk robot, so that the feature works with robots that require a signing Secret.
16. As an operator, I want shared settings to survive a restart, so that notification behavior does not silently reset.
17. As an authorized administrator, I want settings protected by existing management permissions, so that ordinary users cannot change global destinations or analysis credentials.
18. As an operator, I want saved analysis keys and signing secrets not to be echoed back, so that loading a settings page does not disclose usable credentials.
19. As an operator, I want leaving a password field blank to preserve its saved value, so that editing an unrelated setting does not erase a working credential.
20. As an operator, I want failed or invalid saves to be visible, so that the console does not claim that an unusable configuration was enabled successfully.
21. As an operator, I want channel-specific alert cooldowns, so that a continuously failing channel does not generate a message on every test cycle.
22. As an operator, I want generic alert cooldowns separate from Codex governance cooldowns, so that sharing a robot does not merge unrelated suppression policies.
23. As an operator, I want small failure batches, so that multiple failures are readable without waiting for an entire large test cycle to finish.
24. As an operator, I want optional Chinese AI summaries above the original alert facts, so that I can quickly understand a batch without losing the underlying evidence.
25. As an operator, I want AI analysis skipped when no analysis key is configured, so that basic DingTalk alerts do not depend on an external model.
26. As an operator, I want an AI error, timeout or empty answer to fall back to the base alert, so that summary generation is not required for notification delivery.
27. As an operator, I want explicitly saved endpoint and model values to take precedence over environment variables, so that the service I configured is the service actually used.
28. As an operator, I want environment variables or built-in defaults used only when the corresponding value has not been configured, so that deployment defaults remain available without overriding explicit choices.
29. As an operator, I want known credential patterns masked before analysis and again in AI output, so that common secrets are not exposed through alert summaries.
30. As an operator, I want to know that channel identifiers and fault facts can still be sent to the analysis service, so that I do not mistake masking for complete anonymization.
31. As an operator, I want finite analysis and delivery budgets, so that an unavailable external service cannot hold the task indefinitely.
32. As an operator, I want shared-database cooldown coordination across instances, so that normal concurrent send attempts suppress duplicate alerts for the same channel.
33. As an operator, I want failed delivery not to consume a successful-delivery cooldown, so that a later eligible test can try again.
34. As an operator, I want failed batches logged without automatic replay, so that the feature has a clear best-effort scope rather than an implied reliable queue.
35. As an operator, I want cancellation and lease loss to stop outstanding alert work, so that an obsolete task runner does not continue external analysis and delivery independently.
36. As an operator, I want alert failures to leave test results and governance decisions unchanged, so that a broken robot does not disable or recover channels differently.
37. As a maintainer, I want SQLite, MySQL and PostgreSQL behavior verified on real engines, so that cooldown persistence does not work on only one supported database.
38. As a maintainer, I want external behavior tested through existing task and settings boundaries, so that implementation refactors do not require a duplicate test suite for every layer.

## Implementation Decisions

### 1. 迁移范围与 OP 边界

- 按固定二开基线迁入通用渠道测试失败钉钉告警、可选 AI 摘要、后台配置及其必要的渠道告警冷却能力；适配 OP，不整体替换二开的渠道测试控制器或后台循环。
- 保留 OP 的系统任务排队、active key、执行租约、心跳、进度、取消、既有结果统计及测试并发上限，不另起绕过系统任务的巡检循环。
- 保留 `scheduled_all`、`auto_ban_only`、`passive_recovery` 三种模式及其现有渠道筛选、禁用和恢复权限。测试渠道集合由既有模式决定，新增告警不扩大集合，也不额外遍历渠道的全部模型。
- 模块职责分为：既有测试任务生产失败事实；告警服务处理批次、脱敏、可选摘要与投递；持久化层协调渠道告警冷却；既有选项和后台表单管理共享配置。无需为每个步骤新建抽象层或增加测试专用业务入口。

### 2. 触发与结果归并

- 新告警只消费既有自动任务实际产生的测试错误，行为对齐二开的非手动通知路径。包含实际参与测试的 Codex 渠道，不按品牌排除。
- 手动单渠道测试、手动全量测试及其现有完成通知不接入该自动失败推送；普通用户 API 请求失败不直接产生本功能告警。既有请求错误处理和状态变更通知不受影响。
- 使用经过既有测试逻辑判定的最终错误，包括现有响应时间阈值产生的失败；不能用告警服务重新判断渠道健康、自动禁用或恢复。
- 失败事实包含渠道 ID、名称、类型、错误状态 / 代码 / 描述、自动禁用相关标记和时间。错误来自同一次测试，不追加调用被测渠道，也不让 AI 复测渠道。
- 既有有界 worker 完成后，由串行结果归并路径累计失败候选；每 5 条触发一次批次投递，正常完成发送尾批。保持进度和统计串行更新，不让各 worker 竞争同一个不受保护的切片或独立启动无限发送任务。
- 批次内先按渠道冷却筛选，再构建原始告警与可选摘要；没有可发送条目时不调用 AI 或钉钉。正常并发下不依赖渠道测试完成顺序来定义业务正确性。
- 告警不是渠道禁用成功的证明。禁用相关字段只能表达已有证据，不把“已触发异步禁用流程”误写为“已确认持久化禁用成功”，也不为告警重写原有禁用流程。

### 3. 共享配置与去除旧设计

- 机器人配置统一使用 `monitor_setting` 下的 `dingtalk_alert_enabled`、`dingtalk_alert_webhook_url`、`dingtalk_alert_secret`、`dingtalk_alert_cooldown_minutes`。
- 分析服务统一使用同一设置域下的 `ai_analysis_api_key`、`ai_analysis_base_url`、`ai_analysis_model`；分析密钥独立于被测渠道密钥和钉钉签名 Secret。
- 默认值对齐二开：钉钉关闭，Webhook / Secret / 分析密钥为空，通用渠道告警冷却 60 分钟，分析 Base URL 为 `https://api.openai.com/v1`，模型为 `gpt-5.4-mini`。摘要没有额外强制启用开关，按分析密钥是否配置决定是否调用。
- Codex 治理通知读取同一机器人开关、目标和签名配置；原 Codex 公告 AI 分析只改为读取共享接入配置，保留原分析触发条件、协议、证据门槛和审核语义，不用通用摘要替换原治理分析。
- Codex 治理业务开关、探测开关、规则和自身 `alert_cooldown_minutes` 保留。共用机器人及 AI 接入不合并通用渠道冷却与治理冷却，也不把机器人开关当成治理总开关。
- 从旧 Codex 设置结构、后台表单和运行读写中移除独立的钉钉 / AI 接入字段；治理页面可以指向共享设置，但不能保留第二套可编辑凭据来源。
- 不搬迁旧独立配置、不设置旧键别名、不读取旧值回退、不增加版本兼容开关。旧独立记录即使仍存在，也不能成为有效通知或分析配置；旧字段不能继续通过管理写入改变运行行为。
- 无兼容不意味着删除用户数据库中的旧记录；本期不做部署数据清理。共享配置为空时保持相应能力未配置，不能静默找回旧密钥。需要配置时由管理员在共享入口重新填写。

### 4. 分析配置优先级

- 管理员明确保存的共享接口 / 模型值优先，即使值等于内置默认值；未配置对应项时才使用二开支持的环境变量，再回退内置默认值。
- “曾明确保存”必须与“注册默认值在内存中存在”区分，不能按非空或等于默认值来推断。优先利用持久化选项键的存在性及现有设置加载机制表达该差异，热更新与重启后语义一致。
- 计算环境回退不能反向改写持久化记录，不能把环境值或默认值伪装成管理员已保存的值。后台应让管理员确认实际使用的接口 / 模型，不回显分析密钥。
- 此优先级是用户接受的有意偏离二开之处，不以“完全照搬”为由恢复环境变量覆盖显式默认值的行为。

### 5. 后台与凭据保存

- 扩展 OP 既有“监控与警报”表单、共享设置组件及受保护的选项 API，不增加另一个配置管理页面，不涉及 static 分支或静态控制台。
- API Key 和签名 Secret 使用敏感输入；已保存值不通过选项回读或表单重新显示。表单留空表示保持已存值，不因编辑其他设置而发送空串清除密钥。
- 移除旧独立入口不等于将共享表单的空白解释成清除；显式替换 / 清除若使用项目已有能力，必须是明确用户动作，不创建旧配置兼容来源。
- 服务器和表单校验值类型、冷却范围及启用所需配置；冷却使用正整数分钟，最少 1 分钟，并拒绝非数字、非有限值和时长溢出。按二开运行语义，异常加载的非正冷却值兜底为 60 分钟，0 不表示关闭冷却。启用前先成功保存必需的目标及相关配置，失败时不继续宣称启用成功。
- 沿用现有管理读取 / 写入授权与请求安全边界；只读或普通用户不能写全局设置。新文案遵守现有 i18n 和可访问性规范。

### 6. 脱敏与 AI 摘要

- 基础告警中的渠道名称、类型和错误文本先做规则式脱敏。送往分析服务前对完整告警再次脱敏并限长；AI 返回的文本也必须再次脱敏，不能因模型声称不会泄密而省略处理。
- 对齐二开的已知凭据模式：`access_token`、`refresh_token`、`id_token`、`api_key` / `api-key` / `apikey`、`authorization` 及其值，`sk-...`、AWS `AKIA` / `ASIA`、Google `AIza...`；兼顾普通文本及带引号的 JSON 风格，并复用现有 URL、域名和 IP 掩码。
- 脱敏不是完整匿名化。渠道 ID、通常的渠道名称和故障事实仍可能发给所配置的分析服务；未知格式凭据、个人信息和上游错误回显中的业务内容不保证全部识别。后台说明不能宣传“绝不泄露”或“已去除全部标识”。
- 不主动拼入用户请求正文、被测渠道密钥、钉钉凭据或其他无关配置。分析 API Key 仅作为向分析服务认证的凭据，钉钉凭据仅用于对应机器人投递，不加入模型输入。
- 通用摘要对齐二开的非流式 Chat Completions 调用、中文摘要和事实约束：只总结当前批次，不臆造根因或修复事实，不输出治理执行指令作为权威决策。
- 分析请求输入上限为 12000 个 rune，响应体上限 64 KiB，模型输出预算 600 completion tokens，最终摘要最多 1000 个 rune / 5 个要点。过大、无法解析或不合要求的响应按分析失败处理。
- 分析预算最长 60 秒，并受更早的任务取消约束；任务仍有效时，分析失败、自身预算超时、空结果或无效输出继续回退基础告警，成功时在原告警之前附加中文摘要。父任务取消优先，不在取消后执行回退发送。分析失败后的回退可能已经等待完整分析预算，不宣称为即时回退。
- AI 仅辅助阅读，不修改渠道状态、模型能力、禁用 / 恢复结果或 Codex 治理证据规则。

### 7. 钉钉投递与取消

- 按二开支持机器人 Webhook 和可选签名 Secret。投递使用有限请求和响应体预算；钉钉 HTTP 请求预算为 10 秒，响应体上限 64 KiB。
- 沿用 OP 已有受保护的外部 HTTP 能力和 URL / SSRF 检查，不因复制二开代码退化为绕过域名、地址解析、重定向或网络访问策略的独立客户端。错误日志同样不能携带完整 Webhook Token、签名 Secret 或分析认证头。
- 只有钉钉明确返回成功才认定已投递；非成功 HTTP、机器人业务错误、无法解析或过大响应均视为投递失败，不仅凭 HTTP 200 认定成功。
- AI 和钉钉调用必须派生自当前系统任务 context，而不是独立 `context.Background()`；任务取消或失去租约后，不启动新批次、不在清理时强制发送尾批，未完成 HTTP 应可中止。
- 未发送批次在取消时丢弃，不转移给无任务所有权的后台协程；按已有预占 token 释放自己的 pending 状态。释放预占可以做有界必要清理，但不能借清理继续生成摘要或发送消息。
- 已发送消息不能撤回，网络超时也不能证明远端从未接收。保留 OP 租约协调不等于承诺外部通知恰好投递一次。

### 8. 渠道告警冷却及数据库

- 新增或适配按渠道唯一的持久化告警冷却记录，仅保存成功发送时间、待发送预占及所有权 token，不保存待投递消息正文，不构成 outbox。
- 正常情况下以共享主数据库事务和数据库时间协调预占，防止不同实例同时为同一渠道发送；行锁遵循 OP 统一锁封装及 SQLite / MySQL / PostgreSQL 兼容规则，不能照搬二开的直接锁写法绕过项目约束。
- 预占在分析和投递之前取得；pending 窗口覆盖分析与发送预算，对齐二开当前 80 秒边界。token 限定提交和回滚，过期持有者不能清除或提交后来实例的新预占。
- 发送失败尝试释放本批持有的预占，不写入成功冷却；成功后提交冷却。发送成功但提交冷却失败时记录错误，不把已发消息重新当成待发消息立即重投。
- 对齐二开：数据库预占故障或数据库不可用时退回进程内冷却，并记录降级；此时不保证跨副本或重启后去重。预占 token 生成失败不能绕过协调直接发送。
- 冷却不改变测试筛选或渠道调度；不得把渠道告警冷却复用为并发冷却、渠道禁用或模型治理冷却，不新增“0 表示关闭冷却”的不同语义。
- 为主数据库迁移注册必要记录，兼容 fresh / upgrade 及重复启动；不改已有治理冷却记录含义，不对独立日志数据库新增无关表。

### 9. 尽力投递与原有通知

- 发送失败记录脱敏错误，批次处理结束即清空；没有持久化待投递队列、自动补发、重启恢复或逐消息投递重试。后续测试可以重新产生告警，不是重放旧批次。
- 告警服务错误不能改变测试成功 / 失败统计、自动禁用 / 恢复判断或将原本正常执行的测试任务改判为渠道测试失败；维持既有任务完成和错误分类边界。
- 原有 New API 管理员渠道状态通知保持原配置、触发与通知限流，新增钉钉失败告警保持自己的渠道告警冷却；两种事件不做跨类型合并。
- 同一次自动测试失败并成功触发禁用时，两种通知都可能出现，且不一定送往同一接收位置；没有触发禁用时，不凭测试失败额外伪造一条禁用通知。原有实际请求触发的禁用 / 恢复通知继续保留。

## Testing Decisions

### 测试边界

- **主 seam：现有渠道测试系统任务执行入口 → 真实渠道测试链路 → 结果归并 → 告警外部 HTTP。** 优先在这一条闭环验证调用次数、失败事实、批次、冷却、AI 可选性、投递及取消，模拟的仅是不可控的被测上游、分析服务和钉钉 HTTP，不 mock 被测业务判断。
- **必要辅助 seam：现有受保护选项保存 / 回读 API 与后台设置表单。** 验证保存后的共享设置确实被主执行路径和 Codex 读取，而不是只断言 getter 常量或控件存在。
- 对跨实例预占、token 过期及数据库事务竞争等主 seam 不易稳定构造的契约，复用现有持久化测试设施作最小补充。不得为控制器、服务和模型各复制一份相同的整链路矩阵。
- 既有测试先例包括 `controller/channel_test_internal_test.go` 中的模式筛选、并发与取消用例，`controller/codex_model_governance_test.go` 中的任务租约用例，以及 `service/codex_model_governance_test.go` 中的受保护 HTTP、AI 失败回退和确认投递后写冷却用例。
- 后台先例为 `web/src/features/system-settings/integrations/__tests__/codex-governance.test.tsx` 中的选项加载、密钥不回显、空白保留和保存失败用例；迁移后需要更新其共享配置预期，而不是保留旧独立配置来让测试通过。
- 优先扩展现有适合的测试文件；整个聚焦功能至多新增一个后端测试文件，不因调用链跨层就每层新建一份。新 / 重写断言遵守项目 require / assert 规范。
- 好测试保护外部可见行为、数据库一致性和权限契约；不以私有函数数量、结构体布局、日志文本相等、固定 sleep 或堆砌重复用例代替验证。时间竞争使用可控同步或时钟，避免真实等待 60 秒 / 60 分钟。

### 核心行为矩阵

1. 默认关闭或显式关闭时不发钉钉、不为通用摘要调用 AI；原测试次数、统计、状态变化及原通知路径保持不变。
2. 自动测试失败产生含正确渠道和错误事实的告警；成功测试不告警。包含实际参与测试的 Codex 渠道，三种 OP 模式继续保持各自渠道范围与禁用 / 恢复权限。
3. 手动单测、手动全量及用户请求失败不额外触发该自动测试告警；手动完成通知、真实请求触发的原始状态通知保持原有语义。
4. 开启告警、配置 AI、启用冷却都不能增加被测上游调用次数或遍历全部模型；用可控 HTTP 调用计数证明是同一份测试结果。
5. 5 条失败候选触发批次，正常结束发送剩余不足 5 条的尾批；并发完成不会丢条目、重复累计或让进度统计产生竞态。
6. 按渠道而非整个系统冷却；同一渠道成功发送后在冷却内抑制，其他渠道仍可发送；全部被冷却抑制时无 AI / 钉钉请求。合法最小值为 1 分钟，异常非正值按二开兜底 60 分钟，不误作无限制发送。
7. 无分析密钥时发送基础告警；有密钥时只把当前合格批次的脱敏文本发送给配置的接口 / 模型，成功结果以中文摘要加基础告警呈现。
8. 分析 HTTP 失败、超时、空输出、无效 / 过大响应回退基础告警；普通摘要错误不能令已失败的测试被重新执行或改变渠道状态。
9. 已知凭据在 AI 请求正文、钉钉文本和失败诊断中不可见；AI 返回的新凭据样式也经过脱敏。渠道 ID 和通常的名称仍保留，测试不宣称任意未知数据均可识别。
10. 验证输入与输出限长、响应体限额、签名开启与关闭，以及 HTTP 200 但业务失败不能写成功冷却。
11. 钉钉发送失败时释放自己的预占且不写成功冷却；以后符合条件的新失败可再告警；渠道恢复后不补发上一次遗失的旧批次。
12. 发送成功但冷却提交失败时不立即重复投递，仍留下可观测错误；不把冷却记录当作消息队列。
13. 多发送者对同一渠道正常情况下仅有一个有效预占；pending 过期可以接管，旧 token 的提交 / 回滚不覆盖新所有者。数据库降级时验证本地冷却，并明确跨实例抑制不受保证。
14. 任务取消或模拟租约丢失时，中止正在分析 / 投递的请求，不发送尚未开始的批次或尾批，不遗留无限后台发送工作；取消前已发消息不要求撤回。
15. 测试失败但未禁用时只有失败告警；成功触发原有禁用通知时两类通知都可出现；原有恢复通知仍在。不能用“触发禁用流程”冒充已确认的禁用成功。

### 后台与共享配置矩阵

1. “监控与警报”覆盖机器人开关、目标、签名、通用冷却和三项分析配置；默认值、分钟单位和必要说明正确，表单使用已有组件和 i18n。
2. 通用告警和 Codex 通知使用同一机器人；Codex 公告分析读取同一分析接入配置，但保留原任务、分析协议、治理开关、规则和自身冷却。旧独立设置不再参与运行。
3. 仅预先写入旧 Codex 钉钉 / AI 记录时，新共享设置仍未配置，不自动开启、不搬迁、不回退；尝试使用已移除旧字段不能改变实际通知和分析行为。
4. 密钥与签名 Secret 不回显，空白输入不覆盖已有密钥，替换或明确清除的动作不能被误当作留空；不要新增伪配置字段代替明确保存契约。
5. 未授权写入被拒绝；非法冷却、无必需目标的启用和保存失败可见，失败时不顺带启用功能。现有合法治理选项不因删除接入字段被误拒。
6. 分析接口 / 模型未配置时使用环境变量或内置默认值；显式保存默认字符串后必须压过冲突环境变量，热更新与重启后均成立；页面显示不能把环境回退误说成已保存值。

### 数据库与交付验证

- 实际执行 SQLite、MySQL 和 PostgreSQL 三引擎验证，不能以 mocks、仅 SQLite 或编译通过代替。至少使用每类一个受支持版本；若语法或锁行为依赖版本，则补测项目最低支持版本。
- 必须覆盖 fresh、从代表性既有版本升级、重复迁移 / 启动至少两次，以及既有渠道、系统任务、治理记录、索引和唯一性未受损。旧配置不自动搬迁，但其存在不能破坏启动。
- 验证真实事务竞争、成功冷却持久化、失败释放及 token 所有权；确认普通配置重载 / 重启不丢失管理员明确保存状态。
- 按改动范围运行聚焦 Go 回归、适用的 race 检查、后台 Vitest、类型检查 / lint 与构建；记录真实命令、数据库版本、结果及尚未覆盖的外部边界。
- 使用本地可控 HTTP 验证，不使用真实钉钉机器人、付费分析 Key 或生产凭据。真正公网投递和真实模型摘要如未验证，交付时明确说明。

## Out of Scope

- 旧 Codex 独立凭据的自动迁移、兼容别名、回退读取、双配置并存及部署数据库旧记录清理。
- 替换 OP 系统任务或三种测试模式、降低现有并发能力、增加第二次测试、全模型探测或用户请求逐条告警。
- 修改 API 请求重试、渠道调度、首事件超时、主动保活、计费或 Codex 模型治理决策规则。
- 替换原 New API 管理员渠道禁用 / 恢复通知，跨类型通知合并或建立统一通知中心。
- 持久化消息正文、可靠消息队列、自动重试补发、重启重放、完整投递历史和恰好一次投递承诺。
- 完整匿名化、任意秘密自动识别、AI 自动修复 / 禁用权限，以及把分析输出作为健康或治理权威证据。
- 迁移二开广告运营、支付告警、公告翻译、其他监控能力、static 分支或静态控制台。
- 新增第三方分析依赖、设置全局认证重构、泛化全项目 HTTP 客户端或借迁移修复无关问题。

## Further Notes

- 用户已确认 Q1 共用监控配置、Q2 自动测试触发范围、Q3 二开可选摘要与脱敏流程、Q4 不做旧结构兼容、Q5 尽力投递、Q7 显式保存优先；最后以 `$to-spec 没问题` 确认 Q6 保留官方原通知和新增二开失败告警，授权生成本 spec，而非开始实现。
- 详细问答和源码锚点保存在 [grilling.md](grilling.md)，工作入口为 [map.md](map.md)；领域术语遵循根 GLOSSARY，渠道告警冷却不与并发冷却混用。
- 对齐对象是可访问镜像 `https://github.com/MetaInsight7/new-api-solve.git` 的固定 SHA。最近一次 `git ls-remote` 核对 HEAD / main 仍为本文基线；不代表其他私有仓库或作者部署配置。
- 默认关闭、5 条批次、60 分钟通用冷却、60 秒分析预算、脱敏限制和尽力投递来自固定二开事实；OP 的三种模式、任务租约、取消、受保护 HTTP 与独立治理规则来自现有实现。无兼容与配置优先级是用户明确指定的设计选择。
- 不新增 ADR：当前是可聚焦实现的功能迁移和已确认的配置纠偏，没有另行批准需要写入架构决策目录的文档。
- 实施前遵守项目关于 API 凭据、服务端授权、敏感日志及 SSRF 的安全要求，并核对适用 OWASP 指引；本 spec 不等于已完成安全审计或合规验证。
- 本轮只发布本地 spec 和同步访谈记录，未实现业务代码、运行功能测试、调用真实外部服务、暂存或提交。

## Implementation Outcome

- 后续用户以 `$implement 没问题` 授权三张 ticket 的实施、双轴审查和当前分支提交；实现及验收已完成，历史访谈和范围不变。
- 结果与已知基线问题见 [verification.md](verification.md)，独立 Standards / Spec 审查及修正见 [review.md](review.md)。未使用真实生产机器人、分析密钥或数据库。
