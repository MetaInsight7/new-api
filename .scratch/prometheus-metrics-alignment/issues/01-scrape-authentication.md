# 01: 抓取鉴权与失败限流

Type: task

**What to build:** 既有指标入口按二开校验专用 Token，提供明确的配置错误响应和仅针对错误凭据的限流，合法监控不受失败配额影响。

**Blocked by:** None (can start immediately).

**Status:** resolved

- [x] loopback、正确／缺失／错误／畸形凭据及伪造转发头的访问边界正确，配置缺失返回 503。
- [x] 失败配额、429 及 Redis／内存／未初始化客户端行为正确；合法抓取不占配额。
- [x] 复用既有高层测试入口，凭据不泄露，不新增 UI 或部署。

## Comments

- 2026-10-15：用户在四票拆分提案后请求 `$implement`，按该拆分发布并开始实施。
- 2026-10-15：先观察 429／503 回归失败，再实现失败限流与响应区分；定向 middleware 测试通过，完整验证见后续最终记录。

## Answer

- `middleware/prometheus_metrics_auth.go` 区分 503 配置错误与 401 凭据错误，仅对后者调用独立失败限流器。
- `middleware/rate-limit.go` 提供默认 60 次／60 秒的专用限流，支持 Redis、内存及客户端未初始化兜底。保留恒定时间比较和直连 loopback 边界。
- `middleware/rate_limit_test.go` 验证失败配额、非正值默认回落、到期恢复、Redis 异常不放行、合法 Token 不消耗失败配额以及凭据不泄露；定向及 race 均通过。
