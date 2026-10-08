# 08: Codex Chat 流式失败与截断保护

**What to build:** 首个有效输出前可按现有重试语义处理失败；已有输出后发送协议正确的失败而非成功，不重试、不追加普通 JSON，计费仅使用可信 usage。

**Blocked by:** 07: Codex 请求字段约束。

**Status:** completed (2026-10-08)

- [x] prelude 不提前提交；显式失败与缺 terminal 的流拒绝伪成功。
- [x] 失败 usage 保留可信事实，无 authoritative usage 不猜计费。
- [x] 已输出后避免重试和普通 JSON；保留 Base 其他 provider 行为。
- [x] Chat/Responses/Compact 相关回归通过，独立功能提交。

## Verification

- 独立提交：`96aa88a39`，`fix: preserve Codex Chat stream failures and billing facts`。
- stream／buffered 原生 usage、错误分类、输出前／后失败与截断回归通过，见 `/tmp/codex-chat-native-usage-green.log`、`/tmp/codex-chat-error-classification-green.log`。
- 最终复审补齐全局 Chat→Responses 策略兼容，以及 completed／真实输出／缺 usage 时的 Base fallback；失败、截断、空内容及原生显式零 usage 不估算。
- root 相关完整回归通过，其他 provider 对照保留。详细结果见 `docs/codex-management-migration-verification.md`。
