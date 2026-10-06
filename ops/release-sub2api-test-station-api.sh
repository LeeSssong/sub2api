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
# Reuse runtime dependencies only when the entire backend and runtime source match.
while IFS= read -r path; do
  case "$path" in
    upstream/sub2api/frontend/src/*|docs/*|ops/*|tests/*|artifacts/*) ;;
    *) fail "API-only release excludes non-UI runtime changes: $path" ;;
  esac
done < <(git diff --name-only "$previous" HEAD)
printf 'test_station_api_release stage=frontend_build\n'
(cd "$root/upstream/sub2api/frontend"; pnpm run build)
printf 'test_station_api_release stage=embedded_server_build\n'
version=$(cat "$root/upstream/sub2api/backend/cmd/server/VERSION")
build_date=$(date -u +%Y-%m-%dT%H:%M:%SZ)
(cd "$root/upstream/sub2api/backend"; CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -tags embed -trimpath \
  -ldflags="-s -w -X main.Version=$version -X main.Commit=$commit -X main.Date=$build_date -X main.BuildType=release" \
  -o "$staging/sub2api" ./cmd/server)
python3 - "$staging" "$commit" "$tree" <<'PY'
import hashlib,json,pathlib,sys
root=pathlib.Path(sys.argv[1]); previous=json.loads((root/'previous-state.json').read_text())
manifest={'source_commit':sys.argv[2],'source_tree':sys.argv[3], 'previous_commit':previous['source_commit'],
          'base_image_id':previous['image_id'],'binary_sha256':hashlib.sha256((root/'sub2api').read_bytes()).hexdigest()}
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
