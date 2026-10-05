# Task 1 integration candidate report

Status: DONE_WITH_CONCERNS (development candidate only; visual acceptance and added navigation approval pending). No shared-main push, deployment, SSH, or production data access by this implementer.

Sources: main d36f54bdd69c183b64b5ab2456687b0ec79ff764; test 96c08182779b12c5a39ffc0ffdda9ba89ad38bde. This is a real merge of those parents. Approved task spec/plan included. User later explicitly requested removal of scheduler logs page; specification/plan/ledger updated accordingly.

## Conflict and semantic decisions

All 37 conflicted paths were inspected and resolved per hunk, retaining nonconflicting three-way changes. No broad backend/frontend ours/theirs replacement. Policies:
- wire/router/http: compose main lifecycle/drain/serverless/gateway-role auth boundaries with test dependency readiness and DB injection; Feishu callbacks remain primary-only. Generated wire signature passes actual DB.
- Monitor repository: main real terminal deduplication, exclusions, unknown usage rejection and usage-only cache numerator/denominator retained; shared SQL used by aggregate and test timeline; test P50 fields/manual group scoping retained. Retired current_operational removed from SQL/projection/snapshot consistently, without modifying any applied migration. Probe success/cache totals must not inflate real-user cache rate. Manual candidate priority now uses native account priority, not removed custom group priority helper.
- Common dialogs: test branding/panelClass and topmost dialog ownership compose main drawer/fullscreen/focus/scroll lock and overlay drag protection. Select multiple/accessibility/keyboard logic retained with test brand tokens. Compact language controls retain test appearance; placement top remains compatible.
- Header/layout/sidebar: test UserHeader and existing navigation lists preserved, observer restricted navigation retained; main-only FeatureSearch/smart operations/capture/harvest entries not exposed without approval. All main routes/components remain. Three brand CSS files identical to test.
- Keys: test viewport sizing, shared create LineSelect dialog, inline line health and bulk edit retained. Main live concurrency/queue polling, native key limit editing, status-safe quota reset retained. Current badge retains test dimensions and only current value; full/limit/waiting/stale info in focusable title/accessibility text, avoiding additional row height. Creation uses backend default; concurrency remains editable. Unreachable alternate provider-create fieldset removed. Bulk tests adapted to actual test bulk-dialog events, not an obsolete main modal.
- Payments: test recharge work surface/limits/error retry kept, main bonus/discount calculations connected conditionally to AmountInput and summary; requested amount distinguished from credited total. Main recovery, WeChat flow, feature flags and renewal checkout retained. Default recharge surface keeps existing layout; subscription checkout accessible through existing ?tab=subscription renewal path. Added unconditional duplicate account card removed.
- Usage: test four default filters retained. Main request-type/compaction/billing-type contracts and logic retained with new controls hidden pending visual approval; CSV export captures one stable filter snapshot. Admin usage keeps test compact money format including main image cost logic.
- Ops errors: main observer owned-error API and correlated-data restrictions compose test credential redaction. Non-sensitive JSON diagnostic whitespace preserved; credential-bearing records redacted. Tests distinguish safe raw payload from credentials.
- Register/profile/tests/locales: main auth confirmation/domains/DingTalk retained, test title/balance unknown semantics retained. English/Chinese schema parity fixed (two main reasoning multiplier keys missing English). Provider count test includes main TypeSafe provider.
- SchedulerLogsView delete/modify resolved by explicit user removal. Old view, route, sidebar links and API absent; native scheduler unchanged. Existing immutable 240_remove_custom_scheduler_artifacts.sql still drops historical openai_scheduler_logs on future migration. This is data deletion, not code-only rollback; future release requires corresponding data backup/retention decision, no archival writer or fake compatibility introduced.
- Homepage main-only visual App/Hero/Canvas/site config restored to exact test versions; main public/docs new guide, screenshots/downloads retained as separate documentation functionality. HeroEndpoint visual addition removed. No dependency/security rollback.

Auto-merge regressions actually fixed: absent composite available-models bool argument (TypeSafe native listing included); removed scheduler priority helper reference; missing PAT import active_probe_enabled; duplicate Keys selection/control/import state; DB/router generated signature mismatch; duplicated ptr unit helper; locale schema mismatch; stale diagnostics mocks; cached redeem history hidden after fetch error. Baseline Go test failed on conflict markers; initial typecheck likewise failed on markers. Additional initial meaningful failures and their final results appear in evidence below.

## Main feature preservation matrix

