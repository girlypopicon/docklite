#!/usr/bin/env bash
# DockLite server inventory — READ-ONLY.
#
# Reports what is running on a server before DockLite is installed or
# upgraded on it: DockLite installs (old and new), every Docker container
# classified by how DockLite relates to it, nginx sites mapped to the
# ports/containers they proxy to, web apps running outside Docker, and
# unused Docker resources. It changes nothing.
#
# Usage: sudo bash inventory.sh            (root is needed to read nginx
#                                           configs and see all processes)
set -uo pipefail

if [[ -t 1 ]]; then
    B='\033[1m'; D='\033[2m'; R='\033[0;31m'; Y='\033[0;33m'; G='\033[0;32m'; C='\033[0;36m'; N='\033[0m'
else
    B=''; D=''; R=''; Y=''; G=''; C=''; N=''
fi
section() { echo -e "\n${B}${C}== $* ==${N}"; }
note()    { echo -e "  ${D}$*${N}"; }
warn()    { echo -e "  ${Y}!${N} $*"; }
bad()     { echo -e "  ${R}✗${N} $*"; }
good()    { echo -e "  ${G}✓${N} $*"; }

# Label/env values that look like credentials are never printed.
redact() { sed -E 's/((password|passwd|secret|token|key)[^=:]*[=:])[^ ,}]*/\1***/Ig'; }

[[ $EUID -ne 0 ]] && warn "Not running as root: nginx configs and some processes may be hidden. Use: sudo bash $0"

section "Host"
echo "  hostname: $(hostname)   public IP: $(curl -s -m 5 https://ifconfig.me 2>/dev/null || echo '?')"
echo "  $(. /etc/os-release 2>/dev/null; echo "${PRETTY_NAME:-unknown OS}")   uptime since $(uptime -s 2>/dev/null)"
echo "  disk /: $(df -h / | awk 'NR==2{print $3" used of "$2" ("$5")"}')   memory: $(free -h | awk '/Mem:/{print $3" used of "$2}')"

