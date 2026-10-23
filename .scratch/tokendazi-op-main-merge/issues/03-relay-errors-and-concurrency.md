# 03: 统一 Relay 错误协议与并发饱和语义

**Spec:** `../../2026-10-23-tokendazi-op-main-merge-spec.md`


**Type:** task
**What to build:** 让 Relay 在普通 JSON、已提交 SSE、Claude 和 Realtime 连接上都返回协议正确且不泄密的错误，同时保留二开的渠道并发饱和、最后失败根因、永久不可用模型和任务错误行为。

**Blocked by:** 01: 保留分组路由与事务化渠道生命周期

**Status:** resolved

- [x] 未提交响应时按目标协议返回 JSON 错误，已提交 SSE 时通过当前流发送终止错误而不是静默返回或写第二个响应。
- [x] Realtime、Claude 和 OpenAI 错误均精准遮蔽当前请求的自定义 upstream 地址，并继续执行二开的其他敏感信息清理。
- [x] 服务端日志保留可诊断根因和 request ID，但不记录凭据或完整敏感响应。
- [x] 渠道并发满载映射为 429 且跳过无意义 retry；候选耗尽时保留最后一次真实失败原因。
- [x] 官方永久不可用模型拒绝、普通无渠道错误和 task 429/非 429 错误均保留正确状态码、文案和重试语义。
- [x] 响应已提交、错误脱敏、并发 cooldown/lifecycle、永久不可用和 task error 回归通过。
- [x] 如上游错误协议要求与二开并发、可用性或脱敏契约不能同时成立，停止并标记 `needs-info`，列出对客户端兼容和安全的影响。

## Answer

- 未提交响应按协议返回 JSON；已提交 SSE 通过当前流发送终止错误，非 SSE 不写第二响应；OpenAI、Claude、Realtime 均按当前 upstream 精准脱敏。
- 渠道并发 429/SkipRetry、最后真实错误、永久不可用模型和 task 错误语义保留；控制器定向回归通过。
