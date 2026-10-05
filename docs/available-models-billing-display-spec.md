# Available Models 首阶梯价格展示规格

## Problem Statement

`available-models` 当前只适配了传统的倍率/固定价格字段。对于使用动态计费表达式或分档计费的模型，页面仍可能展示旧的输入/输出倍率，导致用户看到的价格与实际结算规则不一致。页面也需要在分时段计费时明确提示当前展示的是哪个时段的价格。

用户不需要在模型列表中阅读完整计费表达式、所有阶梯或缓存价格；他们只需要在现有模型卡片中快速看到一个可理解的输入价格和输出价格。

## Solution

保持 `available-models` 当前模型卡片、价格区域和响应式布局不变。价格展示按照以下优先级处理：

1. 普通模型继续使用现有输入/输出价格格式。
2. 动态计费模型复用现有动态计费解析与格式化能力，选择要展示的价格阶梯。
3. 普通分档计费选择配置顺序中的第一阶梯。
4. 分时段计费先选择当前时间生效的时段，再选择该时段的第一阶梯。
5. 最终只展示第一阶梯的输入和输出价格，不展示缓存读取、缓存写入、其他阶梯或原始表达式。
6. 分时段计费在现有价格标题区域增加“当前时段价格”提示，并沿用已有的一分钟时间刷新机制。

## User Stories

1. As a model user, I want the Available Models card to keep its current layout, so that the pricing information remains familiar and compact.
2. As a model user, I want a dynamically priced model to show its first configured input price, so that I can understand the model's starting price without reading an expression.
3. As a model user, I want a dynamically priced model to show its first configured output price, so that input and output costs remain comparable with legacy models.
4. As a model user, I want only one price tier shown, so that the model list does not become a pricing matrix.
5. As a model user, I do not want cache read or cache write prices mixed into the initial compact display, so that the existing two-column layout remains stable.
6. As a model user, I want a time-priced model to show the price for the currently active period, so that the displayed price matches the period that applies now.
7. As a model user, I want a visible current-period hint for time-priced models, so that I understand why the displayed price may change later.
8. As a model user, I want the displayed time-priced value to refresh at minute boundaries while the page is visible, so that an open page does not keep showing a stale period indefinitely.
9. As a maintainer, I want existing dynamic pricing parsers and formatters reused, so that Available Models and the pricing management views share the same billing semantics.
10. As a maintainer, I want legacy request-based and ratio-based models to keep their existing display behavior, so that this change does not alter unrelated pricing.

## Implementation Decisions

- Keep the existing Available Models card structure and responsive price panel. No pricing table, expansion control, expression text, or additional cache columns are introduced.
- Reuse the shared dynamic pricing summary and the existing billing-time hook. Do not add a second expression parser or duplicate tier-selection rules.
- For non-time dynamic token pricing, use the first parsed token tier in configuration order.
- For time-dependent token pricing, resolve the active period using the timezone encoded by the expression, then use the first token tier in that period. The display refreshes through the existing minute-based billing clock while the document is visible.
- Select only the dynamic entries representing input and output token prices. Cache read, cache write, image cache, media prices, and secondary tiers remain out of scope for this compact card.
- Keep the official-price context used by the current page: no user-group surcharge or recharge-price transformation is added to this page.
- Preserve the existing legacy formatter as the fallback for models without a usable dynamic summary. A dynamic expression that cannot expose input/output prices must not invent a value from unrelated fields.
- Reuse the existing translated “Current period price” label for the time-pricing hint; no new translation key is required.
- Remove the development-only billing prototype and its route switch after the real card behavior is implemented, so prototype query parameters cannot become an accidental product surface.

## Testing Decisions

- Add behavior coverage at the Available Models price-panel seam rather than testing private component state.
- Verify a dynamic token model displays the first tier's input/output values and does not display cache entries or later tiers.
- Verify a time-dependent expression selects different input/output values for different controlled times, shows the current-period hint, and updates through the existing billing-time mechanism.
- Verify legacy token and request models retain their existing price labels and values.
- Run the affected Vitest tests, TypeScript typecheck, targeted Oxlint/Oxfmt checks, and Playwright against the actual Available Models route. Browser verification must include a time-priced fixture or controlled response and confirm there are no console errors.
- Do not use screenshot pixel comparison as the primary assertion; assert accessible labels and visible price text.

## Out of Scope

- Showing cache read/write prices in Available Models.
- Showing all pricing tiers, tier conditions, raw billing expressions, or a pricing breakdown dialog.
- Changing backend pricing, settlement, model metadata, group ratios, or pricing APIs.
- Changing pricing management pages or other model-card views.
- Adding new billing expression syntax or changing how requests are actually charged.

## Further Notes

The first tier means the first tier returned by the shared parser for the selected pricing context. For time pricing, selecting the active time branch happens before selecting its first tier; the first branch in the raw expression is not always the current price.
