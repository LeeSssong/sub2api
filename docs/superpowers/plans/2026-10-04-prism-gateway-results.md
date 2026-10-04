# Prism implementation result — deployment pending

Date: 2026-10-04, Asia/Shanghai. Branch: `codex/prism-gateway-improvements`. Base: fetched `origin/main` `3156ad1d0e616174147d563fa282d27221f461d7`. This is a local development report, not a deployment record; changes are retained as a reviewable working-tree diff, not merged, pushed, or deployed.

## Implemented

- Port PR #292 net diff (head `36805e35a7983471bcb2fd4f2e5efead477269a9`, base `a146fb98dd2da8cb4ae5eeeb8279e9767d379009`) to the embedded `upstream/sub2api` tree. Preserve later local login/anonymous checks, model scopes and 30-second editor readiness budget.
- Reclaim closed Chromium editor contexts; wait at most 30 seconds for memory to fall below 750 MiB. Keep 900 MiB service hard bound and unknown-outcome journals.
- Detect project creation/runtime startup rate limiting, cool only that account for at least 60 seconds, preserve in-flight work.
- Permit official-page reconnection starts only after explicit completed sandbox_reconnecting failure, with unchanged input/conversation/previous response/project/model/effort and at most three attempts. Never retry unknown outcomes.
- Add allowlisted terminal reasons, diagnostic enums, numeric status, hint categories and failure receipt codes. Preserve upstream project access 403 without confusing it with private bridge authentication errors.
- Expose `PRISM_ADAPTER_MAX_ACCOUNTS=1|2` (default 1) for independent account browser contexts in multiplex mode. Retain global/per-account inflight, bootstrap and memory bounds; do not automatically enable or raise live concurrency. Account selection reuses the native Sub2API scheduler.
- Add per-account admitted/waiting/peak metrics and elapsed stage logs. These are not token timing or proof of active model execution.
- Add Go Prism-only waiting SSE comments using existing configured keepalive interval (default 10 seconds). After headers are committed, failures end once with response.failed; terminal/body/tool validation remains buffered. Comments usually have no visible UI output; no promise of faster answer text or reasoning display.

## Isolation and scope

New Go behavior is inside the existing selected-Prism forwarding branch. Disabled OAuth/API-key accounts and unselected models stay native. Four existing Prism model IDs and 6.1 Sol client-tool scope unchanged. No database migration, new account ledger, token estimation, protocol/model fallback, prompt manipulation or automatic tool execution. No live server, credentials, production account API or test station accessed for this development.

## Verification

- Python 3.12.14, Playwright 1.63.0 from `/tmp/prism-oct03-venv`, project-pinned requirements. Complete adapter unittest discovery: **90 passed**. PR tests were first run against baseline and failed for missing behavior; new metrics/config tests likewise failed before implementation.
- Real HTTP heartbeat test: adapter waits for the client to receive SSE comment before releasing terminal. Both successful terminal and upstream 429/failure cases pass. Regressions cover disabled OAuth/API-key and unselected-model routes.
- `go test -race -tags=unit ./internal/service ./internal/handler -run 'Test(Prism|AccountUsesPrism|PrismBrowser)' -count=1`: **both packages passed**. Main agent ran with required unit build tag; reviewer shell without that tag encountered unrelated test compilation setup.
- Real pinned Chromium headless shell + mock upstream, one account, 5 concurrent, 6.1 Sol/xhigh, 3 rounds, reconnect-first: **15/15 completed**, 15 successful starts plus 5 explicit environment reconnect attempts, 5 projects reused, peak **2 pages / 1 context / 5 admitted**, state mismatches 0. The fixture holds completion until all batch starts arrive, so a serial executor cannot pass.
- Same real browser/mock fixture, **two accounts**, 5 concurrent, 3 rounds: **15/15 completed**, 15 starts, 5 projects, peak **3 pages / 2 contexts / 5 admitted**, state mismatches 0.
- `git diff --check`: passed. Independent Codex review: no remaining actionable findings; main agent supplied successful pinned-runtime and unit-tag test evidence.

## Expected effect and limits

Expected fewer transient memory-pressure refusals and clearer account-specific startup cooldowns; bounded dual-account contexts allow a second account to work alongside a busy first context. SSE comments can prevent idle disconnects and retries if the front proxy streams/flushing is configured; they do not make model answers appear sooner. Diagnostics allow preparation, memory wait, polling and upstream failure to be distinguished.

All browser load tests used synthetic upstream responses and **0 real Prism inference requests**. Linux cgroup memory behavior is unit tested, not measured on this Mac. Real login handoff, account entitlements, 5/20/30/50/59 capacity, latency gains, proxy timeout behavior, and long real tool sessions remain unverified. Current logged-out account limitation is not solved by this change. Require user approval before integration/deployment and use only isolated targeted real validation after deployment.

## Agent participation

Implementation and tests: primary Codex, GPT-6; more specific runtime model ID/reasoning strength unavailable. Independent code review: Codex subagent `prism_review`, inherited GPT-6; more specific model ID/reasoning strength unavailable. No other agent models selected.
