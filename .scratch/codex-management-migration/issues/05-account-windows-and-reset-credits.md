# 05: 单账号真实额度窗口与安全重置

**What to build:** 保留 Base 较完整的账号用量弹窗，校正缺失账号信息和实际窗口语义，并让额度消费、认证重试和随后刷新向管理员反馈真实结果。

**Blocked by:** 03: 管理员 Codex Limits 用量与额度报表（共用窗口展示契约）。

**Status:** completed (2026-10-08)

- [x] 账号 ID 展示／复制、真实及未知窗口、附加限额顶层／嵌套结构正确解析；不回退现有券详情和布局。
- [x] 同次消费认证刷新重试复用同一 redeem_request_id；响应限额、限流、防重复提交与原有渠道操作授权存在。
- [x] 超时或其他不确定失败不自动生成新 ID 再消费；不声称跨请求／跨标签全局幂等。
- [x] windows_reset=0 不显示虚假重置成功，消费成功后的用量刷新失败不被误报成消费失败。
- [x] 使用现有凭证刷新服务并检查持久化错误；保留凭证／账号信息脱敏边界。
- [x] 上游 HTTP 回归、相关前端交互测试、类型检查、lint 和 Playwright 确认／失败场景已验证。

## Verification

- 独立提交：`d68fe1a49`，`fix: align Codex account windows and reset credit handling`。
- 三数据库 OAuth／Reset HTTP 回归通过，见 `/tmp/codex-auth-reset-{sqlite,mysql,postgres}.log`。
- 真实／未知／附加窗口、同次消费 ID、零重置与刷新失败交互回归通过；生产 Playwright 验证确认、权限和诚实反馈。
- 上游消费受控模拟，没有消费真实 Reset Credit，不声称跨请求全局幂等。详细结果见 `docs/codex-management-migration-verification.md`。
