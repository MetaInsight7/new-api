# Prometheus 观测基线对齐与 OP 增量覆盖

Status: resolved

Type: task

Branch: tokendazi_op

Created: 2026-10-15

OP baseline: 51f66111b17ec4f2fe1a3231f00a826151917075

Solve baseline: 762ce21c429a3aa9adc104bcd80eb7df3ad3bdbd

## Problem Statement

OP 已有 `/metrics`、渠道尝试及渠道×模型的指标，也有真实请求驱动的健康／性能报表，但缺少二开基线中的模型级最终请求计数、成功耗时／首响应分布、流式成功计数、最终错误分类及相应导出保护。管理员无法直接获得与二开一致的模型观测契约。

OP 现有报表会忽略部分客户端取消和业务拒绝，而用户要求 Prometheus 与健康／性能报表共同采用二开的请求结果口径。只新增几个同名指标、保留旧报表分类，会留下两套有意不同的成功率定义，不符合本轮目标。

OP 同时具有二开基线没有的 Responses WebSocket 逐请求统计、异步任务终态统计和异常退出采样兜底。机械复制二开的采样位置会丢失 OP 已有覆盖。用户要求二开作为共同能力的基线，而不是 OP 监控能力的上限；新的 relay 或其他新功能如有必要，可以额外增加监控。

## Solution

在 OP 现有观测体系中补齐二开的模型指标和抓取鉴权增量，直接调整不一致的旧统计实现，让共同的请求指标与健康／性能报表消费同一套最终成败判定，同时保留和适配 OP 自有能力。

- 保持既有 `/metrics` 和渠道观测能力，增加模型指标、固定错误分类及资源保护。
- 按请求最终结果采样，不把渠道尝试次数、重试次数、流事件数或任务轮询次数当作最终请求数。
- 健康／性能报表也切换为二开口径，不保留旧忽略分类的兼容开关或双轨计算。
- 保留 Responses WebSocket、异步任务终态、异常退出兜底；OP 新能力可以补充采样或增加语义明确的独立指标。
- 当前为测试环境，不开发历史数据处理；不部署 Prometheus、Grafana 或作者专属云端采集设施。

## User Stories

