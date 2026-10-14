# 重试无候选时保留最近一次真实失败

Status: ready-for-agent

Implementation: completed (2026-10-15); see verification.md.

## Problem Statement

一次请求已经在某条渠道真实尝试并失败，重试时当前候选集合可能因排除、冷却等既有规则而为空。OP 此时生成新的“无可用渠道”错误，覆盖了更有诊断价值的原失败，调用方无法区分上游限流、上游不可用与本地渠道容量不足。

任务请求还会把选路错误重新包装为本地 `get_channel_failed`，导致只迁移二开的公共选路改法仍可能丢失原任务错误的业务码、数据和来源。

## Solution

只有在没有新选路故障、仅因候选为空而结束时，返回本次请求最近一次真实尝试的失败，并保留其来源及协议语义。该失败既可以是上游 429/503，也可以是允许重试的本地渠道容量失败。

新的数据库、并发系统或其他实际选路故障优先；没有历史真实尝试时继续返回现有无渠道错误。请求级不可重试失败仍直接结束。普通 Relay 与任务提交遵循相同原则，但任务错误保留其原生完整结构。

## User Stories

1. As an API caller, I want the last upstream rate-limit status to survive candidate exhaustion, so that I can apply the appropriate client backoff.
2. As an API caller, I want the last upstream service-unavailable status to survive candidate exhaustion, so that I can distinguish provider failure from missing routing configuration.
3. As an API caller, I want the most recent attempted failure rather than an earlier failure, so that the response describes the latest known outcome.
4. As an API caller, I want retryable local channel-capacity failures to remain local, so that provider health is not misrepresented.
5. As an API caller, I want a new routing-system fault to take precedence over an older attempted failure, so that the current blocking condition is visible.
6. As an API caller, I want the existing no-channel response when no attempt has happened, so that missing configuration is still reported correctly.
7. As an API caller, I want successful fallback attempts to return their normal response, so that error preservation does not turn recovery into a failure.
8. As an API caller, I want existing retry budgets and candidate ordering to remain unchanged, so that the fix does not cause additional upstream traffic.
9. As an API caller, I want insufficient-balance and invalid-request failures to stop immediately, so that retries do not conceal terminal request errors.
10. As a task API caller, I want the original task status and business code preserved, so that I can recognize the provider's rejection.
11. As a task API caller, I want the original task message and data preserved, so that details such as retry hints remain available.
12. As a task API caller, I want local provenance and non-retryable semantics preserved, so that channel policy and duplicate-submission protection remain correct.
13. As a task API caller, I want an accepted task never submitted again, so that the fix cannot create duplicate billable work.
14. As a streaming API caller, I want already-written responses to remain protected from retry, so that the gateway never mixes responses from multiple attempts.
15. As an administrator, I want existing sanitization and 429 presentation rules preserved, so that useful errors do not expose channel secrets.
16. As an administrator, I want candidate exhaustion not to count as another failed upstream attempt, so that health decisions and attempt records stay truthful.
17. As an administrator, I want request-local failure state cleared for a new submission and after success, so that previous failures cannot leak into another operation.
18. As a maintainer, I want regressions verified at existing controller seams, so that tests protect observable behavior without a new error-classification framework.

## Implementation Decisions

- Keep error preservation in the host relay controller and task-submission flow, not in provider plugins or a new routing policy.
- Distinguish candidate exhaustion without an operational selection error from a new selection, context-setup, or concurrency-system fault. The latter always retains precedence.
- Ordinary Relay preserves the structured API error, its status, business code, source and existing error options. Candidate exhaustion stops the current retry loop.
- Task submission retains its original `TaskError`, including `StatusCode`, `Code`, `Message`, `Data`, `LocalError`, `NoRetry` and underlying error semantics, instead of reconstructing it from an API-error projection.
- History means a failure produced by an actual attempt in this request. A request-scoped local capacity rejection can qualify without reaching the upstream; a generic no-channel result cannot.
- Reset history when starting a task submission and clear it after a successful attempt. New cancellation, setup, persistence or other terminal failures keep their existing precedence.
- Reuse existing retry decisions, error sanitization, protocol presenters and request-policy logging. Reusing history must not repeat channel punishment or invent another submission.
- Respect the accepted core-routing, authoritative-concurrency and Responses recovery ADRs. No routing, capacity ownership, pricing, settlement, schema or configuration contract changes are required.

## Testing Decisions

- Use the existing controller layer: ordinary Relay through HTTP request/response fixtures and task submission through the existing injected submit-attempt boundary.
- Extend existing controller test files; do not scatter equivalent tests across service, relay or settings packages.
- Use deterministic request fixtures, a local upstream server, explicit routing state, and exact expected response values. Upstream submission counts protect against duplicate external operations, not internal implementation details.
- Cover upstream 429/503 followed by candidate exhaustion, multiple attempted failures where the latest wins, retryable local-capacity failure with preserved local provenance, and the no-history baseline.
- Cover new database/routing and concurrency-system failures taking precedence over history, task status/code/message/data preservation, normal fallback success and request-local history reset.
- Retain existing coverage for request-level non-retryable failures, written-response guards, accepted-task guards and billing lifecycle invariants.
- Prior art includes the HTTP channel-concurrency routing fixtures and task-submission controller regressions. Use `require` for setup and fatal assertions, and `assert` for value checks.

## Out of Scope

- Static/frontend changes, first-stream-event watchdogs, alerts, metrics and other migration candidates.
- Retry-budget changes, candidate ordering, group routing, concurrency ownership, cooldown rules or a general-purpose error taxonomy.
- Pricing, pre-consume, settlement, refund, database schema, new administrative settings or JavaScript plugin API changes.
- Returning unsanitized upstream messages or changing existing protocol-specific error envelopes and 429 display rules.

## Further Notes

- This is an OP-adapted improvement, not a verbatim copy of the second-development implementation. It borrows last-error preservation while covering OP task fidelity and local provenance.
- The comparison source is the available second-development mirror at `762ce21c429a3aa9adc104bcd80eb7df3ad3bdbd`; it is a fixed baseline, not a claim about all other forks or future upstream revisions.
- Earlier research suggested preserving only upstream failures. The current conversation supersedes that restriction: a retryable local capacity failure is also retained, with its local source unchanged.
- The preceding pre-consume retry/logging repair is already committed separately and is not reimplemented here.
