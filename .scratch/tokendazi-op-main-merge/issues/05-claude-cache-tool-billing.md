# 05: 保留 Claude Cache、Tool 与 Search 精确计费

**Spec:** `../../2026-10-23-tokendazi-op-main-merge-spec.md`


**Type:** task
**What to build:** 让 Claude 流式请求在 cache-only、未拆分 cache write、tool-only、web search 和异常中断场景下生成准确 usage，既采用上游新增分类计费，又不覆盖二开已获取的 cache 字段或制造未交付输出费用。

**Blocked by:** 04: 结算 Responses 终止 Usage 并保持失败指标保真

**Status:** resolved

- [x] 缺失 input usage 的判断包含 cache read/write，完整缓存的 prompt 不被误判为无输入。
- [x] fallback 只补缺失字段，保留 message start/delta 已返回的 cache 和显式零 usage。
- [x] tool-use fallback 计入必要开销，server tool、web search 和 web fetch 使用权威上报覆盖推测数量。
- [x] 中断流继续标记本地 token 估算来源；没有真实内容交付时不产生可收费 completion usage。
- [x] cache-only、显式零、tool-only、search、正常完成和中断场景的 billing/usage 测试通过。
- [x] 如上游计数与二开无输出退款、cache 保真规则冲突，停止并标记 `needs-info`，不得通过整份覆盖 usage 解决。

## Answer

- Claude cache read/write 纳入 input 判定，fallback 仅补缺字段并保留显式零；中断 tool-use 开销、server tool 与 search 权威计数均保留。
- 无真实内容不制造 completion 费用；cache-only、tool-only、search、完成与中断定向测试通过。
