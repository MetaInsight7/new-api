# 渠道测试失败告警：实施与验证

Status: resolved
Branch: tokendazi_op
Baseline: 8b89ed6db061d8fd70ba02f77735ad2f6bef18d0

## 实现边界

- 共享 `monitor_setting` 机器人与 AI 接入；原 Codex 独立字段直接移除，无搬迁、别名或回退，旧数据库记录不删除。
- 自动测试结果在既有串行归并中按 5 条失败候选成批，正常结束发送尾批；三种模式、有界 worker、任务取消和手动通知语义保留。
- 渠道 ID 唯一的持久化冷却记录只保存成功时间及 80 秒 token 预占，不保存消息。成功确认后提交，失败释放；数据库故障降级进程内冷却，不承诺跨实例去重或恰好一次。
- 复用受保护的外部 HTTP、钉钉签名及明确成功确认；可选 Chat Completions 中文摘要受 60 秒预算、输入 / 响应 / 输出限制约束，失败回退基础告警，父任务取消优先。
- 原管理员渠道状态通知未替换；告警中的禁用字段只表述触发了既有流程，不冒充持久化禁用成功。
- 后台组合现有 `SettingsForm` / `SettingsFormGrid` / `SettingsSwitchItem` / `FormField` / `Input` / `Switch`，不引入替代通用交互或新依赖。新增文案覆盖七种语言。

## 安全依据与限制

- 2026-10-15 核对 OWASP ASVS 发布列表，稳定版为 `v5.0.0_release`（2025-05-30），没有把自动生成的 `latest` 当作稳定版。
- 已阅读 Authentication / Session Management / Secrets Management Cheat Sheets，以及 ASVS 5.0 的授权、外部通信、配置及数据保护相关要求。适用控制：沿用 root 设置授权和会话 / 请求安全链路；服务端校验；Secret / AI Key 不回显、不写浏览器存储、不输出到新日志；空白保留与显式清除可区分；外部 URL、DNS、重定向遵循已有 SSRF 策略。
- 生产接入应使用 HTTPS、最小权限机器人及分析密钥，并由部署方管理保存数据的访问和轮换；沿用现有受保护选项存储，不宣称已完成全站 ASVS 合规审计。
- 规则式掩码不等于完整匿名化，渠道 ID、常规名称和故障事实可能外发。Webhook Token 是钉钉协议中的查询凭据，不能出现在新日志或 AI 输入中。
- 测试全部使用本地可控 HTTP 与虚构凭据，没有读取或使用真实机器人、生产数据库或付费分析密钥；真实公网投递和真实模型质量不在本轮已验证范围。

## TDD 与聚焦回归

- 先将既有 Codex 投递测试改成保存共享选项：观察到共享设置不生效导致投递数为 0，再完成共享读取并观察通过。
- 后台先断言旧连接字段被移除及监控页出现共享控件：观察到旧字段仍存在 / 新控件缺失，再实现表单；新增保存回归覆盖空白保留、显式替换 / 清除、默认端点 / 模型显式固定及失败时不启用。
- 系统任务闭环先观察到 6 条失败无消息，再接入结果归并、5+1 批次和冷却；可选摘要测试先观察到 AI 调用为 0，再实现摘要与失败回退。
- 新增服务端回归先观察到已启用状态下可清空必需目标，再修正有效配置校验并观察通过。
- 双轴审查补充两项 RED / GREEN：捕获配置后并发替换接入值，复现旧 Key 发往新 URL / Model；阻塞 DNS 后取消任务，复现分析 / 机器人预检不退出。修正为完整连接快照，以及在 DNS 之前建立预算并贯通 context；聚焦结果见 `/tmp/channel-alert-review-{red,green}.log`。
- 已通过：`go test ./controller ./service ./model -run '^(TestChannelAlert|TestCodexGovernance|TestChannelTestAlert)' -count=1`；`go test -race ./controller ./service ./model -run '^(TestChannelAlert|TestCodexGovernance|TestChannelTestAlert|TestRunChannelTestWorkers)' -count=1`。日志位于 `/tmp/channel-alert-focused.log`、`/tmp/channel-alert-race.log`。
- 主任务回归包含同一次上游测试、Codex 渠道、关闭 / 手动 / 三种模式、5+1 批次、冷却前置、脱敏输入输出及 rune 限额、分析失败回退、投递失败后新测试可再发、取消中断 AI / 钉钉及丢弃尾批、DB 降级、token 生成失败、root 写入授权。后端只新增 `controller/channel_test_alert_test.go` 一份测试文件，其余扩展已有文件。

## 三数据库与发布版升级

- 全部使用 `/tmp/channel-alert-db/` 下独立临时实例 / 文件，未使用部署连接。SQLite 3.50.4、Oracle MySQL Community Server 8.4.7、PostgreSQL 15.19；额外用 MariaDB 10.11.18 验证兼容行为，但没有以 MariaDB 代替要求中的真实 MySQL。
- 真实主矩阵：

  ```sh
  TEST_MYSQL_DSN='root@tcp(127.0.0.1:33317)/channel_alert_test?parseTime=true' \
  TEST_POSTGRES_DSN='postgres://postgres@127.0.0.1:55416/postgres?sslmode=disable' \
  go test ./model -run '^TestChannelTestAlertDatabaseMatrix$' -count=1 -v
  ```

  6 个 fresh / upgrade 子测试全部执行通过，无跳过；覆盖重复迁移、渠道 / 选项 / 任务保留、唯一性、真实事务竞争只允许一个持有者、失败释放、过期接管、旧 token 不得提交或释放新持有者，以及成功冷却持久化。日志：`/tmp/channel-alert-real-db-matrix.log`。

