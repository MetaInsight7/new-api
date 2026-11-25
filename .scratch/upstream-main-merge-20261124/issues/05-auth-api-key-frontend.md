# 05: 注册与 API Key 前端安全体验

**What to build:** 用户注册和创建或编辑 API Key 时继续使用 Tokendazi 的品牌化、安全默认和小时级有效期体验，同时获得官方密码反馈、无障碍属性和防表单串值修复。

**Blocked by:** 01: 认证缓存与 API Key 安全契约.

**Status:** resolved

- [x] 强制法律确认继续由现有 LegalConsent 承担，不恢复无调用方的展示型条款组件。
- [x] 注册表单保留广告归因、OAuth、Turnstile 和法律勾选，并吸收密码强度、确认反馈、aria-describedby 与 new-password autocomplete。
- [x] API Key 抽屉保留小时级有效期、权限字段和默认关闭额度查询，并吸收官方新增字段。
- [x] 切换编辑记录时先禁用保存并等待详情，再按当前记录 reset，任何上一把 Key 的字段都不会串入。
- [x] 非 Q6 静态键按双方新增集合合并，只删除确认无调用方的旧密码提示键。
- [x] 新密码统一使用服务端嵌入的 10001 项常见口令词表及明显可预测模式筛查；历史密码登录不追溯阻断。
- [x] 通过涉及页面的定向测试、格式和 lint 检查；全局 typecheck 与七语言生成结果纳入 Ticket 11 最终门禁。

## Answer

- 注册继续由既有 `LegalConsent` 强制确认服务条款与隐私政策，并保留 Tokendazi 品牌、广告归因、OAuth、Turnstile；同时吸收官方密码强度、确认反馈、无障碍描述与 `autocomplete="new-password"`。
- API Key 抽屉保留小时级快捷有效期、权限字段、模型预览和默认关闭额度查询；编辑切换时在详情加载期间禁用保存，加载完成后按当前 Key 重置表单。
- `DateTimePicker` 的清空值统一适配为 schema 接受的 `null`，避免清空有效期后产生 `undefined` 校验回归。
- 静态翻译键按双方并集合并，保留 Tokendazi 渠道并发文案与官方缓存计费文案。
- 通过 9 个相关测试文件共 90 项测试；相关文件 `oxfmt --check` 与 `oxlint` 通过。全局 typecheck、七语言同步和生产构建在全部前端冲突清零后由 Ticket 11 执行。

## OWASP review

- 基线：OWASP 官方项目页在 2026-11-24 标示 ASVS 5.0.0 为最新稳定版；同时复核了 [Authentication Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Authentication_Cheat_Sheet.html) 与 [Session Management Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html)。本 Ticket 没有改变会话创建、轮换、退出或失效语义。
- `v5.0.0-6.2.1`、`v5.0.0-6.2.5`、`v5.0.0-6.2.9`：注册密码保持 8–128 个 Unicode 字符，不增加字符种类限制，并允许至少 64 个字符。
- `v5.0.0-6.2.4`：所有新密码写入在 `HashAccountPassword` 前统一执行服务端筛查；词表固定为 SecLists `10k-most-common.txt` 提交 `fcde9140df969b1e4d7bb056122646dace4ed4cc`（10001 项，SHA-256 `68782d6a4a19a4768d5f15dd66bd534e7a33055cc755411e33f16d18c50fdcce`），对完整口令做大小写不敏感的精确匹配，并仅对前后端共享的固定高危根口令拒绝纯数字/符号尾缀，同时拒绝重复字符和完整顺序键盘串；短词加高熵随机尾缀不得仅因短词出现在词表中而被误杀。已有 bcrypt/Argon2id 密码只在选择新密码时受约束，历史登录继续兼容。
- `v5.0.0-6.2.6`、`v5.0.0-6.2.7`：密码和确认字段继续使用遮罩输入，允许粘贴与密码管理器，并使用 `autocomplete="new-password"`；强度和确认提示通过 `aria-describedby` 关联，不替代服务端校验。
- `v5.0.0-8.2.3`、`v5.0.0-8.3.1`：API Key 的额度查询权限由服务端字段显式执行；前端新建默认关闭，编辑时按当前记录重新加载，避免上一把 Key 的权限值串入下一把。
- 验证覆盖：注册密码长度/Unicode/确认反馈与可访问性测试；API Key 默认映射、显式开启、记录切换和详情加载期间禁止提交测试；后端注册默认 Key 和 Token 创建/更新回归。
- 验证补充：服务端测试强制词表不少于 3000 项，覆盖 `password123`、大小写与高危根口令装饰尾缀、重复/顺序模式、高熵随机尾缀通过、强密码通过及旧常见密码登录；账户改密路由验证固定 `PASSWORD_POLICY_REJECTED` 响应且不改写存量哈希。前端即时拒绝明显可预测口令并提供七语言错误文案，服务端完整词表始终为权威。
