# LiteLLM `encrypted_content_affinity` 官方方案调查

> 调研截止：2026-10-11（Asia/Shanghai）
> 第一方来源范围严格限定为：LiteLLM 官方文档、`BerriAI/litellm` 官方 GitHub 仓库源码、提交、Pull Requests、Issues 与 Releases。
> 官方文档快照：`https://docs.litellm.ai/docs/response_api.md`，页面声明 `last_updated: 2026-10-10`，本次抓取内容 SHA-256 为 `b6d271e7d4ce653e3c035c6759048c01703144b7af531ce49f3c9592baacb3ec`。
> 源码基线：官方 `main@246c0e126b6ef09654af058a9f0cb966ad78c18d`，其 `pyproject.toml` 版本为 `1.106.0`；调研时最新正式稳定标签为 [`v1.104.2`](https://github.com/BerriAI/litellm/releases/tag/v1.104.2)。因此本文会明确区分“当前主干行为”和“稳定版已发布行为”。

## 结论摘要

1. **当前 LiteLLM 方案不是 Redis affinity cache。** 它把来源部署的 `model_info.id` 写进返回给客户端的 reasoning item：有 `id` 时改写为 `encitem_<base64(...)>`；无 `id`（官方特别点名 Codex）时，还会给 `encrypted_content` 外包一层 `litellm_enc:<base64(model_id)>;<原密文>`。下一轮直接从请求内容中解码来源部署，因此没有 Redis、内存缓存、TTL 或 cache miss 生命周期。
2. **affinity 既不是 `prompt_cache_key`，也不是对密文本身做哈希。** 路由载体是 LiteLLM 改写后的 item `id`，并以包裹后的 `encrypted_content` 作为冗余载体；其中承载的值是 **deployment `model_id`**，不是模型名、账号、API Key 或 `prompt_cache_key`。
3. **正常路径会改写客户端看到的协议字段，但上游收到的是原值。** LiteLLM 会改写 item ID，也会给密文字符串加私有前缀；转发给 OpenAI/Azure 前再恢复原始 ID 和原始密文。它不重新加密，也不改变上游密文本体的字节。
4. **当前 `main` 的不可用处理是“同加密边界 peer，否则有损降级”。** 原部署不健康时，先找有效 `api_base + api_key` 完全相同的健康部署；没有 peer 时删除不可解密 reasoning 的 `encrypted_content` 和 `id`、保留可读 summary/content，无可读文本则删除整个 reasoning item，然后按普通路由继续。
5. **版本差异很大。** 基础能力从 `v1.82.3` 开始；原部署不可用时自动剥离并继续是在 `v1.104.0`；混合来源 item 的精细剥离只进入 `v1.105.0-rc.*`；跨 order/configured fallback 的完整剥离修复仅存在于当前 `main`，调研时还没有正式稳定标签。
6. **可把思路移植到 new-api，但不应照搬实现。** new-api 必须绑定到“渠道 + 具体 Key/凭证身份”，不能只嵌入渠道 ID；并应使用带签名或加密的路由令牌，避免 LiteLLM 当前“明文 base64 deployment ID 可伪造、还可绕过 RPM/TPM 选择”的缺陷。

官方总览：

- [Responses API 文档：The Solution: `encrypted_content_affinity`](https://docs.litellm.ai/docs/response_api#the-solution-encrypted_content_affinity)
- [当前主干实现说明与路由入口](https://github.com/BerriAI/litellm/blob/246c0e126b6ef09654af058a9f0cb966ad78c18d/litellm/router_utils/pre_call_checks/encrypted_content_affinity_check.py#L1-L74)
- [最初问题 Issue #18094](https://github.com/BerriAI/litellm/issues/18094)
- [合并功能 PR #22651](https://github.com/BerriAI/litellm/pull/22651)

## 1. 精确路由算法与生命周期

### 1.1 注册和优先级

配置 `optional_pre_call_checks: [encrypted_content_affinity]` 后，Router 注册 `EncryptedContentAffinityCheck`。当前源码会强制把它移动到普通 `deployment_affinity` 之前，防止用户级/session 级 affinity 先把候选集缩到错误部署。这是早期 [Issue #29733](https://github.com/BerriAI/litellm/issues/29733) 暴露过的真实冲突。

来源：

- [当前 Router 注册并把 encrypted affinity 放在 deployment affinity 前](https://github.com/BerriAI/litellm/blob/246c0e126b6ef09654af058a9f0cb966ad78c18d/litellm/router.py#L2397-L2440)
- [支持的 `OptionalPreCallChecks` 类型](https://github.com/BerriAI/litellm/blob/246c0e126b6ef09654af058a9f0cb966ad78c18d/litellm/types/router.py#L153-L190)

### 1.2 首轮请求

首轮 `input` 中没有 LiteLLM marker，检查器不缩减候选集，仍使用正常负载均衡。它只在 Responses 请求元数据中写入 `encrypted_content_affinity_enabled=true`，通知响应后处理器给后续返回项加来源标记。

因此它不像 `deployment_affinity` 那样把同一用户的所有请求长期固定到一个部署，也不会因 session 数量减少首轮可用配额。

来源：

- [无 marker 时返回完整健康候选集](https://github.com/BerriAI/litellm/blob/246c0e126b6ef09654af058a9f0cb966ad78c18d/litellm/router_utils/pre_call_checks/encrypted_content_affinity_check.py#L323-L376)
- [官方文档：“only pins requests that contain encrypted items”](https://docs.litellm.ai/docs/response_api#the-solution-encrypted_content_affinity)

### 1.3 响应编码

只处理带非空 `encrypted_content` 的 output item，并对每个此类 item 做两层标记：

```text
原 item.id:
rs_xyz

返回给客户端的 item.id:
encitem_<base64("litellm:model_id:{deployment_model_id};item_id:rs_xyz")>

原 encrypted_content:
<provider ciphertext>

返回给客户端的 encrypted_content:
litellm_enc:<base64("model_id:{deployment_model_id}")>;<provider ciphertext>
```

- 有 item ID：ID 和密文都会带来源标记。
- 没有 item ID：仍可从包裹后的密文中取得来源；源码和测试把这条路径明确标为 Codex 兼容路径。
- 不含 `encrypted_content` 的普通 message/tool item 不改写。

来源：

- [item ID codec 与密文 wrapper 格式](https://github.com/BerriAI/litellm/blob/246c0e126b6ef09654af058a9f0cb966ad78c18d/litellm/responses/utils.py#L390-L475)
- [遍历响应 output，并同时改写 ID 和密文](https://github.com/BerriAI/litellm/blob/246c0e126b6ef09654af058a9f0cb966ad78c18d/litellm/responses/utils.py#L477-L534)
- [Codex 无 ID 回归测试](https://github.com/BerriAI/litellm/blob/246c0e126b6ef09654af058a9f0cb966ad78c18d/tests/unit/router_utils/pre_call_checks/test_encrypted_content_affinity_check.py#L535-L632)

### 1.4 下一轮请求解析

检查器按 `input` 数组顺序扫描，找到**第一个**能够解码的 marker 就停止：

1. 先尝试解码 `id` 的 `encitem_...`；
2. 没有可解码 ID 时，再尝试解包 `encrypted_content` 的 `litellm_enc:...`；
3. 两者都没有则视为“没有 encrypted affinity”，继续普通负载均衡。

它不读取 `prompt_cache_key`，不查询 `previous_response_id`，也不尝试解密上游密文来判断来源。

来源：

- [扫描 input、返回第一个 marker 中的 `model_id`](https://github.com/BerriAI/litellm/blob/246c0e126b6ef09654af058a9f0cb966ad78c18d/litellm/router_utils/pre_call_checks/encrypted_content_affinity_check.py#L106-L149)

### 1.5 候选选择

当前 `main` 的决策顺序是：

1. **来源 deployment 仍在当前健康候选集：** 精确返回该 deployment。
2. **来源 deployment 不在当前候选集：** 从当前候选集寻找与来源 deployment 的有效 `(api_base, api_key)` 完全相同的部署；找到一个或多个则只返回这些同边界 peer。
3. **来源已删除、未知、被冷却、切换了 model group/order，或没有同边界 peer：** 删除不能被当前目标解密的 reasoning 密文，保留可读历史，然后把请求交回当前健康候选集按普通策略选择。

同边界比较不是 `model`、账号名或组织名，而是当前配置解析后的精确二元组：

```text
(effective api_base, effective api_key)
```

来源：

- [按 `model_info.id` 精确匹配 deployment](https://github.com/BerriAI/litellm/blob/246c0e126b6ef09654af058a9f0cb966ad78c18d/litellm/router_utils/pre_call_checks/encrypted_content_affinity_check.py#L185-L194)
- [加密边界定义及同边界 peer 搜索](https://github.com/BerriAI/litellm/blob/246c0e126b6ef09654af058a9f0cb966ad78c18d/litellm/router_utils/pre_call_checks/encrypted_content_affinity_check.py#L196-L259)
- [精确 pin、peer pin、无 peer 降级完整分支](https://github.com/BerriAI/litellm/blob/246c0e126b6ef09654af058a9f0cb966ad78c18d/litellm/router_utils/pre_call_checks/encrypted_content_affinity_check.py#L378-L424)
- [原部署不可用的 Issue #43000](https://github.com/BerriAI/litellm/issues/43000) 与 [修复 PR #43015](https://github.com/BerriAI/litellm/pull/43015)

### 1.6 路由优先级与限流

当精确 pin 后候选只剩一个部署，Router 直接返回该部署，不再让普通 routing strategy/RPM/TPM 选择器将它排除。官方文档称其“bypasses rate limits”，理由是其他凭证本来也无法解密该请求。

来源：

- [单 deployment affinity pin 直接返回](https://github.com/BerriAI/litellm/blob/246c0e126b6ef09654af058a9f0cb966ad78c18d/litellm/router.py#L13864-L13907)
- [官方文档 rate-limit bypass 说明](https://docs.litellm.ai/docs/response_api#the-solution-encrypted_content_affinity)

### 1.7 转发前恢复

选好部署后、发给上游前，LiteLLM 会原地恢复：

- `encitem_...` → 原始 `rs_...` ID；
- `litellm_enc:...;<ciphertext>` → 原始 `<ciphertext>`。

所以上游看不到 LiteLLM marker。

来源：

- [恢复 item ID 与 encrypted_content](https://github.com/BerriAI/litellm/blob/246c0e126b6ef09654af058a9f0cb966ad78c18d/litellm/responses/utils.py#L537-L569)
- [Responses 调用上游前执行恢复](https://github.com/BerriAI/litellm/blob/246c0e126b6ef09654af058a9f0cb966ad78c18d/litellm/responses/main.py#L1480-L1492)

### 1.8 后续生命周期

每个成功 turn 的新 encrypted item 都会带上**本次实际服务 deployment** 的 marker。因此：

- 正常连续对话继续 pin 到原 deployment；
- 同边界 peer 接管后，后续 turn 可继续在该边界内工作；
- 无 peer 时有损降级，当前 turn 重新推理，返回的新 item 会改为新 deployment 的 marker，之后转而 pin 到新 deployment；
- 生命周期不由时间控制，而由客户端是否保留 marker、部署 ID/凭证配置是否仍然一致决定。

官方来源：

- [当前文档的完整生命周期和降级说明](https://docs.litellm.ai/docs/response_api#encrypted-content-affinity-multi-region-load-balancing)
- [集成测试：来源被 block/delete 后在 sibling 上有损继续](https://github.com/BerriAI/litellm/blob/246c0e126b6ef09654af058a9f0cb966ad78c18d/tests/integration/routing/test_encrypted_content_affinity_no_peer.py#L192-L240)

## 2. Affinity key 到底是什么

严格地说，当前实现**没有 cache key**，而是有“客户端携带的自描述路由 marker”。

| 候选 | 是否是当前方案的 affinity key/载体 | 精确含义 |
|---|---|---|
| `encrypted_content` 原密文 | 否 | 不哈希、不解析、不以密文本体做 lookup；当前实现只给它加一个可逆前缀作为冗余载体。 |
| reasoning item 原始 `rs_...` ID | 否 | 原始 ID 本身不表达 deployment；LiteLLM 把它改写为 `encitem_<base64(model_id + original_id)>` 后才可路由。 |
| 改写后的 `encitem_...` ID | **是，主要载体** | 内含 `model_id` 和原始 item ID。 |
| 包裹后的 `litellm_enc:...` | **是，冗余载体** | 内含 `model_id` 和原始上游密文；用于 Codex 等不稳定回传 item ID 的客户端。 |
| `prompt_cache_key` | 否 | 完全不参与该检查；属于另一类 session/deployment affinity。 |
| `previous_response_id` | 否 | 由独立的 `responses_api_deployment_check` 处理。 |

关键结论：**它不是“encrypted content → deployment”的服务端映射，而是“在客户端回传的 item 中直接携带 deployment ID”。**

官方来源：

- [文档：直接把 `model_id` 编码进 item ID，无 cache](https://docs.litellm.ai/docs/response_api#the-solution-encrypted_content_affinity)
- [源码：ID 和密文两种 carrier](https://github.com/BerriAI/litellm/blob/246c0e126b6ef09654af058a9f0cb966ad78c18d/litellm/responses/utils.py#L390-L475)
- [源码：解析时先 ID、后 wrapped encrypted_content](https://github.com/BerriAI/litellm/blob/246c0e126b6ef09654af058a9f0cb966ad78c18d/litellm/router_utils/pre_call_checks/encrypted_content_affinity_check.py#L132-L172)

### 历史澄清

PR 开发早期确实出现过一个 cache 版本：

```text
encrypted_content_affinity:v1:{item_id} -> model_id
```

并曾设置 24 小时 TTL。但同一合并 PR 在最终版本中改成自描述 ID，不再使用 `DualCache`；官方支持文档最终要求 `>=1.82.3`，`v1.82.3` 源码已经是无缓存方案。因此不能把早期 review/commit 中的 cache 行为当成现在的实现。

官方来源：

- [早期 cache 版提交 `058fac...`](https://github.com/BerriAI/litellm/blob/058fac848e0f7e16ea0985e9ed7eae897523bb60/litellm/router_utils/pre_call_checks/encrypted_content_affinity_check.py#L1-L267)
- [同一 PR 改为无 cache 编码的提交 `122f534...`](https://github.com/BerriAI/litellm/commit/122f534d8762ff76e72dad0ff264bc0f51b6f7b5)
- [`v1.82.3` 最终实现已声明 no cache/no TTL](https://github.com/BerriAI/litellm/blob/61409275c8d8478d0a7ffc23d375a4fd86717b23/litellm/router_utils/pre_call_checks/encrypted_content_affinity_check.py#L1-L58)

## 3. Value 是 deployment、model、account 还是 key

### 3.1 Marker 中的值

Marker 中唯一持久携带的是：

```text
deployment.model_info.id
```

即 LiteLLM 的 deployment `model_id`。它不是：

- 客户端请求的 `model`/model group 名称；
- OpenAI/Azure account 或 organization ID；
- API Key 本身；
- API Key hash；
- `prompt_cache_key`。

来源：

- [response post-processor 从 `model_info.id` 取值](https://github.com/BerriAI/litellm/blob/246c0e126b6ef09654af058a9f0cb966ad78c18d/litellm/responses/utils.py#L334-L376)
- [文档配置示例显式设置每个 deployment 的 `model_info.id`](https://docs.litellm.ai/docs/response_api#encrypted-content-affinity-multi-region-load-balancing)

### 3.2 运行时 peer 比较值

当 marker 指向的 deployment 当前不能直接服务时，Router 才从配置中读取来源及候选部署的凭证，按以下值判断“同一个 encryption boundary”：

```text
(effective api_base, effective api_key)
```

`litellm_credential_name` 场景会尝试解析 named credential 后再比较。API Key 不写入客户端 marker，也没有服务端缓存 value。

来源：

- [当前 boundary key 实现](https://github.com/BerriAI/litellm/blob/246c0e126b6ef09654af058a9f0cb966ad78c18d/litellm/router_utils/pre_call_checks/encrypted_content_affinity_check.py#L196-L236)
- [named credential 回归测试](https://github.com/BerriAI/litellm/blob/246c0e126b6ef09654af058a9f0cb966ad78c18d/tests/unit/router_utils/pre_call_checks/test_encrypted_content_affinity_check.py#L1111-L1267)
- [仍开放的 named credential 边界报告 Issue #43045](https://github.com/BerriAI/litellm/issues/43045)

## 4. TTL、Redis/内存、miss/过期、不可用和多 worker

| 问题 | 当前实现结论 |
|---|---|
| TTL | **无 TTL。** `deployment_affinity_ttl_seconds` 不控制 encrypted affinity。 |
| Redis | **不需要。** 不查询 Redis。 |
| 进程内缓存 | **不需要。** 不查询内存 affinity cache。 |
| cache miss | 没有 cache miss 概念；只有 marker 缺失、损坏、未知或无法匹配。 |
| marker 缺失/无法解码 | 检查器不 pin，回到普通路由；若请求仍带原始 encrypted reasoning，就可能再次触发 `invalid_encrypted_content`。 |
| marker 指向健康 deployment | 精确 pin；单候选时绕过普通限流/策略选择。 |
| marker 指向不健康 deployment | 当前 `main` 先找同 `(api_base, api_key)` peer；否则剥离 encrypted reasoning 并普通路由。 |
| deployment 已删除 | 无法从旧 ID 取回旧凭证，因此无法发现“其实同边界”的新 deployment；直接进入剥离降级。 |
| 多 worker | marker 自带 deployment ID，所以不需共享 Redis；但所有 worker 必须使用相同且稳定的 `model_info.id`、一致 deployment 配置和一致 marker 编解码版本。 |
| worker/config 滚动升级 | 新旧 worker 对 marker/降级语义不一致时仍可能失败；尤其 WebSocket、fallback 和 mixed-item 修复跨越多个版本。 |
| 凭证轮换 | 没有历史凭证快照。若同一 deployment ID 下更换 API Key、组织或上游安全域，仍会精确 pin 到该 ID，但旧密文可能无法解密。 |

### “过期”实际变成了配置失配

这个方案解决了 new-api 当前“一小时 TTL 过期后重新随机选渠”的问题，但没有消灭状态失配，只是把时间失效改成以下配置生命周期风险：

- client 丢失或清洗 marker；
- deployment ID 改名；
- deployment 被删除；
- 相同 ID 的 API Key/账号被替换；
- 请求经过不认识 `encitem_` / `litellm_enc:` 的中间网关；
- 多 worker 使用不一致配置。

官方来源：

- [官方文档明确“No cache required / no Redis / no TTL”](https://docs.litellm.ai/docs/response_api#the-solution-encrypted_content_affinity)
- [当前实现只从请求 marker 解码，不持有 cache](https://github.com/BerriAI/litellm/blob/246c0e126b6ef09654af058a9f0cb966ad78c18d/litellm/router_utils/pre_call_checks/encrypted_content_affinity_check.py#L64-L85)
- [unknown/removed origin 的 strip-and-dispatch 测试](https://github.com/BerriAI/litellm/blob/246c0e126b6ef09654af058a9f0cb966ad78c18d/tests/unit/router_utils/pre_call_checks/test_encrypted_content_affinity_check.py#L1549-L1605)
- [原部署不可用问题与修复：Issue #43000](https://github.com/BerriAI/litellm/issues/43000)、[PR #43015](https://github.com/BerriAI/litellm/pull/43015)

## 5. 流式、多 encrypted items、OpenAI/Azure/Codex 支持

### 5.1 支持矩阵

| 场景 | 支持情况 | 版本/边界 |
|---|---|---|
| 非流式 Responses | 支持 | `v1.82.3+`。完成响应中的所有带密文 output item 都会改写。 |
| SSE 流式 Responses | 支持 | `v1.82.3` 已对 `response.output_item.added`、`response.output_item.done` 和含完整 response 的终态事件编码。 |
| Native Responses WebSocket | 支持，但较晚 | Issue #41253 暴露 native WS 没有 restore/affinity；PR #41893 修复，首个正式稳定标签为 `v1.103.0`。 |
| 多个 encrypted items，同一来源 | 支持 | 响应端逐个标记；下一轮第一个 marker 决定 pin，其余同来源 item 保留。 |
| 多个 encrypted items，不同来源 | 当前 `main` 有条件支持 | 第一个 marker 决定目标；只保留该目标或同 boundary 可解密的 item，其他 item 去密文、保留摘要。一个请求不可能同时发送给两个上游账号。 |
| 原始、未带 LiteLLM marker 的 encrypted item | **不能恢复来源** | 纯密文不会被反推出 deployment；可能普通选路后报错。旧会话不是自动迁移。 |
| 正规 OpenAI API deployment | 支持 | 官方 Python 示例就是多个 `openai/...` deployment、不同 API Key。 |
| Azure OpenAI 多区域 | 支持 | 官方 Proxy 示例就是多个 Azure endpoint/key。 |
| Codex | 支持 | 实现专门为“不稳定回传 item ID、但回传 encrypted_content”的 Codex 行为增加密文 wrapper。 |
| Chat Completions/embeddings | 不触发 | 没有 Responses encrypted marker 时检查器不影响普通调用。 |

### 5.2 SSE 细节

SSE 处理器会：

- 在 `response.output_item.added` / `response.output_item.done` 中包裹 `encrypted_content`；
- 对带完整 response body 的事件调用通用响应改写器，因此终态 response output 也带 marker；
- 保证客户端无论保存 item event 还是保存最终 response，都能在下一轮回传来源信息。

来源：

- [`v1.82.3` 流式实现](https://github.com/BerriAI/litellm/blob/61409275c8d8478d0a7ffc23d375a4fd86717b23/litellm/responses/streaming_iterator.py#L131-L175)
- [当前主干 SSE 实现](https://github.com/BerriAI/litellm/blob/246c0e126b6ef09654af058a9f0cb966ad78c18d/litellm/responses/streaming_iterator.py#L399-L481)

### 5.3 WebSocket 细节

Native WS 修复后双向都处理 marker：

- client → upstream：恢复 ID、密文与 wrapped response ID；
- upstream → client：给 `output_item.done` 和 `response.completed.output` 加 marker。

来源：

- [Issue #41253](https://github.com/BerriAI/litellm/issues/41253)
- [修复 PR #41893](https://github.com/BerriAI/litellm/pull/41893)
- [当前 WebSocket 双向回归测试](https://github.com/BerriAI/litellm/blob/246c0e126b6ef09654af058a9f0cb966ad78c18d/tests/unit/responses/test_responses_websocket_all_providers.py#L2965-L3079)
- [`v1.103.0` Release](https://github.com/BerriAI/litellm/releases/tag/v1.103.0)

### 5.4 多来源 item 的精确行为

当前主干不是“为每个 item 分别路由”，而是：

1. 第一个 marker 决定本请求的 deployment/boundary；
2. 遍历全部 reasoning item；
3. 目标可解密的保留；
4. 其他来源的 item 删除 `encrypted_content` 和 `id`，保留 summary/content；无可读文本则删除整个 item。

来源：

- [first-marker-wins 提取逻辑](https://github.com/BerriAI/litellm/blob/246c0e126b6ef09654af058a9f0cb966ad78c18d/litellm/router_utils/pre_call_checks/encrypted_content_affinity_check.py#L106-L130)
- [逐 item 判断目标能否解密](https://github.com/BerriAI/litellm/blob/246c0e126b6ef09654af058a9f0cb966ad78c18d/litellm/router_utils/pre_call_checks/encrypted_content_affinity_check.py#L261-L317)
- [OpenAI + Azure 混合来源回归测试](https://github.com/BerriAI/litellm/blob/246c0e126b6ef09654af058a9f0cb966ad78c18d/tests/unit/router_utils/pre_call_checks/test_encrypted_content_affinity_check.py#L1982-L2061)

## 6. 是否删除或改写密文

答案必须分“正常路径”和“降级路径”。

### 6.1 正常路径

**会改写客户端看到的字符串，但不改写上游密文本体：**

- item ID 被替换成 `encitem_...`；
- `encrypted_content` 被替换成 `litellm_enc:<metadata>;<原密文>`；
- 发给上游前剥掉 LiteLLM wrapper，恢复原始 ID 和原始 ciphertext；
- 不解密、不重加密、不截断原 ciphertext。

这意味着它事实上扩展了 OpenAI Responses wire format，而不只是“内部路由”。任何中间层或客户端若校验 ID 格式、重写字段或不原样回传 `encrypted_content`，都会破坏 affinity。

来源：

- [编码与解码 round-trip](https://github.com/BerriAI/litellm/blob/246c0e126b6ef09654af058a9f0cb966ad78c18d/litellm/responses/utils.py#L390-L475)
- [恢复后再上游调用](https://github.com/BerriAI/litellm/blob/246c0e126b6ef09654af058a9f0cb966ad78c18d/litellm/responses/main.py#L1480-L1492)

### 6.2 不可用/跨边界降级路径

**会删除密文：**

- reasoning item 有可读 `summary` 或 `content`：删除 `encrypted_content` 和 `id`，留下可读文本；
- reasoning item 没有可读文本：删除整个 item；
- 对 `/v1/messages` bridge 的 thinking block，则删除不能验证的整个 block。

这能避免 `invalid_encrypted_content`，但属于**有损续接**：隐藏推理连续性、缓存价值和潜在模型内部状态都会丢失，本 turn 重新推理。

来源：

- [Responses strip 规则](https://github.com/BerriAI/litellm/blob/246c0e126b6ef09654af058a9f0cb966ad78c18d/litellm/responses/utils.py#L571-L621)
- [跨边界自动 strip 的路由分支](https://github.com/BerriAI/litellm/blob/246c0e126b6ef09654af058a9f0cb966ad78c18d/litellm/router_utils/pre_call_checks/encrypted_content_affinity_check.py#L411-L424)
- [混合来源的保留/剥离测试](https://github.com/BerriAI/litellm/blob/246c0e126b6ef09654af058a9f0cb966ad78c18d/tests/unit/router_utils/pre_call_checks/test_encrypted_content_affinity_check.py#L1910-L2000)

## 7. 配置方法与版本

### 7.1 Python SDK

```python
from litellm import Router

router = Router(
    model_list=[
        {
            "model_name": "gpt-responses",
            "litellm_params": {
                "model": "openai/gpt-example",
                "api_key": "<deployment-a-key>",
            },
            "model_info": {"id": "deployment-a"},
        },
        {
            "model_name": "gpt-responses",
            "litellm_params": {
                "model": "openai/gpt-example",
                "api_key": "<deployment-b-key>",
            },
            "model_info": {"id": "deployment-b"},
        },
    ],
    optional_pre_call_checks=["encrypted_content_affinity"],
)
```

### 7.2 Proxy `config.yaml`

```yaml
model_list:
  - model_name: gpt-responses
    litellm_params:
      model: azure/gpt-example
      api_base: https://region-a.example.openai.azure.com/
      api_key: os.environ/AZURE_API_KEY_A
    model_info:
      id: deployment-a

  - model_name: gpt-responses
    litellm_params:
      model: azure/gpt-example
      api_base: https://region-b.example.openai.azure.com/
      api_key: os.environ/AZURE_API_KEY_B
    model_info:
      id: deployment-b

router_settings:
  enable_pre_call_checks: true
  optional_pre_call_checks:
    - encrypted_content_affinity
```

运维要求：给 deployment 显式配置稳定、唯一、跨 worker 一致的 `model_info.id`。虽然这是从源码得出的部署约束，而不是文档中的 MUST 字样，但 marker 只存这个 ID，多 worker 能否解析完全依赖它稳定。

官方来源：

- [官方 Python 与 Proxy 配置](https://docs.litellm.ai/docs/response_api#encrypted-content-affinity-multi-region-load-balancing)
- [Router 配置类型](https://github.com/BerriAI/litellm/blob/246c0e126b6ef09654af058a9f0cb966ad78c18d/litellm/types/router.py#L153-L190)

### 7.3 按 model group 开启

当前源码支持：

```yaml
router_settings:
  model_group_affinity_config:
    gpt-responses:
      - encrypted_content_affinity
```

并保证该 group 的 encrypted affinity 优先于全局 deployment affinity。该能力由 PR #29760 引入，首个正式稳定系列为 `v1.90.0`。

来源：

- [PR #29760](https://github.com/BerriAI/litellm/pull/29760)
- [当前 per-group enable 判断](https://github.com/BerriAI/litellm/blob/246c0e126b6ef09654af058a9f0cb966ad78c18d/litellm/router_utils/pre_call_checks/encrypted_content_affinity_check.py#L91-L104)
- [per-group 优先级回归测试](https://github.com/BerriAI/litellm/blob/246c0e126b6ef09654af058a9f0cb966ad78c18d/tests/unit/router_utils/pre_call_checks/test_encrypted_content_affinity_check.py#L1653-L1825)
- [`v1.90.0` Release](https://github.com/BerriAI/litellm/releases/tag/v1.90.0)

### 7.4 版本矩阵

| 版本 | 能力/变化 | 官方证据 |
|---|---|---|
| `v1.82.3` | 首个官方文档支持版本；无 cache marker、Codex 无 ID wrapper、非流式与 SSE 路径。 | [Release](https://github.com/BerriAI/litellm/releases/tag/v1.82.3)、[固定版本源码](https://github.com/BerriAI/litellm/blob/61409275c8d8478d0a7ffc23d375a4fd86717b23/litellm/router_utils/pre_call_checks/encrypted_content_affinity_check.py#L1-L172)、[文档版本修正提交](https://github.com/BerriAI/litellm/commit/3de0a2a834c00f740f02de11db8f8b394efe4f69) |
| `v1.85.0` | 支持按同 `(api_base, api_key)` encryption boundary 找 peer；当时无 peer 采用 fail-fast，而非当前降级。 | [PR #27820](https://github.com/BerriAI/litellm/pull/27820)、[Release](https://github.com/BerriAI/litellm/releases/tag/v1.85.0) |
| `v1.90.0` | per-model-group 配置；encrypted affinity 排在 deployment affinity 前。 | [PR #29760](https://github.com/BerriAI/litellm/pull/29760)、[Issue #29733](https://github.com/BerriAI/litellm/issues/29733)、[Release](https://github.com/BerriAI/litellm/releases/tag/v1.90.0) |
| `v1.102.0` | auto-router/model-group 变化时剥离跨边界 reasoning，而不是直接 503。 | [Issue #40237](https://github.com/BerriAI/litellm/issues/40237)、[PR #40280](https://github.com/BerriAI/litellm/pull/40280)、[Release](https://github.com/BerriAI/litellm/releases/tag/v1.102.0) |
| `v1.103.0` | Native Responses WebSocket restore/affinity；provider/named credential 边界相关修正。 | [Issue #41253](https://github.com/BerriAI/litellm/issues/41253)、[PR #41893](https://github.com/BerriAI/litellm/pull/41893)、[Release](https://github.com/BerriAI/litellm/releases/tag/v1.103.0) |
| `v1.104.0` | 来源 deployment cooled down/删除且无 peer 时，改为 strip-and-dispatch 有损续接。 | [Issue #43000](https://github.com/BerriAI/litellm/issues/43000)、[PR #43015](https://github.com/BerriAI/litellm/pull/43015)、[Release](https://github.com/BerriAI/litellm/releases/tag/v1.104.0) |
| `v1.105.0-rc.*` | 精细处理“同一请求混合多个来源”的 item，只保留目标可解密项。调研时尚无 `v1.105.0` 正式稳定标签。 | [PR #43781](https://github.com/BerriAI/litellm/pull/43781)、[RC Release](https://github.com/BerriAI/litellm/releases/tag/v1.105.0-rc.1) |
| 当前 `main` / `1.106.0-dev` | order-based/configured fallback 每一跳都剥离目标不能解密的 reasoning；调研时尚未进入正式稳定标签。 | [PR #45393](https://github.com/BerriAI/litellm/pull/45393)、[固定主干源码](https://github.com/BerriAI/litellm/blob/246c0e126b6ef09654af058a9f0cb966ad78c18d/litellm/router_utils/pre_call_checks/encrypted_content_affinity_check.py#L323-L424) |

**实际选型建议：** 不能只看到文档写“requires >= 1.82.3”就认为全部故障路径已成熟。若依赖当前文档描述的“部署不可用后仍继续”“mixed items”“fallback 每跳安全”，需要分别核对 `v1.104+`、`v1.105 RC+` 与尚未稳定发布的 `main` 修复。

## 8. 能否直接移植到 new-api，仍有哪些漏洞

### 8.1 能否直接移植

**结论：架构思路可移植，LiteLLM 实现不能原样照搬。**

new-api 当前已有适合落点：

- Responses `input` 是 `json.RawMessage`，可在选渠前扫描并在上游转发前恢复：`relaykit/dto/openai_request.go:1008`。
- 普通 Responses 非流式 handler 已读取完整原始 body，可在写回前改写 output：`relay/channel/openai/relay_responses.go:31`。
- SSE handler 已逐事件解析 `response.output_item.*` 和 terminal response，可插入 marker：`relay/channel/openai/relay_responses.go:81`。
- 当前渠道亲和只缓存 `channel_id` 且默认 TTL 3600 秒：`service/channel_affinity.go:550`、`service/channel_affinity.go:727`、`setting/operation_setting/channel_affinity_setting.go:117`。
- 选中渠道后还会调用 `GetNextEnabledKey()`，多 Key 会 random/polling：`middleware/distributor.go:657`、`model/channel.go:214`。因此仅编码 channel ID 仍会在同渠道内漂移到不同账号凭证。
- Responses WebSocket 在一个物理连接内已经锁定 channel 和具体 key/index，但断线重连或 HTTP 多 turn 仍缺少跨请求的密文身份 marker：`relay/responses_websocket.go:266`、`relay/responses_websocket.go:354`。

### 8.2 new-api 应移植的核心流程

建议实现为独立于现有短 TTL session affinity 的 `encrypted-state affinity`：

1. **选渠前解析 marker。** 从 Responses `input` 中寻找经过 new-api 认证的 marker，并让它比 `prompt_cache_key` affinity、普通优先级、权重和重试候选具有更高优先级。
2. **绑定物理身份，而非只有渠道。** 至少绑定：`channel_id + stable credential identity`；多 Key 场景必须锁定具体 Key 身份，不能继续调用普通轮询。
3. **响应端覆盖所有形态。** 同时处理非流式 response、SSE `output_item.added/done`、terminal full response、Responses WebSocket 双向 frame。
4. **上游前恢复。** 上游只能看到原 item ID 和原 ciphertext。
5. **同安全域 fallback。** 原身份不可用时，只允许相同 provider/security-domain 的凭证接管；没有兼容身份时按配置选择：严格报 `encrypted_state_affinity_lost`，或显式有损 strip-and-reason-fresh。
6. **所有 retry/fallback hop 重做检查。** 不能仅在第一次选渠时 pin；优先级下降、auto group 切换、重试和 fallback 都必须限制在同安全域，或先 strip。
7. **返回新 marker。** 有损迁移后的新 turn 应标记实际服务身份，使后续 turn 稳定到新域。

### 8.3 不应照搬的部分：建议使用签名路由令牌

LiteLLM marker 只是 base64，不带签名。new-api 更安全的 carrier 应类似：

```text
version
channel_id
credential_id 或不可逆 key fingerprint
provider / normalized base URL / organization / project 的安全域指纹
credential generation
original item id
HMAC/AEAD authentication tag
```

要求：

- 绝不放原 API Key；
- 多 worker 共用 marker 签名密钥或 key ring；
- marker 可验证但客户端不可伪造；
- Key 顺序调整不应改变 identity，因此不能只存多 Key 数组 index；
- 可把“安全域指纹”放进 token，使原渠道已删除时仍能寻找同域 peer，而不需要保存旧明文凭证；
- 支持签名密钥轮换，否则“无 TTL”会被网关签名密钥轮换重新打断。

### 8.4 LiteLLM 当前仍存在的漏洞/边界

#### A. Marker 可伪造，并可影响路由和限流

`encitem_` 只是 base64；任何客户端都能构造一个有效 deployment ID。只要目标 ID 在当前健康候选集，检查器就会 pin，并走官方设计的 rate-limit bypass。它不会绕过 model group/access control，但会让已授权调用者操纵组内具体 deployment 和普通 RPM/TPM 选择。

来源：

- [无签名 base64 codec](https://github.com/BerriAI/litellm/blob/246c0e126b6ef09654af058a9f0cb966ad78c18d/litellm/responses/utils.py#L390-L423)
- [有效 ID 直接 pin](https://github.com/BerriAI/litellm/blob/246c0e126b6ef09654af058a9f0cb966ad78c18d/litellm/router_utils/pre_call_checks/encrypted_content_affinity_check.py#L378-L391)
- [pin 后直接返回 deployment](https://github.com/BerriAI/litellm/blob/246c0e126b6ef09654af058a9f0cb966ad78c18d/litellm/router.py#L13880-L13886)

#### B. Marker 只绑定 deployment ID，不绑定“生成当时的凭证版本”

若保持同一 `model_info.id`，但替换其 API Key、OpenAI organization/project、Azure resource 或后端账号，旧 marker 仍精确命中该 deployment。代码会因为 ID 相同而直接信任，不会比较历史凭证，旧密文仍可能报错。

来源：

- [ID 命中优先于 boundary fallback](https://github.com/BerriAI/litellm/blob/246c0e126b6ef09654af058a9f0cb966ad78c18d/litellm/router_utils/pre_call_checks/encrypted_content_affinity_check.py#L378-L397)

#### C. Encryption boundary 定义过于简化

当前 peer 定义只有 `(api_base, api_key)`：

- 没有显式纳入 OpenAI organization/project headers、Azure tenant/deployment 语义或其他 provider 身份字段；
- 字符串完全相等可能把仅有斜杠/规范化差异的同一 endpoint 判成不同边界，导致不必要 strip；
- 反过来，若相同 key/base 搭配不同组织/项目 header，可能被错误视为兼容。

此外 [Issue #43045](https://github.com/BerriAI/litellm/issues/43045) 在调研时仍为 Open，报告 named credential peer 匹配仍有边缘问题，尽管主干已有相应解析和单元测试。

来源：

- [边界精确定义源码](https://github.com/BerriAI/litellm/blob/246c0e126b6ef09654af058a9f0cb966ad78c18d/litellm/router_utils/pre_call_checks/encrypted_content_affinity_check.py#L196-L259)

#### D. 原 deployment 删除后，无法发现同凭证的新 deployment

Marker 只保存旧 deployment ID，没有保存安全域指纹。旧 deployment 一旦从配置删除，Router 无法取得它原来的 `api_base/api_key`，即使新 deployment 实际仍使用同一账号/Key，也无法证明是 peer，只能 strip。

来源：

- [origin lookup 失败即没有 boundary](https://github.com/BerriAI/litellm/blob/246c0e126b6ef09654af058a9f0cb966ad78c18d/litellm/router_utils/pre_call_checks/encrypted_content_affinity_check.py#L238-L259)
- [removed-origin 测试](https://github.com/BerriAI/litellm/blob/246c0e126b6ef09654af058a9f0cb966ad78c18d/tests/unit/router_utils/pre_call_checks/test_encrypted_content_affinity_check.py#L1039-L1067)

#### E. First-marker-wins，多个安全域不能同时无损续接

一个请求最终只能发往一个上游身份。多个 encrypted item 来自不同账号时，第一个 marker 决定路由，其余不兼容 item 必须有损 strip；item 顺序会影响保留哪一组推理。

来源：

- [只返回第一个 `model_id`](https://github.com/BerriAI/litellm/blob/246c0e126b6ef09654af058a9f0cb966ad78c18d/litellm/router_utils/pre_call_checks/encrypted_content_affinity_check.py#L106-L130)
- [混合来源测试](https://github.com/BerriAI/litellm/blob/246c0e126b6ef09654af058a9f0cb966ad78c18d/tests/unit/router_utils/pre_call_checks/test_encrypted_content_affinity_check.py#L1982-L2061)

#### F. 未标记的 encrypted item 仍是盲区

旧会话、其他网关产生的 raw `encrypted_content` 或被客户端去掉 marker 的 item 无法路由。更细的当前测试还显示：当同一请求已有一个 marked item 触发 pin 时，无法识别来源的 raw encrypted item 可能被归因到该 pin 而保留；如果它实际来自另一个账号，上游仍可能失败。

来源：

- [解析器只认识 `encitem_` / `litellm_enc:`](https://github.com/BerriAI/litellm/blob/246c0e126b6ef09654af058a9f0cb966ad78c18d/litellm/router_utils/pre_call_checks/encrypted_content_affinity_check.py#L132-L172)
- [unknown marker 被 strip、unmarked content 被保留的测试](https://github.com/BerriAI/litellm/blob/246c0e126b6ef09654af058a9f0cb966ad78c18d/tests/unit/router_utils/pre_call_checks/test_encrypted_content_affinity_check.py#L2283-L2358)

#### G. 有损降级不是“透明容灾”

strip 后虽然对话可继续，但 reasoning 连续性已经丢失；如果 summary 为空，整个 reasoning item 消失。业务必须决定这是可接受降级，还是应严格失败并要求用户等待原凭证恢复。

来源：

- [strip 规则](https://github.com/BerriAI/litellm/blob/246c0e126b6ef09654af058a9f0cb966ad78c18d/litellm/responses/utils.py#L571-L608)

#### H. 发布版与文档/主干存在能力落差

调研时最新稳定 `v1.104.2` 不含：

- PR #43781 的 mixed-origin 精细剥离（仅 `v1.105.0-rc.*`）；
- PR #45393 的每个 fallback hop 安全剥离（仅当前 `main`）。

直接按最新文档理解旧版部署会高估其可靠性。

来源：

- [`v1.104.2` Release](https://github.com/BerriAI/litellm/releases/tag/v1.104.2)
- [PR #43781](https://github.com/BerriAI/litellm/pull/43781)
- [PR #45393](https://github.com/BerriAI/litellm/pull/45393)

### 8.5 对 new-api 的最终建议

建议借鉴 LiteLLM 的“**把来源身份随 opaque encrypted item 一起带回客户端**”这一核心，而不是照抄 `encitem_<base64(model_id)>`：

1. 使用认证过的自描述 token，解决 Redis TTL 和多 worker 一致性；
2. token 绑定渠道、具体 Key/凭证 generation 和完整安全域；
3. encrypted affinity 优先于 `prompt_cache_key` affinity、权重、优先级和重试；
4. 默认严格模式不跨安全域；可选有损恢复模式才 strip；
5. 所有 HTTP/SSE/WebSocket、multi-item、multi-key、auto group、优先级下降、fallback、渠道删除、Key 轮换都必须有回归测试；
6. 对旧会话明确提示“无来源 marker，无法安全无损迁移”，不要假装普通随机重试可以恢复。

这样可以解决本次真实路径：

```text
渠道 1 产生 encrypted reasoning
→ prompt_cache_key affinity 过期
→ 新增渠道 2
→ 普通选渠漂移到渠道 2
→ invalid_encrypted_content
```

同时避免只把问题从“跨渠道”缩小成“同渠道多 Key 继续随机漂移”。

## 官方证据索引

### 文档与 Release

- [Responses API / Encrypted Content Affinity](https://docs.litellm.ai/docs/response_api#the-solution-encrypted_content_affinity)
- [`v1.82.3`](https://github.com/BerriAI/litellm/releases/tag/v1.82.3)
- [`v1.90.0`](https://github.com/BerriAI/litellm/releases/tag/v1.90.0)
- [`v1.102.0`](https://github.com/BerriAI/litellm/releases/tag/v1.102.0)
- [`v1.103.0`](https://github.com/BerriAI/litellm/releases/tag/v1.103.0)
- [`v1.104.0`](https://github.com/BerriAI/litellm/releases/tag/v1.104.0)
- [`v1.104.2`](https://github.com/BerriAI/litellm/releases/tag/v1.104.2)

### 核心 PR / Issues

- [Issue #18094：Codex encrypted content could not be verified](https://github.com/BerriAI/litellm/issues/18094)
- [PR #22651：Add support for encrypted content affinity](https://github.com/BerriAI/litellm/pull/22651)
- [PR #23854：最低版本文档修正](https://github.com/BerriAI/litellm/pull/23854)
- [Issue #29733：deployment affinity callback 顺序可绕过 encrypted affinity](https://github.com/BerriAI/litellm/issues/29733)
- [PR #29760：per-model encrypted affinity](https://github.com/BerriAI/litellm/pull/29760)
- [Issue #40237 / PR #40280：auto-router 跨 group](https://github.com/BerriAI/litellm/issues/40237)
- [PR #40280](https://github.com/BerriAI/litellm/pull/40280)
- [Issue #41253 / PR #41893：Native WebSocket](https://github.com/BerriAI/litellm/issues/41253)
- [PR #41893](https://github.com/BerriAI/litellm/pull/41893)
- [Issue #43000 / PR #43015：来源 deployment 不可用](https://github.com/BerriAI/litellm/issues/43000)
- [PR #43015](https://github.com/BerriAI/litellm/pull/43015)
- [Issue #43045：named credential boundary peer](https://github.com/BerriAI/litellm/issues/43045)
- [PR #43781：混合来源 item 精细 strip](https://github.com/BerriAI/litellm/pull/43781)
- [PR #45393：fallback hop 安全 strip](https://github.com/BerriAI/litellm/pull/45393)
