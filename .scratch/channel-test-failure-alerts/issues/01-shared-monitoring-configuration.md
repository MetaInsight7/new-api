# 01: 统一监控配置，纠正 Codex 独立配置

**What to build:** 管理员在“监控与警报”配置一次钉钉和分析服务，现有 Codex 通知与公告分析使用同一配置；原独立入口被直接替换，不做旧结构兼容。

**Blocked by:** None (can start immediately).

**Status:** resolved

- [x] 共享机器人及 AI 配置通过受保护的设置 API 和表单保存，运行中的 Codex 路径实际读取共享值。
- [x] 移除旧独立接入字段和读写入口，不搬迁旧值、不增加别名或回退，治理规则与独立冷却不变。
- [x] master 启动时幂等删除 6 个已废弃的 Codex 独立配置数据库行；只记录删除数量，不读取或迁移旧值，共享配置和独立冷却不受影响。
- [x] 明确保存的接口 / 模型值压过环境变量，即使值等于默认值；未配置时才回退，重载及重启一致。
- [x] 密钥不回显，空白保留，显式清除与替换可区分；非法输入及失败保存不继续启用。
- [x] 配置与 HTTP 行为在现有测试边界有回归，受影响后台测试及类型检查通过。

## Comments

- 2026-10-15：用户确认三张 ticket 并授权 implement；认领第一张，测试边界沿用已确认 spec。
- 2026-11-11：删除的旧键为 `codex_model_governance_setting.` 下的 `dingtalk_enabled`、`dingtalk_webhook_url`、`dingtalk_secret`、`ai_analysis_api_key`、`ai_analysis_base_url`、`ai_analysis_model`；不删除 `alert_cooldown_minutes`。

## Answer

- 共享配置已接入选项 API、监控表单和 Codex 运行路径；旧独立字段无搬迁、别名或回退，残留数据库行由 master 启动清理。
- 设置 API 回归验证明确保存默认值优先于环境变量、两次重载、旧字段失效及敏感值不回读；后台聚焦测试和类型检查通过，最终全量验证统一记录于 verification.md。
