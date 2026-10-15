# Prometheus 指标对齐

Status: resolved
Type: task
Branch: tokendazi_op
Created: 2026-10-15

## Notes

- 用户请求：`$grill-with-docs 那就先把prometheus的指标对齐吧`。
- OP 固定起点：`51f66111b`；上一项渠道测试失败告警已经完成，本轮不重复迁移。
- 对照可访问二开镜像 `solve/main`：`762ce21c429a3aa9adc104bcd80eb7df3ad3bdbd`。不据此断言作者线上 Grafana 或其他私有仓库现状。
- 已完成 grilling + domain-modeling、spec 及四票实施。最终测试与双轴复核通过，按用户 `$implement` 要求提交当前 `tokendazi_op` 分支，不推送或新建分支。
- 历史迁移研究在 `.scratch/solve-migration/`；这些笔记固定于较早 OP 基线，不能把其中的首事件超时、测试失败告警继续视为未实现。
- 本次访谈与代码锚点见 [grilling.md](grilling.md)。
- 实施规格见 [spec.md](spec.md)，包括 48 条用户故事、共同指标契约、OP 增量边界和 18 项验收矩阵。

## Decisions-so-far

- 用户希望下一项先对齐 Prometheus 指标，而不是响应对象 ID 日志增强。
- 已澄清 OP 具有 `/metrics` 和渠道 / 渠道×模型导出；对齐不是从零接入 Prometheus。
- 已澄清应用指标代码不等于 Prometheus 存储或 Grafana 看板；二开仓库有 GCP 抓取部署配置，但未发现通用 Grafana 部署与可导入看板 JSON。
- 将既有领域区别记录为“渠道尝试”和“请求最终结果”，不把一次请求中的多个渠道尝试当作多个客户端请求。
- Q2 已确认：新增 Prometheus 模型指标采用二开的请求结果统计口径，不采用此前保留 OP 健康归因的推荐。
- Q1 已确认：不新增 Prometheus / Grafana 服务；应用侧抓取鉴权也对齐二开，包括鉴权失败限流及未配置服务端 token 的响应行为，不删除既有访问保护。
- Q3 已确认：健康 / 性能报表也对齐二开，与新增模型请求指标统一最终成败判定；之前开发的代码若不一致，直接修改，不保留旧口径开关或双轨兼容。本轮范围扩展限于观测功能，不因此改动重试、路由、计费或渠道治理的业务规则。
- Q4 已调整：用户说明当前为测试环境，历史数据不用处理；不开发历史回算、迁移、切换时间标记或新旧口径过渡机制。该说明不等于要求清库或删除现有数据。
- Q5 已确认：保留 OP 的 Responses WebSocket 逐请求、异步任务终态及 panic 失败兜底采样覆盖；接入共同的请求指标 / 报表时统一二开统计口径，同一请求或任务终态只记一次最终结果。保留覆盖不等于保留旧忽略分类。
- 用户要求本轮共同观测能力对齐二开：指标契约、默认资源保护、报表统计行为以固定二开为基线；不再把此前“只新增 Prometheus、保留旧报表”当作已选设计。不能把对齐静默实现为删除既有能力或数据。
- 后续补充原则：二开是共同能力的基线，不是 OP 监控能力的上限。OP 新 relay 或其他新功能若需要监控，可以额外增加采样或独立指标，按功能实际语义设计，不强制照搬二开不存在或不适用的实现。扩展应明确统计边界、避免重复计数，并控制标签基数及敏感信息暴露；这不自动扩展为全面重做其他功能或部署监控服务。
- 已核对二开健康报表：与模型 Prometheus 请求计数共用最终成败判定，不另行排除进入最终打点的取消和上游参数拒绝；报表自行落库查询，不依赖部署 Prometheus / Grafana。共用判定不代表不同窗口及存储来源的显示数值完全一致。

## Design Tree

