#!/usr/bin/env bash
set -euo pipefail
name=${REAUTH_CONTAINER_NAME:-sub2api-openai-reauth-worker}
action=${1:-}
config_dir=${REAUTH_CONFIG_DIR:-/opt/sub2api/production/reauth-worker}
if [[ $action == pause || $action == resume ]]; then
  export REAUTH_ENV_FILE=${REAUTH_ENV_FILE:-$config_dir/worker.env}
  if [[ ! -f $REAUTH_ENV_FILE ]]; then exit 0; fi
  if [[ -f $config_dir/runtime.env ]]; then
    while IFS='=' read -r key value; do
      case "$key" in
        REAUTH_IMAGE|REAUTH_SOURCE_ROOT|REAUTH_GO_CONTAINER)
          if [[ -z ${!key:-} ]]; then export "$key=$value"; fi
          ;;
        ''|'#'*) ;;
        *) echo 'Unexpected runtime configuration key' >&2; exit 1 ;;
      esac
    done < "$config_dir/runtime.env"
  fi
fi

pause_worker() {
  if ! docker inspect "$name" >/dev/null 2>&1; then return; fi
  docker update --restart=no "$name" >/dev/null
  if [[ $(docker inspect -f '{{.State.Running}}' "$name") != true ]]; then return; fi
  docker kill --signal TERM "$name" >/dev/null
  local deadline=$((SECONDS + ${REAUTH_DRAIN_TIMEOUT_SECONDS:-1800}))
  while [[ $(docker inspect -f '{{.State.Running}}' "$name") == true ]]; do
    if (( SECONDS >= deadline )); then
      echo 'Reauth drain timed out; login remains running. Do not recreate Go worker.' >&2
      return 1
    fi
    sleep 1
  done
}

case "$action" in
  prepare)
    root=${2:?usage: prepare SOURCE_ROOT IMAGE [BUILDER]}
    image=${3:?image required}
    [[ $(git -C "$root" branch --show-current) == main ]]
    [[ -z $(git -C "$root" status --porcelain) ]]
    commit=$(git -C "$root" rev-parse HEAD)
    remote=$(git -C "$root" ls-remote origin refs/heads/main | cut -f1)
    [[ "$commit" == "$remote" && "$commit" == $(git -C "$root" rev-parse origin/main) ]]
    build=$(mktemp -d)
    trap 'rm -rf "$build"' EXIT
    python3 - "$root/infra/reauth-worker/dependencies.json" > "$build/dependencies.tsv" <<'PY'
import json,sys
lock=json.load(open(sys.argv[1]))
for name in ('turb','tosub2'):
    print(name,lock[name]['repository'],lock[name]['commit'],sep='\t')
PY
    while IFS=$'\t' read -r dependency repository sha; do
      git init -q "$build/$dependency-git"
      git -C "$build/$dependency-git" fetch -q --depth 1 "$repository" "$sha"
      [[ $(git -C "$build/$dependency-git" rev-parse FETCH_HEAD) == "$sha" ]]
      mkdir "$build/$dependency"
      git -C "$build/$dependency-git" archive FETCH_HEAD | tar -x -C "$build/$dependency"
    done < "$build/dependencies.tsv"
    builder=()
    if [[ -n ${4:-} ]]; then builder=(--builder "$4"); fi
    docker buildx build "${builder[@]}" --platform linux/amd64 --load \
      --label "org.opencontainers.image.revision=$commit" \
      --build-context "turb=$build/turb" --build-context "tosub2=$build/tosub2" \
      -f "$root/upstream/sub2api/tools/Dockerfile.openai-oauth-reauth-worker" \
      -t "$image" "$root/upstream/sub2api"
    docker image inspect --format '{{.Id}}' "$image"
    ;;
  pause) pause_worker ;;
  resume)
    : "${REAUTH_IMAGE:?set REAUTH_IMAGE to verified image ID}"
    : "${REAUTH_ENV_FILE:?protected worker-only env file required}"
    : "${REAUTH_GO_CONTAINER:?current Go worker container required}"
    : "${REAUTH_SOURCE_ROOT:?deployed source root required}"
    [[ $REAUTH_IMAGE == sha256:* ]] || { echo 'Use an immutable image ID' >&2; exit 1; }
    [[ -f $REAUTH_SOURCE_ROOT/infra/reauth-worker/supervisor.py ]]
    python3 - "$REAUTH_ENV_FILE" <<'PY'
