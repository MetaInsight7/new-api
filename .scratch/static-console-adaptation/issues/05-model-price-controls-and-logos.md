# 05: 调整官网价格格式、展开入口并复用供应商图标

**Spec:** [官网模型价目表入口与图标修订](../models-controls-spec.md)

**Status:** awaiting-browser-verification

- [x] 最低文案统一，价格显示四位小数，原始计算精度保留。
- [x] 黄色按钮只保留组数，计费类型入口移动到模型标签区域。
- [x] 区分纯分时与阶梯加分时，多个入口同步展开状态。
- [x] 使用供应商图标配置和同源官方 SVG，保留安全回退、来源与许可证。
- [x] 全部 41 项回归与本轮 JavaScript 语法检查通过。
- [ ] Playwright 桌面/移动及真实公开 API 验收通过。
- [ ] 完成按功能独立提交。

## 实际验证记录

最小复现：当前 Static 对 `MiniMax-M3 / MiniMax / Minimax.Color` 返回空字符串，`/<svg/` 断言失败。修正后相同图标调用返回官方彩色 SVG 引用。

在 Static 根目录执行：

```bash
node --test --test-isolation=none tests/public-pricing.test.mjs tests/pricing-presentation.test.mjs
timeout 30s nice -n 10 bun --smol test tests/*.test.mjs
git diff --check
```

针对性 Node 测试 14 项全部通过，Bun 运行全部既有 Node 测试文件为 41 项全部通过，未跳过原解析器源码包完整性测试。新增和修改的 JS 文件逐项 `node --check` 通过。

当前会话的受限环境阻止 Node 默认隔离测试子进程及 Node `spawnSync tar`；因此改用同一进程运行针对性 Node 测试，并使用 Bun 运行全部测试。此为测试运行环境限制，没有修改原源码包测试来忽略错误。

Playwright Chromium 已尝试启动，在 `sandbox_host_linux.cc` 处因 `shutdown: Operation not permitted` 退出（SIGTRAP）；本会话也无法连接 localhost 预览。浏览器阶段尚未执行，不能宣称页面验收通过；以前的截图不是本次证据。恢复允许浏览器和网络的执行环境后运行：

```bash
NODE_PATH=/usr/lib/node_modules STATIC_TEST_URL=http://43.173.125.76 \
  node --run test:browser
```

代码和规划保留在工作区，等待本次浏览器验证后提交；没有 push 或重启既有服务。
