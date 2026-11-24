# 性能健康统计恢复官方三态与 Prometheus v2 Spec（2026-11-25）

## 1. 文档状态

- 状态：**实现、验证与提交完成；未 push、未部署**。
- 实施分支：`merge/upstream-cb074d31e`。
- 固定上游基线：`cb074d31e296b53ce252d402a7063079cbf7f990`。
- 本轮只在临时 worktree `/tmp/newapi-upstream-merge-assessment-20261123` 开发、测试和提交；**不 push、不部署**。
- 若实现需要偏离本 Spec，必须先更新本文件并记录原因。

## 2. 背景与问题

Tokendazi 曾为对齐二开 Prometheus，将官方模型健康统计的 `success / failure / ignored` 三态压缩为 `success / failure` 二态。该改动会把用户取消、本地业务拒绝、内容策略拒绝等不代表供应商健康的事件计入失败，导致内置健康报表和供应商稳定性判断被系统性拉低。

当前决定恢复官方三态分类，并以官方健康统计语义为权威；Tokendazi 已有的 Prometheus、持久化、Redis 聚合、性能报表和渠道观测能力继续保留，但必须服从同一三态口径。

## 3. 已确认决策

1. 最终 Relay 结果统一分类为 `success`、`failure`、`ignored`。
2. 官方 `ClassifyRelayOutcome` 语义是权威实现；仅做与本地调用链兼容所需的最小适配。
3. 健康成功率固定为 `success / (success + failure)`；`ignored` 不进入分子或分母。
4. `ignored` 不进入错误计数、延迟、TTFT、TPS 或 token throughput 统计。
5. 新健康数据使用独立 v2 存储，不迁移、不删除、不混读旧二态历史。
6. 旧 Prometheus 二态指标暂时保留兼容，明确标记 legacy/deprecated；新增 v2 三态指标作为健康口径权威来源。
7. 敏感词预检拒绝、并发门控拒绝等未触达上游的本地结果归类为 `ignored`。
8. 异步任务仅在取得最终成功或失败结果后进入健康统计；即时提交失败沿用既有“不作为健康样本”契约。

## 4. 领域契约

### 4.1 Outcome

```text
success  上游调用完成，结果可作为成功健康样本
failure  上游、网络、凭据、配额、限流、超时或协议故障，反映服务健康
ignored  用户取消、本地业务拒绝、内容策略拒绝或其他不应评价供应商健康的结果
```

分类与重试、渠道禁用、计费和日志记录相互独立。一个请求是否扣费、是否重试、是否写业务日志，不得反向改变健康 Outcome。

### 4.2 聚合

- `request_count = success_count + failure_count`。
- `ignored_count` 单独累计，仅用于可观测性，不参与健康成功率。
- `success` 与 `failure` 作为 eligible 样本，继续沿用官方内置报表现有的延迟、TTFT、输出 token 与生成耗时聚合语义。
- Prometheus 成功性能直方图只累计 `success`；错误分类只累计 `failure`。
- `ignored` 除 Outcome 计数外不累计任何内置报表或 Prometheus 性能值。
- 空窗口不得产生 NaN/Inf；沿用当前空值表示合同。

### 4.3 Relay 前事件

- 敏感词阻断或其他本地业务拒绝：`ignored`。
- 渠道并发已满且未发起上游请求：`ignored`。
- 调用方必须使用表达 Outcome 的接口；不得继续用名称暗示所有 Relay 前事件均为 failure 的 API。

## 5. 持久化与缓存隔离

### 5.1 数据库

- 新表：`perf_metrics_v2`。
- 新模型包含现有聚合字段，并增加 `ignored_count`。
- 唯一键仍为 `(model_name, group, bucket_ts)`；时间索引与旧表等价。
- 新代码的健康、性能、汇总和最近成功率查询只读写 `perf_metrics_v2`。
- 旧表 `perf_metrics` 保留原样，供回滚和历史审计使用；本轮不迁移、不清理旧数据。
- `AutoMigrate` 必须在 SQLite、MySQL 与 PostgreSQL 上可重复执行。

### 5.2 Redis

