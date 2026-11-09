# Available Models 实付价与官方价对比展示规格

## Problem Statement

`available-models` 原有价格区只展示按 `1×` 计算的基础价格。用户选择访问分组后，无法直接看出模型专属倍率或分组倍率对应的实际支付价格，也无法确认模型专属倍率是否会与分组倍率相乘。

动态计费、分时段计费与缓存计费模型还需要继续复用统一的计费解析规则，避免页面展示与真实结算语义不一致。同时，创建 API 密钥弹窗空间有限，不能直接复用完整的价格对比布局。

## Solution

在独立的 `/available-models` 页面中，将每个模型的价格区展示为并列的两部分：

1. “当前实付价”按照当前访问分组的有效倍率计算。
2. “官方基准价”固定按照 `1×` 计算，供用户横向比较。
3. 模型名称下方显示当前生效倍率及其来源：模型专属倍率显示“专属倍率 `n×`”，否则显示“分组倍率 `n×`”。
4. 模型专属倍率覆盖分组倍率，两者不相乘。
5. `auto` 分组无法提前确定最终路由分组时，不推算实付价，改为提示“按实际路由命中的分组计费”；官方基准价仍正常展示。
6. Token 模型同时展示输入、输出和已配置的缓存读取/写入价格；请求计费模型展示单次请求价格。
7. 动态计费继续展示当前生效分支的首个阶梯，并在分时段计费时提示“当前时段价格”。

## User Stories

1. As a model user, I want to see my current effective price beside the official base price, so that I can understand the actual price without calculating ratios manually.
2. As a model user, I want to see whether a price comes from an exclusive model ratio or the group ratio, so that the billing source is unambiguous.
3. As a model user, I want exclusive ratios to visibly replace rather than multiply group ratios, so that the page matches settlement behavior.
4. As a model user, I want cache read and cache write prices to remain visible, so that cache-capable models can be compared completely.
5. As an Auto-group user, I want the page to avoid showing a guessed effective price, so that a future routing decision is not presented as a guaranteed amount.
6. As a mobile user, I want the comparison panels to stack without horizontal overflow, so that all prices remain readable.
7. As an API-key creator, I want the compact model preview to show the selected key's effective price, so that I can confirm its billing without opening the full model page.
8. As a maintainer, I want legacy, dynamic, time-dependent, request, token, and cache pricing to use shared pricing helpers, so that display semantics stay aligned.

## Pricing Semantics

- Official base price always uses ratio `1×`.
- Effective ratio resolution follows the backend billing priority:
  1. Use the exact model ratio when the selected scope defines one.
  2. Otherwise use the selected scope's default group ratio.
  3. If no deterministic ratio exists, do not calculate an effective price.
- Exact model ratios and group ratios are alternatives, not multiplicative factors.
- Fixed-account views use `account_model_ratios` and `account_default_ratio` rather than identity-level fallback values.
- Legacy token and request prices use the existing `formatGroupPrice` and `formatFixedPrice` helpers.
- Dynamic expressions use the existing dynamic-pricing parser with the resolved display ratio.
- For a normal tiered expression, use the first parsed tier in configuration order.
- If a normal expression mixes token and request tiers, display only that first tier's billing unit instead of merging both units into one card.
- For a time-dependent expression, resolve the currently active time branch first, then use that branch's first tier.
- Provider override collections retain the shared pricing summary's provider range; provider variants are not treated as later tiers of one expression.
- The existing billing-time hook watches both the model expression and provider override expressions, refreshing time-dependent display values at minute boundaries while the page is visible.
- Only configured cache fields are shown. Supported fields include cache read, generic cache write, and dynamic-expression cache write durations exposed by the shared parser.

## Presentation Decisions

- The desktop access-group rail is fixed at `15rem` (`240px`) so group names and statistics remain easy to scan.
- Group cards reuse `GroupMultiplierBadge` and consistently display `n×` or “自动”.
- The full comparison is enabled only through `pricingPresentation="comparison"` on `/available-models`.
- The API-key model preview keeps the default compact presentation, but shows only the current effective price for that key configuration; it does not show the official base price.
- At compact desktop widths, the API-key preview price column is fixed at `220px`; below the card breakpoint it stacks at full width.
- Compact token pricing shows input, output, and configured cache prices, while compact request pricing shows the per-request price.
- The compact preview shows the applied exclusive ratio, group ratio, or Auto state below the model identity. For Auto, it does not guess a price and instead states that billing follows the group selected during routing.
- Effective and official prices use two compact sections inside the price area: the highlighted effective section appears first and the muted official base-price reference section appears second.
- Both section headers show the same billing unit: per `1M tokens` for token pricing or per request for request pricing; the official section does not display a separate `1×` label.
- Each section contains its own input, output, and configured cache values; narrow cards stack the two sections without changing their order.
- The applied exclusive or group ratio badge stays below the model identity rather than inside the pricing area; protocol compatibility badges are omitted from model cards.
- Model identity and pricing stay on the same row on wide desktop layouts; narrower desktop layouts may stack the pricing table below the model identity.
- Long prices may truncate within their own cells but must not create page-level horizontal overflow.
- Cache labels use secondary text while cache amounts retain readable, emphasized monospace text.
- The page remains read-only; creating a key is linked through the selected scope and does not modify pricing.

## Testing Decisions

- Verify legacy token, request, cache, dynamic-tier, and time-dependent price calculations.
- Verify mixed token/request expressions keep only the first configured tier and provider time expressions retain current-period state.
- Verify an exclusive model ratio is used instead of the group ratio for effective prices.
- Verify the full page shows separate effective and official sections, and keeps the applied ratio badge below the model identity.
- Verify the API-key preview keeps the compact presentation, renders effective token, request, and configured cache prices, omits the official price, and uses the Auto fallback when routing is not deterministic.
- Verify fixed-account ratio fields are resolved correctly.
- Run affected Vitest tests, TypeScript typecheck, targeted Oxlint/Oxfmt checks, and a production build.
- Use browser checks at desktop, compact desktop, and mobile widths; confirm the `240px` rail, group switching, Auto-group fallback, absence of horizontal overflow, and absence of console errors.
- Keep component-level layout contracts for the `220px` compact price column, two primary price columns, wrapping cache row, and narrow comparison stacking.

## Out of Scope

- Changing backend settlement, pricing APIs, model metadata, group ratios, or billing expressions.
- Multiplying model-specific and group ratios.
- Predicting which concrete group an Auto route will select.
- Replacing the compact API-key creation preview with the full comparison layout.
- Showing every tier, raw billing expressions, or a detailed pricing breakdown dialog.
