# 二开最新版 Codex 渠道迁移规格

> 状态：已确认，进入 ticket 拆分和实现阶段。\
> 目标分支：`tokendazi_op`\
> Base：当前 `new-api` `main` 架构及其已有迁移\
> 来源：`dev-new-api` `solve/main`（当前审计基线 `762ce21c4`）\
> 文档日期：2026-10-05

## 1. 问题与目标

当前 `tokendazi_op` 已经继承了 Base 的 Codex Responses 基础能力，并且此前已经迁移和验证了模型可用性探测相关能力。但二开最新版还在 Codex 渠道上增加了 Chat bridge、图片能力、渠道指纹、全局客户端身份和若干终止事件处理。

本次目标不是把二开分支整体合并到 Base，也不是复制二开的旧目录，而是：

1. 以 Base 当前的 relay、DTO、计费、重试和安全边界为底座；
2. 逐项迁移二开最新版 Codex 的实际新增能力；
3. 对二开使用的旧接口进行适配，避免回退 Base 已经完成的修复；
4. 让 `temp_url` 使用 Tokendazi 自己的 Cloudflare R2，而不是二开作者的 GCS；
5. 每个功能独立提交、独立验证，前端变更必须使用 Playwright 验证。

## 2. 对比基线与迁移原则

### 2.1 基线

本次只使用以下两个代码基线判断差异：

- Base：`new-api` 的 `main` 及当前 `tokendazi_op` 已有、且已确认需要保留的迁移；
- 二开最新版：`dev-new-api` 的 `solve/main`。

`tokendazi-overlay` 只作为历史背景，不作为本次 Codex 功能的实现来源。此前已经确认，Overlay 没有二开最新版新增的 Codex 指纹管理和全局 Codex Identity 功能。

### 2.2 不允许整分支覆盖

- 不直接把 `solve/main` 合并到 `tokendazi_op`；
- 不直接覆盖 `relay/channel/codex`、`relay/channel/openai` 或整套前端目录；
- 不复制依赖旧 `dto`、`pkg/apicompat` 或旧 relay 生命周期的文件；
- 对每个能力提取行为、边界和测试，再适配到 Base 当前接口；
- 如果 Base 在提交时间上已有更晚、更安全或更通用的实现，以 Base 为准；
- 二开测试可以迁移为 Base 的回归测试，但不机械复制实现细节。

### 2.3 现有能力必须继续保留

以下不是本次重写目标，迁移过程中不得回退：

- `/v1/responses`；
- `/v1/responses/compact`；
- `/v1/alpha/search`；
- Codex OAuth 凭证解析、刷新和 401 后重试；
- Base 当前的动态 Codex 模型发现；
- Base 当前的 Responses/`relaykit` 转换、流状态和通用安全限制；
- `tokendazi_op` 已有的模型可用性探测、模型级状态和分组调度行为；
- 真实消耗才扣费的计费原则。

## 3. 功能范围矩阵

| 功能 | 二开最新版情况 | 本次决策 |
|---|---|---|
| Responses/Compact/Alpha Search | 二开也支持，但 Base 已有实现更适合作为底座 | 保留 Base，不覆盖 |
| OAuth、动态模型、额度和用量查询 | 两边均有，Base 的刷新和模型发现更适合保留 | 保留 Base，统一 Identity 接口 |
| `/v1/chat/completions` | 二开提供 Chat → Responses → Chat bridge | 迁移 |
| Codex 图像生成 | 二开通过 Responses `image_generation` 支持 `gpt-image-*` | 迁移 |
| Codex 图像编辑 | 支持 multipart，并兼容 Playground JSON data URL 附件 | 迁移 |
| Image Carrier Model | 支持渠道级覆盖、全局设置和默认模型 | 迁移 |
| 图片输入/输出 usage | 二开有更完整的图片 usage 解析 | 迁移，但不接受无依据的猜测扣费 |
| `temp_url` | 二开上传图片后生成签名 URL | 迁移，存储改为 Cloudflare R2 |
| 渠道级指纹收敛 | `off/device/session/full`，有 seed 生命周期和请求体处理 | 迁移 |
| Codex 请求最终出口策略 | 清洗客户端伪造身份并重新写入可信 Header/body | 迁移 |
| 全局 Codex Identity | 管理 User-Agent、客户端版本、官方版本同步和统一身份开关 | 迁移 |
| `response.failed`/`response.incomplete` | 二开有专项流状态、重试和 usage 处理 | 按 Base 生命周期补齐回归，不覆盖 Base |
| Codex 账号封禁/渠道状态扩展 | 二开有独立的渠道状态改造 | 不混入本次 relay 迁移，若 Base 缺失则另立渠道生命周期 ticket |
| PLG、品牌、供应商、订阅、营销 | 与 Codex relay 无关 | 不迁移 |

