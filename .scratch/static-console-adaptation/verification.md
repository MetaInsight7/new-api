# Static 模型报价与排行榜适配：实施与验收

日期：2026-10-08。三个本地 Tickets 均已完成，按功能和仓库独立提交；没有推送、切换分支或部署新服务。

## 提交与边界

| 仓库 / 分支 | 提交 | 实际功能 |
| --- | --- | --- |
| `new-api/tokendazi_op` | `7857d181a` | 原生公开定价响应提供受可见分组及可用模型限制的模型专属倍率 |
| `tokendazi_static/master` | `1c6067a` | 官网应用公开模型专属分组倍率、正确选择最优组并保留零与小额精度 |
| `tokendazi_static/master` | `6129485` | 官网排行榜适配现有四周期、失败与乱序状态，保留既有展示倍率和曲线 |
| `tokendazi_static/master` | `ac389b3` | 官网显示表达式首档、缓存与完整阶梯、分时参考价及非 Token/provider 明细 |

后端只修改公开定价控制器并新增 HTTP 契约测试，没有修改数据库、模型定价配置、真实预扣或结算。前端仅修改独立 Static 的相关页面和工具，没有修改 Console React，也没有重跑无关的整套 Console 前端测试。

源版本固定参照仍见 [设计讨论记录](design-decisions.md)；完成后的提交不能反向替代研究基准。Tickets 位于本目录的 `issues/`，没有创建 GitHub Coding Plan issue。

## 后端验证

在 `new-api` 执行：

```bash
go test ./controller ./setting/ratio_setting
go build -o /tmp/new-api-static-pricing .
gofmt -w controller/pricing_test.go
go test ./controller -run '^TestGetPricing'
git diff --check
```

- controller 与倍率包完整测试通过；controller 本轮完整运行耗时 138.805 秒。
- 命名审查修正后重新运行全部 `TestGetPricing`，通过，耗时 0.156 秒。
- 构建通过，输出为 `/tmp/new-api-static-pricing`；该文件没有替换运行中的 3000 服务。
- HTTP 契约覆盖匿名/登录调用者、隐藏分组与模型、禁用能力、名称匹配、`all` 展开、显式零、空映射，以及响应缓存不被修改。
- 无数据库行为修改，没有以这些测试声称完成全项目三数据库兼容验收。

## Static 自动化验证

在 `tokendazi_static` 执行：

```bash
systemd-run --scope -p MemoryMax=512M -p CPUQuota=150% \
  /root/.bun/bin/bun scripts/build-billing-core.mjs
node --run test
NODE_PATH=/usr/lib/node_modules node --run test:browser
git diff --check
```

- 资源受限的 Bun 纯解析器构建通过；产物为 30,606 字节，包含原有版权通知，没有引入 Console 运行时。
- 30 项 Node 测试全部通过：表达式及来源完整性 12 项、传统公开报价与快照校验 6 项、排行榜回归 12 项。
- JS 语法检查覆盖页面、报价适配器、导出/复现脚本与浏览器测试脚本，均通过。
- Playwright 使用系统安装的 Playwright/Chromium，临时启动本地 4011 预览，结束后关闭；没有停止 3000、5173 或其他既有服务。
- 本 VPS 补装 `google-noto-sans-cjk-ttc-fonts`，确保截图中的中文不缺字。

### 浏览器实际覆盖

- 桌面 1440×1000 与移动 390×844：公开可见分组、专属倍率、最优组、分组切换及传统价格。
- 单分组模型展开：首档条件、缓存 5m/1h、完整阶梯，以及音频输出单独配置和显式零。
- UTC 07:59→08:00：摘要从 `$0.5000` 更新为 `$1.0000`，日间首档保留 `len <= 200000`；展开状态不丢失，不重新请求 API。
- 页面隐藏后恢复、BFCache `pageshow` 恢复：当前参考价重新解释，仍不重新请求 API。
- 图片按张、任务按秒与独立按次基础费、provider 分列和未配置来源、附加倍率另列、特殊表达式不回退旧价。
- 模型名及表达式作为文本显示，注入测试没有执行脚本；无意外控制台错误或 `pageerror`，移动没有横向溢出。
- 桌面/移动分页、搜索与计费筛选，以及解析器源码下载 HTTP 200。
- 有效匿名快照加错误实时响应：保留快照；跨分钟仍更新分时价，不将快照生成时刻的价格冻结。该场景的预期刷新错误与正常页面无意外错误分开处理。
- 对运行中的开发 API 实际请求：`/api/pricing` 成功；`today/week/month/year` 排行四周期均 HTTP 200，页面没有 `all` 操作。

