# 渠道并发实时状态可视化集成验收记录

验收日期：2026-10-10。

## 基线与边界

- 工作树：`/tmp/newapi-channel-concurrency-07-3b76bf8`。
- 分支：`codex/channel-concurrency-07`。
- 验收开始时 `HEAD` 与 `tokendazi_op` 均为 `3b76bf8836cadc705bf14348e2afa6ee21b8f1b3`，`git rev-list --left-right --count tokendazi_op...HEAD` 为 `0 0`；收口前上游分支未变化，因此无需合并。
- 功能固定基线为 `708d3ed782c9b218c7d2dc4ac21e2d5d56bf7b0f`，即六个功能提交之前、包含本规格文档的提交。本文所称“基线复现”均指该固定提交，不是用当前结果替代基线结果。
- 本票没有修改产品代码。验收只新增本文并更新 Issue 07；没有修复无关基线问题。
- 使用 Go `1.25.1`、Bun `1.4.2`、Node.js `24.21.0` 和 Chromium `153.0.8010.12`。
- 根仓库 `/root/newapi/new-api/one-api.db` 未作为服务数据库、未由验收脚本打开或修改；隔离服务显式设置仓库外临时 `SQLITE_PATH`。

## 功能测试清单

从最终功能 HEAD 重新执行：

```bash
git diff --name-only 708d3ed78..HEAD -- '*_test.go' 'web/src/**/__tests__/*'
```

退出码 `0`。功能差异派生出 12 个 Go 测试文件、6 个 Go 包、72 个顶层 Go 测试，以及 8 个前端测试文件。完整派生清单保存在 `/tmp/channel-concurrency-07-test-inventory.txt`。

Go 测试文件：

- `controller/channel_concurrency_status_test.go`
- `controller/channel_max_concurrency_update_test.go`
- `controller/relay_channel_concurrency_cooldown_test.go`
- `controller/relay_channel_concurrency_lifecycle_test.go`
- `middleware/distributor_channel_concurrency_test.go`
- `model/channel_max_concurrency_test.go`
- `model/option_channel_concurrency_test.go`
- `router/channel_router_test.go`
- `service/channel_concurrency_cooldown_test.go`
- `service/channel_concurrency_lifecycle_test.go`
- `service/channel_concurrency_pressure_test.go`
- `setting/operation_setting/channel_concurrency_setting_test.go`

前端测试文件：

- `web/src/features/channels/components/__tests__/channel-concurrency-editing.test.tsx`
- `web/src/features/channels/components/__tests__/channel-concurrency-i18n.test.ts`
- `web/src/features/channels/components/__tests__/channel-concurrency-list.test.tsx`
- `web/src/features/channels/components/__tests__/channel-concurrency-status.test.tsx`
- `web/src/features/channels/components/__tests__/channel-configuration.test.tsx`
- `web/src/features/channels/hooks/__tests__/channel-concurrency-snapshots.test.tsx`
- `web/src/features/system-settings/operations/__tests__/channel-concurrency-i18n.test.ts`
- `web/src/features/system-settings/operations/__tests__/channel-concurrency-settings.test.tsx`

## 后端验证

### 聚焦与竞态

- 聚焦回归：

  ```bash
  GOMAXPROCS=2 GOWORK=off go test -json -p 2 ./controller ./middleware ./model ./relay ./router ./service ./setting/config ./setting/operation_setting -timeout=20m
  ```

  退出码 `0`，耗时 `244s`；8 个包通过，记录到 3184 个通过事件、15 个跳过事件、0 个失败事件。跳过项是需要额外外部数据库／ClickHouse／VLLM 的既有测试；Ticket 06 的三库测试另行显式执行且没有跳过。日志：`/tmp/channel-concurrency-07-go-focused.log`。

- 功能范围 race：先从上述 12 个差异测试文件提取 72 个顶层测试名，以锚定的 `^(TestA|TestB|...)$` 正则作为 `RACE_RE`，实际调用为：

  ```bash
  GOMAXPROCS=2 GOWORK=off go test -race -json -p 2 ./controller ./middleware ./model ./router ./service ./setting/operation_setting -run "$RACE_RE" -timeout=20m
  ```

  `RACE_RE` 的 72 个实际成员逐项记录在 `/tmp/channel-concurrency-07-test-inventory.txt`，没有使用模糊包级通配结果冒充清单。退出码 `0`，耗时 `106s`；6 个包、72 个顶层测试、121 个含子测试的通过事件，0 跳过、0 失败，没有 race 报告。日志：`/tmp/channel-concurrency-07-go-race.log`。

