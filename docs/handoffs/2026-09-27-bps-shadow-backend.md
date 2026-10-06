# Task 2 — BPS persistent recovery backend

Base `ba6121afba`, branch `codex/bps-shadow-recovery`. Backend only; UI integration remains with the orchestrator.

Implemented:
- Every actual BPS HTTP403/500–599 degrades, including model-access rejection, corrective requests and native uploads. Transport/stream errors and429 do not; business429 retains imported failover/cooldown behavior. Non-shadow accounts switch BPS off with safe status/time metadata. Shadow accounts preserve desired configuration and groups.
- Existing account `extra` stores server-owned recovery state (active, generation, trigger status, timing, failure count, safe result, renewable ownership lease). No schema migration. Atomic PostgreSQL claims use row locks and SKIP LOCKED. Successful restore requires same config, credentials, proxy, recovery generation/lease and current normal eligibility; unrelated quota/usage updates are allowed. A late pre-degradation failure cannot degrade a recovered generation.
- Initial delay5min, failed probe delay10/15/20/25/30min, capped30min. Three concurrent probes per batch, repeated bounded batches,3min probe timeout/4min lease. Existing scheduler lifecycle owns cancellation; due recovery is independent of user test plans. Expired claims recover after restarts. Actual tick is minute-granularity.
- Fallback whitelist is an independent identity model mapping before passthrough checks, including model-list projection. BPS mapping is restored unchanged. Manual shadow-off keeps fallback; desired BPS-off clears runtime and normal-model settings. New opt-in requires a nonempty normal model list. Single/bulk/partial admin writes cannot forge runtime state.
- Native text + exact nonce echo tool + completed tool-result acceptance. Probe copies account context, never creates a schedulable account or client business-usage/billing record, bypasses token refresh and suppresses account auth/group/quota/cooldown mutations; eligibility/config/credentials are rechecked before each network step. Native-only acceptance is the stated assumption after unanswered optional clarification, reversible before deployment.

Verification (run from repository root unless stated):

1. `python3 tests/operations/bps-shadow-recovery/run-service-tests.py`: PASS. Script uses `go list -tags=unit -json` for all actual production service files plus exact real test/helper files in tracked `tests/operations/bps-shadow-recovery/service-test-files.json`; runs `ExcelBPS|BPSProbe` tests. It does not delete, suppress or rewrite unrelated test files and does not claim a full package test pass. Coverage includes403/5xx baseline,429 regression, repairs/uploads, preservation/mapping/model-list behavior, native acceptance failures, shadow error isolation, scheduler rechecks and admin/group settings.
2. From `upstream/sub2api/backend`: `go test -tags=unit ./internal/repository ./internal/handler -run 'ExcelBPS|SchedulerMetadataAccount' -count=1`: PASS.
3. From backend: `DOCKER_HOST=unix:///Users/gongtengxinwen/.colima/default/docker.sock TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE=/var/run/docker.sock SUB2API_TEST_POSTGRES_IMAGE=postgres:15-alpine SUB2API_TEST_POSTGRES_TMPFS=1 go test -tags=integration ./internal/repository -run 'TestExcelBPSRecovery|TestMoveExcelBPSOn403HonorsCurrentSettings' -count=1 -timeout=8m`: focused real PostgreSQL15 transitions, five concurrent callers/single winner, stale lease, restart/expiry, relevant versus irrelevant edits, disabled/expired/admission/cooldown states, legacy status behavior and admin state preservation. Uses temporary local DB/Redis only.
4. From backend: `go build ./internal/service ./internal/repository ./internal/handler`: PASS.
5. `git diff --check`: PASS.

Limits and findings: full service package compilation still fails on pre-existing duplicate helper and missing image/OAuth cooldown APIs; no full-suite pass claimed. The native selected-files suite also exposed an existing missing bulk-target prefetch for BPS settings; fixed narrowly with `openAISettings.any()` before target validation. Initial PostgreSQL18 testcontainer could not initialize because Colima disk was full; no shared containers/images/caches were deleted. Existing harness PostgreSQL15+tmpfs override ran migrations and exercised the targeted transitions; this does not assert full production schema/runtime equivalence. No live BPS request or production validation was performed.

Root owns UI cherry-pick, final deployable build/review and authorization boundary. No push/main integration/server access/deployment performed.

## Scoped final-review fixes — 2026-09-28

Two P2 review regressions were reproduced with failing tests and fixed on integrated base `340a9faa5b`:

- Same-ID proxy changes: compare a SHA-256 fingerprint of transport-relevant proxy fields before every native probe step. Degrade/Finish now lock the referenced proxy row `FOR SHARE` in the transition transaction and compare those fields, preventing an address/authentication edit from committing between comparison and state transition. Labels, observation timestamps and warning preferences are excluded; secrets are not logged. Tests prove unchanged/metadata-only proxies pass and same-ID host/password edits reject old business failures and stale recovery success.
- Full extra replacements: explicitly preserve an omitted saved fallback whitelist while desired BPS remains on, both at the admin service boundary and repository merge under the account lock. An explicit invalid/empty replacement still follows validation. Bulk/partial payloads retain patch semantics. Tests cover active recovery and paused recovery full edits, plus real PostgreSQL full/bulk/partial saves preserving normal-only support and rejecting unlisted models.

Scoped verification:

- `python3 tests/operations/bps-shadow-recovery/run-service-tests.py`: PASS (3.731s); selected production/service test harness, not a full package suite.
- From backend, `DOCKER_HOST=unix:///Users/gongtengxinwen/.colima/default/docker.sock TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE=/var/run/docker.sock SUB2API_TEST_POSTGRES_IMAGE=postgres:15-alpine SUB2API_TEST_POSTGRES_TMPFS=1 go test -tags=integration ./internal/repository -run 'TestExcelBPSRecoverySameProxyTransportGuard|TestExcelBPSRecoveryWhitelistReplacementAndPatch|TestExcelBPSRecoveryAtomicLifecycle|TestExcelBPSRecoveryRejectsStaleAcceptance' -count=1 -timeout=8m`: PASS (5.268s), real local PostgreSQL15.
- `go build ./internal/service ./internal/repository ./internal/handler` and `git diff --check`: PASS.

No frontend changes/tests, server access, push or deployment. Existing orchestrator-owned docs changes were preserved; final Linux artifact relink remains with the orchestrator.
