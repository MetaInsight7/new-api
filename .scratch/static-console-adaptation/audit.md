# Static 官网与 Console 对接核对

核对日期：2026-10-08。本文记录实施前的现状核对与方案建议，**不是实现 Spec 或 Ticket**；审计阶段没有修改业务代码、数据库或生产配置。后续经确认的实施结果见 [验收记录](verification.md)。

后续需求已收敛，最新批准范围以 [设计讨论记录](design-decisions.md) 为准。尤其不迁移排行榜 `all`，也不把本文第 8 节的全部审计建议自动列为开发任务。

## 1. 范围与固定基准

| 对象 | 仓库 / 分支 | 固定提交 | 用途 |
| --- | --- | --- | --- |
| 真正 Base | `new-api/main` | `56758edf95ec162033a6b73b553ac789404f87c9` | 判断原生能力，不把迁移代码误认为官方能力 |
| 当前迁移目标 | `new-api/tokendazi_op` | `f904f2a7171f50551e9483e54154a7838fbe55dc` | 评估实际待适配处 |
| 三开最终版本 | `dev-new-api/origin/tokendazi-overlay` | `8f8390e4b274c1855a65a1fecb02703692d00d92` | 核对既有产品行为；不使用该仓库当前 checkout 的二开 main |
| 独立官网 | `tokendazi_static/master` | `ada037f6be625c490e8a6b97e1606d2ad300e7c8` | 保留现有静态页面与设计语言 |

覆盖官网首页、公开模型、公开排行榜，Console 公共导航/根守卫/认证父子路由/登录注册入口，公开数据接口、注册与 OAuth 归因落库、快照、Nginx/Docker/本地预览与四个客户端配置脚本。不是全仓代码审计。

引用 Overlay 文件时均指上述固定 ref，通过 `git show origin/tokendazi-overlay:<路径>` 读取；本机 `dev-new-api` 工作树的同名文件不能代表三开最终版本。

## 2. 最关键结论：线上 `/pricing` 为什么不是 Console 原页面

### 2.1 源码事实

Overlay 的 `web/default/src/routes/__root.tsx:202` 先检查 setup，再处理全局路由黑名单。黑名单从 `:227` 开始：

```text
/pricing /blog /about /rankings /cli-page
/changelog /careers /contact /features /quickstart
```

精确路径或 `前缀 + /` 命中后，`:242` 使用 `window.location.href` 跳到 **`https://tokendazi.com` 首页**，随后抛外部 redirect。

因此：

- Console `/pricing` → 官网 `/`，不是官网 `/models`。
- Console `/rankings` → 官网 `/`，不是官网 `/rankings`。
- `/pricing/<模型>` 同样先被根守卫拦到官网首页。
- 虽然 `/pricing` 子路由仍挂着 Pricing 组件，但它不是用户最终能看到的页面；不能只看子路由文件下结论。
- `/models` 不在黑名单内，仍是 Console 模型管理入口；管理员进入 `/models/metadata`，未登录先走认证守卫，普通用户没有管理权限。
- `/available-models`、`/workspace`、`/keys` 等控制台业务页面不在公开跳转范围内。

Overlay 顶部导航另走一条链：

`use-top-nav-links.ts:48` → `origins.ts:23` → 外部链接组件：

| 导航 | 实际目标 |
| --- | --- |
| Home | `https://tokendazi.com/` |
| Models | `https://tokendazi.com/models` |
| Rankings | `https://tokendazi.com/rankings` |

`VITE_OFFICIAL_WEBSITE_ORIGIN` 配置官网域名，缺省为 `https://tokendazi.com`；导航目标不随 Console 语言增加前缀。

Overlay 登录/注册的 `features/auth/auth-layout.tsx:518` 也直接把品牌、首页、模型、排行榜链接到官网。它不是依靠 Nginx 修改 Console 页面的链接。

### 2.2 线上只读验证

对实际部署 `https://console.tokendazi.com/pricing` 的验证：

| 检查 | 实际结果 |
| --- | --- |
| curl 获取响应 | HTTP 200，没有 `Location` |
| curl 跟随 HTTP 跳转 | HTTP 重定向数 0 |
| Playwright 禁用 JavaScript | 留在 Console `/pricing` |
| Playwright 启用 JavaScript，重复两次 | 最终到 `https://tokendazi.com/` |

