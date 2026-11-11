# 01: 建立结构化敏感文本提取接缝

**What to build:** 为现有通用 Relay 模型请求建立有序、结构化的敏感文本提取能力；该扩展本身不启用新风控行为，也不改变 Token 统计、计费或请求序列化。

**Blocked by:** None (can start immediately)

**Status:** completed

- [x] 支持现有通用 Relay 协议中的 system、developer、user、assistant 和 tool 模型可见文本。
- [x] Tool 描述、文本参数、文本结果和模型可见的结构化输出 Schema 文本可提取，结构字段、角色标签、ID 和普通 metadata 不作为文本。
- [x] 图片、音频、文件、URL 型媒体、Base64 和二进制数据不进入提取结果。
- [x] 提取结果保留稳定顺序、角色、字段类型和审计轮次信息。
- [x] 现有 Token 统计和请求 wire shape 回归测试保持通过。
