#!/usr/bin/env bash
# DockLite Uninstaller / Cleanup
# Removes all DockLite services, configs, and optionally data.
# Usage: sudo bash uninstall.sh
set -euo pipefail

# ── Colors ────────────────────────────────────────────────────────────────────
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'
CYAN='\033[0;36m'; BOLD='\033[1m'; DIM='\033[2m'; NC='\033[0m'
PINK='\033[38;2;255;106;213m';   CORAL='\033[38;2;255;154;139m'
GOLD='\033[38;2;255;209;102m';   MINT='\033[38;2;184;242;162m'
SKY='\033[38;2;154;208;255m';    LAVENDER='\033[38;2;199;164;255m'

ok()   { echo -e "  ${GREEN}✓${NC} $*"; }
warn() { echo -e "  ${YELLOW}⚠${NC}  $*"; }
skip() { echo -e "  ${DIM}  skipped ($*)${NC}"; }
info() { echo -e "  ${DIM}$*${NC}"; }

REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SUDO=""
[[ "${EUID}" -ne 0 ]] && SUDO="sudo"

rainbow_line() {
    local chars="" i=0
    local colors=("$PINK" "$CORAL" "$GOLD" "$MINT" "$SKY" "$LAVENDER")
    while [[ $i -lt 50 ]]; do
        chars+="${colors[$((i % 6))]}━"
        i=$((i + 1))
    done
    echo -e "${chars}${NC}"
}

# ══════════════════════════════════════════════════════════════════════════════
clear
echo ""
echo -e "${PINK}     ____             __   __    _ __       ${NC}"
echo -e "${CORAL}    / __ \\____  _____/ /__/ /   (_) /_____ ${NC}"
echo -e "${GOLD}   / / / / __ \\/ ___/ //_/ /   / / __/ _ \\${NC}"
echo -e "${MINT}  / /_/ / /_/ / /__/ ,< / /___/ / /_/  __/${NC}"
echo -e "${SKY}  \\____/\\____/\\___/_/|_/_____/_/\\__/\\___/ ${NC}"
echo -e "${LAVENDER}                             uninstaller${NC}"
echo ""
rainbow_line
echo ""
echo -e "  ${BOLD}This will clean up all DockLite services and configs.${NC}"
echo ""

# ══════════════════════════════════════════════════════════════════════════════
# Scan what exists
# ══════════════════════════════════════════════════════════════════════════════

echo -e "  ${BOLD}${CYAN}Scanning...${NC}"
echo ""

found_items=()