## 4. 功能规格

### 4.1 Chat Completions 双向桥

#### 输入与上游

- 客户端调用 `/v1/chat/completions` 时，Codex adaptor 将 Chat 请求转换为 Responses 请求；
- 上游固定请求 `/backend-api/codex/responses`；
- 上游固定使用流式响应，客户端是否要求流式由 Base relay 上下文决定；
- 必须保留 system/instructions、普通文本、多模态输入、tool、tool choice、reasoning 和客户端要求的模型字段；
- 必须清理 Codex 明确不支持的 sampling、存储和其他禁用字段；
- `instructions` 必须存在，即使值为空字符串。

#### 输出

- 客户端流式时，将 Responses SSE 事件转换为 Chat Completions chunk；
- 客户端非流式时，聚合上游事件后返回标准 Chat Completions JSON；
- 支持文本 delta、reasoning summary、tool call arguments、finish reason 和 usage；
- 只在 Base 的统一 usage/billing 入口结算，不在 bridge 内重复扣费；
- 上游非 2xx 必须保留可供 Base 重试策略使用的状态码和错误分类；
- 不把 ChatGPT/Codex 内部错误、账号信息或上游敏感 Header 直接透传给用户。

#### 明确限制

- 不把 Chat bridge 改造成通用 OpenAI Responses 转换器；
- 不为了复制二开行为而回退 Base 当前 `relaykit` 和 Responses handler；
- 已向客户端写出内容后发生终止错误，不允许透明重试造成重复内容。

### 4.2 Codex 图片生成与编辑

#### 请求能力

- 支持 `gpt-image-*` 模型，具体模型仍以 Base 的模型注册和渠道能力判断为准；
- 将 OpenAI 图片请求转换为 Codex Responses `image_generation` tool；
- 保留 prompt、size、quality、background、output format、compression 和 moderation 等受支持字段；
- 不向上游发送已经确认不支持的 `n` 等字段；
- 图片编辑支持标准 multipart 的 `image`/`image[]`/带序号文件和 `mask`；
- 同时支持 Playground JSON 中的 data URL 附件，不要求所有编辑请求都经过 multipart；
- 处理输入数量、文件类型、单文件大小和整体 SSE 响应大小限制。

#### Image Carrier Model

解析优先级固定为：

1. 当前渠道的 `Image Carrier Model`；
2. 全局 Codex 图像设置；
3. 代码内安全默认值。

该设置只决定承载 `image_generation` 请求的 Codex 文本模型，不改变用户请求的目标图像模型，也不绕过 Available Models 或渠道能力检查。

#### 返回能力

- 默认仍可返回 `b64_json`；
- 当 `temp_url=true` 时，图片不能把上游 CDN 或 Base64 泄露给客户端，必须经过 R2 上传后返回签名 URL；
- URL 返回过期时间字段，客户端可判断链接是否仍有效；
- R2 上传失败时返回清晰错误，不静默退回 Base64，也不返回上游 URL；
- 图像响应保持 OpenAI 兼容的数据结构，不将 GCS、R2、Flatkey 等内部品牌写入公开错误。

