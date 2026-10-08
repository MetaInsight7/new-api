# Codex 配套功能迁移验收记录

## 范围与基线

- 开发分支：`tokendazi_op`，不切分支、不 push。
- 本轮业务基线：`3eb021c5b`；规格和本地 tickets 已分别提交为 `05ac1cdfc`、`c2ff89917`。
- 官方对照：`main@56758edf9`；二开最新版最终树：`762ce21c4`。没有将旧二开工作树或 overlay 的部分历史提交当作完整来源。
- 本记录仅覆盖 Codex 配套补齐与审计校正，不代表其他运营功能或整站的全部上线验收。
- 当前状态：2026-10-08 完成本轮 Codex 迁移范围的开发、相关完整回归、生产产物 Playwright、两轴复审和九个业务独立提交；九个本地 tickets 完成。额外扩大范围的全包竞态检查并未全绿，其官方已有问题和未稳定复现项单独记录，不冒充整库通过。

## 功能入口

| 功能 | 使用入口／契约 |
|---|---|
| OAuth 授权 | `/channels` 创建／编辑 Codex 渠道；新渠道生成凭证后按正常渠道保存流程创建，已有渠道使用绑定渠道的重新授权接口 |
| 永久失效状态 | 所有渠道类型的“标记为已封禁／解除封禁”和状态筛选；状态 `4` 只由人工管理产生，不被自动任务创建或覆盖 |
| Limits 报表 | `/dashboard/codex-limits`；管理员 `channel:read` 权限，消费日志时间范围与上游当前额度分别展示 |
| 模型生命周期治理 | `/codex-model-governance` 审核；`/system-settings/operations/codex-governance` 配置与主动触发 |
| 单账号额度与重置 | `/channels` 的账号用量弹窗；真实主／附加／未知窗口，重置需 `channel:operate` 并二次确认 |
| 请求与指纹 | Codex 共用字段约束、显式 `service_tier`、typed/full 指纹输入校验；指纹默认 `off` 不变 |
| Chat 失败保护 | 普通 Chat 经 Responses 转换；首有效输出前不提前提交，失败或截断不伪造 `[DONE]`，只按可靠的原生 usage 结算 |
| 模型发现与图片 | 有界模型发现与版本缓存；按实际输出格式处理图片，继续使用既有私有 R2 存储，不回退 GCS |

治理只有主动探测和公告入口，没有普通 Relay 热路径触发器；配置默认关闭，不会因升级而自动发起公告抓取、AI 判断或主动探测。治理只抑制相应模型的渠道能力，不以模型问题关闭整条渠道的 WebSocket。

## 最终行为对齐矩阵

| 能力 | 相对来源最终树的处理 | 本轮交付或保留边界 |
|---|---|---|
| 渠道 OAuth 授权 | 迁移管理闭环，适配 Base AuthFlow | 新建回填、已有渠道重新授权；补齐有效期、绑定、单次消费和保存错误处理 |
| 永久失效渠道 | 保留来源通用人工状态 | 所有渠道可标记／解除／筛选；自动处理不创建或覆盖状态 4 |
| Limits 汇总 | 迁移报表，保留 Base 权限和日志口径 | 所有 Codex 渠道、失败行、本地消费统计、实际／未知／附加窗口 |
| 模型生命周期治理 | 迁移完整领域和管理闭环 | 主动探测、公告／可选 AI、审核、抑制、恢复、移除、持久化冷却和通知；默认关闭 |
| 单账号用量与 Reset | 在较新的 Base 弹窗上适配 | 不回退券详情；认证重试沿用同一消费 ID，零重置和刷新失败如实反馈 |
| Chat bridge | 延续上一批迁移并修正遗漏 | 全局 Chat→Responses 策略仍走 Codex bridge；输出后失败不透明重试或伪成功 |
| Responses／Compact | 保留 Base 较新协议层 | 原生入口和 converter 不被旧实现替换；不添加来源旧倍率体系的合成 Compact 模型别名 |
| 显式 Fast／字段过滤 | 对齐来源 Codex 共享策略 | Chat 不丢 `service_tier`，默认不注入 Fast；不缩减通用 OpenAI DTO |
| 指纹／Identity | 延续上一批完整管理和出口策略 | 指纹默认 `off`；typed／full 输入校验补齐；Identity 写入映射到 `option:write` |
| Alpha Search／凭证刷新 | 保留 Base 已具备的较新功能 | 使用既有刷新、身份和权限服务，不重造或回退 |
| 模型发现／版本 | 保留 Base 发现链路，吸收来源边界 | 有界读取、非成功响应优先处理、冷失败缓存和并发查询合并 |
| 图片／临时 URL | 延续上一批迁移，补齐实际格式 | 每个输出采用实际格式；继续使用私有 R2，不复制 GCS 或部署凭据 |

