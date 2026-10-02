# TokenDazi Overlay 到 Base 迁移记录与实施计划

## 1. 文档目的

本文档记录将 TokenDazi Overlay 最终代码迁移回 Base 项目的范围、判断依据、迁移状态和实施规则。

本文档不是将两个项目整体合并的方案。Overlay 仅作为功能最终行为和界面最终效果的参考实现，Base 的现有实现、上游更新和项目架构仍然需要保留。

## 2. 项目与分支

| 角色 | 项目/分支 |
| --- | --- |
| 目标项目 | `/root/newapi/new-api` |
| 目标分支 | `tokendazi` |
| 功能来源 | `/root/newapi/dev-new-api` 的 `origin/tokendazi-overlay` |
| Base 基线 | 目标项目 `main` 及其后续上游更新 |

两个仓库虽然可以追溯到共同上游提交 `4ca47ee2`，但当前分支已经分别经过 Base 上游更新、二开改造和官方架构演进，不能把 Git 的共同祖先当成可直接覆盖的迁移边界。因此不执行整分支合并，也不使用 `--allow-unrelated-histories` 生成正式合并提交。完整差异必须通过最终代码树、提交历史和功能行为三者结合分析。

## 3. 总体迁移原则

### 3.1 以 Overlay 最终代码为功能结果标准

不能只查看某个 `feat` 提交。每项功能都必须追踪：

1. 首次实现提交；
2. 后续修复提交；
3. 配套的后端、前端、路由、数据结构和测试提交；
4. 最终 Overlay 中该功能的实际行为。

如果最终代码与最初功能提交不同，以最终代码为准。

### 3.2 Base 的新代码不能被盲目覆盖

Overlay 相对较早，Base 可能已经包含更近的上游修复。提交时间用于发现潜在冲突，但不是唯一判断标准。

遇到冲突时按以下顺序判断：

1. 先确认两边解决的是不是同一个业务问题；
2. 保留 Base 已有的安全修复、兼容性修复、性能修复和测试覆盖；
3. 将 Overlay 的功能行为适配到 Base 当前架构，而不是覆盖整个文件；
4. 如果 Base 和 Overlay 的行为目标不同，以用户确认的目标行为为准；
5. 如果无法判断，保留 Base 行为并记录为待确认项，不直接采用旧 Overlay 逻辑。

### 3.3 迁移粒度

- 每个业务功能单独迁移；
- 每个业务功能单独提交；
- 提交信息描述实际实现内容，不使用 `C01`、`C05` 等内部编号；
- 同一功能的前后端、测试和必要文档必须在同一个功能提交中保持完整；
- 不把无关品牌、供应商、PLG 或支付改动混入功能提交。

### 3.4 当前明确范围

已确认需要保留的历史功能编号为 `01`、`05`、`06`、`07`、`11`。编号只用于历史讨论，后续提交和文档使用实际功能名称。

当前明确不做：

- 插件化改造；
- TokenDazi 品牌和站点定制；
- Overlay 专属供应商业务；
- PLG 独立分组行为；
- 未经确认的支付、订阅和营销功能。

## 4. 当前已迁移功能

以下功能已经进入目标分支 `tokendazi`。后续差异分析仍需用 Overlay 最终代码做一次行为复核，但不应重复迁移。

### 4.1 上游无有效输出时不扣费

目标提交：

- `8b6a473c1 fix: refund local estimates when upstream sends no output`

目标行为：

- 本地预估扣费后，如果上游没有返回可计费的有效内容，退回本地预扣额度；
- 避免“请求没有实际产出但仍然扣费”；
- 保留 Base 当前计费、日志和错误处理结构。

来源侧需要关联复核的提交包括：

- `4a7eff228 fix(billing): prevent over-charging when stream ends without upstream usage`

复核重点不是提交是否同名，而是以下行为是否一致：正常有 usage、空输出、流中断、上游错误、重试和异步任务分别如何结算。

### 4.2 Workspace 用户工作台

目标提交：

- `00cc550e3 feat: add workspace dashboard as the user home`

已包含：

- Workspace 页面；
- 用户首页跳转；
- 用量摘要；
- API 访问入口；
- API Key 操作入口；
- 相关路由和前端测试。

来源侧需要整体参考的提交链包括：

