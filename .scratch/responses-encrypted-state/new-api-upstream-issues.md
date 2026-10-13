# QuantumNous/new-api：Responses 加密推理状态上游调查

> 调研截止：2026-10-11（Asia/Shanghai）
> GitHub 页面中的时间均按 GitHub API 返回的 UTC 时间记录。
> 来源范围严格限定为 `github.com/QuantumNous/new-api` 官方仓库的 Issues、Pull Requests、Discussions、提交、Release 与对应官方源码。

## 结论摘要

1. **官方仓库已有几乎完全相同的公开报告。** [Issue #3240](https://github.com/QuantumNous/new-api/issues/3240) 报告了 `Codex + new-api + 多个 Azure OpenAI deployment`、已经按 `prompt_cache_key` 开启渠道亲和，仍出现完全相同的 `invalid_encrypted_content / could not be decrypted or parsed`。该 Issue 截至调研日仍为 **Open + stale**，没有关联已合并 PR。
2. **维护者已明确认可根因属于跨渠道/跨账号状态不可移植。** [Issue #1743](https://github.com/QuantumNous/new-api/issues/1743) 讨论 OpenAI Responses 的 reasoning 状态必须保持在同一账号/API Key/组织安全域；[Issue #2660 的维护者评论](https://github.com/QuantumNous/new-api/issues/2660#issuecomment-3742327791) 也明确概括为“A 渠道的签名不能用在 B 渠道”。
3. **官方已经合并的方案只是“限时渠道亲和”，不是 encrypted-content identity affinity。** [PR #2669](https://github.com/QuantumNous/new-api/pull/2669) 于 2026-01-26 合并，以 `prompt_cache_key` 将会话暂时绑定到一个 `channel_id`；最早进入 [v0.10.7-alpha.1](https://github.com/QuantumNous/new-api/releases/tag/v0.10.7-alpha.1)。它没有持久化上游账号身份或 Key 身份。
4. **用户描述的触发路径在最新官方代码中仍然成立。** 最新 Release [v1.0.0-rc.43](https://github.com/QuantumNous/new-api/releases/tag/v1.0.0-rc.43) 和最新 `main@93ea55f3` 的相关路由文件完全一致：默认 TTL 是 3600 秒；缓存值只有整数 `channel_id`；缓存 miss 后回到普通随机选渠；请求的 Responses `input` 以 `json.RawMessage` 原样保留。因此“渠道 1 建立会话 → 空闲至亲和过期 → 新增渠道 2 → 命中渠道 2 → 旧 `encrypted_content` 无法验证”没有被根治。
5. **当前官方最新版没有已合并的密文降级或清除开关。** `invalid_encrypted_content`、`thinking_signature_invalid`、`REMOVE_RESPONSES_REASONING_INPUT` 等专用处理在 `v1.0.0-rc.43` 与最新 `main` 中均为零代码命中。
6. **已有四类修复提案，但均未进入官方分支：** 全量移除 reasoning input（PR #2662、#3237，均关闭未合并）；按错误移除 `encrypted_content` 后重试（PR #6392、#7019，均仍开放且冲突）；带 replay 恢复的 fork 方案（PR #5852，作者明确关闭并声明只给自己的 fork）；Key 级亲和（PR #5006，关闭未合并）。
7. **官方仓库没有“让上游不产生 encrypted state”的已合并选项。** 两个关闭的 `REMOVE_RESPONSES_REASONING_INPUT` PR 是在后续请求中删掉历史 reasoning item，并非阻止上游首次生成；开放的恢复 PR 也是错误发生后降级，且会损失 reasoning 连续性。

## 判定标准

- **完全相同 / 高度同类**：同时涉及 Responses reasoning 状态、跨渠道或跨账号、`invalid_encrypted_content`/签名不可验证，或明确讨论同一状态域约束。
- **部分相关**：涉及渠道亲和、重试、Key 轮询、`prompt_cache_key` 丢失或其他协议的签名状态，但不是本次“亲和 TTL 过期后跨身份回放密文”的完整路径。
- **无关**：仅因示例 Payload 出现 `encrypted_content`、reasoning item 或 Codex 字样而被关键词命中，实际问题是流事件、计费、字段转换或断流。

## 完全相同或高度同类

### 1. Issue #3240：亲和已开启仍报 `invalid_encrypted_content`

- URL：[请支持类似 LiteLLM 里 encrypted_content_affinity 的功能](https://github.com/QuantumNous/new-api/issues/3240)
- 状态：**Open**；标签 `enhancement`、`stale`。
- 日期：创建于 2026-03-13；最后更新于 2026-09-14。
- 报告场景：`Codex + new-api + 多个 Azure OpenAI deployment`，同一模型部署到多个 region，已根据 `prompt_cache_key` 开启渠道亲和，仍收到：

  ```text
  code: invalid_encrypted_content
  The encrypted content ... could not be verified.
  Encrypted content could not be decrypted or parsed.
  ```

- 维护者结论：仓库成员 Calcium-Ion [建议将亲和规则改为失败后不重试](https://github.com/QuantumNous/new-api/issues/3240#issuecomment-4059912722)。报告者随后 [确认这样配置仍会报错](https://github.com/QuantumNous/new-api/issues/3240#issuecomment-4082038184)。
- 关联 PR/Commit/Release：**无**。Issue 没有关联关闭 PR，至今未解决。
- 与本次问题的关系：错误码、协议、Codex、多上游、`prompt_cache_key` 亲和仍失效均相同；Issue 没有明确写出“TTL 过期后新增渠道”这一具体时序，但属于相同故障类。

### 2. Issue #1743：官方早期状态管理总议题

- URL：[关于 API 状态管理的一些初步想法](https://github.com/QuantumNous/new-api/issues/1743)
- 状态：**Open**。
- 日期：创建于 2025-09-04；最后更新于 2026-07-25。
- 核心证据：Issue 明确描述 Responses 的两种状态路径：
  - `store=true` 时，后续请求通过响应或 item ID 引用服务端状态；
  - `store=false` 时，客户端回传加密 reasoning；
  - 两种路径都要求后续请求仍处于相同账号/API Key/组织可访问域，否则无法继续。
- 维护者结论：协作者 seefs001 在 [2026-01-25 的评论](https://github.com/QuantumNous/new-api/issues/1743#issuecomment-3796924449) 中称此问题“不得不探讨”，当时方案是利用 Codex 固定的 `prompt_cache_key`，把请求在一段时间内固定到一个渠道，并指向 [PR #2669](https://github.com/QuantumNous/new-api/pull/2669)。
- 与本次问题的关系：这是官方仓库内对根因最完整的描述；它提出的“固定一段时间”也正好留下 TTL 过期后的缺口。

### 3. PR #2662：多账号轮询下剥离账号绑定 reasoning

- URL：[feat(openai): strip account-bound reasoning from Responses input](https://github.com/QuantumNous/new-api/pull/2662)
- 状态：**Closed, not merged**。
- 日期：创建于 2026-01-13；由作者于 2026-02-27 关闭。
- 提案：增加默认关闭的 `REMOVE_RESPONSES_REASONING_INPUT`，在转发前移除 `input` 中所有 `type` 以 `reasoning` 开头的 item，以规避多账号轮询时的账号/会话绑定错误。
- 关键维护者证据：
  - 协作者 seefs001 在 [评论中贴出完全相同的 `invalid_encrypted_content`](https://github.com/QuantumNous/new-api/pull/2662#issuecomment-3747942326)，并确认 encrypted 字段也属于多账号问题。
  - 他在 [后续评论](https://github.com/QuantumNous/new-api/pull/2662#issuecomment-3747944602) 表示考虑请求到账户的映射，但拟做“提高对应账户优先级”，失败后仍去其他渠道。
  - PR 作者指出 [带账号绑定字段时跨渠道重试仍会失败，删除只是权宜之计，并会损失长推理性能/成本优势](https://github.com/QuantumNous/new-api/pull/2662#issuecomment-3747985370)。
- 关联提交：[bfeed1eb](https://github.com/QuantumNous/new-api/commit/bfeed1eb6de50b41d97c2b77fabed0b40455eb57)。
- 关联 Release：无；未合并。
- 结论：官方协作者认可了同一根因，但这个“删 reasoning”方案没有进入主干。

### 4. PR #3237：再次提出可选 reasoning 输入兼容开关

- URL：[feat(openai): add responses reasoning input compatibility flag](https://github.com/QuantumNous/new-api/pull/3237)
- 状态：**Closed, not merged**。
- 日期：创建于 2026-03-12；由仓库成员 Calcium-Ion 于 2026-03-17 关闭。
- 提案：同样增加 `REMOVE_RESPONSES_REASONING_INPUT=true`，明确把 `invalid_encrypted_content` 列为目标错误；默认行为保持不变。
- 官方讨论：贡献者 RedwindA [建议研究并使用渠道亲和](https://github.com/QuantumNous/new-api/pull/3237#issuecomment-4048904627)，没有给出可合并的密文处理替代方案。
- 关联提交：PR head 为 [86494be6](https://github.com/QuantumNous/new-api/commit/86494be647d7993c4f34b6eb3145d7202f229028)。
- 关联 Release：无；未合并。
- 结论：这是与 #2662 基本相同的兼容降级方案，仍未被官方接受。

## 官方已合并方案及演进

| 日期 | 官方项 | 状态与证据 | 对本问题的作用 |
|---|---|---|---|
| 2026-01-13 | [Issue #2660](https://github.com/QuantumNous/new-api/issues/2660) | Claude 签名问题；维护者称“A 渠道签名不能用于 B 渠道”，后由 #2669 关闭 | 不同协议，但确认相同的“状态不能跨上游身份”模型 |
| 2026-01-26 | [PR #2669](https://github.com/QuantumNous/new-api/pull/2669) / [commit d9321b7d](https://github.com/QuantumNous/new-api/commit/d9321b7da34f396d238223737075d7cec55b7886) | **Merged**；增加渠道亲和、TTL、Redis/内存缓存、成功后更新绑定 | 缓解跨渠道问题，但只保存 `channel_id`，且会过期 |
| 2026-01-26 | [v0.10.7-alpha.1](https://github.com/QuantumNous/new-api/releases/tag/v0.10.7-alpha.1) | Release notes 明确列出 #2669 和“渠道优选” | 渠道亲和首次正式发布 |
| 2026-02-02 | [PR #2799](https://github.com/QuantumNous/new-api/pull/2799) / [commit f244a9e6](https://github.com/QuantumNous/new-api/commit/f244a9e661d6ca8f8d4b0b1561734044e6861869) | **Merged**；支持单规则失败后停止重试 | 防止已命中亲和后透明跨渠道重试，但不处理缓存 miss |
| 2026-02-02 | [PR #2809](https://github.com/QuantumNous/new-api/pull/2809) / [commit 723b85b8](https://github.com/QuantumNous/new-api/commit/723b85b87fb7e809c8107bd8254f232d01a58e1c) | **Merged**；默认开启 Codex/Claude 亲和 | 默认启用缓解方案；进入 [v0.10.7](https://github.com/QuantumNous/new-api/releases/tag/v0.10.7) |
| 2026-03-19 | [commit 5fe8e98e](https://github.com/QuantumNous/new-api/commit/5fe8e98eeba283938c84d8952935b706eb74a045) | 把 Codex/Claude 默认模板改为 `SkipRetryOnFailure=true`；最早包含于 `v0.11.9-alpha.1` | 命中绑定后默认 strict；仍不保护过期后的 cache miss |
| 2026-06-05 | [PR #5306](https://github.com/QuantumNous/new-api/pull/5306) / [commit 4a188dee](https://github.com/QuantumNous/new-api/commit/4a188deeaa721cd87eaba1ef989b8aa698f82fb3) | **Merged**；可配置渠道禁用后是否清除亲和；进入 [v1.0.0-rc.11](https://github.com/QuantumNous/new-api/releases/tag/v1.0.0-rc.11) | 处理“绑定渠道被禁用”，不处理自然 TTL 过期 |

### 对官方方案的准确评价

- 亲和缓存解决的是“在 TTL 内优先回到同一渠道”。
- `SkipRetryOnFailure=true` 解决的是“已经命中一个现存绑定后，失败时是否换渠道”。
- 两者都没有解决“绑定已经过期/被清除/从未建立，而请求又带着旧的加密 reasoning 状态”这一情况。
- 因而 #3240 报告者即使关闭跨渠道重试，仍可能在**首轮 cache miss 的普通选渠**阶段直接选到不同身份。

## 尚未合并的修复提案

### PR #6392：错误后删除 `encrypted_content` 并重试

- URL：[fix(openai): recover from invalid reasoning signatures](https://github.com/QuantumNous/new-api/pull/6392)
- 状态：**Open**，`mergeable_state=dirty`，尚未合并。
- 日期：创建于 2026-07-21；最后更新于 2026-08-30。
- 作者身份：外部贡献者；截至调研日只有自动化审查，没有人工维护者结论。
- 行为：针对 `thinking_signature_invalid`，以首个非空 `reasoning.encrypted_content` 的 SHA-256 作为会话标识，使用 24 小时滑动缓存；触发后删除所有 reasoning item 的 `encrypted_content`，额外重试一次，并允许恢复状态跨渠道 fallback 延续。
- 关键提交：
  - [050667e0](https://github.com/QuantumNous/new-api/commit/050667e028d7e04c9f0b94025a69c1d8d38553a7)
  - [b762d8c0](https://github.com/QuantumNous/new-api/commit/b762d8c01d6180012fd0ca42246dcecfed7ad629)
  - [30478c52](https://github.com/QuantumNous/new-api/commit/30478c52c024ae6bd79e0837b3e06bc75b5a339d)
  - [cabc2562](https://github.com/QuantumNous/new-api/commit/cabc25623638f5e4108dca8ff2231be253e01739)
- 限制：它是“失败后破坏性降级”，不是保持原账号身份；PR 代码主要识别 `thinking_signature_invalid`，并没有进入 Release。

### PR #7019：精确识别“encrypted content could not be verified”

- URL：[fix(relay): auto-recovery for images, context overflow, and encrypted reasoning items](https://github.com/QuantumNous/new-api/pull/7019)
- 状态：**Open**，`mergeable_state=dirty`，尚未合并。
- 日期：创建于 2026-08-26；最后更新于 2026-08-30。
- 作者身份：外部贡献者；截至调研日只有自动化审查，没有人工维护者结论。
- 行为：识别包含 `encrypted content` 且同时含 `could not be verified/decrypted/parsed` 的错误；先移除 `encrypted_content`，必要时把 summary 转为普通 assistant 消息或删除整个 reasoning item，然后在**同一渠道**重试一次。
- 关联提交：[856e1dcc](https://github.com/QuantumNous/new-api/commit/856e1dcc6578b88ce54db57ab42185d47abce691)。
- 限制：PR 同时包含图片、上下文溢出、tool pairing 等大范围恢复逻辑；仍是有损降级，且未进入 Release。

### PR #5852：Responses replay 恢复

- URL：[feat: add Responses replay and identity sync](https://github.com/QuantumNous/new-api/pull/5852)
- 状态：**Closed, not merged**。
- 日期：创建并关闭于 2026-07-02。
- 行为：记录安全 transcript、清理无效 reasoning/encrypted content、重放失败流。
- 明确结论：作者在 [关闭评论](https://github.com/QuantumNous/new-api/pull/5852#issuecomment-4863563295) 中说明该 patch series 只面向 `freeman-mp` fork，不准备进入 QuantumNous upstream。
- 关联 Release：无。

### PR #5006：Key 级渠道亲和

- URL：[feat: add key-level channel affinity](https://github.com/QuantumNous/new-api/pull/5006)
- 状态：**Closed, not merged**。
- 日期：创建于 2026-05-21；作者于 2026-06-23 关闭。
- 行为：在同一个多 Key 渠道内，把亲和从渠道级细化到 Key index，并在日志中展示 Key 级命中。
- 关联提交：[d97f80d2](https://github.com/QuantumNous/new-api/commit/d97f80d24eb3c5a41a0f07eb2743a5f1269f36a6) 等；最终 head 为 [3cad5d4a](https://github.com/QuantumNous/new-api/commit/3cad5d4acd9a7e18867a8c96e5c7463707ef19b6)。
- 限制：只覆盖“同渠道多 Key”身份漂移，不处理跨渠道 TTL miss；且没有合并。

## 最新官方源码核验

### 核验基线

- 最新官方分支：[`main@93ea55f3`](https://github.com/QuantumNous/new-api/commit/93ea55f3c3102f7bc11cd005570c7965f0f22a03)，提交时间 2026-10-11 06:19:10Z。
- 最新 Release：[`v1.0.0-rc.43@c6741c36`](https://github.com/QuantumNous/new-api/releases/tag/v1.0.0-rc.43)，发布时间 2026-10-10 06:11:03Z。
- `v1.0.0-rc.43..main` 在以下相关文件中没有差异：`service/channel_affinity.go`、`service/channel_select.go`、`middleware/distributor.go`、`setting/operation_setting/channel_affinity_setting.go`、OpenAI adaptor 与 Responses DTO。

### 代码证据

| 事实 | 官方源码证据 | 含义 |
|---|---|---|
| Codex Responses 默认从 `prompt_cache_key` 建亲和，默认 TTL 为 3600 秒 | [`channel_affinity_setting.go#L117-L138`](https://github.com/QuantumNous/new-api/blob/93ea55f3c3102f7bc11cd005570c7965f0f22a03/setting/operation_setting/channel_affinity_setting.go#L117-L138) | 规则中的 `TTLSeconds=0` 不是永久，而是继承 3600 秒 |
| 亲和缓存的值类型是 `HybridCache[int]` | [`channel_affinity.go#L34-L42`](https://github.com/QuantumNous/new-api/blob/93ea55f3c3102f7bc11cd005570c7965f0f22a03/service/channel_affinity.go#L34-L42) | 只存一个整数渠道 ID，不存账号、组织、deployment 或 Key identity |
| Redis 和内存都使用相同的整数值；默认内存 TTL 也是 3600 秒 | [`channel_affinity.go#L82-L107`](https://github.com/QuantumNous/new-api/blob/93ea55f3c3102f7bc11cd005570c7965f0f22a03/service/channel_affinity.go#L82-L107) | Redis 只能延续同一 TTL 语义，不能补足身份信息 |
| 缓存 miss 直接返回 `(0,false)` | [`channel_affinity.go#L590-L627`](https://github.com/QuantumNous/new-api/blob/93ea55f3c3102f7bc11cd005570c7965f0f22a03/service/channel_affinity.go#L590-L627) | miss 不会被识别成“旧状态绑定已丢失” |
| 只有 `found=true` 才进入亲和有效性和 strict 检查；否则回到随机满足条件的渠道 | [`channel_select.go#L312-L352`](https://github.com/QuantumNous/new-api/blob/93ea55f3c3102f7bc11cd005570c7965f0f22a03/service/channel_select.go#L312-L352) | `SkipRetryOnFailure/strict` 不能阻止 TTL miss 后的首轮跨渠道选路 |
| 每次 HTTP 成功后才记录/刷新亲和 | [`distributor.go#L124-L129`](https://github.com/QuantumNous/new-api/blob/93ea55f3c3102f7bc11cd005570c7965f0f22a03/middleware/distributor.go#L124-L129) 与 [`channel_affinity.go#L726-L755`](https://github.com/QuantumNous/new-api/blob/93ea55f3c3102f7bc11cd005570c7965f0f22a03/service/channel_affinity.go#L726-L755) | TTL 是成功请求驱动的滑动续期；错误请求不会建立新绑定 |
| Responses `input` 是 `json.RawMessage` | [`openai_request.go#L1037-L1066`](https://github.com/QuantumNous/new-api/blob/93ea55f3c3102f7bc11cd005570c7965f0f22a03/relaykit/dto/openai_request.go#L1037-L1066) | 网关默认不理解 item 内的加密状态身份 |
| 普通 OpenAI Responses 转换直接返回 request，或只调整 reasoning effort | [`adaptor.go#L681-L749`](https://github.com/QuantumNous/new-api/blob/93ea55f3c3102f7bc11cd005570c7965f0f22a03/relay/channel/openai/adaptor.go#L681-L749) | 没有删除、改写或捕获 `encrypted_content` 的专用路径 |
| 选完渠道后仍单独调用 `GetNextEnabledKey()` | [`distributor.go#L619-L631`](https://github.com/QuantumNous/new-api/blob/93ea55f3c3102f7bc11cd005570c7965f0f22a03/middleware/distributor.go#L619-L631) | 渠道亲和不等于 Key/账号亲和 |
| 多 Key 渠道可随机或轮询 Key | [`channel.go#L206-L255`](https://github.com/QuantumNous/new-api/blob/93ea55f3c3102f7bc11cd005570c7965f0f22a03/model/channel.go#L206-L255) | 即使 channel ID 不变，也可能发生上游身份漂移 |

### 对用户触发路径的逐步还原

1. 会话在渠道 1 成功，`prompt_cache_key → channel_id=1` 被写入亲和缓存。
2. 每次成功请求会用 3600 秒重新写入，所以活跃会话会滑动续期；空闲超过 TTL 后映射自然消失。
3. 此时新增渠道 2。
4. 下一轮请求仍携带渠道 1 产生的历史 reasoning item 和 `encrypted_content`，但亲和查询返回 miss。
5. 由于 `found=false`，`strict` 分支没有执行；选路落回普通候选随机逻辑，可能选择渠道 2。
6. `input` 被原样转发到渠道 2。若渠道 2 属于不同账号、项目、deployment 或其他不兼容安全域，上游无法验证渠道 1 的密文，返回 `invalid_encrypted_content`。
7. 该 400 不会刷新亲和；下一次请求可能再次随机选路。如果恰好重新命中渠道 1，请求会成功，并重新建立绑定。

这条代码路径与“第一条失败、再次请求又成功”的现象一致，也解释了为什么仅设置“亲和失败不重试”无法覆盖 TTL 已经 miss 的请求。

## 最新版是否已解决

| 能力 | `main@93ea55f3` / `v1.0.0-rc.43` |
|---|---|
| 在 TTL 内保持同一渠道 | **有** |
| Redis 跨进程保存渠道绑定 | **有** |
| 绑定自然过期后识别“这是旧加密会话” | **无** |
| cache miss 时拒绝携带旧 encrypted reasoning 的请求 | **无** |
| 持久绑定到上游账号/组织/deployment identity | **无** |
| 渠道 + 具体 Key identity 亲和 | **无** |
| 专门识别 `invalid_encrypted_content` 并恢复 | **无** |
| 已合并的 reasoning input 清除开关 | **无** |
| 无损跨不同上游身份续接 Responses reasoning | **无** |

**结论：最新官方分支仍未解决用户描述的 TTL 过期跨渠道问题。** 当前只有概率缓解和有损恢复提案，没有已合并的完整解决方案。

## 部分相关

| 项目 | 状态 | 相关性与排除理由 |
|---|---|---|
| [Issue #2660](https://github.com/QuantumNous/new-api/issues/2660) | Closed / completed by #2669 | Claude Code 签名跨渠道问题，机制相同但不是 OpenAI Responses `encrypted_content` |
| [Issue #4541](https://github.com/QuantumNous/new-api/issues/4541) | Open | 讨论 Codex 亲和与 429 failover 的冲突；维护者称 Azure/Codex 混用问题促使默认规则改为不跨渠道重试。该评论的布尔值文字存在明显笔误，当前源码和 [commit 5fe8e98e](https://github.com/QuantumNous/new-api/commit/5fe8e98eeba283938c84d8952935b706eb74a045) 可确认默认值为 `true` |
| [Issue #4544](https://github.com/QuantumNous/new-api/issues/4544) | Closed / not planned | 请求 strict 亲和支持 fallback channel；没有 encrypted reasoning 身份约束，也没有维护者方案 |
| [Issue #6021](https://github.com/QuantumNous/new-api/issues/6021) | Open | 讨论亲和渠道失败后如何重置优先级和排除失败渠道；属于选路语义，不解决密文跨账号不可移植 |
| [Issue #6571](https://github.com/QuantumNous/new-api/issues/6571) | Open | 讨论亲和命中低优先级渠道后重复重试同一渠道；不涉及密文身份 |
| [Issue #6538](https://github.com/QuantumNous/new-api/issues/6538) | Closed / not planned | Chat/Claude→Responses 转换丢 `prompt_cache_key`；会导致亲和/缓存变差，但不是已有亲和 TTL 过期。维护者仅表示 Chat 路径可以补，Claude→Responses 暂无计划 |
| [Issue #6594](https://github.com/QuantumNous/new-api/issues/6594) | Open | Responses 流内 terminal failure 被当成功 EOF，导致错误亲和保留；维护者 [说明流已开始后网关不能透明重试，只能记录错误并由客户端重试](https://github.com/QuantumNous/new-api/issues/6594#issuecomment-5691547031) |
| [Issue #5311](https://github.com/QuantumNous/new-api/issues/5311) | Open | 只是建议把现有亲和缓存清理入口移到渠道页，不修改路由算法 |

## 关键词命中但与本问题无关

| 项目 | 为什么无关 |
|---|---|
| [Issue #6822](https://github.com/QuantumNous/new-api/issues/6822) | `encrypted_content` 只出现在 Responses 流样例；实际问题是浮点 `created_at` 导致事件和 usage 丢失 |
| [Issue #4377](https://github.com/QuantumNous/new-api/issues/4377) | `encrypted_content:null` 只在上游响应样例中；实际问题是 new-api 流式响应为空 |
| [Issue #2267](https://github.com/QuantumNous/new-api/issues/2267) | Codex 重连并最终 `stream closed before response.completed`，不是密文验证失败 |
| [Issue #1660](https://github.com/QuantumNous/new-api/issues/1660) | 请求中的空数组 `[]` 被转成 `null`，与账号绑定状态无关 |
| [PR #4819](https://github.com/QuantumNous/new-api/pull/4819) | Responses→Anthropic 协议转换功能，不是跨账号 encrypted reasoning |
| [Issue #5965](https://github.com/QuantumNous/new-api/issues/5965) | `/v1/responses/compact` 与 `response.compaction` 的协议兼容问题，不是亲和过期 |
| [Issue #6677](https://github.com/QuantumNous/new-api/issues/6677) | 检查 malformed reasoning item ID，属于输出 ID 校验，不涉及解密身份 |

## 检索审计

### Issues / PRs

GitHub Issues Search 同时覆盖 Issue、PR 标题、正文和评论。以下为调研时官方仓库的总命中数；宽泛词的结果包含大量无关项，已按上面的标准人工分类。

| 查询词 | 命中数 | 主要相关结果 |
|---|---:|---|
| [`invalid_encrypted_content`](https://github.com/search?q=repo%3AQuantumNous%2Fnew-api+invalid_encrypted_content&type=issues) | 3 | #3240、#3237、#2662，三项均直接相关 |
| [`encrypted_content`](https://github.com/search?q=repo%3AQuantumNous%2Fnew-api+encrypted_content&type=issues) | 8 | #6392、#7019 相关；其余多数是样例或转换问题 |
| [`"encrypted reasoning"`](https://github.com/search?q=repo%3AQuantumNous%2Fnew-api+%22encrypted+reasoning%22&type=issues) | 10 | #6392、#7019、#5852 相关 |
| [`"reasoning item"`](https://github.com/search?q=repo%3AQuantumNous%2Fnew-api+%22reasoning+item%22&type=issues) | 37 | 大量 item 生命周期、ID、协议转换问题；仅少数涉及账号绑定密文 |
| [`prompt_cache_key`](https://github.com/search?q=repo%3AQuantumNous%2Fnew-api+prompt_cache_key&type=issues) | 35 | #3240、#1743、#4541、#6538、#6594 等 |
| [`"channel affinity"`](https://github.com/search?q=repo%3AQuantumNous%2Fnew-api+%22channel+affinity%22&type=issues) | 151 | 包含亲和 UI、统计、重试、并发等大量宽泛结果 |
| [`渠道亲和`](https://github.com/search?q=repo%3AQuantumNous%2Fnew-api+%E6%B8%A0%E9%81%93%E4%BA%B2%E5%92%8C&type=issues) | 177 | 宽泛结果；与 encrypted state 直接相关的仍集中在本文列出的条目 |
| [`"Codex 会话"`](https://github.com/search?q=repo%3AQuantumNous%2Fnew-api+%22Codex+%E4%BC%9A%E8%AF%9D%22&type=issues) | 12 | #1743 最相关；其余多为一般会话/转换问题 |
| [`跨渠道`](https://github.com/search?q=repo%3AQuantumNous%2Fnew-api+%E8%B7%A8%E6%B8%A0%E9%81%93&type=issues) | 95 | #6392、#6021 等；大部分为其他路由或管理功能 |
| [`账号切换`](https://github.com/search?q=repo%3AQuantumNous%2Fnew-api+%E8%B4%A6%E5%8F%B7%E5%88%87%E6%8D%A2&type=issues) | 23 | #2662 相关；大量结果是 UI 登录账号切换 |

### Discussions

- 官方仓库调研时共有 **105** 个 Discussions。
- 已检查每个 Discussion 的标题、正文、accepted answer、全部顶层评论和全部嵌套回复；最大顶层评论数与最大回复数均为 8，没有分页截断。
- 上述 10 个关键词在 Discussions 中均为 **0 命中**。
- 因此官方 Discussions 没有维护者给出的额外解决方案。

### 默认分支提交

- `invalid_encrypted_content`：0 个提交命中。
- `encrypted_content`：0 个提交命中。
- `"encrypted reasoning"`：0 个提交命中。
- `prompt_cache_key`：5 个提交命中，内容集中在缓存字段转发和计费，不是密文身份恢复。
- `"channel affinity"`：17 个提交命中，主要是 #2669、#2799、默认启用、禁用渠道清理、请求头 key source 和 UI。
- 这与源码核验一致：专用 encrypted-state 修复只存在于未合并 PR head，不存在于官方默认分支历史。

## 对“如何不让它产生加密状态”的官方仓库答案

在限定的 QuantumNous/new-api 第一方资料中，答案是：**当前没有已合并配置能让 OpenAI Responses 上游不生成 encrypted reasoning state，也没有已合并配置会在每次请求前自动删除它。**

- `REMOVE_RESPONSES_REASONING_INPUT` 只出现在关闭未合并的 [#2662](https://github.com/QuantumNous/new-api/pull/2662) 和 [#3237](https://github.com/QuantumNous/new-api/pull/3237)。
- 这两个提案删除的是**下一轮回放的 reasoning item**，不是控制首次响应是否产生密文。
- 开放的 [#6392](https://github.com/QuantumNous/new-api/pull/6392) 与 [#7019](https://github.com/QuantumNous/new-api/pull/7019) 只在错误发生后删除 `encrypted_content` 或 reasoning item，再做有损重试。
- 当前主干把整个 Responses `input` 保存在 `json.RawMessage` 中并默认原样转发，因此是否请求/返回加密 reasoning 主要由客户端请求和上游协议行为决定，new-api 没有专门关闭它。

## 可操作结论（区分官方现状与工程建议）

### 官方现状

- 官方当前认可并已经发布的缓解机制只有 `prompt_cache_key → channel_id` 的短期亲和与 strict 重试控制。
- #3240 已证明该机制不能覆盖所有 `invalid_encrypted_content` 场景。
- 最新主干没有 accepted fix；两个最接近的错误恢复 PR 仍开放且冲突。

### 基于官方源码得出的工程建议

以下不是维护者已经合并的结论，而是由上述代码缺口直接推导出的实现方向：

1. 对携带 encrypted reasoning 的会话，绑定粒度应至少是 `provider/security-domain + channel_id + key identity`，而不是只有 `channel_id`。
2. 绑定应覆盖会话实际寿命；单纯把 3600 秒调大只能降低概率，不能解决缓存丢失、清空、迁移或 Redis 数据丢失。
3. 当请求包含 encrypted reasoning 但绑定缺失时，不应静默随机选一个新身份；应明确拒绝、要求新会话，或只在同一 identity group 内选路。
4. 对同一渠道的多 Key，当前官方代码仍会轮询/随机 Key；在 Key 是否共享安全域不确定时，应避免把有状态 Responses 会话放入多 Key 轮询。
5. “删除 encrypted content 后重试”可以作为显式、有损恢复开关，但不应冒充无损 failover；应向调用方暴露连续性降级。

## 最终判断

**有同类官方报告；维护者知道根因；官方已经用渠道亲和做过一次缓解，但截至 `main@93ea55f3` / `v1.0.0-rc.43`，亲和 TTL 过期后跨渠道、跨账号导致 `invalid_encrypted_content` 的问题仍未解决。**

最直接的公开跟踪项仍是 [Issue #3240](https://github.com/QuantumNous/new-api/issues/3240)。当前没有可指向的已合并修复 commit 或包含该修复的 release。
