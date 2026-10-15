# 协议级心跳排除与桥接兼容

Status: resolved
Type: task
Blocked by: 01-admin-first-event-timeout.md
Spec: ../spec.md

## Scope

- 按真正的上游协议识别 Claude Messages `type=ping`，覆盖原生与桥接路径。
- 保持协议转发和旧 TTFT 口径；元数据、思考、工具调用以及被原解析器接受的未知事件算首事件。
- 不通过下游协议、渠道品牌、文本包含 ping 或缺少 content 猜测心跳。
- 实际上游错误、终态、畸形输入和 EOF 保持原语义；不改变 Usage 和计费。

## Acceptance

- 协议/桥接回归覆盖 ping-only 超时、非文本进展、错误与完成优先，以及非 Claude 的同形数据。
- 定向测试、全量验证、双轴审查后按 implement 工作流提交当前分支，不 push。

## Comments

- 2026-10-15：01 已完成，开始协议级心跳与桥接回归。

## Answer

- 在真正解析 Claude Messages 的入口识别合法且无错误的 `type=ping`，覆盖原生、chat、Responses 和 Gemini 下游格式；保留转发、原 TTFT 和用量口径。
- 非 Claude 的同形数据、元数据、思考、工具调用和接受的未知事件保持进展语义；上游错误、畸形错误、EOF 和终态不被心跳保护吞掉。
- 所有共享 scanner 适配器贯通首事件失败；已提交的 Claude、原生 Responses 及已有 Responses 桥接尽力发送协议内失败，禁止伪造完成。
- 补齐背压、取消后已排队 Usage、迟到事件、工作退出和并发保存/同步回归；未改变重试、租约或计费规则。
- 定向、race、真实三库、宿主全量测试/vet、独立 relaykit 和后台类型检查/lint/构建通过。后台全量 2396 通过、1 个既有模型列表用例失败，已在未修改基线复现。
- Standards 初审 1 项、Spec 初审 2 项均修复并由原审查者复查关闭。详细命令及限制见 ../verification.md；分轴审查见 ../review.md。
