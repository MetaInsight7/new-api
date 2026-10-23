# `main` 合入 `tokendazi_op` 执行 Spec（2026-10-23）

## 1. 文档状态

- 状态：**已执行；本地可执行门禁完成，继承基线债务与外部数据库跳过项见第 13 节**。
- 目标分支：`tokendazi_op_merge1023`。
- 本 Spec 是本次合并的实现与验收基准；冲突处理、补充修复和测试结果均应能回指本文件中的决策或验收项。
- 本文件只记录合并约束，不代表冲突已经解决，也不代表允许提交或推送。

## 2. 固定基线

| 项目 | 固定值 |
|---|---|
| 定制基线 | `tokendazi_op@0f542cd5eccee9445c644392c8200d2c93efc822` |
| 上游基线 | `main@7fb6e57a5ff671582131c74e00bdfaa867d4e985` |
| 共同祖先 | `56758edf95ec162033a6b73b553ac789404f87c9` |
| 集成分支 | `tokendazi_op_merge1023` |
| 合并方式 | 已执行 `git merge --no-commit --no-ff main` |
| 当前状态 | 冲突已解决并完成本地验证；本 Spec 随最终 merge commit 提交，未 push |
| 初始冲突 | 36 个文件、70 个冲突块 |
| 当前剩余 | 0 个 unresolved path、0 个冲突块 |

本次执行期间不移动上游基线。若 `main` 再次更新，应在本轮完成后另做增量评估，不把新提交混入当前验收范围。

## 3. 目标

1. 合入固定上游基线的功能、安全、依赖和稳定性更新。
2. 保留 `tokendazi_op` 已上线的渠道并发、分组路由、Codex 治理、失败计费、首事件超时、运营监控及相关前端能力。
3. 对双方同时修改的行为做组合实现，不以“能编译”为完成标准。
4. 在提交前完成后端模块、前端、数据库兼容和关键状态机测试。
5. 给后续二次检查提供可重复的基线、决策记录、命令和测试结果。

## 4. 非目标

- 不顺带实现新的候选功能或迁移其他二开能力。
- 不重做已明确暂缓或不修改的 P2/P3 项目；包括不处理 `P3-09`、不动 `P3-05`，也不以“与二开一致”为理由修改此前明确无需变更的行为。
- 不重构与冲突无关的模块，不做目录、命名或 UI 风格清理。
- 不升级到 `7fb6e57a5` 之后的上游提交。
- 不直接修改原分支 `tokendazi_op`。
- 不在测试完成前创建 merge commit，不执行 push。

## 5. 安全与执行约束

1. `.scratch/` 中现有研究资料全部保留，不清理、不覆盖。
2. 高风险文件禁止整文件选择 `ours` 或 `theirs`；必须按不变量组合实现。
3. 任何计费路径都必须继续遵守 `.agents/rules/billing.md` 和 `pkg/billingexpr/expr.md`。
4. 用户可见 API 文案使用英文源消息和 `common.ApiErrorT` / `common.NewMessage`；服务端固定日志使用 `common.LogText`。
5. 前端新增或保留的英文源键必须存在于七个 locale，并通过 i18n 同步校验。
6. 对已自动合并的敏感文件也要做语义审计，不能只检查仍有冲突标记的文件。
7. 如实现与本 Spec 冲突，应先更新 Spec 并说明原因，再继续代码修改。

## 6. 核心行为不变量

### 6.1 计费与结算

- 上游新增的 OpenAI 12 小时 cache write、Claude cache-only/未拆分 cache write、Web Search、Google grounding 和 task retry 重新定价必须保留。
- `tokenkit/` 是合并后的统一 token 估算实现，不恢复旧 host estimator。
- 有上游权威 usage 时以权威 usage 为准；本地估算不能覆盖已拿到的 cache/search/tool 分类。
- 流式请求未向客户端交付真实输出时，本地估算 usage 必须归零，不能因心跳或空事件收费。
- Codex/Responses 在终止错误前已返回可靠 usage 时必须结算一次；该请求仍记为失败，不能额外写一条成功性能样本。
- 错误结算失败时不得静默退款或重复扣费；需要按现有策略保留预扣并写清日志内容。
- 用户、渠道累计用量、消费日志、预扣结算和性能样本均只能更新一次。
- 所有 quota 转换、饱和标记、分组倍率和 billing snapshot 继续遵守现有防溢出与审计规则。

