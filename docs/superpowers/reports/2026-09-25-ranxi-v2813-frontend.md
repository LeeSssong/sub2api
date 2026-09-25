# ranxi v2.8.13 frontend fusion report

Date: 2026-09-25. Status: local frontend candidate, with explicitly confirmed pre-existing account-creation test failures. No push, deployment, main change, real model request, or notification.

## Sources and isolation

- Worktree: `/Users/gongtengxinwen/.codex/worktrees/ranxi-v2813-frontend/sub2api搭建`.
- Branch: `codex/ranxi-v2813-frontend`, starting at `babb83a9ac` (approved fusion preparation and existing Pelican changes).
- Three-way official baseline: `fd80b08c90`; ranxi source: `6b0c0ddbd1649caad5d980e92368059b1a5d1158`.
- Each source-inventory frontend path was compared with both baseline and current checkout. Existing-only paths were retained. No wholesale replacement of overlapping files.
- Dependencies cloned with APFS copy-on-write from existing main frontend node_modules into this isolated checkout (Pelican worktrees had no node_modules). No package manifests or lockfiles changed. Main dependencies were not modified.

## Delivered frontend behavior

- New admin pages/routes: account quality operations, account alert operations, credential/token guard, Codex harvest flow, errors-only request capture.
- Existing admin settings include BPS image relay, ticket harvest scope/strategy/proxy controls, Mihomo controls, retained Pelican showcase settings and Xingqiao scheduler/turn-state/billing/probe settings.
- Existing account editor and bulk editor gain account group-rate multiplier, Copilot SDK, per-group model restrictions and source diagnostics while retaining native cost/probe/model-detection fields.
- Group editor gains streaming-only and denied-user-model controls. Announcements retain native editor plus source targeting/user selection additions.
- Admin usage gains timing drilldown and TPS/latency health. Existing detail action, cost/profit evidence, columns, and hidden-by-default reasoning preference are retained.
- Pelican menu remains controlled by public `pelican_showcase_enabled`; backend integration explicitly enables the new deployment default. Empty gallery behavior and explicit administrator disable remain as previously tested.

## Ordinary-user preservation and conflicts

- No changes to `src/views/user/*`, `src/views/KeyUsageView.vue`, application shell template/style, global stylesheet, or existing branding.
- Source changes to KeyUsageView and its coupled test were deliberately omitted to preserve the requested ordinary-user experience.
- Ordinary-user UsageView reuses admin UsageTable: new TPS and timing UI now require the admin-only `enableTimingDetails` prop, preserving user two-row latency layout and original gradient. Regression test verifies this.
- GroupSelector keeps its original grid layout; only optional label capability is added. BaseDialog retains center placement default and fullscreen support, adding opt-in right drawer and preventing accidental close when text selection ends outside a dialog.
- Sidebar retains native account monitor, business operations, performance monitor, user menu order and route redirects. New admin groups are added alongside them.
- Reconciled duplicate Pelican imports/types/routes caused by prior integration. Scheduled-test panel retains native cron defaults and excludes source quality-only plans from the ordinary Pelican list.
- `IQTestModal` and its namespace tests retain existing `authStorageGet`. Newly imported manual-harvest SSE was corrected from raw localStorage to `authStorageGet` and tested against cross-environment token leakage.
- Source advertises a plugin-management route/menu whose `PluginsView.vue` does not exist in the pinned source; that broken placeholder route/menu is not introduced. Actual harvest/quality/token guard/capture features are available.
- Both locale dictionaries preserve all native cost/profit/monitor keys and add source translations; completeness check passes.

## Token guard contract coordinated with backend parent

- Read responses must mask `probe_headers` values, `relogin_headers` values, `bark_key`, and each `relogin_accounts[].password` / `.mfa_secret` with `********`.
- Save keeps unchanged mask values server-side (accounts matched by normalized email; headers by name). Explicitly removing a header/account removes it; blank Bark key clears it. Mask for a nonexistent secret must not become a real credential.
- Probe/relogin endpoint defaults remain blank; `enabled`, `auto_relogin`, `restore_schedulable` default false server-side. UI displays saved values rather than inventing endpoints.
- UI explains masks and trusted endpoint credential disclosure. Bark input is a password field. Browser tests verify mask roundtrip, password replacement without losing masked MFA, and no automatic run/relogin while loading/saving unrelated settings.
- Backend enforcement, actual provider traffic, credential persistence, and redaction of events remain backend integration responsibilities; no real credential/provider exercise was performed.

## Validation

