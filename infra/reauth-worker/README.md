# Official OpenAI reauthentication worker

The official Python worker and Dockerfile remain unchanged. `dependencies.json`
pins separately audited external protocol checkouts. Turb's newer d90f88f snapshot
requires pyotp and core.db, which the official Dockerfile does not include; the
selected d32e49e snapshot matches its exact copied files. No browser, registration
service, web console, database, or third-party account upload service is started.

## Prepare

From clean, pushed root main, run:

```sh
bash ops/openai-reauth-worker.sh prepare /path/to/source sub2api-openai-reauth:RELEASE BUILDER
```

The helper verifies the remote main commit, fetches immutable dependency SHAs,
exports clean build contexts, and builds the official Dockerfile for linux/amd64.
Run against the intended Docker context; select the corresponding builder. Retain
the image ID/digest, build log (including base-image digests), source commit/tree,
lock-file checksum and supervisor checksum in the single release record. Base
tags and transitive package resolutions remain controlled by the upstream
Dockerfile: promote the same built image instead of claiming independent builds
are bit reproducible. An image tagged with a commit is insufficient evidence by
itself; the build log and verified inputs must match.

## Start and drain

Use a dedicated 0600 env file, never the full API env:

```text
OPENAI_REAUTH_WORKER_TOKEN=<same protected random token configured on Go worker>
SUB2API_BASE_URL=http://127.0.0.1:8080
OPENAI_REAUTH_WORKER_ID=production-reauth-1
```

Set `REAUTH_IMAGE` to the verified local image ID, `REAUTH_ENV_FILE` to that file,
`REAUTH_SOURCE_ROOT` to the deployed verified source directory, and
`REAUTH_GO_CONTAINER` to the running Go worker container. Then:

For release hooks, install the script at
`/usr/local/libexec/sub2api-reauth-worker.sh`. The default dedicated secret file is
`/opt/sub2api/production/reauth-worker/worker.env`; if it is absent, pause/resume
are no-ops. Place `REAUTH_IMAGE`, `REAUTH_SOURCE_ROOT` and `REAUTH_GO_CONTAINER`
as plain unquoted KEY=value lines in adjacent `runtime.env` (root-owned 0600).
The parser only accepts these three keys and does not execute shell content.
`REAUTH_CONFIG_DIR` can override that directory for tests or another environment.

```sh
bash ops/openai-reauth-worker.sh resume
bash ops/openai-reauth-worker.sh status
bash ops/openai-reauth-worker.sh pause
```

The container shares the Go worker network namespace, so HTTP localhost and
Mihomo leases resolve to the same API instance throughout a task. Internal routes
have no API-role restriction. Do not route these calls through the public API
blue/green load balancer. Before recreating the Go worker, pause reauthentication;
resume against the new container only after it is ready. Apply the same sequence
on rollback. An API-only blue/green switch does not require restarting this worker.

Pause disables Docker restart, signals only the supervisor, and waits for the
current official `--once` process. It never forcibly kills login. Drain timeout
(default 1800 seconds) fails the caller; leave the Go worker intact and investigate.
Password protocol is bounded by the official 25-minute timeout; task stale reclaim
is 30 minutes. Do not run a second worker to accelerate drain.

The Docker healthcheck tests supervisor liveness without claiming tasks or reading
secrets. It does not prove API authentication, queue progress, real OpenAI login,
or email/MFA delivery. Verify initial startup for import errors, then observe
sanitized job status through the admin interface when authorized tasks exist.
Do not run `--once` as a probe against production.

## Secrets and validation boundaries

Protocol logs from Turb are suppressed by the official worker. toSub2 stdout and
stderr are captured and omitted on failure; temporary OAuth/checkpoint files stay
under the 0700 Python temporary directory on tmpfs. Password/TOTP use child env;
proxy credentials and email may occur in child argv, so host/process access must
remain restricted. The worker token is removed from the protocol child env.
The supervisor mount is read-only and the container root filesystem is read-only.

Dependencies contact OpenAI, account-selected proxies and the configured mailbox;
proxy geolocation may contact the dependency's configured public IP services.
Do not inject unrelated CHATGPT/AUTH/proxy/solver environment variables. Private
mailbox hosts require explicit narrow OPENAI_REAUTH_TRUSTED_OTP_HOSTS approval.

Offline validation on 2026-09-29: 24 official worker unit tests; exact Dockerfile
Turb subset imports with Python 3.12/curl_cffi 0.16.0; empty mock claim; toSub2
local mock password/email OTP + 2FA/workspace suite. No real accounts were used.
The local image build was blocked by a full Colima Docker volume. Target Linux
image startup and real login must be recorded separately after actual validation.
The toSub2 TLS fingerprint transport local-server suite also passed. Node tests
used local Node 26; the final official image uses Node 20 and still requires its
own import/startup smoke. Lifecycle verification covers TERM drain, non-claiming
health, missing configuration, fail-closed drain timeout, shared namespace,
idempotent resume and namespace replacement (seven tests total).
