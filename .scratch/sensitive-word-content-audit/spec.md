# 敏感词风控与结构化审计重构

Status: completed

Type: task

## Problem Statement

当前代码只有旧式请求关键词检查，缺少一套可运营、可审计、适合生产多实例部署的敏感词风控能力。官方 PR #7565 提供了全局／分组规则、观察与阻断模式、审计、违规计数、阈值停用和后台管理能力，但它基于更早的上游版本，不能机械合入当前二开分支，并且仍存在以下生产风险：达到阈值必然自动封禁、普通启用账号会隐式清零违规次数、每个请求读取数据库策略版本、审计证据是扁平 Prompt、权限粒度过宽、匹配结果可能重复放大，以及 MySQL 大字段容量不足。

运营人员需要在不改变计费、余额、订阅、API Key 持久状态和既有 Relay 行为的前提下，获得一套规则明确、故障边界清晰、证据受控、支持多实例且能安全迁移旧配置的敏感词风控系统。

## Solution

以 PR #7565 的规则、后台和审计能力为基础，在当前代码上做适配式迁移而非整段 cherry-pick。系统提供全局／分组敏感词规则、`block / observe / off` 模式、显式白名单、持久敏感词违规次数、默认关闭的独立自动封禁开关、专用敏感词解封、结构化审计证据、细粒度风控与审计权限，以及适合多实例的本机策略快照和跨节点刷新机制。

所有受支持请求在鉴权和协议解析成功后、渠道选择与额度预扣前完成检查。既有通用 Relay 协议只匹配每个独立会话最新输入轮次中的 user 文本和文本型 Tool 结果；历史轮次及其他角色只作为命中后的审计上下文候选。命中证据按字段扫描并形成有界唯一结果，审计证据保存 system/developer、当前命中项和此前三轮审计轮次，序列化后总量不超过 512 KiB。

Root、管理员、普通用户和白名单的处置与 PR #7565 对齐：Root 被阻断、审计并累计，但永不自动封禁；非 Root 管理员与普通用户一致；只有显式白名单用户放行、审计且不累计。自动封禁由独立开关控制，封禁只改变账号状态和认证状态，不清余额、订阅或 Key。

## User Stories

