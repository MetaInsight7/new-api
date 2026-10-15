# Streaming first-event timeout

## Notes

- 规格： [spec.md](spec.md)
- 当前分支：tokendazi_op；固定起点：cf0105704687bdbcbfef54546e488a029c3d0655。
- [01](issues/01-admin-first-event-timeout.md)：resolved，后台配置与共享 SSE 超时闭环。
- [02](issues/02-protocol-heartbeats.md)：resolved，协议级心跳、桥接兼容及联合验收完成。
- 验收：[verification.md](verification.md)；双轴审查：[review.md](review.md)。

## Decisions-so-far

- 按既定 Relay HTTP、公共 scanner、受保护管理选项及后台交互边界做 TDD。
- 默认关闭，600 秒仅预填；复用现有管理和选项同步，不增加环境变量。
- 保留原并发、重试、Usage、计费和 Responses 恢复规则。
- OP 对 504 有既有硬性不重试规则；新超时返回 504，继续遵守该限制，不顺带调整策略。
- 同步按模块校验发布，并丢弃被并发成功保存淘汰的旧读取；原生 Responses 已提交后复用 SSE error，不伪造成功。

## Fog

- 本功能无未决项。前端全量 2396 通过、1 失败；同一模型列表用例在未修改的固定基线独立复现，保留为无关既有失败，没有顺带修复。