- 目标：补齐 OP 与二开最终指标契约的真实差异。
  - Q1：迁移范围及抓取鉴权（resolved：不部署；应用鉴权对齐二开）。
    - 已核对：二开最终请求计数、成功耗时 / TTFT 分布、成功流式请求数、最终错误分类，以及模型准入 / 丢样指标；精确契约见 grilling.md。
    - Q5：保留 OP 独有 Responses WebSocket、异步任务终态及 panic 兜底的采样覆盖，共同请求指标统一为二开口径（resolved；不保留旧分类）。
    - OP 扩展原则（resolved）：新 relay / 新功能按需增加监控，不要求二开先有对应实现；独立观测可按实际语义设计，公共计数不得重复累计。
    - 已定方向：共同模型指标的准入、空闲回收、总序列预算与丢样展示以二开为基线；OP 扩展按需设计必要保护，不因对齐而禁止新观测。
  - Q2：新增模型指标采用二开请求结果口径（resolved）。
    - 已选择：计入二开打点范围内的取消和上游参数拒绝，不用 OP 的忽略结果代替这些最终错误。
    - Q3：旧健康统计 / 性能报表一起对齐二开（resolved；直接修改旧实现，不保留双轨）。
    - Q4：历史数据处理（resolved：测试环境，历史不纳入本轮范围，不做回算、迁移或切换机制）。
    - 已核对：旧分类器只影响观测，治理链独立；同渠道恢复中间拒绝不额外记最终失败，与 ADR0006 不冲突。
  - 验收与交付（范围已由 Q1 至 Q5 确定，落实到后续 spec）。
    - 应覆盖指标契约、成功 / 失败与错误分类、流式终态、重试最终一次计数、OP 独有入口、并发 / 预算保护及鉴权回归。
    - 不部署外部监控服务；应用导出与 HTTP 抓取契约可自动化验证，不因此宣称生产采集或 Grafana 看板联调完成。
    - 若实施触及数据库行为，按仓库规则验证真实 SQLite / MySQL / PostgreSQL；不额外实现历史数据处理，也不自动删除现有数据。

## Fog

- 无待用户决定的访谈分支；Q1 至 Q5 已确认，不继续追问旧口径兼容、默认预算、历史切换或独有覆盖等已选方向。后续实施细节与验收条目在 spec 中展开。
- Q4 已按测试环境说明调整：历史数据不处理，撤销此前记录切换时间和管理跨切换窗口的要求；不因此自动清库。
- Q5 已确认保留覆盖、共同指标统一口径；OP 新功能可按需扩展监控。Responses WebSocket 不是 Realtime；两树都没有完整 Realtime 最终成功采样，是否补充应依据实际需要，而非以二开是否已有为唯一条件。
- Q5 澄清：不是 OP 现在漏记，而是只照搬二开采样位置会漏掉 OP 已有的统计覆盖；二开缺异步任务终态统计不等于缺少异步任务执行能力。
- 二开模型指标契约已由只读代理核对；其采样覆盖、预算和丢样计数存在明确限制，后续应在 spec 中如实记录，不把源码存在当作线上验收完成。
- 实施定向测试、真实三数据库、完整后端、重复执行的采样／鉴权／任务 race 及双轴复核全部通过，当前无待解决评审项。不部署或宣称完成生产 collector／Grafana 联调。

## Comments

- 2026-10-16：实现 [01](issues/01-scrape-authentication.md)、[02](issues/02-final-outcomes-and-reports.md)、[03](issues/03-final-errors-and-protection.md)，[04](issues/04-success-distributions.md) 完成主体并进入最终验证。保留 OP Responses WebSocket／异步终态／异常收尾，不改变计费、重试或渠道治理。
- 2026-10-16：真实数据库验证通过。隔离测试库为 `codex_prometheus_20261016`，版本 SQLite 3.50.4、MySQL 8.0.46、PostgreSQL 15.19；设置隔离 `TEST_PERF_MYSQL_DSN`、`TEST_PERF_POSTGRES_DSN` 后执行 `go test ./pkg/perf_metrics -run TestPerformanceAggregationAndFlush -v -count=1`，三个子测试均通过，无 DSN 跳过。日志 `/tmp/prometheus-db-matrix.log`；没有迁移或删除历史数据。
- 2026-10-16：定向验证包括 `go test ./middleware ./service -run 'TestPrometheus.*|TestUpdateBatchTasksSettlesTieredUsageForTerminalStates' -count=1`、HTTP／WebSocket 最终结果与账务回归以及 `go test -race ./pkg/perf_metrics -count=1`；均已通过。完整验证结果后续追加，不预填。

