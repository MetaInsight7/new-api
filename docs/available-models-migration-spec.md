# Available Models 页面迁移规格

## 1. 目标

将 `/root/newapi/dev-new-api` 中 `origin/tokendazi-overlay` 的最终
`/available-models` 页面完整迁移到 `/root/newapi/new-api` 的
`tokendazi` 分支。

本规格的对比基线是：

| 角色 | 基线 |
| --- | --- |
| Base | `new-api/main@56758edf9` |
| Overlay | `dev-new-api/origin/tokendazi-overlay@8f8390e4b` |
| 实施分支 | `new-api/tokendazi` |

`tokendazi` 现有的 Available Models 代码不是可信基线，只用于识别已经迁移的可用性探测、阻断和模型健康实现。页面相关代码需要重新按本规格校正。

## 2. 已确认决策

### 2.1 API 范围

- 页面使用的接口是 `GET /api/user/model-access`。
- `/v1/available_models` 不是页面调用链的一部分，本次不迁移、不新增调用方。
- 如果未来确认有外部客户端依赖 `/v1/available_models`，另开独立功能评估。

### 2.2 分组语义

保留 Base 的用户分组逻辑：

- 用户自身分组；
- Base 的可见分组配置；
- Base 的 Auto 分组；
- 分组下启用 Ability 和启用渠道对应的模型。

不迁移 Overlay 的：

- `fixed_account` 独立账户模式；
- `plg` 固定账户模式；
- PLG Token 独立允许分组；
- 定价分组与充值分组拆分。

Overlay 的页面能力需要适配到 Base 的分组模型，而不是覆盖 Base 的分组行为。

### 2.3 模型可用性

迁移 Overlay 的完整可用性功能，而不是只迁移页面状态标签：

- 持久化模型可用性状态；
- 主动探测；
- 按模型汇总多个渠道结果；
- 临时错误、未知错误和官方不支持的分类；
- 连续失败阈值和状态转换；
- 已确认不支持模型在计费前阻断；
- 非普通 Relay 接口的探测适配；
- 系统任务、手动触发、限流和取消处理；
- Available Models 页面状态展示。

`tokendazi` 中已有的探测与阻断实现作为实现参考，并保留其已完成的测试和修复；本次只修正它与页面及 Base 当前架构的衔接。

### 2.4 价格与倍率

页面保留 Overlay 最终行为：

- 使用 Base 现有公共价格接口和价格格式化能力；
- 价格显示在模型卡片右侧；
- 支持请求计费、输入价格和输出价格；
- 加载中、没有价格和不适用计费类型有明确降级状态；
- 支持模型专属倍率；
- 没有配置专属倍率时不显示专属倍率 Badge；
- 有专属倍率时，专属倍率覆盖分组默认倍率，不与默认倍率相乘。

模型专属倍率需要迁移 Overlay 的 `GroupModelRatio` 配置和实际计费语义，不能只在前端增加展示字段。

### 2.5 API Key 创建

页面保留 Overlay 的创建闭环：

- 普通入口直接打开 API Key 创建抽屉；
- 从某个分组进入时携带分组参数；
- 创建抽屉打开后预选该分组；
- 无效或不可见分组回退到 Base 的默认分组规则；
- 关闭或消费参数后清理 URL，刷新页面不会重复打开抽屉。

Base 当前 API Key 页面使用 Drawer，不强行引入 Overlay 其他页面结构，只扩展现有 Drawer 的受控打开和初始分组能力。

### 2.6 品牌与文档入口

Overlay 品牌即 TokenDazi，本次保留：

- 页面标题右侧的操作文档入口；
- Overlay 的 TokenDazi 官网/文档来源配置；
- `VITE_OFFICIAL_DOCUMENTATION_URL` 环境变量覆盖能力；
- 新窗口打开时的 `noopener` 和 `noreferrer`。

不得复制 Overlay 中与其他品牌、供应商或营销页面相关的内容。

## 3. Base 与 Overlay 的能力差异

### 3.1 Base 已有、应复用

- `service/group.go` 的用户可用分组和 Auto 分组解析；
- `model.Ability` 与渠道启用状态查询；
- `model.GetPricing()`、`GET /api/pricing` 和前端价格格式化器；
- `model/model_meta.go` 的精确、前缀、后缀、包含匹配；
- Vendor 元数据和模型图标管理；
- 现有 API Key Drawer、分组选择器和 Auto 分组选择器；
- Base 当前路由、认证、缓存和错误处理约定。

### 3.2 Base `main` 缺失、需要补齐

- `/api/user/model-access` 聚合契约；
- Available Models 页面路由和完整组件；
- 页面对应的模型权限 DTO；
- 页面专属倍率 DTO 和 `GroupModelRatio` 计费衔接；
- 页面到 API Key Drawer 的深链接；
- 操作文档入口；
- 页面和接口的完整测试。

