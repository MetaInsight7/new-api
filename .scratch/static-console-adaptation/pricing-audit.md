# Static 对接 tokendazi_op：价格／计费展示契约核查

日期：2026-10-08。范围仅价格与计费展示；这是研究报告，不是实施方案的批准或功能实现。

后续已确认边界与展示决策见 [设计讨论记录](design-decisions.md)，实施结果见 [验收记录](verification.md)。第 8 节保留研究阶段的候选问题，不代表全部仍待确认，也不代表建议已获批准。

## 0. 版本、证据口径与边界

| 标记 | 源码位置 | 固定版本 |
| --- | --- | --- |
| B：真正 Base | `/root/newapi/new-api` 的 `main` | `56758edf95ec162033a6b73b553ac789404f87c9` |
| T：目标 | `/root/newapi/new-api` 的 `tokendazi_op` | `f904f2a7171f50551e9483e54154a7838fbe55dc` |
| O：最终 Overlay | `/root/newapi/dev-new-api` 的 `origin/tokendazi-overlay` | `8f8390e4b274c1855a65a1fecb02703692d00d92` |
| S：Static | `/root/newapi/tokendazi_static` | `ada037f6be625c490e8a6b97e1606d2ad300e7c8` |

- **已复核事实**：开始及收尾前均用 `git rev-parse` 确认上述版本。O 的工作树实际是 `main`，HEAD 为 `72969097e8b3c039dd028c9078528e30b04adeae`；没有把该工作树当 Overlay。所有 O 的源码证据均来自 `git show 8f8390e4b274c1855a65a1fecb02703692d00d92:<path> | nl -ba`。下文 O 的绝对路径是仓库位置，行号是固定提交的行号，不是当前 checkout 的行号。
- **阅读门槛**：读取适用根／模块 `AGENTS.md`，完整读取 T 的 `/root/newapi/new-api/.agents/rules/billing.md:1`、`/root/newapi/new-api/pkg/billingexpr/expr.md:1`，并读取 O 固定版本的表达式说明及涉及模块规范。S 无 `AGENTS.md`。Static 的 `assets/models.js`、快照生成器、校验器、三项校验测试、已提交快照和价格页元数据均已通读。
- **边界**：不研究导航、注册归因、部署或 CLI 集成；只为判断价格 API 的可访问性追踪其路由鉴权。不移植 Overlay 的独立官网商业分组语义。不更改计费引擎、数据库、配置、价格或权限。
- **用户已确定的政策**：保留 Static 三个公开页面的设计语言；排行榜 100 倍放大及模拟趋势保留，不参与价格换算。Console available-models 的“基础价＋独立倍率标记”与 Static 的“分组折算价”是不同展示口径，不能合并成同一种价格。
- 下文明确标记“事实／直接验证／推断／建议／待确认”。合成 CLI 样例证明源码行为，不证明生产配置中已经出现该样例。

## 1. 核心结论

1. **事实：目标已有复杂计费的原材料，不是缺一个普通单价接口。** `/api/pricing` 已有原始表达式、缓存／图片／音频倍率、任务用量 schema／示例，以及多插件有效计费变体。目标 Console 已有 AST 解析、有限解释器、完整形状识别和安全降级；不用另造正则表达式提价器。证据：T `/root/newapi/new-api/model/pricing.go:18`、`/root/newapi/new-api/web/src/features/pricing/lib/billing-expr.ts:307`。
2. **事实：最严重的缺口是生效分组倍率。** T 实际扣费已经优先使用 GroupModelRatio，但公开价格响应没有该字段，也没有每个可见模型／分组的最终生效倍率。O 的 `/api/pricing` 有经过可见性过滤的 `group_model_ratio`；S 即使收到这个字段也不使用。单改 Static 的表达式展示，不能修正这类错价。证据：T `/root/newapi/new-api/setting/ratio_setting/group_ratio.go:148`、`/root/newapi/new-api/controller/pricing.go:67`；O `/root/newapi/dev-new-api/controller/pricing.go:148`；S `/root/newapi/tokendazi_static/assets/models.js:59`。
3. **直接验证：Static 已存在数值错价，不只是“动态计费没展开”。** 已提交快照本应得到输入 `$0.01875/1M`、输出 `$0.1125/1M`，现有函数得到 `$0.0190`、`$0.1130`；小额正价会显示成免费；缺失倍率也会显示成零；忽略模型专属倍率会选错“最优分组”。详见第 4、9 节。
4. **建议：最小维护面是混合方案。** 后端只补权限安全的可见分组／生效倍率展示契约；前端复用目标现有纯解析／结构化展示层，经独立 JS 产物供 Static 使用，保留 Static 的 DOM/CSS。不要复用 available-models 的价格字符串 helper 作为 Static 的完整契约。
5. **事实＋推断：完整后端展示投影有价值，但不是现成功能。** T `/api/pricing` 没有价格预览、结构化阶梯、分时选档、`billing_unit`、`fixed_price` 等展示字段。管理端预览和消费日志中的同名信息不是公开价格字段。O 有另一个受专属官网组约束的 v2 投影，但对表达式只返回类型、不展开价格，不能直接满足本任务，也不应把其组语义带过来。
6. **待确认：仍不能擅自决定 UI。** 首档文案、缓存价的呈现位置、分时时钟／提示、按次与 token 混合行、多插件范围，以及特殊表达式详情，都需要用户确认。三个公开页设计与排行榜政策不在本次重新决策范围内。

## 2. public `/api/pricing`：现有接口与真正缺失项

### 2.1 路由鉴权与响应范围

**事实**：T `/root/newapi/new-api/router/api-router.go:18` 建立 `/api` 组并挂全局 API 限流；`:38` 给 `/pricing` 挂 `HeaderNavModuleAuth("pricing")`。B 同位置、O `/root/newapi/dev-new-api/router/api-router.go:48` 使用相同模块鉴权。

T `/root/newapi/new-api/middleware/header_nav.go:17` 默认 enabled=true、requireAuth=false，但受 `HeaderNavModules` 配置控制；`:104` 的三条路径是：

- 模块 disabled：403，controller 不执行。
- requireAuth=true：执行 `UserAuth`，不是匿名公共接口。
- 否则：`TryUserAuth`，允许匿名，也可能按有效身份个性化。

