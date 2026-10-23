# 01: 保留分组路由与事务化渠道生命周期

**Spec:** `../../2026-10-23-tokendazi-op-main-merge-spec.md`


**Type:** task
**What to build:** 让真实请求在合入上游模型名归一化和任务渠道能力后，仍按二开的分组优先级、权重、约束和重试策略选出渠道；同时保证多 Key 状态、渠道生命周期、能力状态与 Codex 治理继续原子更新。

**Blocked by:** None (can start immediately)

**Status:** resolved

- [x] 精确模型没有合格渠道时才执行一次路由模型名 fallback，且 fallback 不绕过原始请求的渠道约束。
- [x] 分组 priority/weight、retry 降级、同级随机、非法渠道检测和权重溢出保护保持二开语义。
- [x] 普通渠道参与共享 task-plugin endpoint 竞争，同时保留排除渠道和专用插件约束。
- [x] 多 Key 状态、轮询游标、渠道状态、ability、Codex fingerprint/治理和缓存通知保持事务与锁一致性。
- [x] 渠道测试继续支持跳过日志及自定义日志名称，消费日志迁移到结构化消息且默认使用英文源文案。
- [x] 路由、状态事务和渠道测试的定向回归通过，并覆盖精确模型与 fallback 两条路径。
- [x] 实施前对比上游、共同祖先和二开行为；若存在无法同时满足的语义冲突，停止实施、将状态改为 `needs-info` 并记录影响与方案，不得静默选择上游实现。

## Answer

- 精确模型优先、单次 fallback、分组 priority/weight/retry 与 task-plugin 约束已组合；多 Key、ability、Codex 治理和缓存发布继续在事务与行锁内更新。
- 渠道测试保留跳过日志和自定义名称，结构化英文消息已接入；模型路由、事务和渠道测试定向回归通过。