### 4.3 `temp_url` 的 R2 存储方案

#### 数据流

```text
Codex SSE image_generation_call.result (Base64)
        |
        v
校验 MIME、大小、输出格式
        |
        v
Cloudflare R2 私有 Bucket
        |
        v
S3-compatible presigned GET URL
        |
        v
客户端 ImageData.url + expiresAt/expiresIn
```

#### 存储边界

- R2 Bucket 必须是私有桶；
- 服务端只保存临时对象，不把对象内容写入数据库；
- 对象 Key 不能包含原始文件名中的路径片段；
- Key 至少包含固定前缀、用户 ID、UTC 日期和随机值；
- 通过 R2 生命周期规则自动清理，建议生命周期不超过 1 天；
- 签名 URL 默认有效期建议 12 小时，代码上限不超过 7 天；
- 不使用二开作者的 GCS Bucket、Service Account 或默认 Flatkey 名称；
- 不把 R2 Access Key、Secret Key、Endpoint 或 Bucket 凭证写入代码和 Git。

#### 配置契约

实现阶段使用独立于任务 Artifact 的 Codex 临时媒体配置，建议变量如下：

| 环境变量 | 必填条件 | 建议默认/限制 |
|---|---|---|
| `CODEX_TEMP_MEDIA_ENABLED` | 启用 R2 时 | `false`，未配置时禁用 |
| `CODEX_TEMP_MEDIA_S3_ENDPOINT` | 启用时 | R2 S3 endpoint，不允许 query/fragment/userinfo |
| `CODEX_TEMP_MEDIA_S3_BUCKET` | 启用时 | Tokendazi 自有私有桶 |
| `CODEX_TEMP_MEDIA_S3_REGION` | 启用时 | R2 使用 `auto` |
| `CODEX_TEMP_MEDIA_S3_ACCESS_KEY` | 启用时 | 仅从运行环境读取 |
| `CODEX_TEMP_MEDIA_S3_SECRET_KEY` | 启用时 | 仅从运行环境读取 |
| `CODEX_TEMP_MEDIA_S3_PREFIX` | 启用时 | `codex-temp-media` |
| `CODEX_TEMP_MEDIA_PRESIGN_TTL_SECONDS` | 启用时 | 默认 43200，最大 604800 |
| `CODEX_TEMP_MEDIA_MAX_IMAGE_BYTES` | 启用时 | 默认 20 MiB |

现有 `TASK_ARTIFACT_STORE_*` 配置暂不直接复用：该配置当前是任务产物领域的预留契约，S3 实现被明确禁用，且任务产物的生命周期、权限和 URL 语义不同。实现时可以抽取底层 S3 签名/上传模块，但不能让 Codex 临时媒体依赖任务产物开关。

#### 未配置时的行为

- `temp_url` 未请求：不需要 R2，按普通 Base64 流程返回；
- `temp_url=true` 且 R2 未启用：返回明确的服务配置错误，不能静默返回 Base64；
- 上传成功但签名失败：不返回对象 Key，不暴露未签名的私有对象 URL；
- 配置语法错误：启动时记录脱敏错误并保持禁用，不能带着半有效配置运行。

### 4.4 图片 usage 与计费

计费遵循当前 Tokendazi 已确认的原则：**只有确实发生并且有可靠依据的消耗才扣费**。

