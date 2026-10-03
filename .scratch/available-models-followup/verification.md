# Verification record

## Automated checks

- Targeted Vitest: 8 files, 31 tests passed.
- TypeScript typecheck passed.
- Targeted Oxlint passed.
- Targeted Oxfmt check passed.
- `git diff --check` passed.
- Full `bun run test` was started but produced no output or completion within approximately three minutes and was safely interrupted; it is not reported as passing.

## Browser acceptance

Playwright was installed temporarily under `/tmp/newapi-playwright` from the npm official registry without changing project dependencies. Against an isolated SQLite backend and dev frontend, the browser check passed for:

- Administrator Available Models rendering, documentation-link target/security attributes, model cards and prices.
- Group-to-API-Key deep link `open=create&group=default` and Dialog visibility.
- Narrow-screen Dialog rendering with the `View models` action visible.
- Regular-user navigation retaining Workspace and Available Models while hiding Overview and Dashboard.

URL cleanup and the remaining Dialog state branches remain covered by the targeted Vitest suite.
