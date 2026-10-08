#!/usr/bin/env bash
set -euo pipefail
umask 077
fail(){ printf 'test_station_api_release status=failed: %s\n' "$1" >&2; exit 1; }
root=$(pwd -P)
[[ -d "$root/.git" && $(git branch --show-current) == main ]] || fail 'use the root main checkout'
[[ -z $(git status --porcelain) ]] || fail 'main must be clean'
git fetch origin main >/dev/null 2>&1 || fail 'origin fetch failed'
commit=$(git rev-parse HEAD)
tree=$(git rev-parse 'HEAD^{tree}')
[[ "$commit" == $(git rev-parse origin/main) && "$tree" == $(git rev-parse 'origin/main^{tree}') ]] || fail 'main must match pushed origin/main'
: "${TEST_STATION_APPROVED_HOST:?explicitly approved test-station host required}"
[[ "$TEST_STATION_APPROVED_HOST" == 43.133.75.82 || "$TEST_STATION_APPROVED_HOST" == 49.51.203.200 ]] || fail 'invalid test-station host'
ssh_opts=(-o BatchMode=yes -o StrictHostKeyChecking=yes -o ConnectTimeout=15 -o ServerAliveInterval=10 -o ServerAliveCountMax=18)
config=$(ssh -G "${ssh_opts[@]}" sub2api-test-station 2>/dev/null)
grep -Fqx "hostname $TEST_STATION_APPROVED_HOST" <<<"$config" || fail 'SSH alias differs from approved host'
grep -Fqx 'user ubuntu' <<<"$config" || fail 'test station must use ubuntu'
grep -Fqx 'port 22' <<<"$config" || fail 'test station must use port 22'
staging="$root/.release/test-station-api-$commit"
mkdir -p "$staging"
ssh -T "${ssh_opts[@]}" sub2api-test-station 'sudo -n cat /opt/sub2api-test-station/release-state.json' >"$staging/previous-state.json"
previous=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["source_commit"])' "$staging/previous-state.json")
update_worker=${TEST_STATION_UPDATE_WORKER:-false}
[[ "$update_worker" == true || "$update_worker" == false ]] || fail 'invalid worker update flag'
# Monitor service changes require the same new binary in API and singleton worker.
# Native group catalogue handlers and their read-only tool mapping can update API only.
# The reviewed user popularity projection reuses native read-only aggregates, API only.
# The reviewed manual check admission only affects API handlers; worker work is unchanged.
# Route SLA timeline and admin raw counts are read-only API queries, not worker jobs.
# Runtime dependencies and unreviewed backend changes remain excluded.
# The exact production integration files require the serialized worker update.
# Embedded migrations must already be applied with identical checksums on the host.
# Homepage files are deployed independently and are not inputs to the API binary.
# The embedded brand icon handler only reads existing public settings.
while IFS= read -r path; do
  case "$path" in
    upstream/sub2api/frontend/src/*|upstream/sub2api/frontend/DESIGN.md|docs/*|ops/*|tests/*|artifacts/*) ;;
    homepage/*|infra/independent-test-station/Dockerfile.homepage|\
    upstream/sub2api/backend/internal/web/embed_on.go|\
    upstream/sub2api/backend/internal/web/embed_test.go|\
    upstream/sub2api/backend/internal/web/favicon.go|\
    upstream/sub2api/backend/internal/web/favicon_test.go) ;;
    upstream/sub2api/backend/internal/handler/usage_handler.go|\
    upstream/sub2api/backend/internal/handler/usage_model_popularity.go|\
    upstream/sub2api/backend/internal/handler/usage_model_popularity_test.go|\
    upstream/sub2api/backend/internal/middleware/line_check_rate_limiter.go|\
    upstream/sub2api/backend/internal/server/middleware/line_check_rate_limit.go|\
    upstream/sub2api/backend/internal/server/middleware/panel_rate_limit.go|\
    upstream/sub2api/backend/internal/server/routes/monitor_v4_check_rate_limit_test.go|\
    upstream/sub2api/backend/internal/service/monitor_v4.go|\
    upstream/sub2api/backend/internal/service/monitor_v4_check.go|\
    upstream/sub2api/backend/internal/service/monitor_v4_check_test.go|\
    upstream/sub2api/backend/internal/service/monitor_v4_timeline.go|\
    upstream/sub2api/backend/internal/service/monitor_v4_timeline_test.go|\
    upstream/sub2api/backend/internal/handler/monitor_v4_handler_test.go|\
    upstream/sub2api/backend/internal/repository/monitor_v4_timeline.go|\
    upstream/sub2api/backend/internal/repository/ops_repo_dashboard.go|\
    upstream/sub2api/backend/internal/repository/ops_sla_sql.go|\
    upstream/sub2api/backend/internal/repository/route_sla_timeline_postgres_test.go|\
    upstream/sub2api/backend/scripts/verify_prototype_monitor_postgres.py|\
    upstream/sub2api/backend/internal/server/routes/user.go|\
    upstream/sub2api/backend/internal/handler/api_key_handler.go|\
    upstream/sub2api/backend/internal/handler/gateway_handler.go|\
    upstream/sub2api/backend/internal/handler/gateway_user_models.go|\
    upstream/sub2api/backend/internal/handler/gateway_model_catalog.go|\
    upstream/sub2api/backend/internal/handler/gateway_user_models_test.go|\
    upstream/sub2api/backend/internal/handler/api_key_available_groups_tools_test.go|\
    upstream/sub2api/backend/internal/handler/wire.go|\
    upstream/sub2api/backend/internal/handler/handler_wiring_test.go|\
    upstream/sub2api/backend/internal/service/model_plaza_service.go|\
    upstream/sub2api/backend/internal/service/model_plaza_service_test.go|\
    upstream/sub2api/backend/internal/service/group_tool_mapping.go|\
    upstream/sub2api/backend/internal/service/api_key_group_tool_mapping_test.go) ;;
    upstream/sub2api/backend/internal/service/account_test_service.go|\
    upstream/sub2api/backend/internal/service/account_test_service_openai_test.go|\
    upstream/sub2api/backend/internal/service/account_monitor_probe_test.go|\
    upstream/sub2api/backend/internal/service/account_probe_cost_test.go|\
    upstream/sub2api/backend/internal/service/monitor_v4*.go)
      [[ "$update_worker" == true ]] || fail 'monitor backend changes require a worker update' ;;
    upstream/sub2api/Dockerfile|\
    upstream/sub2api/backend/internal/repository/account_quality_models.go|\
    upstream/sub2api/backend/internal/repository/account_quality_models_test.go|\
    upstream/sub2api/backend/internal/repository/intelligence_rules.go|\
    upstream/sub2api/backend/internal/repository/intelligence_rules_integration_test.go|\
    upstream/sub2api/backend/internal/repository/migrations_runner.go|\
    upstream/sub2api/backend/internal/repository/migrations_test_main_online_test.go|\
    upstream/sub2api/backend/internal/repository/pelican_group_tests_repo.go|\
    upstream/sub2api/backend/internal/service/model_rate_limit.go|\
    upstream/sub2api/backend/internal/service/pelican_scheduled.go|\
    upstream/sub2api/backend/internal/service/quality_supported_models.go|\
    upstream/sub2api/backend/internal/service/quality_supported_models_test.go|\
    upstream/sub2api/backend/migrations/265_monitor_v4_legacy_default.sql|\
    upstream/sub2api/backend/migrations/241_remove_monitor_v4_operational_flag.sql|\
    upstream/sub2api/backend/migrations/deferred/241_remove_monitor_v4_operational_flag.sql|\
    upstream/sub2api/backend/migrations/deferred/README.md)
      [[ "$update_worker" == true ]] || fail 'production integration requires a worker update' ;;
    *) fail "release excludes unsupported runtime changes: $path" ;;
  esac
done < <(git diff --name-only "$previous" HEAD)
binary_commit=$commit
binary_tree=$tree
if [[ -n ${TEST_STATION_REUSE_BUILD_FROM:-} ]]; then
  reuse=$TEST_STATION_REUSE_BUILD_FROM
  [[ "$reuse" == "$root/.release/test-station-api-"* && -d "$reuse" && ! -L "$reuse" ]] || fail 'reuse requires a local release artifact'
  binary_commit=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["source_commit"])' "$reuse/manifest.json")
  binary_tree=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["source_tree"])' "$reuse/manifest.json")
  [[ "$binary_commit" =~ ^[a-f0-9]{40}$ && "$binary_tree" == $(git rev-parse "$binary_commit^{tree}") ]] || fail 'reused source identity is invalid'
  git diff --quiet "$binary_commit" HEAD -- upstream/sub2api || fail 'runtime inputs changed; rebuild required'
  python3 - "$reuse" "$staging/previous-state.json" <<'PY'
import hashlib,json,pathlib,sys
root=pathlib.Path(sys.argv[1]); manifest=json.loads((root/'manifest.json').read_text())
previous=json.load(open(sys.argv[2]))
binary=root/'sub2api'
if binary.is_symlink() or hashlib.sha256(binary.read_bytes()).hexdigest()!=manifest['binary_sha256']:
    raise SystemExit('reused binary checksum mismatch')
if previous['image_id']!=manifest['base_image_id']:
    raise SystemExit('runtime dependencies changed; rebuild required')
PY
  cp "$reuse/sub2api" "$staging/sub2api"
  printf 'test_station_api_release stage=reused_build binary_source_commit=%s\n' "$binary_commit"
else
  printf 'test_station_api_release stage=frontend_build\n'
  (cd "$root/upstream/sub2api/frontend"; pnpm run build)
  printf 'test_station_api_release stage=embedded_server_build\n'
  version=$(cat "$root/upstream/sub2api/backend/cmd/server/VERSION")
  build_date=$(date -u +%Y-%m-%dT%H:%M:%SZ)
  (cd "$root/upstream/sub2api/backend"; CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -tags embed -trimpath \
    -ldflags="-s -w -X main.Version=$version -X main.Commit=$commit -X main.Date=$build_date -X main.BuildType=release" \
    -o "$staging/sub2api" ./cmd/server)
fi
python3 - "$staging" "$commit" "$tree" "$binary_commit" "$binary_tree" "$update_worker" <<'PY'
import hashlib,json,pathlib,sys
root=pathlib.Path(sys.argv[1]); previous=json.loads((root/'previous-state.json').read_text())
manifest={'source_commit':sys.argv[2],'source_tree':sys.argv[3], 'previous_commit':previous['source_commit'],
          'base_image_id':previous['image_id'],'binary_sha256':hashlib.sha256((root/'sub2api').read_bytes()).hexdigest(),
          'binary_source_commit':sys.argv[4], 'binary_source_tree':sys.argv[5], 'update_worker':sys.argv[6]=='true'}
migrations=root.parents[1]/'upstream/sub2api/backend/migrations'
manifest['migration_checksums']={p.name:hashlib.sha256(p.read_text().strip().encode()).hexdigest() for p in sorted(migrations.glob('*.sql')) if p.read_text().strip()}
digest=hashlib.sha256()
for name, checksum in sorted(manifest['migration_checksums'].items()):
    digest.update((name+'\0'+checksum+'\n').encode())
manifest['migration_set_sha256']=digest.hexdigest()
(root/'manifest.json').write_text(json.dumps(manifest,indent=2)+'\n')
PY
[[ -z $(git status --porcelain) && "$commit" == $(git rev-parse HEAD) ]] || fail 'source changed during build'
cp "$root/ops/deploy-sub2api-test-station-api.py" "$staging/deploy.py"
tar -czf "$staging/bundle.tar.gz" -C "$staging" sub2api manifest.json deploy.py
archive_sha=$(shasum -a 256 "$staging/bundle.tar.gz" | awk '{print $1}')
remote=$(ssh -T "${ssh_opts[@]}" sub2api-test-station 'mktemp -d /var/tmp/sub2api-test-station-api.XXXXXX')
[[ "$remote" =~ ^/var/tmp/sub2api-test-station-api\.[a-zA-Z0-9]+$ ]] || fail 'unsafe remote staging path'
printf '%s\n' "$remote" >"$staging/remote-staging-path"
printf 'test_station_api_release stage=transfer\n'
scp -q "${ssh_opts[@]}" "$staging/bundle.tar.gz" "sub2api-test-station:$remote/bundle.tar.gz"
ssh -T "${ssh_opts[@]}" sub2api-test-station \
  "test \"\$(sha256sum '$remote/bundle.tar.gz' | cut -d' ' -f1)\" = '$archive_sha' && tar -xzf '$remote/bundle.tar.gz' -C '$remote' && sudo -n python3 '$remote/deploy.py' deploy '$remote'"
printf 'test_station_api_release status=promoted verification_required=true staging=%s\n' "$staging"