- 优先读取上游 authoritative usage；
- 识别输入文本 token、输入图片 token、输出文本 token、输出图片 token 和 total token；
- 保持 Base 已有的 cached token、reasoning token 和图片价格字段语义；
- 不把 Base64 字节长度当作 token 数；
- 不因 `response.failed`、没有可交付图片或只收到错误事件而凭空扣费；
- 二开中“usage 缺失时写入固定 fallback token”的逻辑不能直接迁移为扣费依据；
- 如果上游没有 usage，但模型配置的是已经明确发生图片生成后的固定价格，则必须由现有计费契约显式决定是否结算，不能在 Codex image handler 内自行追加扣费；
- 同一请求只能由一个统一结算入口结算，避免 Chat bridge、图像 handler 和重试流程重复扣费；
- 重试前未向客户端写出内容且没有可靠 usage 时，不结算本次失败尝试；
- 已向客户端交付部分内容且上游给出可靠 usage 时，按实际 usage 结算，并禁止再次透明重试。

### 4.5 渠道级 Codex 指纹收敛

#### 可用模式

每个 Codex 渠道支持：

- `off`：不注入服务端指纹；
- `device`：注入稳定的 installation/session 设备身份；
- `session`：在渠道 seed 基础上对客户端会话生成稳定 thread 身份；
- `full`：额外生成本次请求级 turn/window 等完整 metadata。

模式只作用于 Codex 渠道，不改变其他 relay 的请求身份。

#### 生成与生命周期

- 每个渠道拥有独立的 UUID seed；
- seed 由服务端生成、校验、回填和修复，管理员不能直接填写任意 seed；
- 稳定身份由 seed、部署命名空间和用途标签确定性派生；
- turn ID 每次请求生成，不能在并发请求之间复用；
- `CODEX_FINGERPRINT_DEPLOYMENT_NAMESPACE` 缺失时，启用指纹的请求必须安全失败，而不是使用所有部署共用的默认值；
- 渠道 seed 不能跨渠道复用；
- 编辑、批量编辑、复制渠道和渠道恢复时必须保持 seed 生命周期一致；
- 保存失败、批量更新部分失败和并发请求不能产生半写入状态。

#### 默认策略

当前建议：

- 新建渠道默认 `off`；
- 已有渠道缺失配置时按 `off` 处理；
- 管理员可以显式选择 `device/session/full`；
- 保留全局 kill switch 和单渠道关闭能力。

这是为了避免迁移上线后未经确认地改变现有 ChatGPT 账号的设备/会话识别。若最终决定完全追求二开最新版默认行为，再单独变更该默认值，不在实现中隐式改变。

### 4.6 Codex Identity 与客户端版本

Codex Identity 表示 Relay 对上游声明的客户端软件身份，不等于用户、OAuth 账号或渠道指纹。

全局设置需要支持：

- User-Agent override；
- 手动 Codex client version；
- 官方稳定版本；
- 官方版本自动同步；
- 强制统一客户端身份开关。

统一身份应覆盖：

- 普通 Responses 请求；
- Chat bridge 请求；
- OAuth 授权/刷新相关请求；
- 动态模型列表；
- 额度和用量查询；
- 模型可用性探测。

版本同步任务只在主节点或控制台任务节点运行，采用有限超时和周期性同步；网络失败时保留上一个已知稳定版本，不阻断普通请求。

当前建议默认开启强制统一身份，但必须保留关闭开关，以便上游客户端版本策略变化时快速回滚。Identity 不能覆盖渠道 OAuth 凭证，也不能把用户输入的任意 User-Agent 当作可信身份继续向上游传递。

### 4.7 请求最终出口策略

指纹和 Identity 不能只在 adaptor 初始构造时写一次。Base 的请求发送链路中若存在 Header Override、pass-through 或重试，需要在最终发送前再次收敛。

最终出口策略必须：

- 在 Header Override 之后重新写入 Authorization、account id、OpenAI-Beta、originator、User-Agent 和客户端版本；
- 删除 Cookie、tracing、未知 `x-codex-*`、客户端伪造的 Identity Header；
- 对 `pass_through_body_enabled` 的请求体重新应用 Codex metadata 策略；
- 清理重复 JSON key、超过深度限制或超过大小限制的恶意请求；
- 每次重试重新建立正确的 request state，不复用已经写出的 SSE/header 状态；
- 不记录 access token、refresh token、R2 secret、签名 URL 完整 query 或敏感 metadata。

