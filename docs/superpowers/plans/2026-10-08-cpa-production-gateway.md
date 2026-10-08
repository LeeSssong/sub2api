# CPA Production Gateway Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deploy an isolated CPA + CPAMP + ticket-gateway stack on production and publish only its management surface at `cpa.xingqiaolab.top`.

**Architecture:** A dedicated Compose project joins its private network and the existing `sub2api_default` network without host port mappings. Existing Caddy adds one independent hostname, rejects public inference paths, and proxies management/usage paths to the new services; Cloudflare provides the proxied DNS record.

**Tech Stack:** Docker Compose, Caddy 2.10, Cloudflare DNS, CLIProxyAPI v8.0.20, CPA Manager Plus v1.14.4, shell contract tests.

**Spec:** `docs/superpowers/specs/2026-10-08-cpa-production-gateway-design.md`

## Global Constraints

- Do not create a CPA business API key and do not modify Sub2API upstream accounts or groups.
- Do not restart or recreate Sub2API, Codex2API, PostgreSQL, Redis, workers, detectors, or Caddy; Caddy may only receive a validated graceful reload.
- Use `linux/amd64` pinned image manifests and ticket plugin archive SHA-256 `3d3c6126e965258a5a56c7d29bc28594785382f69eecaa17e83e0b639921a88c`.
- Do not publish host ports 8317 or 18317.
- Preserve all rollback artifacts and never run `docker compose down -v`.
- Keep secrets in `/opt/cpa-manager-plus/secrets` with mode `0600`; never print or commit them.

---

### Task 1: Add the isolated production stack and edge contract

**Files:**
- Create: `infra/cpa-production/compose.yaml`
- Create: `infra/cpa-production/config.example.yaml`
- Modify: `infra/Caddyfile`
- Create: `tests/infra/validate-cpa-production.sh`

**Interfaces:**
- Consumes: external Docker network `sub2api_default`.
- Produces: Docker aliases `cpa-gateway:8317` and `cpa-manager:18317`; public hostname `cpa.xingqiaolab.top` with inference paths rejected.

- [ ] **Step 1: Write the failing contract test**

Create a shell test that requires pinned image digests, no `ports`, the two named networks, plugin/data mounts, health checks, the Caddy hostname, management proxies, and explicit public inference rejection.

- [ ] **Step 2: Run the test and verify it fails**

Run: `bash tests/infra/validate-cpa-production.sh`

Expected: non-zero because `infra/cpa-production/compose.yaml` and the Caddy site do not exist.

- [ ] **Step 3: Add minimal Compose, config example, and Caddy site**

The Compose file must use:

```yaml
name: xingqiao-cpa
services:
  cpa-gateway:
    image: eceasy/cli-proxy-api@sha256:0c59d29e962089bec30e5cdf174ef29024693bb3af7fb2a7c111dfab664b84c5
  cpa-manager:
    image: seakee/cpa-manager-plus@sha256:37933b2b64dd60c7a1696096d738ad7cf9f0a7475fd7bc12c2eb7a0124b2f7f9
```

Both services join `xingqiao-cpa-internal` and external `sub2api_default`, use no host port mapping, and mount only their own paths/volume. The example config contains placeholders only, never production secrets.

The Caddy site must reject `/v1/*`, `/v1beta/*`, `/responses`, `/chat/completions`, and `/messages`; route `/usage-service/*` to `cpa-manager:18317`; and proxy remaining management UI/resource paths to `cpa-gateway:8317`.

- [ ] **Step 4: Run focused tests**

Run:

```bash
bash tests/infra/validate-cpa-production.sh
bash tests/infra/validate-baseline.sh
git diff --check
```

Expected: all exit 0.

- [ ] **Step 5: Commit**

```bash
git add infra/cpa-production infra/Caddyfile tests/infra/validate-cpa-production.sh
git commit -m "infra: add isolated CPA production stack"
```

### Task 2: Integrate and publish the production source

**Files:**
- Modify: root `main` through a non-fast-forward merge from `codex/cpa-production`

**Interfaces:**
- Consumes: Task 1 commit and design/plan commits.
- Produces: pushed `origin/main` whose commit and tree match the clean local release source.

- [ ] **Step 1: Re-run the focused tests on the feature branch**

Run the Task 1 verification commands and record the exact commit/tree.

- [ ] **Step 2: Merge into root main without disturbing user files**

Fetch `origin`, confirm local `main` equals `origin/main`, merge `codex/cpa-production`, and preserve the existing untracked local CPA installation and installer. Do not stage or commit local secrets.

- [ ] **Step 3: Push and verify source identity**

Push `main`, fetch `origin/main`, and verify local/remote commit and tree equality. Abort deployment if the worktree is detached, dirty, or mismatched.

### Task 3: Deploy, expose, and verify CPA

**Files:**
- Create on host: `/opt/cpa-manager-plus/` runtime files and protected secrets
- Modify on host: `/opt/sub2api/production/Caddyfile` through backup and atomic replacement
- Create on host: `/var/lib/sub2api/release-records/<timestamp>-cpa-production.json`

**Interfaces:**
- Consumes: pushed `main`, Task 1 Compose/Caddy files, external network `sub2api_default`, Cloudflare zone `xingqiaolab.top`.
- Produces: `https://cpa.xingqiaolab.top/management.html`, private `cpa-gateway:8317`, and an operator handoff for manually creating the Sub2API key.

- [ ] **Step 1: Capture the unchanged-service baseline**

Record container IDs, start times, restart counts, public health responses, current Caddy checksum, and the current production commit/tree without printing environment values.

- [ ] **Step 2: Stage runtime files and generate protected management secrets**

Copy only committed Compose/config inputs from pushed `main`, generate CPA/CPAMP management secrets on the server with `umask 077`, download the AMD64 plugin archive, verify both the release checksum and package checksum, and install its `.so` under the CPA plugin mount.

- [ ] **Step 3: Start only the new Compose project**

Run `docker compose --project-name xingqiao-cpa up -d`, wait on CPAMP health and CPA management readiness, then verify plugin v1.4.0 registration and management authentication.

- [ ] **Step 4: Create the Cloudflare DNS record**

Create proxied A record `cpa.xingqiaolab.top -> 64.83.10.67`, TTL Auto. Do not modify other records.

- [ ] **Step 5: Validate and reload Caddy**

Build a candidate Caddyfile from pushed `main`, validate it inside the existing Caddy image/container context, back up the active file, atomically replace it, and run `caddy reload`. Restore the backup immediately if validation, reload, certificate issuance, or routing fails.

- [ ] **Step 6: Run public and isolation verification**

Verify management HTTP 200 and page rendering, inference-path rejection, management authentication, CPAMP health, plugin registration, and unchanged Sub2API/Codex2API public health. Compare baseline container IDs/start times/restart counts and fail if unrelated services changed.

- [ ] **Step 7: Write the release record and handoff**

Record source commit/tree, image manifests, plugin checksum, changed host files, timings, checks, result, rollback path, and that the test station was neither queried nor synchronized. Tell the administrator how to add OAuth accounts and create a dedicated `sk-...` key in CPA without exposing its value.