- `c5df99adc feat(default): add user workspace dashboard`
- `51ec1ab5b feat(default): align workspace with Token Dazi brand`
- `e063f26f0 fix(workspace): align API access header spacing`
- `2b0c0acd5 fix(workspace): remove auto-append /v1 from API address display`
- `f613d7c62 fix(workspace): API 密钥按钮使用 navigate 替代 window.location`
- `5471fccbc fix(workspace): fix docs link field name and rename button to 使用文档`
- `803f84b23 style(workspace): polish API Access card selects and credential area`

品牌文字、站点链接和品牌专属样式不自动迁移；功能性修复需要与 Base 当前路由和 API 结构重新核对。

### 4.3 Available Models 用户可用模型目录

目标提交链：

- `c67001ab1 feat: add user available model catalog with pricing and health`
- `e960a7e2a fix: align model access with base user groups`
- `80e26c399 fix: expose workspace and available model navigation`
- `bda1f2549 fix: show public model prices without base ratios`
- `17ce86b80 fix: align model pricing panel with overlay layout`
- `ad936dabc fix: keep selected access group focus ring visible`
- `50f69230e fix: align available models page with overlay layout`

已包含：

- 用户可用分组和模型列表；
- 可用/不可用模型状态；
- 模型搜索；
- 供应商筛选；
- 支持协议展示；
- 公共价格展示；
- 模型健康状态；
- 桌面端和移动端布局；
- 与 Overlay 最终页面一致的外层容器、分组栏、模型卡片和价格区域。

当前仍属于数据配置问题而不是页面功能缺失的内容：

- `/models/metadata` 没有匹配模型时会显示 `Unlabelled vendor`；
- 供应商没有有效 `icon` 时会显示首字母降级图标；
- 页面使用 Base 公共价格，不代表已经迁移 Overlay 的用户专属倍率逻辑。

## 5. Base 已有但需要避免重复迁移的功能

目标分支较早已经包含以下一组后端能力：

- `2ab0cba26 feat: add channel concurrency, model health, admin logs, metrics, and error handling`
- `5dfc19e2e fix: update channel fixture for concurrency limit`

涉及渠道并发、模型健康、性能指标、管理员日志和错误信息处理。Overlay 的对应实现仍需用最终代码核对，但不能因为 Overlay 中存在同类提交就整块覆盖 Base。

复核时重点检查：

- Base 更新后的渠道选择和并发租约逻辑；
- Base 的错误脱敏和请求 ID 行为；
- Prometheus 鉴权和指标边界；
- 模型健康数据的统计窗口和失败语义；
- 既有测试是否覆盖当前实现。

## 6. `/models/metadata` 与模型目录数据

`/models/metadata` 是 Base 的模型目录元数据配置，不是渠道路由配置。

### 6.1 字段作用

| 字段 | 作用 |
| --- | --- |
| 模型名称 | 被匹配的模型名或匹配样本 |
| 匹配方式 | 精确、前缀、包含、后缀匹配 |
| 描述 | 定价页模型卡片和详情信息 |
| 模型图标 | 定价页模型图标；优先于供应商图标 |
| 标签 | 定价页搜索和标签筛选 |
| 供应商 | 定价页和 Available Models 的供应商归属 |
| 供应商图标 | Available Models 当前模型卡片使用的图标 |
| Endpoints | 模型支持的协议、API 示例和公开模型能力信息 |
| Status | 模型目录/定价页是否展示 |

匹配优先级为：

```text
精确匹配 > 前缀匹配 > 后缀匹配 > 包含匹配
```

因此不需要每个模型建立一条精确记录，但希望每个模型拥有正确供应商和图标时，必须确保每个实际模型能够被某条规则覆盖。

### 6.2 不应误解的影响

元数据配置不会直接：

- 创建渠道；
- 创建模型访问权限；
- 改变转发路由；
- 修改用户分组；
- 修改模型倍率。

删除元数据时，如果额外勾选“从渠道移除”或“同时移除定价”，才会进一步修改渠道模型列表或价格配置。

## 7. Overlay 差异功能分类

### 7.1 已迁移或正在复核

| 功能 | Overlay 代表提交 | 目标状态 |
| --- | --- | --- |
| 无有效上游输出时退款 | `4a7eff228` | 已迁移，需完整行为复核 |
| Workspace | `c5df99adc`、`f6f9f1160` 相关链 | 已迁移 |
| Available Models | `f6f9f1160`、`92f85864f` 及后续修复 | 已迁移 |
| 模型健康和性能基础能力 | 上游模型健康相关提交 | Base 已有，需差异复核 |
| 渠道并发限制 | Overlay/上游渠道并发相关提交 | Base 已有，需差异复核 |