1. As an operator, I want the existing metrics endpoint retained, so that adding model monitoring does not require changing my scrape target.
2. As an operator, I want final model request counts separated into success and error, so that I can calculate request-result success rates.
3. As an operator, I want model labels to use the requested original model name, so that upstream model mappings do not fragment my logical model view.
4. As an operator, I want channel attempts（渠道尝试）distinguished from final request outcomes（请求最终结果）, so that a retry does not look like another client request.
5. As an operator, I want a request that fails on one channel and succeeds on another counted as one final success, so that retry recovery is represented correctly.
6. As an operator, I want a final timeout counted once as an error, so that multiple failed attempts do not multiply model failures.
7. As an operator, I want sampled client cancellations counted according to the newer baseline, so that model metrics and internal reports use the same definition.
8. As an operator, I want sampled upstream parameter rejections counted as final errors, so that old ignore rules do not silently survive in one report.
9. As an operator, I want gateway validation failures distinguished from sampled model calls, so that rejected requests are not invented as upstream executions.
10. As an operator, I want streaming success determined from the final stream outcome, so that an HTTP 200 followed by a broken stream is not reported as success.
11. As an operator, I want normal token-limit completion distinguished from an abnormal stream termination, so that legitimate bounded answers are not treated as transport failures.
12. As an operator, I want a successful model-duration histogram, so that I can observe latency distributions using the same buckets as the reference implementation.
13. As an operator, I want a successful streaming first-response histogram, so that I can observe first-response latency without mixing failed streams into that distribution.
14. As an operator, I want the meaning of first response stated accurately, so that I do not mistake a first accepted data event for a guaranteed first semantic text token.
15. As an operator, I want keepalive traffic excluded from model-response timing, so that gateway heartbeats do not make an idle upstream appear responsive.
16. As an operator, I want absent or invalid timestamps handled explicitly, so that missing observations do not become fabricated zero-latency samples.
17. As an operator, I want successful streaming requests counted separately, so that missing TTFT observations do not hide successful stream volume.
18. As an operator, I want bounded final-error categories, so that I can distinguish timeouts, cancellation, authentication, rate limits and upstream failures without using raw messages as labels.
19. As an operator, I want the existing channel and channel-model metrics preserved, so that current dashboards retain their established meaning.
20. As an operator, I want a model-performance admission limit, so that new histogram groups do not grow without the configured protection.
21. As an operator, I want idle metric groups retired, so that inactive model and channel combinations do not remain in memory indefinitely.
22. As an operator, I want a total scrape-series budget, so that adding model histograms does not silently bypass the endpoint's resource protection.
23. As an operator, I want model performance families exported as complete groups, so that a scrape does not contain a partial histogram.
24. As an operator, I want admission and dropped-observation metadata, so that resource protection is visible rather than mistaken for a lack of traffic.
25. As an operator, I want the limits of dropped-observation accounting documented, so that I do not treat those counters as a complete loss ledger.
26. As an operator, I want remote scraping protected by a dedicated monitoring token, so that model API credentials do not have to be reused for monitoring.
27. As an operator, I want loopback handling based on the direct network peer, so that a caller cannot obtain the exemption merely by sending a forged forwarding header.
28. As an operator, I want a missing server-side monitoring token reported as unavailable, so that deployment misconfiguration is distinguishable from an invalid supplied credential.
29. As an operator, I want invalid monitoring credentials rate-limited, so that repeated unauthorized access receives the reference implementation's additional protection.
30. As an operator, I want valid scrapes excluded from the authentication-failure quota, so that a legitimate collector is not penalized by the failure limiter.
31. As an operator, I want authentication failure limiting to work with the project's Redis and memory modes, so that a missing Redis client cannot cause a panic or an authorization bypass.
32. As an operator, I want model metrics and health/performance reports to share the final success decision, so that a cancellation is not an error in one and deliberately ignored in another.
33. As an operator, I want passive health reports to keep using the application's database, so that these reports work without deploying Prometheus or Grafana.
34. As an operator, I want report windows and data-cutoff metadata preserved accurately, so that delayed database data is not mistaken for a different success definition.
35. As an operator, I want aggregate success rates weighted by request counts, so that a small group does not carry the same weight as a busy group.
36. As an operator, I want health thresholds and insufficient-sample handling aligned, so that low traffic is not presented as proven health.
37. As an authorized report viewer, I want OP's existing group scope and access controls retained, so that aligning statistics does not expose other groups or require a different frontend.
38. As an operator, I want each Responses WebSocket request counted independently, so that reusing a connection does not collapse multiple model operations into one observation.
39. As an operator, I want asynchronous tasks sampled at their terminal transition, so that successful submission is not mistaken for successful completion.
40. As an operator, I want repeated task polls and competing workers not to duplicate terminal observations, so that task volume and success rates remain meaningful.
41. As an operator, I want request-level abnormal-exit sampling retained, so that replacing the telemetry baseline does not hide failures already observed by OP.
42. As an operator, I want new OP relays or features monitored when their behavior requires it, so that the reference implementation's smaller feature set does not limit OP's observability.
43. As an operator, I want OP-specific metrics to have explicit observation units and bounded labels, so that an extension does not corrupt shared request counts or expose secrets.
44. As an operator, I want the existing performance-collection switch respected by every new metric, so that disabling collection stops new observations consistently.
45. As an operator, I want monitoring changes to leave retry, billing, concurrency and channel governance unchanged, so that observation does not become a new business-control path.
46. As a maintainer, I want existing incorrect telemetry logic changed directly, so that obsolete classifications do not become permanent compatibility code.
47. As a maintainer working in the test environment, I want no historical backfill or transition mechanism, so that this migration focuses on the current implementation rather than unused history.
48. As a maintainer, I want reproducible tests through existing request/task and observation interfaces, so that the feature is verified without building a new monitoring infrastructure or testing private map layouts.

## Implementation Decisions