T `/root/newapi/new-api/middleware/auth.go:98` 的可选鉴权只在允许的凭据身份下设置用户上下文，不能视为任何 token 都能取得用户组；异常也不必一律成功匿名降级。`header_nav.go` 的 T/O blob 均为 `70aa1869def6da2ba09581a0da5734e5418a832c`。

**事实**：T `/root/newapi/new-api/controller/pricing.go:38`：取得模型目录；有用户时读取 `user.Group`，用 `GroupGroupRatio(userGroup, targetGroup)` 覆盖普通组倍率；用 `GetUserUsableGroups(group)` 过滤模型与组倍率，返回 `data/vendors/group_ratio/usable_group/supported_endpoint/auto_groups/pricing_version`。匿名 `group=""`，没有用户专属 GroupGroupRatio。可用组来源为 T `/root/newapi/new-api/service/group.go:14`；自动组还经过可选组过滤，见 `:56`。

**推断**：同一路径不代表“一份所有访客都相同的公开价格”。公开快照必须明确匿名、无 Cookie／Authorization；不能缓存登录响应或借 root 预览绕过模块权限。S 浏览器 fetch 未显式设置 `credentials:"omit"`（`/root/newapi/tokendazi_static/assets/models.js:203`），不能仅凭没有手写 Authorization 就断言永远匿名；本次没有检查实际浏览器 Cookie 或域策略。

### 2.2 字段对照

字段事实依据：T `/root/newapi/new-api/model/pricing.go:18`、`:28`、`:347`、`:374`、`:395`、`:408`；O `/root/newapi/dev-new-api/model/pricing.go:18`、`:323`、`:350`；controller 的固定提交行号如上。

| 内容 | T 当前已有 | O `/api/pricing` | 对 Static 的含义 |
| --- | --- | --- | --- |
| 模型身份／供应商／描述／tags／端点／enable_groups | 有 | 有 | 可继续使用，不需要第二套目录 |
| `quota_type/model_ratio/model_price/completion_ratio` | 有，仍保留旧定价字段 | 有 | 表达式模式下不能把旧字段当有效表达式单价；`quota_type` 也不能代表所有表达式叶子的单位 |
| `cache_ratio/create_cache_ratio/image_ratio/audio_ratio/audio_completion_ratio` | 可选指针，显式零可保留 | 有同类字段 | 缺失不是零；不是全套绝对价格或所有入口的总费用 |
| `billing_mode/billing_expr` | 有；非空有效表达式才写入；任务 alias 有补充解析 | 有，模型级非空表达式 | 可复用解析层；不能假设缺失字段必然表示有效旧倍率模式 |
| `billing_usage_schema/billing_usage_examples` | 有，插件按模型 profile 选择 | 无此公开字段 | 目标比 Overlay 丰富；不能照旧 Overlay 显示方式裁掉任务元数据 |
| `billing_plugin_variants[]` | 有 plugin_key/name、有效 mode/expr、各自 schema/examples | 无 | 顶层表达式不是所有插件的价格；空表达式变体可能表示该插件未配置 |
| `group_ratio/usable_group/auto_groups` | 有 | 有 | `group_ratio` 只含普通组／用户组覆盖，不能代替 GroupModelRatio |
| `group_model_ratio` | **没有** | 有，过滤后的顶层 map | 目标实际结算已支持，展示契约未接上 |
| 结构化 tiers／绝对价 entries／请求规则／分时说明／有效期 | **没有** | 此接口没有 | 若选择后端投影，必须新增只读契约，不能宣称已可读取 |
| public `billing_unit/fixed_price/cache_write_mode/billing_details` | **没有** | 此接口没有 | 前两者在表达式结果／日志中；后两者在管理端描述／预览中，不等于公开字段 |
| 完整 `configured/unconfigured` 价格状态 | 没有统一字段；部分任务变体可据 mode+空 expr 判断 | 没有统一字段 | 未配置旧倍率／空模型表达式不能可靠由当前公开旧字段区分 |
| 内容修订号／快照生成时间 | 没有 | `/api/pricing` 没有内容修订时间 | 现有 `pricing_version` 是写死字符串，不是报价时间或内容 hash |

**事实**：T `/root/newapi/new-api/model/pricing.go:79` 模型价格缓存最长按一分钟刷新；`:434` 的行级版本是固定值，controller `:75` 的顶层版本也固定。O 顶层版本是 `group-model-ratio-v1`。不能拿这些值保证快照实时性或数据内容一致。

**事实**：root 管理组为 T `/root/newapi/new-api/router/api-router.go:221`；其 `:231`、`:232` 提供 convert／preview。`/root/newapi/new-api/controller/model_pricing_config.go:38` 返回 `data.effective/cache_write_mode/billing_details`，接收管理员完整草稿；`/root/newapi/new-api/model/model_pricing_config.go:224` 会读定价 maps 并验证草稿。这不是供匿名 Static 调用的公开展示／报价 API。

**事实**：O 另有 `/api/website/pricing`、`/v2`（`/root/newapi/dev-new-api/router/api-router.go:49`）。v2 的 `models[].prices` 是另一响应形状，绑定专属官网组；O `/root/newapi/dev-new-api/service/website_pricing.go:112` 对 tiered_expr 仅设 `billing_kind`，没有阶梯价格提取。该服务的旧价投影使用十进制字符串并校验非负有限数（`:182`），这一设计思路可参考，但不能把该端点、字段或商业分组语义宣称为 T 已有能力，更不能直接替换 S 当前 payload。

### 2.3 价格的真正来源与计量

**事实**：公开模型目录由启用能力、模型元数据和运行时定价 getters 构成，元数据 status!=1 的模型跳过，见 T `/root/newapi/new-api/model/pricing.go:193`、`:327`。并非从 Static 快照或 Console 表单反向决定扣费。

