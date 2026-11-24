# 11: Q6 决策与 Spec 更新

**What to build:** 维护者通过单独的 grilling 明确敏感词体系、官方插件共存、迁移、计费、性能口径、导航与生成文件的最终合同，并把批准结果写回 Spec，供后续重新拆分可执行 tickets。

**Blocked by:** 10: 非 Q6 集成检查点.

**Status:** resolved

- [x] 完成 Q6 专项 grilling，逐项决定权威敏感词体系、迁移入口、日志与计费边界、性能统计和前端信息架构。
- [x] 明确官方 request-plugin/request-filter 与 Tokendazi 敏感词预检的共存和执行顺序。
- [x] 明确生产数据兼容、三数据库迁移、回滚和多实例启动契约。
- [x] 更新 Spec 的 Q6 不变量、冻结边界、验证矩阵和完成定义。
- [x] 基于批准后的 Spec 完成 tickets、Q6 实现，并进入全量验证、双轴 review 与本地 merge commit 阶段。

## Answer

- 用户确认 Tokendazi 敏感词预检继续作为观察、阻断、白名单和独立审计的唯一权威；官方 request-plugin/request-filter 作为独立通用处理链共存，不替代或重复执行敏感词匹配。
- 预检结果通过请求 context 复用；观察/白名单合并到唯一消费日志，阻断保留唯一类型 8 日志，失败回退也不会重复计数或重复审计。
- `MigrateSensitiveWordData()` 保留旧 option 原文与已有 `RequestPluginInstances`；SQLite 使用 `BEGIN IMMEDIATE`，MySQL 使用连接级 `GET_LOCK`，PostgreSQL 使用连接级 advisory lock，迁移查询另开 GORM session，确保租约覆盖完成标记提交且不会被连接池换连接；官方删除式迁移不执行。
- request-processing 插件页与 `/system-settings/request-policies/filtering` 敏感词策略页同时保留；旧 security URL 只重定向到敏感词策略页，独立监控面板及其 sidebar 权限继续存在。
- SQLite 3.50.4、MySQL 8.0.46、PostgreSQL 16.15 的 fresh、dashboard query、RC40 upgrade 与 concurrent lease 四类敏感词迁移场景均在本机一次性测试库通过；单日志、页面共存和七语言生成继续纳入最终门禁，未连接生产库。
