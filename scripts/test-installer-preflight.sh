#!/usr/bin/env bash
# Regression test: the installer's read-only look at the server (install.sh --dry-run) must finish and exit 0 on every
# kind of nginx layout, and must never stop silently. (A server with sites but no default_server used to make it quit
# right after printing the nginx line.) Uses a fake /etc/nginx in a temp folder; changes nothing.
set -uo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
T="$(mktemp -d)"; trap 'rm -rf "$T"' EXIT
pass=0; failn=0
check() { if [[ "$2" == "$3" ]]; then echo "  ok   $1"; pass=$((pass+1)); else echo "  FAIL $1 (wanted '$3', got '$2')"; failn=$((failn+1)); fi; }

scenario() { # name, then lines: file|content ; builds a fake nginx tree, runs the dry run
    local name="$1"; shift
    local N="$T/$name/nginx"; mkdir -p "$N/sites-available" "$N/sites-enabled" "$N/conf.d"
    local spec f c
    for spec in "$@"; do f="${spec%%|*}"; c="${spec#*|}"; printf '%s\n' "$c" > "$N/$f"; done
    for f in "$N"/sites-available/*; do [[ -e "$f" ]] && ln -sf "$f" "$N/sites-enabled/$(basename "$f")"; done
    sed "s#/etc/nginx#$N#g" "$ROOT/install.sh" > "$T/$name/install.sh"
    cp "$ROOT/VERSION" "$T/$name/VERSION"
    out="$(cd "$T/$name" && TERM=dumb bash install.sh --dry-run 2>&1 | sed 's/\x1b\[[0-9;]*m//g')"; rc=${PIPESTATUS[0]}
}

echo "1. sites but no default_server (the case that used to quit silently)"
scenario nodefault 'sites-available/a.conf|server { listen 80; server_name a.example.com; }' 'sites-available/b.conf|server { listen 80; server_name b.example.com; }'
check "exits 0" "$rc" "0"
check "reaches the end" "$(grep -c 'Run without --dry-run to install' <<<"$out")" "1"
check "no false alarm" "$(grep -c 'stopped unexpectedly' <<<"$out")" "0"

echo "2. an existing default_server"
scenario withdefault 'conf.d/zz-default.conf|server { listen 80 default_server; return 444; }' 'sites-available/a.conf|server { listen 80; server_name a.example.com; }'
check "exits 0" "$rc" "0"
check "says it will not add another" "$(grep -c 'default_server already exists' <<<"$out")" "1"
check "reaches the end" "$(grep -c 'Run without --dry-run to install' <<<"$out")" "1"

echo "3. no nginx sites at all"
scenario empty
check "exits 0" "$rc" "0"
check "reaches the end" "$(grep -c 'Run without --dry-run to install' <<<"$out")" "1"

echo "4. an unexpected failure is reported, not silent"
sed "s#/etc/nginx#$T/x#g; s#^preflight() {#preflight() { false;#" "$ROOT/install.sh" > "$T/boom.sh"; cp "$ROOT/VERSION" "$T/"
out="$(cd "$T" && TERM=dumb bash boom.sh --dry-run 2>&1 | sed 's/\x1b\[[0-9;]*m//g')"
check "says where it stopped" "$(grep -c 'stopped unexpectedly at line' <<<"$out")" "1"

echo; echo "$pass passed, $failn failed"; [[ $failn -eq 0 ]]
