#!/usr/bin/env bash
# Sandbox test for the root helper's nginx upstream commands (nginx-upstreams, nginx-fix-port):
# fake /etc/nginx tree, fake `nginx` that can reject a config, no root, nothing outside a temp folder.
set -uo pipefail
HELPER="$(cd "$(dirname "$0")/.." && pwd)/docklite-helper"
T="$(mktemp -d)"; trap 'rm -rf "$T"' EXIT
pass=0; failn=0
check() { if [[ "$2" == "$3" ]]; then echo "  ok   $1"; pass=$((pass+1)); else echo "  FAIL $1 (wanted '$3', got '$2')"; failn=$((failn+1)); fi; }

N="$T/etc/nginx"; mkdir -p "$N/sites-available" "$N/sites-enabled" "$N/conf.d" "$T/bin" "$T/backups"
cat > "$N/sites-available/blog.example.com" <<'CONF'
server {
    listen 80;
    server_name blog.example.com www.blog.example.com;
    location / { proxy_pass http://127.0.0.1:32001; }
}
CONF
ln -s "$N/sites-available/blog.example.com" "$N/sites-enabled/blog.example.com"
cat > "$N/conf.d/evrqr.conf" <<'CONF'
# a hand-written file in conf.d, not made by DockLite
server {
    server_name evrqr.example.com;   # trailing comment with a { brace
    location / { proxy_pass http://127.0.0.1:32002/; }
}
CONF
cat > "$N/sites-available/docklite-sites" <<'CONF'
server {
    listen 80;
    server_name a.example.com;
    location / { proxy_pass http://127.0.0.1:32003; }
}
server {
    listen 443 ssl;
    server_name a.example.com;
    location / { proxy_pass http://127.0.0.1:32003; }
}
server {
    listen 80;
    server_name b.example.com;
    location /api { proxy_pass http://127.0.0.1:32004; }
    location / { proxy_pass http://127.0.0.1:32044; }
}
server {
    server_name c.example.com;
    location / { proxy_pass http://127.0.0.1:320030; }
}
CONF
ln -s "$N/sites-available/docklite-sites" "$N/sites-enabled/docklite-sites"
# fake nginx: -t fails while the marker file exists; reloads are logged
cat > "$T/bin/nginx" <<NGX
#!/usr/bin/env bash
if [[ "\$1" == "-t" ]]; then [[ -e "$T/reject" ]] && { echo "nginx: [emerg] boom" >&2; exit 1; }; exit 0; fi
[[ "\$1" == "-s" && "\$2" == "reload" ]] && echo reload >> "$T/reloads"
exit 0
NGX
chmod +x "$T/bin/nginx"; export PATH="$T/bin:$PATH"

H="$T/helper.sh"
sed -e "s#/etc/nginx#$N#g" -e "s#/var/backups/docklite#$T/backups#g" -e 's#chown --reference="$file" "$tmp"#true#' "$HELPER" > "$H"
run() { bash "$H" "$@" 2>&1; }

echo "1. scanning every server block, including conf.d and multi-site files"
scan="$(run nginx-upstreams)"
check "finds the normal site"        "$(grep -c 'blog.example.com' <<<"$scan")" "1"
check "finds the conf.d file"        "$(grep -c 'evrqr.conf' <<<"$scan")" "1"
check "lists each block of the multi-site file" "$(grep -c 'docklite-sites' <<<"$scan")" "4"
check "reads the port of a symlinked site" "$(grep blog.example.com <<<"$scan" | cut -f3 | tr -d ' ')" "32001"
check "sees both upstreams of a split block" "$(grep 'b.example.com' <<<"$scan" | cut -f3 | tr -s ' ' | sed 's/^ //')" "32004 32044"
check "ignores braces in comments"   "$(grep evrqr <<<"$scan" | cut -f3 | tr -d ' ')" "32002"
check "no duplicate for symlink + target" "$(grep -c 'blog.example.com' <<<"$scan")" "1"

echo "2. fixing a port"
out="$(run nginx-fix-port "$N/sites-available/blog.example.com" blog.example.com 32001 40001)"; check "reports ok" "${out%%:*}" "ok"
check "port changed" "$(grep -c '40001' "$N/sites-available/blog.example.com")" "1"
check "nginx was reloaded" "$(wc -l < "$T/reloads" | tr -d ' ')" "1"
check "a backup was kept" "$(ls "$T"/backups/nginx-blog.example.com-*.bak | wc -l | tr -d ' ')" "1"
run nginx-fix-port "$N/conf.d/evrqr.conf" evrqr.example.com 32002 40002 >/dev/null
check "works in conf.d and keeps the trailing slash" "$(grep -c 'proxy_pass http://127.0.0.1:40002/;' "$N/conf.d/evrqr.conf")" "1"
run nginx-fix-port "$N/sites-available/docklite-sites" a.example.com 32003 40003 >/dev/null
check "fixes both blocks naming the domain" "$(grep -c '40003' "$N/sites-available/docklite-sites")" "2"
check "leaves other sites in the same file alone" "$(grep -c '32004' "$N/sites-available/docklite-sites")" "1"
check "never matches a longer port (32003 vs 320030)" "$(grep -c '320030' "$N/sites-available/docklite-sites")" "1"
run nginx-fix-port "$N/sites-available/docklite-sites" b.example.com 32004 40004 >/dev/null
check "changes only the named upstream of a split block" "$(grep -c '32044' "$N/sites-available/docklite-sites")" "1"

echo "3. refusing / undoing"
before="$(cat "$N/sites-available/blog.example.com")"
out="$(run nginx-fix-port "$N/sites-available/blog.example.com" blog.example.com 45678 40009)"; check "wrong old port: nothing to change" "$(grep -c 'nothing to change' <<<"$out")" "1"
touch "$T/reject"
out="$(run nginx-fix-port "$N/sites-available/blog.example.com" blog.example.com 40001 40010)"
check "nginx -t failing undoes the change" "$(cat "$N/sites-available/blog.example.com")" "$before"
check "says so" "$(grep -c 'undone' <<<"$out")" "1"
rm -f "$T/reject"
for bad in "/etc/passwd blog.example.com 1 40011" "$N/../../x blog.example.com 40001 40011" "$N/sites-available/blog.example.com bad;domain 40001 40011" "$N/sites-available/blog.example.com blog.example.com 40001 80" "$N/sites-available/blog.example.com blog.example.com abc 40011" "$N/sites-available/blog.example.com blog.example.com 40001 70000"; do
    run nginx-fix-port $bad >/dev/null; r=$?
    check "rejects: ${bad:0:60}" "$([ $r -ne 0 ] && echo rejected || echo ACCEPTED)" "rejected"
done
echo; echo "$pass passed, $failn failed"; [[ $failn -eq 0 ]]
