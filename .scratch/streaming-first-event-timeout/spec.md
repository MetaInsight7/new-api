# 流式首事件超时保护：后台可控并兼容主动保活

Status: resolved

Type: task

Branch: tokendazi_op

Created: 2026-10-15

Implementation: completed on 2026-10-15; see [map.md](map.md), [verification.md](verification.md) and [review.md](review.md)

## Problem Statement

OP 已有主动保活、上游响应头超时和流读取闲置超时，但没有独立的首个上游数据事件等待预算。上游持续发送 SSE 注释、空行或协议心跳时，连接看起来一直存活，实际却可能一直没有开始返回响应数据，长期占用连接及渠道并发槽位。

主动保活又是现有慢模型场景需要的功能：OP 会在等待响应头和读取流期间向客户端发送保活。直接关闭保活、把保活当成模型进展，或者不加区分地套用二开默认的 180 秒，都不符合用户希望兼顾长等待与故障保护的目标。

管理员需要在 OP 管理后台独立开启、关闭和调整首事件保护，明确知道它与主动保活、闲置超时以及正文生成时长的区别，而不是只能修改部署环境变量并重启。

## Solution

在 OP 现有管理后台的模型全局设置中，与主动保活相邻提供“首事件超时保护”和“首事件等待时长（秒）”。保护默认关闭；开启后，每次开始读取上游 SSE 流时启动一次独立倒计时。

OP 发给客户端的保活、上游 SSE 注释、空行、空数据和已明确识别的协议心跳，均不能重置或解除这个倒计时。收到首个非心跳数据事件后立即撤销倒计时，之后继续按原来的流转发、保活和闲置超时规则运行，不限制整个生成过程的总时长。

这里的“首事件”不是首个正文 token：正常响应元数据、角色声明、思考、工具调用和正文数据都可以表示上游已经开始响应。仅凭持续心跳无法判断上游正在正常计算还是卡死，管理员通过可调整的等待预算决定允许等待多久。

若预算耗尽仍未收到首事件，明确记录首事件超时、终止该上游尝试并回收本地资源。是否允许换渠仍由 OP 现有重试策略决定；已经向客户端实际写出保活或其他响应后，不绕过现有的禁止重试保护。

## User Stories

