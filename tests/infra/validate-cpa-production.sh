#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
compose_file="$repo_root/infra/cpa-production/compose.yaml"
config_example="$repo_root/infra/cpa-production/config.example.yaml"
caddy_file="$repo_root/infra/Caddyfile"

fail() {
  printf 'FAIL: %s\n' "$*" >&2
  exit 1
}

[[ -f "$compose_file" ]] || fail "missing $compose_file"
[[ -f "$config_example" ]] || fail "missing $config_example"

compose_json=$(docker compose --project-directory "$(dirname "$compose_file")" \
  -f "$compose_file" config --format json)

jq -e '
  .name == "xingqiao-cpa" and
  .services["cpa-gateway"].image == "eceasy/cli-proxy-api@sha256:0c59d29e962089bec30e5cdf174ef29024693bb3af7fb2a7c111dfab664b84c5" and
  .services["cpa-manager"].image == "seakee/cpa-manager-plus@sha256:37933b2b64dd60c7a1696096d738ad7cf9f0a7475fd7bc12c2eb7a0124b2f7f9" and
  (.services["cpa-gateway"] | has("ports") | not) and
  (.services["cpa-manager"] | has("ports") | not) and
  .networks.internal.name == "xingqiao-cpa-internal" and
  .networks.sub2api.name == "sub2api_default" and
  .networks.sub2api.external == true and
  .services["cpa-gateway"].networks.sub2api.aliases == ["cpa-gateway"] and
  .services["cpa-manager"].networks.sub2api.aliases == ["cpa-manager"] and
  (.services["cpa-gateway"].volumes | map(.target) | sort) == [
    "/CLIProxyAPI/config.yaml",
    "/CLIProxyAPI/logs",
    "/CLIProxyAPI/plugins",
    "/root/.cli-proxy-api",
    "/var/lib/cliproxy-ticket-gateway"
  ] and
  (.services["cpa-manager"].volumes | map(.target)) == ["/data"] and
  .services["cpa-gateway"].healthcheck.test[0] == "CMD-SHELL" and
  (.services["cpa-gateway"].healthcheck.test[1] | contains("/usr/bin/bash") and contains("/dev/tcp/127.0.0.1/8317") and contains(" 200 ")) and
  (.services["cpa-gateway"].healthcheck.test[1] | contains("wget") or contains("curl") | not) and
  .services["cpa-manager"].healthcheck.test[0] == "CMD" and
  .services["cpa-manager"].secrets[0].source == "cpamp_admin_key"
' <<<"$compose_json" >/dev/null || fail 'Compose isolation contract failed'

if rg -n '^[[:space:]]+ports:' "$compose_file"; then
  fail 'CPA services must not publish host ports'
fi

for expected in \
  'cpa.xingqiaolab.top {' \
  '@cpa_public_inference path /v1 /v1/* /v1beta /v1beta/* /responses /chat/completions /messages' \
  'respond @cpa_public_inference 404' \
  '@cpa_usage path /usage-service/*' \
  'reverse_proxy @cpa_usage cpa-manager:18317' \
  'reverse_proxy cpa-gateway:8317'; do
  rg -Fq "$expected" "$caddy_file" || fail "missing Caddy contract: $expected"
done

docker run --rm -e SITE_ADDRESS=api.example.com \
  -v "$caddy_file:/etc/caddy/Caddyfile:ro" caddy:2.10.2-alpine \
  caddy validate --config /etc/caddy/Caddyfile --adapter caddyfile >/dev/null

printf 'CPA production infrastructure contract: PASS\n'