# PM2 processes
if command -v pm2 >/dev/null 2>&1; then
    pm2_agent=$(pm2 jlist 2>/dev/null | python3 -c "
import sys,json
try:
    ps=json.load(sys.stdin)
    for p in ps:
        if 'docklite' in p.get('name',''):
            print(p['name'])
except: pass
" 2>/dev/null || true)
    if [[ -n "$pm2_agent" ]]; then
        ok "PM2 processes: ${pm2_agent//$'\n'/, }"
        found_items+=("pm2")
    else
        skip "no PM2 docklite processes"
    fi
else
    skip "PM2 not installed"
fi

# Systemd services
systemd_services=()
for svc in docklite-agent docklite-web docklite; do
    if [[ -f "/etc/systemd/system/${svc}.service" ]]; then
        systemd_services+=("$svc")
    fi
done
if [[ ${#systemd_services[@]} -gt 0 ]]; then
    ok "Systemd services: ${systemd_services[*]}"
    found_items+=("systemd")
else
    skip "no systemd services"
fi

# Running processes (not PM2-managed)
old_procs=$(ps aux 2>/dev/null | grep -E 'docklite-agent|next.*docklite|next.*opt/docklite' | grep -v grep | awk '{print $2, $11}' || true)
if [[ -n "$old_procs" ]]; then
    ok "Running processes found"
    echo "$old_procs" | while read -r pid cmd; do
        info "  PID ${pid}: ${cmd}"
    done
    found_items+=("processes")
else
    skip "no stray processes"
fi

# Nginx configs
nginx_configs=()
for f in /etc/nginx/sites-available/docklite* /etc/nginx/sites-enabled/docklite*; do
    [[ -e "$f" ]] && nginx_configs+=("$f")
done
if [[ ${#nginx_configs[@]} -gt 0 ]]; then
    ok "Nginx configs: ${#nginx_configs[@]} files"
    for f in "${nginx_configs[@]}"; do info "  $f"; done
    found_items+=("nginx")
else
    skip "no nginx configs"
fi

# Sudoers
if [[ -f /etc/sudoers.d/docklite-nginx ]]; then
    ok "Sudoers rule: /etc/sudoers.d/docklite-nginx"
    found_items+=("sudoers")
else
    skip "no sudoers rules"
fi

# Symlink
if [[ -L /usr/local/bin/docklite ]]; then
    ok "Symlink: /usr/local/bin/docklite"
    found_items+=("symlink")
else
    skip "no /usr/local/bin/docklite symlink"
fi

# Old install at /opt/docklite
if [[ -d /opt/docklite ]]; then
    local_size=$(du -sh /opt/docklite 2>/dev/null | cut -f1)
    ok "Old installation: /opt/docklite (${local_size})"
    found_items+=("opt-install")
else
    skip "no /opt/docklite"
fi

# docklite system user
if id docklite >/dev/null 2>&1; then
    ok "System user: docklite"
    found_items+=("user")
else
    skip "no docklite system user"
fi

# Site data in /var/www/sites
if [[ -d /var/www/sites ]]; then
    site_count=$(ls -1 /var/www/sites 2>/dev/null | wc -l)
    ok "Site data: /var/www/sites (${site_count} sites)"
    for d in /var/www/sites/*/; do
        [[ -d "$d" ]] && info "  $(basename "$d")"
    done
    found_items+=("sites")
else
    skip "no /var/www/sites"
fi

# Local config/data in repo dir
local_conf=()
[[ -f "${REPO_DIR}/.docklite.conf" ]] && local_conf+=(".docklite.conf")
[[ -f "${REPO_DIR}/ecosystem.config.js" ]] && local_conf+=("ecosystem.config.js")
[[ -d "${REPO_DIR}/data" ]] && local_conf+=("data/")
[[ -d "${REPO_DIR}/logs" ]] && local_conf+=("logs/")
[[ -f "${REPO_DIR}/.docklite-agent.pid" ]] && local_conf+=(".docklite-agent.pid")
[[ -f "${REPO_DIR}/.docklite-gui.pid" ]] && local_conf+=(".docklite-gui.pid")
if [[ ${#local_conf[@]} -gt 0 ]]; then
    ok "Local files: ${local_conf[*]}"
    found_items+=("local")
else
    skip "no local config files"
fi

echo ""
rainbow_line
echo ""

if [[ ${#found_items[@]} -eq 0 ]]; then
    echo -e "  ${GREEN}Nothing to clean up — DockLite is not installed.${NC}"
    echo ""
    exit 0
fi

# ══════════════════════════════════════════════════════════════════════════════
# Ask what to remove
# ══════════════════════════════════════════════════════════════════════════════

echo -e "  ${BOLD}What would you like to remove?${NC}"
echo ""
echo -e "  ${GOLD}1)${NC} ${BOLD}Reset DockLite${NC}     — remove services, configs, old install, reset database"
echo -e "                      ${DIM}keeps sites (/var/www/sites)${NC}"
echo -e "  ${GOLD}2)${NC} ${BOLD}Services only${NC}      — stop processes, remove systemd/PM2, keep data"
echo -e "  ${GOLD}3)${NC} ${BOLD}Old install only${NC}   — remove /opt/docklite + systemd (keep new PM2 setup)"
echo -e "  ${GOLD}4)${NC} ${BOLD}Everything${NC}         — ${RED}full nuke${NC}, including sites and databases"
echo -e "  ${RED}0)${NC} Cancel"
echo ""
echo -en "  ${CYAN}Choose${NC} ${YELLOW}[0]${NC}: "
read -r remove_choice

REMOVE_ALL=0; REMOVE_SERVICES=0; REMOVE_OLD=0; REMOVE_DATA=0; REMOVE_SITES=0

case "${remove_choice}" in
    1) REMOVE_ALL=1; REMOVE_SERVICES=1; REMOVE_OLD=1; REMOVE_DATA=1; REMOVE_SITES=0 ;;
    2) REMOVE_SERVICES=1 ;;
    3) REMOVE_OLD=1 ;;
    4) REMOVE_ALL=1; REMOVE_SERVICES=1; REMOVE_OLD=1; REMOVE_DATA=1; REMOVE_SITES=1 ;;
    *) echo -e "\n  ${DIM}Cancelled.${NC}\n"; exit 0 ;;
esac

if [[ "$REMOVE_SITES" == "1" ]]; then
    echo ""
    echo -e "  ${RED}${BOLD}⚠  This will delete ALL site files and databases!${NC}"
    echo -en "  ${RED}Are you sure?${NC} ${YELLOW}(y/N)${NC}: "
    read -r confirm
    if [[ "${confirm,,}" != "y" ]]; then
        echo -e "\n  ${DIM}Cancelled.${NC}\n"
        exit 0
    fi
fi

echo ""
echo -e "  ${BOLD}${CYAN}Cleaning up...${NC}"
echo ""

# ══════════════════════════════════════════════════════════════════════════════
# Removal
# ══════════════════════════════════════════════════════════════════════════════

# ── PM2 processes ──
if [[ "$REMOVE_SERVICES" == "1" ]] && command -v pm2 >/dev/null 2>&1; then
    pm2 stop docklite-agent 2>/dev/null && ok "PM2: stopped docklite-agent" || skip "not running"
    pm2 stop docklite-gui 2>/dev/null && ok "PM2: stopped docklite-gui" || skip "not running"
    pm2 delete docklite-agent 2>/dev/null && ok "PM2: deleted docklite-agent" || skip "not registered"
    pm2 delete docklite-gui 2>/dev/null && ok "PM2: deleted docklite-gui" || skip "not registered"
    pm2 save --force 2>/dev/null || true
fi

# ── Systemd services ──
if [[ "$REMOVE_SERVICES" == "1" ]] || [[ "$REMOVE_OLD" == "1" ]]; then
    for svc in docklite-agent docklite-web docklite; do
        if [[ -f "/etc/systemd/system/${svc}.service" ]]; then
            $SUDO systemctl stop "$svc" 2>/dev/null || true
            $SUDO systemctl disable "$svc" 2>/dev/null || true
            $SUDO rm -f "/etc/systemd/system/${svc}.service"
            ok "Systemd: removed ${svc}.service"
        fi
    done
    $SUDO systemctl daemon-reload 2>/dev/null || true
fi

# ── Kill stray processes ──
if [[ "$REMOVE_SERVICES" == "1" ]]; then
    # Kill processes owned by docklite system user
    $SUDO pkill -u docklite 2>/dev/null && ok "Killed docklite user processes" || skip "none running"
    # Kill any leftover next-server for docklite
    $SUDO pkill -f '/opt/docklite' 2>/dev/null && ok "Killed /opt/docklite processes" || skip "none running"
    # Kill stella's docklite processes (not PM2-managed ones)
    pkill -f 'docklite-agent' 2>/dev/null || true
    # Clean PID files
    rm -f "${REPO_DIR}/.docklite-agent.pid" "${REPO_DIR}/.docklite-gui.pid" 2>/dev/null || true
fi

# ── Nginx configs ──
if [[ "$REMOVE_SERVICES" == "1" ]] || [[ "$REMOVE_ALL" == "1" ]]; then
    for f in /etc/nginx/sites-enabled/docklite*; do
        [[ -e "$f" ]] && $SUDO rm -f "$f" && ok "Nginx: removed $(basename "$f") from sites-enabled"
    done
    for f in /etc/nginx/sites-available/docklite*; do
        [[ -e "$f" ]] && $SUDO rm -f "$f" && ok "Nginx: removed $(basename "$f") from sites-available"
    done
    if command -v nginx >/dev/null 2>&1; then
        $SUDO nginx -t 2>/dev/null && $SUDO nginx -s reload 2>/dev/null && ok "Nginx: reloaded" || warn "Nginx reload failed (check config)"
    fi
fi

# ── Sudoers ──
if [[ "$REMOVE_ALL" == "1" ]]; then
    for f in /etc/sudoers.d/docklite /etc/sudoers.d/docklite-nginx; do
        if [[ -f "$f" ]]; then
            $SUDO rm -f "$f"
            ok "Removed sudoers rule: $(basename "$f")"
        fi
    done
fi

# ── Symlink ──
if [[ "$REMOVE_ALL" == "1" ]] || [[ "$REMOVE_SERVICES" == "1" ]]; then
    if [[ -L /usr/local/bin/docklite ]]; then
        $SUDO rm -f /usr/local/bin/docklite
        ok "Removed /usr/local/bin/docklite symlink"
    fi
fi

# ── Old /opt/docklite install ──
if [[ "$REMOVE_OLD" == "1" ]] && [[ -d /opt/docklite ]]; then
    echo ""
    echo -e "  ${DIM}Removing /opt/docklite...${NC}"
    $SUDO rm -rf /opt/docklite
    ok "Removed /opt/docklite"
fi

# ── docklite system user ──
if [[ "$REMOVE_ALL" == "1" ]] && id docklite >/dev/null 2>&1; then
    $SUDO userdel docklite 2>/dev/null && ok "Removed docklite system user" || warn "Could not remove user (may have running processes)"
fi

# ── /etc/docklite ──
if [[ "$REMOVE_ALL" == "1" ]] && [[ -d /etc/docklite ]]; then
    $SUDO rm -rf /etc/docklite
    ok "Removed /etc/docklite"
fi

# ── Site data in /var/www/sites ──
if [[ "$REMOVE_SITES" == "1" ]] && [[ -d /var/www/sites ]]; then
    $SUDO rm -rf /var/www/sites
    ok "Removed /var/www/sites"
elif [[ "$REMOVE_ALL" == "1" ]] && [[ -d /var/www/sites ]]; then
    ok "Kept /var/www/sites (site files preserved)"
fi

# ── Local files in repo ──
if [[ "$REMOVE_ALL" == "1" ]] || [[ "$REMOVE_SERVICES" == "1" ]]; then
    rm -f "${REPO_DIR}/.docklite.conf" 2>/dev/null && ok "Removed .docklite.conf" || true
    rm -f "${REPO_DIR}/ecosystem.config.js" 2>/dev/null && ok "Removed ecosystem.config.js" || true
    rm -rf "${REPO_DIR}/logs" 2>/dev/null && ok "Removed logs/" || true
fi

if [[ "$REMOVE_DATA" == "1" ]] && [[ -d "${REPO_DIR}/data" ]]; then
    rm -rf "${REPO_DIR}/data"
    ok "Removed data/ (database deleted)"
elif [[ "$REMOVE_ALL" == "1" ]] && [[ -d "${REPO_DIR}/data" ]]; then
    ok "Kept data/ (database preserved)"
fi

# ══════════════════════════════════════════════════════════════════════════════
# Done
# ══════════════════════════════════════════════════════════════════════════════

echo ""
rainbow_line
echo ""
echo -e "  ${GREEN}${BOLD}Cleanup complete!${NC}"
echo ""

case "${remove_choice}" in
    1) echo -e "  ${DIM}To reinstall: sudo bash install.sh${NC}" ;;
    2) echo -e "  ${DIM}To restart: ./docklite setup${NC}" ;;
    3) echo -e "  ${DIM}Old install removed. New install unaffected.${NC}" ;;
esac

echo ""