### 6.2 Relay、SSE 与 WebSocket

- 首事件超时只由第一个有效业务 `data` 事件解除；注释、空行和心跳不能解除。
- 首事件超时发生时，即使下游 handler 被慢客户端阻塞，也必须及时关闭上游 body 并回收 goroutine。
- 慢客户端接收大型 SSE 事件时不得因 scanner buffer 或 handler 背压被截断。
- 已经排队的权威 usage 不能因随后发生的客户端取消而丢失。
- Responses 已正常完成后发生 context cancellation，不能改记为断流失败。
- 已提交 SSE 响应头后出现终止错误时，错误必须通过当前流协议返回，不能静默丢弃或改写成第二个 JSON 响应。
- Responses WebSocket 必须发送正确 close frame，并忽略已被替换的旧 upstream reader 事件。
- 渠道切换后使用绑定时的完整渠道快照、key、route 和 context；并发 lease 在所有退出路径恰好释放一次。
- 加密 reasoning 状态缺失或失配必须继续按定制逻辑终止，不能发送不可恢复的后续事件。

### 6.3 渠道路由与状态

- 渠道选择以定制的“分组 priority/weight”算法为准，并保留权重溢出保护。
- 精确模型无可用能力时，才使用上游 `RoutingMatchModelName` 做一次 fallback；fallback 后仍按原始请求模型执行 constraints。
- 普通渠道参与共享 task-plugin endpoint 竞争的上游能力必须保留。
- 渠道并发满载要保持 429/跳过无意义 retry 的语义，并保留 cooldown、排队和负载快照。
- 多 key 状态、轮询游标、渠道状态、ability 状态、Codex fingerprint 和模型治理必须在同一事务/锁语义下更新。
- 手工禁用、永久不可用、自动禁用及其原因不能被上游旧的非事务状态路径覆盖。
- 上游优先级拖拽不得破坏分组 priority/weight 覆盖；仅操作全局优先级接口定义的范围。

### 6.4 Codex 与 TLS

- Codex reset 的同一次用户操作在首次请求和凭据刷新重试之间复用同一个 request ID，避免重复消费 reset credit。
- 凭据刷新必须走集中式 `RefreshCodexChannelCredential`，使用 `ExpectedKey` 做并发保护并可靠持久化 rotation。
- 刷新后的请求必须使用刷新后渠道的 base URL、proxy 和完整 TLS settings。
- 渠道 `skip TLS verify` 只影响该渠道的目标上游连接；GitHub release/version 查询始终验证 TLS。
- 全局 Codex 身份强制开启时不查询 release，也不削弱最终出口的身份/version snapshot 约束。
- reset 响应没有确认重置窗口时视为不确定结果，提示先刷新 usage/credits，不允许鼓励立即重复提交。

### 6.5 错误、日志与国际化

- 用户错误只精准遮蔽当前请求涉及的自定义 upstream 地址，同时保留定制的其他敏感信息清理。
- 服务端日志保留可诊断根因，但使用 `LocalLogPreview` 和 `LogText`，不得把凭据写入日志。
- AI client 错误、通知和消费日志使用英文源消息；前端负责翻译。
- `RecordConsumeLogParams.Content` 使用 `[]*common.Message`，不恢复旧字符串格式。

### 6.6 前端能力

