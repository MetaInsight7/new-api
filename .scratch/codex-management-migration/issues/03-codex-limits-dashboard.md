# 03: 管理员 Codex Limits 用量与额度报表

**What to build:** 管理员 Dashboard 展示所有 Codex 渠道的当前额度、真实窗口、重置时间、套餐、附加模型限额和所选区间本地 Token／金额；部分上游查询失败不丢失其他账号或本地统计。

**Blocked by:** None (can start immediately).

**Status:** completed (2026-10-08)

- [x] 管理员路由、渠道只读权限及访问令牌 scope 保持 Base 授权边界。
- [x] 所有 Codex 渠道含禁用渠道，失败行保留；查询成功不冒充“账号可用”。
- [x] 区间统计取日志数据库消费日志，按当前渠道 ID 和闭区间聚合；默认 7 天、最大 90 天、1/7/14/29 天预设。
- [x] 上游额度是当前快照而非历史曲线；真实／未知／缺失窗口和附加限额均正确展示。
- [x] 查询最多 10 并发、60 秒报表上下文，认证刷新后最多一次重试；缓存刷新合并，不暴露凭证。
- [x] API、统计与窗口测试、真实三数据库日志聚合、类型检查、lint 和 Playwright 导航／刷新／失败状态已验证。

## Verification

- 独立提交：`8f20ab7a4`，`feat: add Codex account limits and usage dashboard`。
- 三数据库共用／独立日志库、闭区间、501 渠道分批和消费日志口径验证通过，见 `/tmp/codex-ticket03-three-db.log`。
- 上游部分失败、并发／超时边界和刷新后最新配置重试回归通过，见 `/tmp/codex-limits-additional-plan-green.log`。
- 前端窗口、时间范围和报表测试，以及生产 Playwright 桌面／窄屏、错误／空状态通过。详细结果见 `docs/codex-management-migration-verification.md`。