排行榜空值、零值、失败恢复、乱序、重复切换、原曲线政策和脚本注入也由 12 项持久化 Node 回归覆盖，不调用收费上游，不进行压测。

### 截图证据

- `/tmp/static-public-pages/models-desktop.png`
- `/tmp/static-public-pages/models-mobile.png`
- `/tmp/static-public-pages/expression-desktop.png`
- `/tmp/static-public-pages/expression-mobile.png`
- `/tmp/static-public-pages/models-live-api.png`
- `/tmp/static-public-pages/rankings-live-api.png`

截图为本机验收证据，没有打包进源码仓库。

## 复用、来源及源码发布

复用真正 Base `main` 的纯解析器，不另写正则解释器。发布产物对应的 8 个纯 TS/类型文件分别与固定 Base `main` 源文件 SHA-256 比对，全部一致。来源文件、来源提交、Bun 版本、产物及源码包 SHA-256 均记录在 Static 的 `assets/vendor/billing-expression-source.json`。

保留 new-api/QuantumNous 原通知及完整 AGPL-3.0 许可证。模型页页脚提供源码包下载，包含全部所需解析器、类型定义、许可证与独立构建脚本，不以未推送提交的 GitHub 链接作为唯一源码入口。

实际执行了独立解包构建：

```bash
source_dir=$(mktemp -d /tmp/static-billing-source.XXXXXX)
tar -xzf assets/vendor/billing-expression-source.tar.gz -C "$source_dir"
systemd-run --scope -p MemoryMax=512M -p CPUQuota=150% \
  /root/.bun/bin/bun "$source_dir/build.mjs"
cmp assets/vendor/billing-expression.js "$source_dir/billing-expression.js"
```

构建通过，`cmp` 确认逐字节相同；Node 回归同时校验源码包、所有源文件、许可证和构建脚本的完整性。

## 审查与修正

### Standards

初次审查指出 2 项规范违反及 1 项维护性问题：原来源链接未闭环、测试新增单字母参数、重复 provider 渲染。

已分别补齐可独立复现的公开源码包与下载入口、将新增 Go 测试参数改为 `testContext`、复用 provider 渲染行为。验证依据为源码解包构建/哈希测试、针对性 Go 测试及 provider 的 Playwright 展开验收。

### Spec

初次审查指出 2 项缺失：分时摘要未显示当前首档适用条件、只配置音频输出倍率时该项目被遗漏。

已使用解释层的当前档条件，音频输出缺少独立音频输入配置时沿用 Base `GetAudioRatio` 的默认 1，保留明确的零。新增 Node 和桌面/移动 Playwright 回归均通过；格式畸形的表达式、provider/schema 另有安全降级回归。

标准与需求两条审查轴分开记录；上述初次发现均已修正，没有将初次发现隐去或把研究报告当作验收结果。

## 部署状态与后续操作

- 两个仓库的业务功能已独立提交；本轮没有 push 或修改远程配置。
- 3000 与 5173 保持原有服务。真实 API 页面验收说明现有接口可被新 Static 消费，不代表 3000 已加载本轮新增的后端字段；新增字段由当前代码的 HTTP 测试和构建验证。
- 上线时需部署本轮 new-api 后端、同步 Static 的页面和 `assets/vendor/` 全部文件，包括许可证与源码下载包。
- 发布快照仍通过现有无登录凭据的生成命令，从已部署的新后端取匿名响应；本轮没有用旧运行服务覆盖发布快照。
- 本轮没有实施审计中的全站路由、认证、归因或 Nginx 候选改造，也没有新增排行榜后端 `all`、修改文案、计费或真实统计。