上述是完整功能行为对齐，不是来源代码的字节级拷贝；独立账号中心、账号自动连带封禁和无关运营功能没有纳入本轮。

## 后端与真实数据库

验证在隔离数据库进行，未将开发服务的用户库作为测试库。DSN、账号密码及上游凭证不写入本记录。数据库环境通过私有的 `/tmp/codex-management-test-db.env` 加载。

| 引擎 | 实际版本 | 已执行覆盖 |
|---|---|---|
| SQLite | 3.50.4 | OAuth／凭证 CAS、状态保护、Reset、消费日志聚合、治理新建／升级与重复迁移 |
| MySQL | 8.0.46 | 同上；额外验证真实行锁与事务交错 |
| PostgreSQL | 15.19 | 同上；额外验证真实行锁与事务交错 |
| Redis | 7.2.15 | 真实双客户端通知、数据库隔离、忽略自身事件、无通知回环、缓存刷新及本机 WebSocket 关闭 |

没有更改 GORM 或数据库驱动版本；没有引入只在新版本数据库可用的业务 SQL。这里不声称已测试 MySQL 5.7 或 PostgreSQL 9.6，也不把 mock 当作真实三库验证。

### 已观察通过的专项回归

- OAuth 与 Reset：SQLite／MySQL／PostgreSQL；包括到期、重放、跨用户／会话／渠道，凭证 CAS，认证刷新后的最新 BaseURL／代理，重试沿用同一 `redeem_request_id`，不确定失败不重复消费。
- Limits：三种真实数据库的共用／独立日志库、闭区间消费日志、501 个渠道 ID 分批、退款／错误日志排除；上游查询部分失败、有界并发及最新配置重试。
- 治理：三引擎的新建／升级六种组合，至少两次迁移；既有数据、索引与唯一性；普通恢复／能力重建不绕过抑制；查询错误不能当成“没有表”。
- 治理并发：真实 MySQL／PostgreSQL 的 18 种可控事务交错，无固定 sleep 或压测；包括禁用、恢复、移除与 partial receiver、nil／显式事务、单渠道／标签／渠道状态启用。18 种交错的 race 检查通过。
- Chat usage：stream／buffered 都只信任经过边界检查的原生 input／output／details，忽略 legacy prompt／completion、Cost、UsageSemantic 及嵌套 billing sidecar。
- Chat 错误分类：保留原来的 code／type／HTTP 状态及模型健康判定，复用已有 Codex Responses 分类器；上下文超限不被伪装成服务器健康失败，认证错误优先于泛化输入错误；输出后不重试，错误文案脱敏。
- 模型发现、实际图片格式、R2 与请求／指纹回归：沿用原有测试和补齐用例，不执行真实上游授权或真实 Reset Credit 消费。

本机专项日志：

```text
/tmp/codex-auth-reset-sqlite.log
/tmp/codex-auth-reset-mysql.log
/tmp/codex-auth-reset-postgres.log
/tmp/codex-ticket03-three-db.log
/tmp/codex-limits-additional-plan-green.log
/tmp/codex-governance-source-semantics-green.log
/tmp/codex-governance-interleaving-focused.log
/tmp/codex-governance-interleaving-race.log
/tmp/codex-chat-native-usage-green.log
/tmp/codex-chat-error-classification-green.log
/tmp/codex-channel-notifications-common-green.log
/tmp/codex-channel-notifications-service-green.log
```

