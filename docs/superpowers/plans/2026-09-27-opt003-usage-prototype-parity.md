# OPT-003 Usage Prototype Parity Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Match the frozen v0.2 `/usage` page structure without replacing real data with prototype Mock values.

**Architecture:** Recompose only the user UsageView into three continuous sections, keeping existing shared data components and their API bindings. Use page-scoped styling to flatten their card shells; retain independent error boundaries.

**Tech Stack:** Vue 3 SFC, Tailwind, Vitest/Vue Test Utils, Vite.

---

### Task 1: Lock Page Topology

**Files:** `upstream/sub2api/frontend/src/views/user/__tests__/UsageView.spec.ts`, `upstream/sub2api/frontend/src/views/user/UsageView.vue`

- [ ] Add a test asserting `.usage-overview` contains the two selectors before `.usage-stats`, `.usage-analysis-grid` contains the four charts in order, and `.usage-records` contains filter controls plus `UsageTable` with `flat=true`.
- [ ] Run `./node_modules/.bin/vitest run src/views/user/__tests__/UsageView.spec.ts` and confirm the new assertion fails on the old separate-card page.
- [ ] Move existing controls/components into those semantic sections without changing their props or handlers; run the same test to green.

### Task 2: Match Responsive Surface

**Files:** `upstream/sub2api/frontend/src/views/user/UsageView.vue`

- [ ] Add page-scoped rules: overview 4-column/2-column statistics with subtle separators, no icon badges; analysis 2-column/1-column grid with one outside border and interior separators; records one work surface. Preserve focus, errors, and empty text.
- [ ] Run the targeted usage tests, `vue-tsc --noEmit`, and `vite build`; inspect desktop/mobile render and adjust only observed overflow or hierarchy defects.

### Task 3: Record Baseline Comparison

**Files:** `docs/project/OPT-003-v02-usage-parity-followup-2026-09-27.md`

- [ ] Compare six frozen initial page captures to current candidate at matching 1280×900 and 390×844 where a legitimate session exists; state role, viewport, step, differences and unverified real API states.
- [ ] Run `git diff --check`, verify root changes unchanged and candidate branch remains isolated. Do not push or deploy without fresh authorization.
