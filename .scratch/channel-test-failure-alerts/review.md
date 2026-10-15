# 渠道测试失败告警：双轴审查

Branch: tokendazi_op
Baseline: 8b89ed6db061d8fd70ba02f77735ad2f6bef18d0
Status: resolved

## 审查范围

- 两名独立只读审查代理并行检查相对固定基线的实现与测试；修正后分别复查，未将两个轴混合排序。
- Standards 依据根 / web 的 AGENTS.md、项目测试与领域约定，以及 Fowler smell baseline；Spec 依据本目录 spec.md 与已确认 tickets。
- 最终测试、真实三数据库及已知基线问题另见 [verification.md](verification.md)。审查通过不代表真实公网服务或全站安全合规已验证。

## Standards

首次发现 3 项，均为 P2：

1. 硬规范：数据库降级测试遗留进程内渠道冷却，重复执行可能受到前次缓存影响。已将该用例归入现有 service fixture，并显式备份、重置、恢复本地冷却表；重复执行验证见 verification.md。
2. 硬规范：旧配置控件缺席断言在未渲染 DOM 上执行，不能保护真实页面行为。已移至实际渲染并等待治理表单加载完成的测试中。
3. 判断性问题（Data Clumps）：AI Key、Base URL、Model 分次读取可能混合不同热更新版本，将旧密钥发往新目标。已引入完整连接配置值，由同一个受锁快照解析并传递；先以可控随机数边界复现，再观察回归通过。

复查结果：硬违规无；Fowler 判断性问题无。冷却 fixture、实际 DOM 断言与完整连接快照均已核对；新增 DNS context 改动保留旧调用 API、贯通预算与取消，未发现新增规范风险。Standards 代理本轮只读核对代码及既有 RED / GREEN 日志，未自行重跑完整测试。

## Spec

首次发现 1 项 P2：

1. spec.md 的 60 秒分析预算及任务取消契约要求覆盖整个外部调用，但同步 DNS 预检发生在预算之外，不能被父任务取消。已在 AI / 钉钉入口先建立 60 / 10 秒 context，DNS 使用 context resolver，预检、受保护请求与重定向传递请求 context；未移除或放宽原 SSRF 策略。

复查结果：原 P2 已关闭；缺失或部分实现无，未要求的范围扩张无，未发现新增实现错误。context 贯通属于原预算与取消契约的必要修复。代理定向运行 DNS 取消及分析连接快照两项回归，均通过；未重跑全套或数据库矩阵。

汇总：Standards 首轮 3 项（轴内最严重 P2），复查未关闭 0 项；Spec 首轮 1 项（轴内最严重 P2），复查未关闭 0 项。
