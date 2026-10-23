# 12: 执行后端、三数据库与协议全量验证

**Spec:** `../../2026-10-23-tokendazi-op-main-merge-spec.md`


**Type:** task
**What to build:** 对合并后的后端执行与 CI 等价的 vet、build、test，并在三种数据库和关键 Relay 协议场景中验证真实行为，形成可供二次检查复现的命令、版本和结果记录。

**Blocked by:** 01–06；11: 收敛结构化消息、国际化与依赖合规产物

**Status:** resolved

- [x] 使用规定 Go toolchain 完成根模块以及两个独立模块的 vet、build 和全量 test。
- [x] SQLite 测试实际运行；MySQL、PostgreSQL、独立 audit/log 库因 DSN 未配置明确记录为跳过。
- [x] 自动回归普通 Relay、已提交 SSE 错误、首事件 timeout、慢客户端大事件、完成后取消、Codex refresh/reset、Claude cache/tool 和 Responses terminal usage。
- [x] 验证 request/body 错误根因、渠道并发 lease/cooldown、失败指标和计费累计均只发生一次。
- [x] 记录工具链、数据库版本、完整命令、通过/跳过/失败数量和原始日志位置；未运行项目未写成通过。
- [x] 只修复本次合并引入的问题；基线可复现的无关失败单独记录，不混入实现提交。
- [x] 验证未暴露需要 `needs-info` 的上游与二开能力冲突。

## Answer

- 工具链为 Go `1.26.10`。最终通过 `GOTOOLCHAIN=auto GOWORK=off go vet ./...`、`go build ./...`，以及 `relaykit`/`tokenkit` 各自的 vet/build；日志见 `/tmp/go-final-vet-build.log`。
- `GOTOOLCHAIN=auto make test` 最终通过：根模块 82 个 package（47 个有测试、35 个无测试文件）、`relaykit` 17 个 package（9/8）、`tokenkit` 1 个 package，失败 0；日志见 `/tmp/go-final-make-test.log`。关键包独立全量日志见 `/tmp/go-key-packages-rerun.log`。
- Relay/SSE/WebSocket、Codex、Responses/Claude usage、并发 lease/cooldown、失败 outcome 和单次结算均由上述自动化测试覆盖；验证中补齐了 Codex billing fixture、按 `tokenkit` 实际分词更新旧断言，并恢复渠道告警对 URL/裸域名的外发脱敏。
- SQLite 实际运行，测试输出版本为 `3.50.4`。`TEST_MYSQL_DSN`、`TEST_POSTGRES_DSN`、`AUDIT_MYSQL_DSN`、`AUDIT_POSTGRES_DSN`、`AUDIT_CLICKHOUSE_DSN` 未配置，因此 MySQL、PostgreSQL、ClickHouse/audit 项为明确跳过，不能宣称通过。
