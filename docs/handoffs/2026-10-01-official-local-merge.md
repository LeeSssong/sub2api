# October 1 official update — local only

- Branch: `codex/official-october01`; local base / origin/main: `a7884896e2540394e897b9d13394f4520dcb1fec`.
- Source: https://github.com/ranxi2001/sub2api production, verified at `a20c0334a6bccc6e7d082bfba9a4f1cb075ef50c` (2.9.6).
- Previous official baseline: `7124114c22c7cb786a62d5e3ee64713ca87ebdfc`.
- Applied the official binary diff with a three-way merge under `upstream/sub2api`; conflicting hunks favor official. Ten conflicted files resolved. Retained non-conflicting local code.
- Repaired semantic merge leftovers: restored official Grok media fallback and OpenCode Go settings support, removed duplicate Icon import, aligned OAuth configuration save and status-priority tests with official behavior. OAuth completion identity checks remain present and tested.
- No push, root-main integration, server access, deployment or migration execution. Migration 262 is included only as source.

## Verification

- Frontend `pnpm run build` passed, including locale checks and TypeScript.
- Selected frontend tests: 187 passed, 11 existing skips across AccountQualityView, SettingsView, PrioritySchedulingView, AccountStatusIndicator, FeatureSearch, ServerlessSettings and qualityRulePatch. AccountQualityView alone: 60 passed.
- Backend `go build -tags embed ./cmd/server` passed.
- Relevant Go tests selected with `-tags unit` and name pattern `Test.*(Quality|Priority|Fairness|ModelMapping|ModelAllowlist|SessionBinding|OAuthReauth|Reauth|Serverless|ExcelBPS|Scheduler|Runtime|CLIIdentity|GrokMedia)` passed after explicitly excluding the 12 baseline failures listed below. Packages reporting no selected tests are not counted as test coverage. No database integration or live upstream verification performed.
- Python reauth tests: 46 passed, 1 skipped. Used `TMPDIR=/private/tmp` to avoid macOS `/var` versus `/private/var` path assertion mismatch.
- Diff whitespace checks passed. These are local checks, not production acceptance.

The initial broader Go run failed. All following failures were reproduced on unchanged root main, then explicitly excluded from the final targeted run; they remain unresolved:

1. TestPriorityOAuthProfitUsesUserChargeAndTheoreticalCost
2. TestSchedulerSnapshot_DropsTemporarilyUnschedulableCachedAccounts
3. TestSchedulerSnapshotGetAccountRejectsExplicitBalanceVeto
4. TestSettingService_GetAllSettings_OpenAIAdvancedSchedulerEffectiveValuesUseConfig
5. TestUpdateExtraCodexDisplaySnapshotsAvoidSchedulerOutbox
6. TestSchedulerCacheSetAccountUnencodableStalePayloadDoesNotDeleteNewerState
7. TestSchedulerCacheSetAccountRejectsStaleUpdatedAtAndKeepsPayloadsConsistent
8. TestSchedulerCacheSnapshotWritePreservesNewerAccountPayload
9. TestSchedulerCacheDeleteFencesStaleAccountAndSnapshotWrites
10. TestSettingHandlerSchedulerBusinessPolicyCompilesOnServerAndRejectsInvalidPriority
11. TestSettingHandlerSchedulerExtraRetryCountRoundTripsWithLegacyPolicyFields
12. TestSettingHandlerSchedulerPresetsRoundTripAndRejectsReferencedPresetDeletion

## Quality operations findings

- Group templates create independent per-account plans. A definite incorrect answer affects only that plan's account; transport errors alone are inconclusive.
- `disable_scheduling` sets the account's global schedulable flag false, affecting that account across all groups. `remove_groups` removes only configured memberships for that account.
- Existing same-scope rules are skipped, not adopted by a new template. The screenshot shows all five matching accounts already have same-kind rules.
- The group card has edit/toggle controls and coverage count, but no per-account results expansion. Account cards retain history and the operations table retains per-account rounds. No UI changes made for this diagnostic request.
