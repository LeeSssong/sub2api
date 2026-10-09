# Official 2.10.1 integration

Source: https://github.com/ranxi2001/sub2api/tree/11be589504b482d77827ab383b4b53f777241238 (production, live ls-remote verified). Prior imported source is bc83ff9c367883e5b7d0140e6bb42e2e7cc5239c / 2.9.7; local base b6f7f2e597da14527c87ed5215d7b11cea9294b0. This branch does not merge external Git history. 694 upstream changed files were compared against the vendor: 39 equivalent files skipped, 488 direct upstream changes, 95 automatic content merges, 72 conflicts reviewed with local customization retained. Wiring regenerated. File decisions are in 2026-10-09-official-source.json.

Nine new migrations retain exact upstream SQL bytes. All previous migration bytes and compatibility checksums remain unchanged. Runner bounds each new transaction with 100ms lock_timeout and 2s statement_timeout.

| Migration | Changes | Online compatibility condition |
|---|---|---|
| 244_astra_gateway_history | New history table/index | Additive |
| 249_astra_scheduling_states | New state table referencing accounts | Additive, bounded lock |
| 263_strip_pelican_prompt_restriction | UPDATE three plans/templates, removing old prompt phrase | Keeps historical result snapshots; bounded transaction |
| 264_channel_monitor_v3 | New categories/components/config tables/index/default row | Additive; does not alter monitor-v4 |
| 265_new_api_site_authorizations | Two new tables/index; accounts BEFORE UPDATE trigger | Bounded trigger installation; removes binding when upstream identity changes |
| 266_account_ops_notifications | Expands kind/state CHECK; adds details/deliveries/identity columns | ACCESS EXCLUSIVE short lock; old kinds remain valid |
| 267_account_ops_threshold_episodes | Two new tables/indexes; copies incomplete threshold tasks then suppresses obsolete queue | Release preflight must confirm no old threshold pending/inflight; rollback must not assign new pending threshold tasks to old worker |
| 268_controlled_experiments | New runs/attempt tables and single-active index | Additive |
| 268_support_tickets | New tickets/messages tables/indexes | Additive |

Existing local billing/quota/financial ledgers, probe cost, AccountAdmission, process role separation, monitor-v4/native/hybrid modes, user AI-tools navigation, Prism keepalive/session/per-account limits and expiring unsupported-model catalog markers are retained. Official qualityqueue implementation replaces the equivalent local Redis implementation; old queue keys are retained. SheetJS 0.20.3 upstream upgrade remains, with SHA512 integrity computed from the actual official CDN tarball to satisfy supply-chain verification. No production or test-station access, push, deployment, or root-main modification performed by this integration agent.

Validation results will be appended after current checks finish. Real database tests use a disposable local PostgreSQL database; production checks belong to the release controller.

Validation at candidate freeze:
- `GOMAXPROCS=2 go build -p 1 ./cmd/server` passed after Wire regeneration.
- `vue-tsc --noEmit`, locale completeness (3), and Vite production build passed. Local frontend checks reused the existing dependency installation (Vue 3.5.26 / xlsx 0.18.5); the committed upstream lock upgrades these to Vue 3.5.43 / SheetJS 0.20.3 and the release artifact build must install that exact lock.
- Relevant frontend test files passed: HelpTooltip 19, AccountQuality 65, usageTps 6, support ticket utility 4, user status 3, CreateAccount 75, EditAccount 122, Sidebar 16, ControlledExperiments 7, SupportTicketsAdmin 4, monitor mode switch 3; locale 3.
- `go test` config/upstreamroute selected tests and qualityqueue/upstreamroute full package tests passed.
- Prism selector/protocol/server/image tests (39) and multiplex tests (30) passed. Multiplex uses Python >=3.11; initial system Python 3.9 invocation could not support asyncio.timeout and was replaced by a bundled Python virtual environment.
- First real PostgreSQL 18 upgrade run passed: baseline migration set, bounded accounts lock failure, candidate migration application/retry, all old receipts/columns preserved, old alert sent row preserved, old worker insert/update contract, no spurious threshold tasks. Final run adds explicit old-binary migration verification and is still running at candidate commit.
- Broad service/handler unit chain was cancelled during service test compilation due concurrent compiler memory pressure; no passing claim for those packages. Final integrated tree needs only the related conflicts tests.
- No container application build, production runtime, browser interaction, or online feature verification performed by this agent.

Review focus: custom billing/quota/financial/probe-cost paths and AccountAdmission retained; local ProcessRole API/worker split remains alongside official RuntimeRole. Official new Astra preparation setup runs from ProvideAccountTestService and should respect singleton worker ownership. Local Prism readiness/session gates and per-account limits retain their stronger behavior. Quality model support rejection markers keep the local five-minute expiry instead of upstream permanent suppression. ChannelMonitor supports the local native_probe/hybrid modes in addition to official v3; route feature gate and mode selection should not hide current main's monitoring pages. User navigation keeps the local AI-tools/intelligence-test design while exposing opt-in support tickets. Existing local HelpTooltip/AccountStatusIndicator rendering retained to preserve model quality-hold display; new upstream callers still typecheck. New API configuration methods use a checked optional capability interface so existing upstream-billing lifecycle test stubs remain compatible.
