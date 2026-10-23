# 10: 保留用户通知与监控设置的增量保存

**Spec:** `../../2026-10-23-tokendazi-op-main-merge-spec.md`


**Type:** task
**What to build:** 让用户和管理员修改任一通知、IP 记录、quota reminder、渠道告警或性能指标设置时，都基于当前完整设置增量保存，不覆盖其他字段或丢失二开监控能力。

**Blocked by:** None (can start immediately)

**Status:** resolved

- [x] 用户设置读取现有完整对象并只覆盖提交字段；无旧设置时填充稳定默认值。
- [x] 权限限定的 IP 记录开关、通知类型开关、quota reminder 阈值和邮箱/webhook 字段不会互相覆盖。
- [x] 监控表单同时包含渠道告警、quota reminder 和上游性能指标说明/配置。
- [x] 数值 schema、默认值、保存顺序和整个表单的 pending/error 状态保持一致。
- [x] PUT payload、无设置默认、权限可见性、阈值单位、告警配置和性能设置测试通过。
- [x] 如上游字段模型与二开通知/监控对象无法无损合并，停止并标记 `needs-info`，列出迁移和数据丢失风险。

## Answer

- 用户设置保存从服务端最新完整对象增量覆盖，未选择的通知开关保持 unset；通知、quota reminder、渠道告警与性能指标保存路径已组合。
- 按既有 ADR 不恢复 `record_ip_log` 控件，旧 JSON 字段继续被忽略且日志始终记录 IP；profile 与 monitoring 定向测试通过。