- 2026-10-15：开始第一轮访谈；本地检查 OP 统计与设置，代理只读核对固定二开树；不改业务代码、不暂存、不提交。
- 2026-10-15：二开核对完成；用户追问 Q2“业务归因沿用 OP”的含义，已改用“成功 / 失败 / 不计入的统计规则”解释；尚未收到 Q1 / Q2 的选择。
- 2026-10-15：用户确认 Q2 对齐二开最新版；推进第二轮，保留 Q1 范围问题，并确认新增请求结果指标与 OP 既有健康报表是否隔离。不因该回答开始业务实施。
- 2026-10-15：用户确认不部署，并追问抓取鉴权与二开健康报表实现。本地核对现有鉴权，复用只读代理核对二开报表数据流；尚未收到 Q3 的最终选择。
- 2026-10-15：报表核对完成：二开健康报表只读已落库安全窗口，性能报表合并数据库及当前节点热桶，二者与模型 Prometheus 共用成败判定；主动探测是另一套状态。澄清此前保留 OP 旧报表仅是迁移建议，Q1 鉴权增量及 Q3 仍待确认。
- 2026-10-15：用户确认“Q1对齐二开，Q3健康/性能报表也对齐二开，全都对齐，之前开发的代码如果没对齐，那就直接改”。采纳全观测域对齐及直接修改旧实现，撤销保留旧报表口径的推荐；继续核对独有采样覆盖与历史聚合数据切换，不擅自删除能力或数据。
- 2026-10-15：用户确认 Q4 按推荐保留历史、不回算；追问 Q5 是否因 OP 有新能力而漏统计。澄清这是直接照搬二开挂点的潜在回退，不是 OP 当前已漏记，也不能把缺少终态统计混同为缺少任务执行能力。
- 2026-10-15：用户确认“保留统计覆盖，统一二开口径”。Q5 解决，Q1 至 Q5 全部确认，访谈标记 resolved；最终范围供后续 spec 使用，本轮未开始业务实现或提交。
- 2026-10-15：用户补充“历史不用管，因为当前也是测试环境”。覆盖之前 Q4 的切换处理要求，将历史回算、迁移、切换时间标记及过渡机制移出范围；不推导为清库授权，其余决定不变。
- 2026-10-15：用户补充“op如果有新的relay/其他新功能需要监控的，可以额外增加监控，不必强制对齐”。将最终原则明确为“二开作为共同能力基线，OP 按需增量扩展”，允许新增采样及独立指标，不因二开缺少实现而放弃 OP 新功能的必要观测。
- 2026-10-15：按用户 `$to-spec` 请求发布 [spec.md](spec.md)，标记 ready-for-agent。测试主要复用真实请求／任务终态至指标导出及报表查询的高层边界；未创建 tickets、修改业务代码、运行业务测试或提交。

## Answer

- [01 抓取鉴权](issues/01-scrape-authentication.md)、[02 最终结果与报表](issues/02-final-outcomes-and-reports.md)、[03 错误分类与保护](issues/03-final-errors-and-protection.md)、[04 成功分布](issues/04-success-distributions.md) 均 resolved。
- 核心实现：`middleware/prometheus_metrics_auth.go`、`middleware/rate-limit.go`、`pkg/perf_metrics/outcome.go`、`pkg/perf_metrics/model_requests.go`、`pkg/perf_metrics/model_performance.go`、`pkg/perf_metrics/channel_prometheus.go`。OP 适配为 `relay/responses_websocket.go`、`service/task_polling.go`；环境变量示例见 `.env.example`。
- 保留 OP 的观测覆盖和既有 API／权限；未部署外部监控，未处理历史，未改变计费、重试、槽位或渠道治理。原有十份未跟踪迁移研究笔记不纳入本次提交。

## Standards

- 初审发现三项文档／测试规范问题：夹具未显式固定配置与预算、四张票缺 `Type:`、01 缺 `## Answer`；复核另发现任务固定模型名在 `-count=2` 时保留旧计数。均已修复，并补充认证内存限流来源地址的重复执行隔离。
- 最终复核原文：All previously reported Standards findings are resolved. No remaining material documented-standard breaches or optional baseline-smell findings in this bounded recheck.

