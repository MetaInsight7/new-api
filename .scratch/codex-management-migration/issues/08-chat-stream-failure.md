# 08: Codex Chat 流式失败与截断保护

**What to build:** 首个有效输出前可按现有重试语义处理失败；已有输出后发送协议正确的失败而非成功，不重试、不追加普通 JSON，计费仅使用可信 usage。

**Blocked by:** 07: Codex 请求字段约束。

**Status:** ready-for-agent

- [ ] prelude 不提前提交；显式失败与缺 terminal 的流拒绝伪成功。
- [ ] 失败 usage 保留可信事实，无 authoritative usage 不猜计费。
- [ ] 已输出后避免重试和普通 JSON；保留 Base 其他 provider 行为。
- [ ] Chat/Responses/Compact 相关回归通过，独立功能提交。

## Verification

待实现。