### 4.8 Terminal failure、重试和探测兼容

- 识别 `response.failed`、`response.incomplete`、`response.completed` 等 Codex terminal event；
- 首字节前失败且没有可交付内容时，可按 Base 的渠道重试策略重试；
- 已经向客户端写出内容后失败，不透明重试；
- 保留可靠的 terminal usage 和错误分类；
- 不把无 usage 的失败事件转换成猜测 token 扣费；
- 不改变 `tokendazi_op` 已验证的模型级可用性探测和模型级阻断语义；
- 探测请求使用统一 Codex Identity，但不应因为探测失败直接污染其他模型的状态；
- 失败详情对用户做白标脱敏，完整原因只进入受控服务端日志。

## 5. 数据与接口变更

### 5.1 渠道数据

增加或适配渠道级 Codex fingerprint seed 和模式字段。字段应满足：

- seed 不进入普通管理员可编辑的明文表单；
- API 返回时不泄露可用于伪造其他渠道的内部派生值；
- 旧渠道没有 seed 时保持兼容，只有显式启用指纹或保存渠道时才生成/修复；
- 读写、批量编辑、缓存刷新和多节点并发都使用同一校验规则；
- 不新增与用户分组、渠道分组或 PLG 分组相关的领域模型。

### 5.2 全局选项

Codex Identity 和版本同步使用现有系统选项持久化机制，不新建独立账号表。必须对：

- 选项读取默认值；
- 管理端保存权限；
- 非法版本格式；
- 自动同步失败；
- 旧配置迁移；
- 选项缓存刷新

增加单元测试。

### 5.3 请求 DTO

- 图片请求增加可选 `temp_url` 布尔字段，并兼容客户端可能使用的别名解析；
- 图片响应在返回 URL 时增加过期时间字段；
- 既有 `b64_json`、`response_format` 和 OpenAI 图片响应结构保持兼容；
- 不把存储供应商字段暴露到公共 API。

### 5.4 路由与前端

- 仅增加 Codex 图片能力实际需要的接口；
- 不为了复制二开的通用 temp media 路由而扩大公开上传面；
- 若 Playground JSON 附件需要独立上传接口，必须单独验证鉴权、上传限流、文件大小和 R2 权限后再纳入 ticket；
- 管理端渠道编辑、批量编辑和系统设置增加相应配置入口时，适配当前 `web/src` 目录，不复制二开的 `web/default` 目录；
- 前端用户可见文本全部接入 i18n；
- 所有前端修改完成后运行 Playwright。

## 6. 安全与运维要求

- 不在代码、文档、测试 fixture 或 commit 中写入真实 OAuth、R2 或 GCS 凭证；
- 测试只使用假凭证、假 R2 endpoint 和内存 fake store；
- access token、refresh token、account id、指纹派生值和签名 URL query 必须脱敏；
- R2 配置默认关闭，生产启用前先验证私有桶、生命周期和最小权限；
- `temp_url` 仅返回短期 URL，不提供永久公开 URL；
- R2 上传和签名失败必须可观测，但日志不得泄露对象签名；
- 迁移上线先对单个 Codex 渠道灰度；
- 指纹默认关闭，确认上游行为和额度无异常后再按渠道启用；
- Identity 保留 kill switch；
- R2 配置错误只能禁用临时 URL，不得阻断普通 Base64 图片能力；
- 发生回滚时，普通 Responses、Chat bridge 和 Base64 图片路径不能被 R2 配置拖垮。

## 7. 测试与验收

### 7.1 后端单元和集成测试

