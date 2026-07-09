# New-API 扣费异常排查记录

## 问题1：用户取消请求（client_gone）导致大额扣费

### 现象
- 所有超过 $10 的扣费记录都是 `end_reason: client_gone`
- `completion_tokens: 0`，`frt: -1000`（没收到第一个 token）
- `local_count_tokens: true`（本地估算，非上游返回）
- `cache_creation_tokens: 0`, `cache_tokens: 0`
- 模型均为 claude-opus-4-8，prompt 400~575 万 tokens

### 典型记录
| prompt_tokens | quota | 金额 |
|--------------|-------|------|
| 5,750,987 | 14,377,468 | $28.75 |
| 5,438,173 | 13,595,433 | $27.19 |
| 4,970,773 | 12,426,933 | $24.85 |
| 4,250,763 | 10,626,908 | $21.25 |

### 根因
1. 用户取消时，上游还没返回 usage → 走 fallback 路径 `ResponseText2Usage()`
2. fallback 用 `GetEstimatePromptTokens()` 本地估算，按完整 prompt 全价计费
3. 本地估算不含缓存信息 → 本该有缓存折扣的 tokens 按原价计费
4. 上游大概率没开始生成（frt=-1000），实际不应该收取完整 prompt 费用

### 代码位置
- `relay/channel/claude/relay-claude.go:838-851` — HandleStreamFinalResponse fallback 逻辑
- `service/usage_helpr.go:22-29` — ResponseText2Usage 本地估算
- `service/text_quota.go:177-183` — usage 为 nil 时用估算值

## 问题1b：上游超时/无响应（end_reason=eof）但 completion=0，同样大额扣费

### 现象
- `end_reason: eof`（流正常结束），但 `completion_tokens: 0`
- `local_count_tokens: true`，`frt: -1000`
- 请求耗时长（54s~213s），上游返回 HTTP 200 但流里没有 usage 事件
- 与问题1 **根因完全相同**：上游没返回 usage → 走本地估算 fallback

### 典型记录
| prompt_tokens | quota | 金额 | 耗时 | end_reason |
|--------------|-------|------|------|-----------|
| 3,175,727 | 7,939,318 | $15.88 | 99s | eof |
| 3,173,224 | 7,933,060 | $15.87 | 210s | eof |
| 3,172,753 | 7,931,883 | $15.86 | 213s | eof |
| 1,549,527 | 3,873,818 | $7.75 | 54s | eof |

### 根因
与问题1 相同。上游返回了 HTTP 200 + event stream，但流中没有包含 message_start（含 usage）事件就结束了。可能是上游处理超时、内部错误等，但 new-api 仍然按本地估算的全量 prompt 扣费。

## 问题2：cache_creation_tokens（缓存写入）导致的大额扣费 — 正常

### 现象
- `local_count_tokens: false`（上游返回了真实 usage）
- `prompt_tokens` 很小（2~2000），`completion_tokens` 也很小
- 但 `cache_creation_tokens_5m` 很大（几十万~近百万）
- 扣费 $5~$7

### 典型记录
| prompt | completion | cache_write_5m | quota | 金额 |
|--------|-----------|---------------|-------|------|
| 2 | 139 | 927,090 | 2,898,899 | $5.80 |
| 2 | 82 | 927,090 | 2,899,018 | $5.80 |
| 1,877 | 9,974 | 0 | 3,247,686 | $6.50 |

### 验算
以 prompt=2, comp=139, cache_write_5m=927,090 为例：
```
cache_creation = 927,090 * 1.25 (5m ratio) = 1,158,862.5
completion = 139 * 5 = 695
total = (2 + 1,158,862.5 + 695) * 2.5 * 1 = 2,898,899 ✓
```

### 结论
**这类扣费是正确的。** cache_creation_tokens 是 Anthropic 的 prompt caching 写入费用，上游确实会收取。首次缓存写入贵，但后续请求命中缓存后 prompt 按 0.1x 折扣计费，长期看是划算的。

## 问题3：prompt=2 但大额扣费 — 正常（也是 cache 写入）

### 现象
| prompt | completion | cache_write_5m (在 other 里) | quota | 金额 |
|--------|-----------|---------------------------|-------|------|
| 2 | 598 | 805,485 | 2,524,621 | $5.05 |
| 2 | 4,045 | 809,100 | 3,097,124 | $6.19 |
| 2 | 139 | 927,090 | 2,898,899 | $5.80 |

### 说明
这些也是 cache_creation 费用。`cache_creation_tokens_5m` 只记录在 `other` 字段中，顶层 `prompt_tokens` 仅为 2（Anthropic 语义下 prompt 不包含缓存写入 token）。验算全部匹配，**扣费正确**。

---

## 总结

| 问题 | 严重性 | 是否需要修复 |
|------|--------|------------|
| 问题1/1b: client_gone/eof + local_count 导致全价扣费 | **严重** | 是 — 上游没收费但本地按全价扣 |
| 问题2: cache_creation 正常扣费 | 正常 | 否 — 上游确实收取 |
| 问题3: prompt=2 大额扣费 | 正常 | 否 — 也是 cache 写入费用，验算匹配 |

---

## 修复方案

### 上游 PR 分析

