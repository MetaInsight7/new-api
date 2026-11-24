# 07: Relay 失败、重试与并发冷却

**What to build:** 请求最终失败时用户和管理员只看到一次准确记录，平台只对真实上游容量故障执行冷却或重试，本地策略拒绝不会污染渠道健康状态或重复处理计费。

**Blocked by:** 02: 渠道生命周期与有效渠道策略.

**Status:** resolved

- [x] 已向客户端提交真实响应后不再切换普通 HTTP 或 SSE 渠道，既有 affinity、固定渠道和 SkipRetry 规则继续生效。
- [x] 每次最终失败只写一条错误日志，并保留官方 request-plugin 审计记录。
- [x] request-plugin 与其他 local 错误不触发渠道冷却或自动禁用，即使文本包含容量或限速词。
- [x] 真实上游 429 或容量并发错误继续进入 Tokendazi 冷却，配额耗尽不被误判为瞬时并发故障。
- [x] ingress、attempt 与 egress 的预扣和退款边界保持单次副作用，无可用渠道与并发等待失败不扩大日志噪音。
- [x] 通过可执行的重试、单日志、插件审计、冷却分类和计费幂等定向测试。

## Answer

- `DecideRelayRetry` 在普通 HTTP/SSE 已写出真实响应后立即返回 `response_already_written`，同时保留 WebSocket 独立状态机、affinity、固定渠道、channel error 与 `SkipRetry` 优先级。
- `ProcessChannelError` 只处理服务端诊断、冷却与自动禁用；持久化错误日志改由请求生命周期在最终失败时写入。`RecordRelayErrorLog` 另以请求策略状态原子认领，防止多个路径重复写行。
- 普通 Relay 与异步任务只在所有重试最终失败后记录一次；成功重试不留下失败日志，ingress/attempt 插件拒绝仍携带 `request_plugins` 公共与管理员审计。
- distributor、pre-relay 与最终 attempt 失败统一进入终局记录器；每个失败请求只增加一次持久化错误日志和一次 Prometheus failure，DEBUG 诊断也不进入客户端或公共日志。
- `RelayErrorSource` 将 request-plugin、转换与本地策略错误分类为 `local`，因此即使文本含 `capacity`、`rate limit` 或禁用关键词也不会冷却、自动禁用；真实 upstream 429/容量错误继续触发配置的冷却。
- 配额、余额和额度耗尽关键词在 429/message-match 前排除，避免把持久配额不足误判为瞬时并发压力。
- 通过：service 重试/错误来源/禁用/冷却/单日志测试，controller Relay/任务插件测试，middleware 分发与敏感词组合测试，request-plugin 全包，以及 relaykit 全包。
