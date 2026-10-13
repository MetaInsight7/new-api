# Responses 加密推理状态有损续接验收

日期：2026-10-12。分支：`tokendazi_op`。

## 实现边界

- 只处理 Responses HTTP、Compact、SSE 和 WebSocket 在上游明确拒绝加密推理状态后的恢复。
- 识别 `invalid_encrypted_content`、`thinking_signature_invalid`，以及 encrypted content 无法验证／解密／解析和 `invalid signature in thinking/reasoning block` 的窄语义消息。
- 清理完整 `input` 中 reasoning item 的 `encrypted_content` 与 provider-owned `id`，保留非空 summary/content；不修改 `previous_response_id`、`conversation` 或 `output`。
- 每个入站请求最多清理一次，先在当前渠道额外重试；失败后才进入普通重试预算。首次拒绝不处罚渠道。
- 配置默认开启；管理员可在请求策略的会话与重试区域关闭，并看到有损连续性警告。

## 自动检查

- `go test ./service ./model ./relay ./relay/channel/openai ./controller ./setting/operation_setting -count=1`：以最终文件状态全部通过；`controller` 全量套件耗时约 137 秒。
- `go test ./controller -run 'TestResponses(HTTP|Compact|WebSocket).*(EncryptedReasoning|ThinkingSignature)' -count=1`：通过。
- `go test ./relay/channel/openai -run 'TestOaiResponsesStreamHandler.*EncryptedReasoning' -count=1`：通过。
- `bun run i18n:sync`：命令退出码为 0；它同时补写了大量本功能之外的既有缺失翻译，已撤销这些同步噪音，只保留本功能七种语言各五个新键。
- `bun run typecheck`：通过。
- `bun run test -- src/features/system-settings/request-policies`：3 个文件、52 项测试通过，其中 routing 文件 31 项。
- 对本功能四个 TS／TSX 文件执行定向 `oxlint`：通过；执行定向 `oxfmt --check`：通过。
- `bun run build`：以最终七语文件状态生产构建通过，Rsbuild 报告 `built in 30.8s`；只有既有 route test 文件命名警告。
- `go build -o /tmp/new-api-responses-encrypted-recovery .`：包含最终前端产物的根程序编译通过。
- `git diff --check`：通过。

## 已知基线

- 全库 `bun run lint` 仍被本功能之外的既有 lint 错误阻断，例如 `src/components/page-transition.tsx`、`src/features/channels/lib/upstream-update-utils.ts` 和 `src/lib/utils.ts`；本功能涉及文件的定向 lint 为零错误。

## 结论

- HTTP 与 Compact 能在普通重试次数为零时完成同渠道额外恢复；清理后仍被拒绝只发送两次，不形成恢复循环。
- SSE 在提交业务输出前隐藏首次失败并恢复；已经输出或存在权威用量时不重放。
- WebSocket 在接受前同连接恢复，二次失败可按普通预算换渠；关闭开关或非加密类拒绝保持原行为。
- 管理员请求策略日志记录恢复决策，但不记录密文、摘要正文或用户输入。
