# 最佳线路加权评分 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [x]`) syntax for tracking.

**Goal:** 让 Dashboard 使用所有 active 线路的 24 小时数据，按成功率 50%、缓存命中率 30%、首字速度 20%选出最佳线路。

**Architecture:** 在 `features/ai-tools/model.ts` 增加纯函数评分与排序逻辑，输入 active 线路和 24h 监控指标，输出按总分排序的线路。Dashboard 单独加载并缓存 24h 指标用于最佳线路，保留现有 1h 健康状态和详情行为。

**Tech Stack:** Vue 3、TypeScript、Vitest、Vue Test Utils。

---

### Task 1: Add weighted scoring unit tests

**Files:**
- Modify: `upstream/sub2api/frontend/src/features/ai-tools/__tests__/model.spec.ts`
- Modify: `upstream/sub2api/frontend/src/features/ai-tools/model.ts`

- [x] **Step 1: Write failing tests** for active-only candidate filtering, 50/30/20 weighted score, TTFT normalization, missing metrics, and deterministic ties.
- [x] **Step 2: Run the model test file** with `pnpm vitest run src/features/ai-tools/__tests__/model.spec.ts` and verify the new tests fail because the scoring API is absent.
- [x] **Step 3: Implement the smallest pure scoring API** in `model.ts`: derive valid metric scores, normalize TTFT per tool candidate set, calculate total score, filter active and complete candidates, and sort ties deterministically.
- [x] **Step 4: Re-run the model test file** and verify all tests pass.

### Task 2: Use 24h active candidates for Dashboard best route

**Files:**
- Modify: `upstream/sub2api/frontend/src/views/user/DashboardView.vue`
- Modify: `upstream/sub2api/frontend/src/views/user/__tests__/DashboardView.parity.spec.ts`

- [x] **Step 1: Add failing Dashboard tests** asserting the initial monitor request includes `24h`, an active unlinked route can be selected as best, and an inactive route cannot.
- [x] **Step 2: Run the focused Dashboard tests** and verify the new assertions fail against the current 1h/request-linked implementation.
- [x] **Step 3: Add a separate 24h snapshot state and request** for best-route scoring; build candidates from all active groups returned by the workspace API, while retaining existing linked-line rows and 1h health state.
- [x] **Step 4: Connect each tool card’s `best` field to the weighted scorer** and keep detail best badges on the same 24h weighted result even when the detail window changes.
- [x] **Step 5: Re-run the focused Dashboard tests** and verify they pass.

### Task 3: Full frontend verification

**Files:**
- No additional source files.

- [x] **Step 1: Run the AI tools model and Dashboard parity tests together.**
- [x] **Step 2: Run frontend typecheck.**
- [x] **Step 3: Inspect the diff and confirm no unrelated files changed.**
- [x] **Step 4: Commit the implementation with `feat: rank best routes by weighted 24h quality`.**

### 后端补充：所有 active 线路均进入统计

- [x] 在 `monitor_v4_snapshot_service_test.go` 新增用例：配置仅含分组 7，但 active 分组 7、8 均须进入每个快照投影，inactive 分组 9 排除。
- [x] 使用 `go test -tags unit ./internal/service -run '^TestMonitorV4RefreshIncludesAllActiveGroupsOutsideMonitorConfiguration$' -count=1` 确认旧逻辑失败：group IDs 仅为 [7]。
- [x] 移除 `monitor_v4.go` 快照生成中的监控配置过滤，保留 active 状态检查和用户读取权限。
- [x] 使用 `go test -tags unit ./internal/service -run '^TestMonitorV4' -count=1` 验证相关测试通过。

### 交付范围

代码保存在独立分支及 worktree；没有修改根 main，没有推送或部署。前端测试 52/52 通过，类型检查和后端 MonitorV4 测试通过。未进行浏览器视觉验收或线上验收。
