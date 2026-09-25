# Pelican source port verification

Source archive: `/Users/gongtengxinwen/Downloads/sub2api-2.8.11.zip`, archive comment/revision `3bf31dedc335318238fbf29e376e10f9329d3eb5`.
Baseline Sub: `50c1376e754bf81517b7c1e4fadd9073decf16c6`.

User final scope supersedes every earlier custom proposal: preserve source Pelican behavior; account-quality operations and notifications belong to the separate Codex2API port. No runtime browser renderer, custom screenshot design, extra judgment mode, record-only action or custom alert was added.

## Backend
- Ported archive Pelican request options, scheduled contracts, Cron validation, account/plan leases, parallel execution, completion validation, bounded capture/history and cleanup.
- Preserved existing ordinary Sub probe accounting entry point and all unrelated account execution paths.
- Ported source user showcase publication, independent snapshots, selected groups, count/age retention, lazy bodies, authenticated read endpoints and admin deletion.
- Ported settings defaults/read/update/audit/public enable flag; provider graph and generated composition wired.
- Shared source quality-ops fields/branches excluded because those features belong to Codex2API; Pelican functionality unchanged.
- New migrations use archive names 243/249, neither name existed in baseline.

## Verification
- Source tests first failed because Pelican contracts/methods were absent.
- `go test ./internal/service ./internal/repository -run 'TestPelican|TestIntelligence|TestLegacyCandy' -count=1` — passed.
- `go test ./internal/handler ./internal/handler/admin ./internal/server/routes -run 'TestPelican|TestSettingsPelican|TestProvideHandlers' -count=1` — passed.
- `DOCKER_HOST=unix:///Users/gongtengxinwen/.colima/default/docker.sock TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE=/var/run/docker.sock go test -tags integration ./internal/repository -run TestPelican -count=1 -v` — 5 tests passed against temporary PostgreSQL 18.1 + Redis 8.4; migrations applied successfully. Initial unconfigured testcontainers invocation failed to locate the Colima socket; explicit environment fixed it without product changes.
- `go build ./cmd/server` — passed.
- Optional `-tags unit` compilation hits existing unrelated service test errors (duplicate ptrFloat, outdated pricing signatures, missing context import, stale image cooldown API). The same `go test -tags unit ./internal/service -run ^TestPelican -count=1` against unchanged root main reproduced the identical errors; these files were not changed for the port.

## Frontend and final review
- Integrated frontend commit `45562a5c245331d94d8610fdaaa1f5435d0d4d67` as `21fbbea6d7`; source UI report is `2026-09-25-pelican-ui-port.md`.
- 104 distinct relevant UI assertions passed (11 existing skipped); both Vue typecheck modes and Vite production build passed. Mocked desktop/mobile light/dark gallery + parallel manual output browser smoke passed, no real API calls.
- Independent backend review found no port defects. Integrated review found the source manual SSE reader bypassed this project's namespaced authentication storage. Fixed by native `authStorageGet`; three regression cases covered test_station_, admin_lab_, and no unprefixed fallback. RED 3 failures then GREEN: IQ 9 + authStorage 1 passed; Vue typecheck passed. Fix source commit `c6643bd9f474bc10486538843479ef916723607f`.
- Scoped independent re-review of `26f478faf2` confirmed the authentication finding addressed with no new issues.
- The auth adaptation means 12 (not original 13) dedicated frontend files remain byte-identical to archive. No user-requested source behavior changed.
- Frontend dependencies and backend test evidence are reused across clean cherry-picks; no code conflicts arose.

No real model requests, SMTP sends, deployments, pushes or main-branch changes were performed.
