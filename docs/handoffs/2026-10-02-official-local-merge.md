# October 2 official update — local integration

- User authorized merging into local main and removing this chat’s isolated worktree; no push or deployment.
- Local base: `fc35dd90ef`, preserving all eight existing commits ahead of origin/main.
- Official source: `ranxi2001/sub2api@0994fe0f1c99398b99885f2276dd3b978046da04` (production, version 2.9.6).
- Previous imported official baseline: `a20c0334a6bccc6e7d082bfba9a4f1cb075ef50c`.
- Applied binary three-way diff under `upstream/sub2api`. Resolved 11 conflicted files preserving local OAuth observations, regular session proxies and process-role behavior alongside official API-key admission, Live lease transfers and Prism usage guards.
- Adapted new upstream tests to local public-error sanitization. The provisional WebSocket usage settlement fixture starts semantic output before the error, preserving local pre-output failover while checking authoritative usage and single release.
- Includes migration `237_add_api_key_concurrency_limit.sql` as source only; no database migration executed.

## Verification

- Frontend production build, TypeScript and locale completeness checks passed. Eleven selected frontend test files: 276 tests passed; locale check separately: 3 passed.
- Backend embedded application build passed (`go build -tags embed ./cmd/server`).
- Selected tests in prismbridge, openai, apicompat, config and openai_ws_v2 passed.
- Final selected service/handler/repository/middleware tests passed with `-tags unit -p 1`, covering Prism, API Key admission, Codex catalogs, GPT-6.1, Live, WebSocket passthrough, turn admission and OAuth observations. The six baseline failures below were explicitly excluded.
- Six unrelated existing failures were reproduced on untouched local base and excluded from the final selected run: TestFilterCodexInput_CustomToolCallUsesCtcNamespace; TestCodexImagesLunaErrorDoesNotCoolImageAccount; TestRateLimitService_HandleUpstreamError_APIKeyModel401UsesModelRateLimit; TestUpdateExtraCodexDisplaySnapshotsAvoidSchedulerOutbox; TestApiKeyAuthWithSubscriptionGoogle_InsufficientBalance; TestApiKeyAuthWithSubscriptionGoogle_RejectsExhaustedBalance.
- Prism adapter: 13 offline tests passed with Playwright 1.60.0 in a temporary venv. Upstream pins 1.63.0, which the available package index could not supply; exact pinned runtime and live browser/OAuth behavior remain unverified. No dependency pin changed.
- Scoped independent review found no semantic merge blockers. Diff whitespace checks passed.
- No database integration tests, real upstream requests, server access, push or deployment. These are local checks only.
