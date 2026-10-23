# 07: 渠道控制台同时支持拖拽排序与并发可视化

**Spec:** `../../2026-10-23-tokendazi-op-main-merge-spec.md`


**Type:** task
**What to build:** 让管理员在同一渠道控制台中使用上游优先级拖拽和二开实时并发观察，表格与卡片视图都保留筛选、批量操作、权限、状态原因及 stale 数据保护。

**Blocked by:** 01: 保留分组路由与事务化渠道生命周期

**Status:** resolved

- [x] priority order/reorder API 与并发 snapshot API 同时可用，成功校验、取消和重复请求策略互不覆盖。
- [x] 状态单元格复用单一实现，多 Key 数量、自动禁用原因/时间和服务端文案翻译正确。
- [x] 表格和卡片视图都支持允许范围内的拖拽排序，并同时展示并发状态。
- [x] snapshot stale 或不可用时显示明确提示并隐藏伪造零值；恢复后自动显示新数据。
- [x] 拖拽权限、排序/标签限制、revision 冲突、筛选、刷新、选择和批量操作保持原行为。
- [x] 拖拽、并发 list/status、stale、卡片/表格和状态 tooltip 行为测试通过。
- [x] 如上游全局优先级拖拽会覆盖二开分组 priority/weight，停止并标记 `needs-info`，不得扩大 reorder API 的写入范围。

## Answer

- 渠道 API 同时导出 priority order/reorder 与 concurrency snapshot；表格和卡片共用拖拽句柄、状态单元格与并发数据。
- stale 快照显示提示并隐藏伪零值；priority、并发、状态 tooltip 与快照 hooks 定向测试通过。
