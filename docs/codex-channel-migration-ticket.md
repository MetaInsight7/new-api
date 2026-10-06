# Codex 渠道迁移 Tickets

> Parent spec：`docs/codex-channel-migration-spec.md`\
> Target branch：`tokendazi_op`\
> Source baseline：`dev-new-api` `solve/main`\
> Status：Implementation complete; targeted validation passed

本清单把已确认的 Codex 迁移拆成可独立验证的功能。每个 ticket 完成后单独测试并提交；提交信息描述实际功能，不使用 ticket 编号作为 commit message。

## Ticket 1：支持 Codex Chat Completions 双向桥

### Scope

- 在 Base 当前 Codex adaptor 上增加 `/v1/chat/completions` 到 Responses 的请求转换；
- 增加 Responses SSE 到 Chat Completions 流式 chunk 的转换；
- 增加客户端非流式请求的完整聚合响应；
- 保留 tool call、reasoning、finish reason、usage、上游状态码和白标错误；
- 复用 Base 当前 DTO、Responses handler、重试和结算入口，不复制二开的旧 relay 文件。

### Dependencies

- 无功能依赖；必须先确认 Base 当前 Responses/relaykit 接口和计费入口。

### Acceptance criteria

- 流式 Chat 请求能正确返回文本、reasoning、tool call 和结束事件；
- 非流式 Chat 请求能正确聚合为标准 Chat Completions JSON；
- `instructions` 始终存在，Codex 禁止字段不会发送到上游；
- 上游非 2xx 保留可供渠道重试使用的错误状态；
- 已输出内容后发生 terminal failure 不会重复透明重试；
- 没有新增重复计费；
- 完成 adaptor、转换、SSE、非流式、错误和 usage 测试。

### Commit

`feat: support Codex chat completions bridge`

## Ticket 2：支持 Codex 图片生成与编辑

### Scope

- 增加 `gpt-image-*` Codex image generation tool 请求构造；
- 支持标准 multipart 的 image、image[]、序号图片和 mask；
- 支持 Playground JSON data URL 图片编辑附件；
- 迁移 Image Carrier Model 的渠道级、全局和默认解析；
- 保留图片格式、尺寸、质量、背景、压缩和 moderation 等有效字段；
- 保留 256 MiB 级别的 SSE 总读取保护、输入类型/大小校验和白标错误。

### Dependencies

- Ticket 1 的公共 Codex 请求约束可以复用，但图片路径本身不依赖 Chat bridge；
- 需要先检查当前 Base Playground 图片请求分流，避免复制二开的 `web/default` 目录。

### Acceptance criteria

- `gpt-image-*` 生成请求能进入 Codex Responses image tool；
- multipart 和 JSON data URL 编辑均能正确发送 input image；
- mask、多个输入图片、非法 MIME、超大图片和非法字段均有明确结果；
- Image Carrier Model 按渠道 > 全局 > 默认顺序生效；
- 默认输出保持 OpenAI 图片响应格式；
- 如果修改前端，必须补齐 i18n 并用 Playwright 验证 Playground 生成/编辑流程；
- 完成图片请求、输入附件、响应解析和失败路径测试。

### Commit

`feat: support Codex image generation and editing`

## Ticket 3：为 Codex 图片增加 Cloudflare R2 临时 URL

### Scope

- 增加 R2 S3-compatible 私有对象上传和预签名 GET URL；
- 将 Codex `temp_url=true` 的 Base64 输出转换为短期 R2 URL；
- 增加配置校验、对象 Key 生成、最大文件大小和 URL TTL 限制；
- R2 未启用时拒绝 `temp_url=true`，不静默返回 Base64 或上游 URL；
- 不复用当前禁用的任务 Artifact S3 开关，不迁移 GCS 默认桶或 GCP 凭证；
- 不在本 ticket 扩大通用公开上传接口，只有实际需要时另开接口 ticket。

### Dependencies

- Ticket 2 的图片响应解析；
- R2 Bucket、Access Key、Secret Key 和生命周期规则由部署环境提供；代码和测试使用假凭证。

### Acceptance criteria