| Main family | Retained route/API/behavior | Entry status | Evidence |
|---|---|---|---|
| Native scheduling/retry/drain/serverless | Main scheduler files unchanged; lifecycle/gateway routing retained | Existing routes, additions pending | Go server/lifecycle/routes build/tests |
| Account quality, auto config, token guards v1/v2, BPS, harvest, captures | Main components/types/API/routes retained | New admin navigation pending approval | Typecheck/build; API token-guard/request-capture tests |
| Observer ownership/permissions | Main middleware/router restricted account/usage/error contracts | Restricted role navigation retained | Observer API tests; Ops owned-detail test; handler/server tests |
| Key limits and waiting | Native concurrency API/polling, edit/save/reset retained | Existing edit reachable; compact badge title | Keys 62 tests: known/unknown/stale, limits, refresh cancellation |
| Group tools/available models | Test model/tool mappings adapted to main TypeSafe/composite listing and allowed groups | Existing test AI tools | Go model authorizer/mapping tests; real SQL mapping audit |
| Subscription checkout | Main plans, currencies, renewal, recovery and flag handling | Existing renewal ?tab=subscription; default recharge preserved | Payment running feature/renewal/recovery tests; 6 baseline skipped tests remain skipped |
| Advanced user usage filters | Main params/export semantics retained; control code hidden pending approval | New controls pending | Usage normal filters/errors/export tests; typecheck |
| Intelligence/storefront | Main intelligence routes/pages and configured custom menu behavior retained | Configured entries preserved; proposed user additions pending | Router tests/build |
| Payment bonuses | Main quote, discount, bonus/credited values, server config and provider logic | Conditional within existing work surface | Payment/AmountInput/provider/config tests |
| Homepage/docs | Exact test homepage visual sources; newer main docs guide retained | Existing homepage | Homepage 12 tests/typecheck |

## Verification evidence

Commands below executed locally. Logs copied into task directory; no production evidence or screenshots included.

1. `go test ./internal/repository -run 'Test.*(Monitor|GroupTool)' -count=1` — PASS (1.997s). Monitor projection/cache/snapshot/timeline/mapping unit contracts.
2. `go test -tags unit ./internal/service ./internal/handler/... ./internal/server/... ./migrations -run 'Test.*(MonitorV4|GroupTool|AvailableModels|PaymentConfig|Readiness|Router|GatewayRole|Lifecycle|Migration)' -count=1` — PASS across selected packages (service 21.502s, handler 4.238s, admin 4.797s, server 10.696s, routes 7.357s, migrations 9.702s). No-test package entries explicitly shown in log. Unit tags required by existing raw ChatCompletions test fixture; duplicate scoped ptr helper renamed. Not all unrelated account-monitor tests run: main itself contains outdated full-package handler assumptions about unavailable ranks; not claimed full regression.
3. `go build -o /tmp/integration-sub2api-server ./cmd/server` — PASS. Plain non-embed server entry build; production frontend embed source is internal/web/dist.
4. `DOCKER_HOST=unix:///Users/gongtengxinwen/.colima/build/docker.sock TESTCONTAINERS_RYUK_DISABLED=true go test -tags integration ./internal/repository -run '^TestMigrationsRunner_IsIdempotent_AndSchemaIsUpToDate$' -count=1` — PASS (6.234s). Actual ApplyMigrations fresh candidate schema and rerun/verification. Initial default Testcontainers failed `rootless Docker not found`; resolved with actual configured Docker host.
5. Disposable Docker PostgreSQL18 (`--network none`, no ports/host volumes), all 321 test-branch migrations applied to empty schema with immutable contents, schema_migrations populated, then 44 candidate/main missing SQL files applied in actual filename order — PASS. Both current_operational and old scheduler table absent. All candidate migrations inherited from each parent retain exact contents; full test upgrade included duplicate numeric filename prefixes, relying on actual lexicographic runner. Script is evidence fixture in task directory; empty synthetic data only, not real site schema dump or production data.
6. `MONITOR_SQL_IMAGE=postgres:18-alpine MONITOR_SQL_REPORT=/tmp/integration-monitor-postgres.json python3 backend/scripts/verify_prototype_monitor_postgres.py` (from repo root path) — PASS 30 actual SQL assertions. P50 all windows, real evidence exclusion/dedup, scoped manual probe/timeline, tool mapping versions/audit, snapshots and removed flag. Ephemeral container removed. Script now accepts image/report environment overrides without changing default.
7. `pnpm typecheck` — PASS, latest after visual-preserving adjustments. `pnpm build` — PASS latest 16.35s; i18n 3 tests passed, vue-tsc and Vite production bundle. Existing chunk-size warning only. Build writes backend/internal/web/dist (ignored), not prototype/native or mock assets.
8. Direct Vitest selection 41 suites — 40 suites /437 tests PASS, 6 existing baseline skipped subscription cases; one Groups mock typo fixed then isolated suite 1 test PASS. Later key/payment/usage adjustment rerun: Keys 62, Payment/Usage direct suites PASS; no new skips. Redeem overflow conditional and concurrency-response tests final 11 PASS. Additional account/API direct selection: 4 suites passed /82 tests originally, then corrected actual PAT regression, compact multiplier expectations, test tooltip cleanup, cached redemption rendering; account/Create 45 + billing 14 + Redeem11 =70 PASS. Exact logs preserve initial failures and final results. None labeled CI or live test.
9. Homepage `pnpm typecheck`, `pnpm exec vitest run src/App.test.tsx src/sections/HeroSection.test.tsx src/components/HeroSignalCanvas.test.tsx` — PASS 12 tests. Incidental pnpm lock removed (project uses existing package-lock).
10. `git diff --check` for integration edits and conflict-marker scan — PASS before staging/commit. Full cached check additionally reports trailing whitespace inherited unchanged in historical test evidence logs and prototype minified vendor; those provenance files deliberately not rewritten. All index conflicts resolved by staged edited/deleted files.

