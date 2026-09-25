# Account Import Admission Implementation Plan

> **For agentic workers:** Use subagent-driven-development for independent frontend/backend implementation. User-approved UI correction overrides all earlier mockups.

**Goal:** Gate newly imported/authorized accounts with one candy and one pelican test before native formal group membership.
**Architecture:** Native creation and import APIs carry one optional admission config; backend creates isolated accounts and durable jobs, then applies both results in one promotion transaction. Reuse native account/group/test/scheduler machinery and retain authorization pages.
**Tech Stack:** Go, PostgreSQL/Ent, Vue 3, TypeScript, Vitest.
**Spec:** docs/superpowers/specs/2026-09-26-account-import-admission-design.md

## Global Constraints
- Only two additional authorization controls; original group_ids control and second authorization page remain unchanged.
- admission: {enabled:true,test_group_id?:number}; existing group_ids are eventual groups. JSON root group_ids is added.
- Temp group required on normal/OAuth creation, optional on JSON imports. Candy and pelican each once; AND semantics.
- Each worktree has one writer. No root-main edits, push, deployment, real probes or unrelated changes.
- Native test execution, group binding, default model choices and scheduler invalidation remain authoritative.

## Task 1 — Backend admission lifecycle
- [ ] Inspect CreateAccountInput/admin_account, account create/data/codex/OAuth handlers, scheduled test persistence, native model defaults and runtime wiring.
- [ ] Write direct behavior tests first: enabled creation is non-schedulable before persistence; missing temp group in normal creation rejects; JSON may be ungrouped; destination binding waits for two successful results.
- [ ] Implement the shared service/repository contract and native handler plumbing in upstream/sub2api/backend. Use durable admission state, claim/finish ownership and account/credential/group checks; persist results and scheduler changes transactionally. Keep ordinary paths identical when admission is absent.
- [ ] Test one-success-one-failure, transport inconclusive, immediate enqueue, process lease recovery, subsequent 10-minute rounds, manual edits and duplicate imports. Use local fakes/DB only; no real upstream calls.
- [ ] Run only relevant backend tests and compilation, report exact commands and commit the backend branch.

## Task 2 — Minimal frontend integration
- [ ] Extend frontend request types/API with optional admission and JSON group_ids.
- [ ] Add failing Vitest cases to CreateAccountModal and data-import integration: checkbox gates a required temp selector; existing group_ids is unchanged; no new auth step; unselected/disabled admission preserves payload.
- [ ] Add two controls before the native GroupSelector in CreateAccountModal; keep all current authorization UI and buttons. Pass the shared config through supported create paths without duplicate target UI. Scope to actual supported authorization platforms.
- [ ] Extend ImportDataModal with the previously accepted optional detection and native destination group selector; JSON temp group may be empty.
- [ ] Run targeted Vitest cases, vue-tsc/typecheck; commit frontend branch.

## Task 3 — Integrate and verify
- [ ] Review each worker diff against the spec, cherry-pick only feature commits to the isolated integration branch.
- [ ] Check the frontend/backend request contract, atomic initial isolation, AND gate, existing schedulers and mutation paths.
- [ ] Run affected package tests and frontend typecheck, avoiding repeated unchanged checks; fix only observed gaps.
- [ ] Complete a focused independent review of concurrency/membership and minimal UI constraints, fix actionable findings and rerun affected checks.
- [ ] Record candidate commit, tests, baseline dependency distinction and non-deployment status; preserve worktrees.
