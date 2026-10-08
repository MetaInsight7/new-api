# 模型专属倍率可视化配置验收

日期：2026-10-08。分支：`tokendazi_op`。来源：二开最新版 `solve/main@762ce21c4` 的分组折叠编辑器。

## 实现边界

- 管理入口：`/system-settings/billing/group-pricing` 的“定价分组”区域。
- 复用原 `GroupModelRatio` 字段、options 保存接口、后端倍率优先级与 Model Access 投影，不增加数据库字段或 API。
- 新编辑器保留按分组折叠、添加模型、编辑计费模型键／倍率和删除覆盖的交互；零倍率合法，空值不被转换为零。
- JSON 与可视化共用表单字段。拒绝负数、非有限数字、错误结构和重名编辑；原有未知键及未编辑的非归一化键不会被清理或改名。
- 成功更新 `GroupModelRatio` 后失效 `user-model-access` 缓存；保存失败不失效该缓存。
- Available Models 显示显式专属倍率，并继续显示现有基础价格。实际计费由已有后端以专属倍率覆盖默认倍率，不相乘。
- `DraftNumberInput` 的空值归零行为不适用于免费倍率配置，因此组合已有 Input、Field、FieldError。折叠、卡片、错误／空状态沿用项目组件；没有新增公共控件或依赖。

## 自动检查

前端任务串行运行，通过 systemd 限制 `MemoryMax=4G`、`MemoryHigh=3500M`、`CPUQuota=180%`，设置 `NODE_OPTIONS=--max-old-space-size=3072` 和 `RAYON_NUM_THREADS=2`。

- `bun run typecheck`：通过。
- 对本功能 18 个新增／修改前端文件执行 oxfmt 写入与检查：通过；对其中 11 个 TS／TSX 文件执行定向 oxlint：0 errors、0 warnings。
- `bun run test src/features/system-settings/models/__tests__ src/features/system-settings/hooks/__tests__/model-ratio-cache.test.tsx src/features/available-models --maxWorkers=1`：14 个测试文件、118 个测试通过。
- `go test ./setting/ratio_setting ./service -run 'Test(.*GroupModelRatio|EffectiveGroupRatio.*|ResolveUserModelAccess.*)'`：两个包通过，覆盖专属倍率优先级、零倍率、负数拒绝及用户模型访问投影。
- `bun run build`：完整生产构建通过，Rsbuild 报告 `built in 23.1s`。构建前停止开发服务以释放资源，结束后按原资源限制恢复 5173；3000 后端没有重启。
- `go build -o /tmp/new-api-model-ratio-visual-editor .`：包含新前端产物的后端编译通过；没有替换当前开发后端进程。

执行包含 shell 变量的 systemd 检查命令时使用 `--expand-environment=no`，避免 systemd 提前展开 shell 数组而将定向格式／lint 意外变成全目录扫描。已撤销本次意外产生的无关格式改动。

日志：`/tmp/group-model-ratio-ui-acceptance.log`、`/tmp/group-model-ratio-go-tests.log`、`/tmp/group-model-ratio-ui-build.log`。

## Playwright

脚本：`/tmp/verify-group-model-ratio-ui.mjs`，使用系统已安装的 Playwright；密码仅由 `NEWAPI_UI_PASSWORD` 环境变量传入。连接 `http://127.0.0.1:5173` 和真实开发后端，不替换业务 API，不请求上游模型。

在 `default / MiniMax-M2` 上完成 13 个检查点：

1. 原先无专属倍率的模型不显示专属倍率标记。
2. 可视化保存 0.25，原有其他分组和模型配置保持不变。
3. 同一浏览器单页导航回 Available Models，显示 `Exclusive ratio 0.25×`，基础价格不变。
4. 整页刷新后管理界面仍显示已保存的 0.25。
5. 空值及负值不可保存，空值也不可通过切换 JSON 静默丢弃。
6. 明确保存 0 后，页面显示 `Exclusive ratio 0×`。
7. 可视化 → JSON → 可视化的修改双向同步。
8. 390px 宽度下编辑器没有横向溢出。
9. 删除覆盖并保存，Available Models 不再显示专属倍率标记，恢复继承。
10. 390px 宽度下模型卡片没有横向溢出。
11. 中文界面正确解析新增编辑器标题与文案。
12. 没有 pageerror 或意外 HTTP 失败；既有 Base UI 调试提示单独记录。
13. 最终恢复原 `GroupModelRatio`，通过真实 API 复核与备份一致。

证据：`/tmp/group-model-ratio-ui/report.json` 和同目录的桌面／手机截图。此脚本修改共享倍率，仅用于已授权的开发环境，不应直接在生产执行。

## 既有警告

- 开发态既有 Base UI `nativeButton` 提示已单独记录。公共 `ui/button.tsx` 和 `ui/combobox.tsx` 与 main 无差异，本功能的独立编辑器测试没有新增这类提示，不为本功能修改公共组件。
- 构建仍提示既有 `routes/_authenticated/keys/__tests__/index.test.ts` 不是路由文件；路由生成器跳过它，构建成功。未在本任务修复无关路由测试组织问题。

## 交付

仅本地独立功能提交，不 push，不代表已创建 GitHub Issue。前后端开发服务继续运行于 5173 与 3000。