1. As a Root administrator, I want to enable or disable sensitive-word enforcement globally, so that I can control rollout without deleting rules.
2. As a risk-control administrator, I want to create global sensitive-word rules, so that one policy can protect every user group.
3. As a risk-control administrator, I want to bind rules to pricing groups, so that group-specific risk requirements can coexist with global rules.
4. As a risk-control administrator, I want each rule to use `block`, `observe`, or `off`, so that I can stage and suspend rules safely.
5. As a risk-control administrator, I want new rules to default to `observe`, so that an unreviewed rule does not immediately interrupt production traffic.
6. As a risk-control administrator, I want any matching `block` rule to take precedence over matching `observe` rules, so that a blocking rule cannot be weakened by another match.
7. As an operator, I want `observe` matches to pass through while creating an audit fact, so that I can evaluate false positives before enforcement.
8. As an operator, I want `off` rules to have no request effect, so that disabled rules remain editable without participating in matching.
9. As an ordinary API user, I want a stable protocol-compatible error when a blocking rule matches, so that my client can distinguish policy rejection from an upstream failure.
10. As an ordinary API user, I want one blocked request to add exactly one sensitive-word violation, so that repeated words inside one request do not inflate my count.
11. As an ordinary API user, I want retries carrying the same request identity to remain idempotent, so that transport retries do not count as new violations.
12. As a Root user, I want blocking matches to be audited and counted but never automatically disable my account, so that Root remains recoverable without silently bypassing policy.
13. As a non-Root administrator, I want my model requests to follow ordinary-user enforcement unless I am explicitly whitelisted, so that an administrator role does not implicitly bypass policy.
14. As a whitelisted user, I want matching requests to pass while still creating an audit fact, so that approved exceptions remain observable.
15. As a whitelisted user, I want matching requests not to increase my sensitive-word violation count or trigger automatic banning, so that the exception has clear semantics.
16. As a risk-control administrator, I want adding or removing a whitelist flag to leave historical counts intact, so that a temporary exception does not erase history.
17. As a Root administrator, I want automatic banning controlled by an independent switch that defaults to off, so that rule blocking can launch before account automation.
18. As a Root administrator, I want one global configurable ban threshold, so that automatic enforcement has a clear operating value.
19. As a Root administrator, I want existing counts to remain effective when automatic banning is enabled, so that enabling automation does not silently forgive prior violations.
20. As an ordinary user, I want automatic banning to disable only my account and active authentication state, so that it does not destroy balance, subscriptions, historical billing, or API Key records.
21. As an operator, I want sessions and authentication caches revoked after an automatic ban, so that a disabled account cannot continue through stale credentials.
22. As an administrator, I want ordinary account enablement not to clear sensitive-word violations, so that unrelated account recovery does not erase risk history.
23. As a risk-control administrator, I want a dedicated sensitive-word unban action that only reverses a current sensitive-word automatic ban, enables the account, and clears the count, so that an approved appeal cannot override an unrelated administrator disable.
24. As a risk-control administrator, I want clearing the count, unbanning the account, and changing the whitelist to remain separate actions, so that I can perform only the intended operation.
25. As a recovered user, I want previously valid API Keys to work again when my account is unbanned, so that account recovery does not require recreating every Key.
26. As an administrator, I want suspected leaked Keys disabled or deleted through existing Key management, so that sensitive-word unban does not pretend to rotate credentials.
27. As an API user, I want malformed requests to return their normal protocol validation error without a sensitive-word count, so that invalid JSON is not treated as policy abuse.
28. As an operator, I want a valid matched request blocked before channel selection and quota reservation, so that it cannot reach an upstream or consume quota.
29. As an operator, I want a valid matched request audited even if it would later fail for no channel, unsupported model, or insufficient quota, so that downstream availability cannot bypass enforcement.
30. As a risk-control administrator, I want user text and textual Tool results in each conversation's latest input round checked, so that current user-supplied and tool-returned input is covered without rescanning historical conversation context.
31. As a risk-control administrator, I want a request containing only textual Tool results checked as current input, while system/developer/assistant text, Tool definitions, and Tool arguments remain outside matching, so that the boundary is explicit and predictable.
32. As an operator, I want video-generation prompts excluded from this policy, so that video task behavior remains unchanged.
33. As an operator, I want generic asynchronous-task prompts excluded from this policy, so that task-plugin behavior remains unchanged.
34. As an API user, I want images, audio, files, Base64 payloads, and ordinary metadata excluded from scanning, so that binary content and structural values do not create false positives or expensive parsing.
35. As an operator, I want model output and remotely referenced conversation history excluded, so that request filtering does not become output moderation or an external history fetcher.
36. As an operator, I want each structured text field scanned once and duplicate matches bounded, so that large requests cannot amplify CPU or memory through repeated whole-prompt passes.
37. As an operator, I want the existing global request-body limit to remain the only request-size rejection boundary, so that sensitive-word enforcement does not introduce a new 8 MiB incompatibility.
38. As an audit reader, I want audit evidence to preserve roles, field kinds, order, and audit rounds, so that I can reconstruct the relevant context instead of reading an ambiguous flattened Prompt.
39. As an audit reader, I want system/developer context, the current item, and the preceding three audit rounds retained, so that evidence is useful without storing an unbounded conversation.
40. As an audit reader, I want fields containing actual matches prioritized when evidence is truncated, so that the reason for an event is not removed before unrelated context.
41. As a privacy-conscious operator, I want serialized audit evidence capped at 512 KiB and marked when truncated, so that one request cannot create an unbounded evidence record.
42. As a privacy-conscious operator, I want audit evidence and audit facts to have distinct lifecycles, so that content can expire while enforcement history remains explainable.
43. As a privacy-conscious operator, I want evidence to expire after 180 days by default, so that full context, previews, and snippets are not retained indefinitely.
44. As an audit reader, I want an independent sensitive-word audit-read permission, so that ordinary log access does not reveal complete prompts or matched words.
45. As a risk-control administrator, I want an independent sensitive-word management permission, so that editing rules or user risk state does not automatically grant evidence access.
46. As a Root user, I want non-Root administrators prevented from reading my audit evidence even when they have general audit-read permission, so that Root content has an additional boundary.
47. As an ordinary user, I want general usage-log responses to omit matched words, rule details, Prompt hashes, and audit IDs, so that sensitive metadata cannot leak through normal logs.
48. As an administrator, I want the management UI to show policy, rules, user count, whitelist state, and permitted audit details, so that the feature is operable without direct database edits.
49. As an international administrator, I want all new UI and responses integrated with the existing localization system, so that the feature does not ship as a single-language island.
50. As a multi-instance operator, I want policy and rule changes propagated quickly to all nodes, so that enforcement does not depend on which node receives a request.
51. As a multi-instance operator, I want missed Redis notifications corrected by periodic version checks, so that transient Pub/Sub loss cannot leave a node stale forever.
52. As an operator, I want a node to keep using its last valid policy snapshot when a refresh fails, so that a transient database problem does not silently disable established rules.
53. As an operator, I want an observe-mode audit failure to allow the request and emit an operational warning, so that a non-blocking rollout does not become an outage.
54. As an operator, I want a block-mode audit failure to keep the request from reaching an upstream, so that a database failure cannot turn a known policy match into a bypass.
55. As an operator, I want evidence cleanup processed in bounded, retryable batches, so that retention work does not create a large table lock.
56. As a database operator, I want audit records and request idempotency to work on PostgreSQL, MySQL, and SQLite, so that deployment dialect does not change enforcement semantics.
57. As a database operator, I want MySQL evidence columns capable of storing the full configured cap, so that evidence does not fail at the `TEXT` size boundary.
58. As an upgrader, I want legacy sensitive-word options and rules migrated once and idempotently, so that an existing deployment keeps its intended policy after upgrade.
59. As a multi-instance upgrader, I want non-migrating nodes to avoid enforcing a partially migrated rule set, so that rolling deployment remains safe.
60. As a billing operator, I want sensitive-word enforcement to leave model pricing, quota math, subscriptions, payments, and retry selection unchanged, so that this security feature cannot alter revenue behavior.

