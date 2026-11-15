#!/usr/bin/env bash
# Sandbox test for the root helper's nginx commands (nginx-upstreams, nginx-fix-port, nginx-down-write, nginx-down-patch):
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
sed -e "s#/etc/nginx#$N#g" -e "s#/var/lib/docklite#$T/lib#g" -e "s#/var/backups/docklite#$T/backups#g" -e 's#chown --reference="$file" "$tmp"#true#' "$HELPER" > "$H"
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

echo "4. the stopped-box page and its snippet"
PAGE='<html>this box is stopped</html>'
touch "$T/reject"
out="$(printf '%s' "$PAGE" | run nginx-down-write)"
check "a snippet nginx rejects is undone"   "$(grep -c 'undone' <<<"$out")" "1"
check "and left no snippet behind"           "$([ -e "$N/snippets/docklite-down.conf" ] && echo present || echo none)" "none"
rm -f "$T/reject"; : > "$T/reloads"
out="$(printf '%s' "$PAGE" | run nginx-down-write)"; check "writes the page" "${out%%:*}" "ok"
check "page content"                         "$(cat "$T/lib/down.html")" "$PAGE"
check "page is world-readable"               "$(stat -c %a "$T/lib/down.html")" "644"
check "snippet swaps the 502"                "$(grep -c 'error_page 502 =503 /__docklite_down;' "$N/snippets/docklite-down.conf")" "1"
check "snippet page can't be reached directly" "$(grep -c 'internal;' "$N/snippets/docklite-down.conf")" "1"
check "nginx was reloaded for the new snippet" "$(wc -l < "$T/reloads" | tr -d ' ')" "1"
printf '%s' "$PAGE v2" | run nginx-down-write >/dev/null
check "new wording: page replaced"           "$(cat "$T/lib/down.html")" "$PAGE v2"
check "unchanged snippet: no second reload"  "$(wc -l < "$T/reloads" | tr -d ' ')" "1"
out="$(printf '' | run nginx-down-write)";   check "empty page refused" "$(grep -c 'empty or over' <<<"$out")" "1"
out="$(head -c 70000 /dev/zero | tr '\0' x | run nginx-down-write)"; check "page over 64 KB refused" "$(grep -c 'empty or over' <<<"$out")" "1"
check "refused pages left the page alone"    "$(cat "$T/lib/down.html")" "$PAGE v2"

echo "5. adding the include to a site"
INC="include $N/snippets/docklite-down*.conf;"
scan="$(run nginx-upstreams)"
check "scan shows blocks without it as '-'"  "$(grep blog.example.com <<<"$scan" | cut -f4)" "-"
cat > "$N/conf.d/redir.conf" <<'CONF'
server {
    listen 80;
    server_name redir.example.com;
    return 301 https://redir.example.com$request_uri;
}
CONF
cat > "$N/conf.d/own.conf" <<'CONF'
server {
    server_name own.example.com;
    error_page 502 /my-own.html;
    location / { proxy_pass http://127.0.0.1:32010; }
}
CONF
cat > "$N/conf.d/oneline.conf" <<'CONF'
server { server_name one.example.com; location / { proxy_pass http://127.0.0.1:32011; } }
CONF
cat > "$N/conf.d/panel.conf" <<'CONF'
server {
    server_name panel.example.com;
    location / { proxy_pass http://127.0.0.1:3000; }
}
CONF
before_panel="$(cat "$N/conf.d/panel.conf")"
out="$(run nginx-down-patch "$N/sites-available/blog.example.com" blog.example.com)"; check "reports ok" "${out%%:*}" "ok"
check "include sits right after server_name, same indent" "$(grep -A1 'server_name blog' "$N/sites-available/blog.example.com" | tail -1)" "    $INC"
check "nginx was reloaded"                   "$(wc -l < "$T/reloads" | tr -d ' ')" "2"
check "scan now shows it as down"            "$(run nginx-upstreams | grep blog.example.com | cut -f4)" "down"
out="$(run nginx-down-patch "$N/sites-available/blog.example.com" blog.example.com)"; check "second run: nothing to change" "$(grep -c 'nothing to change' <<<"$out")" "1"
check "still one include (not doubled)"      "$(grep -c 'docklite-down' "$N/sites-available/blog.example.com")" "1"
run nginx-down-patch "$N/sites-available/docklite-sites" a.example.com >/dev/null
check "both blocks naming the domain get it"  "$(grep -c 'docklite-down' "$N/sites-available/docklite-sites")" "2"
check "other sites in the file are left alone" "$(grep -B3 'proxy_pass http://127.0.0.1:32044' "$N/sites-available/docklite-sites" | grep -c 'docklite-down')" "0"
out="$(run nginx-down-patch "$N/conf.d/redir.conf" redir.example.com)"; check "a redirect-only block is left alone" "$(grep -c 'nothing to change' <<<"$out")" "1"
out="$(run nginx-down-patch "$N/conf.d/own.conf" own.example.com)";     check "a block with its own 502 page is left alone" "$(grep -c 'nothing to change' <<<"$out")" "1"
out="$(run nginx-down-patch "$N/conf.d/oneline.conf" one.example.com)"; check "a one-line block is left alone (no safe place)" "$(grep -c 'nothing to change' <<<"$out")" "1"
out="$(run nginx-down-patch "$N/conf.d/evrqr.conf" nobody.example.com)"; check "a domain the file doesn't name: nothing to change" "$(grep -c 'nothing to change' <<<"$out")" "1"
check "the panel's block was never touched"  "$(cat "$N/conf.d/panel.conf")" "$before_panel"
before="$(cat "$N/conf.d/evrqr.conf")"; touch "$T/reject"
out="$(run nginx-down-patch "$N/conf.d/evrqr.conf" evrqr.example.com)"
check "nginx -t failing undoes the patch"    "$(cat "$N/conf.d/evrqr.conf")" "$before"
check "and says so"                          "$(grep -c 'undone' <<<"$out")" "1"
rm -f "$T/reject"
for bad in "/etc/passwd blog.example.com" "$N/../../x blog.example.com" "$N/sites-available/blog.example.com bad;domain"; do
    run nginx-down-patch $bad >/dev/null; r=$?
    check "rejects: ${bad:0:60}" "$([ $r -ne 0 ] && echo rejected || echo ACCEPTED)" "rejected"
done
echo; echo "$pass passed, $failn failed"; [[ $failn -eq 0 ]]
