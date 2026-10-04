# Intelligence test implementation plan

Approved design: user confirmation in this chat on 2026-10-04. Implementation proceeds inline.

Goal: replace user Pelican gallery entry with an independent, custom-menu-mounted intelligence dashboard matching supplied screenshots, with group candy and randomized drawing tests configured by admin rules.
Architecture: extend native group plans/config JSON, retain legacy endpoints and snapshots, share native quality grading, expose account-free result DTOs. A rule fans out atomically into native per-group schedules. Keep history configuration immutable. No account-removal/BPS action from these observational tests.

- [x] 1. Backend rule config + atomic group-plan fanout; tests for validation, unchanged legacy drawing, randomized prompt snapshots, candy grading and error categories. Extend group runner using existing account runner and quality judge. Store result metrics in existing config JSON; no database schema changes.
- [x] 2. Read-only authenticated dashboard endpoints using native group records, 48 latest results per type, no account identities/raw upstream errors. Guard disabled showcase, inactive/deleted groups and record lookup. Test DTO classification and visibility.
- [x] 3. User Vue dashboard: dark screenshot palette (#1f1f1f/#282828/#2e2e2e; pass #00bb87, abnormal #ffbf00, incorrect #ff293f), two timelines and selected artwork, details and expand; truthful no-data/loading/error states. Native custom-menu internal route integration; legacy page redirect.
- [x] 4. Admin rule editor: candy defaults/expected answer/judge config, drawing template/actions/scenes, model/effort/schedule and multiple group selection. Reuse settings and existing rule execution/history. Tests for payload/editing and errors.
- [x] 5. Run directly affected Go/Vitest tests and frontend typecheck/build, inspect rendered UI and interactions. Final focused review. Record actual results and limitations here. No production access/deployment in this scope.

Validation commands: Go service/handler/repository packages filtered to Intelligence/PelicanGroup/QualityJudgment; frontend Vitest intelligence, routing/custom menu and existing affected suites; pnpm typecheck and build. Existing legacy samples must remain unchanged. New tests distinguish answer_mismatch from request/judge errors and prevent response/config mutation leaks.

Progress: baseline main 9000e6672b; independent worktree; no existing changes. Implementation authorized. Rules/records reuse config JSON rather than new schema: persisted type metadata is backwards compatible and new public DTO explicitly selects safe fields.


## Local verification — 2026-10-05

Candidate implemented in `codex/intelligence-test`; source changes remain uncommitted. No merge, push, server access or deployment performed.

Passed:
- `go test -tags=unit ./internal/service ./internal/handler/admin -run 'Test(Intelligence|PelicanGroupTest|QualityJudgment|SettingsIntelligence)' -count=1`.
- `go test -tags=unit ./internal/server/routes -run 'Test.*(Pelican|MonitorV2)' -count=1`.
- Real disposable PostgreSQL via Docker: `go test -tags=integration ./internal/repository -run '^TestIntelligence' -count=1 -timeout=180s`, covering atomic multi-group saves, rollback, conflicting rules, running leases, independent history windows, result metadata persistence and single-result visibility.
- Six targeted Vitest files: 32 tests passed (statistics, rule payloads, result sanitization, custom menu, administrator workflows, existing artwork preview).
- Frontend production build including locale completeness and vue-tsc; backend `go build ./cmd/server`; targeted new-code ESLint; `git diff --check`.
- Browser rendered actual Vue components against synthetic data/SVG fixture, tested wrong-answer/request-error detail dialogs, artwork enlargement, multi-group form save, desktop reference dimensions and 390px responsive width (no horizontal overflow).

Limitations: no live model-provider calls or authenticated deployment verification. Browser fixture is local and mocked, not production evidence. No automated pixel-diff equivalence claim. New intelligence rules must be created/enabled and the custom menu saved after a future deployment; legacy plans are not silently converted or publicly exposed through the new raw-detail endpoint. Existing published-gallery API remains available.

Known baseline issue: `AppSidebar.spec.ts` locale-switcher source assertion fails because main already lacks LocaleSwitcher/user-sidebar-locale markup. Confirmed against HEAD via git show; unrelated behavior left unchanged. Build emits existing dependency age/chunk size warnings.

Review: one read-only reviewer identified menu filtering, cross-rule overlap, legacy detail exposure and repeated full-dashboard reads; each addressed with regression tests. Manual multi-group launches retain native per-plan semantics; UI explicitly lists started and failed groups.

Preview fixture (ignored local QA file): frontend/scripts/fixtures/intelligence-verification.html, served at http://127.0.0.1:5178/scripts/fixtures/intelligence-verification.html. Screenshot in the chat visualization directory iq-preview/implemented-desktop.jpg. Dependency directory reuses installed packages through local links and is ignored.

## Local acceptance corrections — 2026-10-05

User feedback: remove duplicate compatibility-plan and user-display controls; user page must include the reference's surrounding page layout, overview, time range and fixed cells rather than stretching one record.

- Admin now shows one Create Rule entry, rule management and the shared test history; old group drawing APIs/history remain available for compatibility without a second UI configuration area.
- New authenticated dashboard is independent of the legacy gallery-enabled switch. Custom menus configure its navigation entry.
- Native site sidebar/header restored; added overview, logic pass rate, next scheduled run, 24h/3d controls, refresh stamp and five-state legend.
- 48/144 fixed half-hour cells; missing slots are neutral, active leases are blue. Only real completed samples contribute to statistics. Most recent sample per half-hour displayed when a rule runs more frequently.
- New rules retain 1024 records (old single drawing plans keep 100); reads cover 72 hours with up to 512/type to support the 3-day view. Existing dropped history cannot be recreated.
- Local observed failure is no_available_account: the user's fresh database has zero upstream accounts. No success data or artwork manufactured.
- Focused tests: 12 frontend tests, service Intelligence/PelicanGroupTest tests, vue-tsc, targeted ESLint and production frontend build passed. Local Linux embedded binary rebuilt for actual container acceptance. Database and user-created rule preserved.


## Approved production release — 2026-10-05

User authorized correcting the duplicate title, renaming it to 智商监测, merging root main, pushing origin/main and deploying production without downtime. The native header is the single page title (also visible on mobile); the content toolbar only contains range and refresh actions. Default menu labels, admin copy and rule defaults use 智商监测. No database migrations or production-data import are part of this release.
