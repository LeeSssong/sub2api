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
- [ ] Write/run failing fit geometry, sandbox message validation and document instrumentation tests.
- [ ] Add one shared preview component/helper with stable document viewport, sandbox-side content measurement, bounded source-checked postMessage updates, proportional centering, safe SVG sizing and lifecycle cleanup.
- [ ] Replace card iframe with that renderer; keep 4:3 wrapper, lazy body loading and overlay click semantics.
- [ ] Deliver a fit/100% preview dialog component or integration instructions. Root owns `PelicanShowcaseView.vue` and locales to avoid conflicts. Preview uses available remaining dialog height, not fixed 65vh; 100% permits intentional scrolling.
- [ ] Run only directly relevant tests and targeted type/lint checks. Commit worker changes locally and report commands/results.

### B — durable statistics backend (isolated backend worker)
- [x] Write/run failing tests for eligibility, duplicate multi-group totals, terminal outcomes, rolling window and coverage, visibility and unavailable reads.
- [x] Add migration 255 and lightweight outcome persistence integrated atomically with result insertion; no HTML or sensitive identity in the public statistics data. Preserve counts across plan/result/gallery pruning.
- [x] Add independent 48-hour retention, bounded 1,000-row cleanup batches and stable collector-start coverage metadata.
- [x] Extend existing showcase service/repository and API using the exact shared contract. Collection is independent of showcase on/off state; reads filter by current visible groups.
- [x] Run focused service/handler tests and local PostgreSQL integration tests including migration, rollback, deletion survival and transactional failure. Source commit `a4bd9e4d2a`; integrated as `74f201faf0`.

### C — statistics UI and integration (root)
- [x] Write/run failing gallery tests for selected-group statistics, null/empty rates, incomplete coverage and error handling. Initial red 5 failures, then all 10 tests passed.
- [x] Add typed API fields, compact summary beneath tabs, group-header stats, explicit artwork counts and bilingual copy. Reuse existing theme, loading/refresh behavior and i18n.
- [ ] Integrate worker commits after focused review; connect the shared preview dialog.
- [ ] Run affected Vitest tests, typecheck, locale check, frontend production build and required backend integration checks. Reuse unchanged test evidence.
- [ ] Inspect real Vue components in a local browser with synthetic fixtures: desktop, short viewport, mobile, portrait/landscape HTML/SVG, oversized document, delayed size changes, zoom and statistics filters/states.
- [ ] Fix concrete failures, perform one focused review, record local commit(s), test evidence and remaining deployment requirement.

## Completion record
- Backend: focused service/repository and handler suites pass. Real PostgreSQL 18.1 integration passes all 12 top-level `TestPelican*` tests, including seven new statistics tests and ten eligibility subcases (8.073s). Full service `-tags unit` is blocked by unrelated baseline test compilation errors; no unrelated repairs attempted.
- Frontend so far: integrated gallery 10/10; native BaseDialog and locale completeness 13/13; targeted lint, full `vue-tsc -b`, application production build and minified fixture build pass. Renderer worker initial package 34/34.
- Browser so far: five thumbnail cases fit, including 1200×1600 HTML, portrait SVG, viewBox-only SVG and delayed content. 1280×720 and 390×844 dialogs fit without body/document overflow. 100% intentionally scrolls; mode switch preserves the same document.
- Focused review confirmed a negative-to-positive animated element can shrink the document origin offset; renderer follow-up in progress. Browser verification also identified modal entrance transforms affecting measurements, and native iframe growth potentially changing media queries. Final verification follows these fixes.
- Implementation and directly relevant verification constitute candidate completion only. Production deployment has not been requested.
- The statistics migration is part of this feature; a future authorized deployment must follow the repository database migration release policy.