### 1. 基线、模块与接口边界

- 本轮以固定二开版本作为共同模型指标、抓取鉴权增量和健康／性能统计行为的参考，不整体合并二开业务、前端或导出器。
- 修改范围围绕最终结果采集、性能聚合与报表、Prometheus 聚合／导出、抓取鉴权及必要的请求／任务收尾适配。保持直接的模块边界，不新增一层通用监控框架。
- 优先复用 OP 已有请求结束边界和任务终态边界，形成一次逻辑操作的一份最终观测，再供共同请求指标与报表消费；不为了源码挂点一致重新回到多处结算重复采样。
- 观测属于宿主网关职责，不向独立协议转换模块引入数据库、Prometheus 或认证依赖。
- 保留 OP 已有 API 入口、权限、分组域及必要的响应形状。二开的健康 API 前缀与 OP 不同，不为路径字面一致新增别名路由，也不迁入其官网专用分组限制。
- 旧统计实现如与目标不符直接修正，不保留旧分类开关、旧算法回退或并行旧聚合链。已经正确的渠道观测和其他功能不做无关重写。

### 2. 请求最终结果与采样覆盖

- 采集总开关开启且原始模型名非空时，符合边界的最终操作进入统计；这不是随机抽样。关闭开关不再新增相应模型指标或报表样本，不因此清空已有数据。
- 共同 relay 的最终成功要求处理成功且没有最终错误；流式请求存在终态记录时，还要求正常结束且没有流内软错误。HTTP 状态、已发送部分内容或已产生结算均不能单独证明成功。
- 客户端取消、最终上游参数／上下文长度／内容拒绝，只要进入最终模型采样边界，就按失败计入总数和成功率，不继续使用 OP 旧忽略分类。
- 网关前置解析／参数校验直接返回、不经过对应最终模型采样边界的请求不补造模型执行样本。不能将“取消属于错误类别”解释为所有阶段的取消都必然产生模型样本。
- 已正常完成且没有软错的 token 上限结束可以成功；异常流结束、超时、客户端断开或流内错误不能仅因外层已返回 200 而成功。
- 模型请求计数以请求最终结果为单位；渠道尝试另有自己的统计单位。A 渠道失败、B 渠道成功时，最终模型成功一次，渠道尝试仍按既有定义记录。
- 同渠道加密推理有损续接等内部恢复不额外制造一次最终请求失败；最终操作结束后采样一次，不改变恢复预算或 ADR 中的治理边界。
- Responses WebSocket 继续按每次逻辑请求结束采样，不按长连接、数据帧或控制消息数计数；连接复用、取消及错误交错不能造成重复终态样本。
- 异步任务继续在终态状态转换的原有原子竞争成功后采样一次，以任务最终状态判成败。提交成功和轮询成功都不等于任务完成，不在它们与终态之间重复累计同一个最终操作。
- 异步耗时采用任务端到端时间；不伪造 TTFT，也不在缺少上游输出量／生成时长时制造吞吐率。错误类型无法可靠映射时归入有界兜底类别，不把任务错误原文变成标签。
- 保留异常退出／panic 转失败的既有请求级兜底，仍遵守一次最终操作只采样一次，不改变异常响应和原有诊断行为。

| 场景 | 共同请求指标及健康／性能报表的成败定义 |
| --- | --- |
| 普通请求完成 | 成功一次 |
| A 失败，B 成功 | 最终成功一次，不按尝试放大请求数 |
| 多次尝试后最终超时 | 最终失败一次，错误类别 timeout |
| 已进入最终采样的客户端取消 | 失败一次，可归 client_cancel |
| 最终上游参数 400／上下文拒绝 | 失败一次，按最终错误分类 |
| 网关前置校验直接返回 | 不产生该次模型调用样本 |
| 正常 token 上限结束且无软错 | 成功一次 |
| 已返回 200，但流异常结束或带软错 | 失败一次 |
| 加密推理拒绝后内部恢复成功 | 最终成功一次，不额外记中间失败 |
| 异步任务终态 CAS 重复竞争／重复轮询 | 仅成功提交该终态的一次观测生效 |

