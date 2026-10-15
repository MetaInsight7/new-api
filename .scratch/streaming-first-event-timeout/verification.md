# 首事件超时实施验收

Branch: tokendazi_op
Base: cf0105704687bdbcbfef54546e488a029c3d0655
Date: 2026-10-15
Status: completed; unrelated baseline frontend failure recorded

## 实施结果

- 新选项使用现有 root 保护的 `/api/option/`：默认关闭、600 秒预填，1–9223372036 秒整数校验，无新增环境变量或数据库结构。
- 单项保存走现有可靠事务路径，落库成功才发布配置。配置同步按整个 streaming 模块校验/发布；非法值或不完整读取不能部分启用策略。进程内保存持有模块锁，并推进修订号；同步读取期间发生保存时，丢弃该次陈旧发布，不能覆盖刚保存的新值。
- Relay 建立时复制锁保护的策略值，所有重试沿用；每次 scanner 独立观察首事件。计时从上游响应头返回后的 SSE 扫描开始。
- 本地 ping、SSE 注释、空数据和 Claude 上游 `type=ping` 不解除预算；按真正的 Claude 解析路径接入，不根据下游格式推断。元数据、思考、工具调用和接受的未知事件算进展。
- 504 / `upstream_first_response_timeout` / `first_response_timeout` 贯通共享路径。适配器先检查失败再执行正常 finalizer；Gemini→Responses 桥接不再把该错误包装成空成功。
- 未提交时清理 SSE 响应头，返回普通错误；已提交后不追加普通 JSON，不伪造成功。Claude 原生、原生 Responses 和已有 Responses 桥接尽力发送协议内失败。
- 超时仲裁不等待下游写锁：立即关闭上游，再等待原有有界写入和工作退出。并发槽位仍由尝试生命周期释放，未修改 Usage/计费/恢复策略。

## 重要兼容说明

- OP 既有 `IsAlwaysSkipRetryStatusCode` 硬性排除 504/524；把 504 加进后台可重试状态范围也不覆盖硬性排除。本特性保留此规则，未新增 SkipRetry 标记，也未放宽旧策略。
- 先遇到可重试的 503 时仍可换渠；新渠道继续使用入站时的首事件策略。实际写过本地或协议心跳后仍由 `response_already_written` 停止重试。
- 其他节点沿用既有选项同步周期，无即时广播。未增加独立 timer 环境变量。
- 未触及 tokendazi_static；没有改变协议模块、广告运营、任务插件、支付或渠道负载快照所有权。

## TDD 记录

- 配置非法 0：先复现保存成功且覆盖旧值，再以校验及可靠持久化修复。
- 静默 SSE：先复现闲置超时而非首事件超时，再增加独立预算。
- Relay HTTP：先复现 200、SSE content-type 和假 `[DONE]`，再贯通 504 失败及清理响应头。
- 后台：先复现缺少开关，再实现既有设置布局中的表单及保存顺序。
- 非法同步 / 失败的部分读取：先复现部分启用，再按模块发布并拒绝失败读取。
- Claude 原生及三种桥接的 ping-only：先复现预算被心跳解除，再增加上游协议分类。
- 背压：先复现心跳转发持有写锁时 watchdog 无法结束（受控测试超时），再将超时仲裁与写锁解耦。
- 取消兼容：先复现新增取消 guard 会丢弃已排队的用量回调，再将新丢弃条件收窄为首事件预算已过期，保留旧的取消/用量处理语义。
- 已提交的原生 Responses：先复现只输出本地 ping 后直接结束、没有错误帧，再复用既有 writer 发送带安全错误码和脱敏消息的 SSE `error`，保留原 504 失败。
- 并发保存与同步：在数据库查询完成后暂停重载，成功保存 1800 秒再释放重载；先复现运行时与回读退回旧 1200 秒，再以模块修订校验修复。三库均运行该确定性回归。
- 时间相关新用例使用 Go `testing/synctest`、`io.Pipe` 和通道握手；无任意真实 sleep 或随机压力循环。

## 已通过的定向验证

- `go test ./controller ./relay/helper ./pkg/perf_metrics -run '^(TestStreaming|TestClaudeApplicationHeartbeat|TestRelayFirstEvent|TestFirstEventDeadline|TestStreamOutcomeClassification)' -count=1`
- `go test ./controller -run '^TestStreamingAdaptorsPropagateFirstEventTimeout$' -count=1`：覆盖所有共享宿主入口及已有协议内错误能力。
- `bun run test src/features/system-settings/models/__tests__/first-event-settings.test.tsx`：13 个交互用例通过，含保存顺序、失败、非法输入、可访问状态、回读和中英文切换；延迟两个保存请求，验证整个过程中按钮禁用、无提前成功反馈，并在成功/失败后恢复。
- `bun run typecheck`，涉及 TS/TSX 文件的 `oxlint`，保留许可证头的定向 `oxfmt` 校验以及七种语言新增键校验通过。
- `bun run build`：生产构建通过。产物总量约 67.14 MB（gzip 20.62 MB），沿用既有分包布局。
- 管理权限用例复用真实 `middleware.RootAuth()` 链，未登录 401、普通管理凭据 403。