### 最终完整后端回归

最新 probe 取证和公开错误脱敏修复之后，以下 14 个受影响 Go 包的完整测试通过：

```bash
set -a
source /tmp/codex-management-test-db.env
set +a
export TEST_REDIS_ADDR=127.0.0.1:16379
GOMAXPROCS=2 go test -p 2 ./common ./controller ./model ./service \
  ./router ./middleware ./service/authz ./relay ./relay/common ./relay/helper \
  ./relay/channel/codex ./relay/channel/openai ./setting/operation_setting \
  ./pkg/perf_metrics -timeout=15m
```

日志：`/tmp/codex-management-go-final-regression.log`。后端完整构建 `GOMAXPROCS=2 go build -o /tmp/new-api-codex-management-next .` 通过，重新启动后生产端口 3000 验收使用该最新产物。

最终竞态回归覆盖本轮 24 个变更测试文件的全部 105 个测试入口，包括表驱动子用例，而不是只挑几个成功用例。八个包均通过：

```bash
mapfile -t changed_test_files < <(git diff --name-only 3eb021c5b c75525083 \
  -- '*_test.go' | rg -v '^relaykit/')
test_names=$(rg --no-filename '^func Test' "${changed_test_files[@]}" \
  | sed -E 's/^func (Test[[:alnum:]_]+).*/\1/' | sort -u | paste -sd '|')
GOMAXPROCS=2 go test -race -p 2 ./common ./controller ./model ./service \
  ./router ./relay ./relay/channel/codex ./relay/channel/openai \
  -run "^(${test_names})$" -timeout=15m
```

日志：`/tmp/codex-management-codex-final-race.log`。另有真实 HTTP 200 SSE 的三种错误 envelope 取证／公开脱敏红绿回归：`/tmp/codex-probe-public-{red,green}.log`。

独立模块验证已经实际通过：

```bash
cd relaykit
GOWORK=off go build ./...
GOWORK=off go test ./relayconvert/internal/oai_chat ./relayconvert/internal/oai_responses
```

### 扩大范围的竞态诊断

额外执行七个包不加测试范围筛选的 `go test -race` 时，controller／model／service 失败；该执行不是通过结果，完整日志保留为 `/tmp/codex-management-final-race.log`。

- `TestPreConsumePolicyDatabaseMatrix`：测试清理写 `common.RedisEnabled`，与退款异步额度缓存读取竞争。在归档官方 `main@56758edf9` 上实际复现相同竞态。
- 视频任务测试：`logger/logger.go` 的无锁 `logCount++` 发生竞态。官方 `main` 的 `DefaultSleep` 用例实际复现；`SlowChannel` 在该次基线运行通过，不能声称每个用例都稳定复现。
- `TestReserveFallsBackToDatabaseWhenRedisIsUnavailable`：扩大范围执行观察到前一个额度累加测试的异步缓存与测试清理冲突。官方 `main` 最小前驱组合本次通过，不能据此宣称稳定复现或不存在问题。
- `TestSecurityAccountDeletionConcurrentRequestsHaveOneWinner`：扩大范围执行一次 SQLite `SQLITE_BUSY` 导致断言失败；当前和官方基线的专项重跑通过，保留为未稳定复现项。

这些生产／测试路径相对本轮业务基线没有改动，没有为 Codex 迁移顺带改写 Base。官方对照使用临时归档，不切换工作分支；日志为 `/tmp/codex-management-base-race-repro.log`、`/tmp/codex-management-base-model-race-repro.log`，当前树最小复现日志为 `/tmp/codex-management-unrelated-race-repro.log`。完成结论限于本轮 Codex 相关验证，不代表整库竞态问题全部解决。

## 前端与 Playwright

### 单测、类型、lint 与格式

`bun run typecheck` 通过。最终相关完整测试集实际结果为 **39 文件、401 测试通过**：

