# 顶部导航验收记录

日期：2026-10-09。分支：`tokendazi_op`。参考树：Overlay `8f8390e4b`。

## 复用审查

- 官网链接生成、顶栏背景/边框、品牌组合标识及SVG以最终Overlay为参考；SVG与源文件逐字`cmp`一致。
- 保留Base更新的Sidebar/DropdownMenu组件、48px布局变量、认证/权限机制、搜索命令面板及主题偏好，不回移Classic框架或旧版本库。
- 仅顶栏inline标识使用TokenDazi字标；既有侧栏可配置系统身份、原项目文字及10个修改源文件的原版权头均保持原样。
- 移动父容器隐藏问题不照搬：恢复已有TopNav小屏下拉；主题设置复用原ConfigDrawer，由用户菜单控制打开，并明确返回焦点。
- 后端、数据库结构、Console内部页面路由及Static工作树不变；未提交的旧Static规划记录未混入本功能。

## 自动回归与静态检查

修改前的导航契约回归：5项中4项失败，确认旧版内置导航与目标不符。实现后最终命令：

```bash
cd /root/newapi/new-api/web
systemd-run --scope -p MemoryMax=1500M -p CPUQuota=150% -- nice -n 10 \
  bun run test src/hooks/__tests__/top-nav-links.test.tsx \
  src/components/layout/__tests__ src/lib/__tests__/origins.test.ts \
  src/components/__tests__/operation-documentation-link.test.tsx --maxWorkers=2
systemd-run --scope -p MemoryMax=3G -p CPUQuota=150% -- nice -n 10 \
  /usr/bin/env GOMAXPROCS=2 bun run typecheck
```

结果：5个文件、20项全部通过；类型检查通过。修改文件的`oxlint -c .oxlintrc.json`、`oxfmt -c .oxfmtrc.json --check`及`git diff --check`通过。新增浏览器驱动同样纳入lint/format。

第一次类型检查以1500MiB运行时达到该进程组的cgroup内存上限，被内核停止；提高至3GiB后通过，没有取消资源限制或改动服务器测试策略。

## 生产构建

```bash
systemd-run --scope -p MemoryMax=3500M -p CPUQuota=150% -- nice -n 10 \
  /usr/bin/env NODE_OPTIONS=--max-old-space-size=2304 \
  RAYON_NUM_THREADS=2 GOMAXPROCS=2 bun run build
```

结果：完整生产构建通过，Rsbuild报告28.6秒。存在此前已有的`routes/_authenticated/keys/__tests__/index.test.ts`非路由文件提醒，与本任务无关，没有修改它。

## Playwright：生产产物与5173双验收

使用系统安装的`playwright/test`及Chromium。生产产物在临时仅本机4173端口运行；API代理指向现有3000服务的IPv4地址。验收后停止临时预览，未停止或重启既有5173/3000/Static服务。

最终开发环境命令（用户名/密码从会话环境传入，真实值不存入驱动或文档）：

```bash
systemd-run --scope -p MemoryMax=1100M -p CPUQuota=150% -- nice -n 10 \
  /usr/bin/env NODE_PATH=/usr/lib/node_modules \
  CONSOLE_TEST_URL=http://127.0.0.1:5173 \
  CONSOLE_TEST_OUTPUT_DIR=/tmp/tokendazi-header-final-verification \
  node src/components/layout/__tests__/header.browser.mjs
```

生产环境使用同一驱动，将URL改为`http://127.0.0.1:4173`、截图目录改为`/tmp/tokendazi-header-production-verification`。两次最终运行均通过：

- 现有真实用户登录、四个正确的官网/文档目的地、安全的新标签页，以及实际点击模型外链产生正确目的地的新窗口。外部官网内容使用网络边界stub，不依赖该域名的旧部署页面。
- 48px sticky顶栏、非透明背景、1px底边；最终Overlay SVG实际加载；默认搜索/版本不占据工具区。
- 320/375/419/420/421/768/1024/1920px八种宽度，无页面横向溢出；桌面横排、小屏下拉、420px字标边界、菜单关闭焦点。
- 用户菜单可打开原主题/内容宽度设置；切换暗色成功；关闭返回用户按钮。侧栏折叠及Ctrl+K搜索仍可用。
- en/zhCN/zhTW/fr/ru/ja/vi七种真实翻译更新正确，官网路径始终不受语言影响。
- 无浏览器运行异常、无认证后资源失败或控制台error。登录前的匿名`/api/user/auth/refresh`返回401是Base既有认证探测；驱动仅明确允许该路径及状态，不忽略其他失败。

早期驱动真实保存多种语言触发Base“20次/20分钟”的关键操作限流，未改后端、未关闭限流。收尾将测试账户返回最初观察到的英文界面；最终驱动对语言保存请求做边界stub，并比较真实`/api/user/self`的完整`setting`字段，确保该轮七语UI验收不改账户偏好。登录提示通过真实关闭按钮处理；遵守Tailwind的`max-[420px]`严格小于边界；未使用force点击或固定sleep掩盖问题。

截图保留在上述两个`/tmp`目录，未进入Git；没有保存登录密码或可用认证令牌。

## 部署配置与提交边界

`web/.env.example`提供`VITE_OFFICIAL_WEBSITE_ORIGIN`及既有`VITE_OFFICIAL_DOCUMENTATION_URL`。它们是**前端构建变量**：通过`web/.env.local`或构建进程环境配置后，需要重新构建；不能仅给已构建的Go服务设置变量来改变链接。

默认目的地：`https://tokendazi.com/`、`https://tokendazi.com/models`、`https://docs.tokendazi.com/`、`https://tokendazi.com/rankings`。本任务不改变直接访问Console `/pricing`等内置路由的行为。

本功能独立提交，不push，不包含Static或此前未完成的模型展示工作。