证据：`/tmp/overlay-pricing-routing-check.json`。这与根守卫源码吻合，证明这里不是普通 Nginx 301/302 跳转。

**纠正此前判断：**之前只看了 `/pricing` 子路由，遗漏了根守卫，才会错误判断它仍显示 Console 原价格页。这是分析链路不完整，不是需要用户提供 Nginx 才能解释。

本轮已足以明确跳转机制；生产外层 Nginx/CDN 配置仍会影响 HTTPS、真实 IP、缓存及 WebSocket，但只在部署验收时需要补充核对。

### 2.3 不应一起复制的旧代码

Overlay 根守卫 `:247` 明确依赖 localStorage 认证缓存；当前 Base/目标根路由 `web/src/routes/__root.tsx:117` 已有服务端认证 bootstrap、setup 并行检查、会话变化清 Query 缓存及跨窗口同步。**只迁移官网跳转规则，不整文件覆盖这些认证机制。**

目标 `/` 已经按现有产品要求分流：未登录 → `/sign-in`，普通用户 → `/workspace`，管理员 → `/dashboard`，见 `web/src/lib/home-redirect.ts:31`。Overlay 是先统一去 `/sign-in`。建议保留目标现有分流，官网入口则独立指向 Static。

当前 `components/layout/components/top-nav.tsx:42` 已正确支持 external 链接，不需要重写组件。默认 `external: false` 会被后面的实际 link 属性覆盖，不是 bug。

### 2.4 语言与遗留入口的边界

Overlay 还有 `routes/$locale/` 的旧官网路由，并非所有语言前缀路径都会被根黑名单拦截：

- `lib/public-locale-route.ts:38` 将默认 `en` 去前缀，之后再次进入正常路由。
- 非默认合法 locale 的 `/pricing` 列表还会执行旧 Console Pricing 逻辑。
- locale `/rankings`、模型详情子路由还会跳到带 locale / 模型详情的官网路径。
- 但 Token Static 只有 `/`、`/models`、`/rankings`，没有语言版或 `/models/<模型>` 独立页面。

这些是 Overlay 历史代码遗留，不应为了“复用”全部搬回目标。正式官网导航维持无语言前缀的三个现有页面；Console 未知路径不改成全量白名单阻断，也不擅自新增 `/models` 对外兼容跳转。

## 3. 两个域名的正确职责

| 域名 / 路径 | 职责 | 建议 |
| --- | --- | --- |
| `tokendazi.com/` | 公开首页 | 保留 Static 页面 |
| `tokendazi.com/models` | 公开模型与价格 | 保留 Static 布局，适配新计费数据 |
| `tokendazi.com/rankings` | 公开排行榜 | 保留 Static 排版与已确认的展示算法 |
| `tokendazi.com/pricing` | Static 旧别名 | Nginx 已配置 301 → `/models` |
| `console.tokendazi.com/pricing` 等黑名单入口 | Console 历史公开页面入口 | 按 Overlay：前端跳官网首页 |
| `console.tokendazi.com/models/...` | 后台模型元数据/管理 | 原样保留 |
| `console.tokendazi.com/available-models` | 登录用户可用模型 | 原样保留其业务与数据权限边界 |
| Console 工作台/Key/报表/渠道管理 | 控制台业务 | 不移到 Static |
| Console 登录、注册、OAuth | 用户认证 | 继续使用当前 Base 安全机制，不在 Static 建第二套认证 |

**同一个 `/models` 路径在两个域名职责不同，这是已有设计，不是冲突。**用户手输 Console `/models`，不会自动被当成公开模型页。Go 的 `router/web-router.go:22` 是 SPA fallback，没有依据 Host 把公开页面分配给 Static 的逻辑。

## 4. 数据接口不是只改域名就能对齐

### 4.1 已有接口可以继续复用

Static 运行时调用 `/api/pricing`、`/api/rankings`，经官网 Nginx 同源代理到 Go。目标已具备两者，不需要另造公开模型聚合 API 或复制结算逻辑。

