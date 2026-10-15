# 02: 统一最终结果、模型请求计数与健康／性能报表

Type: task

**What to build:** 模型请求计数与健康／性能报表使用统一最终成败规则，保留 OP HTTP、Responses WebSocket、异步终态和异常退出覆盖，不因重试重复计数。

**Blocked by:** None (can start immediately).

**Status:** resolved

- [x] 新模型请求成功／失败计数与报表共同采用二开口径，直接修改旧忽略规则。
- [x] 网关前置返回、流式终态、内部恢复、任务终态和 WebSocket 请求单位正确。
- [x] 保持既有接口、分组权限、窗口和计数加权语义；模型请求计数有空闲回收及抓取预算。
- [x] 对应高层回归和真实数据库验证通过，不改业务治理、不处理历史。

## Answer

- 直接删除 ignored 观测分类；已采样的取消和上游拒绝均计失败，正常 token 上限仍可成功。
- `pkg/perf_metrics/outcome.go` 统一判断；`model_requests.go` 保存独立的六小时请求计数。WebSocket 在模型访问前置校验之后才进入采样，不把无效请求新增为调用样本。
- 扩展既有 HTTP／WebSocket／任务轮询回归；重试成功只记一次最终成功，连接复用逐请求计数，重复任务终态不重复累计。
- 真实 SQLite 3.50.4、MySQL 8.0.46、PostgreSQL 15.19 聚合／落库回归通过：同批 9 成功＋1 取消在 Prometheus、性能报表及安全窗口落库健康报表均为 90%。完整验证记录见 [map](../map.md)。
- 评审补充：缓冲 SSE 转 JSON 沿用协议终态，不强制要求流式传输结束标记；任务观测脱离账务 finalizer，只在既有 CAS 赢得首次终态时计入，重复轮询及旧快照竞争不再重复采样。
