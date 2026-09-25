# Native Billing Restore Implementation Plan

> **For agentic workers:** Use executing-plans inline. Do not spawn agents without authorization. Checkboxes distinguish verification from implementation; incomplete steps are not completion claims.

**Goal:** Restore selected pricing and charging behavior to a fixed upstream Sub2API reference while retaining wallet classification/ledger compatibility and unrelated product behavior.

**Architecture:** Compare functions and execution paths, not whole commits or files. Keep native balance deduction as authority, retain wallet projection in its existing transaction, and preserve historical schema/data. Disconnect custom automatic upstream-cost registration without deleting historical evidence.

**Tech Stack:** Go, PostgreSQL, Vue/TypeScript.

**Spec:** User-approved design in this task: restore pricing and deduction behavior selectively; retain wallet classification and ledger; remove specified upstream fee-query customization; preserve unrelated features, configuration and history. No deployment authorized.

## Global Constraints

- Local base: origin/main `50c1376e754bf81517b7c1e4fadd9073decf16c6`.
- Official comparison reference: Wei-Shaw/sub2api `a3eb7ef302961cba716dc78b39b93b60c467db0e`.
- Worktree: `/Users/gongtengxinwen/.codex/worktrees/native-billing-restore/sub2api搭建`.
- Branch: `codex/native-billing-restore`.
- No edits to root main, queue, global progress, production, historical balances, existing pricing configuration or historical usage rows.
- No migrations, history deletion, retroactive billing, bulk file replacement or new tool-fee feature.
- Preserve `projectUsageBillingWallet`, native balance transaction, recharge/redemption/refund dependencies and existing unrelated retry/transport functionality.

## Observed Difference Inventory

Paths below are relative to `upstream/sub2api/backend/`.

| Area | Official behavior | Local difference | Disposition |
|---|---|---|---|
| `internal/service/account_stats_pricing.go` | Resolve stats price, apply account multiplier | Image tier cost may bypass account multiplier; final `AccountCost` snapshot added | Restore pricing semantics; retain snapshot only as compatibility output of native formula. Verify existing image rules because their meaning changes; do not silently migrate configuration. |
| `internal/service/gateway_usage_billing.go` account quota | `Cost.TotalCost * AccountRateMultiplier` | `accountCostForBilling` prefers overridden stats cost | Restore native account quota semantics; stats reporting must not change quota consumption. |
| `internal/service/billing_token_cost_request.go` | Unified/catalog token pricing | Legacy Gemini split-range rule runs before catalog fallback | Restore native pricing dispatch; retain handler API compatibility where required. |
| `internal/service/openai_gateway_usage.go` | Reasoning effort passed to unified pricing; native Free Fast handling | Several call sites omit reasoning effort; Free Fast error behavior differs | Selectively align pricing functions; do not overwrite gateway metadata or transport logic. |
| Gateway post-usage registration | Native usage persistence | Calls `UsageCostEvidenceRegisterer.RegisterOnce` after insert; also reaches NewAPI rate registration | Disconnect automatic calls from both gateways; preserve usage persistence/fallback and history. |
| `internal/repository/usage_billing_repo.go` | Native deduction transaction | Wallet projection + consumption ledger inside same transaction | Retain per approved scope; tests must show one deduction, synchronized classification, rollback and idempotency. |
| Logical request/attempt and incomplete usage handling | Different upstream interfaces | Existing retry/transport integration | Map callers before edits; do not silently remove safeguards or change retry behavior as a side effect. |
| Detail page | Native account-cost accounting | Local detail labels computed amount as actual upstream charge | Correct only source/labels tied to removed custom behavior; no redesign. |
| Probe-driven rate writes | Not part of native deduction | Separate background probe can write multiplier | Trace independently from post-usage registration before claiming all automatic writes removed. Preserve unrelated probe diagnostics. |

## Task 1: Establish Baseline and Bounded Scope

- [x] Fetch origin/main; confirm clean root and create isolated branch.
- [x] Download seven relevant official source files at pinned SHA into temporary reference directory; compare function-level diffs.
- [x] Compile service and repository production packages:

```sh
cd upstream/sub2api/backend
go build ./internal/service ./internal/repository
```

Result: PASS before implementation.

- [x] Verify repository billing/wallet tests:

```sh
go test -tags unit ./internal/repository -run 'Test(ProjectUsageBillingWallet|UsageBillingRepository)' -count=1
```

Result: PASS before implementation.

- [x] Attempt service baseline:

```sh
go test -tags unit ./internal/service -run 'Test(CalculateSearchCost|ProjectUsage|ApplyAccountStatsCost|GatewayServiceRecordUsage_RegistersEvidenceAfterInsert|OpenAIGatewayServiceRecordUsage_RegistersEvidenceAfterInsert)$' -count=1
```

Result: compile failure before implementation, including:
- duplicate `ptrFloat` in payment-config and account-monitor tests;
- missing `pricingAt` arguments in two account-stats-pricing tests;
- missing `context` import in chat-completions-forward tests;
- image-tool-cooldown tests reference nonexistent `SynthesizedFromModelText` and `shouldCoolOpenAIImagesToolForError`.

- [ ] Resolve baseline-test approach with user before production-code edits. Do not disable tests or implement unrelated image behavior merely to make the package compile.

