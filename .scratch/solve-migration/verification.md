# 重试无候选保留最近失败：验证记录

## 范围与基线

- 分支：`tokendazi_op`。
- 实施前基线：`c0d12a220dbc84328bbdbeb919e1bc18acf26b0a`。
- 规格：`spec.md`；任务：`issues/01-preserve-last-attempt-error.md`。
- 环境：Linux amd64，Go `1.25.1`；实施开始于 2026-10-14，验证延续至 2026-10-15（Asia/Shanghai）。
- 使用 OP 适配方案，不是照搬二开；此前预扣错误修复保持独立。
- 没有生产服务、真实付费上游或生产数据库联调，没有 push。

## 控制器回归与 TDD

测试只扩展两个现有控制器测试文件，没有新建分散在各层的测试文件。

正式 red → green 观察：

1. HTTP 真实上游测试服务器返回 503，下一轮仅候选为空；原代码最终返回无渠道 500，修复后返回原 503、业务码和协议错误类型。
2. 仅恢复公共 API 错误后，结构化正文仍绕过已执行的脱敏；在最终协议渲染处使用已脱敏消息后，响应不再包含测试上游密钥、地址或私有诊断正文。
3. 任务 429 的状态可以保留，但原包装仍使业务码变为 `get_channel_failed`、丢失 Data、错误链及本地/上游来源；保留原 TaskError 后通过。
4. 可重试本地容量错误原先在请求策略记录中被投影为 upstream；本地 API 错误投影修复后记录为 local，同时仍能进入下一轮选路。
5. 带有上一提交 LastError 的新提交，在没有候选和实际尝试时错误复用旧 429；入口清理后维持原无渠道 500。
6. 失败后成功的任务原先在返回的 RelayInfo 中留有旧错误；成功尝试清理后保持正常结果和既有持久化/结算流程。

补充确定性用例覆盖：

- 普通 Relay：429、503、多次失败取最近一次、换渠成功、新数据库选路故障优先。
- 任务：429、503、多次失败取最近一次、连续两次本地容量失败仍允许换渠重试并保持本地来源、已接受任务禁止重复提交。
- 新故障优先：数据库不可用、并发 Redis 客户端关闭、另一候选渠道真实容量已满。
- 实际上游/提交次数、策略尝试数和失败事件数均保持匹配，不虚构额外失败尝试。
- 现有预扣不可重试/日志选项、已写响应、已接受提交及脱敏回归继续执行。

最后一次定向命令（通过）：

```bash
GOMAXPROCS=2 go test -p 2 ./controller -run '^(TestRelayHTTPCandidateExhaustionPreservesLastFailure|TestTaskCandidateExhaustion.*|TestTaskRetrySuccessClearsFailureHistory|TestTaskNewRoutingFailureTakesPrecedence|TestTaskPreConsumeFailurePreservesRetryAndLoggingPolicy|TestAcceptedSubmitStreamNeverRetries|TestRelayDoesNotAppendJSONToCommittedStreamOnError|TestRelayKeepsJSONErrorBeforeResponseCommit|TestSanitizeError.*)$' -count=1
```

新夹具使用独立测试数据库、显式路由与全局配置恢复；外部上游使用本地 HTTP 服务器。数据库读取故障在 GORM 查询边界注入，并发系统故障使用已关闭的测试 Redis 客户端；容量耗尽通过真实槽位租约模拟。

## 全量与静态检查

以下命令均执行成功：

```bash
GOMAXPROCS=2 go test -p 2 ./... -count=1
(cd relaykit && GOWORK=off GOMAXPROCS=2 go test -p 2 ./... -count=1)
GOMAXPROCS=2 go vet -p 2 ./...
(cd relaykit && GOWORK=off GOMAXPROCS=2 go vet -p 2 ./...)
git diff --cached --check
```

根模块全量测试中的 controller 耗时 199.091 秒，其余有测试的包均通过；独立 relaykit 模块全量也通过。全量验证后生产代码未变化，仅补强本地容量换渠测试，并重新执行上述定向命令。

修改的 Go 文件执行 `gofmt`，技能配置、词汇表和 ADR 引用存在。本轮未修改数据库行为、结构、驱动或 SQL，测试数据库为 SQLite 3.50.4；未声称本轮进行了 MySQL/PostgreSQL 真实矩阵或生产验证。

## 双轴审查

两名独立审查代理以实施前固定基线到暂存区的非空 WIP diff 审查，生产代码、测试、规格及已授权的技能配置均在范围内。两轴都纳入最后补强的本地容量双渠道重试用例；最后更新的任务完成状态和验证记录仅为文档收尾。

### Standards

- 硬违规：0；判断性 smell：0；最高严重项：无。
- 核对根规范、计费规则、Task Plugin API 和技能配置；现有控制器测试接缝与任务 stub 符合项目约定。

### Spec

- 需求缺失/部分实现：0；未授权扩展：0；实现错误：0；最高严重项：无。
- 核对错误保真、新故障优先、本地来源与重试、已接受任务防重复及协议脱敏行为。

总计 Standards 0 项，Spec 0 项；两个轴均无未解决发现。

## Git 管理

规格、开发任务、技能配置和本验证记录沿用现有开发 Markdown 的 Git 管理方式。此前未跟踪的三份比较研究草稿不在本次实现提交范围内。