`new-api/router/api-router.go:38` 和 `:52` 使用 `HeaderNavModuleAuth`。`middleware/header_nav.go:107` 在模块禁用时返回 403、requireAuth 时要求认证。公开官网应明确启用相应模块并允许匿名访问，不能绕过这些后端权限。

当前代码中的 Console Refresh Cookie 是 host-only（`service/auth_session.go:333` 没有设置 Domain），官网请求不能被假定为“同一个已登录用户”。生产外层代理是否重写 Cookie 仍未核对。建议公开官网固定使用匿名可见模型/价格；个性化权限与可用模型仍在 `/available-models`，浏览器请求与发布快照都避免携带个人凭据。

### 4.2 排行榜有一个真实契约缺口

Static `rankings/index.html:24` 有 today / week / month / year / **all** 五个按钮。

目标 `service/rankings.go:165` 与真正 Base main 完全一致，仅支持前四个。Overlay 最终版本 `service/rankings.go:176` 另有 all：从历史起点聚合、按 30 日分桶、不计算前期增长。除此之外整份 service 与目标的主要差别仅是 Base 更现代的排序实现，应保留当前实现而非覆盖文件。

实际 GET 验证：today/week/month/year 均 HTTP 200，all 是 HTTP 400，消息 `invalid ranking period: all`。Playwright 点击“全部”复现 HTTP 400 提示，同时旧排行仍留在页面。

**建议补齐 all 支持**，复用 Overlay 的既有配置及当前 service/数据库聚合，不删官网按钮，不拿 week 数据冒充 all。

排行榜继续保留用户已确认的：

- `assets/rankings.js:2` 的 Token 展示 100 倍尺度；
- `:18` 的稳定日期权重与模拟 30 日趋势；
- 真实模型相对排序/份额、时段切换及刷新失败保留上次结果。

后端已有五分钟进程内缓存（`service/rankings.go:15`），不是即时用量流水。趋势没有直接绘制后端真实 points，页面也已有“用量指数由平台真实排序与日期稳定曲线生成”的说明。不要把它解释成真实逐日财务报表。

首屏无数据会显示空/错误状态；README 所说“模型和排行榜都有构建快照”不准确，目前只有模型快照。排行榜成功显示后的失败回退仅存在于当前页面内存。

### 4.3 价格核对

详细计费契约、源码位置、确定性样例与限制见 `pricing-audit.md`。结论不是“换一个 API 地址即可”，也不是“把整个 Console 定价模块移入 Static”。

**可直接复用的原材料：**目标 `/api/pricing` 已输出原始计费表达式、可选缓存/媒体倍率、任务 schema 和多插件计费变体；目标与真正 Base main 的纯表达式解析/展示层相同。没有公开结构化阶梯或 price preview 字段；管理端 `/api/option/model_pricing/preview` 不能开放给匿名官网代用。

**已证实的缺口：**

| 项目 | 核对结果 | 最小处理方向 |
| --- | --- | --- |
| 专属模型分组倍率 | 目标结算支持 GroupModelRatio，但 `/api/pricing` 未输出；Static 即使收到旧 Overlay 的这个字段也没使用 | 补仅限可见模型－分组组合的生效倍率展示契约，Static 折算一次 |
| 可见性 | 目标只过滤整项模型，未裁剪每项 enable_groups；Static 展开处会显示不可见组 | 复用 Overlay 的裁剪/去重/`all` 展开思路，逐组合校验；不扩张匿名权限 |
| 表达式首档 | Static 把正常非空表达式统一显示为“动态计费”，未展示首档/缓存/分时条件 | 复用已有完整 AST 识别，不用正则抓局部系数；不支持形状明确降级 |
| 插件覆盖 | Static 未看 billing_plugin_variants，顶层旧价可能掩盖实际插件表达式 | 保留有效模式和 schema；复杂或多来源价格用明确明细/状态，不假造单价 |
| 小额精度 | 原算法先 toFixed(3) 再 toFixed(4)，快照 `$0.01875` 会变成 `$0.0190`，`$0.0004` 会变成 `$0.0000` | 数值计算保留精度，只在最终显示格式化；合法 0 与未配置/缺失分开 |