## Implementation Decisions

- PR #7565 is a behavioral and UI baseline, not a merge unit. Its seven feature commits are adapted incrementally to the current branch, preserving current relay, billing, permissions, task-plugin, concurrency, and frontend architecture.
- The domain model uses the glossary terms sensitive-word violation count, audit round, match evidence, audit evidence, audit fact, sensitive-word management permission, sensitive-word audit-read permission, and sensitive-word count exemption.
- Rules retain `global` and pricing-group scope and `block / observe / off` modes. New rules default to `observe`; if a request matches both `block` and `observe`, `block` wins.
- Matching remains case-insensitive and uses an Aho-Corasick-style immutable runtime index. Rule mutations and policy mutations advance a durable generation in the same transaction.
- The request scanner consumes ordered structured text fields rather than one concatenated `CombineText`. Each supported protocol extractor identifies role, field kind, audit round, original order, and text while excluding structural labels, IDs, ordinary metadata, URLs that are not model input, and binary payloads.
- For each independent conversation, matching selects the highest round containing user input or a textual Tool result, then scans that round's user text and Tool results. A Tool-result-only request is still scanned. Historical rounds, system/developer/assistant text, Tool definitions and arguments, and response Schema text remain available only as audit context and do not participate in matching. No remote `previous_response_id` history is fetched.
- Video generation, generic asynchronous tasks, Midjourney, Suno, arbitrary plugin-defined custom fields, and Realtime WebSocket events are not scanned in this scope.
- The scanner performs one automaton pass per text field, deduplicates matches by rule and normalized word, bounds retained match metadata and snippets, and records truncation. It must not concatenate an unbounded Prompt, retain every repeated occurrence, or rescan the full input once per matched word.
- The feature introduces no sensitive-word-specific request-size rejection. Existing global body limits, rate limits, and user concurrency remain responsible for worst-case request size and throughput.
- Enforcement occurs only after token authentication and successful protocol parsing and before channel selection, token estimation, quota reservation, billing, plugin execution, or upstream I/O. Malformed requests stay on their existing validation path.
- Request identity and user identity form the idempotency key. The database enforces uniqueness for that pair in addition to transactional user locking, so concurrent duplicate delivery cannot create multiple audit facts or counts.
- A blocking match returns HTTP 400 with the stable error code `sensitive_words_detected`, uses the protocol-appropriate error envelope, and is marked as ineligible for upstream retry.
- An observe-only match passes, writes an audit fact, and does not increment the sensitive-word violation count. An `off` rule does not participate.
- A non-whitelisted ordinary user or non-Root administrator receives a block, an audit fact, and one persistent count increment for a blocking request. Automatic banning occurs only when the independent switch is enabled and the global threshold is reached.
- Root follows blocking and counting but is always excluded from automatic banning. Root is not automatically whitelisted.
- A user of any role gains count exemption only through the explicit sensitive-word whitelist. A whitelisted match passes and writes an audit fact without incrementing or banning; changing whitelist state does not reset prior counts.
- The automatic-ban switch defaults to disabled. Enabling it does not reset or ignore existing counts. The global threshold remains configurable and applies on the next committed blocking violation.
- Automatic banning changes user status, records sensitive-word automatic-ban ownership, advances authentication version, publishes authentication-cache invalidation, revokes active sessions, and invalidates token caches. It does not change quota, used quota, balance, subscriptions, billing records, or API Key persistent status.
- Ordinary account status management and sensitive-word unban are separate operations. An explicit administrator enable or disable clears sensitive-word automatic-ban ownership and preserves the count. Sensitive-word unban only accepts a currently disabled account with that ownership marker, then enables it, clears the marker, and clears the count. Count clearing, unban, and whitelist mutation remain independently callable operations.
- Dedicated sensitive-word management actions do not require an operator reason and do not create a new detailed operation-history subsystem. Existing baseline management logs remain in place.
- Audit evidence is structured. It retains system/developer context, the current item, and the three preceding audit rounds; one audit round starts with a user input and includes following assistant/tool activity until the next user input. Tool activity does not consume an additional round.
- Fields containing actual matches are retained before non-matching context. Serialized evidence has one 512 KiB total cap, excludes binary/Base64 payloads, and records whether content was truncated.
- Audit facts retain subject, request, model, group, protocol, matched rule identity, disposition, count, account-state transition, rule generation, and timestamps. Prompt hash and other sensitive metadata are protected by audit-read permission and never exposed through ordinary log projection.
- Audit evidence, redacted preview, and match snippets default to 180-day retention and are cleared at expiry; the audit fact remains. Retention cleanup runs in bounded batches, supports retry, and respects existing multi-node system-task ownership.
- MySQL uses large-text or JSON-capable column types for evidence and collection fields; PostgreSQL and SQLite use dialect-appropriate equivalents. Collection serialization records truncation instead of relying on silent database truncation.
- Sensitive-word policy and rule state are published as an immutable in-process snapshot. Mutation commits first, then broadcasts a Redis refresh notification. Nodes also perform periodic durable-generation checks so missed notifications converge.
- A refresh failure retains the last valid snapshot and raises an operational warning. If migration has completed but a node has never loaded a valid snapshot, request enforcement fails closed rather than assuming rules are disabled; before the one-way migration marker exists, the new runtime remains inactive.
- Observe-mode audit persistence failure is fail-open with an operational warning. Block-mode persistence failure never forwards the matched request upstream; only successfully committed transactions may claim a count increment or automatic ban.
- The management UI adapts the PR policy form, rule table/editor, user risk controls, and audit detail into the current frontend architecture and permission model. It must not restore removed legacy request-check controls or bypass current capability checks.
- General log APIs continue to strip administrator-only metadata. Complete evidence has a dedicated API guarded by sensitive-word audit-read permission, and non-Root callers are denied Root evidence.
- Legacy options and rule columns are one-way migration input only. Migration imports the valid subset under the 200-character-per-word and 10,000-words-per-rule limits, warns about skipped entries, and leaves the legacy source unchanged for manual recovery; skipped entries do not prevent the migration marker or disable valid imported protection. Runtime enforcement reads the new policy, rule, audit, and user fields after migration and does not maintain two writable sources of truth.

