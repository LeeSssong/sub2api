# Pelican preview and success statistics implementation plan

**Goal:** Implement the two user-approved designs: complete proportional artwork previews and accurate rolling-24-hour scheduled drawing statistics.

**Approved spec:** Current task conversation: preview design approved, statistics design approved, implementation explicitly requested. Visual references: `pelican-preview-fit.html` and `pelican-success-statistics.html` in the task visualization directory.

**Baseline:** Refreshed `origin/main` at `50c1376e75`; feature worktree started there and fast-forwarded to existing local main `189d2efb8a` because the approved gallery exists only in those local dependency commits. Original main and remote remain unchanged. All task diffs are measured from `189d2efb8a`.

## Global constraints
- One writer per worktree; separate preview/backend worktrees, integration and statistics UI owned by root. No root-main, queue, ledger, production, or unrelated modifications.
- Local development and directly relevant tests only; no pushing, merging main, deploying, or server access is authorized in this implementation turn.
- Preserve original HTML/SVG, animation, CSP, `sandbox=allow-scripts`, no same-origin permission, no network. Never infer statistics from artwork counts.
- The current public scope is the active configured showcase groups. Counts must expose no account or plan identities.
- Count completed recorded scheduled drawing results, excluding candy, graded quality, manual and pending jobs. Each parallel output counts once; page totals deduplicate one execution shared by multiple groups.
- Record outcomes and execution-start group attribution independently of artwork/raw-result pruning. Capture group memberships when claiming execution and copy that snapshot to each parallel result. Collection continues when showcase display is disabled. No fabricated historical backfill.
- Zero results display `0 / 0` and `—`; failed statistics reads display unavailable, not zero. Coverage begins when the new collection is enabled; partial 24-hour coverage is explicit.

## Shared API contract
Extend `GET /pelican-showcase`, keep the existing route and fields.

```ts
interface PelicanShowcaseStats {
  success_count: number
  total_count: number
  success_rate: number | null // percentage 0..100, unrounded server value
}
interface PelicanShowcaseStatsWindow {
  from: string // inclusive completed-at boundary
  to: string // exclusive boundary
  coverage_started_at: string
  complete: boolean
}
// Add optional-compatible fields:
// view.stats: PelicanShowcaseStats | null (deduplicated across visible groups)
// view.stats_window: PelicanShowcaseStatsWindow | null
// groups[].stats: PelicanShowcaseStats | null
```

Stats failures leave gallery usable and return null stats. The frontend must tolerate absent fields during compatibility transitions. First-version period is fixed rolling 24 hours.

## Work packages

### A — complete preview renderer (isolated preview worker)
- [x] Write/run failing fit geometry, sandbox message validation and document instrumentation tests.
- [x] Add one shared preview component/helper with stable document viewport, sandbox-side content measurement, bounded source-checked postMessage updates, proportional centering, safe SVG sizing and lifecycle cleanup.
- [x] Replace card iframe with that renderer; keep 4:3 wrapper, lazy body loading and overlay click semantics.
- [x] Deliver a fit/100% preview dialog component or integration instructions. Root owns `PelicanShowcaseView.vue` and locales to avoid conflicts. Preview uses available remaining dialog height, not fixed 65vh; 100% permits intentional scrolling.
- [x] Run only directly relevant tests and targeted type/lint checks. Commit worker changes locally and report commands/results.

### B — durable statistics backend (isolated backend worker)
- [x] Write/run failing tests for eligibility, duplicate multi-group totals, terminal outcomes, rolling window and coverage, visibility and unavailable reads.
- [x] Add migration 255 and lightweight outcome persistence integrated atomically with result insertion; no HTML or sensitive identity in the public statistics data. Preserve counts across plan/result/gallery pruning.
- [x] Add independent 48-hour retention, bounded 1,000-row cleanup batches and stable collector-start coverage metadata.
- [x] Extend existing showcase service/repository and API using the exact shared contract. Collection is independent of showcase on/off state; reads filter by current visible groups.
- [x] Run focused service/handler tests and local PostgreSQL integration tests including migration, rollback, deletion survival and transactional failure. Source commit `a4bd9e4d2a`; integrated as `74f201faf0`.