GroupModelRatio 是**替代**默认组倍率，不是两者连乘；专属倍率会同时影响“全部·最优”的选组结果和金额。有限模型名归一化已有后端 getters，不应在 Static 再维护一套模糊匹配规则。建议后端按公开模型名给出已解析的有效倍率，具体字段名在 Spec 中确定；若沿用原 map，仍需补齐匹配契约。

**推荐实现接缝：后端小投影 + 前端纯解析复用。**

- Go 仅在现有公开接口权限范围内补可见组合的生效倍率及必要的“已配置/未配置”状态；不改变扣费配置、预扣/结算或后台选组规则，不公开完整管理员 options。
- 复用当前 Console parser/runtime/display 等纯层，以窄接口返回数值、单位、条件、规则、降级状态；生成一个版本化独立 JS 产物供 Static 使用，不搬 React、Zustand、认证/币种状态或整个 Console bundle。
- Static 保持当前 HTML/CSS、供应商筛选、分组价表与移动布局；运行时请求和快照使用同一解释逻辑。
- 标准 token 摘要沿用“首档”策略，主表保留输入/输出；建议缓存、完整阶梯、分时条件和特殊单位放现有展开区域，不重新设计整站。首档不一定是全表达式最低价，不能无条件写“起价”。
- 分时基础价与分时附加倍率分别提示；需要当前时段价时按时间边界刷新，服务器 Local 时区或条件未知时不猜价格。按次/fixed、混合、任务、多插件使用自己的单位与明确状态，不强行塞成两个 token 单价。

Console `/available-models` 当前的“基础价＋独立倍率标记”不改。Static 保持“选定组折算价 / 同档可见组最优价”，共享解析核心但入口使用不同倍率政策；不能把两者价格口径混成同一种。

备选是后端做完整只读价表投影，Static 只渲染。其服务器时区与多客户端复用更集中，但现有 Go 引擎没有通用的公开价表 getter，必须新增保守 AST 展示适配和等价测试，复用面更小。当前只接这一个 Static 官网，**不优先新造这套后端表达式展示器**。

## 5. 官网 → 注册 → Ops 报表的归因尚未闭环

### 5.1 目标后端已具备接收与保存能力

密码注册链路：

`features/auth/sign-up/components/sign-up-form.tsx:171` → `features/auth/api.ts:250` → `controller/user.go:297` → `sanitizeAdsAttribution` → `User.AdsAttribution`。

OAuth 链路：

`features/auth/api.ts:179` → POST `/api/oauth/state` → `controller/oauth.go:58` 清洗并存入 flow payload → `findOrCreateOAuthUser` 保存归因。不用改成把完整归因/凭据拼到 OAuth 回调 URL。

`controller/ads_attribution.go:9` 已限制 JSON 文档长度、字段白名单、字符串值及单项长度；Ops 报表 `controller/ops_report.go:616` 读取 campaign、keyword、lng、first_landing_path/landing_path、referrer、match type。现有字段足够承接这次基础官网归因，不需要先增加数据库字段。

### 5.2 真正缺口在前端交接

- 当前 Static `assets/site.js:6` 仅拼接 Console 地址，不采集 UTM/aff，不保存归因，也不把官网落地页/来源传给 Console。
- 目标 `lib/analytics/attribution.ts:20` 只读 Console 本域 localStorage；不读共享归因 Cookie。
- Overlay 最终 `lib/analytics/attribution.ts:132` 会读 `flatkey_ads_attribution`，并具有首次付费触点保留规则。
- 但 Token Static（无论 Overlay 嵌入版还是独立版）没有写这个 Cookie。Overlay 仓库的写入方位于另一套 `website/src/components/root-document.tsx` / `website-static/html/assets/track.js`，不能因 Console 能读就认定 Token Static 已接通。

只读浏览器验证：带 `utm_source`、`utm_campaign`、`aff` 打开官网模型页，注册链接仍只有 `/sign-up?redirect=/workspace`；没有归因 localStorage/Cookie。

目前 Console 的 capture 还会把后来访问的业务路径当落地页并延长有效期（`attribution.ts:140`）。简单拼 UTM 不能完整解决“官网首次落地页不被后续 workspace 覆盖”的问题。

