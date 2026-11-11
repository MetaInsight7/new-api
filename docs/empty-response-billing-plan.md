# 空回复计费分阶段计划

## 背景

当前流式 Chat Completions 已有一层保守保护：当 usage 来自本地估算、没有 completion token，且没有任何响应写给客户端时，最终结算把本地估算 usage 清零。该保护会在统一结算入口同时阻止 token、固定按次和 tiered expression 计费。

OpenAI 兼容非流式上游可能返回 HTTP 200、`choices: null` 或空 choices 以及零 usage。通用 OpenAI 兼容 handler 目前会把零 usage 回填为本地估算的 prompt token，但非流式没有接入上述保护，因此会产生空回复扣费。

## 当前决策

先补齐非流式与现有流式保护的计费语义，不在本阶段引入完整的 Relay 输出状态重构。

本阶段只处理以下事实组合：

1. 请求进入通用 OpenAI Chat Completions 或 Responses 非流式处理/转换链；
2. 上游没有返回协议输出（Chat 的 choices 或 Responses 的 output 为 `null`、缺失或空数组）；
3. usage 因上游零 prompt usage 而由本地回填；
4. 本地回填没有产生 completion token。

满足以上条件时，最终结算复用现有 `zeroLocalUsage` 分支，把计费 usage 清零，并跳过固定按次与 tiered expression 结算。该规则与具体渠道无关；不解析任何供应商私有错误字段，不增加自动重试、渠道禁用或人工 `502 empty_response`，也不改变现有 HTTP 状态和响应转发语义。

有 choices/output 的响应继续按现有行为处理，包括文本、reasoning、tool/function call 以及旧 Completions 输出。当前阶段不尝试判定“存在 choice/output item 但字段全部为空”的更细粒度语义，以避免把协议状态重构混入事故修复。

## 本阶段实现

1. OpenAI 非流式 handler 在回填 usage 时明确标记“本地计数”。
2. 当 Chat/Responses 响应没有 choices/output 时，再记录一个仅用于本阶段结算的空回复事实；每个渠道 attempt 初始化时成组清除本地计数与空回复标记，避免重试污染。
3. `shouldZeroLocalUsage` 对流式保留原判断；对非流式只接受上述显式空回复事实。
4. 使用供应商无关的 `choices:null + usage:0` 响应增加 handler 回归测试。
5. 增加结算判断测试，确认普通非流式本地估算不会被一概退款。

## 明确不在本阶段处理

- MiniMax `base_resp.status_code` 的 JSON、upfront JSON 与 SSE 业务错误映射；MiniMax adaptor 本阶段完全不修改；
- 流式路径把任意 SSE 帧视为已发送响应的粗粒度判断；
- 上游已报告 prompt usage、但没有有效输出时的统一免计费政策；
- “存在 choice 但只有空字段”的协议级有效输出判断；
- 客户端主动取消、网络断开和平台写出失败的责任归属；
- Chat Completions、Responses、Claude、Gemini 等协议的统一输出状态模型；
- 空回复自动重试、自动禁用渠道或渠道健康惩罚。

## 后续完整方案

后续单独引入请求级 `RelayOutcome`，协议 handler 只记录事实，结算层统一应用政策：

- `effective_output_observed`：上游已产生文本、reasoning、tool/function call、audio、refusal 或其他有效模型输出；
- `effective_output_delivered`：有效输出已成功写给客户端；
- `terminal_reason`：正常结束、客户端断开、上游错误或平台错误；
- `usage_authority`：上游可信 usage、本地估算或无 usage。

完整方案应覆盖流式与非流式，并用统一结算决策替代分散的 context 布尔标记。MiniMax `base_resp` decoder 作为 provider-specific 错误识别独立实现，`2056` 映射为 `429 insufficient_quota`，同时覆盖 JSON 和 SSE 帧。

## 验收标准

- 任意 OpenAI 兼容渠道的非流式 `choices:null + usage:0` 不再扣除本地估算的 prompt 费用；
- 空回复仍沿用现有 HTTP 状态和响应格式，不被改写成错误；
- 有 choices 的非流式响应不受本阶段保护误伤；
- 现有流式行为保持不变；
- 不提交、推送或部署，直到单独确认。