### 全量、vet 与 build

- 全量 root + relaykit 回归：

  ```bash
  GOMAXPROCS=2 GOFLAGS=-p=2 make test
  ```

  退出码 `0`，耗时 `211s`；52 个有测试包通过，47 个包无测试文件。日志：`/tmp/channel-concurrency-07-go-full.log`。

- Root vet：`GOMAXPROCS=2 GOWORK=off go vet ./...`，退出码 `0`，耗时 `16s`。日志：`/tmp/channel-concurrency-07-go-vet-root.log`。
- relaykit vet：在 `relaykit/` 执行 `GOMAXPROCS=2 GOWORK=off go vet ./...`，退出码 `0`，耗时 `1s`。日志：`/tmp/channel-concurrency-07-go-vet-relaykit.log`。
- relaykit build：在 `relaykit/` 执行 `GOMAXPROCS=2 GOWORK=off go build ./...`，退出码 `0`。日志：`/tmp/channel-concurrency-07-go-build-relaykit.log`。
- Root 原样执行 `GOMAXPROCS=2 GOWORK=off go build ./...` 时退出码 `1`：Go VCS stamping 从 `/tmp` 探测 Git，命中环境中既有的空 `/tmp/.git` 后得到 `error obtaining VCS status: exit status 128`。同一命令在导出的固定基线 `708d3ed78` 上也以相同错误和退出码失败；日志分别为 `/tmp/channel-concurrency-07-go-build-root.log`、`/tmp/channel-concurrency-07-go-build-baseline.log`，诊断记录为 `/tmp/channel-concurrency-07-go-build-vcs-diagnosis.log`。
- 禁用仅与构建元数据有关的 VCS stamping 后，`GOMAXPROCS=2 GOWORK=off go build -buildvcs=false ./...` 退出码 `0`，耗时 `7s`。日志：`/tmp/channel-concurrency-07-go-build-root-no-vcs.log`。浏览器验收使用同一选项构建的临时二进制。

### Ticket 06 三数据库矩阵

- `/tmp/codex-management-test-db.env` 权限为 `0600`。执行 shell 仅通过 `source` 将 `TEST_MYSQL_DSN` 和 `TEST_POSTGRES_DSN` 注入子进程环境；命令、日志和本文均未输出变量值：

  ```bash
  set -a
  source /tmp/codex-management-test-db.env
  set +a
  GOMAXPROCS=2 GOWORK=off go test -json ./model -run '^TestChannelUpdateWithMaxConcurrencyPersistsExplicitZeroAcrossDatabases$' -count=1
  ```

- 退出码 `0`，耗时 `9s`。SQLite、MySQL、PostgreSQL 三个子测试全部真实执行并通过，0 跳过；覆盖显式 `0` 清除渠道并发上限的跨数据库持久化。日志：`/tmp/channel-concurrency-07-three-database-matrix.log`。

## 前端验证

所有命令均在 `web/` 执行。

### 测试、类型与生产构建

- 聚焦 Vitest：

  ```bash
  bun run test \
    src/features/channels/components/__tests__/channel-concurrency-editing.test.tsx \
    src/features/channels/components/__tests__/channel-concurrency-i18n.test.ts \
    src/features/channels/components/__tests__/channel-concurrency-list.test.tsx \
    src/features/channels/components/__tests__/channel-concurrency-status.test.tsx \
    src/features/channels/components/__tests__/channel-configuration.test.tsx \
    src/features/channels/hooks/__tests__/channel-concurrency-snapshots.test.tsx \
    src/features/system-settings/operations/__tests__/channel-concurrency-i18n.test.ts \
    src/features/system-settings/operations/__tests__/channel-concurrency-settings.test.tsx \
    --maxWorkers=2
  ```

  退出码 `0`，耗时 `100s`；8 个文件、150 个测试通过。日志：`/tmp/channel-concurrency-07-frontend-focused.log`。