### 3. 公共 Prometheus 指标契约

模型标签使用逻辑请求的原始模型名，不改为上游映射名。共同指标不新增请求 ID、用户 ID、API Key、错误原文或渠道名称标签。

| 指标 | 类型及标签 | 采集含义 |
| --- | --- | --- |
| `newapi_model_requests_total` | counter，`model,status` | 已采样最终操作；status 仅 success／error |
| `newapi_model_request_duration_seconds` | histogram，`model,le` | 最终成功操作且耗时有效的时延分布 |
| `newapi_model_ttft_seconds` | histogram，`model,le` | 最终成功流式操作且首响应时序有效的分布 |
| `newapi_model_stream_success_total` | counter，`model` | 最终成功的流式操作，不要求一定有 TTFT 样本 |
| `newapi_model_errors_total` | counter，`model,error_category` | 最终失败的有界错误分类，受模型性能组准入／导出保护 |
| `newapi_model_histogram_active_models` | gauge，无业务标签 | 当前存活的模型性能组数量，不等于本次成功导出的模型数 |
| `newapi_model_histogram_dropped_samples_total` | counter，`reason` | 二开既有准入／无效观测／导出预算丢弃口径 |
| `newapi_perf_metrics_series` | gauge，沿用既有契约 | 本次应用指标导出的序列计数口径，与导出预算保持一致 |

- 耗时 histogram 的有限秒桶为 0.25、0.5、1、2、3、5、10、15、30、60、120、300、600。
- TTFT histogram 的有限秒桶为 0.1、0.25、0.5、1、2、3、5、10、15、30、60。
- 两类 histogram 均输出正确的累计 bucket、`+Inf`、sum、count，时间单位为秒；模型标签正确转义双引号、反斜线和换行，输出合法 Prometheus 文本。
- 最终错误类别固定为 client_cancel、timeout、rate_limit、auth、bad_response、network、upstream_4xx、upstream_5xx、other。分类优先级采用二开：取消／超时等明确语义先于泛化的 HTTP 类别；401／403／无效渠道凭据归 auth，429 归 rate_limit。只有流终态信息时也按其取消、超时或异常结尾分类；失败不得使用 none 类别。
- 现有渠道尝试、渠道耗时、渠道首响应和渠道×模型 Token 指标保持其既有名称、标签与业务含义。新增最终模型指标不重新解释这些指标，也不重复记录原有尝试。

### 4. 耗时、首响应与采集精度

- 模型耗时从逻辑操作既有起点到最终结束，包含该操作内部等待、重试或恢复；渠道耗时仍以相应渠道尝试为边界。异步任务沿用其端到端时间精度，不伪装为同步 HTTP 延迟。
- Prometheus 两种模型 histogram 只采最终成功结果。失败仍有请求计数及可用的错误分类，不混入成功分布。
- 没有有效起始时间、非正或不合法耗时时，不伪造零耗时 histogram；记录 invalid_latency 丢弃原因。已有首响应时间但顺序无效时，不写入 TTFT，并按二开记录 invalid_ttft。
- 尚未出现首响应与出现了无效首响应不是同一事实；不得为了补齐数据制造 TTFT。TTFT 不能超出本次操作时序范围。
- TTFT 沿用实际协议采集到的首个有效响应／data 事件，不承诺所有协议均等于首个语义文本 token。网关主动保活和 SSE 注释不得伪造上游首响应、成功样本或新的请求。
- 健康／性能报表的平均耗时及 TTFT 按二开报表样本聚合，可包含失败样本；不为了与成功 histogram 数值相同而另改其过滤口径。共享的是最终成败定义，不是所有指标的分母。

### 5. 准入、回收、预算与元监控

| 配置／生命周期 | 基线行为 |
| --- | --- |
| `PROMETHEUS_MAX_MODEL_HISTOGRAM_MODELS` | 默认 50 个模型性能组；非正值取消该上限 |
| 模型性能组空闲回收 | 空闲 1 小时回收；错误分类和流式成功计数也属于该组 |
| 模型请求 counter 空闲回收 | 空闲 6 小时回收；与 50 个性能组上限不是同一保护 |
| `PROMETHEUS_MAX_SERIES_PER_SCRAPE` | 默认 5000 个应用序列；非正值取消该上限 |

