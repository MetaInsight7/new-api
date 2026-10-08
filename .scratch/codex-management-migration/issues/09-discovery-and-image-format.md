# 09: 模型发现边界与图片真实格式

**What to build:** Codex 模型列表按有界读取和健壮版本缓存发现；图片临时 URL 的 MIME 与实际生成输出一致。

**Blocked by:** None (can start immediately).

**Status:** completed (2026-10-08)

- [x] 模型发现响应限制 4 MiB，非成功状态先处理，保留凭证刷新后一次重试。
- [x] 版本缓存不持锁联网、缓存冷失败并合并并发查询；保留既有稳定版本任务和权限边界。
- [x] 每个图片输出采用实际 output_format；保留 R2、当前输入限制和真实 usage。
- [x] 边界与图片格式回归通过，独立提交。

## Verification

- 独立提交：`149da32d8`，`fix: bound Codex discovery and preserve image output formats`。
- 模型发现状态／体积边界、版本缓存并发和图片实际格式／R2 回归通过，root 相关完整回归与后端构建通过，见 `/tmp/codex-management-go-final-regression.log`。
- 延续现有 R2 私有媒体和实际 usage，不使用来源 GCS 凭据，不以真实存储上传代替协议测试。详细结果见 `docs/codex-management-migration-verification.md`。