- 全量 Vitest：`bun run test --maxWorkers=2`，退出码 `0`，耗时 `635s`；209 个文件、2372 个测试通过。日志：`/tmp/channel-concurrency-07-frontend-full.log`。日志中的 `Window.scrollTo()` 是 jsdom 既有提示，不是失败。
- 七语静态键：对两个渠道并发 i18n 测试文件执行 `bun run test ... --maxWorkers=2`，退出码 `0`，耗时 `5s`；2 个文件、16 个测试通过，覆盖 `en`、`zh`、`zh-TW`、`fr`、`ru`、`ja`、`vi`。日志：`/tmp/channel-concurrency-07-i18n-static.log`。
- 类型检查：`bun run typecheck`，退出码 `0`，耗时 `14s`。日志：`/tmp/channel-concurrency-07-typecheck.log`。
- 生产构建：

  ```bash
  systemd-run --scope -p MemoryMax=4G -p MemoryHigh=3500M -p CPUQuota=180% -- \
    env NODE_OPTIONS=--max-old-space-size=3072 RAYON_NUM_THREADS=2 GOMAXPROCS=2 bun run build
  ```

  退出码 `0`，耗时 `32s`；`dist` 为 `68M`、1768 个文件。唯一警告是既有的 `src/routes/_authenticated/keys/__tests__/index.test.ts` 不导出 Route。日志：`/tmp/channel-concurrency-07-build.log`。

### lint、format 与 i18n sync

- 功能范围 lint：对 27 个功能 TS／TSX 文件执行 `./node_modules/.bin/oxlint -c .oxlintrc.json <files>`，退出码 `0`。日志：`/tmp/channel-concurrency-07-lint-feature.log`。
- 功能范围格式：对 34 个功能文件执行 `./node_modules/.bin/oxfmt -c .oxfmtrc.json --check <files>`，退出码 `0`；34 个文件全部匹配。日志：`/tmp/channel-concurrency-07-format-feature.log`。
- 全量 lint：`bun run lint` 退出码 `1`，报告 158 errors、64 warnings。固定基线 `708d3ed78` 上同一命令同样为 158 errors、64 warnings、退出码 `1`；本功能文件的定向 lint 已通过，因此未擅自修改无关基线文件。日志：`/tmp/channel-concurrency-07-lint.log`、`/tmp/channel-concurrency-07-lint-baseline.log`。
- 全量格式：`bun run format:check` 退出码 `1`，列出 30 个无关文件。固定基线同一命令退出码 `1`，列出 32 个文件；当前少掉的两个是 Ticket 06 已格式化的 `src/features/channels/lib/channel-actions.ts` 与 `src/features/channels/lib/channel-form.ts`。日志：`/tmp/channel-concurrency-07-format.log`、`/tmp/channel-concurrency-07-format-baseline.log`。
- i18n sync-diff：`bun run i18n:sync` 自身退出码 `0`，随后 `git diff --exit-code -- web/src/i18n/locales/{en,fr,ja,ru,vi,zh-TW,zh}.json web/src/i18n/static-keys.ts` 退出码 `1`。生成器会改动 7 个 locale，共 1508 行新增、46 行删除；固定基线也产生同类差异，共 1518 行新增、46 行删除，等价 diff 检查退出码 `1`。两次生成改动均已恢复，未进入工作树。日志与补丁证据：`/tmp/channel-concurrency-07-i18n.log`、`/tmp/channel-concurrency-07-i18n-current.patch`、`/tmp/channel-concurrency-07-i18n-baseline.log`、`/tmp/channel-concurrency-07-i18n-baseline.patch`。

因此 Issue 07 的“七语国际化、typecheck、full lint、full format、生产构建全部通过”组合项保持未勾选：七语静态测试、typecheck、功能 lint／format 和生产构建均通过，但 i18n sync-diff、full lint、full format 被固定基线既有问题阻断。

## Chromium 端到端验收

### 隔离环境

- 先使用已通过的生产前端产物执行 `go build -buildvcs=false -o <临时运行目录>/new-api .`；最终二进制构建退出码 `0`，耗时 `10s`，大小 `166888289` bytes。日志：`/tmp/channel-concurrency-07-binary-build-final.log`。
- `/tmp/verify-channel-concurrency-07.mjs` 为仓库外验收驱动。它创建全新临时 SQLite、随机一次性 Root 和随机 session secret，显式设置临时 `SQLITE_PATH`，启动生产二进制，并使用系统 Chromium `153.0.8010.12`。
- 浏览器 context 通过 `context.route('**/*')` 阻断所有非本机 URL；动态渠道快照由 `page.route` 夹具提供。后端同时把大小写代理变量全部指向 `127.0.0.1:9`，并禁用后台更新任务。没有点击渠道测试、余额查询或 relay 操作。
- ChannelRead／ChannelWrite 身份通过受控的登录／self／refresh 响应夹具降低权限；真实写操作仍调用隔离后端。Root 使用真实 Option API。没有调用任何真实上游模型。

最终命令：

