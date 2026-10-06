# Task 1 — upstream PR imports

Base `bf519ef6d4`; branch `codex/bps-shadow-recovery`.

| Source | Local commit | Import |
| --- | --- | --- |
| [ranxi2001/sub2api PR #161](https://github.com/ranxi2001/sub2api/pull/161), head `dc01c71b758e8c24bd76ea5f7ddb089fc76481d3` | `569d282aae` | BPS 429 account failover and BPS-only route cooldown, 19 files. |
| [ranxi2001/sub2api PR #163](https://github.com/ranxi2001/sub2api/pull/163), head `26c623e1b995eeb6ac1ed309d3047630bd3ebc9e` | `2012d30115` | Selective bulk quality-rule editing and responsive quality workspace, 9 files. |

Both PR heads and base `production` were verified through `gh pr view`; immutable commits were fetched from `ranxi2001/sub2api`. No equivalent implementation existed locally. #163 applied cleanly. #161 required three local adaptations: retain the runtime-block comment describing persistent auth revocation, place the BPS cooldown field amid locally extended gateway fields, and import the upstream image/compact 429 test that was absent locally. Its handler regression test checks the locally localized empty-pool response by status, type and safe nonempty message instead of an English phrase. Existing 403 group movement, auth revocation and capture paths were retained.

Verification:

- `go build ./internal/service ./internal/handler` from `upstream/sub2api/backend`: passed.
- `go test -tags unit ./internal/handler -run 'ExcelBPS' -count=1`: passed.
- `go test -tags unit ./internal/service ./internal/handler -run 'ExcelBPS|BPSRateLimited' -count=1`: handler tests exposed the localized assertion, which was adapted and rerun successfully; service package did not compile because unrelated baseline tests conflict. First errors: `ptrFloat` redeclared in `payment_config_plans_validation_test.go:137` and `account_monitor_quality_fusion_test.go:25`; stale `resolveAccountStatsCostResolution` calls in `account_stats_pricing_test.go:1037,1058`; missing `context` import in `gateway_forward_as_chat_completions_test.go:202,239`; undefined image cooldown fields/functions in `openai_images_tool_cooldown_test.go`. `git grep` at base `bf519ef6d4` confirms both duplicate `ptrFloat` declarations and the cost function signature already existed; none of these test files changed in this task. A temporary Go overlay excluding those files still exposed further unrelated stale tests, so no service test pass is claimed. The overlay made no repository edits.
- `pnpm exec vitest run src/utils/__tests__/qualityRulePatch.spec.ts src/views/admin/__tests__/AccountQualityView.spec.ts` from `upstream/sub2api/frontend`: 29 tests passed.
- `pnpm exec vitest run src/i18n/__tests__/localeKeyCompleteness.spec.ts`: 3 tests passed.
- `pnpm run typecheck`: passed. First attempt collided with concurrent initial pnpm installation; serial rerun passed.
- `git diff --cached --check` before each commit: passed.

No push, main integration, server access or deployment. Reviewer check is pending.