### 5.3 建议边界

补一条有界、白名单的统计信息交接：官网保存首次有效来源/落地页，进入 Console 时传递并合并到现有归因 envelope，保留 90 日 TTL 与服务端清洗。复用 Overlay 的归因合并业务规则，但不搬其他品牌的广告 SDK、PT 充值实验、注册即自动进入 Playground 的旧流程。

实现形态建议优先通过已有 Console 链接附带受控归因参数，必须用 URL API 合并现有 `redirect` 等参数，不能照搬 Overlay `buildAttributionHref` 无条件添加 `?` 的写法。若之后需要跨子域长期连续归因，可使用**独立、非认证的统计 Cookie**；不得为此扩大认证 Cookie Domain 或复制用户 session/token 到官网。

目标 `/sign-up` 当前没有 validateSearch/redirect 参数链路；注册成功 `use-auth-redirect.ts:110` 固定去 `/sign-in`，登录又有默认 `/dashboard`。官网 `?redirect=/workspace` 现在没有完整保留到登录成功，浏览器也观察到注册页的登录链接仅为 `/sign-in`。

建议补齐目标路径的传递与同源检查，让官网注册后正常登录去 `/workspace`，**不改变当前注册必须再登录的安全流程**。登录/注册相互切换、OAuth、MFA 的目标传递应一起核对，不能只补一条链接。

认证安全参考为最新稳定 OWASP ASVS 5.0.0、Authentication / Session Management / OAuth2 / Unvalidated Redirects Cheat Sheets。保留现有 `sanitizeAuthRedirect` 的同源校验、host-only HttpOnly Refresh Cookie、state/flow 校验；统计数据不能获得认证或授权意义。本轮仅检查接缝，不声称完成全站 ASVS 合规审计。

## 6. 独立 Static 与 Overlay 嵌入版的完整文件比较

逐文件比较 `origin/tokendazi-overlay:web/tokendazi_static/` 与独立官网固定 HEAD，共 29 个源文件：17 个相同，12 个不同，独立版另有 4 个文件。

相同的 17 个：

```text
Dockerfile
assets/brand-icons.js assets/config.js assets/home.js
assets/model-snapshot.js assets/models.css assets/models.js
assets/rankings-static.css assets/rankings.css assets/rankings.js
assets/site.js assets/token-dazi-logo.svg favicon.ico package.json
scripts/generate-model-snapshot.mjs scripts/pricing-core.mjs
tests/pricing-core.test.mjs
```

不同的 12 个及完整差异性质：

| 文件 | 独立版变化 |
| --- | --- |
| `README.md` | 增加中文开发/部署说明，少数描述与实现不一致 |
| `assets/header.html` | 文档链接改为 `wiki.tokendazi.com/cn` |
| `assets/home.css` | 适配 72px 顶栏，删除原滚动宣传条相关样式 |
| `assets/site.css` | 加强顶栏字号、间距、背景/阴影，适配移动端 |
| `compose.example.yml` | 上游改 `console:3000`，网络改 `solveacx-console_backend` |
| `index.html` | 补 SEO/OG/Twitter 信息、更新文档链接，主 CTA 改工作台，删除宣传条 |
| `models/index.html` | 补 SEO、更新 Logo alt 与文档链接 |
| `nginx.conf.template` | 增加四个 CLI 安装配置脚本的响应类型/缓存设置 |
| `rankings/index.html` | 补 SEO、更新 Logo alt 与文档链接 |
| `robots.txt` | 增加禁止抓取 `/api/` |
| `scripts/dev-server.mjs` | 增加 `.sh` / `.ps1` MIME 类型 |
| `sitemap.xml` | 增加 lastmod/changefreq/priority |

新增四个：`setup-codex.sh`、`setup-codex.ps1`、`setup-claudecode.sh`、`setup-claudecode.ps1`。

**迁移做法：保留独立 Static 最终页面，而不是用嵌入旧版覆盖。**模型/排行核心 JS 完全相同，既有设计语言无需重新实现；新的计费展示是新增契约适配，不是重新设计整站。

