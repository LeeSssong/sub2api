# SDD ledger — plan: docs/superpowers/plans/2026-09-27-bps-shadow-recovery.md

## Preflight

| Tasks | Shared scope | Coordination |
| --- | --- | --- |
| 1 and 2 | `openai_excel_bps*` service paths | Import completes before recovery implementation; one writer. |
| 1 and 3 | Quality locale files and account UI scope | Separate quality and account keys; preserve both. |
| 2 and 3 | Account `extra` recovery keys | Agree on the persisted contract before UI serialization. |

## Task 1 — upstream PR import

Base: `bf519ef6d4`. Source PR #161: `dc01c71b758e8c24bd76ea5f7ddb089fc76481d3`. Source PR #163: `26c623e1b995eeb6ac1ed309d3047630bd3ebc9e`.

Implementation committed as `569d282aae` and `2012d30115`. Focused review remains pending. See `task-1-report.md` for adaptation and verification details.
