# SDD ledger — plan: docs/superpowers/plans/2026-10-05-test-uiux-main-integration.md

Base: d36f54bdd69c183b64b5ab2456687b0ec79ff764
Merge source: 96c08182779b12c5a39ffc0ffdda9ba89ad38bde
Backup remote verified: backup/pre-main-integration-20261005-96c0818277

| Scope | Interface/self-consistency check | Finding |
|---|---|---|
| Task 1 | Existing UI vs main business API | Explicit adapter requirement; old scheduler semantic gap must be reported, not fabricated |
| Task 2 | Backup SHA/tree vs restore commit | Must use exact verified backup and normal push; does not restore database |
| Task 3 | Completion evidence vs UI approval | New navigation approval required; all unresolved checks stated |
| Task 1/2 | Same Git worktree/index | Sequential writers; Task 2 after Task 1 writer stops |
| Task 1/3 | Candidate routes/UI and verification | Existing tests reused; screenshot comparison separately required |
| Task 2/3 | Rollback entry evidence | Temporary bare-repo functional test required; no live rollback |

Task 1: active; merge has 37 text conflicts, 109 overlapping paths.

2026-10-05 用户追加明确指令：去掉调度日志页。删除旧调度日志页面、路由、导航和 API 依赖，保留主站原生调度。此指令覆盖旧页面保留要求；不实施历史归档 UI。主站既有 240_remove_custom_scheduler_artifacts.sql 保持不可变，未来部署执行前须确认历史日志删除范围与数据备份恢复边界。本次不部署。

Task 1: candidate merge conflict resolution and directly related build/tests complete; candidate visual comparison/new-entry approvals pending. Scheduler page removed per explicit user change; no release. Detailed evidence task-1-report.md.

Task 1: candidate 884d82dda6; implementer /root/integrate_candidate released writer; review pending, visual additions pending user reply.
Task 2: active, BASE 884d82dda6.

Task1 review findings resolved: configured admin intelligence direct route restored in both modes; bonus badge owner buttons position relative. Red→green targeted tests (33 passing), typecheck passing, no new UI entries. Task2 preserved.
