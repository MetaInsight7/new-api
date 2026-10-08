# 官网模型报价展示精简验收

日期：2026-10-09。依据 [展示 Spec](models-display-spec.md) 与 [实施 Ticket](issues/04-model-pricing-display.md)。

## 实施结果

- Static `master` 独立提交 `b4e8d47` — `feat: simplify website model pricing and clarify tiered and timed prices`。
- 计费标签与筛选只保留按量、按次；将用户所写“按词”按已有页面“按次”理解。原金额及单位不改变。
- 主表和移动摘要去掉首档参考价、内部档名与默认条件，不在页面或隐藏展开内容渲染原始表达式；分时模型保留简短提醒。
- 只有多档或分时规则触发计费明细，缓存 5m/1h 本身不触发。多分组仍可单独展开分组价格，但普通缓存模型不因此展示计费明细。
- 桌面使用价格对照表，移动使用逐档条件与价格列表，输入、输出、缓存依序展示。时间表标记当前时段，不复制当前报价卡片。
- 条件展示复用 Base AST，长度使用千分位，小时条件准确转换为时间范围。来源独立报价、未配置状态及无法报价状态保留。
- 本轮只改 Static 展示、相关测试与文档，不改 Go 后端、计费结算、分组权限、管理员配置或排行榜。

## 实际验证

在 `/root/newapi/tokendazi_static` 执行：

```bash
git diff --check
node --run test
systemd-run --scope -p MemoryMax=768M -p CPUQuota=150% \
  /usr/bin/env NODE_PATH=/usr/lib/node_modules \
  STATIC_TEST_URL=http://43.173.125.76 node --run test:browser
```

- 差异空白检查通过，38 项 Node 回归全部通过，相关 JavaScript 语法检查通过。
- 系统 Playwright/Chromium 对真实公网预览完成桌面与移动验收，不是仅访问 localhost；脚本退出码为 0。
- 覆盖普通缓存单组无明细按钮、多组只有分组比价、主表无冗余文案与原始表达式、阶梯表、分时切换、页面隐藏后恢复及 BFCache 恢复。
- 覆盖筛选与标签一致、多来源主表报价与来源阶梯、匿名离线快照、真实公开 API、排行榜四周期。
- 正常场景没有意外 console error 或 pageerror，移动页面没有横向溢出。预期的离线刷新失败与正常页面错误分别断言。
- 收尾再次执行 `node --run test`，仍为 38 项通过、0 项失败；公网 `/models` HTTP 200。

## 截图证据

- `/tmp/static-public-pages/models-simplified-desktop.png`
- `/tmp/static-public-pages/models-simplified-tiers.png`
- `/tmp/static-public-pages/models-simplified-mobile.png`
- `/tmp/static-public-pages/models-simplified-time.png`

截图为本机测试证据，没有打包到仓库；阶梯截图包含真实 MiniMax-M3 两档输入、输出与缓存读取价格。

## 预览与部署边界

公网预览为 `http://43.173.125.76/models`，现有预览服务实时读取 Static 工作树，刷新即可查看。本轮没有重启 3000/5173，也没有 push、切换分支或执行正式部署。

运行中的 3000 仍为此前后端版本，本次前端验收不代表其已加载上一任务新增的 `group_model_ratio` 字段；该字段的后端验证及部署边界继续见 [原验收记录](verification.md)。本次展示改动不依赖重建 Console，也未修改表达式计费计算和结算。