### C — statistics UI and integration (root)
- [x] Write/run failing gallery tests for selected-group statistics, null/empty rates, incomplete coverage and error handling. Initial red 5 failures, then all 10 tests passed.
- [x] Add typed API fields, compact summary beneath tabs, group-header stats, explicit artwork counts and bilingual copy. Reuse existing theme, loading/refresh behavior and i18n.
- [x] Integrate worker commits after focused review; connect the shared preview dialog.
- [x] Run affected Vitest tests, typecheck, locale check, frontend production build and required backend integration checks. Reuse unchanged test evidence.
- [x] Inspect real Vue components in a local browser with synthetic fixtures: desktop, short viewport, mobile, portrait/landscape HTML/SVG, oversized document, delayed size changes, zoom and statistics filters/states.
- [x] Fix concrete failures, perform one focused review, record local commit(s), test evidence and remaining deployment requirement.

## Completion record

- Local implementation candidate: `273e9db3dfe8c99de2554877be24ed6fb53e879b`, tree `3c7b7d8c0759e0ba26c1ffd3ebc204bd71a20014`, branch `codex/pelican-preview-stats`. The following documentation-only commit records this evidence.
- Frontend: 56 distinct directly relevant tests verified: preview helper 25, preview component 5, integrated gallery 10, native dialog plus locale completeness 13, and existing HTML extraction 3. After the final clipping correction, component plus gallery tests were rerun: 15/15 pass; unchanged helper/dialog/locale/extraction evidence reused.
- Full `vue-tsc -b`, scoped ESLint, `git diff --check`, application production build (17.36s), and minified browser fixture build (4.95s) pass. Build outputs are local validation artifacts under `/tmp`, not deployed release artifacts.
- Backend: focused service/repository and handler suites pass. Real PostgreSQL 18.1 integration passes all 12 top-level `TestPelican*` tests, including seven new statistics tests and ten eligibility subcases. This covers atomic rollback, execution-start group attribution, terminal status filtering, exact rolling-window boundaries, duplicate-group totals, missing metadata, collector activation, cleanup backlog and counts surviving raw-result/plan/artwork deletion.
- Backend commands: `go test ./internal/service ./internal/repository -run 'TestPelican|TestScheduledSaveResultPublishes|TestIntelligenceQuestion|TestLegacyCandy' -count=1`; `go test -tags unit ./internal/handler -run '^TestPelicanShowcaseHandler' -count=1`; real-PG `go test -tags integration ./internal/repository -run '^TestPelican' -count=1 -v` using the local Colima Docker socket.
- Known baseline limitation: the broader service `-tags unit` suite cannot compile because of existing unrelated test issues (duplicate `ptrFloat`, stale cost-resolution function arguments, missing `context`, and outdated upstream-error fields). No unrelated fixes or full-regression expansion were made.
- Browser verification used real Vue components and synthetic data, including the production-minified measurement script: 1200×1600 HTML, landscape SVG, portrait SVG, viewBox-only SVG, delayed content and a responsive 1600px document. Verified desktop 1440×960 / 1280×720, phone 390×844, short window 960×540 and light/dark themes.
- All six fitted artworks preserve their top-left paint origin and stay within their preview bounds. The modal stays inside the viewport without body/document overflow. Explicit 100% scrolls only the intended canvas, retains the document across mode changes, and opening another preview resets to Fit.
- Responsive artwork retains native `1024×768` iframe dimensions and `matchMedia('(min-width:1200px)') === false`; CSS media styling remains unchanged. The renderer also preserves author root zoom.
- Statistics browser checks: selected group changes the server-supplied summary; `0 / 0` shows a dash rate, unavailable data shows dashes while artwork remains visible, and partial coverage explicitly shows collection start. No browser runtime errors observed.
- One focused review found shrinking negative-coordinate offsets could move stationary content. Fixed with monotonic origin offsets and a regression. Browser QA additionally fixed entrance-transform sizing, media-query changes from iframe growth, and hidden wrappers auto-scrolling the rendered canvas. The final renderer uses a fixed native viewport, internal document zoom, outer fit/actual scaling, and non-scrollable clipping.
- Work remains on the isolated feature branch. Main, remote and servers were not changed; no production deployment or test-station synchronization was performed. The local preview is clearly labeled synthetic sample data.
- Migration `255_pelican_drawing_statistics.sql` creates independent outcome and coverage storage. A future authorized deployment must follow the project database-migration release policy. Collection starts when the new collector runs; no historical backfill is fabricated.
