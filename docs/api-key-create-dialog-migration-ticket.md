# Ticket: 对齐 Overlay 的 API Key 创建 Dialog

## Parent

`#5` Available Models 页面和 API Key 创建闭环迁移

## What to build

将 Base 的 API Key 创建/编辑右侧抽屉适配为 Overlay 的 Dialog 交互，并打通 Available Models 分组到 API Key 创建的深链接闭环。保留 Base 现有表单字段、服务端接口、分组权限、Auto 分组、模型限制和安全校验，只迁移 Overlay 的 Dialog 布局、分组预选、模型预览和 URL 状态管理。

## Acceptance criteria

- [ ] API Key 创建和编辑使用 Dialog；宽屏显示表单与模型预览，窄屏仍能完成创建并查看模型预览。
- [ ] Available Models 的分组创建入口生成 `open=create` 和可选 `group` 参数。
- [ ] API Key 路由解析创建参数，等待模型访问数据后只打开一次 Dialog。
- [ ] 有效且可见的分组自动预选；无效或不可见分组按 Base 默认分组规则回退。
- [ ] 用户手动修改分组后，模型访问数据刷新不会覆盖选择；编辑已有 API Key 时不使用创建深链接默认值。
- [ ] Dialog 预览使用当前分组的模型、价格、协议、供应商、状态和专属倍率数据。
- [ ] 模型访问加载失败提供重试状态；空模型范围显示明确空状态。
- [ ] Dialog 关闭后清理 `open`/`group` 参数，刷新页面不会重复打开。
- [ ] 保留 Base 原有表单校验、API payload、创建/编辑成功处理和列表刷新。
- [ ] 补齐路由、深链接状态、分组预选、Dialog 交互和 Available Models 入口测试，并通过受影响 lint、typecheck、Vitest 和 Playwright 验证。

## Blocked by

- Available Models 用户模型访问聚合接口和分组模型专属倍率已完成；无新的代码阻塞项。

## Status

ready-for-agent