- 发布版固定为仓库最新标签 `v1.0.0-rc.41`，commit `2035a82aeb5414253a728bd937d4b8f97aa99b9b`。用 `git archive v1.0.0-rc.41` 导出到 `/tmp/channel-alert-release`，临时 fixture 调用该版本真实 `InitDB()` 创建三引擎数据库，写入渠道 777、禁用能力、保留选项和成功系统任务；不使用当前模型伪造发布版初始化。
- 当前代码分别针对这三份发布版数据库执行：

  ```sh
  CHANNEL_ALERT_RELEASE_FIXTURE=1 CHANNEL_ALERT_SQLITE_PATH=/tmp/channel-alert-db/release.db SQL_DSN='' \
    go test ./model -run '^TestChannelTestAlertReleasedUpgrade$' -count=1 -v
  CHANNEL_ALERT_RELEASE_FIXTURE=1 SQL_DSN='root@tcp(127.0.0.1:33317)/channel_alert_release_mysql?parseTime=true' \
    go test ./model -run '^TestChannelTestAlertReleasedUpgrade$' -count=1 -v
  CHANNEL_ALERT_RELEASE_FIXTURE=1 SQL_DSN='postgres://postgres@127.0.0.1:55416/channel_alert_release_pg?sslmode=disable' \
    go test ./model -run '^TestChannelTestAlertReleasedUpgrade$' -count=1 -v
  ```

  三项均通过；各自实际启动 / 迁移两次，检查新表注册、旧记录、任务及治理索引、选项唯一约束。日志：`/tmp/channel-alert-upgrade-{sqlite,mysql,postgres}.log`。预期的重复唯一键错误属于测试断言，不是升级失败。
- 没有修改日志库迁移，不向独立日志数据库注册渠道告警冷却表。本次使用既有跨引擎语法及 `lockForUpdate`，未增加依赖特定新版本的数据库特性。

## 最终检查

- 最终 `GOMAXPROCS=4 GOFLAGS=-p=4 make test` 通过，包含 root 与独立 relaykit 模块；日志 `/tmp/channel-alert-go-final-full.log`。`GOMAXPROCS=4 GOWORK=off go vet -p=4 ./...` 与 `GOMAXPROCS=4 GOWORK=off go build -p=4 -buildvcs=false ./...` 均通过，日志 `/tmp/channel-alert-go-{vet,build}.log`。
- 最终真实 MySQL / PostgreSQL 环境执行 `go test -race ./common ./controller ./service ./model -run '^(TestChannelAlert|TestChannelTestAlert|TestCodexGovernance|TestProtected|TestSSRF|TestValidateURL|TestRunChannelTestWorkers)' -count=1` 通过，无竞态报告；SQLite 同属该 fixture 矩阵。环境 DSN 与真实矩阵一致，使用 `GOMAXPROCS=4`。日志 `/tmp/channel-alert-final-race.log`。
- `GOMAXPROCS=4 GOFLAGS=-p=4 go test ./service -run '^TestChannelTestAlert' -count=2` 通过，确认冷却 fixture 和全局替换在重复执行时正确隔离；日志 `/tmp/channel-alert-repeat.log`。
- 后台最终 `bun run typecheck` 和聚焦 `bun run test src/features/system-settings/integrations/__tests__/codex-governance.test.tsx` 通过，8 项测试全部通过；日志 `/tmp/channel-alert-ui-final-focused.log`。涉及文件的 oxlint 无 error。
- 全量 `bun run test` 完整结束：211 个文件中 210 通过、1 失败；2400 项中 2399 通过、1 失败。唯一失败是未修改的 `src/features/models/__tests__/model-listing.test.tsx` 中 `keeps an active visibility filter when its server result is empty`，缺少 `/Model square visibility.*Unavailable/` 按钮；已在固定基线导出目录以该测试名复现同一失败。日志 `/tmp/channel-alert-ui-full.log`、`/tmp/channel-alert-ui-baseline-test.log`。不将该结果宣称为前端全绿。
- 全量 `bun run lint` 在基线与最终实现均为 154 个 error；对比错误记录无新增 error。日志 `/tmp/channel-alert-ui-{baseline-lint,final-lint}.log`。未修复无关文件的既有问题。
- `bun run build` 成功生成生产产物，日志 `/tmp/channel-alert-ui-build.log`。对全部 18 个变更前端文件运行项目 protected-header formatter 的 scoped `--check` 通过；修改 Go 文件 `gofmt -l` 无输出，`git diff --check HEAD` 通过。没有为过检查而变更其他路径或受保护版权头。
- 两名代理分别完成 Standards / Spec 审查与修正后复查；首轮 3 / 1 项均为 P2，最终两个轴均无未关闭问题，详见 [review.md](review.md)。
- 三个临时数据库实例已停止（额外 MariaDB 也已停止），仅保留 `/tmp/channel-alert-db/` 下可复核数据与日志；没有停止或修改生产服务。未提交构建产物、临时 fixture、测试数据库或其他功能的未跟踪研究文件。
