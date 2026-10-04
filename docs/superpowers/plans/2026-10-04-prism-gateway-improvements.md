# Prism reliability and gateway integration

**Goal:** Integrate PR #292 and useful Gateway scheduling/waiting behavior within existing Prism accounts, then present tested changes for deployment approval.

**Architecture:** Existing OpenAI OAuth account and per-model Prism switches own routing. Keep browser-authored start/poll, terminal/tool validation, unavailable usage, and persistent unknown-outcome journals. Extend the isolated adapter for bounded multi-account contexts and stage metrics; add waiting-only SSE comments in the Go Prism forwarding path.

**Tech stack:** Python 3.12, pinned Playwright/Chromium, Go/Gin, existing Redis account scheduler.

**Source/design:** User-approved conversation design, PR https://github.com/ranxi2001/sub2api/pull/292 head `36805e35a7983471bcb2fd4f2e5efead477269a9`, Gateway `2df3e7314273b559f9aa732dfceef8331ccc777e`.

## Constraints

- Only enabled Prism OAuth accounts and selected Prism models use new behavior; other account/model routes unchanged.
- Preserve current four model IDs; only 6.1 Sol has the client tool bridge.
- Never retry unknown starts or switch protocols/models after Prism failure.
- No new credential ledger, billing estimates, hosted tools, or artificial token deltas.
- Preserve hard resource bounds. Do not enable higher production concurrency automatically.
- Keep this work on its branch/worktree; no merge, push, deployment, or live credentials before user approval.

## Tasks

- [x] Import PR #292 tests, observe failures on original baseline, port net diff under upstream/sub2api while preserving login checks; run adapter unit tests.
- [x] Add bounded per-account context configuration, account-scoped load/peak/elapsed metrics, and safe cooldown handling; test multi-account concurrent isolation and credential rotation. Reuse native load scheduler rather than add duplicate routing state.
- [x] Add Prism-only waiting heartbeat before terminal response; test bytes arriving before completion, error after heartbeat, and non-Prism route isolation. Existing Python cancellation/journal tests retained.
- [x] Run pinned Python suite, real Chromium/mock upstream rounds and reconnect fixture, relevant Go tests including race; inspect diff and independent review.
- [x] Save implementation/validation/result report in this branch with provenance and practical limits; deliver deployment review summary.