### 7.2 待按功能继续分析

以下内容不能直接判断为需要迁移，需先和用户确认业务价值，再逐项分析：

- 管理员日志增加请求体大小、User-Agent、缓存命中率等字段；
- 模型可用性探测的手动触发；
- 渠道按分组覆盖优先级和权重；
- Overlay 对模型元数据和供应商目录的批量管理增强；
- Overlay 后续的账户、Wallet、充值和订阅流程调整；
- Overlay 的额外审计和运营报表功能。

### 7.3 明确暂不迁移

- TokenDazi/Flatkey 品牌替换、站点文案和专属域名；
- Overlay 专属供应商接入和供应商业务流程；
- PLG Token 可使用独立允许分组的逻辑；
- PLG 定价分组与充值分组拆分；
- Overlay 专属支付、订阅、充值码和营销流程；
- 将现有能力改造成插件的工作。

## 8. 分组逻辑处理原则

当前 Base 将用户分组和渠道分组放在同一套分组逻辑中。Overlay 的部分代码试图引入：

- 定价分组；
- 充值分组比例对应的用户分组；
- 根据可见性配置额外可见分组；
- PLG Token 独立允许分组。

这不是简单的前端迁移，涉及用户权限、Token 分组、渠道能力、价格计算和 Available Models 返回结果。

在用户明确决定前，维持 Base 分组逻辑，不迁移 Overlay 的 PLG 分组行为。后续如需要迁移，应作为独立的后端功能设计、数据库行为分析和兼容性迁移，不能混入模型目录功能。

## 9. 冲突处理流程

每个待迁移功能执行以下流程：

1. 从 Overlay 最终分支确认用户可见行为和后端契约；
2. 列出该功能的完整提交链，不只取首个功能提交；
3. 对照 Base 当前代码和近期上游提交；
4. 按文件和业务行为拆分冲突；
5. 保留 Base 更新且更完整的实现；
6. 只移植 Overlay 中 Base 缺失的业务行为；
7. 补齐 Base 风格的测试和错误处理；
8. 运行受影响的后端测试、前端测试和必要的 Playwright 验证；
9. 使用功能名称创建独立提交；
10. 在本文档更新迁移状态和验证结果。

提交时间只用于定位潜在的“新旧实现”关系，不能单独作为取舍依据。最终以行为、测试和当前架构兼容性为准。

## 10. 验证要求

### 后端功能

- 运行受影响包的 `go test`；
- 运行涉及路由、服务和模型访问的相关测试；
- 如涉及 SQLite、MySQL 或 PostgreSQL 语义，验证对应数据库行为；
- 验证错误、重试、退款、权限和缓存刷新路径。

### 前端功能

- 运行受影响 feature 的 Vitest 测试；
- 运行 `bun run typecheck`；
- 运行涉及文件的 `bunx oxlint`；
- 涉及 UI 时用 Playwright 验证桌面端和移动端；
- 修改前端后才执行生产构建；不改前端时不重复构建；
- Bun 命令按资源可控方式执行，避免再次造成 VPS 终端无响应。

### 提交前

- 工作区只包含当前功能变更；
- 提交信息描述实际功能，不使用历史代号；
- 测试结果记录到迁移记录；
- 不把无关上游差异和品牌差异带入提交。

## 11. 下一步实施顺序

1. 完成 Overlay 最终代码与目标分支的功能差异矩阵；
2. 对已迁移的计费、Workspace 和 Available Models 做一次最终提交链复核；
3. 选择第一个尚未迁移且属于用户确认范围的功能；
4. 完成该功能的后端/前端/测试迁移并单独提交；
5. 再进入下一个功能，避免并行修改多个业务域。

在没有完成差异矩阵前，不进行大范围代码覆盖或整目录复制。

## 12. 已完成的实际代码差异核对

本节记录已经对 Overlay 最终代码和目标分支逐文件核对后的结果，避免只根据提交标题判断迁移状态。

### 12.1 计费退款：目标代码较新，不能覆盖

Overlay 相关文件：

- `relay/channel/openai/relay-openai.go`
- `relay/channel/claude/relay-claude.go`
- `service/text_quota.go`

