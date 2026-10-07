# 07: Codex 请求字段约束、显式 Fast 与 full 指纹校验

**What to build:** Chat、Responses 和 Compact 遵守来源 Codex 专属上游字段约束；显式 Fast 正确透传；full 指纹在重建前验证原始输入。

**Blocked by:** None (can start immediately).

**Status:** ready-for-agent

- [ ] Codex 共享约束用于所有入口，不缩减通用 OpenAI DTO；有效字段、显式 false/zero 保留。
- [ ] Chat 转 Responses 不丢 service_tier；默认不注入 Fast，不依赖整体 body passthrough。
- [ ] full/typed 指纹在重建前验证 metadata 形状、体积、深度，保持 raw 路径和渠道隔离。
- [ ] 聚焦回归和 root/relaykit 独立构建通过，独立提交。

## Verification

待实现。