| PR | 状态 | 方案 | 缺陷 |
|----|------|------|------|
| [#4199](https://github.com/QuantumNous/new-api/pull/4199) | Open，未合并 | 在 `PostTextConsumeQuota` 中，当 `IsStream && !IsNormalEnd() && SendResponseCount==0 && usage==nil` 时清零 usage | **漏掉 `eof+completion=0` 场景**：`eof` 被 `IsNormalEnd()` 归为"正常结束"，但上游可能 200 响应却无内容事件 |
| [#5291](https://github.com/QuantumNous/new-api/pull/5291) | Open，未合并 | 对 /v1/responses 端点，客户端断开后保留预扣费 | 仅覆盖 Responses API，不通用 |

### 我们的方案：两层防护

#### 第一层（核心）— 计费层集中拦截

在 `service/text_quota.go` 新增 `shouldZeroLocalUsage()` 函数，当同时满足以下条件时返回 true：

1. **流式请求**（`info.IsStream`）
2. **本地估算**（`ContextKeyLocalCountTokens == true`）— 意味着上游没返回 usage
3. **无实际输出**（`CompletionTokens == 0` 且 `!info.HasSendResponse()`）

在 `calculateTextQuotaSummary` 函数的 `usage == nil` 检查之前调用此函数。当返回 true 时，将 usage 清零为空 `&dto.Usage{}`。

**效果链**：`TotalTokens=0` → 已有的 line 303 守卫 `summary.Quota = 0` → `SettleBilling()` 结算 `delta = 0 - preConsumed < 0` → 退还预扣费。

**关键设计**：使用 `!info.HasSendResponse()` 而非 `!info.StreamStatus.IsNormalEnd()` 作为判断条件。
- `HasSendResponse()` 检查 `FirstResponseTime > StartTime`，只有 `StreamScannerHandler` 收到真实数据事件才会更新
- 这样就覆盖了 `eof+completion=0` 的场景（PR #4199 遗漏的）
- 同时也覆盖 `client_gone`、`timeout` 等所有无输出结束的场景

```go
func shouldZeroLocalUsage(ctx *gin.Context, info *relaycommon.RelayInfo, usage *dto.Usage) bool {
    if !info.IsStream {
        return false
    }
    if !common.GetContextKeyBool(ctx, constant.ContextKeyLocalCountTokens) {
        return false
    }
    if usage != nil && usage.CompletionTokens > 0 {
        return false
    }
    return !info.HasSendResponse()
}
```

#### 第二层（纵深防御）— handler 层阻止虚假 usage 生成

| 文件 | 修改 |
|------|------|
| `relay/channel/claude/relay-claude.go` `HandleStreamFinalResponse` | 仅当 `len(responseText) > 0 \|\| claudeInfo.Usage.PromptTokens > 0` 时才调用 `ResponseText2Usage` |
| `relay/channel/openai/relay-openai.go` `OaiStreamHandler` | 仅当 `len(responseText) > 0 \|\| toolCount > 0` 时才调用 `ResponseText2Usage` |
| `relay/channel/openai/relay_responses.go` | 已有保护，无需修改 |

### 场景覆盖表

| 场景 | end_reason | completion | HasSendResponse | local_count | 修复前 | 修复后 |
|------|-----------|-----------|----------------|-------------|--------|--------|
| 用户取消，无输出 | client_gone | 0 | false | true | 扣 $10-28 | **退款** |
| 上游 EOF，无内容事件 | eof | 0 | false | true | 扣 $7-16 | **退款** |
| 超时，无输出 | timeout | 0 | false | true | 扣费 | **退款** |
| 用户取消，有部分输出 | client_gone | >0 | true | true | 保留本地估算 | 保留本地估算 |
| 正常完成，上游返回 usage | done | >0 | true | false | 正常扣费 | 正常扣费 |
| cache_creation 大额扣费 | done | >0 | true | false | 正常扣费 | 正常扣费 |
| Claude message_start 后取消 | client_gone | 0 | true | - | 扣估算 prompt | 扣估算 prompt（HasSendResponse=true） |

### 修改文件清单

| 文件 | 改动类型 |
|------|---------|
| `service/text_quota.go` | 新增 `shouldZeroLocalUsage()`，在 `calculateTextQuotaSummary` 顶部调用 |
| `relay/channel/claude/relay-claude.go` | `HandleStreamFinalResponse` 中守卫 `ResponseText2Usage` 调用 |
| `relay/channel/openai/relay-openai.go` | `OaiStreamHandler` 中守卫 `ResponseText2Usage` 调用 |

### 不需要修改

- `stream_status.go` — `IsNormalEnd()` 语义不变，其他代码依赖它
- `billing.go` — `SettleBilling()` 已正确处理 quota=0（退还 delta）
- `controller/relay.go` — defer 退款路径仅用于流前错误
- `relay_info.go` — `HasSendResponse()` 已存在且正确

### 验证方法

1. `go build ./...` — 编译通过
2. `go test ./service/... ./relay/...` — 现有测试通过
3. 对照 billing_issues.md 中的问题1/1b 日志：这些记录的 `local_count_tokens=true` + `completion_tokens=0` + `frt=-1000`（意味着 HasSendResponse=false）都会被 `shouldZeroLocalUsage` 拦截
4. 对照问题2/3 日志：`local_count_tokens=false` → 守卫直接 bypass，不受影响