```bash
node /tmp/verify-channel-concurrency-07.mjs
```

退出码 `0`，耗时 `24s`，10 个场景组全部通过。日志：`/tmp/channel-concurrency-07-playwright-final.log`。机器可读结果：`/tmp/channel-concurrency-07-20261010-185655-cYMzyn/artifacts/result.json`。

### 场景结果

1. 使用全新临时 SQLite 完成一次 setup，并从真实 Option API 读到 12 项已确认默认值。
2. 创建只指向本机拒绝端口的临时渠道，初始并发上限为 `4`。
3. ChannelRead 卡片只读：首次快照 `503` 显示不可用占位，不出现编辑控件或错误 toast。
4. 受控序列 `503 → 0/4 → 2/4 +2 Cooling → 503 → 4/4` 验证首次失败占位、灰／黄／红状态、`+waiting`、蓝色 `Cooling` 与失败后的最后成功状态保留。
5. 桌面卡片和默认表格均显示并发状态；表格并发列不可排序，饱和 `4/4` 为红色。
6. `390×844` 移动卡片与移动表格均隐藏并发能力；两种布局分别截图复核。
7. ChannelWrite 通过真实 API 和行内控件执行 `4 → 6 → 0 → 5`：正数持久化、显式 `0` 清除限制并退出快照集合、重新设为正数后重新进入集合。
8. Root 面板始终显示 8 项常用策略与 4 项高级参数；依赖开关关闭后相关字段保留值并禁用。
9. 12 项原值按稳定顺序从 `true,5000,0,true,30,true,false,true,30,100,200,8` 改为 `false,5001,1,false,31,false,true,false,31,101,201,9`，逐项保存成功；刷新页面和复读真实 Option API 均与临时值一致。
10. 浏览器没有意外 HTTP、console 或 page error；非本机浏览器请求为 0，真实上游请求、渠道测试和余额查询均为 0。后端尝试的既有 Codex release 查询被本机代理 sink 拒绝，没有出网。

有效截图：

- `/tmp/channel-concurrency-07-20261010-185655-cYMzyn/artifacts/desktop-card.png`
- `/tmp/channel-concurrency-07-20261010-185655-cYMzyn/artifacts/desktop-table.png`
- `/tmp/channel-concurrency-07-20261010-185655-cYMzyn/artifacts/channel-write-card.png`
- `/tmp/channel-concurrency-07-20261010-185655-cYMzyn/artifacts/mobile-card.png`
- `/tmp/channel-concurrency-07-20261010-185655-cYMzyn/artifacts/mobile-table.png`
- `/tmp/channel-concurrency-07-20261010-185655-cYMzyn/artifacts/root-settings-desktop.png`
- `/tmp/channel-concurrency-07-20261010-185655-cYMzyn/artifacts/root-settings-mobile.png`

### 失败尝试与重跑

- `184914-pP2Euw`：驱动错误地期望 `/dashboard`，实际产品路由为 `/dashboard/overview`；退出码 `1`。
- `185229-O9pBW3`：隔离服务的默认 Web 限流在多身份导航时返回 `429`；退出码 `1`。
- `185323-wTsJ4Z`：仅关闭 API limiter 后仍由 Web limiter 返回 `429`，确认来源；退出码 `1`。
- `185515-IQ4HGi`：功能 10/10、退出码 `0`，但假时钟冻结 Motion opacity，截图为空白，因此未用作视觉证据。
- 最终重跑改用 reduced-motion 且不冻结时钟，得到上述有效截图。每次失败或弃用运行均进入同一 `finally` 清理路径；运行日志分别为 `/tmp/channel-concurrency-07-playwright.log`、`/tmp/channel-concurrency-07-playwright-rerun.log`、`/tmp/channel-concurrency-07-playwright-rerun2.log`、`/tmp/channel-concurrency-07-playwright-rerun3.log`。

## 恢复与清理

最终运行的 `finally` 完成并复核：

- 12 项设置逐项恢复为原默认值，并再次读取后做完整对象相等断言。
- 渠道并发上限从临时 `5` 恢复为原值 `4`，再次读取确认后删除临时渠道，并复读列表确认不存在。
- 关闭三个浏览器 context 和 Chromium，停止临时服务；最终没有遗留验收进程或监听端口。
- 删除临时 SQLite 及其 WAL／SHM／journal、临时二进制；一次性 Root 随临时数据库删除。
- 最终及四次先前运行目录顶层均只保留 `server.log`／证据目录，不含数据库或二进制。最终 JSON 的 `RESTORATION_COMPLETE` 为 `true`。
- 工作树根没有生成 `one-api.db`；根仓库 `one-api.db` 不在脚本可删除路径内，也未用于验收。