- 新 key 前缀：`perf:v2:`。
- v2 hash 增加 ignored 字段，并保持当前 TTL、原子增量和 flush 语义。
- 不读取或删除旧 `perf:` key；旧 key 自然过期。

### 5.3 内存桶

- `Sample` 使用 `Outcome`，不再用布尔 `Success` 表达三态。
- 热桶包含 `request_count`、`success_count`、`ignored_count` 及既有性能字段。
- 数据库、Redis、内存三条聚合路径必须产生相同计数。

## 6. Prometheus 合同

### 6.1 权威 v2 指标

新增：

```text
newapi_model_health_outcomes_total{model, outcome="success|failure|ignored"}
```

- 每个已观察模型按最终 Outcome 累计。
- 推荐健康率 PromQL：

```promql
sum by (model) (rate(newapi_model_health_outcomes_total{outcome="success"}[5m]))
/
sum by (model) (rate(newapi_model_health_outcomes_total{outcome=~"success|failure"}[5m]))
```

- `newapi_model_errors_total` 只记录 `failure`，不得记录 `ignored`。
- 模型延迟、TTFT 与成功流式计数只记录 `success`。
- v2 Outcome 计数受现有模型序列基数保护，不引入未受控标签。

### 6.2 Legacy 兼容

- 暂时保留 `newapi_model_requests_total{status="success|error"}`，避免现有看板和告警在切换时直接断裂。
- legacy 指标维持既有二态映射：`success -> success`，`failure|ignored -> error`。
- HELP 文本必须明确其为 legacy/deprecated，且不可用于供应商健康率。
- 新 Grafana 看板和健康告警必须改用 v2 指标；完成迁移后再单独决定旧指标下线时间。

## 7. API 与展示兼容

- 内置模型健康/性能 API 的 JSON 字段保持兼容。
- `request_count` 对外含义改回官方 eligible 样本数，即只包含 `success + failure`。
- `series_schema` 必须升级，避免客户端把 v2 结果误认为旧二态历史。
- 本轮不要求把 `ignored_count` 加入现有管理页面；它首先通过持久化与 Prometheus 可观测。
- 模型健康页的数据质量说明必须明确：客户端取消和业务拒绝已从健康率中排除。
- 最近成功率、模型汇总和分组汇总全部使用 v2 eligible 分母。

## 8. 测试先行验收场景

### 8.1 分类

- 本地参数、配额、敏感词、内容策略拒绝为 `ignored`。
- 用户取消、client gone、ping failure 为 `ignored`。
- 上游 401/403 凭据、429、配额、网络、超时、5xx 为 `failure`。
- 正常非流式、正常完成流、输出 token 上限为 `success`。
- 上游业务型 400（如 context limit）按官方规则为 `ignored`。

### 8.2 聚合

- 9 个成功 + 1 个用户取消：成功率 100%，`request_count=9`，`ignored_count=1`。
- 9 个成功 + 1 个上游 429：成功率 90%，`request_count=10`，`ignored_count=0`。
- ignored 不改变延迟、TTFT、TPS、token throughput 或错误分类。
- 旧 `perf_metrics` 中已有数据不得出现在 v2 查询结果。

### 8.3 Prometheus

- success、failure、ignored 均出现在 v2 Outcome 指标。
- ignored 不出现在 `newapi_model_errors_total` 或模型性能直方图。
- legacy 二态指标继续输出，并按兼容映射累计。
- 指标输出顺序稳定、HELP/TYPE 唯一、标签已转义。

## 9. 数据库验证矩阵

任何实现完成声明前必须验证真实数据库：

| 引擎 | 场景 |
|---|---|
| SQLite | fresh migration、旧 `perf_metrics` 共存、重复 migration、v2 upsert/query |
| MySQL | fresh migration、旧 `perf_metrics` 共存、重复 migration、v2 upsert/query |
| PostgreSQL | fresh migration、旧 `perf_metrics` 共存、重复 migration、v2 upsert/query |

必须记录真实版本、命令和结果。模拟、mock 或只跑 SQLite 不能替代三库验证。