- `temp_url=false` 的响应行为与 Ticket 2 保持一致；
- `temp_url=true` 上传到 fake R2 后返回 URL、`expiresAt` 和 `expiresIn`；
- R2 endpoint、Bucket、Region、Prefix、TTL 和大小配置均经过校验；
- URL 最大 TTL 不超过 7 天，默认 12 小时；
- 上传失败、签名失败、配置缺失不会泄露对象 Key、Base64、上游 URL 或密钥；
- 私有对象 Key 不包含未清洗的用户文件名路径；
- 完成 fake store、配置校验、失败处理和安全日志测试；
- 有真实 R2 测试凭证时再进行一次无敏感图片公网下载验证。

### Commit

`feat: add Cloudflare R2 temporary media URLs for Codex images`

## Ticket 4：按 authoritative usage 结算 Codex 图片消耗

### Scope

- 读取 Codex 图片输入/输出 usage 和 total usage；
- 适配 Base 当前 cached token、reasoning token、图片 token 和价格语义；
- 将 usage 交给唯一的 Base 结算入口；
- 删除或隔离二开中可能导致猜测扣费的 fallback token 行为；
- 明确失败、无图片、无 usage、部分输出和可靠 usage 的收费边界。

### Dependencies

- Ticket 2 的图片 SSE 解析；
- 不依赖 Ticket 3 的 R2，Base64 和 `temp_url` 两种输出都必须使用同一计费规则。

### Acceptance criteria

- authoritative usage 能正确映射到 Base 计费结构；
- Base64 字节数不会被当作 token；
- 没有可靠 usage 且没有可交付内容的失败请求不扣费；
- fallback 不会独立制造 token 费用；
- partial output 和 terminal failure 不会重复结算；
- 成功图像生成、编辑、缓存和 reasoning usage 均有回归测试。

### Commit

`fix: settle Codex image usage from authoritative upstream data`

## Ticket 5：增加可配置的 Codex 渠道指纹模式

### Scope

- 增加 `off/device/session/full` 四种渠道级模式；
- 增加服务端生成、校验、回填和修复的渠道 seed；
- 增加 `CODEX_FINGERPRINT_DEPLOYMENT_NAMESPACE` 校验；
- 实现稳定 installation/session/thread/window 和请求级 turn ID；
- 适配单渠道编辑、批量编辑、复制渠道、缓存和并发保存；
- 新渠道和旧渠道缺失值默认 `off`；
- seed 不进入普通管理员可编辑的明文表单。

### Dependencies

- 不依赖 R2；
- 需要先明确当前 Channel DTO、缓存和批量编辑事务边界。

### Acceptance criteria

- 四种模式的字段范围和稳定性符合 spec；
- 不同渠道不会复用 seed 或稳定派生身份；
- 同一 session 的 session 模式保持 thread 稳定，不同 session 隔离；
- turn ID 不在并发请求间复用；
- 缺少 namespace、非法 seed 和保存失败均安全处理；
- 管理端可以选择模式但不能编辑 seed；
- 前端修改必须完成 i18n、组件测试和 Playwright 验证；
- 完成 seed 生命周期、并发、批量编辑和 API 响应脱敏测试。

### Commit

`feat: add configurable Codex channel fingerprint modes`

## Ticket 6：在最终出口收敛 Codex 请求身份

### Scope

- 在 Base 请求最终发送边界增加 Codex `RequestFinalizer` 或等价 seam；
- Header Override、pass-through 和重试之后重新写入可信 Authorization、account id、OpenAI-Beta、originator、User-Agent 和客户端版本；
- 清理 Cookie、tracing、未知 `x-codex-*` 和客户端伪造 metadata；
- 对 pass-through body 重新应用 Codex metadata 和 fingerprint 策略；
- 增加重复 key、深度、大小和敏感 header 防护。

### Dependencies

- Ticket 5 的 fingerprint 解析；
- Ticket 7 的全局 Codex Identity 配置；
- 必须先理解 Base `Header Override` 和请求重试顺序，不得在 adaptor 初始阶段做一次性替代。

### Acceptance criteria

- 最终发送的请求身份不受客户端 Header Override 伪造影响；
- OAuth token 和 account id 始终来自渠道凭证；
- pass-through body 不会绕过 Codex identity policy；
- 重试会重建 request state，不重复发送已写出的 SSE 状态；
- 恶意重复 JSON key、超深 JSON 和超大 metadata 得到明确拒绝；
- 测试确认日志不会出现 token、secret、完整签名 URL 或内部指纹。

### Commit

`feat: enforce Codex request identity at final egress`

