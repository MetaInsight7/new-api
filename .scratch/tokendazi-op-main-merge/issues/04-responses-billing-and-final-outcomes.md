# 04: 结算 Responses 终止 Usage 并保持失败指标保真

**Spec:** `../../2026-10-23-tokendazi-op-main-merge-spec.md`


**Type:** task
**What to build:** 让 Responses 成功、无输出、加密 reasoning 失败和 Codex terminal failure 都只执行一次正确结算，并让数据库健康统计和 Prometheus 继续采用二开的失败保真口径，同时吸收“正常完成后 context 才取消仍算成功”的上游修复。

**Blocked by:** 03: 统一 Relay 错误协议与并发饱和语义

**Status:** resolved

- [x] 消费日志和附加内容统一使用结构化英文源消息，不恢复旧字符串格式。
- [x] 仅有本地估算且没有真实输出交付时 usage 归零；权威 usage 存在时不被本地估算覆盖。
- [x] terminal failure 前已收到的可靠 usage 只结算一次，请求仍只记录一次失败性能样本。
- [x] 错误结算失败时按既有策略保留预扣并记录原因，不发生静默退款、双扣或重复累计用户/渠道用量。
- [x] 加密 reasoning 状态缺失、Codex terminal evidence、search/tool usage 和 Responses 完成事件均保持既定语义。
- [x] 业务拒绝、客户端取消、首事件超时和真实终止错误继续计失败；正常 `max_tokens` 结束计成功；完整结束后 context cancellation 仍计成功。
- [x] 数据库聚合、hot bucket、Prometheus、健康详情、recent history 和七天边界测试全部覆盖合并后的统一口径。
- [x] 如上游忽略失败样本的口径与二开失败保真无法兼容，必须保持二开口径并标记 `needs-info` 说明差异，不得无声恢复 `ignored` 行为。

## Answer

- Responses 权威 usage、无输出归零、terminal failure 单次结算和结算失败保留预扣已组合，结构化英文消息替代旧字符串附加内容。
- 失败指标继续采用二开保真口径，仅正常结束后的 context cancellation 归成功；perf、Prometheus、recent history 与七天边界测试通过。