1. As an operator, I want a dedicated first-event timeout switch in the OP administration console, so that I can enable protection without changing deployment files.
2. As an operator, I want first-event protection to be disabled by default, so that upgrading does not unexpectedly interrupt existing slow-model traffic.
3. As an operator, I want to configure the first-event waiting time in seconds, so that the budget matches my upstream latency expectations.
4. As an operator, I want the configured waiting time to survive disabling and re-enabling the switch, so that I do not have to enter it again.
5. As an operator, I want first-event settings to remain independent of keep-alive settings, so that changing one protection does not silently change another.
6. As an operator, I want saved settings to apply to new requests without restarting the handling instance, so that operational changes are straightforward.
7. As an operator, I want an in-flight request to keep its initial policy snapshot, so that a settings change does not unexpectedly shorten its current wait.
8. As an operator, I want settings to survive a service restart, so that the protection does not silently revert.
9. As an operator, I want invalid settings rejected by the server as well as the form, so that direct management requests cannot create unsafe timeouts.
10. As an operator, I want failed saves to remain visible and leave the last valid setting intact, so that a failed update is not presented as successful.
11. As an authorized administrator, I want these settings to use the existing protected management interface, so that ordinary users cannot change gateway-wide behavior.
12. As an API caller, I want OP to continue sending configured keep-alive pings while waiting, so that downstream proxies do not disconnect a legitimate slow request merely because the connection is quiet.
13. As an API caller, I want a local keep-alive ping not to count as an upstream response, so that a stalled upstream cannot appear healthy just because OP is sending pings.
14. As an operator, I want upstream SSE comments and empty lines not to extend the first-event deadline, so that transport activity cannot keep a response-less attempt alive indefinitely.
15. As an operator, I want explicitly recognized application-level heartbeat events not to satisfy the deadline, so that wrapping a heartbeat in a data event does not defeat protection.
16. As an API caller, I want real metadata and role events to satisfy the first-event requirement, so that the protection does not unnecessarily wait for answer text.
17. As an API caller, I want reasoning, tool-call and content events to satisfy the first-event requirement, so that valid non-text response progress is preserved.
18. As an API caller, I want an unfamiliar data event to follow its existing protocol handling, so that heuristic heartbeat detection does not discard valid provider extensions.
19. As an API caller, I want upstream errors and terminal events to keep their existing meaning, so that timeout protection does not mask an actual upstream error or invent a different outcome.
20. As an API caller, I want the first-event deadline removed after the first qualifying event, so that a valid long-running generation is not cut off by its former startup budget.
21. As an operator, I want the existing response-header timeout to keep protecting the header-wait phase, so that first-event protection does not silently change that separate limit.
22. As an operator, I want existing stream-idle protection to continue operating before and after the first event, so that adding one deadline does not remove another.
23. As an API caller, I want a genuine first-event timeout reported as a timeout rather than a successful empty response, so that I can handle the failure correctly.
24. As an operator, I want a distinct first-response timeout reason in existing diagnostics, so that I can distinguish startup silence from idle timeouts and client cancellations.
25. As an API caller, I want safe retries before response commitment to follow the existing retry policy, so that protection does not unnecessarily disable existing fallback.
26. As an API caller, I want OP not to replay an attempt after a ping or other response has been committed, so that response streams are not combined across channels.
27. As an API caller, I want committed streams to avoid appended plain JSON errors or fabricated success markers, so that failure handling remains protocol-consistent.
28. As an operator, I want timeout and client cancellation to close the upstream response and stop local stream workers, so that abandoned attempts do not retain local connections and concurrency slots.
29. As an operator, I want each retry attempt to have its own first-event observation state, so that a previous attempt cannot incorrectly disable protection for the next one.
30. As an API caller, I want existing usage, charging and refund rules preserved, so that changing timeout protection does not introduce a new billing policy.
31. As an operator, I want disabled protection to preserve the old behavior, so that I can roll back the feature through the console.
32. As an administrator using another interface language, I want translated labels and clear validation messages, so that I can configure the feature without confusing heartbeat, first-event and total-generation timeouts.
33. As an operator running multiple instances, I want the feature to use existing option synchronization, so that I do not need a new configuration distribution system.

## Implementation Decisions

### 1. 范围与现有模块

- 只改 OP 宿主管理设置、共享 SSE 流处理和必要的宿主协议适配器；管理后台属于 OP，不是 tokendazi_static。
- 首版覆盖经过宿主共享 SSE 扫描模块的流式路径，包括现有 Chat Completions、Responses、Claude Messages、Gemini、协议桥接及已使用该模块的媒体 SSE。不是对所有传输形式的统一超时改造。
- 不整体替换二开 scanner，不带入其特定供应商的首帧门控，也不重新实现协议转换模块。
- 遵守“渠道并发负载快照仅作为观察提示”和“Responses 加密推理状态仅在拒绝后做一次有损续接”两项既有架构决策，不改变渠道并发租约所有权或有损续接条件。

### 2. 管理设置契约

- 在现有配置注册和选项持久化体系中增加独立的流式设置模块，避免把新策略混入主动保活开关。
- 管理选项采用 `streaming_setting.first_response_timeout_enabled` 与 `streaming_setting.first_response_timeout_seconds`。前者默认 `false`，后者默认预填 `600` 秒；预填时长不代表自动启用，也不是所有模型的推荐值。
- OP 最终采用显式布尔开关控制启用，关闭时保留正整数时长。时长必须至少为 1 秒，且不能超过安全换算为宿主时间类型的秒数上限；拒绝小数、负数、零、非数值和溢出，不静默截断或夹到其他值。
- “设置为 0 关闭”是此前环境变量方案及二开配置的语义；OP 后台方案用独立开关关闭，不允许出现“开关显示开启但时长为 0、实际上不工作”的状态。
- 复用现有受保护的选项读取和更新接口及错误响应格式，不另建公共配置接口、不降低既有管理权限。服务端在写入前校验，不能只依赖前端校验。
- 两个设置可以复用现有逐项保存方式，不承诺整页原子保存。用户同时修改时长并开启保护时，先成功保存有效时长，再开启；前一步失败不继续开启，不显示整次保存成功。
- 持久化失败不得发布新运行时值或假报成功；刷新、选项重载和重启后应回读到最后成功保存的值。沿用现有选项存储，不新增业务表、字段、索引或数据库专属语法。
- 复用配置管理器已有的配置读写锁及校验扩展方式，提供受同步保护的值快照，不能把共享可变指针当作请求快照。
- 首版不增加与后台设置竞争的同名环境变量开关，不从二开默认的 180 秒隐式启用保护；部署级默认值和覆盖优先级不属于本批新增契约。