```bash
cd web
bun run test src/features/channels src/features/codex-model-governance \
  src/features/dashboard \
  src/features/system-settings/integrations/__tests__/codex-governance.test.tsx \
  src/features/system-settings/integrations/__tests__/codex-identity-settings-section.test.tsx \
  --maxWorkers=2
```

本轮 42 个可 lint 的变更文件检查结果为 **0 warning、0 error**。49 个变更文件以现有 formatter 做定向、受保护版权头安全的检查，差异为零；生成的路由树沿用项目 ignore 规则。没有用新增 disable 注释绕过检查。

七种语言的 98 个新增翻译键均存在且非空。最后的类型／测试检查使用 `MemoryMax=4G`、`MemoryHigh=3500M`、`CPUQuota=150%` 和两个测试 worker；unit 实际峰值 2.4G、swap 峰值为零。

OAuth 和治理备注均复用 RHF、Zod 与 FormField。治理成功后的服务端备注刷新先有稳定失败回归，再修复裸 state 遮蔽持久化备注的问题；定向七个用例通过，红绿日志为 `/tmp/codex-governance-review-form-{red,green}.log`。

完整目录 lint 的历史诊断不等于本轮变更诊断；本轮只对涉及文件修复和验收，不重写无关组件或全量翻译文件。日志为 `/tmp/codex-management-frontend-final-regression.log`、`/tmp/codex-management-changed-file-lint.log`、`/tmp/codex-management-format-final.log`。

### 浏览器验收

系统安装的 Playwright 1.63.0／Chromium。10 月 8 日，使用表单、Relay 和 probe 最终校正后的最新后端及受限生产构建产物，在端口 3000 重跑以下九组场景，最终 `errors=[]`；开发端口 5173 的验收也通过，不以早期结果冒充生产终验：

1. 实际管理员登录，真实 Limits 与治理路由返回成功，而不是仅 mock 页面入口。
2. 非 Codex 临时渠道的前端封禁／解除封禁，通过真实 API 验证 `4 -> 1`；已封禁筛选真实返回仅状态 4，刷新后 URL 与本地保存的筛选保持一致。
3. 真实 OAuth 发起、S256 PKCE、固定 localhost 回调、不可信回调拒绝、实际剪贴板复制、重新发起及窄屏布局。
4. 已有 Codex 渠道通过绑定渠道的重新授权接口完成成功反馈；仅模拟上游授权完成，不登录真实 OpenAI 账号。
5. 单账号实际主／附加窗口展示，确认重置、成功刷新及消费成功但刷新失败的诚实反馈；重置上游受控模拟，没有消耗真实 Credit。
6. Limits 免费／失败账号、附加／未知窗口、日期选择、失败与空状态、桌面／390px 窄屏。
7. 治理备注和确认载荷、审核成功、空状态、桌面／窄屏。
8. 真实治理设置、默认关闭、禁用时不能触发外部任务、规则测试。
9. 只读管理员可查看账号／治理，但不能重置、封禁或审核；服务端权限同时由后端回归验证，不以浏览器隐藏代替鉴权。

结果与截图保存在 `/tmp/codex-management-ui/`；最新结果为 `result.json`，日志为 `/tmp/codex-management-playwright.log`。受控网络 fixture 只用于上游不可控数据和受限用户身份场景。

验证脚本为 `/tmp/verify-codex-management-ui.mjs`，密码仅通过私有运行环境传入，没有进入本记录。反复执行曾触发 Base 既有限流：route interception 禁用浏览器 HTTP cache 后造成静态 chunk 429，以及多次登录／刷新累计触发敏感请求的 20 次／20 分钟限额。没有修改业务限流；静态资产仅缓存首次真实成功响应以模拟正常缓存，终验前重启开发后端清除该服务的内存计数，再完整通过九组场景。首次真实请求错误仍被记录，不借缓存伪造成功产物。

已识别的既有 Base 问题：390px 下全局 `app-header` 存在约 26px 文档横向溢出；该文件与 `main` 完全一致，本轮不改无关头部。新增页面主体和弹窗分别检查自身边界，没有溢出，不能把这描述成已经修复整站头部。