目标分支也存在上述文件，并已有：

- 本地估算用量归零保护；
- 无真实输出时退款；
- 工具调用附加费用；
- 更完整的流状态、计费表达式和用量语义处理。

逐文件比较后发现，目标分支的 `service/text_quota.go` 明显包含比 Overlay 更新的 Base 计费能力。结论是：

- 不能复制 Overlay 的整个 `text_quota.go`；
- 只确认无有效输出时不扣费这一行为是否被目标实现覆盖；
- Base 新增的工具计费、动态计费和流状态处理必须保留；
- 后续验证应以计费行为测试为主，而不是以文件差异大小为依据。

### 12.2 模型访问：目标代码已做 Base 分组适配

Overlay 与目标分支均存在：

- `model/model_access.go`
- `service/model_access.go`
- `controller/model_access.go`
- `web` 下的 Available Models 页面。

目标分支的 `model/model_access.go` 与 Overlay 最终实现已经基本一致；`service/model_access.go` 则存在较大差异，目标版本保留了 Base 的用户分组语义。

结论是：

- 不应把 Overlay 的 `service/model_access.go` 直接覆盖到 Base；
- 目标分支当前的用户分组适配是有意修改；
- Available Models 的页面结构和接口已经完成迁移；
- Overlay 的专属分组、PLG 分组行为暂不回灌。

### 12.3 模型健康与可用性：当前迁移并不完整

这是本次实际比对发现的主要缺口。

目标分支当前使用：

- `model/perf_metric.go`
- `controller/model_health.go`
- `service/model_access.go`

目标实现通过最近 24 小时的真实请求统计推导状态：

- 有成功请求：`available`；
- 只有失败请求：`temporary_failure`；
- 没有统计数据：`unknown`。

Overlay 最终代码额外包含：

- `model/model_availability.go`
- `controller/model_availability_task.go`
- `controller/model_availability_task_test.go`
- 启动时调用 `StartModelAvailabilityDetectionTask`；
- 定时主动探测启用渠道中的模型；
- 文本、图片、Embedding、重排等不同协议的探测分类；
- 对“官方不支持模型”和“临时上游故障”进行区分；
- 将检测结果持久化到 `ModelAvailabilityState`；
- 管理员手动触发探测的 API 和模型健康页面按钮。

该功能的实际提交链不是单个提交，至少包括：

- `ce7d8cde4`（2026-06-09）：首次检测官方不支持模型；
- `a147d87e7`（2026-07-06）：图片模型改用图片端点探测；
- `e57904fdb`（2026-07-18）：探测修复及临时渠道处理；
- `59474258d`（2026-07-18）：按模型模态区分探测方式；
- `e9594d5eb`（2026-07-23）：模型健康监控页面；
- `e92757d2f`（2026-07-28）：手动触发探测。

因此，不能只迁移最后的手动触发按钮。必须以最终探测任务、状态模型、路由、数据库迁移、健康页面和测试共同组成一个完整功能。

目标分支目前没有 `model/model_availability.go` 和 `controller/model_availability_task.go`，也没有对应的定时探测任务和手动触发接口。

另外，Overlay 通过独立的定时 Goroutine 启动探测；目标 Base 已经有基于数据库租约的 `system task runner`。即使决定迁移该功能，也不能直接复制 Overlay 的启动代码，应将探测逻辑适配为 Base 的系统任务，避免多节点重复探测。

因此，当前 Available Models 页面虽然已经具备“显示健康状态”的前后端结构，但状态来源仍是被动请求统计，并未达到 Overlay 的主动可用性探测行为。这项功能不能标记为完全对齐，后续应作为独立功能迁移，并重点审查其对真实渠道的调用成本、测试用户权限和误判风险。

### 12.4 管理员日志诊断：后端部分已存在，前端部分缺失

Overlay 提交链：

- `26b5e8e19 feat(logs): record user-agent in admin_info for request tracing`
- `7f528ecbb feat(logs): add request body size and cache hit rate for admins`

目标分支后端 `service/log_info_generate.go` 已经写入：

- `admin_info.user_agent`；
- `admin_info.request_body_size`。

目标前端 `web/src/features/usage-logs/components/dialogs/details-dialog.tsx` 当前已经显示 User Agent，但没有 Overlay 最终版中的：

- Request Body Size；
- Cache Hit Rate。

