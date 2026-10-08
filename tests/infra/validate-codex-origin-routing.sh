#!/usr/bin/env bash
set -euo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
cd "$ROOT"

fail() { printf 'FAIL: %s\n' "$1" >&2; exit 1; }
require() { rg -Fq -- "$1" "$2" || fail "missing $1 in $2"; }

CADDY=infra/Caddyfile

require 'codex.xingqiaolab.top, 64-83-10-67.nip.io {' "$CADDY"
require 'Alt-Svc "clear"' "$CADDY"
require 'reverse_proxy 172.18.0.1:18087 {' "$CADDY"
require 'header_up X-Forwarded-Proto {scheme}' "$CADDY"
require 'header_up X-Forwarded-Host {host}' "$CADDY"
require 'header_down Alt-Svc "clear"' "$CADDY"
require 'response_header_timeout 15m' "$CADDY"

printf 'PASS: codex origin routing contract\n'