### 3. 后台交互与生效时机

- 在模型全局设置中紧邻现有主动保活设置展示独立开关和秒数输入，复用现有设置布局、开关、数字输入、表单校验、保存反馈与查询失效机制，不新增另一套表单基础设施。
- 关闭保护时禁用时长输入但保留已配置值；显示说明：此项等待首个上游非心跳数据事件，不等正文，不限制整个生成过程，且不受主动保活续期。
- 文案明确标示两个方向：上游模型服务向 OP 返回事件；OP 向客户端发送保活。不能把 OP 发送 ping 描述成上游模型已响应。
- 入站 Relay 请求建立时读取一份不可变的启用状态与时长快照。本请求的所有重试尝试使用该快照，不因后台中途修改而改变；保存后的新请求读取已生效的新配置。
- 处理保存的实例无需重启。其他实例通过现有选项同步机制加载新值；不承诺跨实例瞬时生效，不增加新的广播或配置中心。
- 新文案沿用项目 i18n、可访问标签、键盘操作和数值显示规则。保存失败保留明确反馈，不把失败请求当作成功回读。

### 4. 计时生命周期

- 倒计时属于本次上游尝试：上游响应头已经返回、共享扫描模块开始读取 SSE 时启动。它不包含入站请求校验、渠道排队、预扣或等待响应头阶段。
- 等待响应头继续由现有响应头超时负责；现有全请求超时和流读取闲置超时保持原语义。多个适用限制同时存在时，先成立的结束条件生效；首事件预算不覆盖或延长其他超时。
- 启用保护时从启动起只计算一次绝对等待预算。OP 下行 ping、上游注释、空行、空数据、已识别的应用层心跳均不能重置它。
- 收到首个符合条件的数据事件时撤销该计时器，不等待正文 token、下游写入完成或生成结束。之后不再启动本次尝试的首事件计时器，主动保活与闲置超时继续照常运行。
- 每次上游尝试建立独立的首事件观察状态；不得用请求历史中的 `FirstResponseTime`、`ReceivedResponseCount` 或“客户端已经写出内容”判断新计时器是否应该启用。
- 计时器到期、首事件到达、上游结束和客户端取消之间必须有一致的终态仲裁。已确认的首事件不能因迟到的计时器通知被误判超时；超时已成立后不能继续转发迟到的数据。
- 关闭保护时不启动新 watchdog，继续保留现有保活、取消和闲置超时行为。正常结束、失败和取消都必须停止并释放本次计时器。

### 5. 首事件与心跳的判定

