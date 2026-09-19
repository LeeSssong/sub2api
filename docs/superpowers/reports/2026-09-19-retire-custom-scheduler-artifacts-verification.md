# Retire Custom Scheduler Artifacts Verification

Date: 2026-09-19

## Candidate

- Branch: `codex/retire-custom-scheduler-artifacts`
- Base: `origin/main@682f20c88ec9e64b75c1076b2564d85b2387b8a6`
- Feature commit: `68003cd5947a0f04b715a69c438d474213b49f7d`
- Feature tree: `889854b99dc1ac6f84c84644fe02e3fe4f66d322`
- Migration: `239_remove_custom_scheduler_artifacts.sql`
- Migration SHA-256: `55ccbef953da0bb197de9ab1d5bcb823c24678f0974518a45e4381c105b6558b`

## Delivered Behavior

- Deletes the seven retired custom scheduler setting rows and drops `openai_scheduler_logs` idempotently.
- Removes scheduler log persistence, cleanup lifecycle, repository, handlers, APIs, admin route, page, sidebar entry, client, and translations.
- Removes the OpenAI scheduler experience API and dashboard card.
- Removes retired scheduler fields from settings GET/PUT models, parsing, persistence, runtime reads, frontend API types, and settings UI.
- Preserves the native scheduler enablement, sticky/subscription switches, Top-K, and weight settings.
- Retired API and page routes now resolve as absent rather than returning retired payloads.

## Verification

Passed:

- `go test ./migrations -run TestRemoveCustomSchedulerArtifactsMigration -count=1`
- Focused backend DTO, retirement, and route contract tests.
- `go build ./cmd/server`
- Focused frontend router and sidebar tests: 4 tests passed.
- `pnpm run check:i18n`
- `pnpm run typecheck`
- `pnpm run build`
- `git diff --check`
- Runtime reference scan found no production references to the removed log/experience routes or symbols. Remaining matches are destructive migration evidence, historical migration coverage, and negative contract tests.

An exploratory broad package test run also reached unrelated pre-existing failures in gateway error-message expectations and account-monitor response assertions. Those files were not changed for this task; focused tests and both production builds pass.

## Release Mode

This is not eligible for lightweight or direct deployment. Migration 239 permanently deletes settings rows and drops a table, so release must use the database maintenance path:

1. Prepare and verify the exact committed artifact from clean `main` after integration.
2. Stop business writes and relevant workers.
3. Verify a restorable database backup.
4. Apply migration 239 and start one new application version.
5. Verify health, settings save behavior, native scheduler controls, retired endpoint 404s, and absence of the retired admin page.
6. Restore traffic only after verification succeeds.

Application-only rollback is not valid after migration. Recovery requires the verified database backup plus the previous application artifact/configuration. No merge, push, deployment, production write, or worktree cleanup was performed in this task.
