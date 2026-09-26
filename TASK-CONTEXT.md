# BPS upstream + PR99 integration

User authorized implementing the four proposed BPS fixes together with ranxi2001/sub2api source and PR #99. No production deployment authorization in this request.

Base: origin/main ee77d72d2d113a3de84b8e954644b08d650305cf.
Sources pinned at discovery:
- production 33fd8f24d251c761b282e8e54922bc0e642e51e9 (includes #97/#98/#102)
- PR99 head 60f813d06fc780ea3abd061e4697068a7d8cbc37, base 1c5151cc65e2e49afa254f2345d817ef733309c0; open/unmerged.
- Local proposal /Users/gongtengxinwen/Documents/运营/诊断报告/2026-09-26-BPS四项修复方案.md

Scope: relevant BPS upstream implementation/dependencies, PR99 native attachments/schema validation/transactional tool delivery/session catalog/TTL, and local auth isolation/capture outcomes/tool_choice capability/safe protocol errors. Preserve native Sub2API integration, local billing/model policy, configured legacy image relay, bounded correction retries, account/API-key/thread isolation, and safe output/replay boundaries.

Integration decisions:
- Use production `33fd8f24d251` for BPS tool transport, bounded correction, replay-cache accounting/TTL, 429 isolation, explicit native-attachment opt-in, and attachment validation. Layer PR #99's schema validation, atomic tool dispatch, session catalog and one-time unknown-target correction onto that baseline. The PR's older attachment and replay implementations are intentionally not copied. Detailed source comparison: `/Users/gongtengxinwen/.config/sub2api/diagnostics/bps-upstream-99/comparison.md`.
- Preserve local blue/green public image relay, independent image memory pools, account/model policy and usage fields. Native uploads are enabled only by the explicit `native` image mode; relay remains the default.
- Isolate structured permanent OAuth failures (including shadow-account parent credentials), distinguish capture read diagnostics from confirmed forwarding outcomes, route forced `tool_choice` to native, and expose fixed client-facing BPS protocol errors while retaining sanitized internal diagnostics.
- Keep corrections on the selected account, model, token and proxy. Neither BPS 429 nor attachment/correction failure changes Codex cooldown or schedules a different account.

Verification completed:
- `go test ./internal/service/basispoints ./internal/requestcapture ./internal/service -run 'TestExcelBPS|TestForwardAs(Chat|RawChatCompletions)|TestPermanentAuthFailure_|TestOpenAIResponses|TestBasispoints|TestCatalog|TestTool|TestReplay' -count=1`
- `go test -race ./internal/service/basispoints ./internal/requestcapture -count=1`
- `go test ./internal/service -count=1` (full service package passed in 136.164s).
- Frontend account/settings/locale Vitest: 63 passed, 11 skipped; `vue-tsc --noEmit` passed; `npm run build` passed (existing Vite chunk warnings).
- `git diff --check` and Go formatting check passed.

Remaining validation: real upstream OAuth, file upload and SSE/tool round-trip have not been exercised; no production/test-station configuration or account was changed. No root-main/queue/ledger/release-evidence edits, push or deployment in this task. Do not mark production DONE before push, release and online verification.
