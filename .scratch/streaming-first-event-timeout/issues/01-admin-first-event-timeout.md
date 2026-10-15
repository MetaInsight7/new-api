# 后台可控的首事件超时完整闭环

Status: resolved
Type: task
Blocked by: none
Spec: ../spec.md

## Scope

- 默认关闭的独立流式配置，正整数秒数预填 600，锁保护的请求策略快照。
- 管理选项校验、可靠持久化与重载；模型全局设置紧邻主动保活，先保存时长再启用。
- 共享 SSE scanner 的单次尝试首事件预算；注释、空数据和本地保活不算进展。
- 504 / upstream_first_response_timeout / first_response_timeout 贯通适配器、Relay、结果分类。
- 保留现有响应提交后禁止重试、并发租约和用量规则，关闭 Body 并等待工作退出。

## Acceptance

- Relay HTTP + 可控 SSE 主验收，补公共 scanner 取消/竞争和管理接口回归。
- 设置保存、回读、重载、非法输入和故障不污染运行时；真实 SQLite/MySQL/PostgreSQL。
- 后台交互测试、类型检查、lint/格式和生产构建。

## Comments

- 2026-10-15：用户以 `$implement 开始吧` 批准两张实施票；开始第一张。

## Answer

- 已完成后台配置、同步快照、共享计时器、全部宿主适配器错误传播和普通错误响应头恢复。
- 定向 HTTP/scanner/后台测试、类型检查、涉及文件 lint 和后台生产构建通过。
- 真实 SQLite 3.50.4、MySQL 8.0.46、PostgreSQL 15.19 完成保存/回读/重载及非法配置保护验证；另有保存与同步故障注入。
- OP 既有策略硬性排除 HTTP 504，即使把 504 加入可重试范围也不能覆盖；本功能不改变此规则，不另加 SkipRetry 标记。已写保活仍按 response_already_written 停止。
- 最终联合回归、并发保存保护和双轴审查与 02 一并完成，见 verification.md 和 review.md；后台全量的唯一失败已在原基线复现，不属于本特性。
