# 09: 系统任务与插件路由

**What to build:** 官方插件体系和非 Q6 系统任务在单节点与多节点环境中保持幂等、权限正确且不改变已有持久化任务含义，查询请求不会无谓占用用户并发槽。

**Blocked by:** 02: 渠道生命周期与有效渠道策略; 03: Option 与请求策略一致性.

**Status:** resolved

- [x] 非 Q6 handler 集合包含渠道测试、渠道余额、模型更新、Midjourney、异步任务、模型可用性和系统任务清理。
- [x] 系统任务类型与持久化标识不因枚举重排漂移，历史清理只删除超过保留期的已完成任务。
- [x] 插件安装、更新、凭据刷新和系统同步任务保持幂等并避免多节点重复副作用。
- [x] 插件路由使用官方多候选 RouteGroup；查询不占用户并发槽，真实提交仍经过用户并发限制。
- [x] Q6 边界在本 Ticket 保持冻结，并在 Ticket 11 获批后加入敏感词审计清理 handler。
- [x] 通过系统任务、插件路由、权限和并发定向测试。

## Answer

- system-task handler 注册表同时保留渠道测试、余额查询、模型更新、Midjourney/异步轮询、模型可用性、历史清理与后续获批的敏感词审计清理。
- 持久化 task type 使用稳定标识；清理只处理超过保留期的终态任务，pending/running 与有效锁不被删除。
- 插件安装、更新、凭据刷新和系统同步沿用 active-key/lease/CAS 约束，避免多节点重复副作用。
- 插件动态路由采用多候选 `RouteGroup`；只读查询绕过用户并发槽，真实提交仍通过用户并发中间件，原生凭据 handler 使用正确候选索引。
- 插件保留命名空间覆盖官方页面及 Tokendazi 的 available-models、负载监控、Codex 治理、模型健康、报表、敏感词监控和 Workspace 页面，插件不能劫持控制台直达路由。
- 通过 model/service/controller 的 system-task 测试、middleware task-plugin 测试、`pkg/jsplugin` 测试和 `go test ./router` 全包。
