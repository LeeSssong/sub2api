# BPS degradation and shadow recovery

Approved by user in chat on 2026-09-27. Completion means deployable, then STOP for deployment instruction. No production writes, main integration, push or deployment in this task.

Import ranxi2001/sub2api PR #161 (dc01c71b758e8c24bd76ea5f7ddb089fc76481d3) and #163 (26c623e1b995eeb6ac1ed309d3047630bd3ebc9e) with source attribution, adapted beneath upstream/sub2api. Preserve local customizations.

BPS actual upstream HTTP 403/500-599 degrades an account to regular mode. Shadow recovery is opt-in only while BPS configured; an independent regular-mode model whitelist reuses ModelWhitelistSelector. Preserve configured BPS models for restoration. Separate desired configuration from temporary runtime fallback, so editing an automatically degraded account cannot destroy its BPS settings. Manual off cancels recovery. Non-normal accounts must not be probed or restored.

Shadow is an isolated account context with same credentials, never a second schedulable database account. First probe after 5 minutes; after failures wait 10/15/20/25/30 minutes and continue 30-minute intervals. A pass restores only the matching degradation generation; concurrent failures, edits, credential changes, deletion, manual disable and status changes must not be overwritten. Persist state in existing account extra and synchronize scheduler caches; claim probes atomically across instances. No schema migration needed.

Compatibility discovery: quality-operations state_probe requires Codex ticket headers and explicitly rejects BPS. Await user's selection between BPS native text/tool probe and BPS native probe plus regular state probe; do not silently treat HTTP 200 as success or claim unsupported BPS ticket probing.

429 retains PR #161 failover/cooldown behavior and does not trigger 403/5xx recovery. Never replay a request after client output has started. Probe errors must not modify groups, ban credentials, or enter client billing. Record safe status/timing without credentials or upstream body.
