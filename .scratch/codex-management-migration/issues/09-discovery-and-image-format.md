# 09: 模型发现边界与图片真实格式

**What to build:** Codex 模型列表按有界读取和健壮版本缓存发现；图片临时 URL 的 MIME 与实际生成输出一致。

**Blocked by:** None (can start immediately).

**Status:** ready-for-agent

- [ ] 模型发现响应限制 4 MiB，非成功状态先处理，保留凭证刷新后一次重试。
- [ ] 版本缓存不持锁联网、缓存冷失败并合并并发查询；保留既有稳定版本任务和权限边界。
- [ ] 每个图片输出采用实际 output_format；保留 R2、当前输入限制和真实 usage。
- [ ] 边界与图片格式回归通过，独立提交。

## Verification

待实现。
