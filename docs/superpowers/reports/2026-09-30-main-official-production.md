# 2026-09-30 main integration and production release

- Target: production sub2api-prod / 64.83.10.67. User authorized integration, official-first conflicts, push and no-downtime deployment.
- Result: green promoted, version 2.9.4; API, Go worker and dedicated reauthentication worker healthy. Final application release did not roll back.
- Official production snapshot: ranxi2001/sub2api@7124114c22c7cb786a62d5e3ee64713ca87ebdfc, fetched and remotely verified during this task. Baseline e0b227cc07dfb7094f1c01889c9fdb9f1d391374. Conflicting hunks favor official code.
- Integrated current optimizations: OAuth 5xx drain (de98f71801), image-edit MIME (60db834eca), OAuth import identity (d6e4eafcb6), reauth scheduling recovery (13706b4367). Original worktrees and uncommitted files remain preserved.
- Historical guard-native-backend, guard-native-frontend and integrate-september28 work was superseded by the September 29 official rollout and was not resurrected.

## Source and artifacts

| Item | Verified identity |
| --- | --- |
| Deployed clean, pushed main commit | 7cc75747c73b238e9ab8f63030785d8fad71539d |
| Deployed source tree | 3f0a00986e5855164532b0a0db6543eccee7523c |
| API and Go worker image ID | sha256:b3cf444bc658f6f192c499c7f05fb0cf463fd4311689e5b613576a3fb72abcea |
| Dedicated reauth image ID | sha256:21e60f3f811205046923586f9a313893d6dad83cb947f9d8252166dd397d9750 |
| Reauth source commit | 122615242675b8ce9b849bf917a55e2a93da7be0 |
| Reauth source tree | 84be62f9dfd7963ac280d56e9e96bef3e03ed90d |
| Reauth dependency lock SHA-256 | ecc7a2fa6510d58ccef5cc9331415d3b393290c1a22db09cedccef6dfcba8f8e |
| Reauth supervisor SHA-256 | 2ed331153c34755642dd4b400da8afc42df2e671688585e899a0d41e91b7af9a |
| Final migration-set SHA-256 | d5339ae8cc23d83fcb14727a248cd0e2e077d21741ed597f81edbec76bfafffe |

Reauth build reused unchanged: the later commit changes only Go role configuration and its test. The immutable worker image was built once from pushed main and its revision label verified. This documentation-only follow-up does not change the deployed code identity above.

## Verification and deployment

- Passed targeted Go tests across service, repository, admin handler, OpenAI helpers, config and startup, plus go build; frozen frontend install, typecheck and 79 selected frontend tests; 31 official Python worker tests. Full unrelated regression suites were not run.
- Passed real disposable PostgreSQL 18 tests for additive migrations 259–261, including bounded-lock rollback and legacy writes; reviewed controller/host transition tests passed. Online exception applies only to this requested no-downtime release.
- First candidate stopped before cutover: integration incorrectly mapped API process role to official gateway-only runtime, hiding panel routes. Blue continued serving. Completed additive migrations were retained. A test reproduced the mismatch; the fix passed config/server/startup tests. Explicit official gateway-only behavior remains; API serves panel routes without singleton jobs.
- Reused unchanged frontend, migration, billing, auth and Python evidence; rebuilt application because Go source changed. An intervening upload failed on SSH connection closure before building; retry succeeded. No check was bypassed.
- Final controller succeeded with downtime_required=false. Worker scheduled tasks drained before replacement. Green passed internal smoke, Caddy switched smoothly, public acceptance passed, then blue drained. Recorded drain: 297 seconds after public checks, force termination at the 300-second cutover deadline. One old connection was observed before expiry. Old image and configuration retained.
- Public health, readyz, authenticated version (2.9.4), quality templates and auto-config events returned HTTP 200. Runtime images and process roles match. Reauth worker is healthy, shares the current Go worker network namespace and passes non-claiming --check. No synthetic production login task was created.
- Public monitor: 194 samples from 02:35:09 to 02:56:23 Asia/Shanghai; 192 HTTP 200, two six-second timeouts at 02:35:57 and 02:52:30. These samples do not establish zero interruption. Both domains were healthy in final checks. Additional Python urllib probe received 403; standard curl probes returned 200.
- PostgreSQL, Redis, Caddy and model detector identities preserved. Test station not queried or synchronized. Top-K unchanged.

## Timing and recovery

- First application attempt: about 372 seconds including build/transfer and rejected candidate. Reauth image build: 54 seconds in parallel.
- Successful retry: about 509 seconds total, approximately 185 seconds build/transfer/preflight and 324 seconds host rollout including drain. Host began 02:49:10, cutover approximately 02:49:33, API release finished approximately 02:54:34. Reauth verification completed 02:55:22 (Asia/Shanghai).
- Successful host record: /var/lib/sub2api/release-records/20260929T184910Z-production-2807221.json; failed candidate: /var/lib/sub2api/release-records/20260929T184031Z-production-2791988.json.
- Application recovery: reviewed /usr/local/libexec/deploy-sub2api-blue-green-host.sh --rollback --record /var/lib/sub2api/release-records/20260929T184910Z-production-2807221.json, with protected production environment required by executor. Previous source 671bfbd059647eb475e76703dada901dbfc330d0; image sha256:cf441d30d9cf37c9ef1665ffedd56f4a0b060181cb3bf46795d6a8e6850cadbe. Retain additive schema; never overwrite new writes with an old database backup.
- Reauth recovery: pause/drain using /usr/local/libexec/sub2api-reauth-worker.sh; atomically restore protected /opt/sub2api/production/reauth-worker/runtime.env.before-sep30-122615 and resume against current healthy Go worker. Old image sha256:906095cc20f11c106087c57e48b10e7b525aba77a1d2b7d9913465fb6b1dd4ca and versioned source retained.
- Logs: local /tmp/sep30-release-final.log, /tmp/sep30-reauth-build.log; health evidence /tmp/sep30-health.jsonl. Real login/MFA and end-to-end upstream inference were not exercised; sampled network timeouts remain unattributed.
