# BPS shadow recovery account UI handoff — 2026-09-27

Branch: `codex/bps-account-settings` (base `ba6121afba`). Scope: frontend account Edit and Bulk forms, English/Chinese account labels, focused form tests. No push, merge, server access, or deployment.

- Both forms replace the obsolete 403 auto-disable checkbox with an explanation of automatic fallback on actual BPS 403/5xx errors and an optional shadow recovery control. They reuse the normal OpenAI model selector for independent fallback models, require at least one model when enabling shadow recovery, and trim/deduplicate on save.
- Edit shows active regular fallback and the next BPS text/tool roundtrip probe. It omits server-owned `openai_excel_bps_recovery` and obsolete auto-disable state from saves while preserving desired `openai_excel_bps` and existing BPS model settings. Turning shadow off on a degraded account retains the fallback models, shows probes paused, and leaves regular fallback active. Turning BPS off clears shadow and fallback settings.
- Bulk respects its existing apply checkbox. Applying BPS with shadow off does not alter each account's fallback model list. Applying BPS off clears both settings. Legacy group move preference stays stored during shadow recovery, and the form explains that automatic group changes are suppressed while shadow recovery is enabled.
- Recovery copy describes first probe after 5 minutes and delays of 10, 15, 20, 25, then 30 minutes, followed by every 30 minutes; inactive accounts pause. It does not describe the probe as a Codex quality ticket probe.

Verification: focused Edit/Bulk Vitest suites (162 tests) passed; `pnpm run typecheck` passed; `pnpm run check:i18n` passed; targeted ESLint passed. `pnpm run build` passed (including i18n and Vue type checks; existing bundle-size warnings). Deployment and live behavior were not verified.
