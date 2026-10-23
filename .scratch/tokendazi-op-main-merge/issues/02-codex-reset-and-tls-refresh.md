# 02: 实现幂等 Codex 用量重置与安全刷新

**Spec:** `../../2026-10-23-tokendazi-op-main-merge-spec.md`


**Type:** task
**What to build:** 让管理员查询 Codex usage/reset credits 和执行 reset 时，既得到上游的新代理/TLS 与国际化行为，又保留二开的幂等 request ID、凭据并发保护、rotation 持久化和不确定结果防重复提交能力。

**Blocked by:** 01: 保留分组路由与事务化渠道生命周期

**Status:** resolved

- [x] 同一次 reset 操作在首次请求和凭据刷新重试之间复用同一个 request ID。
- [x] 401/403 只通过集中式凭据刷新执行一次重试，并以 expected credential 防止并发覆盖已轮换的凭据。
- [x] 刷新 token 的 rotation 可靠持久化；刷新后的请求使用刷新后渠道的 base URL、proxy 和完整 TLS settings。
- [x] 渠道跳过 TLS 验证只作用于目标上游；release/version 查询仍验证 TLS，全局身份强制模式不查询 release。
- [x] reset 必须确认实际重置窗口；网络或响应不确定时提示先刷新 usage/credits，不能鼓励立即重复 reset。
- [x] 用户消息使用英文源键/API 翻译，服务端日志保留根因并经过日志本地化入口。
- [x] 查询、成功 reset、401 refresh、rotation 竞争和不确定结果测试通过。
- [x] 如上游刷新流程与二开持久化、身份或幂等约束冲突，立即标记 `needs-info` 并上报，不得退回手工无条件更新凭据。

## Answer

- Reset 全程复用固定 request ID，401/403 仅经集中刷新重试；expected credential、rotation 持久化、代理与完整 TLS 设置均保留。
- 不确定 reset 结果不会鼓励重复提交；Codex usage/reset、刷新竞争及目标上游 TLS 定向测试通过。
