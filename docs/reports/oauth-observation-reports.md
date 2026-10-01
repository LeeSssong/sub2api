# OAuth observation reports

`oauth-observation-reports.sql` provides read-only report queries for the new observation tables. They retain source timestamps and episode IDs because an account name can change and a reimport is a new episode. Resolve account names only in authorized reporting paths and never join or expose credential JSON.

The lifetime report is intentionally narrow: it starts at the first healthy native `turn_state_v1` probe for an episode/model/protocol/version and ends at the first later degraded probe. Inconclusive probes, scheduler changes, BPS state, expiry, quota, and recovery do not alter it. Historical scheduled-test archives are operational context only and do not feed this lifetime metric.

Events and slot intervals are retained for 90 days; usage-minute facts for 180 days. Lifetime summaries and immutable configuration/quality archives remain after those raw-event windows. An absent event, a missing recorder interval, or an unreleased slot is unknown/incomplete data, not a zero. These reports describe association and coverage only; they do not establish causal effects.
