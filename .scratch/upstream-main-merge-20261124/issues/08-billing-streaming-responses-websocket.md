# 08: 计费、Streaming 与 Responses WebSocket

**What to build:** 普通、流式和 Responses WebSocket 请求在空输出、心跳、取消、重试和正常终止场景下形成唯一且准确的输出、计费与结束状态，不因官方合并恢复空回复扣费。

**Blocked by:** 06: Codex 凭据与最终请求构建; 07: Relay 失败、重试与并发冷却.

**Status:** resolved

- [x] 预扣、结算、退款、累计、消费日志和性能样本均最多执行一次。
- [x] 非流式 OpenAI 兼容空输出继续归零计费，显式 empty-response 错误继续作为失败并退款。
- [x] 首响应超时只由首个非心跳业务事件解除，只有精确的 DONE 帧被视为终止。
- [x] timeout、cancel、panic 和正常完成产生唯一且准确的 stream end reason。
- [x] retry 重置 attempt-scoped token、空回复和插件状态，同时保留 request-scoped 诊断和计费累积。
- [x] Responses WebSocket 保留官方 request-plugin 路径和 Tokendazi provider 契约；Q6 混合文件按后续批准合同完成整合。
- [x] 通过计费、scanner、streaming 与 WebSocket 定向测试。

## Answer

- 统一 `BillingSession`、终态 settlement 与错误日志认领继续保证预扣、结算、退款、消费日志和性能样本不会因重试或多条错误路径重复执行。
- 非流式 Chat Completions/Responses 仅在“本地 usage + 协议无 choices/output”时归零，显式 `empty_response` 仍走失败退款；有效但无 token usage 的按 token 图片响应继续不伪造 usage，固定价图片通过真实固定价夹具验证插件与 Advanced Custom 计费一致。
- stream scanner 只把精确 `[DONE]` 当终止帧，心跳不解除首响应超时；timeout、cancel、client-gone、handler-stop、EOF 与正常完成写入唯一 end reason。
- 每次 attempt 重置本地 token、空回复和 request-plugin header/record，保留请求级诊断、计费累积和 stream 状态。
- Responses WebSocket 保留 request-plugin、provider finalizer、敏感词预检、并发租约、取消和单次结算合同。
- 通过 `go test ./relay` 图片计费测试、`go test ./router` 全包以及 `go test ./controller` 全包；scanner、空回复、stream outcome 与 Responses WebSocket 回归均包含在对应包测试中。