## Testing Decisions

- The primary and highest test seam is a route-level request-policy integration harness using real routing, authentication context, protocol parsing, policy persistence, and a fake channel/upstream boundary. Tests assert client response, audit fact, count/account state, and that channel selection, billing reservation, plugin execution, and upstream I/O were or were not reached. They do not assert internal helper call order.
- The same primary seam covers ordinary users, non-Root administrators, Root, whitelisted users, `block / observe / off`, mixed rule modes, malformed bodies, no-channel requests, insufficient quota ordering, duplicate request IDs, and protocol-compatible errors; separate exclusion tests confirm video generation and generic asynchronous tasks do not enter this seam.
- Structured extraction tests use representative protocol payloads and assert ordered model-visible fields, roles, audit rounds, textual Tool-result coverage, and exclusion of IDs, ordinary metadata, URLs, Base64, files, images, and audio. Matching tests separately verify latest-input-round selection, same-round user plus Tool-result scanning, and Tool-result-only requests without treating all extracted audit context as matchable input.
- Matching tests exercise case folding, global and group scope, auto-group candidate order, mixed modes, unique bounded matches, repeated occurrences, long fields, multiple fields, and first-match evidence priority. Performance tests verify work scales with scanned text plus retained matches rather than text multiplied by matched-word count.
- Database behavior tests cover transactional count increments, Root exclusion from automatic ban, whitelist count exemption, automatic-ban ownership, administrator status takeover, strict sensitive-word unban, unchanged quota/subscription/Key state, authentication invalidation, and actual unique idempotency under concurrent delivery.
- PostgreSQL and MySQL tests use real multi-connection databases for duplicate request races, row locking, unique constraints, large evidence values, JSON/text column compatibility, migration idempotency, and cleanup batching. SQLite remains the fast local regression dialect.
- Multi-instance tests use two independent runtime snapshots sharing database and Redis test infrastructure. They cover notification refresh, dropped notification plus periodic convergence, stale refresh failure retaining the last valid snapshot, and a node starting during migration.
- Failure tests inject policy-load failure, observe audit-write failure, block audit-write failure, usage-log failure, cache-publish failure, session-revocation failure, and cleanup-batch failure. Assertions focus on externally visible forwarding/blocking and committed state.
- Retention tests create expired and live audit evidence, run bounded cleanup repeatedly, and verify only evidence, preview, and snippets are cleared while audit facts and enforcement history remain.
- Permission tests exercise APIs through Root, permitted non-Root administrator, unpermitted administrator, and ordinary-user identities. They verify management and evidence-read permissions are independent and Root evidence is hidden from non-Root readers.
- Log projection tests verify ordinary log responses cannot reveal matched words, rules, Prompt hashes, audit IDs, or evidence, while authorized audit detail returns only the permitted event.
- Frontend component tests cover policy defaults, independent automatic-ban toggle, threshold editing, rule modes, rule search/editing, whitelist and count controls, dedicated unban, permission-gated controls, audit loading/error states, and localization.
- A focused browser test uses a temporary observe rule, then a blocking rule and a temporary user to verify the complete administrator workflow without calling a real model. Cleanup restores policy, rules, user state, and counts.
- Validation runs focused Go packages and frontend test files continuously, typechecking regularly, then the complete Go and frontend suites once at the end. Existing unrelated baseline failures are reported rather than repaired.