- 模型性能准入在并发下遵守正数上限，回收与记录竞争不能永久丢弃后续正常观测，也不能让已退休对象继续承担新序列。
- 抓取时基础序列优先，包含既有渠道指标及模型请求计数。基础序列已超预算时返回抓取失败，不返回看似完整的成功响应。
- 为模型组元监控预留 5 个序列；其余预算按二开最近活跃优先策略选择完整模型性能组，不截断单个 histogram 或拆碎同一模型的性能组。
- 若基础指标后连元监控预算也不足，模型性能组及相关元监控可能不导出；不宣称这些健康指标无条件可见。
- reason 固定为 model_limit、invalid_latency、invalid_ttft、series_limit。保留二开计数含义：series_limit 累计未导出 latency／TTFT observation 的新增量，不是所有被省略的请求、错误或流式成功的完整计数；没有新增观测的重复抓取不得重复放大同一批丢弃量。
- 模型性能组准入失败不阻止该操作继续进入正常请求 counter 与报表聚合。不能声称最多 50 个模型意味着所有模型 counter 的基数也被限制为 50。
- 模型指标为进程内累计，重启和空闲退休后可以重置；不是持久账本，也不保证不同保留周期的 counter／histogram 永久相等。
- 复用已有性能采集开关与配置，默认采集开启、默认按小时聚合并每 5 分钟刷新已完成桶；不新增一个与共同采集开关冲突的模型采集开关。

### 6. 健康／性能报表对齐

- 共同报表样本与模型请求指标使用同一最终成败判定；取消、最终上游参数拒绝不再仅在旧报表中被排除。报表元数据、说明及缓存语义不得继续宣称被淘汰的分类规则。
- 性能报表继续合并数据库已落库聚合与当前节点热桶；健康报表继续只读共享数据库，并排除尚未完成或未经过安全刷新时间的桶。不新增对 Prometheus、Grafana 或实时 Redis 查询的依赖。
- 健康安全截止使用桶结束后至少一个刷新间隔再加 60 秒的规则。默认小时桶、5 分钟刷新时，不能把当前热桶直接当作稳定健康历史。
- 健康窗口支持 24、168、720 小时；性能窗口默认 24 小时、最大 720 小时，窗口选择、桶归属与聚合行为参照二开，并如实维护 OP 已公开的窗口元数据。
- 成功率由成功数除以样本总数计算；跨分组／跨桶汇总以原始计数加权，不对百分比作简单平均。吞吐率及其他平均值按对应累计量和有效分母计算，不凭缺省零值制造观测。
- 健康样本不足 20 次为 insufficient；达到样本下限后，成功率至少 99.9% 为 healthy、至少 99% 为 watch、更低为 degraded。无有效样本不制造 100% 健康结论。
- OP 的既有 `/api/perf-metrics`、`/api/perf-metrics/summary`、`/api/model-health` 及详情入口继续服务现有消费者，保留权限、允许的分组范围和必要响应字段。统计对齐不开放额外管理权限，也不照搬二开的官网特定分组域。
- 真实请求驱动的被动健康统计与主动模型探测状态是两套观测；本轮不增加模型复测，也不将主动探测的成功结果混入真实请求成功率。
- 同一采样范围、无保护丢样且时间窗口一致时，两类系统具有相同的成功率定义；进程范围、落库延迟、热桶和保留窗口不同，仍可导致展示数值不同。不能用强行补数解决这一正常差异。

### 7. 抓取鉴权与失败限流