1. `pnpm typecheck`: passed (vue-tsc --noEmit).
2. Main affected-suite sweep: **42 suites passed; 461 tests passed, 11 inherited skips**. Covers imported API/store/components/views, errors-only captures, permissions, quality ops, harvest, Mihomo, settings, Pelican, locales and security adaptations. Account creation baseline failure suite is separately identified below, not silently counted as passing.
3. Focused Copilot + existing storefront navigation: **2 suites passed, 4 tests passed** (40 nonmatching account tests intentionally filtered).
4. After final user-layout preservation adjustment: **3 suites passed, 40 tests passed** (UsageTable 33, integration reasoning effort 4, latency helper 3). Includes the added ordinary-user layout regression.
5. `pnpm build`: locale completeness, vue-tsc build and Vite production build; **passed**, 1198 modules transformed, final Vite build completed in 14.94s.
6. `git diff --check`: passed. No conflict markers remain.

Initial run identified an existing missing `})` in UsageTable.spec.ts at the cost snapshot case; repaired that test syntax to enable both existing cost assertions and added TPS tests. Existing SettingsView test mocks omitted the native turn-state settings/status API and caused a false generic error; added those mocks. Source reasoning-column tests were adapted to native opt-in display rather than changing native visibility.

### Confirmed baseline failures (not introduced by this fusion)

Ran original HEAD versions of both CreateAccountModal.vue and its spec in this isolated checkout, restoring fusion files after the run. Result: **9 failed, 31 passed**, matching the nine failures in the imported suite; new Copilot opt-in test passes separately. These concern native expectations absent from the pre-fusion implementation:

- persist upstream model metadata after preview/create;
- preview current concrete model mapping;
- formal capability sync after create;
- incomplete metadata warning;
- OpenCode Zen and GO endpoint/protocol defaults (two cases);
- adaptive Kimi, Kimi Coding Plan and MiniMax Responses endpoints (three cases).

No claim that the entire frontend suite is green. These independent baseline failures are left unchanged and reported to the parent for final candidate status.

### Warnings and unverified items

- pnpm v11 warns existing package.json overrides format is deprecated; lockfile unchanged.
- Browserslist data age, Node localStorage/deprecation messages, existing mocked router-link warnings, and large bundle chunk warnings are nonfatal.
- No live browser/backend acceptance, provider calls, streaming/WS longevity, deployment, server feature defaults or account mutation was exercised here.
- Existing production/server state not queried.

## Source-path coverage

Paths below are relative to `upstream/sub2api/`. “conflict” means explicitly reconciled as described above; it is not an unresolved conflict. Initial result records distinguish direct imports, clean three-way merge, prior integration, and deliberate user-view preservation.

