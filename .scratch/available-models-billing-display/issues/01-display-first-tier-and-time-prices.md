# 01: Display first-tier prices for Available Models

**What to build:** Keep the existing Available Models card layout while showing the first configured input/output price for dynamic billing models. For time-dependent billing, show the first tier of the currently active period and add a current-period hint that refreshes at minute boundaries.

**Blocked by:** None (can start immediately)

**Status:** implemented

- [x] Dynamic token pricing displays only the first tier's input and output prices.
- [x] Cache prices, later tiers, and raw billing expressions are not added to the compact card.
- [x] Time-dependent pricing selects the active period using the expression timezone, displays that period's first input/output tier, and shows the current-period hint.
- [x] Time-dependent pricing refreshes at minute boundaries while the page is visible.
- [x] Legacy ratio-based and request-based price displays remain compatible.
- [x] The development-only billing prototype route and files are removed.
- [x] Targeted behavior tests, typecheck, lint/format checks, and Playwright validation pass without console errors.