## Out of Scope

- Scanning or moderating model output.
- Fetching remote Responses conversation history through `previous_response_id` or another provider API.
- Parsing PDF/TXT attachments, OCR, audio transcription, image understanding, Base64, or other binary payloads.
- Video-generation prompt checking, generic asynchronous-task prompt checking, Midjourney prompt checking, Suno prompt or lyrics checking, arbitrary task-plugin custom-field checking, and Realtime WebSocket event checking.
- Recursively scanning every unknown JSON string or ordinary metadata value.
- A sensitive-word-specific 8 MiB request limit or another new request rejection size.
- Rolling-window decay, automatic count expiry, per-group thresholds, per-rule thresholds, or role-based implicit whitelist.
- Automatically disabling or deleting API Keys, clearing balances, changing subscriptions, refunding charges, or modifying payment behavior.
- Requiring operation reasons or building a separate detailed risk-operation history system.
- Changing channel selection, ordinary retry budgets, model availability, pricing, quota math, subscription consumption, user concurrency, or rate limiting.
- Introducing prompt redaction or request rewriting before the upstream; this feature either observes or blocks.

## Further Notes

- ADR 0007 is authoritative for persistent counting, separated unban semantics, structured evidence, identity behavior, and retention.
- PR #7565 contains seven feature commits and touches 75 files on an older upstream baseline. The current branch has substantial structural conflicts, so implementation must transplant behavior in reviewable slices rather than merge the PR wholesale.
- The original PR scans the full current-request `CombineText`. This design intentionally narrows matching to each conversation's latest input round while retaining broader structured fields only for bounded audit context.
- The original PR does not naturally expire violation counts. Its relevant difference is that generic account enablement also resets them; this spec removes that implicit coupling and records whether a disable was produced by sensitive-word automatic enforcement.
- The primary route-level seam described above was selected because it validates the most important user-observable boundary with one harness and minimizes test-only seams.
