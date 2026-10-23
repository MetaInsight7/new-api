# 14: 完成二开能力冲突审计与合并就绪检查

**Spec:** `../../2026-10-23-tokendazi-op-main-merge-spec.md`


**Type:** task
**What to build:** 在后端和前端验证完成后，对最终合并结果同时相对上游、二开基线和本 Spec 做一次能力审计，确认没有静默删除、放宽或改变二开行为，并形成可审计的 merge commit 建议。

**Blocked by:** 12: 执行后端、三数据库与协议全量验证；13: 执行前端全量验证

**Status:** resolved

- [x] 逐项核对 Spec 中计费、流式、渠道路由、Codex、错误安全、权限和设置保存不变量，并关联测试证据。
- [x] 对比最终树与二开基线，所有删除或行为替换均有明确上游依据或已批准决策。
- [x] 对比最终树与固定上游基线，安全、计费、协议和依赖修复未在解冲突时丢失。
- [x] 汇总能力冲突、组合方案、用户确认和遗留风险；无未确认冲突。
- [x] 确认无 unresolved path、冲突 marker、未解释的 merge 回归、意外生成物或对原 `tokendazi_op` 引用的修改。
- [x] 输出测试矩阵、未执行环境项、merge commit 边界和后续基线债务建议；未 push。
- [x] 未发现二开能力受限、权限放宽、计费变化或状态机回退，无需重开票据。

## Answer

- 第 6/8 节逐项审计通过：constraints 双字段、配置 helper、`tokenkit` usage、Stripe 国家归因、Codex proxy/TLS、header override→finalizer、Relay 精准脱敏+cooldown、用户增量设置、始终记录 IP、Search Provider 单实例、Channel Card 双能力和 Operations 字段均保留。
- 对二开基线仅有三类删除：host token estimator 由 `tokenkit` 取代；Privacy Card 按 `docs/adr/0004-console-default-layout-and-ip-logging.md` 删除并改为始终记录 IP；旧 Gemini Responses 预处理由新 converter 取代，custom-tool history 由现有 relaykit 测试保护。不存在静默功能删除。
- 组合审计额外修复了上游脱敏语义变化对二开渠道告警的影响（外发前继续屏蔽 URL 与裸域名），并补齐 Stripe 固定日志 `LogText`、Codex billing fixture、英文源文本和新版分词断言；没有以修改测试掩盖真实行为差异。
- 最终树相对 `tokendazi_op@0f542cd5e` 和 `main@7fb6e57a5` 完成审计；原 `tokendazi_op` 引用仍为 `0f542cd5e`。检查结果为 0 unresolved path、0 conflict marker、0 whitespace error，未 push，无 `needs-info`。
- 建议边界为一个最终 merge commit，包含自动合并、冲突组合修复、Spec 与 14 张票；不混入其他 `.scratch/` 资料。后续仅需独立处理父分支既有 lint/format debt，并在具备 DSN 的环境补跑 MySQL/PostgreSQL/audit/ClickHouse 矩阵。