恢复证据位于 `/tmp/channel-concurrency-07-20261010-185655-cYMzyn/artifacts/result.json` 的 `settings.final`、`channel.final_max` 与 `restoration`，最终命令日志位于 `/tmp/channel-concurrency-07-playwright-final.log`。

## 结论

- 功能聚焦回归、功能 race、全量 Go／relaykit、三数据库矩阵、vet、禁用 VCS stamping 的 root build、relaykit build、聚焦与全量 Vitest、七语静态键、typecheck、功能 lint／format、生产 build 和真实 Chromium 验收均通过。
- full lint、full format、i18n sync-diff 与原样 root `go build ./...` 未通过；均已在固定基线复现并记录，没有将其伪装为通过，也没有修改无关代码。
- 浏览器动态状态完全受控，未调用真实上游；临时配置、渠道、账号、数据库、二进制与进程均已恢复或删除。
- 提交前 `git diff --cached --check` 退出码 `0`，暂存范围只有 Issue 07 与本文。

## 2026-10-11 调度一致性收口

初次验收后继续按固定参考分支 `solve/main` 收口并发调度语义：候选优先级耗尽返回空，相同负载率按分组有效权重随机排序，固定分组在递减优先级间共享抢槽预算，`auto` 每进入一个实际分组重置预算，并统一等待重试取消时的并发限制错误。此次只调整后端调度、相应自动化测试和规格记录；初次验收中的前端功能、权限边界与恢复机制保持不变。

### 最终验证

- `GOWORK=off go test ./model ./service`：退出码 `0`，两个包通过。
- 新增调度与取消语义的定向 race：

  ```bash
  GOWORK=off go test -race ./model ./service \
    -run '^(TestSatisfiedChannelCandidatesReturnsEmptyAfterAllPriorities|TestAcquireChannelConcurrencyWithWaitNormalizesCanceledRedisRetry|TestChannelCandidateOrderingUsesGroupWeightForEqualLoad|TestChannelSelectionFallsBackToLowerPriorityBeforeWaiting|TestAutoGroupSelectionResetsAcquireBudgetForEachGroup)$' \
    -count=1
  ```

  退出码 `0`，`model` 与 `service` 均通过且没有 race 报告。日志：`/tmp/channel-concurrency-final-race.log`。

- 真实 Redis 集成：

  ```bash
  TEST_REDIS_ADDR=127.0.0.1:6379 GOWORK=off go test ./service \
    -run '^TestChannelConcurrencyRealRedisIntegration$' -count=1 -v
  ```

  退出码 `0`；两客户端共享原子上限、快照缓存过期进入真实 pipeline、等待取消与冷却清理三个场景全部通过。日志：`/tmp/channel-concurrency-final-real-redis.log`。

- `GOMAXPROCS=2 GOFLAGS=-p=2 make test`：退出码 `0`；root 全量 Go 与独立 `relaykit` 全量测试通过。日志：`/tmp/channel-concurrency-final-go-full.log`。
- root `go vet ./...`、禁用 VCS stamping 的 `go build -buildvcs=false ./...`、`relaykit` 的 `go vet ./...` 与 `go build ./...`：全部退出码 `0`。日志：`/tmp/channel-concurrency-final-vet-build.log`。
- 前端全量 Vitest：209 个测试文件、2372 个测试全部通过；随后 `bun run typecheck` 与 `bun run build` 均退出码 `0`。日志：`/tmp/channel-concurrency-final-frontend.log`。
- 使用当前工作树重新构建临时生产二进制并运行隔离 Chromium 验收：10 个场景组全部通过，`RESTORATION_COMPLETE=true`；结果位于 `/tmp/channel-concurrency-final-20261011-QRm2tn/artifacts/result.json`，日志位于 `/tmp/channel-concurrency-final-playwright.log`。第一次直接调用驱动因未提供必填 `CC07_RUN_DIR`／`CC07_BINARY` 在任何场景执行前退出，随后按隔离要求创建仓库外临时目录并成功重跑。

### 最终结论

- 三项后续调度修复与固定参考实现语义一致，并同时覆盖内存缓存与数据库候选路径、跨优先级回退、分组有效权重以及 `auto` 切组预算重置。
- 全量后端、全量前端、定向竞态、真实 Redis、静态分析、生产构建和隔离浏览器验收均通过。
- 浏览器复验未调用真实上游，临时渠道、设置、账号、数据库、二进制和服务进程均已恢复或删除；证据目录只保留日志、截图和结果 JSON。

