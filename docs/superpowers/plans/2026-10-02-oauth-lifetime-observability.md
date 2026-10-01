# OAuth lifetime observations implementation plan

**Goal:** preserve evidence explaining OAuth probe lifetime and load without changing scheduling.
**Architecture:** additive PostgreSQL archive triggers for native state/quality/usage, bounded Go runtime observation queue, SQL analytics over independent retained facts.
**Tech Stack:** Go, PostgreSQL, existing service/repository constructors and psql.
**Spec:** ../specs/2026-10-02-oauth-lifetime-observability.md

## Global constraints
- One writer in this isolated worktree; no production writes, merge, push or deployment.
- Lifetime: first healthy state probe to first later degraded state probe; ignore inconclusive and non-probe actions.
- No sensitive content in event payloads; HMAC identity, missing identities unlinked.
- Preserve all existing requests/quality decisions; runtime loss observable.

### Task 1: Durable observation schema and reports
- [x] Create additive migration `263_oauth_observations.sql` with subject registry, event archive, lifetime summaries, usage minutes, slot facts and retention.
- [x] Add real PostgreSQL tests for healthy/unknown/degraded ordering, duplicate delivery, out-of-order delivery, account/plan deletion survival, safe payloads and usage aggregation.
- [x] Run regression tests exposing absent hooks, immutable-payload corruption, cost correction duplication, retry recovery and client-label leakage; verify fixes against actual PostgreSQL and Go tests.
- [x] Add read-only SQL report and documented fields, coverage and lifecycle semantics.

### Task 2: Runtime producer and hooks
- [x] Add `service/oauth_observation.go` queue/store contract, tests for saturation, batch failures, retry deduplication and shutdown.
- [x] Add `repository/oauth_observation.go` typed persistence and HMAC subject linkage; archive manual/scheduled probes through common boundary.
- [x] Instrument concurrency acquire/release/denial and scheduler selection/outcome, whitelist errors and metadata.
- [x] Wire startup/shutdown through existing providers; verify nil/no-op behavior and request preservation.

### Task 3: Verification and handoff
- [x] Run targeted Go tests, real PostgreSQL migration tests, race tests for recorder, formatting and diff checks.
- [x] Review whole diff against approved spec; fix concrete issues and retain validation commands in task context.
- [x] Deliver local candidate and database migration/deployment status, with any limits made explicit.
