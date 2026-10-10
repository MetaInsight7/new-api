# 在 Workspace 中承载接入面板

状态：accepted。TokenDazi 的 API、SDK、Codex & Claude Code 接入入口归属普通用户的 Workspace，而不是管理员 Dashboard Overview；因此只替换 Workspace 原 Uptime Kuma 位置的面板，Overview 保持原有结构。接入面板使用右侧紧凑外层卡片和三行入口，入口弹出接入说明，创建 API Key 则统一跳转到现有 `/keys?open=create` 深链接，不修改通用 API Key 后端、管理页或数据库契约。

## 考虑过的方案

- 在 Dashboard Overview 中承载接入面板：拒绝，因为普通用户默认进入 Workspace，且会混淆管理侧与用户侧职责。
- 在 Workspace 下方新增全宽三列卡片：拒绝，因为当前 Workspace 的 Uptime 区域是右侧窄栏，已确认需要在该位置保持紧凑纵向布局。
- 在接入面板内复制 API Key 创建表单：拒绝，因为会重复通用表单和后端行为，增加维护分叉。
