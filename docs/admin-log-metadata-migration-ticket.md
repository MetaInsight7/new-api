# Ticket: 完成管理员日志请求元数据与缓存命中率迁移

## 来源

`dev-new-api` 的 `origin/tokendazi-overlay`，对应 Overlay 提交 `26b5e8e19`、`7f528ecbb` 和 `396458e0c`。

## 实现清单

- [x] 在管理员日志详情显示 `Request Body Size`。
- [x] 在管理员日志详情显示 `Cache Hit Rate`。
- [x] 保留已有 `User Agent` 展示。
- [x] 补充 `LogOtherData.admin_info.request_body_size` 类型。
- [x] 抽取缓存命中率计算函数，覆盖 OpenAI 风格和 Claude 公式。
- [x] 保证请求元数据仅通过 `admin_info` 暴露给管理员。
- [x] 覆盖管理员/普通用户显示边界和缓存公式测试。
- [x] 核对成功请求与渠道错误使用同一套后端元数据路径。
- [x] 使用定向 Go 测试和 Vitest 完成验证。

## 已有实现

- 后端：`service/log_info_generate.go` 的 `AppendRelayLogAdminInfo` 统一写入 User Agent 和请求体大小；渠道错误路径复用该函数。
- 类型：`web/src/features/usage-logs/types.ts` 声明管理员请求元数据。
- 展示：`web/src/features/usage-logs/components/dialogs/details-dialog.tsx` 增加管理员详情行。
- 计算：`web/src/features/usage-logs/lib/format.ts` 的 `getCacheHitRate` 集中处理协议差异。
- 测试：管理员元数据、普通用户隐藏、缓存公式和后端请求元数据均有定向覆盖。

## 完成定义

该功能已完成迁移。后续 Base 合并时只需处理日志详情组件和日志元数据 helper 的常规冲突，不应覆盖 Base 的完整日志详情组件或移除 Base 的脱敏逻辑。
