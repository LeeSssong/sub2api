# BPS Shadow Recovery Implementation Plan

> Execute tasks sequentially using subagent-driven-development, one writer per worktree. Direct related verification only.

**Goal:** Import PRs #161/#163 and implement recoverable BPS fallback, stopping at deployable state.
**Architecture:** Existing account extra persists desired configuration and recovery state; gateway routes regular traffic during degradation; bounded background probes claim due accounts and restore via optimistic conditions.
**Tech Stack:** Go, PostgreSQL/Ent, Vue, TypeScript, Vitest.
**Spec:** docs/superpowers/specs/2026-09-27-bps-shadow-recovery-design.md

## Global Constraints
- Preserve root AGENTS.md and every other worktree; no main writes, pushes, servers or deployment.
- One writer; sequential implementation tasks, scoped tests. No schema change unless necessary and explicitly reported.
- Native BPS probe and quality state probe are distinct; user selection is pending.

## Task 1: Import upstream PRs
- [ ] Fetch immutable PR diffs/commits, check equivalence, apply under upstream/sub2api with three-way adaptation.
- [ ] Preserve local auth, 403 groups, BPS capture and model behavior.
- [ ] Run PR-related backend unit tests and frontend quality rule tests/typecheck; capture commands and results.
- [ ] Commit with original URLs/SHAs; reviewer checks upstream coverage and local compatibility.

## Task 2: Backend recovery
Files: backend/internal/service/account_excel_bps_shadow.go (new), openai_excel_bps.go, account.go, account_test_service_bps_probe.go, backend/internal/repository/account_repo_excel_bps_shadow.go (new), existing scheduler lifecycle/admin extra handling.
- [ ] Add failing tests for 403/5xx degradation, independent normal models, timing cap, status/manual cancellation and stale restores.
- [ ] Implement account helpers and guarded persisted transitions; connect every actual BPS upstream response error path, excluding 429.
- [ ] Reuse BPS probe and existing runner lifecycle with atomic claims, bounded concurrency/timeouts and fresh status checks.
- [ ] Verify focused backend tests, repository transitions and compile; commit.

## Task 3: Account configuration UI
Files: frontend/src/components/account/{CreateAccountModal,EditAccountModal,BulkEditAccountModal}.vue, native i18n/types/helpers and focused tests.
- [ ] Add tests for visibility, normal model selection, serialization, degraded-account editing, manual disable.
- [ ] Reuse ModelWhitelistSelector for independent regular-mode models; show runtime fallback and next probe safely.
- [ ] Verify component tests, typecheck and production frontend build; commit.

## Task 4: Deployment readiness
- [ ] Review branch for actual regressions and spec gaps; resolve blockers with relevant regression checks.
- [ ] Compile backend and frontend, diff-check, record source commits/tests/limitations and rollback scope in this task's handoff.
- [ ] Leave clean feature branch, report readiness and await explicit deployment instruction.