- 命令菜单与侧边栏使用同一权限/配置过滤结果，不显示用户无权访问或主动隐藏的页面。
- `SearchProvider` 只提供 context，不重复渲染 `CommandMenu`。
- 渠道页同时保留优先级拖拽、卡片/表格视图和实时并发状态；并发快照失效时隐藏伪造的零值并显示提示。
- API Key 编辑器同时保留定制的 quota、模型/分组限制、IP 等高级设置，以及上游 `allow_user_quota_query`。
- 监控设置同时保留渠道告警、quota reminder 和上游性能指标说明/配置。
- 用户设置保存必须基于当前完整设置增量更新，不得因修改一个开关覆盖其他通知字段。

## 7. 剩余冲突决策

### 7.1 低耦合与可生成文件

| 文件 | 决策 | 验收重点 |
|---|---|---|
| `THIRD-PARTY-LICENSES.md` | 依赖和 lockfile 稳定后，从当前 `go.mod`、`relaykit/go.mod`、`tokenkit/go.mod`、`web/package.json`、`web/bun.lock`、`electron/package.json` 和 lockfile 重新核对直接依赖；以上游清单为格式基线并补齐定制独有依赖，不手工拼接冲突两侧。 | 名称、版本、scope、license 与实际构建清单一致；`tokenkit` 纳入范围。 |
| `web/src/i18n/static-keys.ts` | 合并双方 key 集合，去重并保持项目排序；随后运行 `bun run i18n:sync`。 | 定制并发/运营键与上游价格/优先级键均存在；七个 locale 无缺键。 |

### 7.2 渠道、Codex 与状态

| 文件 | 决策 | 验收重点 |
|---|---|---|
| `controller/channel-test.go` | 保留 `SkipLog`、`TokenName`、`LogContent`；默认改为英文 `Model test`，日志内容包装为 `common.NewMessage`；服务日志使用 `LogText`。 | `SkipLog` 真正不写消费日志；治理 probe 的自定义名称仍生效。 |
| `controller/codex_usage.go` | 保留固定 reset request ID、消费确认和集中式 credential refresh；吸收上游英文源文案、`ApiErrorT`、错误根因日志及 `GetHttpClientWithProxySettings`。 | 401/403 刷新只重试一次；rotation 持久化；不确定 reset 不会诱导重复操作。 |
| `model/ability.go` | 保留分组 priority/weight 选择、渠道实体校验和溢出保护；仅在精确 ability 为空时加入 routing-name fallback。 | retry 按分组优先级下降；fallback 不绕过 constraints。 |
| `model/channel.go` | 保留定制事务、行锁、多 key/轮询互斥、Codex seed/governance、ability 同步、cache publish；仅吸收 `LogText`。 | 状态与 ability 原子一致；不存在上游旧 `saveStatusState` 的竞态回退。 |

### 7.3 Relay、计费与性能口径

| 文件 | 决策 | 验收重点 |
|---|---|---|
| `controller/relay.go` | 组合精准 upstream URL 脱敏、定制敏感信息清理和上游 SSE in-stream error；保留 officially-unsupported 拒绝、并发 429、`LastError` 根因与 task error 清理；用户文案接入 i18n。 | 已写 SSE 不再静默 return；未写响应仍输出正确协议 JSON；错误根因与状态码不丢失。 |
| `service/text_quota.go` | 将 extra content 统一迁移为 `[]*common.Message`；保留无真实输出归零、错误路径可靠 usage 结算、失败样本不重复、结算失败保留预扣；吸收上游英文日志/消息。 | 成功、无 usage、终止错误、结算失败四类路径均不重复扣费/退款/记样本。 |
| `pkg/perf_metrics/outcome.go` | 有意保留定制“失败保真”口径：业务拒绝、客户端取消和其他终止失败都计失败，不恢复 `OutcomeIgnored`；仅吸收上游“流已正常完成后 context 才取消仍计成功”的修复。 | `max_tokens` 正常结束计成功；首事件超时、content filter、client gone 和真实失败计失败；正常完成后取消计成功。 |
| `pkg/perf_metrics/metrics_test.go` | 保留双方测试：定制取消/失败纳入统计和 Prometheus；上游 recent status sample hours 与七天边界。按合并后的失败口径调整唯一期望值。 | DB 汇总、hot bucket flush、Prometheus、health detail 口径一致。 |
| `relay/channel/openai/relay_responses.go` | 保留加密 reasoning 恢复失败、Codex terminal failure 证据、权威 usage/error settlement；吸收上游 Responses tool/search 计费及完成事件处理。 | accumulator 每事件只 observe 一次；终止错误只结算一次；测试 probe 证据仍可用。 |
| `relay/channel/claude/relay-claude.go` | 采用上游 cache-aware 的 missing-input 判断和 tool-use fallback 开销；保留 `ContextKeyLocalCountTokens`、只补缺失字段和无内容时不制造收费 usage。 | cache-only、显式零 usage、tool-only、流中断四类场景正确。 |