- 旧按次：`GetModelPrice` 命中后公开 `model_price` 和 quota_type=1；否则公开模型倍率及**已经解析硬编码／fallback 的输出倍率**（T `/root/newapi/new-api/model/pricing.go:347`；`/root/newapi/new-api/setting/ratio_setting/model_ratio.go:447`）。不用前端重新决定 CompletionRatio。
- 表达式：模式优先级为显式管理员 mode，然后内置表达式；内置表达式不会覆盖已有显式旧价格。表达式 getter 优先保存表达式，再取适用内置表达式，见 T `/root/newapi/new-api/setting/billing_setting/tiered_billing.go:52`、`:70`。预扣使用它，结算使用冻结快照，而不是页面数字，见 T `/root/newapi/new-api/relay/helper/price.go:329`、`/root/newapi/new-api/pkg/billingexpr/settle.go:28`。
- 旧 token 基础美元／百万 = `ModelRatio × 1,000,000 / QuotaPerUnit`；T `QuotaPerUnit=500000`（`/root/newapi/new-api/common/constants.go:22`），所以当前的 `model_ratio×2` **本身不是 bug**。非测试 Go 源码搜索没有找到另一个赋值。管理转换共用该单位公式，见 `/root/newapi/new-api/model/model_pricing_conversion.go:36`。
- token 表达式单次美元 = 表达式输出／1,000,000；`fixed(amount)` 内部乘百万，选中叶子的真实单位是 request，不是 token；任务 `u()` 表达式已经输出单次美元，不能整体再除百万。证据：T `/root/newapi/new-api/pkg/billingexpr/run.go:88`、`/root/newapi/new-api/pkg/billingexpr/settle.go:12`。
- `len` 是完整上下文而不是扣掉缓存后的 `p`；只按表达式实际使用的子类别变量归一化 token。图片缓存明细有效时才分离 `img_cr`，缺失不能猜比例。后端事实见 T `/root/newapi/new-api/service/tiered_settle.go:26`、`:74`、`:82`；前端模拟输入已经归一化，不应再减一次。
- 旧缓存 getters 没配置时回退 1／1.25，但 public 字段仅在配置命中时发送；缺失与显式零不同。T `/root/newapi/new-api/setting/ratio_setting/cache_ratio.go:164`、`:172`；`/root/newapi/new-api/model/pricing.go:357`。按模型名称决定旧 Claude 5m／1h 展示是另一层元信息，不能把所有 `create_cache_ratio` 都理解为两种 TTL。
- 公共基础单价不是含所有附加项的最终请求账单。旧图片数量／尺寸／质量规则、音频入口差异、工具附加费、任务 OtherRatios 和实际用量，可能改变单次费用。已有管理端只读 `LegacyBillingDetails` 专门描述部分差异，T `/root/newapi/new-api/model/model_pricing_conversion.go:51`、`:63`，但当前 public 未提供这些内容。不得为了展示调用收费请求“测一次”。

## 3. GroupModelRatio：归一化、优先级、可见性与展示口径

### 3.1 生效规则不是倍率连乘

**事实**：T `/root/newapi/new-api/setting/ratio_setting/group_ratio.go:148` 和 O `/root/newapi/dev-new-api/setting/ratio_setting/group_ratio.go:170` 的顺序一致：

```text
R(userGroup, usingGroup, billingModel)
  = GroupModelRatio[usingGroup][FormatMatchingModelName(billingModel)]，若命中
  = GroupGroupRatio[userGroup][usingGroup]，否则若命中
  = GroupRatio[usingGroup]，否则
```

这是替代优先级，不是 `GroupRatio × GroupModelRatio`。合法 0 必须按“键存在”识别，不能用 truthy／`||1`。T `/root/newapi/new-api/setting/ratio_setting/group_model_ratio_test.go:9` 已有源码测试覆盖模型覆盖、零价、用户组覆盖和普通组回退；本次未运行 Go 测试。

实际入口在 T `/root/newapi/new-api/relay/helper/price.go:43`：auto-group 重试选中的组会更新 UsingGroup，再按 `GetBillingModelName()` 解析倍率。普通 token／表达式／task 路径都受这个最终倍率影响；task 表达式路径见 `/root/newapi/new-api/relay/relay_task.go:259`、`:292`。不能把自动组候选中的“最低展示价”说成本次请求必然选择的组。

### 3.2 名称不是任意 wildcard 匹配

**事实**：T `/root/newapi/new-api/setting/ratio_setting/group_ratio.go:115` 调用 `FormatMatchingModelName` 后精确查 map。T `/root/newapi/new-api/setting/ratio_setting/model_ratio.go:727`、O `/root/newapi/dev-new-api/setting/ratio_setting/model_ratio.go:746` 处理 Gemini 2.5 thinking-budget 的指定通配键及两类 gizmo 通配键；没有任意前缀／glob／小写化匹配。

T 还先通过 `/root/newapi/new-api/relay/helper/price.go:299` 为请求选择有效 billing identity：考虑模型 modifiers、canonical 候选和基础名，再让 GroupModelRatio getter 做上面的有限归一化。`/root/newapi/new-api/relay/common/relay_info.go:837` 不把“发往上游的映射名”简单当作所有计费名。任务插件表达式另有 client／mapped model 优先级（T `/root/newapi/new-api/setting/billing_setting/tiered_billing.go:109`）。

**推断**：仅在 Static 写 `group_model_ratio[group][raw.model_name]` 并不足以完整对齐。管理前端已有有限名称 helper（T `/root/newapi/new-api/web/src/features/system-settings/models/lib/group-model-ratio.ts:45`），但它不是完整 request billing identity resolver。最稳妥的是后端输出以公开模型名为键的最终倍率，避免把多套名称规则复制到 Static。

### 3.3 可见性差异是另一项缺口

**事实**：T `/root/newapi/new-api/controller/pricing.go:14` 仅判断模型是否与 usable_group 有交集；一旦返回该模型，仍保留原 `enable_groups`，包括不属于当前 usable_group 的组名。`all` 原样通过。

O `/root/newapi/dev-new-api/controller/pricing.go:40` 则裁剪每项 enable_groups，去空／去重，并将 `all` 展开为排序后的可见组；`:84` 只暴露可见组及可见模型原名／有限归一化名对应的 GroupModelRatio 键。O 的过滤是“可见组集合 × 可见模型集合”，并非逐对验证模型在该组可调用。

**建议**：公开展示应采用更明确的“可见且启用的模型－分组对”投影，至少做到：

1. 先经同一鉴权及 usable_group 过滤，再展开 `all`、裁剪每模型组列表。
2. 每个可展示组合返回实际生效倍率；只从内部 getters 推导，不能公开完整 `GroupModelRatio`／管理员 options。
3. 不可见组合不生成价格行，未知倍率不伪造 1；合并数组／map 时不修改共享的 model pricing 缓存。
4. 匿名公开快照与用户专属响应不能共用一个全局缓存；保留原计费／权限规则，不扩张匿名可见性。

### 3.4 两种 UI 政策必须同时保留

