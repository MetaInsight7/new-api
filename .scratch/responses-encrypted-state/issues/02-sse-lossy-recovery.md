# 02: 覆盖 SSE 流内加密状态拒绝

**What to build:** SSE Responses 在客户端尚未收到业务数据时，把流内加密状态拒绝交给同一有损续接流程，使客户端只看到恢复后的单一响应；已经输出后不进行危险重放。

**Blocked by:** 01/打通 HTTP Responses 有损续接.

**Status:** completed (2026-10-12)

- [x] `response.failed`、`response.error` 和 `error` 中的结构化代码或窄消息能够触发共享恢复规则。
- [x] 被恢复的失败事件和原尝试响应头不会提交给客户端。
- [x] 当前渠道额外重试成功时只产生恢复后的 SSE 序列。
- [x] 已经发送业务事件或存在权威用量时保持既有不可重试边界。
- [x] 恢复后仍失败只进入一次普通策略，不产生清理循环。
- [x] Codex 与普通 OpenAI-compatible Responses 流均有行为覆盖。