### 7.4 Streaming 与 Responses WebSocket

| 文件 | 决策 | 验收重点 |
|---|---|---|
| `relay/helper/stream_scanner.go` | 以组合状态机重写冲突块：保留首个有效 data deadline/classifier，吸收上游 scanner 与 handler 解耦、慢客户端修复和完成后 cancellation 归一化；保持 idle timeout、ping、error precedence 和 body close。 | heartbeat 不解除首事件；大事件不断流；handler 阻塞时 timeout 仍关闭 upstream；队列 usage 不丢；无 goroutine 泄漏。 |
| `relay/helper/stream_scanner_test.go` | 两侧测试全部保留并去重；新增至少一个“正常完成后 context cancellation”和一个“大事件+慢 handler+usage”组合用例。 | 测试覆盖禁用首事件策略、动态配置快照、超时竞态和排队 usage。 |
| `relay/responses_websocket.go` | 使用定制 `bindResponsesWSChannel` 作为渠道快照唯一入口，确认其覆盖上游新增 locked fields；保留旧 target 事件过滤、并发释放和按实际 call 采样；吸收 close code/reason/frame 修复。 | 重连/换渠道不串 key/context；panic、upstream close、client close 的 frame 与 lease 都正确。 |

### 7.5 前端

| 文件 | 决策 | 验收重点 |
|---|---|---|
| `web/src/components/command-menu.tsx` | 使用独立 `search-context` 和统一 `useSidebarView`，删除重复的本地权限过滤拼装；key 使用 title+URL 避免碰撞。 | 搜索结果与侧边栏权限/配置一致；快捷键只打开一个菜单。 |
| `web/src/features/channels/api.ts` | 导出并保留并发快照 API与 priority order/reorder API。 | 两类请求均走 `requireServerSuccess`，重复请求/取消策略不互相覆盖。 |
| `web/src/features/channels/components/channels-columns.tsx` | 复用现有 `ChannelStatusCell`，在该组件内加入 `translateServerText`，不复制上游整段 status UI；保留并发列。 | 多 key 数量、自动禁用原因/时间和翻译均正确。 |
| `web/src/features/channels/components/channels-table.tsx` | 组合优先级拖拽/view mode 与并发 snapshot；columns 同时传 `concurrencyStatusById` 并按需插入 drag handle；card 同时接收 `reorderEnabled` 和并发展示数据；stale alert 位于 priority provider/页面布局的稳定位置。 | 表格与卡片均可拖拽；并发 stale 不显示零；筛选、批量操作和刷新不回退。 |
| `web/src/features/keys/components/api-keys-mutate-drawer.tsx` | 保留定制 drawer/layout、quota、模型访问和高级限制；把 `allow_user_quota_query` 加入现有高级区域，不采用上游旧 footer/layout 覆盖整段。 | create/edit 默认值、提交 payload、桌面预览和移动端滚动均正常。 |
| `web/src/features/profile/__tests__/settings.test.tsx` | 合并双方测试意图：完整设置增量保存、无设置默认值、`record_ip_log` 权限、quota reminder 单位/阈值；不通过删除断言来消冲突。 | PUT payload 不丢字段，也不发送未选择的可选开关。 |
| `web/src/features/system-settings/integrations/monitoring-settings-section.tsx` | 保留 `MonitorAlertFields`、`QuotaRemindThreshold` schema/default/UI；同时保留上游性能指标说明和表单布局；保存态以整个 form submit 为准。 | 告警与性能设置一次提交不互相覆盖；数值校验不回退。 |

