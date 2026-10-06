# OAuth lifetime observability

User approved local implementation of lifecycle/probe/runtime telemetry on 2026-10-02. Primary lifetime is first healthy state probe to first subsequent degraded state probe, account-wide; unknown/network failures and operational actions do not terminate it. No merge/push/deploy authority. Worktree branch codex/oauth-lifetime-observability starts at dfe9e2a259 (origin/main at start).

Spec: docs/superpowers/specs/2026-10-02-oauth-lifetime-observability.md
Plan: docs/superpowers/plans/2026-10-02-oauth-lifetime-observability.md

Root implemented bounded async recorder (`internal/pkg/oauthobs`), typed repository persistence, concurrency/scheduler/probe hooks and cleanup provider. Schema-agent commits and runtime-review fixes were integrated into this branch; all remaining schema/report fixes were completed by root. Only one writer per worktree. Root TASK-CONTEXT.md was preserved unchanged; this is the task-specific context.

Runtime contract: `SELECT oauth_observation_append(accountID,eventKey,occurredAt,eventType,payload)`; function filters OAuth and ensures retained subject. Probe results sync with 2s timeout and idempotent queue fallback. Batch128, queue4096, max3 write attempts, flush1s; health10s, bounded retention every minute. `oauth_observation_recorder_health` fields agreed with agent. Store batches one atomic SQL statement.

Verified: recorder tests initially failed missing implementation, then passed with race; immutable-payload test failed when caller mutated queued pointer, then fixed by copying. Service probe/slot tests initially failed absent hooks, then passed including existing state-probe/concurrency tests. Wire generation passed.

Implementation and independent review complete; final validation and commit recorded below. No production deployment or online collection has occurred. Report entry: docs/reports/oauth-observation-reports.sql and accompanying README. Migration263 is additive with pgcrypto and triggers; deployment must follow the user's database-release policy and needs a separate deployment instruction.

Ruling: implement SQL and runtime concurrently in separate worktrees under dispatching-parallel-agents; no shared writer. No production reads/writes necessary for implementation. Docker shared disk full: schema agent instructed to use task-owned PostgreSQL tmpfs, never delete shared Docker data.

Review fixes: exclude Live admission from occupancy while retaining raw slots; internal attempt correlation across HTTP/WS-turn runtime hooks; deadline-aware writer shutdown; recorder resumes after exhausted transient batch retries; lifetime extrema survive retention and concurrent/out-of-order probes; native standard/site costs and BPS protocol corrected on updates; before/after snapshots exclude timestamp-only writes; arbitrary client model labels become other_model. Regression tests reproduced the defects before fixes where noted in test commits. Probe lifetime remains strictly account-wide first healthy to first subsequent degraded.

Validation:
- `go test -race ./internal/pkg/oauthobs -count=1` passed.
- Scoped unit service OAuthObservation, state-probe, OpenAI scheduler, concurrency and quality tests passed.
- Scoped repository OAuthObservation / QualityObservation tests passed.
- Server cleanup test and handler/route endpoint checks passed; Wire generation passed.
- `bash tests/operations/oauth_observation_migration_postgres_test.sh` passed on actual PostgreSQL16 in task-owned tmpfs, including retained evidence, source correction, protocol, privacy, minute occupancy and executable report.
- `DOCKER_HOST=unix:///Users/gongtengxinwen/.colima/default/docker.sock TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE=/var/run/docker.sock SUB2API_TEST_POSTGRES_TMPFS=1 SUB2API_TEST_POSTGRES_IMAGE=postgres:16-alpine go test -tags integration ./internal/repository -run '^(TestOAuthObservationNativeSchemaRoundTrip|TestQualityEnableBPS|TestQualityBPS)' -count=1` passed with full native migrations, concurrent probe writes, deletion retention and quality-action provenance.
- Broadened checks found TWO existing baseline failures, reproduced on the untouched main checkout: `TestSettingService_GetAllSettings_OpenAIAdvancedSchedulerEffectiveValuesUseConfig` expects99 but receives10; `TestQualityActionsRestoreOwnershipAndStaleRuns` fails at ClaimPelican before action execution (both remove_groups/disable_scheduling subcases). These are not claimed passing and were not changed as unrelated baseline issues.
- Scope decisions: no fabricated historical backfill; unknown model categories sacrifice label detail to prevent content/secret leakage; current names resolved only via authorized report joins. No broader UI or billing source added.
