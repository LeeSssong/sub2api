#!/usr/bin/env bash
set -euo pipefail

ROOT=${TEST_PROJECT_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)}
RELEASE=$ROOT/ops/release-sub2api-blue-green.sh
HOST=$ROOT/ops/deploy-sub2api-blue-green-host.sh
fail() { printf 'FAIL: %s\n' "$1" >&2; exit 1; }

[[ -f "$RELEASE" && -f "$HOST" ]] || fail 'release scripts are missing'

# The reviewed monitor-v4 transition must be represented in both gates.
grep -Fq 'MONITOR_V4_OLD_MIGRATIONS_HASH=' "$RELEASE" || fail 'controller predecessor hash is missing'
grep -Fq 'MONITOR_V4_NEW_MIGRATIONS_HASH=' "$RELEASE" || fail 'controller target hash is missing'
grep -Fq 'MONITOR_V4_OLD_MIGRATIONS_HASH=' "$HOST" || fail 'host predecessor hash is missing'
grep -Fq 'MONITOR_V4_NEW_MIGRATIONS_HASH=' "$HOST" || fail 'host target hash is missing'
grep -Fq 'MONITOR_V4_NEW_MIGRATIONS_HASH' "$RELEASE" || fail 'controller target is not checked'
grep -Fq 'MONITOR_V4_NEW_MIGRATIONS_HASH' "$HOST" || fail 'host target is not checked'

# Online monitor-v4 promotion keeps the old API serving and updates worker;
# detector must remain available for the candidate and rollback path.
grep -Fq 'RELEASE_PRESERVE_WORKER:-false' "$RELEASE" || fail 'controller does not expose worker replacement gate'
grep -Fq 'RELEASE_PRESERVE_DETECTOR:-false' "$RELEASE" || fail 'controller does not expose detector preservation gate'
grep -Fq 'preserve_worker=${PRESERVE_WORKER:-false}' "$HOST" || fail 'host does not expose worker replacement gate'
grep -Fq 'preserve_detector=${PRESERVE_DETECTOR:-false}' "$HOST" || fail 'host does not expose detector preservation gate'

# The destructive monitor retirement migration is deferred and must never be
# part of the executable migration directory or release allowlist.
if find "$ROOT/upstream/sub2api/backend/migrations" -maxdepth 1 -type f -name '241_remove_monitor_v4_operational_flag.sql' -print -quit | grep -q .; then
  fail 'destructive monitor migration is executable'
fi
grep -Fq 'deferred' "$HOST" || fail 'host script does not document deferred destructive migrations'
! grep -Fq '241_remove_monitor_v4_operational_flag.sql' "$HOST" || fail 'host script executes destructive monitor migration'

# Keep explicit evidence that API stop is maintenance-only, never online.
awk '/if \[\[ "\$online_migration_transition" == true \]\]; then/{online=1} online && /stop sub2api-blue sub2api-green sub2api-worker/{exit 1} END{exit 0}' "$HOST" \
  || fail 'online migration path stops the active API'
grep -Fq 'docker stop --time 300' "$HOST" || fail 'rollback/candidate drain no longer has the 300s bound'
printf 'PASS: monitor-v4 online release contract\n'