## Limits and pending work

- Candidate is not a release and not DONE. New admin navigation/additional usage controls/default subscribe switcher remain pending explicit visual approval. No irreversible action authorized or taken.
- Controller captured actual baseline screenshots externally after login; this implementer did not perform candidate screenshot/role/375px visual comparisons. No claim of visual acceptance, DESIGN metric <=1px compliance, or live frontend/backend compatibility. Normal-user screenshot still not available at report time. Existing admin-view-user baseline alignment issue intentionally untouched.
- Future migration execution must stop incompatible old API/worker before removed-column migration; retired logs deletion must be backed up. Code restore alone cannot restore database or new writes. Main/test data/credentials never copied.
- Prototype/mock/dist baseline evidence carried by original test branch remains historical tracked source; actual production entry frontend/index.html → src/main.ts → backend/internal/web/dist, Go embed all:dist. No build references prototype/mock runtime. No production secrets introduced or printed.
- Some existing main-only operational dialogs/options are only typechecked/built rather than exercised against real upstream services; retained unchanged. Public docs assets retained from main but not re-rendered since they were untouched.

## Conflicted paths

- `upstream/sub2api/backend/cmd/server/wire_gen.go`
- `upstream/sub2api/backend/internal/repository/account_monitor_repo.go`
- `upstream/sub2api/backend/internal/repository/account_monitor_repo_test.go`
- `upstream/sub2api/backend/internal/server/http.go`
- `upstream/sub2api/backend/internal/server/router.go`
- `upstream/sub2api/frontend/src/__tests__/integration/usage-reasoning-effort.spec.ts`
- `upstream/sub2api/frontend/src/components/account/CreateAccountModal.vue`
- `upstream/sub2api/frontend/src/components/account/UpstreamBillingRateCell.vue`
- `upstream/sub2api/frontend/src/components/admin/usage/UsageFilters.vue`
- `upstream/sub2api/frontend/src/components/admin/usage/UsageTable.vue`
- `upstream/sub2api/frontend/src/components/common/BaseDialog.vue`
- `upstream/sub2api/frontend/src/components/common/LocaleSwitcher.vue`
- `upstream/sub2api/frontend/src/components/common/Select.vue`
- `upstream/sub2api/frontend/src/components/layout/AppHeader.vue`
- `upstream/sub2api/frontend/src/components/layout/AppLayout.vue`
- `upstream/sub2api/frontend/src/components/layout/AppSidebar.vue`
- `upstream/sub2api/frontend/src/components/layout/__tests__/AppSidebar.spec.ts`
- `upstream/sub2api/frontend/src/components/payment/AmountInput.vue`
- `upstream/sub2api/frontend/src/components/user/profile/__tests__/ProfileInfoCard.spec.ts`
- `upstream/sub2api/frontend/src/features/monitor-v4/__tests__/HybridPerformanceGroupCard.spec.ts`
- `upstream/sub2api/frontend/src/features/monitor-v4/__tests__/HybridPerformanceView.spec.ts`
- `upstream/sub2api/frontend/src/i18n/locales/en/dashboard.ts`
- `upstream/sub2api/frontend/src/i18n/locales/en/misc.ts`
- `upstream/sub2api/frontend/src/i18n/locales/zh/dashboard.ts`
- `upstream/sub2api/frontend/src/i18n/locales/zh/misc.ts`
- `upstream/sub2api/frontend/src/views/admin/SchedulerLogsView.vue`
- `upstream/sub2api/frontend/src/views/admin/__tests__/ChannelMonitorView.grok.spec.ts`
- `upstream/sub2api/frontend/src/views/admin/__tests__/GroupsView.codexManifest.spec.ts`
- `upstream/sub2api/frontend/src/views/admin/ops/components/OpsErrorDetailModal.vue`
- `upstream/sub2api/frontend/src/views/admin/ops/components/__tests__/OpsErrorDetailModal.spec.ts`
- `upstream/sub2api/frontend/src/views/auth/__tests__/RegisterView.spec.ts`
- `upstream/sub2api/frontend/src/views/user/KeysView.vue`
- `upstream/sub2api/frontend/src/views/user/PaymentView.vue`
- `upstream/sub2api/frontend/src/views/user/RedeemView.vue`
- `upstream/sub2api/frontend/src/views/user/UsageView.vue`
- `upstream/sub2api/frontend/src/views/user/__tests__/CustomPageView.spec.ts`
- `upstream/sub2api/frontend/src/views/user/__tests__/KeysView.spec.ts`
