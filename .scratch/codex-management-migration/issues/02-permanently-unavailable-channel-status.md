# 02: 通用永久失效渠道状态与人工管理

**What to build:** 管理员可在所有渠道类型的行菜单标记已封禁、筛选、查看状态与解除封禁；后台自动任务不能创建或覆盖这一人工决定。

**Blocked by:** None (can start immediately).

**Status:** ready-for-agent

- [ ] 新增状态 4，保持状态 1/2/3 和官方自动禁用／恢复规则不变。
- [ ] 使用 Base 专用渠道操作接口与授权；查询筛选、徽标、行菜单与普通开关一致。
- [ ] 自动状态处理和多 Key 自动处理不能创建、解除或覆盖 4；显式人工管理保持来源语义。
- [ ] 路由能力、缓存和跨节点状态同步；保留 Base WebSocket 停止处理，不新增账号连带封禁。
- [ ] SQLite/MySQL/PostgreSQL 状态流转、能力／缓存回归和 Playwright 行操作／筛选已验证。

## Verification

待实现。