## 8. 已暂时解决冲突的复核清单

这些文件在本 Spec 形成前已消除冲突，视为**暂定结果**，执行时仍需按以下决定复核：

| 文件/区域 | 固定决定 |
|---|---|
| `dto/channel_constraints.go` | 同时保留 `ExcludedChannelIDs` 与 `TaskPluginRelayOnly`。 |
| `setting/config/config.go` | 保留导出的 `UpdateConfigFromMap`，固定日志改用 `common.LogText`。 |
| `service/responses_usage.go` | 使用 `tokenkit.Count`，同时保留 `estimatedCompletion=true` 语义。 |
| `controller/topup_stripe.go` | 保留支付国家归因，吸收上游 `LogText`/i18n。 |
| `service/codex_channel_models.go` | 目标上游使用完整 proxy/TLS settings；GitHub 查询验证 TLS；强制全局身份时不查 release。 |
| `relay/channel/api_request.go` | 先应用上游 `ApplyHeaderOverride`，再执行定制 `finalizeRequest`。 |
| `service/relay_error.go` | 同时保留精准 upstream 地址脱敏和渠道并发 cooldown。 |
| `controller/user.go` | 基于现有设置增量更新，保留管理员通知开关，不恢复整对象重建。 |
| `model/log.go` | 采用上游 message/cache 导出结构，保留定制“始终记录 IP”。 |
| Search provider | context 与菜单渲染分离，Provider 不重复渲染菜单。 |
| Channel card | 同时支持 `reorderEnabled` 与 `showConcurrency`。 |
| Operations 设置 | 同时保留 Codex 身份字段和 `QuotaRemindEnabled`。 |

## 9. 实施顺序与门禁

### Phase 0：恢复与基线确认

1. 确认四个在 Spec 前被尝试处理的文件已恢复到原始冲突内容。
2. 保存当前 22 个冲突文件清单和 marker 数量。
3. 不改动 `HEAD`、`MERGE_HEAD` 和固定 merge base。

**Gate 0：** 四个文件内容与三方 merge 原始输出 hash 一致；原分支未变化。

### Phase 1：低耦合类型与渠道状态

1. 处理 `channel-test`、Codex usage、ability、channel status。
2. 复核已解决的 constraints、config、Codex client 和 user settings。
3. 先运行对应 Go package 的定向测试。

**Gate 1：** 路由、状态事务、Codex refresh/reset 定向测试通过。

### Phase 2：计费、Relay 与性能口径

1. 处理 `controller/relay.go`、`service/text_quota.go`、Responses/Claude usage。
2. 处理 performance outcome 与双方测试。
3. 审计所有 settlement/counter/log/sample 调用次数。

**Gate 2：** billing、relay、perf metrics 定向测试全部通过，无双扣、漏退或双样本。

### Phase 3：Streaming 与 WebSocket

1. 重写 scanner 冲突块并合并测试。
2. 处理 Responses WebSocket 渠道绑定和 close frame。
3. 运行 race-sensitive/synctest、Responses、首事件和慢客户端测试。

**Gate 3：** 首事件、慢 handler、大事件、cancel-after-complete、queued usage、WS reconnect 全部通过。

### Phase 4：前端组合

1. 处理 command menu、channels API/table/columns。
2. 处理 API Key drawer、profile tests、monitoring settings。
3. 合并 static keys，运行 i18n sync。