- 通用层沿用当前 SSE 数据读取契约：注释行、单独的事件描述行、空行和空 `data` 不满足首事件条件；结束标记按现有结束语义处理，不作为正文或等待超时。
- 非心跳的正常非空数据事件满足首事件条件，包含响应创建、角色/模型元数据、思考、工具调用、图片进度和正文，不要求存在非空文本 `content`。
- 应用层心跳必须按真正解析该上游协议的适配器识别，不能依据客户端请求的协议格式、渠道品牌或任意字符串包含 `ping` 来猜测。
- 首版明确覆盖 Claude Messages 的合法 `type=ping` 事件；它即使位于 `data` 中也不满足首事件条件。其他协议只有在宿主适配器已有明确的协议心跳定义时才排除，不泛化为“任何 JSON 对象的 type/event 等于 ping 都是心跳”。
- 优先复用适配器已有的事件解析与共享流处理 Interface，在必要时增加最小的可选事件分类能力。不新增通用语义推断框架，不把所有适配器重写成一套新解析器。
- 心跳仍按原协议决定是否向客户端转发，本特性只改变新 watchdog 的进展判断，不擅自删改上游事件，也不顺带重定义现有 TTFT 统计口径。
- 未明确识别为心跳的数据继续走原有解析和转发。未知但被现有协议接受的数据可以解除首事件等待；畸形数据和明确错误事件仍走现有错误路径，不把它们静默吞掉后换成等待超时。
- 上游错误、正常终态或 EOF 先于首事件截止时，保留其现有意义。不能为了满足本特性把它们伪造为首事件超时；既有正常终态也不额外触发换渠。

### 6. 超时错误、重试与资源清理

- 增加独立的 `first_response_timeout` 流结束原因和稳定的 `upstream_first_response_timeout` 错误标识，明确它指首个非心跳数据事件，而不是首正文 token。
- 在超时成立时记录可识别的超时错误，宿主适配器必须将其传回 Relay。未提交响应时以网关超时状态 504 表达，并恢复普通错误响应所需的内容类型及响应头，不能将 JSON 错误标成正常 SSE、仅停止 scanner 后返回空成功，或者继续执行正常完成的发送逻辑。
- 同步接入既有请求结束记录与性能结果分类，使该结果被识别为超时失败，而不是成功、本地业务拒绝或客户端主动取消。不新增独立的监控产品或统计管线。
- 未向客户端提交响应时，仍按现有预算、状态码配置、会话约束与渠道策略决定是否重试；既不无条件重试，也不一律标记不可重试。
- 只要已经实际写出 OP ping、转发的协议心跳或其他响应，就继续遵守“响应已提交后不再自动重试”的现有规则；不能因为还没有正文就绕过该规则。仅设置尚未提交的 SSE 响应头不等于已经发出保活。
- 已提交的流不能改写 HTTP 状态或追加普通 JSON 错误。复用协议已有的错误终止能力，尽力发送脱敏的流内失败并结束；没有合适错误帧时结束该流，不伪造完成事件或成功的结束标记。
- 超时后停止本地保活，关闭上游响应体，取消本次读取并等待相关工作退出，保证处理器返回后不再访问回收的请求上下文。取消上游连接不等于承诺远端供应商一定立即停止计费或生成。
- 渠道并发租约继续由原有尝试生命周期释放，不依据负载快照或计时器手动重复释放；不能因上游持续心跳而长期保留本地槽位。
- 保留原错误优先级、脱敏、请求关联及 Responses 拒绝后的有损续接规则，不新增自动禁用或冷却策略，不把首事件超时误识别为密文状态拒绝。
- 不修改 Usage 推导、预扣、结算、退款或媒体数量规则。超时作为既有失败路径的输入，不能因未见正文就擅自把有权威依据的上游用量清零，也不能伪造完整响应来收费。

## Testing Decisions

### 测试切口

- **主要行为 seam：现有 Relay HTTP 处理入口与可控的上游 SSE。** 从调用方可观察的输出、上游调用次数、请求结束结果和渠道并发释放验证功能，不为每一个生产文件各建一套测试。
- **必要补充 seam：现有后台设置表单与受保护的选项保存/回读 Interface。** 验证管理员可以控制真实生效的行为，不只测试控件能否渲染。
- 优先扩展现有 Relay 渠道并发 HTTP 集成、已提交响应错误处理、共享 scanner、协议流处理、选项更新与后台设置交互测试。scanner 的补充用例只保护难以在最高 seam 稳定验证的取消、分类和终态竞争，不重复整条请求测试矩阵。
- 若需要可控时间，优先使用现有测试能力或 Go 的可控时间支持；必要的新时间 seam 仅留在流处理模块内部，不向所有业务调用方暴露时钟 Interface。只替换时间、外部上游与存储等不可控依赖，不 mock 被测业务逻辑。
- 好测试断言真实外部契约：是否仍在等待、是否转发事件、是否返回超时、是否发生重发、响应是否混入 JSON/伪成功、请求是否结束及资源是否释放。不以私有计时器是否被调用、内部字段布局或日志文本替代行为验收。