**事实**：T `/root/newapi/new-api/web/src/features/available-models/lib/model-access-pricing.ts:35` 为动态价明确传倍率 1；旧价格也使用 `__official__:1`，所以是基础价。倍率独立标记及“不与默认倍率连乘”的解释在 `/root/newapi/new-api/web/src/features/available-models/components/model-access-list.tsx:277`。

S `/root/newapi/tokendazi_static/assets/models.js:60` 在全部模式取启用组中最低倍率，`:67` 将组倍率乘进数字，`:95` 显示各 API 密钥分组折算价。这是另一种有效口径。

**建议**：共享层输出数值／单位／分支，展示入口自己选择 `R=1` 还是该可见组的 `R`。不要给 available-models 卡片偷偷乘倍率；也不要把其基础价 helper 直接替换 Static 的分组价后仍标“最低／各组价格”。

## 4. Static 的具体错价／漏价路径

完整读取的 S 文件：`/root/newapi/tokendazi_static/assets/models.js:1`、`/root/newapi/tokendazi_static/scripts/generate-model-snapshot.mjs:1`、`/root/newapi/tokendazi_static/scripts/pricing-core.mjs:1`、`/root/newapi/tokendazi_static/tests/pricing-core.test.mjs:1`、`/root/newapi/tokendazi_static/assets/model-snapshot.js:1`、`/root/newapi/tokendazi_static/models/index.html:1`、`/root/newapi/tokendazi_static/assets/config.js:1`。

| 场景／等级 | 当前算法与证据 | 结果／判断 |
| --- | --- | --- |
| **数值错价：模型组覆盖** | S `assets/models.js:59` 只查顶层 group_ratio，`:64` 也按它选最优；T 响应还根本不发模型覆盖 | 生效倍率替代被忽略；最优组与金额都可能错误。合成样例：base=$2、普通 default=1/vip=0.1、专属 default=0.5/vip=2；Static 选 vip 报 $0.2，实际最低应 default $1，vip 应 $4 |
| **数值错价：二次格式化** | S `assets/models.js:66` 先 toFixed(3)，`:73` 又解析这个字符串并 toFixed(4) | 第四位不可恢复。已提交快照：$0.01875 → $0.0190，$0.1125 → $0.1130；$0.0004 → $0.0000。不是增加最后一位格式化就能解决 |
| **错误免费：非法／缺失金融字段** | 同 `:66` 的 `value || 0` 会把 NaN 转零；验证器只验 success、数组、名称和组数组，S `scripts/pricing-core.mjs:1` | 缺失 model_ratio 能显示 $0；group_ratio 数组和字符串金融字段也能通过校验。未配置不是免费 |
| **错误组行：不可见组** | S `assets/models.js:95`、`:117` 遍历全部 enable_groups；未知组 `:59` 默认 1，而 `calculatePrice(model, unknownGroup)` 又可能回退选另一个组 | 隐藏组名旁出现其他组的价，倍率栏与金额不一致；不应显示该组合。T 未裁剪数组使其可实际触发，生产存在性未验证 |
| **错误默认：all 通配组** | S `assets/models.js:62` 把 all 当实体组查 group_ratio；`:127` 的具体组过滤只做 includes | 未展开 all 时可能虚构倍率 1，且具体组筛选漏模型；O 裁剪会掩盖此问题，T 返回 all 时仍存在 |
| **错用旧价：插件覆盖不在顶层** | S `assets/models.js:70` 只看顶层 mode+expr；完全不看 billing_plugin_variants | 顶层 legacy＋有效插件表达式时会报旧 token／按次价，真实执行插件表达式；若顶层已经动态则只显示动态，不是所有此类情况都报错数字 |
| **错误标签／筛选：fixed 或混合叶子** | S `assets/models.js:83`、`:128` 仅按 quota_type 显示“按量／按次”和筛选 | `tier(...fixed(...))` 可能仍是 quota_type=0；混合表达式不能只有一种单位。通常价位被降为“动态计费”，但类型仍可能错 |
| **漏展示：标准阶梯／分时／缓存／模态** | S `assets/models.js:70`、`:85` 统一“动态计费”；旧模式仅算 input/output | 正常非空表达式不会被该分支随便计算出旧价，这是安全但不完整的降级。不能笼统称“所有 dynamic 数字都算错”；应补条件、首档及缓存明细 |
| **基础单价被误当总费用：旧图片／任务请求** | S `assets/models.js:71`、`:86` 把 model_price 一律写 /次，不显示数量／质量／时长或其他倍率 | 对简单按次请求是可用基础价；对按张／按用量请求不能保证一次最终账单。T `relay/helper/price.go:121`、`relay/relay_task.go:305` 仍处理相应条件／OtherRatios |
| **后端状态缺口导致误报价：空表达式／未配置旧价** | T `model/pricing.go:352` 丢弃 GetModelRatio 的配置命中 bool；`:374` 对 mode=tiered_expr 但空表达式不写 mode/expr | 旧倍率未配置时可能公开 37.5 fallback；请求侧还受 `UserSetting.AcceptUnsetRatioModel` 控制，默认不接受时会拒绝，而不是一律可按该 fallback 使用。Static 无法仅凭当前字段可靠辨别，需要只读状态投影，不应前端猜测 |
| **快照陈旧／权限撤回后继续展示** | S `assets/models.js:199` 先快照，`:206` 对所有刷新失败保留旧快照并通知 | 网络失败与 401/403／业务失败未区分；撤回公开访问后不应借旧快照继续保证可见性。应明确权限失败与普通离线快照策略，并处理静态资产撤回／失效；本报告不实施 |

**事实**：S `/root/newapi/tokendazi_static/scripts/generate-model-snapshot.mjs:10` 只做一次未带认证头的 fetch；校验后把完整 payload 写入 JS，同时替换 JSON-LD/noscript。生成器没有有效价格计算、匿名权限逐对校验、generated_at 或价格内容 hash。JSON-LD 只是模型 ItemList，不含 Offer／价格（S `scripts/pricing-core.mjs:19`），所以不能声称搜索引擎已经拥有完整金融元数据。

**事实**：已提交快照仅一个 `gpt-5.5`，`group_model_ratio={}`，没有动态表达式或插件样例；这不是覆盖复杂计费的验收数据。S 三项测试只验基础 payload、拒绝无模型名及 HTML escaping；通过不能证明金额正确。

## 5. 当前 Console 解析层的能力与边界

### 5.1 不要把执行、展示、编辑三者混同