- Chat → Responses 字段转换、禁字段清理、instructions 注入；
- 流式 Chat chunk、非流式聚合、tool call、reasoning、finish reason 和 usage；
- Responses terminal completed/incomplete/failed 以及首字节前后重试差异；
- 图像生成、multipart 编辑、JSON data URL 编辑、mask、文件类型和大小限制；
- `temp_url=false` 保持 Base64；`temp_url=true` 调用 fake R2 并返回签名 URL；
- R2 配置校验、过期时间上限、私有对象 Key 生成和失败清理；
- R2 未配置时 `temp_url=true` 返回明确错误，不泄露 Base64/上游 URL；
- 图片 usage 的 authoritative 字段映射、缺失 usage、部分失败和不扣费场景；
- 四种 fingerprint 模式的稳定性、隔离性、并发性、seed 修复和部署命名空间；
- Header Override、pass-through、重试后的最终出口身份；
- 重复 JSON key、JSON 深度、请求体大小和敏感 Header 清洗；
- Identity 默认值、手动版本、自动同步、网络失败回退和 kill switch；
- 动态模型、OAuth 刷新、用量查询和模型探测使用一致 Identity。

### 7.2 前端测试

涉及前端时增加或更新：

- Codex 渠道编辑中的 fingerprint 模式选择和说明；
- 批量编辑的 fingerprint 模式校验和提交；
- 系统设置中的 Codex Identity 表单、保存、错误和关闭开关；
- i18n 中英文文案和默认值展示；
- 不允许 seed 被普通表单直接编辑；
- Playwright 验证宽屏、窄屏、表单保存、错误状态和刷新后配置回显。

### 7.3 真实环境验证边界

- 不在没有明确授权和脱敏凭证的情况下调用真实 ChatGPT/Codex 账号；
- R2 真实验证只使用无敏感内容的小图片和测试桶；
- 真实验证后删除测试对象，确认生命周期规则和签名 URL 到期策略；
- 没有真实 R2 凭证时，使用 fake store、配置校验和本地签名测试完成代码验收，不能宣称完成公网下载验证。

## 8. 提交拆分建议

每个提交只表达一个可维护的功能，commit message 不使用编号或“迁移”代号：

1. `feat: support Codex chat completions bridge`
2. `feat: support Codex image generation and editing`
3. `feat: add Cloudflare R2 temporary media URLs for Codex images`
4. `fix: settle Codex image usage from authoritative upstream data`
5. `feat: add configurable Codex channel fingerprint modes`
6. `feat: enforce Codex request identity at final egress`
7. `feat: add global Codex client identity and version sync`
8. `fix: align Codex terminal failures with retry and billing rules`

如果某个实现同时修改多个层，但用户可见功能仍属于同一项，可以合并为一个提交；不得为了凑提交拆出无法独立验证的半成品。每个提交完成后都要先运行对应测试，再进入下一个功能。

## 9. 非目标

- 不迁移 Overlay 品牌、供应商、PLG 定价分组和充值分组；
- 不迁移订阅、钱包、营销和网站首页；
- 不迁移二开的整套 OpenAI/Claude/Gemini relay；
- 不把 GCS 默认桶、GCP Service Account 或任何作者凭证带入 Tokendazi；
- 不把临时媒体存储做成默认公开资源；
- 不在本次工作中重构整个对象存储/任务 Artifact 领域；
- 不因为 Codex 功能迁移改变 Base 的用户分组、渠道分组或计费基本模型。

## 10. 已确认决策

1. 按矩阵迁移全部 Codex 新增能力，而不是只迁移 Chat、图片和指纹；
2. fingerprint 新建渠道和旧渠道缺失值均默认 `off`；
3. Codex Identity 的强制统一身份默认开启，同时保留 kill switch；
4. R2 的 `CODEX_TEMP_MEDIA_ENABLED` 默认关闭，生产由部署环境显式开启；
5. 没有 authoritative usage 的失败请求严格不扣费，即使二开存在 fallback token 逻辑；
6. Codex 账号封禁/特殊渠道状态不纳入本次迁移，后续单独立 ticket。
