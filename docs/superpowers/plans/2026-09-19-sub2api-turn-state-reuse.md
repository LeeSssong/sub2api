# Sub2API Turn-State Reuse Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add the same account-scoped reusable Astra turn-state behavior to the native Sub2API process and administration surface.

**Architecture:** A service-layer turn-state module uses Redis for hot tickets and leases, with no transport plugin. Group scope is persisted through Ent plus an additive migration; the OpenAI gateway overlays tickets only after existing turn-state guarding. The harvester runs as a bounded backend worker and uses harvest-only proxy routes.

**Tech Stack:** Go, Ent, PostgreSQL, Redis, Vue 3/TypeScript, pnpm/Vitest.

**Spec:** `/Users/gongtengxinwen/Documents/codex2api/docs/superpowers/specs/2026-09-20-codex-turn-state-reuse-design.md`

## Global Constraints

- Match codex2api ticket shape, TTL, renewal, credential hash, status, miss, and recovery semantics exactly.
- Only `platform=openai`, `type=oauth`, enabled-group accounts enter scope.
- Keep `openai_codex_turn_state.go` provenance behavior intact; overlay only current-account Astra tickets.
- Do not enable or depend on the protection transport plugin.
- Redis/injection failures fail open; request paths never wait for harvesting.
- Additive database migration means production release is maintenance/downtime and must stop before service shutdown until explicitly authorized.

---

### Task 1: Shared Ticket Domain and Redis Store

**Files:**
- Create: `backend/internal/service/openai_turn_state_reuse.go`
- Create: `backend/internal/repository/openai_turn_state_store.go`
- Test: matching `*_test.go`

**Interfaces:**
- Produces: parse/key/status/policy types, `OpenAITurnStateStore.Get/Put/Delete/AcquireLease`, and `ApplyOpenAITurnStateOutbound`.

- [ ] Write failing parse/policy/store tests for all approved/rejected lengths, TTL/future clock, credential rotation, stale publish, Redis failure, and compact/non-Astra no-op.
- [ ] Implement domain and Redis repository; rerun focused service/repository tests GREEN.
- [ ] Commit with `git commit -m "feat(turnstate): add native ticket store"`.

### Task 2: Ent Group Field, Migration, Settings, and DTOs

**Files:**
- Modify: `backend/ent/schema/group.go` and generated Ent files
- Create: next numbered `backend/migrations/*_turn_state_reuse.sql`
- Modify: group service/repository/handler/DTO mappings
- Create/modify: setting feature service and admin routes
- Test: schema, migration, setting, group handler/service tests

**Interfaces:**
- Produces: `Group.TurnStateInjectEnabled`, validated `TurnStateReuseSettings`, and redacted status DTO.

- [ ] Add failing schema/migration/default/validation/mapping tests and confirm RED.
- [ ] Add field/migration, run `go generate ./ent`, update mappings and group validation (OpenAI only).
- [ ] Add JSON setting key with default-off normalization and admin get/update/status endpoints.
- [ ] Rerun focused tests GREEN and commit `feat(turnstate): persist native reuse controls`.

### Task 3: Gateway and Scheduler Integration

**Files:**
- Modify: `backend/internal/service/openai_gateway_service.go`, `openai_gateway_forward.go`, `openai_ws_http_bridge.go`, relevant WS payload path
- Modify: `backend/internal/service/openai_account_scheduler*.go`
- Test: gateway, WS, scheduler, and existing turn-state suites

- [ ] Add failing tests for Astra overwrite, compact/non-Astra/out-of-scope unchanged headers, current-account failover tickets, no-ticket-none forwarding, non-`none` miss exclusion, and no-candidate error.
- [ ] Overlay HTTP/WS tickets after normal header filtering and existing provenance guard.
- [ ] Add scope/miss eligibility to all OpenAI scheduling paths, including sticky and retry selection.
- [ ] Verify outbound tickets never become incoming continuation evidence; rerun focused tests GREEN.
- [ ] Commit with `git commit -m "feat(turnstate): inject Astra tickets in native gateway"`.

### Task 4: Native Harvester and State Actions

**Files:**
- Create: `backend/internal/service/openai_turn_state_harvester.go`
- Modify: dependency wiring/startup and account repository APIs
- Test: harvester/repository/runtime tests

- [ ] Add failing httptest cases for two-call qualification, actual-model check, incomplete SSE, route rotation, 429 backoff retaining ticket, 401 current-hash deletion, lease, cadence, and shutdown.
- [ ] Implement bounded worker using explicit URLs or active unexpired proxy inventory without changing account `proxy_url`.
- [ ] Implement `none/rebind_group/unbind_groups/unschedulable` and recovery transitions; use reason exactly `turn_state_miss` and clear only that reason.
- [ ] Wire startup/shutdown, rerun tests GREEN, and commit `feat(turnstate): harvest native Astra tickets`.

### Task 5: Vue Administration Surface

**Files:**
- Modify: group editor, settings/Turn-State panel, account list/detail, API/types, zh-CN/en/ja locales
- Test: related Vitest suites

- [ ] Add failing component/API tests for defaults, OpenAI-only group toggle, target-group validation, and redacted ticket status.
- [ ] Implement controls and status table using existing form/table components.
- [ ] Run focused `pnpm test`, typecheck, and `pnpm build` GREEN.
- [ ] Commit with `git commit -m "feat(turnstate): add native reuse admin UI"`.

### Task 6: Candidate Verification, Merge, and Stop Gate

- [ ] Run `gofmt`, `git diff --check`, focused tests, `go test ./...`, migration verification, frontend tests/typecheck/build.
- [ ] Commit plan/handoff and push `codex/turn-state-reuse-sub2api`.
- [ ] Merge through clean root `main`, push `origin/main`, and run production release preflight only.
- [ ] Because the candidate contains a schema migration, report expected interruption, migration scope, backup/restore path, and stop before any service shutdown or production write until the user explicitly authorizes downtime.
