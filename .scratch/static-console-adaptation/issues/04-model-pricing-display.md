# 04: 精简官网模型主表和计费明细

**Spec:** [官网模型报价展示精简](../models-display-spec.md)

**Status:** complete

- [x] 计费分类统一为按量/按次，筛选与标签一致。
- [x] 移除主摘要的档名、条件和首档文案，不渲染原始表达式。
- [x] 仅多档或分时规则触发计费明细，缓存 TTL 不触发；保留独立分组比价。
- [x] 桌面对照表、移动逐档列表和可读条件展示，当前时段正确标识。
- [x] 来源报价和无法报价状态不丢失，计算口径不变。
- [x] Node 与公网桌面/移动 Playwright 验证通过，按功能独立提交。

**Implementation:** Static `master` 提交 `b4e8d47` — `feat: simplify website model pricing and clarify tiered and timed prices`。

**Verification:** [展示验收记录](../models-display-verification.md)，38 项 Node 测试及公网桌面/移动 Playwright 均通过。