### 核心回归矩阵

1. 默认关闭与后台显式关闭均保持现有行为；等待超过所配置时长后仍可接收正常首事件，但其他既有超时仍可生效。
2. 开启保护并持续接收上游注释心跳、空行和空数据时，即使客户端仍收到 OP 保活，也必须因缺少首事件而超时退出。
3. 仅 OP 主动保活、没有任何上游数据时不能解除新预算；若既有闲置超时更早成立，应保留闲置超时原因而非强改为首事件超时。
4. Claude 原生 ping 循环不能解除新预算；同样 JSON 结构若位于没有该心跳定义的其他上游协议中，不得被通用字符串规则丢弃。
5. 正常元数据、角色声明、思考、工具调用和正文分别可以解除等待，不需要等到文本答案。
6. 首事件在截止前到达后，流继续跨过原首事件截止时间仍可正常结束；保活和首事件之后的闲置超时继续独立工作。
7. 上游响应头阶段不受新预算缩短；开始读 SSE 后才启动本次预算，已有响应头超时仍能独立结束请求。
8. 后台修改不改变已建立请求的快照，包括该请求后续重试；之后新建的请求读取已生效的新值。
9. 未提交响应时，超时返回状态、内容类型和错误结构一致的 504 错误，并按现有可重试状态配置决定是否换渠；新尝试不继承上一尝试的已见首事件状态。
10. 已实际发出保活后超时不再重试，响应不得追加普通 JSON、伪造正常完成或拼接第二条渠道的业务流。
11. 真正上游错误、合法终态或 EOF 先到时不被新计时器覆盖；未知及畸形事件保留现有协议处理语义。
12. 客户端取消、上游断开、首事件与截止通知竞争时，最终只认定一次结束；处理器可以退出，响应体关闭，保活和后台工作停止，不晚写请求上下文。
13. 完整请求在超时和取消后释放自己的渠道并发槽位；后续请求可以正常获取，不需要清理负载快照来恢复容量。
14. 新原因进入既有请求诊断及失败分类，不计为成功；客户端取消仍维持原本的分类，不因共用清理路径被计为上游超时。
15. 至少覆盖一条正常文本流、一条已知应用层心跳协议路径，以及共享 scanner 的媒体流回归；现有正常协议事件顺序和 Usage 契约保持不变，不建立新的计费规则测试体系。

### 后台与持久化验收

- 默认开关关闭、预填时长、开关与输入可访问状态、关闭后保留时长、启用前先保存时长及保存后回读一致。
- 主动保活开关与新开关分别开关时互不修改；用户能通过说明区分首事件、正文和总生成时间。
- 前后端拒绝无效布尔值、零、负数、小数、非数值和超大秒数；直接调用管理接口不能绕过校验，失败不能覆盖最后有效值。
- 复用真实管理权限链验证未授权用户不能读取或修改受保护配置，不只在前端隐藏控件。
- 通过真实 SQLite、MySQL、PostgreSQL 验证新选项保存、回读、启动重载和非法更新不污染旧值；故障注入另验保存失败，不以 mock 替代三库成功路径。
- 多实例通过已有选项同步读取新值；同步失败不产生半初始化或不安全的运行时策略。验证不涉及新跨实例即时广播。
- 后台交互测试沿用现有 Vitest 与 React Testing Library，覆盖字段错误、保存失败、查询回读及文案的 i18n 接入，不用内部 state 或大 DOM 快照代替可见行为。

### 执行与完成门槛