**Gate 4：** 相关 Vitest、typecheck、涉及文件 lint 通过；权限、拖拽和并发 UI 均有行为测试。

### Phase 5：生成文件与全量验证

1. 最后核对 `THIRD-PARTY-LICENSES.md`。
2. 清除全部 conflict marker，执行格式和静态检查。
3. 执行全量后端、前端、数据库兼容和生产构建。

**Gate 5：** 第 10 节全部必需检查通过，或明确记录因环境缺失而无法运行的非本地项目；不得把未运行写成通过。

## 10. 测试矩阵

### 10.1 合并完整性

```bash
git diff --name-only --diff-filter=U
git diff --check
rg -n '^(<<<<<<<|=======|>>>>>>>)' --glob '!*.md' .
```

验收：无 unresolved path、无冲突 marker、无 whitespace error。

### 10.2 Go 定向测试

使用根模块要求的 Go `1.26.10`，必要时设置 `GOTOOLCHAIN=auto`：

```bash
GOTOOLCHAIN=auto GOWORK=off go test ./controller ./model ./pkg/perf_metrics ./relay/... ./service/...
```

至少确认以下现有测试域通过：

- `controller`：relay stream/committed response、channel concurrency、Codex reset/governance、model availability。
- `model`：group priority、channel priority/status、Codex fingerprint/governance。
- `pkg/perf_metrics`：outcome、聚合、Prometheus、recent history。
- `relay/helper`：first-event timeout、慢 handler、cancel、queued usage。
- `relay/channel/openai`：Responses billing、Codex terminal/probe、tool/search usage。
- `relay/channel/claude`：cache-only、message delta、tool billing。
- `relay`：Responses WebSocket、request clone、stream status。
- `service`：text quota、Responses usage/encrypted reasoning、quota saturation、Codex credential refresh。

### 10.3 Go 全量门禁

先构建前端或创建 CI 同等 embed placeholder，再执行：

```bash
mkdir -p web/dist && touch web/dist/index.html
GOTOOLCHAIN=auto GOWORK=off go vet ./...
GOTOOLCHAIN=auto GOWORK=off go build ./...
(cd relaykit && GOWORK=off go vet ./... && GOWORK=off go build ./... && GOWORK=off go test ./...)
(cd tokenkit && GOTOOLCHAIN=auto GOWORK=off go vet ./... && GOTOOLCHAIN=auto GOWORK=off go build ./... && GOTOOLCHAIN=auto GOWORK=off go test ./...)
GOTOOLCHAIN=auto make test
```

### 10.4 前端定向测试

```bash
cd web
bun install --frozen-lockfile
bun run test -- \
  src/features/channels/components/__tests__/priority-order.test.tsx \
  src/features/channels/components/__tests__/table-refresh.test.tsx \
  src/features/channels/components/__tests__/channel-concurrency-list.test.tsx \
  src/features/channels/components/__tests__/channel-concurrency-status.test.tsx \
  src/features/keys/components/__tests__/api-keys-mutate-drawer.test.tsx \
  src/features/profile/__tests__/settings.test.tsx \
  src/features/system-settings/integrations/__tests__/codex-identity-settings-section.test.tsx \
  src/features/system-settings/operations/__tests__/channel-concurrency-settings.test.tsx
```

如冲突实现新增组合行为，应先补对应定向测试，再进入全量门禁。

### 10.5 前端全量门禁

```bash
cd web
bun run i18n:sync
bun run typecheck
bun run lint
bun run format:check
bun run test
bun run build
```

验收：i18n sync 后无意外删除定制键；typecheck/lint/test/build 全部成功。

### 10.6 数据库兼容

- SQLite：本地全量 Go 测试必须实际执行，不得只依赖 skip。
- MySQL：设置 `TEST_MYSQL_DSN` 后运行 model/controller 中 migration、priority、status、perf 和 audit 相关测试。
- PostgreSQL：设置 `TEST_POSTGRES_DSN` 后运行同一组测试。
- 如存在独立日志库，补充 `AUDIT_MYSQL_DSN` / `AUDIT_POSTGRES_DSN` 覆盖 log export/audit。

