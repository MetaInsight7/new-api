# Codex 配套功能迁移 Tickets

> 日期：2026-10-07。
> 规格：`docs/codex-management-migration-spec.md`。
> 已授权按以下完整功能切片实施；当前分支 `tokendazi_op`，来源固定为 `762ce21c4`。
> 每项均覆盖实际使用闭环，不能只交后端或只交页面。当前完成状态以各 ticket 的验证记录为准。

| Ticket | 交付功能 | Blocked by | 独立提交说明 |
|---|---|---|---|
| 01 | 安全 Codex OAuth 授权与渠道凭证管理 | 无 | `feat: add secure Codex OAuth authorization for channels` |
| 02 | 通用永久失效渠道状态与人工管理 | 无 | `feat: add operator-managed permanently unavailable channel status` |
| 03 | 管理员 Codex Limits 用量与额度报表 | 无 | `feat: add Codex account limits and usage dashboard` |
| 04 | 完整 Codex Model Governance 探测与审核闭环 | 无 | `feat: add Codex model lifecycle governance` |
| 05 | 单账号真实窗口展示与安全额度重置 | 03 的共用窗口契约 | `fix: align Codex account windows and reset credit handling` |
| 06 | 最终 Relay 差异校正与整套集成验收 | 01–05 | 根据实际遗漏功能命名；无业务缺口时仅记录验证，不伪造功能提交 |

独立 tickets 保存在 `.scratch/codex-management-migration/issues/`，从无阻塞的 frontier 开始；并行工作必须使用不重叠的写入范围，所有 commit 由主线程审核和验证后执行。前端构建串行且限制资源，不同时开多个 Bun 构建。

最终验证记录保存在 `docs/codex-management-migration-verification.md`，记录真实执行的命令、数据库版本、Playwright 场景和剩余限制；不得提前写“全部通过”。
