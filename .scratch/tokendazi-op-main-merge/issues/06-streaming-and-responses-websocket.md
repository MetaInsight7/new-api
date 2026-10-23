# 06: 组合首事件超时、慢客户端 SSE 与 Responses WebSocket

**Spec:** `../../2026-10-23-tokendazi-op-main-merge-spec.md`


**Type:** task
**What to build:** 让 HTTP SSE 与 Responses WebSocket 在首事件等待、慢客户端、大事件、取消、上游断开和渠道切换时都可靠终止并保留 usage，同时不回退二开的请求策略快照、渠道快照和并发 lease 生命周期。

**Blocked by:** 03: 统一 Relay 错误协议与并发饱和语义；04: 结算 Responses 终止 Usage 并保持失败指标保真；05: 保留 Claude Cache、Tool 与 Search 精确计费

**Status:** resolved

- [x] 首事件 timeout 只由 classifier 认可的首个有效业务 data 解除；注释、空行和心跳不算进展。
- [x] scanner 读取与下游 handler 解耦，慢客户端和大型事件不会截断；handler 阻塞时 timeout 仍及时关闭 upstream body。
- [x] 客户端取消前已排队的权威 usage 仍被处理；完整结束后的取消归一化为正常完成。
- [x] idle timeout、ping、scanner error、panic 和 first-event timeout 的错误优先级及资源回收保持确定性。
- [x] WebSocket 使用单一渠道绑定入口保存 model、channel、key、route 和 context 快照，并忽略已替换 upstream reader 的迟到事件。
- [x] panic、upstream close、client close 和正常完成发送正确 close code/reason/frame；并发 lease 与性能样本各处理一次。
- [x] 首事件、慢 handler、大事件、queued usage、cancel-after-complete、换渠道和 close frame 测试通过且无 goroutine 泄漏。
- [x] 如上游背压/关闭模型与二开首事件预算或渠道快照不能组合，停止并标记 `needs-info`，附最小复现与两种方案影响。

## Answer

- Scanner 与 handler 解耦，首有效事件分类、慢客户端写预算、大事件、排队 usage、取消后完成归一和资源回收已组合。
- Responses WebSocket 保留单一渠道绑定与快照，加入 close frame、旧 reader 过滤和上游 origin；HTTP/WS 定向测试通过。
