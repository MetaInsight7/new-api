# Console 顶部导航对齐 Overlay

日期：2026-10-09。实施分支：`new-api/tokendazi_op`。
唯一参考：`dev-new-api/origin/tokendazi-overlay` 最终树，提交 `8f8390e4b`。

## 已核对的完整路径

- Overlay 实际构建 `web/default`；不是 Classic 导航，也不是二开 `main`。
- 完整核对 `app-header`、`header`、`top-nav`、`system-brand`、品牌组件及 SVG、`use-top-nav-links`、`origins`、公开导航调用、48px 高度及布局消费者。
- Base 当前包含内置首页、控制台、模型广场、排行榜、文档、关于；Overlay 精简为首页、模型、文档、排行榜，官网内容使用独立官网域名。
- Base 顶栏透明、品牌图标20px且附版本、工具区有搜索及主题设置；Overlay 使用侧栏同色背景和底边，品牌图标及字标更大，工具区仅通知、语言和用户。

## 实施契约

1. 默认导航顺序为首页、模型、文档、排行榜。首页及模型始终存在，与 Overlay 一致；排行榜继续尊重 `HeaderNavModules.rankings.enabled/requireAuth`。
2. 首页、模型、排行榜分别使用官网的 `/`、`/models`、`/rankings`；语言切换只改变名称，不改变官网路径。使用正常外链、新标签页及 `noopener noreferrer`，不通过 Console 客户端路由错误加载官网。
3. 复用 Overlay 的官网 origin 规范化和品牌 SVG/组合标识。官网默认 `https://tokendazi.com`，可由 `VITE_OFFICIAL_WEBSITE_ORIGIN` 构建时覆盖。文档复用既有 `VITE_OFFICIAL_DOCUMENTATION_URL` 入口，品牌默认统一为 `https://docs.tokendazi.com/`。
4. 顶栏维持48px、现有左右内边距及右侧导航；背景及底边对齐 Overlay。仅替换顶栏 inline 品牌，侧栏原有可配置名称、Logo、版本及项目身份保留。品牌点击进入官网首页。
5. 默认不显示搜索按钮或顶栏版本。搜索快捷键、命令面板及系统更新模块保留；`showSearch` 仍可显式启用。Base 的主题/内容宽度/侧栏偏好不删除：主题设置入口移入顶栏用户菜单，继续复用同一个 `ConfigDrawer`，关闭后焦点回到用户按钮。
6. 修正 Overlay/Base 共同存在的移动导航父容器隐藏问题：桌面横排，小屏幕使用既有 `TopNav` 下拉菜单，增加已有翻译的可访问名称。保留禁用外链的可访问禁用状态及键盘行为。
7. 所有显示文案使用既有七语翻译。共享链接生成器会影响公开页的链接目的地，但本任务不重做公开页外观、不改首页重定向、用户权限、后端或数据库。

## 复用与边界

复用现有 `Header`、`TopNav`、`SystemBrand`、`DropdownMenu`、`ConfigDrawer`、`ProfileDropdown`、`SearchProvider`。仅新增最终 Overlay 的品牌组合标识（现有图片+系统名称组件无法表达同一个SVG与字标尺寸）。不引入 Classic 或旧组件库。

`tokendazi_static` 的模型展示、图标和样式撤销工作保持原样，不混入本功能提交；此前其他 `.scratch/static-console-adaptation` 未提交记录保持原样。

## 验收

- 单元/组件：官网路径、默认/关闭/需认证排行榜、七语标签与固定路径、origin规范化、移动菜单键盘及禁用项、品牌标识和可配置侧栏保留、主题设置入口与关闭焦点。
- TypeScript、涉及文件lint/format、相关既有文档按钮回归。
- 资源受限生产构建，随后在5173真实Console中用Playwright验证桌面/移动、小屏、明暗主题、侧栏折叠、外链、语言菜单与主题设置入口；浏览器错误需定位并区分既有问题。
- 单功能提交，标题明确说明顶部导航和官网跳转；不push，不提交任何登录凭据。

状态：实现、相关回归、类型/lint/格式检查、受限生产构建及开发/生产双环境Playwright验收完成；详见 [验证记录](verification.md)。
