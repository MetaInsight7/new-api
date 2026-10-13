# Tokendazi Responses 加密状态线上兼容性实验

> 实验时间：2026-10-11（Asia/Shanghai）
> 网关：`https://api.tokendazi.com/v1`
> 模型：`gpt-5.6-sol`
> 请求：`store:false`、`stream:true`、`include:["reasoning.encrypted_content"]`
> 渠道固定：管理员 API Token 使用 `sk-<token>-<channel_id>`，禁止失败后换渠。
> 本文不记录 API Token、OAuth Token、完整 encrypted content、response/item ID。

## 渠道

| 渠道 | 上游身份 |
|---|---|
| 4 | Codex subscription A |
| 5 | Codex subscription B |
| 17 | Azure OpenAI API |

## 实验方法

1. 分别在渠道 4、5、17 上发起高推理请求。
2. 确认输出包含非空 reasoning `encrypted_content` 和可读 summary。
3. 保持 reasoning item、item ID、密文和 assistant message 原样不变，追加下一轮 user message。
4. 将同一份历史分别固定发送到三个渠道。
5. 同渠道回放作为协议正确性的控制组。
6. 额外将渠道 4 的 reasoning item 删除 `encrypted_content` 和 `id`、保留 summary，再发送到渠道 17。

## 原样回放矩阵

| 密文来源 → 目标渠道 | 结果 | 信号 |
|---|---|---|
| 4 → 4 | 通过 | `response.completed` |
| 4 → 5 | 通过，重复验证通过 | `response.completed` |
| 4 → 17 | 失败，重复验证失败 | HTTP 400 `invalid_encrypted_content` |
| 5 → 5 | 通过；另有一次独立的瞬时过载 | `response.completed`；瞬时 `server_is_overloaded` 与密文无关 |
| 5 → 4 | 通过，重复验证通过 | `response.completed` |
| 5 → 17 | 失败 | HTTP 400 `invalid_encrypted_content` |
| 17 → 17 | 通过 | `response.completed` |
| 17 → 4 | 失败 | 流内 `invalid_encrypted_content` |
| 17 → 5 | 失败 | 流内 `invalid_encrypted_content` |

## 有损恢复控制

| 输入 | 目标 | 结果 |
|---|---|---|
| 渠道 4 原始 reasoning 密文和 ID | 渠道 17 | HTTP 400 `invalid_encrypted_content` |
| 删除 reasoning `encrypted_content` 与 `id`，保留 summary | 渠道 17 | `response.completed` |

## 结论

1. 本次实测的两个 Codex subscription 渠道 4 和 5 属于兼容的 encrypted-content 安全域，可以原样跨渠道继续同一个 Responses reasoning 链。
2. Azure 渠道 17 与这两个 Codex subscription 不属于同一安全域；密文双向都不能原样迁移。
3. “Codex subscription 不与 API/Azure 混池”的 CLIProxyAPI 官方建议与本次实测一致。
4. 删除 reasoning 密文和 provider-owned item ID、保留 summary 后可以迁移到 Azure，但这是有损恢复，不是隐藏推理状态的无损续接。
5. 结果只证明当前三个渠道、当前模型和当前上游实现；不能自动推广到所有 Codex subscription 账号、模型或未来凭证代际。兼容关系应按 security domain 明确配置或定期探测。
