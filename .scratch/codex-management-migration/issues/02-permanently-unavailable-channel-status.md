# 02: 通用永久失效渠道状态与人工管理

**What to build:** 管理员可在所有渠道类型的行菜单标记已封禁、筛选、查看状态与解除封禁；后台自动任务不能创建或覆盖这一人工决定。

**Blocked by:** None (can start immediately).

**Status:** completed (2026-10-08)

- [x] 新增状态 4，保持状态 1/2/3 和官方自动禁用／恢复规则不变。
- [x] 使用 Base 专用渠道操作接口与授权；查询筛选、徽标、行菜单与普通开关一致。
- [x] 自动状态处理和多 Key 自动处理不能创建、解除或覆盖 4；显式人工管理保持来源语义。
- [x] 路由能力、缓存和跨节点状态同步；保留 Base WebSocket 停止处理，不新增账号连带封禁。
- [x] SQLite/MySQL/PostgreSQL 状态流转、能力／缓存回归和 Playwright 行操作／筛选已验证。

## Verification

- 独立提交：`99ac1f19e`，`feat: add operator-managed permanently unavailable channel status`。
- 三数据库状态／能力回归，以及真实 Redis 双客户端通知、状态和标签 producer 回归通过，见 `/tmp/codex-channel-status-{producer,tag-producer}-realredis-verification.log`。
- 前端行操作、权限、失败反馈和已封禁筛选测试通过；生产 Playwright 经真实 API 验证非 Codex 渠道 `4 -> 1`。
- 状态 4 为通用人工状态，自动任务不会生成或覆盖；显式人工批量管理仍保留来源语义。详细结果见 `docs/codex-management-migration-verification.md`。