### 生产构建

受限完整 `bun run build` 实际通过，最新 Rsbuild 报告 `built in 24.0s`，产物总大小约 66.96 MB／gzip 20.57 MB。构建前停止开发前端释放内存，最终服务验收时重新建立开发服务。

使用 systemd 限制：`MemoryMax=4G`、`MemoryHigh=3500M`、`CPUQuota=180%`、`NODE_OPTIONS=--max-old-space-size=3072`、`RAYON_NUM_THREADS=2`。没有同时开启多个 Bun 构建，也没有通过无资源限制方式冒险验证。

构建仍有既有 `keys/__tests__/index.test.ts` 被路由扫描器忽略的 warning，未影响构建，未借本任务重命名无关测试。构建日志：`/tmp/codex-management-production-build.log`。

## 安全参照与部署边界

- 参照 OWASP ASVS 5.0.0，以及 Authentication、Session Management、OAuth 2.0、CSRF Cheat Sheets；仅记录本轮落实的控制，不声称整站全面 ASVS 合规。
- OAuth 服务端持久化短期、单次、绑定用户／会话／渠道的流程；固定回调、PKCE、凭证不进入日志、敏感授权、origin 校验、限流及 no-store。
- Reset 继续使用 `channel:operate`；认证重试复用同一消费 ID，响应不确定时不能悄悄重复消费；前端权限不代替后端授权。
- 治理证据截断／脱敏；仅 Codex 的 `IsChannelTest` 且显式注册私有 context observer 才提取原始错误，覆盖 `response.error`、顶层 `error.message` 和 flat message。分类后只持久化规则／规范化解释，不保存 raw token、账号或请求；公开流中的顶层与嵌套 error 同时脱敏。公告使用现有 SSRF、超时和体积限制；AI 和外部来源默认关闭。
- Redis 通知只携带范围、实例 ID 和需关闭的渠道 ID，不传渠道 key 或 OAuth 凭证；无 Redis 保留 Base 周期同步兜底。
- 真实 R2 上传／签名下载已在下述独立外部冒烟中完成。仍没有执行真实 OpenAI OAuth 登录、真实 Reset Credit 消费或真实 Codex 图片生成到 R2 的完整链路；上线前应由账号拥有者完成真实账号授权／请求／额度读取及图片生成链路验收，不能把存储冒烟当作这些操作已完成。

## 真实 R2 外部冒烟（2026-10-08）

在 `tokendazi_op` 分支、`fcfab0701` 后调用生产实现 `service.UploadCodexTempMedia`，使用用户提供的 R2 S3 凭据和 `tokendazi` Bucket；没有通过另一套上传客户端替代生产实现。仅上传自行编码的 77 字节 PNG，所有对象位于独立的 `codex-live-test/<本轮唯一标识>/` 前缀中，不列出、修改或删除已有对象。

首次真实上传返回 HTTP 400／`InvalidRequest`：生产代码把 payload SHA-256 交给 AWS signer，却没有设置 R2 所需的 `X-Amz-Content-Sha256` 请求头。原有 mock 仅验证 Authorization、类型和内容，未约束这个请求头。先扩展既有上传回归，让 mock 拒绝缺失或不匹配的哈希头，观察到相同的 400 失败，再补齐并签名实际 payload hash，回归转绿；没有修改凭据、放宽权限、关闭 TLS 校验或改变存储契约。

| 检查 | 实际结果 |
|---|---|
| 生产上传 | PUT 200，写入 77 字节 `image/png` |
| 生产预签名 URL | TTL 300 秒，签名时间、`ExpiresAt`、`ExpiresIn`、类型和大小一致 |
| 签名下载 | GET 200，Content-Type 正确，`Cache-Control: private, max-age=300`，逐字节及 SHA-256 校验一致 |
| 无签名 S3 访问 | GET 400／`InvalidArgument`，拒绝下载；R2 的缺少认证错误不能硬编码为 403 |
| 篡改签名 | GET 403／`SignatureDoesNotMatch` |
| 已过期签名 | GET 403／`ExpiredRequest`；使用已过期的签名时间验证，不等待或伪称测试了 12 小时默认有效期 |
| 对象清理 | 每次成功上传后签名 DELETE 204，随后认证 GET 404；本次创建的测试对象全部删除 |