import os,stat,sys
p=sys.argv[1]
assert stat.S_IMODE(os.stat(p).st_mode) == 0o600, 'worker env must be mode 0600'
env=dict(line.rstrip('\n').split('=',1) for line in open(p) if '=' in line and not line.startswith('#'))
assert len(env.get('OPENAI_REAUTH_WORKER_TOKEN','').strip()) >= 32, 'worker token missing'
assert env.get('SUB2API_BASE_URL') == 'http://127.0.0.1:8080', 'worker must target shared Go namespace'
allowed={'OPENAI_REAUTH_WORKER_TOKEN','SUB2API_BASE_URL','OPENAI_REAUTH_WORKER_ID','OPENAI_REAUTH_POLL_SECONDS','OPENAI_REAUTH_REQUEST_TIMEOUT','OPENAI_REAUTH_TRUSTED_OTP_HOSTS'}
assert not set(env)-allowed, 'env contains unrelated variables; use dedicated worker env'
PY
    go_state=$(docker inspect -f '{{.State.Running}}|{{if .State.Health}}{{.State.Health.Status}}{{end}}|{{.Id}}' "$REAUTH_GO_CONTAINER")
    IFS='|' read -r go_running go_health go_id <<< "$go_state"
    [[ $go_running == true && $go_health == healthy && -n $go_id ]] || { echo 'Go worker must be running and healthy' >&2; exit 1; }
    current=$(docker inspect -f '{{.State.Running}}|{{.Image}}|{{.HostConfig.NetworkMode}}|{{range .Mounts}}{{if eq .Destination "/app/reauth-supervisor.py"}}{{.Source}}{{end}}{{end}}|{{.HostConfig.RestartPolicy.Name}}' "$name" 2>/dev/null || true)
    expected="true|$REAUTH_IMAGE|container:$go_id|$REAUTH_SOURCE_ROOT/infra/reauth-worker/supervisor.py|unless-stopped"
    if [[ $current == "$expected" ]]; then
      echo 'reauth worker already running with the current Go namespace'
      exit 0
    fi
    pause_worker
    if docker inspect "$name" >/dev/null 2>&1; then docker rm "$name" >/dev/null; fi
    docker run -d --name "$name" --restart unless-stopped \
      --network "container:$go_id" --read-only \
      --cap-drop ALL --security-opt no-new-privileges:true \
      --tmpfs /tmp:rw,nosuid,nodev,size=256m,mode=1777 --pids-limit 128 \
      --log-opt max-size=10m --log-opt max-file=3 \
      --env-file "$REAUTH_ENV_FILE" \
      --mount "type=bind,src=$REAUTH_SOURCE_ROOT/infra/reauth-worker/supervisor.py,dst=/app/reauth-supervisor.py,readonly" \
      --health-cmd 'python /app/reauth-supervisor.py --health' \
      --health-interval 15s --health-timeout 5s --health-retries 3 \
      --entrypoint python "$REAUTH_IMAGE" /app/reauth-supervisor.py
    ;;
  status) docker inspect --format '{{.State.Status}} {{if .State.Health}}{{.State.Health.Status}}{{end}}' "$name" ;;
  *) echo 'usage: openai-reauth-worker.sh prepare SOURCE_ROOT IMAGE [BUILDER] | pause | resume | status' >&2; exit 2 ;;
esac
