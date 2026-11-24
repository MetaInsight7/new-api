# 01: 认证缓存与 API Key 安全契约

**What to build:** 在完整吸收官方认证缓存实现的同时，保持 Tokendazi 的用户并发版本栅栏和 API Key 安全默认值，使缓存篡改、陈旧并发状态以及隐式开放额度查询均不能绕过认证约束。

**Blocked by:** None (can start immediately).

**Status:** resolved

- [x] 认证缓存保留官方 MAC 校验，且完整覆盖最大并发和并发版本。
- [x] Redis Cluster 中认证缓存、并发 fence 和 version 使用同一 hash slot，并有回归证据。
- [x] 用户并发版本栅栏继续拒绝陈旧状态，缓存失效与版本推进保持原有契约。
- [x] 普通创建和注册自动创建的 API Key 均默认关闭额度查询，只有用户显式开启时才允许查询。
- [x] Spec 前临时解决的认证冲突完成三方回审，并通过可执行的定向测试与静态检查。

## Answer

- 注册自动生成的默认 API Key 不再隐式开放用户额度查询；普通创建和显式更新行为保持不变。
- 认证缓存完整性测试新增最大并发、并发版本篡改场景，并确认认证、并发 fence/version 与缓存键使用同一 Redis Cluster hash tag。
- 修正 OAuth 邀请人回归测试以匹配合并后的广告归因参数，确保认证测试包可编译。
- 使用只存在于 `/tmp` 的 Q6 测试 overlay 运行定向测试；overlay 未修改或 stage 任何冻结文件。
- 通过：`go test ./model` 的认证缓存、并发版本和额度查询定向集合；`go test ./controller` 的注册、API Key 创建/更新与 OAuth 邀请人定向集合；涉及文件 `gofmt` 与 `git diff --check`。
- 最终数据库门禁实际覆盖 SQLite 3.50.4、MySQL 8.0.46 与 PostgreSQL 16.15；认证缓存矩阵随 `go test ./model -count=1` 三库整包通过，未连接生产数据库。

## OWASP review

- 基线：OWASP 官方项目页在 2026-11-24 标示 ASVS 5.0.0 为最新稳定版；同时复核了 [Authentication Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Authentication_Cheat_Sheet.html) 与 [Session Management Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html)。
- `v5.0.0-7.2.1`、`v5.0.0-7.4.1`：会话缓存只由可信后端校验；MAC 缺失、过期、跨身份复制和已撤销会话均不能恢复为有效会话，数据库真值可修复缓存但不能绕过撤销状态。
- `v5.0.0-8.2.1`、`v5.0.0-8.2.3`、`v5.0.0-8.3.1`：用户额度查询是 API Key 的显式服务端权限；创建和注册默认关闭，只有显式写入才开启，不能以仅前端默认值代替服务端约束。
- `v5.0.0-8.3.2`、`v5.0.0-15.4.1`：用户请求并发策略的版本 fence 阻止旧缓存覆盖新限制；Redis 更新和修复路径通过原子脚本/同步机制避免并发损坏。
- 验证覆盖：`TestAuthCacheIntegrityDatabaseMatrix` 的缓存篡改、过期、跨身份、撤销与修复竞态；用户认证/并发版本缓存回归；注册默认 Key 与 Token 创建/更新权限回归。最终三数据库命令、版本和结果由 Ticket 03/10 汇总。
- 边界：本 Ticket 不声明整个产品通过 ASVS 认证；Ticket 05 已补齐本轮发现的 `v5.0.0-6.2.4` 常见密码筛查缺口。本 Ticket 范围内未发现其他未验证的适用控制。