### Status

已完成：最终出口策略、pass-through body 约束、敏感 Header 清理、重试隔离和对应测试已提交。

## Ticket 7：增加全局 Codex Identity 和版本同步

### Scope

- 增加系统设置中的 User-Agent override、手动版本、同步版本和自动同步开关；
- 增加强制统一客户端身份开关，默认开启并支持 kill switch；
- 让普通请求、OAuth、模型列表、额度/用量查询和模型探测使用同一 Identity；
- 增加官方稳定版本的周期性同步；
- 同步失败时保留上次有效版本，不阻断业务请求；
- 管理端适配当前 `web/src`，不复制二开的 `web/default`。

### Dependencies

- 可独立开发配置和版本解析；
- Ticket 6 的最终出口会消费本 ticket 的可信 Identity。

### Acceptance criteria

- 默认值与 fallback 行为稳定且可测试；
- 非法版本、预发布版本和同步网络失败不会污染当前有效版本；
- 只有主节点/控制台任务节点运行同步任务；
- 所有 Codex egress 路径使用相同的客户端身份；
- 管理员能保存、关闭和恢复 Identity；
- 前端文案完整 i18n，并通过 Playwright 验证表单、保存、错误和刷新回显；
- 测试覆盖手动版本、自动同步、任务幂等和 kill switch。

### Commit

`feat: add global Codex client identity and version sync`

## Ticket 8：对齐 Codex terminal failure、重试和计费规则

### Scope

- 将二开 terminal event 回归场景适配到 Base 当前 Responses 生命周期；
- 覆盖 `response.completed`、`response.incomplete` 和 `response.failed`；
- 首字节前失败允许按 Base 规则重试，已经输出内容后禁止透明重试；
- 保留可靠 usage 和错误状态；
- 无 authoritative usage 的失败不产生猜测费用；
- 确保模型可用性探测复用已有模型级状态，不新增渠道级误阻断。

### Dependencies

- Ticket 1、Ticket 2 和 Ticket 4；
- Ticket 6 的最终出口重试状态处理。

### Acceptance criteria

- 首字节前、首字节后、部分输出和终止事件都有明确测试结果；
- 重试不会重复扣费、重复写 Header 或重复发送内容；
- 上游错误对用户白标，对服务端保留可诊断原因；
- 探测失败仍遵循 `tokendazi_op` 已确认的模型级阻断和恢复语义；
- 不迁移账号封禁/特殊渠道状态扩展。

### Commit

`fix: align Codex terminal failures with retry and billing rules`

### Status

已完成：terminal failure 的白标错误、首字节前重试、已输出后停止透明重试、authoritative usage 结算和对应测试已提交。

## 不纳入本批次

- Codex 账号封禁/永久失效渠道状态：另立渠道生命周期 ticket；
- Overlay 品牌、PLG、供应商、订阅、钱包和营销功能；
- 二开整套 OpenAI、Claude、Gemini relay；
- GCS 临时媒体和二开作者的任何部署凭证；
- 无实际调用方的通用临时媒体公开上传接口。

## 实现顺序

1. Ticket 1：Chat bridge；
2. Ticket 2：图片生成与编辑；
3. Ticket 4：图片 usage 与计费；
4. Ticket 3：R2 `temp_url`；
5. Ticket 5：渠道指纹模式；
6. Ticket 7：全局 Codex Identity；
7. Ticket 6：最终出口策略；
8. Ticket 8：terminal failure、重试和最终回归。

每一步先阅读目标文件的最新 Base 实现，再实现对应 ticket；完成后运行针对性测试，前端发生变更时追加 Playwright 验证，确认通过后才进入下一步。

## 当前验证记录

- `go test ./relay/channel/codex -run 'TestFinalize|TestCodexFingerprint|TestPrepareRequestBody' -count=1`：通过。
- `go test ./relay/channel/openai -run 'Responses|Billing' -count=1`：通过。
- `go test ./service -run 'TestPostTextConsumeQuotaOnErrorSettlesAuthoritativeUsage|TestResponsesUsageAccumulator' -count=1`：通过。
- `go vet ./relay/channel/codex ./relay/channel ./relay ./service`：通过。
- 受影响包的完整测试中，依赖 `httptest`/`miniredis` 监听本地端口的既有测试受当前沙箱禁止监听端口影响，未作为业务失败判断。