## 2026-10-13 渠道负载监控与紧凑编辑收口

本轮在 `tokendazi_op` 当前工作树完成 Ticket 08–11：新增管理员只读聚合接口、独立渠道负载监控页面，并将渠道列表并发状态改为单行紧凑展示与弹层编辑。验收使用 Go `1.25.1`、Bun `1.4.2` 和 Node.js `24.21.0`。

### 自动化验证

- 后端最终执行 `go test ./model ./controller`，两个包均通过；覆盖最近 60 秒消费日志按渠道聚合、日志类型过滤、受限渠道 RPM、分页独立汇总、聚合接口权限、筛选、排序、服务端分页，以及普通渠道不进入明细、RPM、类型、分组或 Redis 快照读取；`controller` 用时约 166 秒。
- 前端最终串行执行 9 个相关测试文件，使用 `--maxWorkers=1` 避免并行 worker 资源竞争；退出码 `0`，共 91 个测试通过。范围覆盖监控页面、共享渠道状态 tooltip、紧凑编辑、并发 i18n、列表列顺序、状态展示、并发快照、系统设置和侧边栏；监控页用例同时验证分组菜单不显示普通渠道独占分组。
- `bun run typecheck` 退出码 `0`。
- 对工作树中 21 个受影响 TypeScript／TSX 文件执行定向 `oxlint`，退出码 `0`；对本功能 15 个源码、七语 JSON 和公开原型目标执行 `oxfmt --check`，全部符合格式。全仓 `format:check` 仍报告 30 个未由本任务修改的既有文件，不在本轮越界改写。
- `bun run build` 退出码 `0`；唯一提示为既有的 `src/routes/_authenticated/keys/__tests__/index.test.ts` 被路由生成器扫描但不导出 `Route`。
- `git diff --check` 退出码 `0`。

### 隔离浏览器验收

- 第一轮使用仓库外临时 SQLite、临时二进制和隔离端口 `3011/4011` 完成页面生命周期验收；证据目录为 `/tmp/channel-load-monitor-acceptance.sOo72U/artifacts`。桌面页验证四张汇总卡、默认 20 条、第二页 3 条、URL 搜索、Root 策略入口和 10 秒轮询；移动页验证隐藏桌面表格、默认 10 条和负载卡片；编辑从 `3` 调至 `4` 后恢复为 `3`，故意注入 `503` 后页面保留最后成功快照并显示陈旧提示。
- 最新组件对齐与受限渠道口径复验使用 `/tmp/channel-filter-parity.o4dCOl/artifacts`：浏览器确认状态、类型、分组、优先级四个按钮与渠道页复用同一 class 和交互；类型菜单仅有受限渠道实际使用的 `OpenAI(1)`、`MjProxy(1)`，不出现普通 `Azure` 渠道；受限渠道 RPM 为 `3`，普通渠道的 4 条日志未计入，列表也只出现两条受限渠道。
- 最新浏览器验收控制台错误为 `0`；结构化结果为 `result.json`，截图为 `monitor-filter-parity.png` 与 `monitor-type-filter.png`。分组菜单的受限渠道口径另由前端行为测试验证，后端测试验证普通渠道独占分组不进入聚合返回。
- 所有验收均未调用真实上游、未产生费用；隔离 `3012/4012` 服务已停止，临时数据库与二进制已删除，只保留截图和结构化结果。

### 开发服务

- Redis 7.2.15 监听 `127.0.0.1:6379` 且 `PING` 返回 `PONG`；后端使用 `REDIS_CONN_STRING=redis://127.0.0.1:6379 PORT=3000 go run .` 启动于 `*:3000`，日志确认 `Redis is enabled`。`GET /api/status` 返回 `200`，未认证访问聚合接口返回预期 `401`。
- 前端使用 `bun run dev -- --host 0.0.0.0` 同时监听 `4001` 与兼容端口 `5173`，两端都代理 `3000`；本机监控页和代理 `/api/status` 均返回 `200`。
- `http://43.173.125.76:4001/channel-load-monitor/`、`http://43.173.125.76:5173/channel-load-monitor/` 及两端代理 API 均实测返回 `200`；外部五节点 HTTP 探针确认 `3000/api/status` 与 `5173/api/status` 全部返回 `200`，`ss` 确认两个前端端口监听 `0.0.0.0`。
