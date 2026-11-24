# 03: Option 与请求策略一致性

**What to build:** 多节点管理员更新 Option 或请求策略时，各节点获得一致、完整且兼容旧数据的运行时快照，数据库失败或陈旧加载不会覆盖刚写入的配置。

**Blocked by:** None (can start immediately).

**Status:** resolved

- [x] Option 全量加载只执行一次数据库读取，并按既定锁顺序原子应用运行时配置。
- [x] 数据库读取或写入失败时不修改当前快照，也不发送虚假同步通知。
- [x] 官方跨节点 Option sync 与 Tokendazi streaming revision 同时保留，批量更新只各推进一次。
- [x] 请求策略使用官方 MoeForge regexp，同时保留加密推理恢复开关、默认值、校验和旧数据可读性。
- [x] 对双方 Option key、默认值和同步行为做双向清单审计，确认没有静默丢键。
- [x] 通过可执行的 Option、请求策略与并发更新定向测试。

## Answer

- 增加统一 Option 运行时 mutation revision；全量加载只读库一次，随后按 `request policy → passkey → streaming` 的固定顺序持锁，陈旧读取在应用前整体丢弃。
- 单项、批量、请求策略、Passkey 与模型定价写入均在数据库提交且运行时应用成功后才推进运行时 revision 和 `SyncTopicOptions`；读写失败不改变当前 snapshot，也不产生同步任务。
- 保留 Tokendazi `streamingOptionRevision`，批量流式更新恰好推进一次 streaming revision、一次运行时 revision 和一次跨节点 sync topic。
- 请求策略继续使用 MoeForge regexp，保留 `responses_encrypted_reasoning_setting.recovery_enabled`、默认值、校验以及规则 JSON 未知扩展字段的原文可读性。
- 双向审计双方 `OptionMap` 默认键与传统 switch 分支：当前合并结果包含双方并集；Tokendazi 独有的 Codex identity、`GroupModelRatio`、`StreamCacheQueueLength` 与用户并发默认项均保留，无上游键缺失。
- 通过：请求策略、Option、system task 与敏感词迁移矩阵在 SQLite 3.50.4、MySQL 8.0.46、PostgreSQL 16.15 实际执行；`go test ./model -count=1` 三库整包通过，并覆盖单次读取、读写失败、陈旧加载、批量单次同步、streaming revision 与渠道并发 Option 回归。
