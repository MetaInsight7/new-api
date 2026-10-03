# Available Models 收尾规格

## Problem Statement

`tokendazi` 分支已经完成 Available Models 页面主体、模型访问聚合、价格与倍率展示、可用性状态展示，以及从模型分组创建 API Key 的闭环。收尾实现仍缺少两项用户可见行为：用户无法从页面标题直接打开操作文档，且普通用户仍会看到面向管理/分析场景的 Overview 与 Dashboard 导航。项目还缺少统一描述 Available Models、Model Access 和分组语义的领域术语文档，容易导致后续迁移再次混淆用户分组、渠道分组和 Auto 分组。

## Solution

在 `tokendazi` 分支补齐 Available Models 的操作文档入口，并按用户角色收敛根侧栏导航：管理员保留 Overview 与 Dashboard，普通用户隐藏两者但继续看到 Workspace、Available Models、API Keys 等用户入口。新增根目录领域词汇表，固定本次迁移使用的术语和边界。每个用户可感知功能独立提交，最后进行受影响前端检查和定向浏览器验收。

## User Stories

1. As a user browsing Available Models, I want an operation documentation link beside the page title, so that I can learn how to use the displayed model access information without leaving the page to search manually.
2. As a user, I want the documentation link to open in a new tab, so that my current model browsing state is preserved.
3. As a user, I want the external documentation link to use the configured official documentation URL, so that deployments can point the link at their own TokenDazi documentation site.
4. As a user, I want the external link to use `noopener` and `noreferrer`, so that opening documentation does not grant the destination a reference to or control over the console window.
5. As a regular user, I want the sidebar to focus on Workspace and my available models, so that administrative overview screens do not distract from my normal workflow.
6. As a regular user, I want Workspace and Available Models to remain visible, so that hiding administrative dashboards does not remove the primary user workflow.
7. As an administrator, I want Overview and Dashboard to remain visible, so that I can continue to inspect system and model-level information.
8. As an administrator, I want existing role and sidebar-module filtering to continue working, so that this change does not bypass configured navigation visibility.
9. As a maintainer, I want the terms Available Models and Model Access defined consistently, so that page-level concepts and API-level projections are not conflated.
10. As a maintainer, I want user groups, channel groups, Auto groups, and model-specific ratios defined explicitly, so that future migrations preserve Base's actual grouping and billing semantics.
11. As a maintainer, I want the final Available Models path and API Key creation path checked in a browser, so that the documented links, role filtering, deep link, preselection, preview, and URL cleanup are verified as user behavior.

## Implementation Decisions

- Add a reusable operation-documentation link component following the existing Base button and icon conventions. It uses `VITE_OFFICIAL_DOCUMENTATION_URL` when supplied and the TokenDazi Overlay documentation destination as the default. It opens a new browsing context with both `noopener` and `noreferrer`.
- Render the documentation link in the Available Models title action area. Do not copy unrelated Overlay brand navigation, marketing pages, or vendor-specific behavior.
- Make root navigation role-aware at the same seam that already resolves the authenticated role and applies sidebar-module configuration. Hide only Overview and Dashboard for non-administrators; keep the existing administrator filtering and all other entries unchanged.
- Preserve Base's single user-group domain model. The glossary may describe channel-group terminology for comparison, but implementation does not introduce a second independent pricing/充值分组 model, PLG groups, or fixed-account behavior.
- Create `GLOSSARY.md` at the repository root with definitions for Available Models, Model Access, 用户分组, 渠道分组, Auto 分组, and 模型专属倍率. Definitions describe domain meaning and boundaries, not implementation steps.
- Keep the existing `/api/user/model-access`, model availability probing, billing, API Key payload, and Dialog implementation unchanged except where the acceptance checks expose a regression. `/v1/available_models` remains out of scope.
- Land the documentation link and navigation behavior as separate commits. Land glossary and verification notes separately from user-facing code when possible, so each feature can be reverted and maintained independently.

## Testing Decisions

- Test the documentation link through its accessible name and link attributes, including the configured/default destination, new-tab target, and security rel values. Do not assert the full Tailwind class string.
- Test root navigation from the user's role perspective: a regular user cannot see Overview or Dashboard, while an administrator sees both and still receives existing sidebar-module filtering.
- Keep existing Available Models and API Key unit tests as the regression suite for the already-implemented deep-link and preview behavior.
- Run targeted Vitest files, TypeScript typechecking, affected-file lint/format checks, and `git diff --check` after implementation.
- Perform a targeted Playwright/browser smoke check against the already-running application when available. Cover Available Models loading, documentation link behavior, role-specific navigation, group-to-API-Key deep link, preselected group, model preview, narrow-screen preview entry, and URL cleanup. Do not rebuild the frontend unless the changed code requires it.
- A full Vitest run is informative but not a release blocker for this focused change if it exceeds the environment time budget; no unrelated existing failures will be fixed.

## Out of Scope

- PLG groups, fixed-account mode, separate pricing-group and recharge-group domains, supplier business, or pluginization.
- Changes to model availability probing, blocking, billing, model metadata, or the `/v1/available_models` API.
- Migrating the entire Overlay sidebar or branding/marketing surface.
- A visual redesign of Available Models beyond the already completed Overlay alignment.
- A model-specific-ratio administration UI; existing configuration behavior remains unchanged.

## Further Notes

- The trusted visual and behavioral reference remains the final `dev-new-api` `tokendazi-overlay` tree, while Base's role, auth, API, and layout conventions remain authoritative for adaptation.
- The current working tree contains user-owned `CLAUDE.md` and `docs/agents/` changes. They must not be overwritten, staged, or committed as part of this work.
