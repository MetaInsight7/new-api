# Use the newer second-development reporting baseline

**Status:** accepted

The activation funnel and usage report migration will use `new-api-solve/main` as the functional reference. The Overlay implementation is historical context only because its reporting behavior is outdated; the code will still be adapted to the `tokendazi_op` base domain and will not be wholesale-merged. The two reports remain separate, and their activation metrics must retain explicit, context-specific definitions. Since `tokendazi_op` already contains wallet, top-up, and subscription domains, the migration retains the newer implementation's supported recharge, payment, and subscription metrics instead of silently dropping them.

The migration keeps the newer implementation's final frontend behavior, preserves the report-specific date boundaries, and retains asynchronous historical filling, periodic current-day refresh, model-level usage aggregation, CSV export, and a read-only reporting path that does not alter request or billing hot paths.