- 复用专用环境变量 `PROMETHEUS_METRICS_TOKEN`，不复用上游模型 Key、用户登录会话或用户 API Key，也不新增配置管理 UI。
- 本机豁免依据直接连接的 RemoteAddr 是否为 IPv4／IPv6 loopback，不依据任意转发头。保留二开这一信任边界，不把本机反向代理等同于已验证最终客户端。
- 非本机访问的服务端 Token 未配置／为空时返回 503；配置正常但缺失、格式错误或不匹配的 Bearer 凭据返回 401，且只有这一失败分支消耗专门的失败限流。
- 超过失败限额返回现有限流器约定的 429，不进入指标导出；正确凭据不消耗失败配额，即使失败配额已耗尽也不因此阻断合法抓取。
- `PROMETHEUS_METRICS_RATE_LIMIT` 默认 60，`PROMETHEUS_METRICS_RATE_LIMIT_DURATION` 默认 60 秒；非正值回落默认值，不按指标预算配置的规则解释成无限制。
- 复用独立的鉴权失败限流命名空间和项目现有来源地址／可信代理策略，不与通用 API 或模型成功次数配额混用。支持现有 Redis 与内存模式，Redis 已启用但客户端未初始化时采用二开的内存兜底，不空指针崩溃或跳过认证。
- Token 比较保留恒定时间实现；凭据不进入指标标签、错误响应或日志。限流存储异常不得让错误凭据获得指标。远程 Token 应通过受保护传输使用，不在本轮引入新的 TLS 终止设施。
- 安全实施遵循适用的 OWASP ASVS 与认证／会话管理指导；不以对齐二开为理由移除已有安全保护。

### 8. OP 增量与非侵入性

- 二开是共同能力的基线，而不是监控上限。OP 新 relay 能复用一次逻辑请求终态时，纳入同一模型观测；确有独立生命周期或观测对象的功能可增加独立指标，并明确单位、终态、标签与保护策略。
- 本轮明确必须保留并接通的 OP 增量为 Responses WebSocket 逐请求、异步任务终态及异常退出兜底。实施中发现其他具体 relay 采样缺口时可以在同一观测边界补齐，并记录对应外部行为与回归用例；不要求为了“可扩展”提前建设全功能指标框架。
- 不把 Responses WebSocket 等同于 Realtime。既有 Realtime 成功统计并不完整；若需要新的会话级观测，应按实际语义独立设计，不能把帧数或长连接简单伪装成共同请求数。
- 不因二开包含某个业务专属指标家族就迁入 OP 尚不存在的业务功能；同样不因二开没有某个 OP 新功能就禁止补充其监控。
- 监控不得改变重试资格、错误保真、路由、渠道并发槽位、计费／退款、任务终态 CAS、自动禁用／恢复或冷却策略。观测快照不能成为权威槽位，继续遵守相关 ADR。
- 当前测试环境的历史数据不纳入交付：不做回填、重算、切换时间标记、双口径过渡或历史兼容机制，也不由此推导自动清库。

## Testing Decisions

### 测试边界与既有先例

- 主要高层测试边界是“真实请求处理／任务终态事件 → 对外观测输出”：使用本地可控假上游和既有请求／任务测试设施完成操作，再检查 `/metrics` 文本及性能／健康查询结果。指标和报表不是两套互不关联的成功判定测试。
- 优先使用现有 HTTP handler、WebSocket 请求流程、任务轮询终态流程以及公开采样／导出／查询入口，不为每个计数器、存储容器或调用层另造 mock 接口。健康报表通过真实测试数据库和明确桶时间验证，不靠在测试中复制聚合算法。
- 现有 Prometheus 导出与标签转义测试、最终结果／流式分类测试、任务终态聚合与落库测试、模型健康分级测试、Responses HTTP／WebSocket 重试和取消回归，以及任务重复轮询测试均为可复用先例。应修改旧“取消／业务拒绝不计健康”断言，不能为了旧测试继续通过而保留旧规则。
- 现有性能聚合测试已提供 SQLite 及 MySQL／PostgreSQL 的测试连接入口，应优先扩展该体系；不因本轮跨采集、导出、controller 等多层就新增多份重复测试文件。
- 只测试外部可观察行为、输出契约和真实不变量。时间、环境变量、全局采集开关与数据库状态使用隔离夹具；避免 sleeps、随机压力循环、私有常量值／map 布局断言或以实现文件名作为验收。
- 准入／回收等局部并发场景可在现有采样与导出边界定向测试。若确需控制时钟，优先复用现有时间输入；必要时仅在采集模块边界提供最小的时间依赖控制，不把每个内部 helper 都变成新测试接口。

