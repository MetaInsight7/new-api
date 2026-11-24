# 06: Codex 凭据与最终请求构建

**What to build:** Codex 和其他需要签名或插件加工的请求在多节点与取消场景下只消费一次刷新凭据，并始终以最终 URL、Header 和客户端身份约束完成签名后发送。

**Blocked by:** 02: 渠道生命周期与有效渠道策略.

**Status:** resolved

- [x] Codex 刷新保留 expected-key、singleflight、跨节点刷新锁、代理刷新、缓存重置与 CAS 持久化。
- [x] 单个等待者取消不会取消共享刷新或丢失已轮换凭据，持久化使用有界独立 context。
- [x] CAS 失败后读取并使用最新凭据，成功落库后触发渠道同步，usage/reset 复用同一刷新服务。
- [x] HTTP、form 和 WebSocket 均按 plugin header、管理员 override、provider finalizer、request signer 的顺序构造最终请求。
- [x] Codex finalizer 能恢复授权和客户端身份约束，Responses JSON finalizer 与 HTTP finalizer 均保留。
- [x] Spec 前临时实现完成三方回审，并通过刷新并发、取消、签名顺序和 WebSocket Header 定向测试。

## Answer

- Codex 刷新以 `channelID + HMAC(expected key)` 做进程内 singleflight，并以 SQLite 进程锁、MySQL named lock 或 PostgreSQL advisory lock 防止多节点重复消费旋转 refresh token。
- 上游刷新与 CAS 持久化都使用独立、10 秒有界 context；等待者取消只结束自己的等待，不会取消共享刷新或丢弃已轮换凭据。
- CAS 未命中时重新读取并返回最新渠道凭据；成功更新后只推进一次 `SyncTopicChannels`，请求方按 `ResetCaches` 刷新本地缓存。usage、reset、模型发现和管理端刷新都复用统一服务。
- HTTP、form 与 WebSocket 的最终顺序统一为 adaptor header → request-plugin header → 管理员 override → provider finalizer → signer；signer 最后执行，避免后续 URL/header 修改破坏签名。
- Codex finalizer 在签名前恢复 `Authorization`、account/client identity、Beta 与内容协商约束；Responses JSON 的 `ResponsesRequestFinalizer` 继续在 body egress 阶段独立执行。
- 通过：Codex 刷新取消、跨节点单次 token、expected-key/CAS 测试；Codex usage/reset/OAuth 测试；HTTP/form/WS finalizer 与签名顺序、Codex 最终约束和 WebSocket fingerprint snapshot 测试。
