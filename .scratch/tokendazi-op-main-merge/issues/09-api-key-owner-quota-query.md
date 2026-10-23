# 09: 扩展 API Key 查询额度而不覆盖高级限制

**Spec:** `../../2026-10-23-tokendazi-op-main-merge-spec.md`


**Type:** task
**What to build:** 让用户可按 Key 明确授权查询所属账户额度，同时完整保留二开 Key 编辑器的 quota、IP、模型、分组和其他高级限制，以及桌面预览与移动端交互。

**Blocked by:** None (can start immediately)

**Status:** resolved

- [x] owner quota query 开关进入现有表单 schema、默认值、编辑回填和提交 payload。
- [x] 新开关位于既有高级设置语义中，不替换 quota、IP、模型或分组限制区域。
- [x] create/edit、unlimited quota、tokens-only、模型预览和高级折叠状态保持原行为。
- [x] 桌面双栏预览、移动端滚动、关闭和保存按钮继续使用二开布局，不退回不兼容的旧容器。
- [x] 默认关闭、开启提交、旧 Key 无字段、编辑回填和其他限制不丢失的测试通过。
- [x] 如上游表单结构会删除二开限制或改变默认授权，停止并标记 `needs-info`，不得通过采用整段上游 UI 解决。

## Answer

- `allow_user_quota_query` 已接入 schema、默认值、编辑回填、缓存和提交 payload，并嵌入二开高级设置区域，未替换 quota、IP、模型、分组或双栏布局。
- 新 Key 默认关闭并要求显式开启，旧 Key 保留存量值；前后端定向测试通过。