验收程序在仓库外 `/tmp/verify-codex-r2-live.go`，最终真实结果在 `/tmp/codex-r2-live-verification.log`；首次失败与红绿回归分别在 `/tmp/codex-r2-live-red.log`、`/tmp/codex-r2-regression-{red,green}.log`。记录不包含访问密钥、签名 URL、Authorization 或完整上游错误。

测试配置仅保存在仓库外 `/root/newapi/r2-test.env`，权限 `600`，只传入独立测试进程；没有写入 Git，也没有自动给运行中的后台启用 R2。Cloudflare 管理 API Token 未保存或使用。

修复后实际通过两包完整测试、受影响用例 race、两包 vet 和后端编译；没有前端变更，因此未触发 Bun 构建或 Playwright：

```bash
go test ./service ./relay/channel/codex
go test -race ./service ./relay/channel/codex -run '(CodexTempMedia|NormalizeCodexTempMedia|RelayImageOverCodex.*(R2|TempURL|GeneratedOutputFormat)|CodexTempURL)'
go vet ./service ./relay/channel/codex
go build -o /tmp/new-api-r2-verified .
```

日志分别为 `/tmp/codex-r2-affected-packages.log`、`/tmp/codex-r2-focused-race.log`、`/tmp/codex-r2-affected-vet.log`、`/tmp/codex-r2-backend-build.log`。没有把前述整库已知竞态列为本轮通过结果，也没有顺带修复这些无关问题。

边界：无签名 S3 接口拒绝下载，不能证明 Bucket 的 `r2.dev` 或公共自定义域名已关闭；本轮未读取公共域名设置，需要部署方在控制台确认。签名过期也不等于对象自动删除，生产 Bucket 的临时对象生命周期规则仍需单独配置。本轮没有调用 Codex 上游、生成真实订阅图片或消费 Reset Credit。

## 独立提交与最终复审

业务 message 写实际功能，不写 ticket 代号；配置写入 scope 校正也独立提交：

| 提交 | 功能 |
|---|---|
| `11147c84b` | 安全 Codex OAuth 授权与渠道凭证管理 |
| `99ac1f19e` | 通用人工永久失效状态、筛选和跨节点同步 |
| `8f20ab7a4` | Codex Limits 用量与额度报表 |
| `a664c390d` | Codex 请求字段约束、显式 Fast 和 full 指纹校验 |
| `96aa88a39` | Codex Chat 流失败、截断、全局转换兼容和可信 usage |
| `d68fe1a49` | 真实账号窗口与安全 Reset Credit 处理 |
| `149da32d8` | 有界模型发现、版本缓存和图片实际输出格式 |
| `6e12a1287` | Codex Identity 更新使用 option 写入 scope |
| `c75525083` | 完整模型生命周期治理、前后端及私有 probe 取证 |

### Standards

复审无阻塞项。新增表单复用 RHF／Zod／业务组件、JSON 走包装层，probe observer 不进入公众凭证或日志通道。保留一个非阻塞的 possible Duplicated Code 启发式意见：缓存刷新和通知发布的调用形态相似，不是已证实的功能缺陷，本轮没有据此扩大改造。

### Spec

复审无阻塞项。三种 SSE 错误 envelope 取证与公开脱敏，以及全局 Chat 转换／成功缺 usage 的最后遗漏均有回归闭环；普通 Relay 不新增治理触发器，其他 provider 不受 Codex 专属分支影响。真实 R2 存储已补充外部冒烟；真实 OpenAI 授权、实际 Reset Credit 消费和 Codex 图片完整链路仍按前述边界由拥有者验收，不伪称已完成这些外部操作。

本轮九个 tickets 和验收记录随最终文档提交归档；未 push。开发服务 3000／5173 均已启动，业务与文档分别提交，不将工作树勾选代替实际验证。