## 10. 实施顺序

1. 添加三态分类与聚合失败测试。
2. 添加 v2 Prometheus 与 legacy 兼容失败测试。
3. 添加 v2 表隔离和迁移失败测试。
4. 恢复官方 Outcome 分类并改造内存 Sample。
5. 实现 v2 数据库、Redis 与查询路径。
6. 调整 Relay 前事件和 Prometheus 输出。
7. 运行定向、全量及三数据库门禁。
8. 更新本 Spec 状态并创建独立后续 commit。

## 11. 非目标

- 不迁移、回填、合并或删除旧二态历史。
- 不在本轮重做 Grafana 看板布局；只固定需要切换的 PromQL 合同。
- 不改变计费、退款、重试、渠道冷却或日志语义。
- 不修改与 Outcome 口径无关的上游合并内容。
- 不 push、不部署。

## 12. 完成定义

- 官方三态分类测试通过。
- 内置健康报表与 Prometheus v2 对同一事件给出一致结果。
- legacy Prometheus 兼容测试通过，且 HELP 明确弃用语义。
- SQLite、MySQL、PostgreSQL fresh/upgrade/idempotent 验证通过。
- 根模块定向测试、全量 test、vet、build 通过。
- `git diff --check` 通过，无凭据、生产数据或无关文件进入提交。
- 本 Spec 更新为完成状态，并在临时分支创建一个不 amend merge commit 的后续提交。

## 13. 实施结果（2026-11-25）

### 13.1 已完成

- 恢复官方 `success / failure / ignored` 三态分类；与固定上游基线相比，仅保留 Tokendazi 已有的 `StreamEndReasonFirstResponseTimeout -> failure` 明确分类。
- `Sample`、内存桶、Redis 和数据库聚合统一改用 Outcome；`ignored` 单独累计，不进入健康率、错误、延迟、TTFT、TPS 或 token throughput。
- 新增 `perf_metrics_v2` 与 `perf:v2:` 隔离命名空间；旧 `perf_metrics` 表和旧 Redis key 不迁移、不删除、不混读。
- 新增权威指标 `newapi_model_health_outcomes_total{model,outcome}`；旧 `newapi_model_requests_total` 保留兼容并标记 deprecated。
- 敏感词阻断和 Relay 前并发门控拒绝改为 `ignored`；上游凭据、429、5xx、网络与超时仍为 `failure`。
- 模型健康页和七语言资源已更新，明确客户端取消与业务拒绝不进入健康率。
- Responses HTTP/SSE/WebSocket 回归已覆盖取消、超时、业务拒绝、凭据失败、重试成功、正常完成与输出上限完成。

### 13.2 验证结果

- 定向后端：`go test ./middleware ./model ./pkg/perf_metrics ./relay/channel/codex ./relay/channel/openai ./service ./controller -count=1` 通过。
- Responses 回归：`go test ./controller -run '^(TestResponsesInterruptedStreamHealth|TestResponsesWebSocketDisconnectSettlesDeliveredOutputOnce|TestResponsesStreamOutcomesPreserveAccounting|TestResponsesHTTPHealthCountsFinalResult|TestResponsesWebSocketPreRoutingRejectionsFollowHealthClassification)$' -count=1` 通过。
- 全量后端：`go test ./... -count=1`、`go vet ./...`、`go build ./...` 通过。
- 前端：TypeScript project build、受影响文件 `oxfmt --check`、`oxlint`、`bun run build` 通过；构建仅报告仓库已有的 TanStack route 测试文件命名警告。
- SQLite：`3.50.4`，fresh、旧表共存、重复 migration、v2 upsert/query、ignored-only 持久化通过。
- MySQL：`8.0.46`，同上矩阵通过。
- PostgreSQL：`16.15`，同上矩阵通过。
- `git diff --check` 通过；变更中未发现凭据或生产数据。

### 13.3 发布边界

- 本轮不 push、不部署，不修改仓库外 Grafana 实例。
- 发布时需将健康率和健康告警 PromQL 切换到 `newapi_model_health_outcomes_total`；legacy 指标仅用于过渡兼容。
