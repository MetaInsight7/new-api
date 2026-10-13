# ADR 0006：Responses 加密推理状态仅在拒绝后做一次有损续接

Responses reasoning 的 `encrypted_content` 与 provider-owned item ID 属于生成它们的上游凭证安全域，渠道亲和无法保证故障、过期或换渠后仍回到同一安全域。系统选择在上游明确返回 `invalid_encrypted_content`、归一化的 `thinking_signature_invalid`，或 “invalid signature in thinking/reasoning block” 等窄语义错误后，删除完整 `input` 中不可移植的密文和 ID、保留可读 summary/content，并在当前渠道额外重试一次；该恢复默认开启、每个入站请求最多发生一次，之后继续使用已清理请求进入普通重试策略。

该方案有意拒绝每次请求预清理，因为预清理会在没有故障时也损失隐藏推理连续性；也拒绝密文来源缓存和代理侧转换，因为 opaque 密文无法由代理安全验证或重新绑定安全域。恢复成功前的首次拒绝不视为渠道健康故障，不触发禁用、冷却、排除或错误日志；恢复事实只记录在管理员请求日志中。`previous_response_id` 与 `conversation` 保持原样，因为它们可能仍在目标上游有效，扩大删除范围会造成没有证据支持的额外数据损失。