### 验收矩阵

| 编号 | 场景与必须观察到的行为 |
| --- | --- |
| T01 | 成功、最终失败输入后，模型请求 status、完整 histogram、sum／count、成功流式计数及错误分类符合公开契约；时间单位和标签转义正确。 |
| T02 | A 失败 B 成功、最终重试失败、同渠道有损恢复成功分别只产生一次最终结果；渠道尝试独立保留，不改变已有重试和恢复行为。 |
| T03 | 9 次成功加 1 次进入最终打点的取消，稳定无丢样范围内请求成功率为 90%；将同一批样本落入健康可读窗口后，报表也为 90%，不能是旧口径的 100%。 |
| T04 | 最终上游参数 400／上下文拒绝计失败；网关前置解析／校验早退不制造模型样本；401／403、429、超时、网络和坏响应按优先级分类。 |
| T05 | 正常 token 上限结束可成功；HTTP 200 后异常流、软错、客户端断开、首响应超时不能误成功；保活不伪造首响应。 |
| T06 | 失败不进入成功 histogram；成功但缺失／无效时间仍保留成功请求／成功流式计数，缺失观测不伪造零值，丢弃原因按契约累计。 |
| T07 | 一条 Responses WebSocket 连接上的多个请求、取消及错误交错按逻辑请求分别终结且仅记一次；输出／结算的既有回归不变。 |
| T08 | 异步提交和非终态轮询不产生最终完成样本；成功／失败终态只计一次，竞争／重复轮询不重复，端到端耗时正确且没有伪造 TTFT／吞吐。 |
| T09 | 异常退出兜底仍产生一次失败观测，不与正常收尾重复，也不隐藏原有异常响应。 |
| T10 | 正数模型性能组上限在并发准入下生效；已有组能继续记录，空闲退休与并发记录竞争不使后续样本永久失效；请求 counter／报表不被性能组拒绝连带丢弃。 |
| T11 | 总预算按基础优先、最近活跃模型整组导出；不输出半个 histogram；基础超限失败；不足元监控预算时遵守参考退让行为。 |
| T12 | model_limit、invalid_latency、invalid_ttft、series_limit 各按其含义变化；没有新观测的重复受限抓取不把同批 observation 重复计算为新丢弃。 |
| T13 | 默认值、非正值语义、空闲退休／重启可重置累计值符合契约；关闭采集后所有新增共同模型样本停止，已有数据不因此被清空。 |
| T14 | 性能报表数据库＋当前节点热桶、健康报表仅数据库安全窗口及 24／168／720 小时校验均正确；各分组样本量显著不同时验证计数加权和 OP 分组权限不被绕过。 |
| T15 | 健康样本 19／20 次、99%／99.9% 边界正确；无数据不伪造健康；报表说明／元数据与新口径一致。 |
| T16 | loopback IPv4／IPv6、非本机正确／缺失／错误／畸形 Bearer、服务端未配置 Token、伪造转发头，分别验证放行边界及 401／503，响应不泄露凭据。 |
| T17 | 失败配额耗尽得到 429，合法 Token 仍可抓取且不消耗失败配额；Redis／内存及 Redis 客户端未初始化场景不崩溃、不放行错误凭据，不占用模型 API 配额。 |
| T18 | 既有渠道及渠道×模型输出、宿主 API 接口和权限保持兼容；新增 OP 观测若有独立单位，需有对应契约测试，不误并入共同请求数。 |