### 3.3 `tokendazi` 已有、直接作为参考

- `model/model_availability.go`；
- `controller/model_availability_task.go`；
- 模型健康任务、系统任务注册和手动触发；
- 已确认不支持模型的 Relay 前置阻断；
- 可用性错误分类、状态转换、限流和取消修复。

这些代码不因为页面重做而重复实现；需要先确认它们在 `tokendazi` 当前 HEAD 上的测试结果，再通过页面接口接入。

## 4. 页面功能清单

### 4.1 页面框架

- 桌面端左侧分组导航；
- 移动端分组选择；
- Auto 分组模型汇总；
- 空数据、首次加载、请求失败、已有数据刷新失败和重试状态；
- 外层大容器、响应式布局和独立滚动区域；
- 页面文档入口。

### 4.2 模型浏览

- 按当前分组展示可用模型；
- 模型搜索；
- 供应商筛选；
- 协议类型 Badge；
- 模型复制；
- 每页 50 个模型并支持继续加载；
- 无结果和筛选清空操作；
- 模型供应商图标和名称。

### 4.3 价格、倍率和状态

- 公共输入/输出/请求价格；
- 专属倍率仅在实际配置时展示；
- 可用、临时失败、未知失败状态；
- 官方不支持模型明确标记为不可调用；
- 不可用模型单独区域；
- 供应商未配置时使用非持久化的默认推断展示，不能在读取页面时写入数据库。

### 4.4 API Key 操作

- 当前分组创建 API Key；
- 分组预选；
- Drawer 参数消费和 URL 清理；
- 保留 Base 现有 API Key 表格、表单校验、Auto 分组和安全逻辑。

## 5. 实施顺序与提交拆分

每个提交描述真实功能名称，不使用内部编号。

1. **对齐模型可用性探测与按模型阻断**
   - 复核并保留 `tokendazi` 当前实现；
   - 完成 Base 当前架构下的后端测试；
   - 不包含 Available Models 页面 UI。
2. **新增用户可用模型访问聚合接口**
   - 添加 `/api/user/model-access`；
   - 使用 Base 分组语义；
   - 返回协议、Vendor、可用性状态和模型集合；
   - 不返回渠道名称、密钥等敏感字段。
3. **支持分组模型专属倍率**
   - 迁移 `GroupModelRatio` 配置与计费优先级；
   - 页面只展示实际配置的专属倍率；
   - 补齐计费回归测试。
4. **实现 Available Models 页面模型浏览**
   - 迁移 Overlay 最终 UI；
   - 适配 Base 的页面布局、价格接口和组件；
   - 包含状态、筛选、协议、复制、分页和价格卡片。
5. **支持从模型分组直接创建 API Key**
   - 扩展现有 Drawer；
   - 增加 URL 参数、分组预选和参数消费测试。
6. **增加 Available Models 文档入口和导航**
   - 保留 TokenDazi 品牌来源；
   - 只迁移页面所需导航，不复制 Overlay 整套侧栏。
7. **补齐页面、接口和端到端验证**
   - 后端 API/服务测试；
   - 前端组件和路由测试；
   - Playwright 验证价格、倍率、状态、筛选和 API Key Drawer。

如果第 1 项在 `tokendazi` 当前 HEAD 已经完整且测试充分，则不重复提交同一逻辑，只补缺失测试或页面依赖，并在迁移记录中注明复用关系。

## 6. 明确不做

- 不比较或覆盖 `tokendazi` 现有 Available Models 页面实现；
- 不迁移 `/v1/available_models` 页面外部 API；
- 不迁移 PLG 独立分组、固定账户和定价分组/充值分组拆分；
- 不迁移 Overlay 供应商业务和品牌营销页面；
- 不把页面价格再复制一份到 `/api/user/model-access`；
- 不在读取页面数据时创建 Vendor 或修改数据库；
- 不做插件化改造。

## 7. 验收标准

- 普通用户只能看到 Base 分组逻辑允许的分组和模型；
- Auto 分组与 Base 现有调度语义一致；
- 模型价格显示在卡片右侧，且与 `/api/pricing` 一致；
- 未配置专属倍率时页面不出现专属倍率提示；
- 配置专属倍率时显示覆盖关系，实际计费也使用覆盖后的倍率；
- 模型状态来自真实探测结果，官方不支持模型在计费前阻断；
- 未配置模型元数据时不会出现无意义的供应商空白，且不会写数据库；
- 从页面创建 API Key 能打开现有 Drawer 并预选当前分组；
- 普通用户和管理员的导航符合确认后的权限规则；
- 受影响 Go 测试、前端 targeted tests、typecheck、lint 和 Playwright 均通过；
- 前端构建使用资源可控方式，避免 VPS 无响应。