| Source path | Initial three-way result |
|---|---|
| `frontend/src/__tests__/integration/usage-reasoning-effort.spec.ts` | imported |
| `frontend/src/api/admin/__tests__/requestCaptures.spec.ts` | imported |
| `frontend/src/api/admin/accountOps.ts` | imported |
| `frontend/src/api/admin/accountQuality.ts` | imported |
| `frontend/src/api/admin/accountTokenGuard.ts` | imported |
| `frontend/src/api/admin/accounts.ts` | merged |
| `frontend/src/api/admin/codexHarvest.ts` | imported |
| `frontend/src/api/admin/groups.ts` | imported |
| `frontend/src/api/admin/requestCaptures.ts` | imported |
| `frontend/src/api/admin/scheduledTests.ts` | already integrated |
| `frontend/src/api/admin/settings.ts` | conflict |
| `frontend/src/api/admin/usageTiming.ts` | imported |
| `frontend/src/api/pelicanShowcase.ts` | already integrated |
| `frontend/src/components/account/AccountGroupModelLimits.vue` | imported |
| `frontend/src/components/account/AccountUsageCell.vue` | imported |
| `frontend/src/components/account/BulkEditAccountModal.vue` | imported |
| `frontend/src/components/account/CreateAccountModal.vue` | merged |
| `frontend/src/components/account/EditAccountModal.vue` | conflict |
| `frontend/src/components/account/__tests__/AccountGroupModelLimits.spec.ts` | imported |
| `frontend/src/components/account/__tests__/AccountUsageCell.spec.ts` | imported |
| `frontend/src/components/account/__tests__/BulkEditAccountModal.spec.ts` | imported |
| `frontend/src/components/account/__tests__/CreateAccountModal.spec.ts` | merged |
| `frontend/src/components/account/__tests__/EditAccountModal.groupModelLimits.spec.ts` | imported |
| `frontend/src/components/account/__tests__/EditAccountModal.spec.ts` | merged |
| `frontend/src/components/account/__tests__/groupAllowedModels.spec.ts` | imported |
| `frontend/src/components/account/groupAllowedModels.ts` | imported |
| `frontend/src/components/admin/HarvestControlsPanel.vue` | imported |
| `frontend/src/components/admin/HarvestManualConsole.vue` | imported |
| `frontend/src/components/admin/HarvestNodeRecords.vue` | imported |
| `frontend/src/components/admin/__tests__/HarvestManualConsole.spec.ts` | imported |
| `frontend/src/components/admin/account/AccountActionMenu.vue` | already integrated |
| `frontend/src/components/admin/account/IQTestModal.vue` | conflict |
| `frontend/src/components/admin/account/PelicanRecordsDashboard.vue` | already integrated |
| `frontend/src/components/admin/account/PelicanTestFields.vue` | already integrated |
| `frontend/src/components/admin/account/ScheduledTestsPanel.vue` | conflict |
| `frontend/src/components/admin/account/__tests__/AccountActionMenu.position.spec.ts` | conflict |
| `frontend/src/components/admin/account/__tests__/IQTestModal.spec.ts` | conflict |
| `frontend/src/components/admin/account/__tests__/PelicanRecordsDashboard.spec.ts` | already integrated |
| `frontend/src/components/admin/account/__tests__/ScheduledTestsPanel.pelican.spec.ts` | conflict |
| `frontend/src/components/admin/announcements/AnnouncementTargetingEditor.vue` | imported |
| `frontend/src/components/admin/announcements/AnnouncementUserPicker.vue` | imported |
| `frontend/src/components/admin/announcements/__tests__/AnnouncementTargetingEditor.spec.ts` | imported |
| `frontend/src/components/admin/announcements/__tests__/AnnouncementUserPicker.spec.ts` | imported |
| `frontend/src/components/admin/group/GroupUserDeniedModelsModal.vue` | imported |
| `frontend/src/components/admin/group/ModelTagInput.vue` | imported |
| `frontend/src/components/admin/group/__tests__/GroupUserDeniedModelsModal.spec.ts` | imported |
| `frontend/src/components/admin/group/__tests__/ModelTagInput.spec.ts` | imported |
| `frontend/src/components/admin/operations/SmartOpsNav.vue` | imported |
| `frontend/src/components/admin/usage/UsageTable.vue` | merged |
| `frontend/src/components/admin/usage/UsageTimingDialog.vue` | imported |
| `frontend/src/components/admin/usage/__tests__/UsageTable.spec.ts` | merged |
| `frontend/src/components/admin/usage/__tests__/UsageTimingDialog.spec.ts` | imported |
| `frontend/src/components/common/BaseDialog.vue` | conflict |
| `frontend/src/components/common/GroupSelector.vue` | imported |
| `frontend/src/components/common/__tests__/BaseDialog.spec.ts` | imported |
| `frontend/src/components/layout/AppSidebar.vue` | conflict |
| `frontend/src/components/user/pelican/PelicanShowcaseCard.vue` | already integrated |
| `frontend/src/components/user/pelican/pelicanShowcaseFormat.ts` | already integrated |
| `frontend/src/i18n/locales/en/accountOps.ts` | imported |
| `frontend/src/i18n/locales/en/admin/accounts.ts` | merged |
| `frontend/src/i18n/locales/en/admin/harvestFlow.ts` | imported |
| `frontend/src/i18n/locales/en/admin/index.ts` | conflict |
| `frontend/src/i18n/locales/en/admin/overview.ts` | merged |
| `frontend/src/i18n/locales/en/admin/requestCapture.ts` | imported |
| `frontend/src/i18n/locales/en/admin/resources.ts` | imported |
| `frontend/src/i18n/locales/en/admin/settings.ts` | merged |
| `frontend/src/i18n/locales/en/common.ts` | conflict |
| `frontend/src/i18n/locales/en/dashboard.ts` | conflict |
| `frontend/src/i18n/locales/en/index.ts` | imported |
| `frontend/src/i18n/locales/en/qualityOps.ts` | imported |
| `frontend/src/i18n/locales/en/requestTiming.ts` | imported |
| `frontend/src/i18n/locales/en/tokenGuard.ts` | imported |
| `frontend/src/i18n/locales/zh/accountOps.ts` | imported |
| `frontend/src/i18n/locales/zh/admin/accounts.ts` | merged |
| `frontend/src/i18n/locales/zh/admin/harvestFlow.ts` | imported |
| `frontend/src/i18n/locales/zh/admin/index.ts` | conflict |
| `frontend/src/i18n/locales/zh/admin/overview.ts` | merged |
| `frontend/src/i18n/locales/zh/admin/requestCapture.ts` | imported |
| `frontend/src/i18n/locales/zh/admin/resources.ts` | imported |
| `frontend/src/i18n/locales/zh/admin/settings.ts` | merged |
| `frontend/src/i18n/locales/zh/common.ts` | conflict |
| `frontend/src/i18n/locales/zh/dashboard.ts` | conflict |
| `frontend/src/i18n/locales/zh/index.ts` | imported |
| `frontend/src/i18n/locales/zh/qualityOps.ts` | imported |
| `frontend/src/i18n/locales/zh/requestTiming.ts` | imported |
| `frontend/src/i18n/locales/zh/tokenGuard.ts` | imported |
| `frontend/src/router/index.ts` | conflict |
| `frontend/src/router/meta.d.ts` | imported |
| `frontend/src/stores/__tests__/accountQuality.spec.ts` | imported |
| `frontend/src/stores/__tests__/adminSettings.retry.spec.ts` | imported |
| `frontend/src/stores/accountOps.ts` | imported |
| `frontend/src/stores/accountQuality.ts` | imported |
| `frontend/src/stores/adminSettings.ts` | imported |
| `frontend/src/style.css` | already integrated |
| `frontend/src/types/index.ts` | conflict |
| `frontend/src/utils/__tests__/announcementTargeting.spec.ts` | imported |
| `frontend/src/utils/__tests__/harvestAvailability.spec.ts` | imported |
| `frontend/src/utils/__tests__/latencyHealth.spec.ts` | imported |
| `frontend/src/utils/__tests__/pelicanHtml.spec.ts` | already integrated |
| `frontend/src/utils/__tests__/requestTimingHealth.spec.ts` | imported |
| `frontend/src/utils/__tests__/usageTps.spec.ts` | imported |
| `frontend/src/utils/announcementTargeting.ts` | imported |
| `frontend/src/utils/featureFlags.ts` | merged |
| `frontend/src/utils/harvestAvailability.ts` | imported |
| `frontend/src/utils/intelligenceTest.ts` | already integrated |
| `frontend/src/utils/latencyHealth.ts` | imported |
| `frontend/src/utils/pelicanHtml.ts` | already integrated |
| `frontend/src/utils/requestTimingHealth.ts` | imported |
| `frontend/src/utils/usageTps.ts` | imported |
| `frontend/src/views/KeyUsageView.vue` | preserved ordinary-user view |
| `frontend/src/views/__tests__/KeyUsageView.spec.ts` | preserved ordinary-user view |
| `frontend/src/views/admin/AccountOpsView.vue` | imported |
| `frontend/src/views/admin/AccountQualityView.vue` | imported |
| `frontend/src/views/admin/AccountsView.vue` | already integrated |
| `frontend/src/views/admin/AnnouncementsView.vue` | imported |
| `frontend/src/views/admin/GroupsView.vue` | conflict |
| `frontend/src/views/admin/HarvestFlowView.vue` | imported |
| `frontend/src/views/admin/ProxiesView.vue` | imported |
| `frontend/src/views/admin/RequestCaptureView.vue` | imported |
| `frontend/src/views/admin/SettingsView.vue` | conflict |
| `frontend/src/views/admin/UsageView.vue` | conflict |
| `frontend/src/views/admin/__tests__/AccountOpsView.spec.ts` | imported |
| `frontend/src/views/admin/__tests__/AccountQualityView.spec.ts` | imported |
| `frontend/src/views/admin/__tests__/GroupsView.streamOnly.spec.ts` | imported |
| `frontend/src/views/admin/__tests__/HarvestControls.spec.ts` | imported |
| `frontend/src/views/admin/__tests__/HarvestFlowView.spec.ts` | imported |
| `frontend/src/views/admin/__tests__/RequestCaptureView.spec.ts` | imported |
| `frontend/src/views/admin/__tests__/SettingsView.spec.ts` | conflict |
| `frontend/src/views/admin/ops/TokenGuardView.vue` | imported |
| `frontend/src/views/admin/settings/MihomoCountryFilter.spec.ts` | imported |
| `frontend/src/views/admin/settings/MihomoCountryFilter.vue` | imported |
| `frontend/src/views/admin/settings/MihomoSettings.spec.ts` | imported |
| `frontend/src/views/admin/settings/MihomoSettings.vue` | imported |
| `frontend/src/views/admin/settings/PelicanShowcaseSettings.vue` | already integrated |
| `frontend/src/views/admin/settings/mihomoCountry.ts` | imported |
| `frontend/src/views/admin/settings/pelicanShowcase.ts` | already integrated |
| `frontend/src/views/user/PelicanShowcaseView.vue` | already integrated |
| `frontend/src/views/user/__tests__/PelicanShowcaseView.spec.ts` | already integrated |