- 涉及报表数据库读写、过滤／聚合、模型字段或其他数据库行为改动时，必须使用真实 SQLite、MySQL、PostgreSQL 运行相应集成验证；测试入口因缺 DSN 而跳过不能替代三数据库验证。记录实际版本、命令和结果；涉及最低版本特性时覆盖仓库支持的最低版本。
- 本设计不预设新增 schema。实施若确需改 schema／迁移，必须按仓库规则验证新库、代表性旧库升级及重复启动幂等性；“不处理历史统计”不豁免数据库安全验证。
- 对采样、准入、退休及抓取的共享状态运行有针对性的并发／race 验证。不得把一次跑通或大循环随机压测冒充稳定性证明。
- 本阶段仅发布 spec，不执行业务测试；实施验收应记录实际运行结果，不预填通过。真实线上 collector、云监控与 Grafana 联调不属于本轮完成条件。

## Out of Scope

- 部署或启动 Prometheus、Grafana、GCP sidecar／托管采集、云端数据源、看板 provisioning 或整套监控基础设施。
- static 前端迁移、页面重做、监控配置 UI 新建或将 API 路径改成二开前缀以追求源码一致。
- 历史回算、回填、口径切换时间标记、历史兼容、双轨过渡、自动清库或为了历史重建新增事件账本。
- 保留旧取消／业务拒绝忽略规则的兼容配置，或者继续维护两套共同请求成败定义。
- 将采样开关、统计失败或元监控结果当作路由、重试、计费、权威并发准入、禁用／冷却的控制信号。
- 改变主动渠道／模型探测执行方式，新增一次测试，或把主动探测结果混进真实请求报表。
- 迁入 OP 没有的二开业务功能，只为了获得其业务专属监控家族。
- 在没有明确生命周期定义的情况下建设完整 Realtime 帧级／会话级账本，或宣称 Responses WebSocket 覆盖等于完整 Realtime 覆盖。OP 如发现具体观测需求可增加边界清晰的独立指标，不要求与二开机械一致。
- 持久、全入口、全实例、永不丢样的请求审计账本；或承诺 histogram 模型上限覆盖全部 counter 基数、元监控完整记录全部损失。
- 无关依赖升级、通用监控框架、通知／告警规则系统、上游 request ID 日志增强及其他未由本轮观测缺口要求的功能。
- 自动提交、推送或创建 Git 分支。

## Further Notes

- 这是用户已确认的访谈结论综合，不新增需求访谈。最终原则为“二开作为共同能力基线，OP 按需增量扩展”，不是逐文件覆盖，也不是放弃用户要求的健康／性能口径统一。
- 二开参考限于可访问镜像的固定提交，不据此推断作者线上部署、私有配置或后续提交。实施前若 HEAD 有变化，应重新核对相同观测边界，不静默扩大任务。
- 设计与源码核对记录见 [grilling.md](grilling.md)，决策地图见 [map.md](map.md)。术语采用项目 Domain Glossary 中的“渠道尝试”和“请求最终结果”。
- 遵守既有“观测不能代表权威并发槽位”与“加密推理有损恢复中间拒绝不额外触发健康治理”的 ADR；本轮没有需要另立 ADR 的难逆转架构取舍。
- 已核对 OWASP 项目声明的 ASVS 最新稳定版为 5.0.0，并参考 [Authentication Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Authentication_Cheat_Sheet.html) 与 [Session Management Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html)。适用重点为专用凭据保护、恒定时间比较、失败节流、安全日志与传输边界；spec 发布不等于已完成安全实现或合规验证。
- 本机豁免只识别直连对端；若外部请求经本机反向代理转入，代理自身仍需保护监控路径，不能把该豁免误称为最终客户端鉴权。
- 仅发布到本地 Markdown issue tracker，状态为 ready-for-agent；本轮不拆 tickets、不实施业务代码、不提交。

## Comments

- 2026-10-15：按用户 `$to-spec` 请求发布。Q1 至 Q5 以及测试环境不处理历史、OP 新能力可按需增加监控的补充均已纳入；测试优先复用现有请求／任务终态与对外观测边界。

## Answer

- 2026-10-16：按用户 `$implement` 完成四张实施票，测试及双轴复核通过。实现与验证记录见 [map](map.md)；本次 `$implement` 授权提交当前分支，不推送或创建分支。历史范围及外部监控部署边界不变。
