# 首事件超时双轴审查

Date: 2026-10-15
Branch: tokendazi_op
Base: cf0105704687bdbcbfef54546e488a029c3d0655
Spec: [spec.md](spec.md)
Status: findings closed

## Review Input

- 首轮审查基于暂存的实施差异：`git diff --cached cf0105704687bdbcbfef54546e488a029c3d0655 -- .`；审查时尚无本特性提交。
- 修复复查使用同一起点与完整工作树的 `git diff cf0105704687bdbcbfef54546e488a029c3d0655 -- <affected paths>`，包含暂存及未暂存的修复。
- 两名独立、只读审查者分别执行 Standards 与 Spec；修复后各自复查原发现，不合并或跨轴重新排序。

## Standards

初审：1 项文档规范发现，0 项代码异味发现。

### 保存中禁用状态缺少回归 — 已关闭

- 规范：`web/AGENTS.md` 要求覆盖本次变化的 loading / disabled 状态，异步用例等待明确状态或 Promise，不依赖任意延迟。
- 发现：全局设置改用整次表单的 `isSubmitting`，原测试立即完成请求，不能保护两次顺序保存之间的禁用状态。
- 修复：在既有新增交互测试文件内补充成功、失败两种延迟请求用例。保存时长和开启保护分别等待可控 Promise；验证两段期间按钮一直显示 Saving 且禁用、不提前报成功，以及最终按钮和反馈恢复。
- 证据：`bun run typecheck` 与定向交互测试通过，13 个测试；`/tmp/streaming-ui-reviewed.log`。
- 复查：原 Standards 审查者确认发现已关闭，无该项遗留问题。

没有其他有意义的代码异味发现；协议分类接缝、现有锁/校验接口、共享 UI 和单一新增后端测试文件符合本仓库约束。

## Spec

初审：2 项 P2 契约发现，未发现范围蔓延。

### 已提交的原生 Responses 缺少协议内失败 — 已关闭

- 契约：spec 的“已提交的流”要求不追加普通 JSON、不伪造完成，尽力复用已有协议内错误终止能力。
- 发现：原生 Responses 取得 scanner 超时错误后直接返回；若本地 ping 已提交响应，客户端只能看到突然结束，尽管已有 Responses SSE writer 可用。
- 修复：仅已提交时，使用既有 `ResponsesStreamResponse` 和 writer 尽力发送 `error` 事件，保留稳定错误码，并通过 `ToOpenAIError()` 脱敏消息；仍返回原错误，跳过正常 finalizer。
- 证据：新增原生 Responses 已提交情景先红后绿。原失败日志 `/tmp/streaming-native-responses-red.log` 仅含 ping；完整适配器表通过，记录 `/tmp/streaming-native-responses-green.log`。

### 陈旧同步覆盖刚保存的策略 — 已关闭

- 契约：保存后新请求读取已生效的新配置，已建立的请求继续使用原快照。
- 发现：同步先读旧值，期间管理员成功保存新值，随后旧同步再次发布；原配置锁只保护单次读写，无法保证这段生命周期的顺序。
- 修复：保存通过模块互斥锁串行执行数据库事务、运行时发布和修订号推进。同步在读库前获取修订号，只在发布时修订号未变化才应用；保持既有 request-policy 锁在模块锁之前的顺序。
- 证据：受控数据库查询握手先复现成功保存 1800 秒后又退回 1200 秒，记录 `/tmp/streaming-stale-reload-red.log`；修复后真实 SQLite 3.50.4、MySQL 8.0.46、PostgreSQL 15.19 同一回归全部通过，无跳过，记录 `/tmp/streaming-options-reviewed-matrix.log`。

复查：原 Spec 审查者确认两项 P2 均关闭，针对性复查没有新发现。

## Summary

Standards：初审 1 项规范发现、0 项异味，最终 0 项未关闭；Spec：初审 2 项 P2，最终 0 项未关闭。最终验收命令与完整测试限制见 [verification.md](verification.md)。