`assets/header.html` 没有运行时 include，三个 HTML 各自内联同样的导航。未来改链接须同时覆盖三个页面，不能只改这个片段就宣称全站生效。

## 7. 部署与本地预览需要同步适配

### 7.1 配置

- 本地 Go 为 3000、Console 为 5173；Static `assets/config.js:3` 与预览服务器默认都还写 5175。只设置 `API_ORIGIN` 不会同步修复 Console 登录注册链接。
- 生产 `nginx.conf.template:11` 已动态生成 config.js，注入 `CONSOLE_ORIGIN`、`SITE_ORIGIN`，并保持 `apiOrigin: ''` 同源代理，不需要新增浏览器跨域请求/放宽 CORS。
- compose 的网络及 Go 服务名应使用实际部署 topology，不能默认已有 `solveacx-console_backend` 或反过来套回旧 `new-api-network`。
- Console 只需补官网域名配置；已有 `lib/origins.ts` 目前仅含文档 URL，不能当作已有官网 URL 能力。公开导航当前也未调用该文档 helper，应避免两个配置源继续不一致。

### 7.2 API 代理不能只验证短 HTTP 请求

生产 `/v1/` 虽然已关闭响应 buffering，但没有显式 WebSocket Upgrade、HTTP/1.1 与长请求超时。当前 Go `router/relay-router.go:85` 已支持 Responses WebSocket，`:92` 支持 Realtime；若官网 Base URL 继续指向 `tokendazi.com/v1`，代理需要一并支持。

本地 `scripts/dev-server.mjs:16` 只转发 method/Accept、不转发请求 body/Authorization，不处理 WebSocket，且读完整响应后一次性发送。因此它能预览 pricing/rankings，却不能代表 `/v1` SDK/流式请求代理已通过验证。应当完善预览代理或让本地 API 示例明确指向真实 Go/Console API origin。

真实 IP 必须沿外层代理 → Static Nginx → Go 保持一致，配合现有 `TRUSTED_PROXIES`。不能一律信任客户端随意填的 forwarded headers；本轮未获取生产外层代理，因此不宣称生产 IP 链路已验收。

### 7.3 发布文件与缓存

当前 `Dockerfile:2` 把整个工作目录 COPY 进 Web 根目录；没有 `.dockerignore`，清理列表也不删 `.git`。Nginx 没有隐藏文件拒绝规则。本地预览请求 `/.git/HEAD` 已观察到 HTTP 200。

这是仓库/打包默认存在的发布风险；没有据此断言生产桶/服务器一定已泄漏。建议改为明确公开文件白名单 COPY、排除 `.git` / 本地配置/测试资料，并让服务器拒绝隐藏/非发布文件；四个应公开的 setup 脚本单独保留。

CSS/JS/快照目前没有 hash 文件名，却配置七日 immutable（`nginx.conf.template:44`），上线后客户端可能继续使用旧展示逻辑。需要版本化资源或改缓存策略，不能仅刷新服务器文件就认为所有用户已升级。

快照要在发布时从**匿名公开接口**刷新，不能打包管理员/用户专属分组数据；目前仓库快照只有一个模型，不能当作完整正式库存。SEO 生成脚本只处理模型页；README 中脚本名 `snapshot.mjs`、排行榜快照、网络示例等应随实际实现修正。

### 7.4 四个 CLI 脚本

已只读核对全部 581 行；没有执行安装、测试真实 Key、或修改本机 Codex/Claude 配置。

- Codex 使用 Console `/v1`、Responses、`TOKEN_DAZI_API_KEY`。
- Claude Code 使用 Console origin、Anthropic env/config。
- 四个脚本要求 `sk-` Key，与当前 Console 复制的完整 Key 一致；官网例子 `tdz_xxx` 是另外的占位写法，后续应统一。
- 默认模型固定 `gpt-5.4` / `claude-sonnet-4-6`，官网示例固定 `claude-sonnet`。这些不由当前公开模型列表验证；正式发布需核对实际开通模型，不能只因 `/v1/models` 返回 200 就宣称默认模型可用。
- 当前三个页面没有直接引用 setup 脚本，它们是可托管的配套入口，不是 Console 路由的一部分。

## 8. 建议实施边界（待讨论后才做 Spec/Tickets）