Overlay 的 `7f528ecbb` 提交明确把这两个字段限制为管理员可见；迁移时必须保留 `admin_info` 的脱敏边界，不能把请求体大小或缓存信息暴露给普通用户。

同时，目标分支的日志结构已经包含其他较新的 Base 字段，例如配额饱和、插件和根节点信息。因此不能直接覆盖整个日志详情组件，应只迁移缺失的两个展示字段，并补充类型、翻译和行为测试。

### 12.5 按分组覆盖渠道优先级/权重：目标分支缺失

Overlay 提交链：

- `493bf4c85 feat(channel): support per-group priority and weight override`
- `21e8ceadf fix(channel): fix group priority override UI bugs`
- `d73177374 refactor(channels): inline group priority UI and add list column tags`

Overlay 通过渠道上的 `group_priority` JSON 保存每个分组的优先级和权重覆盖，并在以下环节读取：

- `model/channel.go`；
- `model/ability.go`；
- `model/channel_cache.go`；
- `service/channel_select.go`；
- 渠道编辑抽屉和渠道列表。

目标分支目前只有渠道级别的 `priority` 和 `weight`，没有 `group_priority` 字段，也没有按分组覆盖的选择逻辑。

这不是简单的前端增强，涉及数据库字段、Ability 重建、缓存、渠道选择和迁移兼容。它当前不属于已确认的模型目录迁移，应单独评估是否保留。

### 12.6 Overlay 代码与 Base 代码的路径差异

Overlay 当前前端主要位于 `web/default/src`，目标 Base 前端位于 `web/src`。因此：

- 不能把 Overlay 前端文件路径直接复制到 Base；
- 需要按目标路由、组件注册、i18n 和数据请求封装适配；
- Overlay 的品牌组件和 Base 的组件体系不能混用；
- 前端同名文件也必须按最终行为比较，不能只看文件名。

### 12.7 最终代码树的实测差异

为避免把“Overlay 相对二开 main 的增量”误当成完整 Overlay，本次使用两个最终提交的 Git tree 做了路径集合和 blob 内容比较：

| 项目 | 提交 | 受跟踪文件数 |
| --- | --- | ---: |
| 目标 Base `tokendazi` | `50f69230e` | 2,610 |
| Overlay 最终树 | `8f8390e4b` | 3,631 |

结果如下：

- 两边同路径文件：607 个；
- 同路径且内容完全相同：154 个；
- 同路径但内容不同：453 个；
- 仅目标 Base 存在：2,003 个；
- 仅 Overlay 存在：3,024 个。

这组数字说明不能用一次 `git diff origin/main..origin/tokendazi-overlay` 作为完整迁移清单。Overlay 的 72 个提交只描述 `dev-new-api` 的 `origin/main` 之后新增的部分；Overlay 最终树中大量功能来自它的二开基线，尤其是主动模型探测、模型健康页面、Prometheus 导出器和渠道并发的早期实现。

两边的文件集合差异还受到架构迁移影响：

- Overlay 前端主要在 `web/default/src`，Base 前端在 `web/src`；
- Base 新增了 `relaykit/` 和官方 `plugins/`，Overlay 没有对应目录；
- Overlay 额外包含 `website/`、`website-static/`、品牌资源和大量钱包/订阅/营销代码；
- 同路径的后端文件也存在 Base 后续更新，例如 `relay.go`、`service/channel_select.go`、`model/channel.go`、`service/text_quota.go` 和 `controller/relay_sanitize.go`。

所以“文件不存在”只表示需要确认该行为是否缺失，不表示可以直接复制；“同路径不同”也只表示需要逐段审查，不表示 Overlay 一定更新。

### 12.8 五项保留功能的逐项内容结论