- 扩展合适的既有测试文件；整项后端功能最多新增一个测试文件，不能按控制器、设置、业务层、适配器逐层散建重复测试。允许在现有文件中补不同的真实契约用例。
- 后端使用既有 require/assert 约定；测试显式初始化并恢复配置、数据库、缓存与上下文，禁用并行污染。使用通道握手、可控事件和时间替身，禁止用任意 sleep、随机输入或耗时比较证明正确。
- 实施时先跑定向回归，再按影响范围验证宿主测试与 vet；触及独立协议模块时额外验证其脱离工作区的独立构建与测试，不能用宿主成功代替。
- 后台改动必须通过受影响测试、类型检查、涉及文件的 lint/格式检查以及生产构建。持久化相关改动完成前必须执行真实三数据库验证并记录版本、命令、结果及跳过情况。
- 本次交付仅为 spec，以上均为实施验收要求，不代表测试已运行或功能已完成。

## Out of Scope

- tokendazi_static、广告运营、支付、内容产品、其他供应商接入及本批之外的运维迁移项。
- 修改主动保活的开关、间隔、发送方向或通用调度机制；关闭保活以规避本功能。
- 首个正文 token 超时、整个生成时长上限、首事件之后长期只有心跳的业务进展监控，或凭心跳判断远端真实计算状态。
- 渠道级、模型级、用户分组级超时覆盖与自动学习预算；首版只提供全局后台策略，混合快慢模型时管理员选择兼容的全局预算或关闭。
- 修改原有响应头超时、全请求超时和闲置超时语义，或承诺开启首事件保护能够绕过更早成立的其他限制。
- 允许“只发过 ping、尚未有正文”时跨渠道透明重试；改变会话约束、重试次数、自动禁用、冷却或 Responses 有损续接策略。
- 全面重写 SSE 分帧、构建通用心跳语义推断框架、猜测未知供应商事件或移植二开特定供应商门控。
- WebSocket/Realtime、异步任务插件观察协议、未经过宿主共享 SSE 扫描模块的传输，以及非流式 JSON 生成的统一等待改造。
- 新 Prometheus 指标体系、请求正文抽样留存、计费/Usage 规则重构、数据库驱动升级或表结构迁移。
- 新环境变量覆盖体系、配置中心、即时跨实例广播、全局选项框架重构及顺带修复无关保存问题。
- 本轮业务实现、自动拆分实施票据、暂存、commit 或 push。

## Further Notes

- 固定比较基线：OP 为 `cf0105704687bdbcbfef54546e488a029c3d0655`，二开为 `762ce21c429a3aa9adc104bcd80eb7df3ad3bdbd`。此处是可复核的代码快照，不声称覆盖未来版本或其他分支。
- 二开该基线通过启动环境变量 `STREAMING_FIRST_RESPONSE_TIMEOUT` 配置，默认 180 秒，非正值关闭；没有对应后台控制。本 spec 是适配 OP 的增强方案：默认关闭、后台可调、请求策略快照和明确的协议心跳排除，不是原样复制二开的 scanner。
- 对话中 600 秒原为说明例子；本 spec 将其选作关闭状态下的初始预填时长，管理员启用前仍应按实际模型调整。它不代表自动启用、推荐所有模型使用 600 秒或可以保证识别所有卡死。
- 举例：上游响应头已返回，预算为 600 秒，持续有注释心跳；第 240 秒出现正常数据则撤销新预算并继续生成，第 600 秒仍没有符合条件的数据则超时。若现有闲置超时、全请求超时或客户端取消更早成立，仍由那个条件先结束。
- 实施核对：OP 原有策略对 HTTP 504/524 存在硬性不重试规则，后台可重试状态范围不能覆盖。本功能的 504 仍遵守该限制，不新增 SkipRetry 标记，也不以此迁移顺带放开重试；普通可重试上游错误仍按旧策略换渠。
- 首个正常元数据事件到达后，本保护已经完成职责；随后长时间没有正文不再由它判断。不要在 UI、日志或验收中将其描述为“无正文一定在指定时间内断开”。
- 新 spec 单独保存于既有本地 Markdown 任务体系，保留先前迁移研究和已完成错误保真任务及其验收记录，不覆盖其完成状态；文档保持可被 Git 跟踪，但本轮不暂存、不提交。