**事实**：T `/root/newapi/new-api/web/src/features/pricing/lib/billing-expression/README.md:6` 明确分成可执行、可结构化展示、可无损编辑。`max(p*2,100)` 可以模拟费用，但不是“输入单价 2”。未知 request 上下文不能用 `{}`、0 或 false 偷偷补齐。

| 类别 | 当前 T 支持与展示策略 | 不能承诺的内容／证据 |
| --- | --- | --- |
| 普通 token 阶梯 | 完整 AST；有序三元链，条件为 p/c/len 与非负数字的 `< <= > >=`，可 `&&`；price 为变量×非负字面量，接受乘法两侧调换 | 不是任意 OR／任意非线性树都能展开。整条链任何部分不满足时不提取局部单价。T `billing-expression/display.ts:60`、`:102`、`:184` |
| 缓存／模态 | p/c/cr/cc/cc1h/img/img_cr/img_o/ai/ao 有明确字段；缺项不补零；显式零保留；同输入价且非零的 cr 不单列 | 输出基础价不能替代模态明细。T `billing-expr.ts:59`；`dynamic-price.ts:341` |
| 分时时段价 | 识别仅依赖时间的外层条件；内部仍是标准 token/fixed 阶梯；保留所有分支、原时区与补集；摘要按 now 选时间分支再取第一档 | 涉及 param/header 的条件不自动做“当前时间价”；Local 时区浏览器无法知道服务器时区。T `billing-expression/display.ts:246`、`:296`；`runtime.ts:116` |
| 分时倍率规则 | 可以解析／追踪包含时间探针的 `(条件 ? 倍率 : 1)`；基础价与规则分开 | 独立分时倍率不是基础分时阶梯；只看 summary.isTimePricing 会漏这种提示。T `billing-expr.ts:599`；`dynamic-price.ts:266` |
| 固定请求价 | 识别完整 `tier(name,fixed(amount))`，显式 fixed(0) 有效；独立 /request，image_count 因子可显示 /image | 不按 token 单位缩放；不与同叶 token 费用相加；任务／Realtime 的 fixed 被后端拒绝。T `billing-expression/display.ts:116`、`:216`；`dynamic-price.ts:287`；`pkg/billingexpr/expr.md:204` |
| token/request 混合 | 解析不同叶子不同单位；summary 取 token 与 request 的条目并标 isMixedBilling | 两种条目是条件互斥选项，不是累计附加费；不能只复用首个字符串。T `dynamic-price.ts:514` |
| request rules | 兼容层拆开规则；编译器还识别探针条件、字面量倍率、fallback=1 的轨迹；有上下文可运行，无上下文返回 missing_context | 参数未知时不假设“没命中”；普通价格摘要不含这些倍率。T `billing-expr.ts:575`、`:599`；`dynamic-price.ts:266`；`runtime.ts:534` |
| 任务 u() schema | 支持 numeric second/count/token/credit，enum 与 boolean 条件；规范链有常数＋多项单价，token 字段按 /1000000；按 schema/profile 使用描述／单位／示例 | schema 不足不是免费。任务摘要是最后档基础值及逐字段范围，不是 token 的第一档。T `billing-expression/display.ts:368`、`dynamic-price.ts:506`、`:529` |
| 更复杂的任务线性条件 | 展示专用 symbolic adapter 支持 categorical 的 ==/!=、&&/||/!、布尔条件、时间条件和线性 +/*/÷；列举分类组合，保留时间条件文本 | 数量×数量、数量分母、非线性、未知字段／条件及超过展开上限均失败降级；不是任意 raw task 表达式都能展示。T `billing-expression/task-display.ts:39`、`:111`、`:193`、`:245` |
| 多插件价格 | 每变体使用自己的 mode/expr/schema；按 field+unit 合并范围，保留 providerCount、hasUnconfiguredProviders；旧按次继承与空表达式未配置区分 | 不保证每字段最低价来自同一插件；不能把分量最低相加成可购买套餐。T `plugin-pricing.ts:62`、`dynamic-price.ts:385` |
| 特殊／不支持表达式 | 完整形状识别失败，返回空 tiers／isSpecialExpression；模拟器明确 invalid/unsupported/missing_context | 不用正则抓 p/c，不能退回残留 model_ratio。T `billing-expr.ts:307`、`dynamic-price.ts:565`、`task-price-display.ts:118` |

上述路径简写均位于 T `/root/newapi/new-api/web/src/features/pricing/lib/`；报告标记 T 绑定第 0 节固定提交。

**事实**：兼容入口最终存储的是乘法表达式，T `/root/newapi/new-api/web/src/features/pricing/lib/billing-expr.ts:638` 返回 `(base) * rules`。说明文档中的 `|||` 是规则分离的说明形式，不应给 Static 再造一个只按 `|||` 切字符串的解析器。

**事实**：任务 expression 的真正来源按执行插件 override、该插件 mapped model override、模型表达式、适用 mapped model 表达式选择；客户端不选择 billing provider。T `/root/newapi/new-api/setting/billing_setting/tiered_billing.go:109`、`/root/newapi/new-api/relay/relay_task.go:253`。公共模型级元数据是首插件视图，变体才是多插件补充；profile 选择及不兼容变体置空在 T `/root/newapi/new-api/model/pricing.go:387`、`:408`。不能拿顶层 schema 解释所有 provider。

**事实**：O 的旧 `billing-expr.ts` 仍用正则查找 tier 与变量系数，缺失统一补零（O `/root/newapi/dev-new-api/web/default/src/features/pricing/lib/billing-expr.ts:254`、`:268`）；旧 summary 永远 tiers[0]，仅展示正数，没有目标的任务／fixed／当前时间 AST 适配（O `dynamic-price.ts:133`、`:163`）。因此“复用 Overlay 前端 parser”并不是本目标的推荐方案。

### 5.2 available-models helper 只能复用其政策，不能直接当契约

**事实**：T `/root/newapi/new-api/web/src/features/available-models/lib/model-access-pricing.ts:35` 返回的仅为 requestPrice/inputPrice/outputPrice/isTimePricing；它从 dynamic.primaryEntries 取第一项字符串，不携带 tiers、条件、缓存、request rules、特殊状态、多插件范围或未配置状态。

**推断**：如果把它直接搬去 Static，会丢掉“首档为什么适用”和“这是哪个组的价”，任务 second/count/credit 也不能统一塞进 input/output。应复用底层数值结构，而不是这个定制摘要。保留其倍率 1 政策，不改变 Console 卡片。

### 5.3 可复用的纯层与耦合点

| 层／直接依赖 | 复用判断 | 源文件证据 |
| --- | --- | --- |
| types → parser → runtime → display；structure | **可以共享为纯 TS 核心**。相对 imports；type-only schema import 编译后消失；不依赖 React、HTTP、认证、store | T `/root/newapi/new-api/web/src/features/pricing/lib/billing-expression/parser.ts:19`、`runtime.ts:19`、`display.ts:19`、`structure.ts:20` |
| billing-expr 兼容层、task-expr、task-matrix-display、plugin-pricing | **可复用纯解析／任务展示**；需要把最小 schema DTO 放在共享类型边界，不复制计算 | T `/root/newapi/new-api/web/src/features/pricing/lib/billing-expr.ts:30`、`task-expr.ts:19`、`task-matrix-display.ts:19`、`plugin-pricing.ts:19` |
| condition-display | 核心 AST 条件说明可用，但当前有 `@/i18n/languages` alias；t 是注入函数，i18next 的 TOptions 仅类型依赖 | T `/root/newapi/new-api/web/src/features/pricing/lib/billing-expression/condition-display.ts:19`、`:243`；独立产物需纳入／解耦 locale helper，不引入全 Console i18n 初始化 |
| dynamic-price、price | **不是完全无环境的价格函数**：调用 currency formatter；币种／充值折算来自 Console 配置；model-helpers 依赖定价常量 | T `/root/newapi/new-api/web/src/features/pricing/lib/dynamic-price.ts:19`、`price.ts:19`、`/root/newapi/new-api/web/src/lib/currency.ts:81`、`:162` |
| currency → system-config-store | 引入 Zustand/persist 及 Console 的币种配置；不能说“复制 dynamic-price.ts 不需要依赖” | T `/root/newapi/new-api/web/src/stores/system-config-store.ts:19`；Static 原本固定 USD，没有对应 Console 配置加载链 |
| useBillingTime／React 组件 | 不应直接移植 Static；只借鉴分钟边界／visibilitychange 的刷新语义 | T `/root/newapi/new-api/web/src/features/pricing/hooks/use-billing-time.ts:19`、`:28`；保持 Static 原 DOM 与 CSS |
| visual、编辑器、API、stores | 只读官网不需要，排除在独立产物外 | T `/root/newapi/new-api/web/src/features/pricing/lib/billing-expression/visual.ts:19`；其无损编辑能力不构成本任务需求 |

**边界事实**：浏览器解释器支持有限 Expr／GJSON 子集，不是 Go 引擎替代；参数高级查询、任意对象操作、安全整数外运算、深度／长度超限明确降级，见 T `/root/newapi/new-api/web/src/features/pricing/lib/billing-expression/README.md:46`。缓存编译结果最多 128 项（`parser.ts:535`）。不以 eval/new Function 实现价格逻辑，不执行任意后端表达式代码。

## 6. 最小、可维护的复用选择

### A. 推荐讨论：后端小投影＋前端纯核心独立产物

**建议，不是已实现**：

1. 后端沿用 `/api/pricing` 的权限判定，仅补可见组合的有效倍率／明确价格状态。可以兼容 O 的 filtered `group_model_ratio`，但若只返回这个原始 map，仍需在前端维护 normalization；更建议按公开模型名返回已解析的 per-model effective group ratio。具体字段名与版本尚未确定。
2. 单一源码拥有者保持在目标项目；把纯 parser/runtime/display、request-rule 拆分和任务 schema 展示暴露为窄接口，返回数值、单位、条件、规则与降级状态，不读取 Console store。现有 dynamic-price 的摘要策略与格式化应在适配边界保留／参数化，而不是重写 AST 或到 Static 复制一份。
3. 生成一个可由 Static 加载的版本化 IIFE 或 ESM JS 产物。S 当前为直接 defer 的普通脚本（S `/root/newapi/tokendazi_static/models/index.html:18`），浏览器不能直接吃这些带类型、extensionless imports 和 `@/` alias 的 TS 源文件。需要**未来实施阶段**的 transpilation/bundling 或等价独立 JS 发布步骤；不需要把 Static 改造成 React SPA，也不需要搬整份 Console bundle。
4. 独立产物只带纯计费解析和必要 locale helper，注入 USD formatter／t／now／有效组倍率。若要 CNY／充值后价格，另行确认数据来源，不隐式引入 Console 币种／充值优惠口径。
5. Static 沿用价目表与移动卡片，只换数据适配与明细内容；snapshot 和实时 fetch 消费同一展示契约。只生成一次离线价格表不能满足实时价格变化／分时切换；需要运行时纯层或服务端带有效期的动态投影。
6. 复用现有 Go/浏览器共同样例 `pkg/billingexpr/testdata/frontend_simulation.json`，补 Static 的金额、组覆盖、权限组合、fixed/mixed/task/provider 降级案例；已存在测试不能替代 Static 自己的契约验收。

**相对工作量判断**：主要工作是后端可见组合／倍率／状态小投影、纯层边界与独立产物，以及 Static 单位／条件适配和回归，不是新建结算引擎。完整任务 UI、跨实例验收及两仓发布同步会增加范围；需要在 Spec 收敛后才估算具体成本，不能把未测量的 bundle 或人员工期当事实，也不能宣称“复制一个 helper 即完成”。

### B. 可选：后端完整只读展示投影

**建议，不是现有 API**：服务器可以从当前生效配置产生 `kind/status`、完整 tiers/entries/rules、effective groups、单位、时区／条件／有效期和来源版本；字段名只是提案。仅允许已可见模型与分组，绝不输出管理员 options 全量或另开权限绕过接口。输出价格对象不是修改保存表达式，更不是实际试调用。

**优势／推断**：多客户端复用；没有 Static TS bundle；倍率、旧定价默认值、Gemini 独立音频价格、旧图片单位、服务端 Local 时区及公开可见性更容易集中在权威侧。

**成本／事实依据**：T 引擎能编译／运行／记录 tier，但没有通用“表达式到公开价表”的 getter。需要新增保守的 Go AST 展示适配器及与既有 TS 展示形状的契约测试；仅用某组 token 样例求值不能反推非线性单价。已有管理端 `effectiveModelPricing`、`ResolveLegacyBillingDetails`、`ResolveCacheWriteMode` 可以减少旧价格元信息重复，却不能把管理员 preview 直接开放，也没有替代动态／任务阶梯展开。

O 的 v2 只减少旧倍率／按次投影的思考成本，无法省掉复杂表达式、任务变体、时段切换和一般可见组合契约的工作；不得继承它绑定的官网组语义。

**相对工作量判断**：只做旧价／倍率／状态可以复用现有 getters；完整 token/fixed/mixed/time/request/task/provider 投影还需新建 Go 展示语义、边界及与 TS 的等价样例，新增维护面明显更大。复杂 raw 表达式数量、旧定价入口差异与多节点缓存一致性尚未验证，不给出未经实施校准的具体人员工期。

**建议判断**：若“Static 永久不增加任何编译产物流程”是硬约束，或近期还要多个客户端共用价表，B 更合理；否则 A 复用现成且已有契约样例的解析能力，最小新增语义面。两条路线都先解决可见组合／生效倍率缺口；不能只给 API 多加一个猜测的首档数字。

## 7. 首档、分时与特殊表达式的可靠展示条件

### 7.1 首档不是所有表达式的最低价

**事实**：T `/root/newapi/new-api/web/src/features/pricing/lib/dynamic-price.ts:497` 对普通 token 取第一档，对 task 取最后档；分时 token 先筛当前时段再取第一档。多档 token 不是跨档求最小，首档甚至可以更贵。`/root/newapi/new-api/web/src/features/pricing/lib/billing-expression/display.ts:184` 是完整有序链识别。

**建议：仅满足以下条件才显示“首档参考价”**：

- 整个基础表达式被完整结构化识别，不是抓到某一段 p/c 系数；没有不能解释的条件／剩余价格项。
- 首档对应单位明确，展示其原条件；常见上下文阶梯标明 `len ≤ 200K` 等，不用缓存后 p 假装上下文长度。链中的默认档还受此前条件的补集约束。
- 金额按当前可见组最终 R 折算一次，缓存／模态价格跟随**同一档**；缺失项为未单列，不当零价。
- 第一档只有输入、只有输出或只有缓存时按已有项显示，未出现的项目用“—／未单列”，不拼造 p/c。
- 若有 request rules，明确“基础价，按请求条件另有倍率”；未知参数不按默认请求估价。
- “首档”可用于已识别标准摘要；只有证明它确为相关条件下最低价才能写“起／最低”。Static 原“最低”若保留，应明确是**同档、可见组之间**的最低折算价，不是所有时间／所有阶梯／所有插件最低。
- task／多插件不套 token 首档规则；使用其 schema 单位、条件表和适用范围。

### 7.2 分时提示

**建议**：区分“当前时段基础价格”与“按时段触发附加倍率”。前者可展示当前时段首档＋时区＋条件，并可展开所有时段；后者继续展示基础价并附规则提示，不静默乘入所有请求。

若选择实时当前时段摘要，需在分钟边界及页面重新可见时刷新（复用 T `use-billing-time.ts:28` 的语义，不移植 React hook）；临界时刻、客户端时钟偏差、浏览器 IANA/DST 与服务器 Local 都有不确定性。不能从快照生成时间永久冻结“当前时段”。Local 或条件不可求值时，保留完整条件表／动态提示，不猜当前档。

### 7.3 固定、混合、任务与降级

**建议**：fixed 显示独立 `/请求`，image_count 型固定单价显示 `/张` 并说明数量；混合叶子显示条件＋两种单位，不把按次费用加进 token 百万价。任务 token 的 `/1M`、credit 的 `/credit`、second 的 `/秒`、count 的本地化单位必须来自 schema；附加常数独立显示，不能并进每秒单价。

多插件显示“同单位字段范围／多种计费来源”及未配置状态，完整明细按 provider 分开；不提供用户并不存在的“选最便宜插件”承诺，也不拼成不存在的全局最低账单。示例要标明 facts 和“基础示例，不含请求附加倍率”；公开示例不证明真实提交一定以该用量结算。

特殊表达式、未知版本、不支持语法、缺少 schema／request context、未配置 provider，各自保留明确状态。**安全降级是不给数字而给原因／详情，不是 0、不沿用旧倍率、不提局部系数。** 展示原表达式／条件文本时必须 escape 为文本；Static 是 innerHTML 模板而不是 React 默认转义。只允许展示已由公开接口授权的原文，不去管理接口补齐。

## 8. 仍待用户确认的 UI 选择

这些不是已批准改动；保持现有颜色、排版、品牌、价目表／移动卡片及三个公开页设计语言。

1. **公开口径**：Static 是否明确固定匿名公开报价，还是允许登录后用户专属价？建议匿名公开，避免个人价格进入静态快照／共享缓存；不要顺便扩张公开组。
2. **摘要**：继续“全部·最优”与选组折算价，还是额外补基础价＋倍率说明？建议保留原分组折算政策，并明确“可见组、同档最优”；不得与 available-models 基础价卡片混淆。
3. **阶梯**：首档旁采用“首档／条件计费／N 档”的何种短文案？条件在行内、悬浮提示还是展开区？建议首档＋条件摘要，完整档位在现有展开区，不无条件标“起”。
4. **分时**：显示当前时段价并自动刷新，还是固定列出时段表？是否同时显示时区与更新时间？建议当前参考价＋明显分时标记＋全时段详情，但服务器 Local 要可靠降级。
5. **缓存与模态**：缓存读取、创建 5m／1h、图片缓存、音频价是否进入主表，或在扩展明细显示？建议主表维持紧凑，缓存／模态明细展开；显式免费与未单列要区分。
6. **fixed／混合／任务／多插件**：主表输入输出两栏如何放 request/image/second/token/credit 与范围？建议独立单位行或条件价入口，完整条件在明细，不强塞“输入／输出”。
7. **降级与精度**：特殊表达式允许展示原文还是仅提示查看详情？美元价格默认位数／小额精度与过期快照标记如何呈现？建议保留真实非零精度、不可显示为免费；401/403 与离线错误采用不同展示策略。

## 9. 本次实际执行的直接核查证据

没有访问凭证文件、调用管理员接口或发起收费请求；没有启动服务、执行 snapshot 生成、Bun 构建或全仓测试。以下 CLI 全部只读源文件或运行既有纯函数／小规模测试，研究脚本为 stdin，不保存脚本文件。

### 9.1 Base／目标／Overlay 的源码身份

执行 `git rev-parse` 的实际结果见第 0 节。另执行：

```sh
git -C /root/newapi/new-api rev-parse main:controller/pricing.go
git -C /root/newapi/new-api rev-parse f904f2a7171f50551e9483e54154a7838fbe55dc:controller/pricing.go
```

两者均为 `ce3939c22421829daffe10cf294ea2fb9713ef12`。`model/pricing.go` 均为 `0dfcb510612ef33a1a985bf8d90bb23bac1d6406`；`dynamic-price.ts`、`billing-expr.ts`、`billing-expression/display.ts` 也逐项核对相同 blob。

**事实**：Base 原有价格 controller／模型／纯表达式展示能力没有被 target 修改；target 新增 GroupModelRatio 结算及 available-models 定制 UI，却没有同步 `/api/pricing` 的模型组倍率输出。这不是拿 Overlay main 的差异猜出来的结论。

### 9.2 实际执行 Static 原算法

用 Node `vm` 从 S `assets/models.js` 提取实际 `normalizePayload/getRatio/selectedPricingGroup/displayUsd/calculatePrice` 源码运行，没有另抄价格公式替代被测函数。观察结果：

| 输入／检查 | 真实函数输出 |
| --- | --- |
| 模型倍率 1，普通 default=1/vip=0.1，专属 default=0.5/vip=2 | 选 vip、input=$0.2000/output=$0.8000；源代码生效规则应最低 default=$1、vip=$4 |
| model_ratio=0.0002、default=1 | input=$0.0000；正确正价为 $0.0004/1M |
| model_ratio 缺失 | input/output 都 $0.0000 |
| 请求 private 组，但只公开 default/vip 倍率 | 返回 usedGroup=default；不能把这个结果放在 private 组行 |
| enable_groups=[all] | usedGroup=all、ratio=1，没有按可见组展开 |
| 顶层旧价＋插件 tiered_expr | 返回旧 token 价，不识别插件表达式 |
| 顶层 `tier("request",fixed(0.01))` | 返回 dynamic=true，没有请求单价 |
| S 已提交快照 | 选 vip=0.1，input=$0.0190/output=$0.1130；原始乘积为 $0.01875/$0.1125 |

另直接调用 `validatePricingPayload`，`model_ratio:"not-a-number"` 且 `group_ratio:[]` 的响应仍被接受。说明校验确实不验证金融字段，而非仅凭测试数量推测。

### 9.3 实际执行目标纯解析／解释层

环境 Node `v24.21.0`。使用 `node --experimental-transform-types --input-type=module`，仅注册相对 `.ts` import 解析 hook，直接 import T 的 `billing-expr.ts`、`billing-expression/runtime.ts` 和 `task-matrix-display.ts`；没有安装依赖、写临时文件、打包产物或运行 Bun。第一次 strip-only 模式因 parser 的 TS parameter property 报 ERR_UNSUPPORTED_TYPESCRIPT_SYNTAX；改为上述 Node transform 参数后以下探针全部退出 0。该实验**不证明浏览器可以直接加载 TS**。

| 源表达式／上下文 | 实测结果 |
| --- | --- |
| `len <= 200000 ? tier("standard",p*3+c*15+cr*0.3+cc*3.75+cc1h*6) : tier("long",p*6+c*22.5)` | 完整两档；standard 带 len<=200000 及各缓存价 |
| `hour("Asia/Shanghai") < 9 ? tier("offpeak",p*2+c*8) : tier("peak",p*4+c*16)` | 全档保留条件及补集；now=`2026-10-08T00:00:00Z` 时当前档为 offpeak |
| `len <= 32000 ? tier("short",fixed(0.01)) : tier("long",p*2+c*8)` | short 为 billingUnit=request、fixedPrice=0.01；long 为 token 项 |
| `tier("base",max(p*2,100))` | parseTiersFromExpr 返回 []，没有错误抽出单价 2 |
| `(tier("base",p*5+c*25)) * (param("service_tier")=="fast" ? 2 : 1)` | 基础价与请求倍率拆开；无 request 得 missing_context；p=1000/c=100、fast 请求得原始 cost=15000、倍率 2 matched=true（token 美元费用 $0.015，尚未乘组） |
| `tier("base",p*5+c*25+cr*0)` | cacheReadPrice=0 显式保留 |
| `tier("base",u("tokens")*9.8/1000000)`，token schema | unitPrices.tokens=9.8，即 USD/1M，不误当 USD/token |
| `tier("base",u("seconds")*(u("audio")==true ? 0.4 : 0.2))`，second＋boolean schema | 展开 audio=false $0.2/s、audio=true $0.4/s 两条，而不是无脑“特殊表达式” |

### 9.4 小规模既有测试

```sh
cd /root/newapi/tokendazi_static
node --test tests/pricing-core.test.mjs
```

实测 3 passed、0 failed。未运行 package.json 中的全套 test 命令、Console Vitest、Go 测试或 Bun 构建。T 现有共享表达式测试读 Go/浏览器共同样例（`/root/newapi/new-api/web/src/features/pricing/lib/__tests__/billing-expression.test.ts:25`），动态价测试覆盖 fixed/mixed/time/zero/provider（`/root/newapi/new-api/web/src/features/pricing/__tests__/dynamic-price.test.ts:62`、`:129`、`:190`、`:584`）；这里只核对源码，不宣称这些套件本次通过。

## 10. 未验证事项与交接限制

- 未访问运行中的 `/api/pricing`、`/api/status` 或数据库；不能确认生产 HeaderNavModules、匿名可见组、GroupModelRatio 实际配置、启用插件、客户端 Cookie、价格修改时间或服务器 Local 时区。版本表固定的是源码，不证明任何在线服务运行这个版本。
- 未做浏览器视觉、手机、跨时段、DST、同源凭据／缓存、权限撤回或跨实例测试；独立 JS 产物的实际体积和构建集成尚未验证。
- 小规模 CLI 合成样例是确定性源码行为验证，不是生产账单验收；没有调用实际预扣、结算或上游接口，没有测试饱和／舍入的数据库金额。
- 没有扫描凭证文件、改配置／数据、创建分支、commit/push；报告不修改三个页面、排行榜、价格算法或结算逻辑。只有这一个报告文件被新增。
- 下一步先由用户确认第 8 节 UI／公开报价口径及 A/B 复用路线，再进入实现；不能把本报告建议当作已经存在的 price preview 字段或已批准的功能。
