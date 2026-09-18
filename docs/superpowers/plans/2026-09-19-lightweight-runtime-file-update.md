# Lightweight Runtime File Update Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a fourth, tightly scoped `轻量更新主站` authorization path for runtime-direct files while retaining pushed root `main` as the only source of truth.

**Architecture:** Treat the change as one documentation contract spanning the repository entry rules, the authoritative environment/release policy, and the incremental-delivery summary. Qualification is based on proven runtime behavior rather than file type, and all three documents must describe the same authorization, fail-closed eligibility, atomic replacement, rollback, and dual-site drift semantics.

**Tech Stack:** Markdown policy documents, Git, `rg`, `git diff --check`

**Spec:** `docs/superpowers/specs/2026-09-19-lightweight-runtime-file-update-design.md`

## Global Constraints

- Every source file must already be committed and pushed to the clean root `main`, with `HEAD` commit/tree equal to `origin/main`.
- `轻量更新主站` is a distinct explicit production authorization; ordinary deployment wording is insufficient.
- Eligibility requires proof that the active service reads the target file directly and needs no compile, image build, process/container restart, configuration reload, migration, or traffic switch.
- Vue, TypeScript, Go, container-bundled assets, configuration, credentials, certificates, databases, migrations, authentication, security, billing writes, and schedulers are excluded.
- Replacement must use a verified temporary file, SHA256 checks, preserved ownership/mode, same-filesystem atomic replacement, immediate targeted verification, and backup restoration on failure.
- A lightweight production override may leave production and the acceptance station on the same commit/tree but different runtime content; that difference must remain explicit until a full release absorbs it.
- This implementation changes policy documents only. It must not modify release scripts, server files, the task queue, or the project ledger.

---

### Task 1: Align the Three Authoritative Release Contracts

**Files:**
- Modify: `AGENTS.md`
- Modify: `docs/project/acceptance-station-global-constraints.md`
- Modify: `docs/project/native-sub-incremental-delivery-constraints.md`

**Interfaces:**
- Consumes: The approved eligibility, execution, failure, and version-drift contracts in `docs/superpowers/specs/2026-09-19-lightweight-runtime-file-update-design.md`.
- Produces: One consistent four-path production authorization contract, with `轻量更新主站` as the only new authorization phrase.

- [ ] **Step 1: Capture the contradictory baseline**

Run:

```bash
rg -n -C 2 '三条唯一授权|三种明确授权|只有以下三种|所有部署|服务器目录|容器文件|Docker volume' \
  AGENTS.md \
  docs/project/acceptance-station-global-constraints.md \
  docs/project/native-sub-incremental-delivery-constraints.md
```

Expected: The command shows the existing three-path authorization language and unconditional full-release/source prohibitions that need a narrow exception.

- [ ] **Step 2: Update the repository entry rules**

In `AGENTS.md`, make these exact contract changes:

1. Keep the clean pushed root `main` requirement as the universal source requirement.
2. Narrow the prohibition on direct server/container promotion so it still forbids server, container, volume, worktree, and uncommitted content as a source, while allowing a file copied from the verified root `main` only under the lightweight exception.
3. Change the production authorization list from three to four phrases by adding `轻量更新主站`.
4. Add a compact lightweight-update bullet containing all qualification exclusions, atomic backup/checksum/replacement/verification/rollback requirements, and explicit runtime-drift recording.

- [ ] **Step 3: Update the authoritative environment and authorization policy**

In `docs/project/acceptance-station-global-constraints.md`:

1. Rename section 5 to `主站发布的四条唯一授权路径` and update its introduction accordingly.
2. Add section `D. 轻量路径：运行时直读文件原子覆盖` after path C.
3. In path D, reproduce the approved six eligibility conditions and the eight-step execution flow without broadening them.
4. State that this path skips acceptance-station deployment and full image/blue-green release only after eligibility is proven.
5. State that identical commit/tree values do not imply identical runtime content while a production override is active.
6. Update section 6 so lightweight runtime drift is a recognized, recorded exception rather than a false synchronization claim.
7. Update section 7 checks from A/B/C to A/B/C/D and add checks for target mapping, runtime-direct evidence, checksums, backup, atomic replacement, targeted verification, rollback, and runtime drift.

- [ ] **Step 4: Update the incremental-delivery summary and fast-release protocol**

In `docs/project/native-sub-incremental-delivery-constraints.md`:

1. Add `轻量更新主站` to the top authorization summary.
2. Clarify that the root `main` remains the only source even when the lightweight path avoids building a deployment artifact.
3. Update references from three authorization paths to four.
4. Add a new subsection after the general fast-release rules named `轻量运行时文件更新例外`.
5. Summarize fail-closed qualification, prohibited target classes, atomic replacement/rollback, targeted verification, and dual-site runtime drift.
6. Preserve all existing migration, downtime, credential, data-isolation, and full-release protections for ineligible changes.

- [ ] **Step 5: Run policy consistency checks**

Run:

```bash
git diff --check
rg -n '轻量更新主站' \
  AGENTS.md \
  docs/project/acceptance-station-global-constraints.md \
  docs/project/native-sub-incremental-delivery-constraints.md
rg -n '三条唯一授权|三种明确授权|只有以下三种|A、B 或 C|A、B、C' \
  AGENTS.md \
  docs/project/acceptance-station-global-constraints.md \
  docs/project/native-sub-incremental-delivery-constraints.md
rg -n 'main.*origin/main|SHA256|原子|备份|回滚|运行态差异|容器内置|Docker volume|reload' \
  AGENTS.md \
  docs/project/acceptance-station-global-constraints.md \
  docs/project/native-sub-incremental-delivery-constraints.md
```

Expected:

- `git diff --check` exits successfully.
- `轻量更新主站` appears in all three files.
- The obsolete three-path phrases return no matches, except historical quoted text that is explicitly labeled historical; there should normally be no such exception in these three current rules.
- Source, checksum, atomic replacement, rollback, runtime-drift, and exclusion terms are present in the appropriate documents.

- [ ] **Step 6: Review the final diff against the approved spec**

Run:

```bash
git diff -- \
  AGENTS.md \
  docs/project/acceptance-station-global-constraints.md \
  docs/project/native-sub-incremental-delivery-constraints.md
```

Expected: The diff changes only release-policy wording, preserves the pushed-root-`main` source requirement, does not imply that ordinary frontend source files qualify, and does not modify operational commands or credentials.

- [ ] **Step 7: Commit the aligned policy change**

Run:

```bash
git add \
  AGENTS.md \
  docs/project/acceptance-station-global-constraints.md \
  docs/project/native-sub-incremental-delivery-constraints.md
git commit -m "docs: allow controlled lightweight runtime updates"
```

Expected: One commit contains all three policy files so the four-path contract cannot be partially applied.