# ─────────────────────────────────────────────────────────────────────────────
section "DockLite installs"
found_install=0
for dir in /opt/docklite /home/*/docklite* /home/docklite* /home/DOCKLITE* /root/docklite*; do
    [[ -d "$dir" ]] || continue
    [[ -f "$dir/VERSION" || -d "$dir/go-app" || -d "$dir/webapp" || -f "$dir/docklite" || -d "$dir/.next" ]] || continue
    found_install=1
    ver=$(cat "$dir/VERSION" 2>/dev/null || echo "?")
    kind="checkout"; [[ "$dir" == /opt/docklite ]] && kind="installed"
    remote=$(git -C "$dir" remote get-url origin 2>/dev/null || git -C "$dir" remote -v 2>/dev/null | awk 'NR==1{print $2}')
    db=""; for f in "$dir/data/docklite.db" "$dir/docklite.db"; do [[ -f "$f" ]] && db="$f"; done
    echo -e "  ${B}${dir}${N}  (${kind}, version ${ver})${remote:+  remote: ${remote}}"
    if [[ -n "$db" ]] && command -v sqlite3 >/dev/null; then
        sites=$(sqlite3 -readonly "$db" "SELECT COUNT(*) FROM sites" 2>/dev/null || echo "?")
        users=$(sqlite3 -readonly "$db" "SELECT COUNT(*) FROM users" 2>/dev/null || echo "?")
        note "database: ${db} — ${sites} site record(s), ${users} user(s)"
    fi
done
[[ $found_install -eq 0 ]] && note "no DockLite directories found"
for unit in docklite docklite-web docklite-agent pm2-root; do
    if systemctl list-unit-files "${unit}.service" 2>/dev/null | grep -q "^${unit}.service"; then
        echo "  systemd ${unit}.service: $(systemctl is-enabled "$unit" 2>/dev/null) / $(systemctl is-active "$unit" 2>/dev/null)"
    fi
done
[[ -d /etc/docklite ]] && note "/etc/docklite exists (old systemd-style install config): $(ls /etc/docklite | tr '\n' ' ')"
[[ -f /etc/sudoers.d/docklite ]] && note "/etc/sudoers.d/docklite exists"
[[ -x /usr/local/sbin/docklite-helper ]] && note "docklite-helper installed (DockLite ≥ 1.0.2)"
if command -v pm2 >/dev/null; then
    for home in /root /home/*; do
        [[ -d "$home/.pm2" ]] || continue
        owner=$(stat -c %U "$home")
        list=$(HOME="$home" PM2_HOME="$home/.pm2" pm2 jlist 2>/dev/null | python3 -c 'import sys,json
try: print(", ".join(f"{p[\"name\"]}({p[\"pm2_env\"][\"status\"]})" for p in json.load(sys.stdin)))
except Exception: pass' 2>/dev/null)
        [[ -n "$list" ]] && echo "  PM2 (${owner}): ${list}"
    done
fi

# ─────────────────────────────────────────────────────────────────────────────
section "nginx sites"
declare -A PORT_DOMAINS=()     # host port -> domains proxied to it
NGINX_DIRS=(/etc/nginx/sites-enabled /etc/nginx/conf.d)
if ! command -v nginx >/dev/null; then
    note "nginx is not installed"
else
    echo "  $(nginx -v 2>&1)   service: $(systemctl is-active nginx 2>/dev/null)"
    includes=$(grep -hE '^\s*include\s+.*(sites-enabled|conf\.d)' /etc/nginx/nginx.conf 2>/dev/null | sed 's/^\s*//' | tr '\n' ' ')
    note "nginx.conf includes: ${includes:-?}"
    for dir in "${NGINX_DIRS[@]}"; do
        [[ -d "$dir" ]] || continue
        for f in "$dir"/*; do
            [[ -e "$f" ]] || continue
            name=$(basename "$f")
            loaded="loaded"
            if [[ "$dir" == */sites-enabled && "$includes" == *"sites-enabled/*.conf"* && "$name" != *.conf ]]; then
                loaded="NOT LOADED (no .conf suffix)"
            fi
            target=""; [[ -L "$f" ]] && target=" -> $(readlink "$f")"
            echo -e "  ${B}${dir}/${name}${N}${target}  ${D}[${loaded}]${N}"
            # One line per server block: names, listens, upstreams/root.
            awk '
                /^[[:space:]]*#/ { next }
                /server[[:space:]]*\{/ { depth_at_server = depth + 1; in_srv = 1; names=""; listens=""; ups=""; root="" }
                { line=$0
                  if (in_srv) {
                    if (match(line, /server_name[[:space:]]+[^;]+/)) { v=substr(line,RSTART+12,RLENGTH-12); gsub(/^[[:space:]]+/,"",v); names=names" "v }
                    if (match(line, /listen[[:space:]]+[^;]+/))      { v=substr(line,RSTART+7,RLENGTH-7);  gsub(/^[[:space:]]+/,"",v); listens=listens" ["v"]" }
                    if (match(line, /proxy_pass[[:space:]]+[^;]+/))  { v=substr(line,RSTART+11,RLENGTH-11); gsub(/^[[:space:]]+/,"",v); ups=ups" "v }
                    if (match(line, /(^|[[:space:]])root[[:space:]]+[^;]+/)) { v=line; sub(/.*root[[:space:]]+/,"",v); sub(/;.*/,"",v); root=v }
                  }
                  n=gsub(/\{/,"{",line); m=gsub(/\}/,"}",line); depth += n - m
                  if (in_srv && depth < depth_at_server) {
                    printf "    server%s  listen%s  -> %s\n", (names==""?" (no name)":names), listens, (ups!=""?ups:(root!=""?"root "root:"(no upstream)"))
                    in_srv = 0
                  }
                }' "$f" 2>/dev/null
            if [[ "$loaded" == loaded ]]; then
                while read -r port; do
                    names=$(awk '/server_name/{sub(/.*server_name[[:space:]]+/,""); sub(/;.*/,""); print}' "$f" | head -1)
                    PORT_DOMAINS[$port]="${PORT_DOMAINS[$port]:-}${PORT_DOMAINS[$port]:+, }${names:-$name}"
                done < <(grep -oE 'proxy_pass[[:space:]]+https?://(127\.0\.0\.1|localhost|0\.0\.0\.0)(:[0-9]+)' "$f" 2>/dev/null | grep -oE '[0-9]+$' | sort -u)
            fi
        done
    done
    defaults=$(grep -lE 'listen[^;]*default_server' "${NGINX_DIRS[@]/%//*}" 2>/dev/null | tr '\n' ' ')
    note "default_server defined in: ${defaults:-none}"
    if nginx -t >/dev/null 2>&1; then good "nginx -t passes"; else bad "nginx -t FAILS: $(nginx -t 2>&1 | grep -m1 emerg)"; fi
    # Upstream ports with nothing listening behind them.
    for port in "${!PORT_DOMAINS[@]}"; do
        ss -tln "( sport = :$port )" 2>/dev/null | grep -q LISTEN || bad "dead upstream :$port (nothing listening) — used by ${PORT_DOMAINS[$port]}"
    done
fi

# ─────────────────────────────────────────────────────────────────────────────
section "Docker containers"
if ! command -v docker >/dev/null || ! docker info >/dev/null 2>&1; then
    note "Docker not available"
else
    db=""; [[ -f /opt/docklite/data/docklite.db ]] && db=/opt/docklite/data/docklite.db
    note "classes: MANAGED = DockLite site record points at it · OLD-DOCKLITE = DockLite labels/name but no record · FOREIGN = not created by DockLite"
    while IFS='|' read -r id name image state status; do
        labels=$(docker inspect -f '{{range $k,$v := .Config.Labels}}{{$k}}={{$v}} {{end}}' "$id" 2>/dev/null)
        # Live bindings when running; configured ones (port 0/empty = Docker
        # picks a free port at start) when stopped.
        if [[ "$state" == running ]]; then
            ports=$(docker inspect -f '{{range $p,$b := .NetworkSettings.Ports}}{{range $b}}{{if ne .HostIp "::"}}{{.HostIp}}:{{.HostPort}}->{{$p}} {{end}}{{end}}{{end}}' "$id" 2>/dev/null)
        else
            ports=$(docker inspect -f '{{range $p,$b := .HostConfig.PortBindings}}{{range $b}}{{if .HostIp}}{{.HostIp}}{{else}}0.0.0.0{{end}}:{{.HostPort}}->{{$p}} {{end}}{{end}}' "$id" 2>/dev/null \
                | sed -E 's/:(0)?->/:auto->/g')
        fi
        mounts=$(docker inspect -f '{{range .Mounts}}{{if eq .Type "bind"}}{{.Source}}{{else}}vol:{{.Name}}{{end}}:{{.Destination}} {{end}}' "$id" 2>/dev/null)
        restart=$(docker inspect -f '{{.HostConfig.RestartPolicy.Name}}' "$id" 2>/dev/null)
        cls="FOREIGN"
        if [[ -n "$db" ]] && command -v sqlite3 >/dev/null && \
           [[ -n "$(sqlite3 -readonly "$db" "SELECT id FROM sites WHERE container_id LIKE '${id}%' LIMIT 1" 2>/dev/null)" ]]; then
            cls="MANAGED"
        elif [[ "$labels" == *docklite.* || "$name" == docklite* ]]; then
            cls="OLD-DOCKLITE"
        fi
        domains=""
        for hp in $(grep -oE ':[0-9]+->' <<<"$ports" | tr -d ':->'); do
            [[ -n "${PORT_DOMAINS[$hp]:-}" ]] && domains="${domains}${domains:+, }${PORT_DOMAINS[$hp]}"
        done
        hint=$(grep -oE '(docklite\.domain|VIRTUAL_HOST|traefik\.http\.routers\.[^.]+\.rule|caddy)=[^ ]+' <<<"$labels" | head -2 | tr '\n' ' ')
        col=$G; [[ "$state" != running ]] && col=$D
        echo -e "  ${col}${B}${name}${N} ${D}(${id:0:12})${N}  ${cls}  ${state} — ${status}"
        echo "      image: ${image}   restart: ${restart:-no}"
        [[ -n "$ports" ]]   && echo "      ports: ${ports}"
        [[ -n "$mounts" ]]  && echo "      mounts: ${mounts}"
        [[ -n "$domains" ]] && echo "      served by nginx for: ${domains}"
        [[ -n "$hint" ]]    && echo "      labels: $(redact <<<"$hint")"
        [[ "$ports" == *0.0.0.0:* ]] && note "    published on all interfaces (reachable without nginx)"
    done < <(docker ps -a --format '{{.ID}}|{{.Names}}|{{.Image}}|{{.State}}|{{.Status}}')

    if [[ -n "$db" ]] && command -v sqlite3 >/dev/null; then
        while IFS='|' read -r sid domain cid; do
            [[ -z "$cid" ]] && { warn "site record #${sid} ${domain}: no container"; continue; }
            docker inspect "$cid" >/dev/null 2>&1 || bad "GHOST site record #${sid} ${domain}: container ${cid:0:12} no longer exists"
        done < <(sqlite3 -readonly "$db" "SELECT id, domain, IFNULL(container_id,'') FROM sites" 2>/dev/null)
    fi

    section "Docker resources"
    echo "  $(docker system df --format '{{.Type}}: {{.Size}} ({{.Reclaimable}} reclaimable)' 2>/dev/null | paste -sd'   ')"
    dangling=$(docker images -f dangling=true -q | wc -l)
    unused_vols=$(docker volume ls -qf dangling=true)
    note "dangling images: ${dangling}"
    if [[ -n "$unused_vols" ]]; then
        warn "volumes not attached to any container (may hold database data — never prune blindly):"
        for v in $unused_vols; do echo "      $v  ($(docker system df -v --format '{{range .Volumes}}{{if eq .Name "'"$v"'"}}{{.Size}}{{end}}{{end}}' 2>/dev/null))"; done
    fi
    for net in $(docker network ls --format '{{.Name}}' | grep -vE '^(bridge|host|none)$'); do
        n=$(docker network inspect -f '{{len .Containers}}' "$net" 2>/dev/null)
        echo "  network ${net}: ${n} container(s) attached"
    done
fi

# ─────────────────────────────────────────────────────────────────────────────
section "Web apps outside Docker"
note "listening TCP ports, excluding Docker's own proxies"
ss -tlnpH 2>/dev/null | grep -v docker-proxy | awk '{print $4, $6}' | sort -u | while read -r addr proc; do
    p=$(grep -oE '"[^"]+"' <<<"$proc" | head -1 | tr -d '"')
    pid=$(grep -oE 'pid=[0-9]+' <<<"$proc" | head -1 | cut -d= -f2)
    user=""; cwd=""
    if [[ -n "$pid" ]]; then
        user=$(ps -o user= -p "$pid" 2>/dev/null)
        cwd=$(readlink "/proc/$pid/cwd" 2>/dev/null)
    fi
    port=${addr##*:}
    tag=""; [[ -n "${PORT_DOMAINS[$port]:-}" ]] && tag="  <- nginx: ${PORT_DOMAINS[$port]}"
    printf "  %-24s %-14s %-10s %s%s\n" "$addr" "${p:-?}" "${user:-?}" "${cwd}" "$tag"
done

# ─────────────────────────────────────────────────────────────────────────────
section "Site folders"
for base in /var/www/sites /var/www; do
    [[ -d "$base" ]] || continue
    echo -e "  ${B}${base}${N}  ($(stat -c '%U:%G %a' "$base"))"
    find "$base" -mindepth 1 -maxdepth 2 -type d 2>/dev/null | sort | head -60 | while read -r d; do
        mark=""; [[ -f "$d/.dkl" ]] && mark="  [.dkl manifest]"
        [[ -f "$d/package.json" ]] && mark="${mark}  [node]"
        [[ -f "$d/index.php" || -f "$d/composer.json" ]] && mark="${mark}  [php]"
        printf "    %-60s %s%s\n" "$d" "$(stat -c '%U:%G' "$d")" "$mark"
    done
    break
done

echo ""
echo -e "${D}Inventory complete — nothing was changed.${N}"
