# 模型专属倍率可视化配置与可用模型闭环

**Status:** implemented

**What to build:** 对齐二开最新版模型专属倍率的可视化编辑，在 Base 已有分组设置和保存流程中配置 `GroupModelRatio`，并验证 Available Models 展示对应覆盖倍率且保留现有基础价格展示。

**Spec:** `.scratch/group-model-ratio-visual-editor/spec.md`

- [x] 完成来源最终代码、现有表单、倍率后端和目录展示链路核对。
- [x] 支持按分组添加、重命名、编辑及删除模型倍率覆盖，可视化与 JSON 共用字段。
- [x] 防止空值变为免费、负数／非有限值保存、重名覆盖及非法配置被清空。
- [x] 保存成功后刷新模型访问缓存；无覆盖时继续继承且不显示专属倍率。
- [x] 完成受影响行为测试、后端回归、类型、lint、格式与受限生产构建。
- [x] Playwright 真实保存、目录展示、零倍率、删除、持久化及窄屏验收，恢复原配置。
- [x] 使用实际功能名称独立 commit，不 push，不声称已发布 GitHub Issue。

**Verification:** `.scratch/group-model-ratio-visual-editor/verification.md`