## Spec

- 初审 P1：缓冲 Responses SSE 转 Chat／Claude JSON 的成功操作没有传输结束标记，曾误判失败。修复为仅流式操作要求该标记，仍保留软错及协议终态约束；现有转换测试观察公共分类接口。
- 初审 P2：单视频重复终态轮询可能绕过 CAS 后再次调用账务收尾而重复采样。将观测置于 batch／individual／poll-failure 的原有 CAS 赢家分支，排除之前已终态的任务；保留原账务调用及条件。公开 `UpdateVideoTasks` 回归验证非终态、首次成功／失败、同对象重复和旧快照竞争。
- 最终复核原文：Both findings are resolved in the staged changes. No remaining concrete Spec issue found in this bounded recheck. 无实质范围外行为。

评审汇总：Standards 初审／复核共 4 项，最主要为夹具状态隔离；Spec 共 2 项，最高为 P1 缓冲成功误判。两轴均已解决，遗留各 0 项。

## Validation

- 环境：Go 1.25.1；真实 SQLite 3.50.4、MySQL 8.0.46、PostgreSQL 15.19。无 schema 变更；测试仅使用临时 SQLite 文件和隔离的 `codex_prometheus_20261016` 数据库。
- `go test ./...`：最终 43 个有测试的根模块包通过，包含 HTTP／WebSocket、恢复／重试、Codex 转换、任务结算与健康报表回归；初次全量发现的两条旧 Codex ignored 断言已改为新失败口径后复测。最终日志 `/tmp/prometheus-full-suite-final.log`。
- `go build ./...`、`cd relaykit && GOWORK=off go build ./...`、`cd relaykit && GOWORK=off go test ./...`：通过，独立模块日志 `/tmp/prometheus-build-relaykit.log`。
- 配置隔离 `TEST_PERF_MYSQL_DSN`、`TEST_PERF_POSTGRES_DSN` 后，`go test ./pkg/perf_metrics -run TestPerformanceAggregationAndFlush -v -count=1`：SQLite／MySQL／PostgreSQL 三项均通过，无跳过；覆盖加权聚合、权限范围、重复刷新、热桶／持久健康窗口及同批 90% 成功率。日志 `/tmp/prometheus-db-matrix.log`。
- 同样配置三数据库后执行 `go test -race ./pkg/perf_metrics ./middleware ./service -run 'Test(FinalModel|FinalStream|ModelPerformance|ModelMetric|ModelHistogram|Successful|InvalidTimings|Disabling|PerformanceAggregation|Prometheus|IndividualTaskPolling|UpdateBatchTasksSettlesTieredUsage)' -count=2`：三个包通过，无 race；日志 `/tmp/prometheus-final-race-reviewed.log`。
- `PROMETHEUS_MAX_SERIES_PER_SCRAPE=1 PROMETHEUS_MAX_MODEL_HISTOGRAM_MODELS=1 go test ./pkg/perf_metrics ./controller -run 'TestFinalModelRequests|TestModelPerformance|TestSuccessful|TestPerformanceAggregationAndFlush|TestResponsesHTTPHealthCountsFinalResult|TestResponsesWebSocketReusesConnectionAndSettlesEachRequest' -count=1`：通过，确认测试不依赖外部预算环境。日志 `/tmp/prometheus-fixture-isolation.log`。
- `go test ./pkg/perf_metrics ./middleware ./service -run 'TestFinalModel|TestModelPerformance|TestPrometheus|TestIndividualTaskPolling|TestUpdateBatchTasksSettlesTieredUsage' -count=2`：通过，确认进程内重复执行隔离。日志 `/tmp/prometheus-repeat-final.log`。
- OWASP 参考：ASVS 稳定版 5.0.0、Authentication／Session Management Cheat Sheets；已验证专用 Token、恒定时间比较、错误凭据限流及 Redis 失败关闭、直连 loopback／转发头边界、响应不泄露凭据。远程传输保护及本机反向代理路径保护仍由部署侧负责，不宣称完成全 ASVS 或生产 TLS／采集器联调。