## Task 2: Restore Native Pricing and Account-Quota Semantics

**Modify:** `internal/service/account_stats_pricing.go`, `gateway_usage_billing.go`, `openai_gateway_usage.go`, `billing_token_cost_request.go`; touch `billing_service.go` only to retire the legacy rule after all callers are accounted for.

**Tests:** `account_stats_pricing_test.go`, `gateway_service_subscription_billing_test.go`, `openai_gateway_record_usage_test.go`, existing token-pricing and Gemini tests.

- [ ] Add failing assertions for native account multiplier application and separation of account-stat price from account quota cost. Use the same cost inputs to compare official and restored behavior.
- [ ] Verify native Free Fast failure behavior and reasoning-effort propagation with directly relevant tests.
- [ ] Restore the corresponding function blocks only; preserve wallet interfaces, request metadata and unrelated behavior.
- [ ] Run the focused pricing/charging tests and compile production packages.
- [ ] Inspect the patch for accidental configuration, history, transport and wallet changes.

## Task 3: Remove Automatic Upstream Fee Queries

**Modify:** gateway registration call sites in `internal/service/openai_gateway_usage.go`, `gateway_usage_billing.go`, and only necessary wiring in `internal/handler/wire.go`.

**Tests:** existing evidence registrar stubs in `gateway_record_usage_test.go` and `openai_gateway_record_usage_test.go`.

- [ ] Change online gateway contract tests to expect zero registrar calls and a successfully persisted usage row, including API-key success, billing failure and simple-mode branches.
- [ ] Run tests red against current gateway behavior.
- [ ] Disconnect automatic registration while retaining native usage-log persistence and fallback behavior.
- [ ] Verify that no gateway-triggered NewAPI rate registration remains. Separately identify background rate-writes so scope is explicit.
- [ ] Keep historical evidence repositories/schema readable; perform no data deletion or rewriting.

## Task 4: Preserve Compatibility and Verify Delivery

- [ ] Retain wallet projection; rerun repository billing/wallet tests, including idempotency and rollback cases selected from actual test names.
- [ ] Correct affected cost labels without altering unrelated UI structure; run detail-page unit tests if changed.
- [ ] Run `go build ./internal/service ./internal/repository`, directly related service tests and `git diff --check`.
- [ ] Report restored behavior, preserved compatibility differences, verification results, residual native limitations and deployment status. Do not label partial work complete or claim deployment.

## Current Status

Analysis/setup only. Production source is unchanged. Baseline production compilation and repository tests pass; service-package test compilation is blocked by pre-existing test/source mismatches. No implementation, commit, push or deployment has occurred.

## Execution update after test-repair authorization

- User authorized minimal test repair. Fixed duplicate float helper naming, missing context import and two missing pricing timestamp arguments without production behavior changes.
- Unit-tagged service tests still do not compile: image-tool provenance and OAuth 429 cooldown tests reference missing production APIs. No tests were deleted, disabled or skipped; no unrelated cooldown feature was implemented. An attempted image-test compatibility rewrite was reverted in full.
- Default service build does compile. These tests include the OpenAI record-usage and account-quota paths used below; this does not establish unit-tagged service coverage.
- Red/green verified `TestOpenAIGatewayServiceRecordUsage_DoesNotQueryUpstreamAfterInsert`: failed before gateway change (registrar called once), passed after disconnecting registrar calls.
- Disconnected post-usage evidence registration in OpenAI and general gateway call sites. General gateway unit-tagged test expectations updated, execution still blocked by pre-existing package errors.
- Red/green verified `TestNativeAccountQuotaIgnoresStatsOverride`: all three cases failed against custom overrides and passed after restoring TotalCost * AccountRateMultiplier. Existing quota-command and notification tests updated to the native contract.
- Passed `go test ./internal/service -run 'Test(NativeAccountQuota|BuildUsageBillingCommand|NotifyAccountQuota|OpenAIGatewayServiceRecordUsage)' -count=1`.
- Passed `go build ./internal/service ./internal/repository` and `git diff --check`.
- Wallet production code and database schema are unchanged. No commit, push, deployment or historical data mutation.
- Still pending: remaining pricing differences (image stats, Gemini legacy and reasoning/Free Fast), independent background multiplier write scope, UI labels, broader related verification. This is partial implementation, not a completed restoration.

## Scope correction approved by user

Of the four previously listed follow-ups, only fee labels are authorized. Image cost semantics, Gemini legacy rules and independent background multiplier synchronization are NOT implementation tasks in this scope. Earlier inventory entries are observations, not permission to modify these areas. Preserve all prior branch changes without extending them in this label-only step.

Label-only change: keep existing amount bindings and calculations; rename displayed account cost and gross profit as calculated values, and identify the source field as upstream ledger type rather than evidence supporting the displayed calculation. Update both Chinese and English messages and regression assertions.

Label verification: regression assertion failed against old wording, then passed after the message-only update. Full UsageDetailDialog (24), usageDetail (13) and locale completeness (3) tests all passed: 40/40, no skipped tests in final run. `git diff --check` passed. No numeric bindings, layout, API calls or backend files changed in this label-only step. Browser-rendered visual inspection was not performed. No commit, push or deployment.
