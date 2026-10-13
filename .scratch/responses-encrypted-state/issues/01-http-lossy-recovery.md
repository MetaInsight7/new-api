# 01: 打通 HTTP Responses 有损续接

**What to build:** 管理员可以开启或关闭默认启用的 Responses 加密推理状态恢复；普通 Responses 与 Compact 在明确拒绝密文后清理完整 input，并在当前渠道额外重试一次，同时保留未知请求字段和普通重试预算。

**Blocked by:** None (can start immediately).

**Status:** completed (2026-10-12)

- [x] 精确代码和窄语义错误能够触发恢复，包含 `invalid signature in thinking block`，相似普通错误不能触发。
- [x] 所有 reasoning item 删除 `encrypted_content` 与 provider-owned `id`，保留非空 summary/content；无可读内容的 reasoning item 被删除。
- [x] pass-through 与转换路径使用同一清理结果，未知顶层字段不丢失。
- [x] 当前渠道额外重试一次且不消耗普通重试次数；成功时不处罚、不排除渠道、不写错误日志。
- [x] 恢复仍失败时不再次清理，普通重试继续使用已清理请求。
- [x] 全局配置默认开启，后端请求策略接口能够读取、校验和更新它。
- [x] 系统设置的会话与重试区域显示开关及醒目的有损说明，并只保存变化字段。
- [x] HTTP、Compact、配置模型和前端行为测试通过。
