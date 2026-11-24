# 02: 渠道生命周期与有效渠道策略

**What to build:** 管理员可以安全地编辑、测试和治理渠道，真实请求继续按有效渠道策略、渠道并发和冷却状态调度，同时官方插件凭据与跨节点同步能力不会破坏永久禁用、密钥保护或现有渠道数据。

**Blocked by:** None (can start immediately).

**Status:** resolved

- [x] 渠道状态、Ability、Codex governance 与 fingerprint seed 在同一受保护事务中原子提交；任一步失败均回滚且不发布通知。
- [x] 每次真实渠道状态变化在提交后恰好触发一次本地缓存刷新、Redis PubSub 和官方渠道 sync topic；失败与 no-op 均不触发。
- [x] 常规编辑使用明确字段集合，不以陈旧快照覆盖余额详情、Key 状态、禁用原因、轮询游标或其他服务端维护信息。
- [x] 密钥更新与同一编辑请求中的普通字段保持原子性，凭据撤销只在整体提交成功后执行。
- [x] 永久禁用、分组 priority/weight、多 Key、并发等待与冷却、force-max-concurrency、插件凭据锁与 CAS 契约全部保留。
- [x] 渠道管理界面按 ChannelWrite 与 ChannelOperate 权限隐藏或禁用操作，不发送已知无权限请求。
- [x] Spec 前临时解决的渠道冲突完成三方回审，并通过可执行的数据库、路由、权限、格式和静态检查。

## Answer

- 渠道状态、普通编辑、插件凭据与 Ability/governance 更新统一进入受保护事务；提交后才执行一次缓存刷新、渠道 sync topic 与 Redis PubSub，失败和 no-op 不产生通知。
- 普通编辑改为明确字段更新，保留余额、服务端状态、`other_info`、轮询游标和多 Key 禁用信息；Key 与普通字段原子提交，撤销只发生在提交成功后。
- Vertex 追加在省略 `settings` 时复用存量设置；插件默认 BaseURL 在校验填默认值前保留来源审计语义。
- 渠道 UI 按 `ChannelWrite`、`ChannelOperate`、`SensitiveWrite` 分层约束，并在 handler/hook 中二次拒绝已知无权限请求。
- 通过：`go test ./model` 的渠道状态、事务回滚、通知次数和字段保留定向集合；`go test ./controller` 的渠道创建、编辑、并发与插件凭据定向集合；相关 Vitest 共 53 个用例；涉及前端文件 `oxfmt --check` 与 `oxlint`。
- 最终 `go test ./model -count=1` 在 SQLite 3.50.4、MySQL 8.0.46 与 PostgreSQL 16.15 全包通过；provider 渠道管理三库矩阵连续 10 轮通过，原 SQLite `SQLITE_BUSY` 由数据库入口统一补齐 WAL、busy timeout 与 immediate transaction 参数后消除。
- Spec review 补充发现单渠道 `Insert`/`Delete` 仍把 Channel 与 Ability 分成两个事务并在失败后发布同步事件；现已统一为单事务，只有整体提交成功且实际删除 Channel 或 Ability 时才 bump sync topic。重复单删与全不存在批删继续幂等成功且不发布虚假通知，并新增 SQLite/MySQL/PostgreSQL 的失败回滚与 no-op revision 回归。
