# 06: Codex 最终差异校正与整套集成验收

**What to build:** 核对来源完整 Codex 功能清单，把本轮配套与上一批八项 Relay 能力组合验证，确认没有只迁后端／只迁页面或遗漏最终版本行为。

**Blocked by:** 01: OAuth；02: 永久失效渠道状态；03: Limits；04: Model Governance；05: 账号用量与重置。

**Status:** completed (2026-10-08)

- [x] 对源码最终差异做行为矩阵，区分缺失、已具备和保留 Base 较新实现；不以单个提交代替完整功能。
- [x] 任何审计确认的遗漏 Codex 行为单独实现／回归并以功能名提交，不把无关改造混入。
- [x] 已有 Chat／Responses／Compact／Search／图片／R2／Identity／指纹／终止失败／真实计费回归。
- [x] 新增管理闭环、三数据库结果、前端相关测试、类型检查／lint、受限生产构建和 Playwright 验收均记录真实结果。
- [x] 服务在 3000/5173 使用最新代码启动，未验证的真实账号／R2／官方授权事项明确列出，不假称线上端到端成功。
- [x] 全部已完成功能独立提交；spec、ticket 和验证记录状态与实际工作一致。

## Verification

- 九个业务独立提交已完成；最后权限遗漏独立提交为 `6e12a1287`，`fix: authorize Codex identity updates with option write scope`。本项仅验收归档，不伪造额外业务功能提交。
- 最新代码的 14 个受影响 Go 包完整测试、24 个变更测试文件的 105 个测试入口在八个包的 race 检查通过，`relaykit` 独立构建／转换测试通过。
- 真实三数据库迁移／业务、Redis 通知、39 文件／401 前端测试、类型检查、42 文件 lint、49 文件格式检查及受限生产构建通过。
- 端口 3000 最新生产产物 Playwright 九组通过，包含真实状态筛选及刷新保留；端口 5173 健康。Standards／Spec 最终复审均无阻塞。
- 扩大范围的全包 race 并未全绿；官方基线复现和未稳定复现项如实保留，没有修无关 Base 路径。详情与真实上游／Reset／R2 待拥有者冒烟的边界见 `docs/codex-management-migration-verification.md`。
