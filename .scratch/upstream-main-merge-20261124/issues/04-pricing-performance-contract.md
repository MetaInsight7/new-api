# 04: 定价可见性与性能统计口径

**What to build:** 用户只能获得自己可访问分组的模型定价，管理员性能统计继续以真实 success/failure 二态反映平台体验，同时吸收官方即时任务采样与 hot bucket 能力。

**Blocked by:** None (can start immediately).

**Status:** resolved

- [x] 定价接口使用认证上下文中的用户分组，不额外查询用户身份。
- [x] 私有分组在生成任何模型倍率前即被过滤，通配模型和显式零倍率仍正确工作。
- [x] 性能统计不恢复 ignored 第三态；客户端取消、业务拒绝和参数拒绝继续计入真实失败。
- [x] 已有明确正常完成信号的流式请求不会被随后取消误记为失败，非流式请求不做无证据的完成推断。
- [x] 官方即时任务采样与 hot bucket 合并能力被保留，但 Q6 混合实现继续冻结。
- [x] 通过可执行的定价可见性和性能统计定向测试；被 Q6 冲突阻断的检查明确记录而不宣称通过。

## Answer

- 定价接口直接读取认证中间件写入的 `ContextKeyUserGroup`，先按可用分组过滤共享定价缓存，再生成 group/group-model 倍率；私有分组、`all`、通配模型和显式零倍率均有回归覆盖。
- 性能统计保持 Tokendazi 的 success/failure 二态：业务拒绝、参数拒绝、客户端取消和本地策略失败均计入失败，不引入上游的 ignored 第三态。
- 流式请求只有已有 completed + EOF/DONE 且无错误时，才允许忽略随后发生的 context cancel；非流式请求在取消后不推断为已完成。
- 保留官方即时异步任务成功采样、近期 hot bucket 合并和七日非空小时序列能力。
- `gpt-image-2` 仍保留 Tokendazi 生产用旧倍率默认；上游内置表达式快照用无旧倍率覆盖的 `gpt-image-2.5-sunburst` 验证，避免测试错误地改变线上计费优先级。
- 通过：`TestGetPricing*` 控制器测试、内置定价快照测试，以及 `go test ./pkg/perf_metrics` 全包测试。
