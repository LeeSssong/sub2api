# Retire Custom Scheduler Artifacts Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Permanently remove retired custom scheduler settings, persisted logs, APIs, and admin UI while preserving native OpenAI scheduling controls.

**Architecture:** Add one idempotent destructive migration for the seven retired settings and log table, then remove the now-dead backend dependency graph and frontend surfaces. Keep native scheduler settings and request behavior unchanged, and prove the boundary with negative route/UI tests plus positive native-setting tests.

**Tech Stack:** PostgreSQL migrations, Go/Gin/Wire, Vue 3/TypeScript/Vitest, pnpm/Vite.

**Spec:** `docs/superpowers/specs/2026-09-19-retire-custom-scheduler-artifacts-design.md`

## Global Constraints

- Work only in `codex/retire-custom-scheduler-artifacts`; do not modify root `main` or other worktrees.
- Do not merge, push, deploy, or write production data.
- Delete only the seven retired settings named in the spec; preserve native scheduler switches, Top-K, and weight settings.
- Do not modify existing migration `234_openai_scheduler_logs.sql`; add an idempotent later migration.
- Follow red-green TDD for behavior and contract changes.

---

### Task 1: Destructive Migration Contract

**Files:**
- Create: `upstream/sub2api/backend/migrations/240_remove_custom_scheduler_artifacts.sql`
- Create: `upstream/sub2api/backend/migrations/remove_custom_scheduler_artifacts_migration_test.go`

**Interfaces:**
- Produces an embedded migration that deletes exactly seven settings and drops `openai_scheduler_logs` with `IF EXISTS`.

- [ ] Write a migration test that reads `240_remove_custom_scheduler_artifacts.sql`, asserts all seven retired keys and `DROP TABLE IF EXISTS openai_scheduler_logs`, and asserts native Top-K/weight/sticky keys are absent from the delete statement.
- [ ] Run `go test ./migrations -run TestRemoveCustomSchedulerArtifactsMigration -count=1` and verify it fails because the migration is missing.
- [ ] Add the idempotent SQL migration using `DELETE FROM settings WHERE key IN (...)` and `DROP TABLE IF EXISTS`.
- [ ] Re-run the migration test and verify it passes.
- [ ] Commit the migration and contract test.

### Task 2: Backend Log and Experience Surface Removal

**Files:**
- Delete: scheduler log service, sink, repository, handler, and their dedicated tests.
- Modify: `upstream/sub2api/backend/internal/service/wire.go`
- Modify: `upstream/sub2api/backend/internal/repository/wire.go`
- Modify: `upstream/sub2api/backend/internal/handler/wire.go`
- Modify: `upstream/sub2api/backend/cmd/server/wire.go`
- Regenerate/modify: `upstream/sub2api/backend/cmd/server/wire_gen.go`
- Modify: `upstream/sub2api/backend/internal/server/routes/admin.go`
- Delete or trim: scheduler experience service/handler tests and frontend-facing response types.
- Test: backend route contract tests.

**Interfaces:**
- Removes `/api/v1/admin/scheduler/logs`, `/api/v1/admin/scheduler/logs/:id`, and `/api/v1/admin/ops/openai-scheduler-experience`.
- Preserves other admin ops and request telemetry routes.

- [ ] Change route tests first to require the retired endpoints to be absent while representative neighboring admin routes remain registered.
- [ ] Run the focused route tests and verify they fail against current routes.
- [ ] Remove route registrations, handlers, repository, sink, lifecycle wiring, and unused models.
- [ ] Remove scheduler-log-only calls from native convergence tests while retaining native scheduling assertions.
- [ ] Regenerate Wire output with the repository's existing generation command or make the minimal equivalent generated update.
- [ ] Run focused service/handler/repository/server tests and verify they pass.
- [ ] Commit backend surface removal.

### Task 3: Retired Setting Model Removal

**Files:**
- Modify: setting constants, parsers, settings view, DTOs, GET/PUT handlers, and scheduler setting reads.
- Modify: related backend setting and scheduler tests.

**Interfaces:**
- Settings GET omits the seven retired fields.
- Settings updates cannot recreate the seven deleted rows.
- Native scheduler switch, sticky, subscription priority, Top-K, and weights remain functional.

- [ ] Write failing tests asserting retired fields are absent from serialized settings and native fields still round-trip.
- [ ] Run focused setting tests and verify the new assertions fail.
- [ ] Remove the seven constants, DTO fields, parsers, validation/update branches, compatibility defaults, and scheduler reads.
- [ ] Keep native setting parsing and update behavior unchanged.
- [ ] Run focused settings and scheduler tests and verify they pass.
- [ ] Commit backend retired-setting removal.

### Task 4: Frontend Page and Client Removal

**Files:**
- Delete: `SchedulerLogsView.vue`, scheduler log API client, experience card, and dedicated tests.
- Modify: router, sidebar, admin API index, ops API types/functions, Ops dashboard, settings API/types/view, and zh/en locale files.
- Modify: router/sidebar/settings/dashboard tests.

**Interfaces:**
- No `/admin/scheduler-logs` route or sidebar entry.
- No scheduler experience card or API request.
- No controls or payload fields for the seven retired settings.
- Native scheduler controls remain visible and writable.

- [ ] Update router/sidebar/dashboard/settings tests first to assert the retired surfaces are absent and native controls remain.
- [ ] Run the focused Vitest files and verify they fail against the current UI.
- [ ] Remove retired routes, components, clients, fields, controls, computed state, payload construction, and i18n messages.
- [ ] Run focused Vitest tests and verify they pass.
- [ ] Run `pnpm run typecheck` and fix only references caused by this removal.
- [ ] Commit frontend removal.

### Task 5: Candidate Verification and Handoff

**Files:**
- Create: `docs/superpowers/reports/2026-09-19-retire-custom-scheduler-artifacts-verification.md`

**Interfaces:**
- Produces a local-only release candidate identity and explicit maintenance-release warning.

- [ ] Run migration tests, focused backend tests, `go build ./cmd/server`, focused frontend tests, `pnpm run typecheck`, and `pnpm run build`.
- [ ] Search for retired keys, log routes, page imports, repository/sink symbols, and experience API; classify any remaining occurrence as historical migration/test evidence or remove it.
- [ ] Run `git diff --check` and inspect `git status`.
- [ ] Write the verification report with commands, results, commit/tree, migration hash change, expected downtime release mode, rollback requirement, and unverified items.
- [ ] Commit the verification report and stop without merging, pushing, or deploying.
