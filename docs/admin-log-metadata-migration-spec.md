# 管理员日志请求元数据与缓存命中率 Spec

## 背景

Overlay 在日志详情中增加了请求体大小和缓存命中率，帮助管理员判断请求规模、缓存使用情况和上游计费行为。当前 Base 已经在请求日志的 `admin_info` 中记录请求元数据，并保留了比 Overlay 更完整的日志详情、脱敏和缓存计费字段，因此本次采用“核对并适配”，不覆盖 Base 的日志组件。

## 目标

管理员打开请求日志详情时，可以看到：

- `User Agent`（已有能力，继续保留）；
- `Request Body Size`，使用实际保存的请求体大小；
- `Cache Hit Rate`，按协议语义计算并格式化为百分比。

这些字段必须只出现在管理员详情中。普通用户既不能通过详情页面看到，也不能从日志 API 的 `admin_info` 中获得。

## 行为契约

### 后端日志

- 成功请求和可记录的渠道错误都通过 `AppendRelayLogAdminInfo` 写入 `admin_info`。
- `user_agent` 去除控制字符并限制最大长度，避免日志污染和无界增长。
- `request_body_size` 优先取 Base 的 `BodyStorage.Size()`；没有请求体存储时回退到正的 HTTP `Content-Length`。
- 请求体大小小于等于零时不写入字段。
- `model.formatUserLogs` 的现有管理员字段剥离规则继续生效，普通用户响应不包含整个 `admin_info`。

### 前端展示

- 仅当 `isAdmin` 为真时显示管理员元数据。
- 请求体大小大于零时使用已有 `formatBytes` 展示 B、KB、MB 或 GB。
- 非 Claude 模型使用 `cache_tokens / prompt_tokens`。
- Claude 模型使用 `cache_tokens / (prompt_tokens + cache_tokens + cache_creation_tokens)`，因为 Claude 的 `prompt_tokens` 不包含缓存读和缓存写 token。
- `prompt_tokens` 或缓存读 token 不为正时不显示缓存命中率；只有缓存写 token 时不显示命中率。
- 非 Claude 与 Claude 的结果都显示一位小数。

## Overlay 对齐决策

参考提交：

- `26b5e8e19`：记录管理员日志 User Agent；
- `7f528ecbb`：增加请求体大小和缓存命中率；
- `396458e0c`：修正 Claude 缓存命中率公式。

Base 不直接复制 Overlay 的详情组件。Base 当前组件包含更多计费、审计、插件和请求诊断字段；同时 Base 已增加 UA 清洗、请求体存储读取和统一成功/失败日志路径，属于更安全且兼容现有架构的实现。仅复用本功能的字段契约与公式。

## 验收标准

- 管理员详情显示 User Agent、正请求体大小和有效缓存命中率。
- 普通用户详情不显示上述三个字段。
- OpenAI 风格缓存和 Claude 缓存分别按对应公式计算。
- 空 token、零缓存读、仅缓存写、非正请求体大小不会产生错误展示。
- 成功请求和错误请求都沿用同一套管理员元数据写入和脱敏边界。
- 受影响 Go 测试、Vitest、TypeScript 检查和 lint 通过。

## 非目标

- 不重写日志详情 Dialog。
- 不改变计费、缓存 token 的后端计算或日志 API 协议。
- 不把管理员元数据开放给普通用户。
- 不迁移 Overlay 中与本功能无关的日志、审计或供应商字段。