| 功能 | Overlay 最终实现 | Base `tokendazi` 实现 | 内容级结论 |
| --- | --- | --- | --- |
| 渠道并发控制 | `service/channel_concurrency.go`、`model/channel.go`、`model/channel_cache.go`、`service/channel_select.go`、`controller/relay.go`、渠道任务路径 | 已有上述核心服务，并接入 Base 的请求策略、任务插件和新 relay 生命周期 | 核心租约能力已存在；`channel_select.go` 等必须保留 Base 更新，不能覆盖；Overlay 的 `group_priority` 另列为未确认功能 |
| 模型可用性/健康监控 | `model/model_availability.go`、`controller/model_availability_task.go`、`controller/channel-test.go`、`controller/model_health.go`、`pkg/perf_metrics/*`、`web/default/src/features/model-health` | 已有 `PerfMetric` 被动健康统计、`controller/model_health.go` 和 Available Models 的状态字段 | 被动统计已存在；持久化主动探测、模态分类、官方不支持状态、定时任务、手动触发和管理页仍缺失 |
| 管理员日志增强 | `service/log_info_generate.go`、`controller/relay.go`、日志详情组件和类型 | 后端已有 `admin_info.user_agent`、`admin_info.request_body_size`；前端保留了 Base 的复杂日志详情 | 后端行为已对齐且 Base 增加了 UA 清洗；前端仍缺请求体大小和缓存命中率展示，不能覆盖整个详情组件 |
| 渠道健康 Prometheus 指标 | `pkg/perf_metrics/prometheus.go`、`pkg/perf_metrics/*`、`middleware/prometheus_metrics_auth.go`、`router/prometheus_metrics.go`、relay attempt 埋点 | 已拆分为 Base 的渠道指标导出器、模型指标导出器、鉴权中间件和路由 | 指标能力已迁移到 Base 架构；需要以 Base 当前低基数、取消请求和插件任务边界做回归，不应复制 Overlay 的旧导出器 |
| 上游错误脱敏与同优先级重试 | `controller/relay_sanitize.go`、`controller/relay.go`、`service/channel_select.go` | 已适配 `relaykit/types`、任务错误和 Base `RequestPolicy` | 目标实现不是旧代码原样复制；应保留 Base 的错误码、任务错误和诊断链，补充模型官方不支持状态后再复核脱敏分支 |

### 12.9 Overlay 最终树中已确认不属于当前范围的功能

完整树比对还确认了以下功能确实存在于 Overlay 最终代码，但不能因为它们出现在最终树中就自动迁移：

- `model/channel.go`、`model/ability.go`、`model/channel_cache.go` 和渠道前端中的 `group_priority` 分组优先级/权重覆盖；
- `86b320615` 及其后续代码中的 PLG Token 独立可用分组；
- `ea2240ed8` 至 `846e4e64c` 的内容安全审计后端、管理页面和运营开关；
- `e4df48090` 至 `9c16cc364` 的钱包充值、兑换码、支付和订阅流程；
- Codex、BytePlus、Kuaizi 等供应商或供应商专属链路；
- TokenDazi/Flatkey 品牌、官网、静态站、营销和部署文件。

这些功能分别属于分组模型、内容安全、支付账户、供应商适配和产品品牌，不与五项已确认的通用网关功能合并开发。

### 12.10 主动模型探测的 Base 适配边界

Overlay 的 `controller/model_availability_task.go` 直接使用单例 Goroutine 和 `common.IsMasterNode`。Base 当前已有 `model/system_task.go`、`service/system_task.go` 和 `controller/system_task.go` 的数据库租约任务框架，因此迁移时必须拆成：

1. `ModelAvailabilityState` 数据模型和兼容迁移；
2. 复用 `controller/channel-test.go` 的渠道测试构造、超时和错误边界；
3. 把探测作为带租约的系统任务，避免多节点重复调用上游；
4. 保留文本、图片、Embedding、Rerank 与不可同步探测模态的区别；
5. 将探测请求标记为系统行为，不能进入普通用户计费、用量和渠道成功率；
6. 最后接入管理员手动触发接口和管理页面。

Overlay 的“官方不支持”状态还会在 Relay 前置阶段阻断请求。该行为必须在状态模型和探测误判保护完成后再启用，不能只添加一个枚举或直接复制 `controller/relay.go` 的判断。

## 13. 代码比对后的下一步

基于上述实际结果，下一步优先级调整为：

1. 先迁移主动模型可用性探测的后端状态模型、测试和系统任务适配；
2. 再接入探测 API、官方不支持保护和管理页面手动触发；
3. 单独补齐管理员日志详情中的请求体大小和缓存命中率展示；
4. 对计费、Workspace、Available Models、渠道并发、Prometheus 和错误脱敏做最终行为回归；
5. 暂不迁移按分组覆盖渠道优先级/权重、PLG 独立分组、支付订阅和供应商代码；
6. 每项功能完成后单独提交，提交信息写实际功能名称，不使用历史编号。