## 真实数据库

命令（环境文件包含凭据，不输出其内容）：

```sh
set -a
source /tmp/codex-management-test-db.env
set +a
go test ./controller -run '^TestStreamingOptions' -count=1 -v
```

| 数据库 | 实际版本 | 结果 |
| --- | --- | --- |
| SQLite | 3.50.4 | 通过 |
| MySQL | 8.0.46 | 通过 |
| PostgreSQL | 15.19 | 通过 |

- 三库均验证：默认回读、保存、关闭保留时长、重新启用、非法更新、两次启动选项重载、并发保存不被旧重载覆盖，以及非法同步不部分启用；无跳过。复审修复后重新运行同一矩阵通过，日志 `/tmp/streaming-options-reviewed-matrix.log`。
- 保存失败和成功读取后故障注入另用 SQLite 验证不发布新策略，不替代三库成功路径。
- 复用并启动了既有本机 MySQL 测试容器及 PostgreSQL 测试实例；没有使用生产数据库。
- 全部数据库验收结束后，已停止本次启动的 MySQL 测试容器和 PostgreSQL 测试实例，恢复原先停止状态。

## 复用与范围

- UI 复用 `SettingsForm`、`SettingsSwitchItem`、`SettingsPageFormActions`、`FormField`、`Switch`、`Input`、`useUpdateOption` 及查询失效机制；没有新增通用组件。
- 已读取仓库 `.agents/skills/shadcn-ui/SKILL.md` 及 `web/AGENTS.md`，按项目封装优先复用；没有安装额外组件或依赖。前期把父目录技能清单中的缺项误判为仓库缺少该技能，复审时已更正。
- 整项后端只新增 `controller/streaming_first_event_test.go` 一个测试文件，另扩展既有 scanner 和性能结果测试。

## 最终验证与审查

- 宿主完整 `go test -p 2 ./...` 通过，记录 `/tmp/streaming-go-full.log`；完整 `go vet -p 2 ./...` 通过，记录 `/tmp/streaming-vet.log`。两者运行于复审修复前，修复后另跑受影响包和功能回归。
- 独立模块 `cd relaykit && GOWORK=off go build ./... && GOWORK=off go test ./...` 通过，记录 `/tmp/streaming-relaykit.log`；本次没有修改该模块。
- 复审修复后的 `go test -race -p 2 ./controller ./relay/helper ./pkg/perf_metrics -run '^(TestStreaming|TestClaudeApplicationHeartbeat|TestRelayFirstEvent|TestFirstEventDeadline|TestDisabledFirstEventDeadline|TestStreamOutcomeClassification)' -count=1` 通过，记录 `/tmp/streaming-final-race.log`。包含并发保存回归和三库配置路径，无跳过。
- 复审修复后的 `go test -p 2 ./model ./relay/channel/openai ./relay/channel/claude ./relay/channel/gemini ./setting/operation_setting ./relay/common -count=1` 通过，记录 `/tmp/streaming-reviewed-packages.log`；`go vet -p 2 ./model ./relay/channel/openai ./controller` 通过，记录 `/tmp/streaming-reviewed-vet.log`。
- 最终后台 `bun run typecheck`、13 个受影响交互用例和涉及文件 oxlint / 定向格式检查通过；最新交互日志 `/tmp/streaming-ui-final.log`。生产 UI 自构建通过后未再改动，只补测试。
- 后台全量 `bun run test --maxWorkers=2`：210 个文件通过、1 个文件失败；2396 个用例通过、1 个失败。失败为 `src/features/models/__tests__/model-listing.test.tsx:815` 的 `keeps an active visibility filter when its server result is empty`，找不到预期可访问名称的筛选按钮；记录 `/tmp/streaming-web-full.log`，不能宣称前端全量全绿。
- 基线核对：把固定起点 `cf0105704687bdbcbfef54546e488a029c3d0655` 的完整 `web/` 导出到临时目录，复用同一已安装依赖，执行 `bun run test src/features/models/__tests__/model-listing.test.tsx -t 'keeps an active visibility filter when its server result is empty'`，在同一断言独立复现失败。记录 `/tmp/streaming-model-baseline.log`；未改动该无关用例或模型列表代码。
- Standards：初审 1 项文档规范发现、0 项异味；Spec：初审 2 项 P2、无范围蔓延。全部修复并分别由原审查者复查关闭，详见 [review.md](review.md)。
