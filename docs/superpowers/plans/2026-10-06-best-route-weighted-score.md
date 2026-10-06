# 最佳线路加权评分 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让 Dashboard 使用所有 active 线路的 24 小时数据，按成功率 50%、缓存命中率 30%、首字速度 20%选出最佳线路。

**Architecture:** 在 `features/ai-tools/model.ts` 增加纯函数评分与排序逻辑，输入 active 线路和 24h 监控指标，输出按总分排序的线路。Dashboard 单独加载并缓存 24h 指标用于最佳线路，保留现有 1h 健康状态和详情行为。

**Tech Stack:** Vue 3、TypeScript、Vitest、Vue Test Utils。

---

### Task 1: Add weighted scoring unit tests

**Files:**
- Modify: `upstream/sub2api/frontend/src/features/ai-tools/__tests__/model.spec.ts`
- Modify: `upstream/sub2api/frontend/src/features/ai-tools/model.ts`

- [ ] **Step 1: Write failing tests** for active-only candidate filtering, 50/30/20 weighted score, TTFT normalization, missing metrics, and deterministic ties.
- [ ] **Step 2: Run the model test file** with `pnpm vitest run src/features/ai-tools/__tests__/model.spec.ts` and verify the new tests fail because the scoring API is absent.
- [ ] **Step 3: Implement the smallest pure scoring API** in `model.ts`: derive valid metric scores, normalize TTFT per tool candidate set, calculate total score, filter active and complete candidates, and sort ties deterministically.
- [ ] **Step 4: Re-run the model test file** and verify all tests pass.

### Task 2: Use 24h active candidates for Dashboard best route

**Files:**
- Modify: `upstream/sub2api/frontend/src/views/user/DashboardView.vue`
- Modify: `upstream/sub2api/frontend/src/views/user/__tests__/DashboardView.parity.spec.ts`

- [ ] **Step 1: Add failing Dashboard tests** asserting the initial monitor request includes `24h`, an active unlinked route can be selected as best, and an inactive route cannot.
- [ ] **Step 2: Run the focused Dashboard tests** and verify the new assertions fail against the current 1h/request-linked implementation.
- [ ] **Step 3: Add a separate 24h snapshot state and request** for best-route scoring; build candidates from all active groups returned by the workspace API, while retaining existing linked-line rows and 1h health state.
- [ ] **Step 4: Connect each tool card’s `best` field to the weighted scorer** and keep detail best badges based on the existing selected-window behavior unless the card is explicitly using the new best result.
- [ ] **Step 5: Re-run the focused Dashboard tests** and verify they pass.

### Task 3: Full frontend verification

**Files:**
- No additional source files.

- [ ] **Step 1: Run the AI tools model and Dashboard parity tests together.**
- [ ] **Step 2: Run frontend typecheck.**
- [ ] **Step 3: Inspect the diff and confirm no unrelated files changed.
- [ ] **Step 4: Commit the implementation with `feat: rank best routes by weighted 24h quality`.
