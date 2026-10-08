# 07: Codex 请求字段约束、显式 Fast 与 full 指纹校验

**What to build:** Chat、Responses 和 Compact 遵守来源 Codex 专属上游字段约束；显式 Fast 正确透传；full 指纹在重建前验证原始输入。

**Blocked by:** None (can start immediately).

**Status:** completed (2026-10-08)

- [x] Codex 共享约束用于所有入口，不缩减通用 OpenAI DTO；有效字段、显式 false/zero 保留。
- [x] Chat 转 Responses 不丢 service_tier；默认不注入 Fast，不依赖整体 body passthrough。
- [x] full/typed 指纹在重建前验证 metadata 形状、体积、深度，保持 raw 路径和渠道隔离。
- [x] 聚焦回归和 root/relaykit 独立构建通过，独立提交。

## Verification

- 独立提交：`a664c390d`，`fix: align Codex request filtering and explicit Fast mode`。
- root 相关完整回归、后端构建，以及 `relaykit` 独立构建和 `oai_chat`／`oai_responses` 测试通过，见 `/tmp/codex-management-go-final-regression.log`、`/tmp/codex-management-relaykit-final.log`。
- 保护显式 Fast、false／zero、Codex 渠道隔离及 typed／full 输入校验；指纹默认保持 `off`。详细结果见 `docs/codex-management-migration-verification.md`。
