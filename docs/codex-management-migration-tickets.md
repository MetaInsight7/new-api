# Codex 配套功能迁移 Tickets

> 日期：2026-10-07。
> 规格：`docs/codex-management-migration-spec.md`。
> 已授权按以下完整功能切片实施；当前分支 `tokendazi_op`，来源固定为 `762ce21c4`。
> 每项均覆盖实际使用闭环，不能只交后端或只交页面。当前完成状态以各 ticket 的验证记录为准。
> 完成日期：2026-10-08。九项均已完成，业务独立提交与相关验证详见下表和各 ticket；额外全包竞态检查的官方已有问题另行记录，不声称整库全绿。

| Ticket | 交付功能 | Blocked by | 完成提交 |
|---|---|---|---|
| 01 | 安全 Codex OAuth 授权与渠道凭证管理 | 无 | `11147c84b` |
| 02 | 通用永久失效渠道状态与人工管理 | 无 | `99ac1f19e` |
| 03 | 管理员 Codex Limits 用量与额度报表 | 无 | `8f20ab7a4` |
| 04 | 完整 Codex Model Governance 探测与审核闭环 | 无 | `c75525083` |
| 05 | 单账号真实窗口展示与安全额度重置 | 03 的共用窗口契约 | `d68fe1a49` |
| 07 | Codex 请求字段约束、显式 Fast 与 full 指纹校验 | 无 | `a664c390d` |
| 08 | Codex Chat 流式失败与截断保护 | 07 | `96aa88a39` |
| 09 | 模型发现边界与图片真实格式 | 无 | `149da32d8` |
| 06 | 最终 Relay 差异校正与整套集成验收 | 01–05 | 最后权限校正 `6e12a1287`；整套验收以文档提交归档，不伪造业务提交 |

独立 tickets 保存在 `.scratch/codex-management-migration/issues/`，从无阻塞的 frontier 开始；并行工作必须使用不重叠的写入范围，所有 commit 由主线程审核和验证后执行。前端构建串行且限制资源，不同时开多个 Bun 构建。

最终验证记录保存在 `docs/codex-management-migration-verification.md`，记录真实执行的命令、数据库版本、Playwright 场景和剩余限制；不得提前写“全部通过”。