验收：三种主库 AutoMigrate、索引、行锁/事务和 priority revision 行为一致；ClickHouse 不因本次冲突引入新 schema 变更。

### 10.7 手工协议回归

至少保留测试证据或请求/响应记录：

1. 普通 Chat/Responses 成功、上游错误、SSE 已提交后的错误。
2. 首事件超时、仅心跳、慢客户端大事件、完成后取消。
3. Codex 401 刷新后重试、reset 成功、reset 不确定结果。
4. Responses 加密 reasoning 缺失、terminal failure 带权威 usage。
5. Claude cache-only、tool-only 和 web search usage。
6. 分组 priority/weight、`@` 模型 fallback、渠道并发 429/cooldown。
7. Responses WebSocket 正常关闭、上游断开、换渠道后旧 reader 事件。

## 11. 提交策略

1. 合并冲突解决和自动合并结果形成一个可审计的 merge commit；在 Gate 5 前不提交。
2. 若测试暴露与冲突解决无关、但合入后必须修复的问题，放在 merge commit 之后按模块拆分 follow-up commit，不把多个主题混进一个修复提交。
3. Spec、测试结果和未执行项目先汇报，由用户确认后再创建 merge commit。
4. 不 push；如需 push 必须另行明确确认。

## 12. 完成定义

只有同时满足以下条件才可声明“合并完成”：

- 22 个剩余冲突全部按本 Spec 解决，已解决的敏感冲突也完成复核。
- 第 6 节所有不变量都有代码路径和测试证据。
- 第 10 节本地可执行门禁均实际通过，环境依赖项如未执行已明确列出。
- `git diff --check`、conflict marker 检查、Go vet/build/test、前端 typecheck/test/build 均无错误；lint/format 若固定双亲均可复现，则要求当前发现全部属于双亲发现的并集、涉及合并文件的定向检查无 error，并在执行结果中明确记录。
- 无意外功能删除、权限放宽、计费口径回退或数据库兼容回退。
- 尚未 push，且在用户确认前不创建最终 merge commit。

## 13. 执行结果（2026-10-23）

- Go `1.26.10` 下根模块、`relaykit`、`tokenkit` 的 vet/build/test 全部通过；最终 `make test` 覆盖根模块 82 个 package、`relaykit` 17 个 package 和 `tokenkit` 1 个 package，失败 0。
- Bun `1.4.2`、Node `v24.21.0` 下 frozen install、i18n sync、typecheck、12 个定向文件 113 个测试、全量 242 个文件 2664 个测试及生产 build 通过。
- 当前全量 lint 为 149 errors/65 warnings，format check 为 33 个文件；每个当前 lint 的“文件+规则”以及每个 format 文件都存在于固定双亲结果并集中。定制基线为 154 errors/64 warnings、35 个 format 文件；上游基线为 155 errors/66 warnings、22 个 format 文件。本次不批量修复父分支债务。
- SQLite 由 Go 测试实际覆盖（测试输出版本 `3.50.4`）；`TEST_MYSQL_DSN`、`TEST_POSTGRES_DSN`、`AUDIT_MYSQL_DSN`、`AUDIT_POSTGRES_DSN`、`AUDIT_CLICKHOUSE_DSN` 均未配置，相关数据库项明确记为未运行，不记为通过。
- 最终检查为 0 unresolved path、0 conflict marker、0 whitespace error；原 `tokendazi_op` 仍指向 `0f542cd5e`，未 push，未发现需要 `needs-info` 的能力冲突。
- 原始日志：`/tmp/go-final-vet-build.log`、`/tmp/go-final-make-test.log`、`/tmp/web-full-test.log`、`/tmp/web-build.log`、`/tmp/web-final-{install,i18n,typecheck,lint,format}.log`。
