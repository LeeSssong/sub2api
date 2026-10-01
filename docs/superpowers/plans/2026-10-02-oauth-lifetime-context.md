# OAuth lifetime observability

User approved local implementation of lifecycle/probe/runtime telemetry on 2026-10-02. Primary lifetime is first healthy state probe to first subsequent degraded state probe, account-wide; unknown/network failures and operational actions do not terminate it. No merge/push/deploy authority. Worktree branch codex/oauth-lifetime-observability starts at dfe9e2a259 (origin/main at start).

Spec: docs/superpowers/specs/2026-10-02-oauth-lifetime-observability.md
Plan: docs/superpowers/plans/2026-10-02-oauth-lifetime-observability.md

Root implements bounded async recorder (`internal/pkg/oauthobs`), typed repository persistence, concurrency/scheduler/probe hooks, cleanup provider. Schema agent in separate worktree `/Users/gongtengxinwen/.codex/worktrees/oauth-observation-schema/sub2api搭建`, branch codex/oauth-observation-schema, writes migration 263, actual PostgreSQL tests and report SQL/docs. Cherry-pick its commit when ready. Only one writer per worktree.

Runtime contract: `SELECT oauth_observation_append(accountID,eventKey,occurredAt,eventType,payload)`; function filters OAuth and ensures retained subject. Probe results sync with 2s timeout and idempotent queue fallback. Batch128, queue4096, max3 write attempts, flush1s; health10s, bounded retention every minute. `oauth_observation_recorder_health` fields agreed with agent. Store batches one atomic SQL statement.

Verified: recorder tests initially failed missing implementation, then passed with race; immutable-payload test failed when caller mutated queued pointer, then fixed by copying. Service probe/slot tests initially failed absent hooks, then passed including existing state-probe/concurrency tests. Wire generation passed.

Remaining: quality transaction source annotation, repository/store tests, scheduled-source and fallback tests, integrate migration + actual DB tests + docs, final independent branch review and related validation.

Ruling: implement SQL and runtime concurrently in separate worktrees under dispatching-parallel-agents; no shared writer. No production reads/writes necessary for implementation. Docker shared disk full: schema agent instructed to use task-owned PostgreSQL tmpfs, never delete shared Docker data.