1. **Console 官网导航与历史公开路由外跳**：扩展现有 origins/links，迁移根黑名单，补认证页返回官网入口；保留当前会话机制与后台 `/models`。
2. **Static 模型价格契约适配**：在不改设计语言的前提下适配表达式首档、缓存、分时、分组与专属倍率，运行时与快照一致；具体接缝见价格核对。
3. **排行榜“全部”兼容**：恢复 Overlay 的 all 聚合能力，保留 Base 更优实现与 Static 展示算法。
4. **官网获客 → 注册/登录 → 工作台**：归因有界传递，首次落地页/TTL 正确，安全 redirect 全链路保留，不复刻另一套认证。
5. **部署/预览闭环**：域名/端口/network/API 上游、隐藏文件、资源缓存、SDK/流式/WebSocket 代理与发布说明。

不扩展为 Console 全站重做、不移走后台模型元数据、不新增第二套结算引擎、不复制旧钱包/PT 实验或其他品牌埋点、不把 Static 重写成 React。

页面外跳本身不需要新增 Go 页面路由；但价格数据的适配边界和 all 排行契约确实可能需要少量 Go 修改，所以“只改后端路由就够”不成立。数据库迁移预计不需要；以最终 Spec 对字段/查询的确认结果为准。

## 9. 本轮验证与限制

已执行并观察：

- Overlay 嵌入 Static 29 文件逐项比较，独立版新增文件清单。
- Static `node --test tests/*.test.mjs`：3/3 通过。
- site/home/models/rankings 四个 JS 的 `node --check`：通过。
- 两个 Bash 配置脚本 `bash -n`：通过；不等于真实安装/Windows 执行已通过。
- Playwright 官网首页、模型桌面/移动、排行桌面：没有 pageerror/失败请求或横向溢出；模型默认请求返回 9 条，其中表达式展示仍存在旧逻辑缺口。
- Playwright fixture：不可见分组能在 Static 模型展开区域出现，复现权限裁剪/展示过滤缺口。
- Playwright 线上 Console `/pricing`：JS 开关对照及重复验证，确认前端外跳。
- GET / Playwright 排行全部按钮：复现 all HTTP 400，前四时段 HTTP 200。
- Playwright 带 UTM/aff 的官网模型入口：Console 链接没有传递归因。
- Playwright 本地 Console 注册页：登录链接没有保留 `/workspace` 目标；本地 `/pricing` 仍显示原 Console Model Square。
- 价格专项通过执行 Static 原函数与目标纯解析层的确定性样例核对金额舍入、有效分组、标准阶梯/缓存、分时、fixed/mixed、请求倍率和任务单位；它不是实际扣费/生产配置验证，详见 `pricing-audit.md:271`。

既有证据：`/tmp/static-console-audit/report.json` 与该目录截图、`/tmp/overlay-pricing-routing-check.json`；补充只读检查的结果已记录在本文。

所有浏览器检查使用独立临时 context，未登录生产账号、未注册或写业务数据、未请求付费上游。没有运行官网 snapshot 生成器（它会改文件）、没有执行 CLI 配置脚本、没有重启 3000/5173。用于核对的临时 4001 服务器已关闭。

本轮无业务代码修改，因此不做 Bun 生产构建。后续只要改前端，需受限资源的构建/针对性测试与 Playwright 验收；不能拿本次旧代码测试结果替代实现后的验收。

## 10. 外部安全参考

- [OWASP ASVS 最新稳定版 5.0.0](https://owasp.org/www-project-application-security-verification-standard/)
- [Authentication Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Authentication_Cheat_Sheet.html)
- [Session Management Cheat Sheet：Domain/Path、HttpOnly、Secure、SameSite](https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html)
- [OAuth2 Cheat Sheet：state/PKCE、回调与 Token 保护](https://cheatsheetseries.owasp.org/cheatsheets/OAuth2_Cheat_Sheet.html)
- [Unvalidated Redirects and Forwards Cheat Sheet：解析 URL、同源/白名单目的地](https://cheatsheetseries.owasp.org/cheatsheets/Unvalidated_Redirects_and_Forwards_Cheat_Sheet.html)
