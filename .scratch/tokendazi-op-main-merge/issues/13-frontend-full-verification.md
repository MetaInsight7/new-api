# 13: 执行前端全量验证

**Spec:** `../../2026-10-23-tokendazi-op-main-merge-spec.md`


**Type:** task
**What to build:** 对合并后的控制台执行依赖锁定安装、国际化同步、类型检查、lint、格式、全量测试和生产构建，并验证权限、渠道管理、Key 编辑和设置保存的组合行为。

**Blocked by:** 07–11

**Status:** resolved

- [x] 使用锁定 Bun 依赖完成安装，lockfile 无额外漂移。
- [x] i18n sync、typecheck、全量 Vitest 和生产 build 通过；全量 lint/format 已执行并证明全部当前发现继承自固定双亲。
- [x] 权限搜索、优先级拖拽、并发 stale、Channel Card、API Key owner quota、用户设置和监控设置定向测试通过。
- [x] 桌面与移动端关键布局、键盘/可访问名称、loading/error 状态和权限隐藏由组合行为测试覆盖。
- [x] 记录 Bun/Node 版本、完整命令、测试数量、构建结果和基线失败。
- [x] 未通过删除二开组件、字段或测试换取 typecheck/build 通过。
- [x] 前端组合未暴露需要 `needs-info` 的不可兼容交互。

## Answer

- Bun `1.4.2`、Node `v24.21.0`；`bun install --frozen-lockfile` 检查 1246 installs/1334 packages 且无变化，i18n sync 与 `bun run typecheck` 通过，日志见 `/tmp/web-final-{install,i18n,typecheck}.log`。
- 12 个定向文件共 113 个测试通过；`bun run test` 全量为 242 files、2664 tests 全通过，耗时 1434.46s，日志见 `/tmp/web-full-test.log`。权限、拖拽、stale、卡片、Key、profile、monitoring 的桌面/移动/键盘/错误态均包含行为断言。
- `bun run build` 通过，日志见 `/tmp/web-build.log`；仅输出定制基线已存在的 route 文件 warning（`keys/__tests__/index.test.ts`），不影响产物。
- `bun run lint` 当前为 149 errors/65 warnings，`bun run format:check` 当前为 33 个文件；定制父分支分别为 154/64、35 个文件，上游父分支为 155/66、22 个文件。当前每个 lint 文件+规则对及每个 format 文件都属于两固定父分支结果的并集，涉及合并文件的定向 lint 无 error，因此这些是明确的 baseline debt，不在本 merge 批量修复。日志见 `/tmp/web-final-{lint,format}.log`、`/tmp/{head,main}-{lint,format}.log`。
