#!/usr/bin/env bash
set -euo pipefail

REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$REPO_DIR"

# ── colours ────────────────────────────────────────────────────────────────────
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

# ── helpers ────────────────────────────────────────────────────────────────────
print_header() {
  echo ""
  echo -e "${CYAN}${BOLD}╔══════════════════════════════════════╗${NC}"
  echo -e "${CYAN}${BOLD}║        DockLite Installer            ║${NC}"
  echo -e "${CYAN}${BOLD}╚══════════════════════════════════════╝${NC}"
  echo ""
}

ask() {
  local var="$1" prompt="$2" default="$3"
  local input
  echo -en "${BLUE}${prompt}${NC} ${YELLOW}[${default}]${NC}: "
  read -r input
  printf -v "$var" '%s' "${input:-$default}"
}

ask_password() {
  local var="$1" prompt="$2"
  local p1 p2
  while true; do
    echo -en "${BLUE}${prompt}${NC}: "
    read -rs p1; echo
    echo -en "${BLUE}Confirm password${NC}: "
    read -rs p2; echo
    if [[ "$p1" == "$p2" ]]; then
      printf -v "$var" '%s' "$p1"
      break
    fi
    echo -e "${RED}Passwords do not match, try again.${NC}"
  done
}

ask_yn() {
  local prompt="$1" default="${2:-N}"
  local label="y/N"
  [[ "$default" == "Y" ]] && label="Y/n"
  local input
  echo -en "${BLUE}${prompt}${NC} ${YELLOW}(${label})${NC}: "
  read -r input
  input="${input:-$default}"
  [[ "$input" =~ ^[Yy]$ ]]
}

ok()   { echo -e "${GREEN}  ✓ $*${NC}"; }
warn() { echo -e "${YELLOW}  ⚠ $*${NC}"; }
fail() { echo -e "${RED}  ✗ $*${NC}"; }
step() { echo -e "\n${BOLD}── $* ──${NC}"; }

SUDO="sudo"

DOCKLITE_USER="docklite"
AGENT_ENV_FILE="/etc/docklite/docklite-agent.env"
WEB_ENV_FILE="/etc/docklite/docklite-web.env"

# ── detect an existing installation ───────────────────────────────────────────
detect_existing() {
  [[ -f "$AGENT_ENV_FILE" ]]
}

# ── detect and handle existing DockLite containers ────────────────────────────
detect_existing_containers() {
  if ! command -v docker >/dev/null 2>&1; then return 1; fi
  docker ps --filter label=docklite.managed=true --format '{{.Names}}' 2>/dev/null
}

list_existing_containers() {
  local containers; containers=$(detect_existing_containers)
  [[ -z "$containers" ]] && return 1
  echo "$containers"
}

handle_existing_containers() {
  local containers; containers=$(detect_existing_containers)
  [[ -z "$containers" ]] && return 0

  local count; count=$(echo "$containers" | wc -l)
  echo ""
  step "Existing DockLite containers detected"
  echo "  Found ${count} container(s) from a previous installation:"
  echo ""
  while IFS= read -r name; do
    local img status
    img=$(docker inspect "$name" --format '{{.Config.Image}}' 2>/dev/null || echo '?')
    status=$(docker inspect "$name" --format '{{.State.Status}}' 2>/dev/null || echo '?')
    echo -e "    ${YELLOW}●${NC} ${BOLD}${name}${NC}  (${img} · ${status})"
  done <<< "$containers"
  echo ""
  echo "  What would you like to do with these containers?"
  echo ""
  echo "  1) Stop & remove  — clean slate, removes all containers and data"
  echo "  2) Keep running   — leave them, they won't be managed by this install"
  echo "  3) Cancel         — abort fresh install"
  echo ""
  local choice=""
  while [[ "$choice" != "1" && "$choice" != "2" && "$choice" != "3" ]]; do
    echo -en "${BLUE}Choose${NC} ${YELLOW}[3]${NC}: "
    read -r choice; choice="${choice:-3}"
  done

  if [[ "$choice" == "3" ]]; then
    echo "Aborted."
    exit 0
  fi

  if [[ "$choice" == "1" ]]; then
    step "Removing existing containers"
    while IFS= read -r name; do
      if docker stop "$name" >/dev/null 2>&1; then
        docker rm "$name" >/dev/null 2>&1 && ok "Stopped & removed: $name" || warn "Removed (was already stopped): $name"
      else
        docker rm "$name" >/dev/null 2>&1 && ok "Removed: $name" || warn "Could not remove: $name"
      fi
    done <<< "$containers"
    ok "All old containers removed"
  else
    echo ""
    warn "Containers are left running — they won't be managed by this installation."
  fi
}

read_existing_config() {
  INSTALL_DIR="$REPO_DIR"
  AGENT_PORT="3000"
  DOCKLITE_TOKEN=""
  INSTALL_MODE="full"

  if [[ -f "$AGENT_ENV_FILE" ]]; then
    local db_path
    db_path="$(grep -E '^DATABASE_PATH=' "$AGENT_ENV_FILE" | cut -d= -f2-)"
    [[ -n "$db_path" ]] && INSTALL_DIR="$(dirname "$db_path")"
    local addr
    addr="$(grep -E '^LISTEN_ADDR=' "$AGENT_ENV_FILE" | cut -d= -f2-)"
    AGENT_PORT="${addr#:}"
    [[ -z "$AGENT_PORT" ]] && AGENT_PORT="3000"
    DOCKLITE_TOKEN="$(grep -E '^DOCKLITE_TOKEN=' "$AGENT_ENV_FILE" | cut -d= -f2-)"
    local nextjs
    nextjs="$(grep -E '^NEXTJS_URL=' "$AGENT_ENV_FILE" | cut -d= -f2-)"
    [[ "$nextjs" == "disabled" ]] && INSTALL_MODE="headless"
  fi
}

# ── shared: install system deps, runtimes, build ──────────────────────────────
install_system_packages() {
  step "System packages"
  $SUDO apt-get update -y -q
  $SUDO apt-get install -y -q ca-certificates curl git rsync openssl unzip \
    build-essential pkg-config libsqlite3-dev python3
  ok "System packages installed"
}

install_docker() {
  if ! command -v docker >/dev/null 2>&1; then
    echo "Installing Docker..."
    curl -fsSL https://get.docker.com | sh
    $SUDO usermod -aG docker "${USER:-root}" || true
    ok "Docker installed"
  else
    ok "Docker already installed: $(docker --version | head -1)"
  fi
  if ! docker info >/dev/null 2>&1; then
    $SUDO systemctl start docker || true
  fi
}

install_nginx_if_needed() {
  local install_nginx="$1"
  if [[ -n "$install_nginx" ]]; then
    if ! command -v nginx >/dev/null 2>&1; then
      $SUDO apt-get install -y -q nginx certbot python3-certbot-nginx
      ok "Nginx + certbot installed"
    else
      ok "Nginx already installed"
    fi
  fi
}

install_bun() {
  if command -v bun >/dev/null 2>&1 || [[ -x /usr/local/bin/bun ]]; then
    BUN_CMD="$(command -v bun 2>/dev/null || echo /usr/local/bin/bun)"
    ok "Bun already installed: $($BUN_CMD --version)"
    return
  fi
  echo "Installing bun..."
  local arch; arch="$(uname -m)"
  case "$arch" in
    x86_64)  arch="x64" ;;
    aarch64) arch="aarch64" ;;
    *)       fail "Unsupported arch: $arch"; exit 1 ;;
  esac
  curl -fsSL "https://github.com/oven-sh/bun/releases/latest/download/bun-linux-${arch}.zip" -o /tmp/bun.zip
  unzip -o /tmp/bun.zip -d /tmp/bun-install >/dev/null
  $SUDO install -m 0755 "/tmp/bun-install/bun-linux-${arch}/bun" /usr/local/bin/bun
  rm -rf /tmp/bun.zip /tmp/bun-install
  ok "Bun installed: $(/usr/local/bin/bun --version)"
  BUN_CMD=/usr/local/bin/bun
}

install_node() {
  # Node.js is only needed for native addon compilation (bcrypt, better-sqlite3).
  # Bun handles all package management.
  if ! command -v node >/dev/null 2>&1; then
    echo "Installing Node.js 20.x (native addon compilation)..."
    curl -fsSL https://deb.nodesource.com/setup_20.x | $SUDO bash - >/dev/null 2>&1
    $SUDO apt-get install -y -q nodejs
    ok "Node.js installed: $(node -v)"
  else
    ok "Node.js already installed: $(node -v)"
  fi
}

install_go() {
  local GO_VERSION="1.22.6"
  local need_go="true"
  if command -v go >/dev/null 2>&1; then
    local gv maj min
    gv="$(go version | awk '{print $3}' | sed 's/go//')"
    maj="$(echo "$gv" | cut -d. -f1)"
    min="$(echo "$gv" | cut -d. -f2)"
    [[ "$maj" -gt 1 || ( "$maj" -eq 1 && "$min" -ge 22 ) ]] && need_go="false"
  fi
  if [[ "$need_go" == "true" ]]; then
    local go_arch; go_arch="$(uname -m)"
    [[ "$go_arch" == "x86_64" ]] && go_arch="amd64" || go_arch="arm64"
    echo "Installing Go ${GO_VERSION}..."
    curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-${go_arch}.tar.gz" -o /tmp/go.tgz
    $SUDO rm -rf /usr/local/go
    $SUDO tar -C /usr/local -xzf /tmp/go.tgz
    rm -f /tmp/go.tgz
    export PATH="/usr/local/go/bin:$PATH"
    ok "Go installed: $(go version)"
  else
    export PATH="/usr/local/go/bin:$PATH"
    ok "Go already installed: $(go version)"
  fi
}

build_agent() {
  local dir="$1"
  echo "Building agent binary..."
  $SUDO -u "${DOCKLITE_USER:-$USER}" bash -lc \
    "cd '${dir}' && mkdir -p bin && cd go-app && PATH=/usr/local/go/bin:/usr/bin:/bin go build -o ../bin/docklite-agent ./cmd/docklite-agent"
  ok "Agent binary built"
}

build_tui() {
  local dir="$1"
  $SUDO -u "${DOCKLITE_USER:-$USER}" bash -lc \
    "cd '${dir}/cli-repo' && PATH=/usr/local/go/bin:/usr/bin:/bin go build -o ../bin/docklite-tui ." && ok "TUI binary built" || warn "TUI build failed (optional)"
}

build_gui() {
  local dir="$1" port="$2"
  echo "Installing Node dependencies..."
  $SUDO -u "${DOCKLITE_USER:-$USER}" bash -lc "cd '${dir}/webapp' && $BUN_CMD install" 2>&1 | tail -3
  ok "Node dependencies installed"
  echo "Building Next.js..."
  $SUDO -u "${DOCKLITE_USER:-$USER}" bash -lc \
    "cd '${dir}/webapp' && AGENT_URL=http://127.0.0.1:${port} $BUN_CMD run build" 2>&1 | tail -5
  ok "Next.js built"
}

install_motd() {
  local src="${REPO_DIR}/webapp/scripts/motd/10-docklite"
  local dst="/etc/update-motd.d/10-docklite"
  if [[ ! -f "$src" ]]; then
    warn "MOTD script not found at $src — skipping"
    return
  fi
  $SUDO cp "$src" "$dst"
  $SUDO chmod +x "$dst"
  ok "MOTD installed at ${dst}"
}

write_sudoers() {
  local user="$1"
  $SUDO mkdir -p /etc/sudoers.d
  cat <<EOF | $SUDO tee /etc/sudoers.d/docklite >/dev/null
# DockLite sudoers rules
# Service management
${user} ALL=(root) NOPASSWD: /usr/bin/systemctl restart docklite-agent.service
${user} ALL=(root) NOPASSWD: /usr/bin/systemctl restart docklite-web.service
${user} ALL=(root) NOPASSWD: /usr/bin/systemctl start docklite-agent.service
${user} ALL=(root) NOPASSWD: /usr/bin/systemctl start docklite-web.service
${user} ALL=(root) NOPASSWD: /usr/bin/systemctl stop docklite-agent.service
${user} ALL=(root) NOPASSWD: /usr/bin/systemctl stop docklite-web.service
# Nginx management
${user} ALL=(root) NOPASSWD: /usr/sbin/nginx
${user} ALL=(root) NOPASSWD: /usr/sbin/nginx -t
${user} ALL=(root) NOPASSWD: /usr/sbin/nginx -s reload
${user} ALL=(root) NOPASSWD: /usr/bin/tee /etc/nginx/sites-available/*
${user} ALL=(root) NOPASSWD: /usr/bin/ln -sf /etc/nginx/sites-available/* /etc/nginx/sites-enabled/*
${user} ALL=(root) NOPASSWD: /usr/bin/rm -f /etc/nginx/sites-available/*
${user} ALL=(root) NOPASSWD: /usr/bin/rm -f /etc/nginx/sites-enabled/*
# SSL management (certbot)
${user} ALL=(root) NOPASSWD: /usr/bin/certbot
# Reading Let's Encrypt certs
${user} ALL=(root) NOPASSWD: /usr/bin/cat /etc/letsencrypt/live/*/fullchain.pem
${user} ALL=(root) NOPASSWD: /usr/bin/ls /etc/letsencrypt/live
EOF
  $SUDO chmod 440 /etc/sudoers.d/docklite
  # Clean up old/deprecated sudoers files
  $SUDO rm -f /etc/sudoers.d/docklite-nginx /etc/sudoers.d/docklite-update 2>/dev/null || true
  ok "Sudoers configured"
}

install_logrotate() {
  if [[ -d /etc/logrotate.d ]]; then
    $SUDO cp "${INSTALL_DIR}/webapp/scripts/logrotate/docklite" /etc/logrotate.d/docklite
    $SUDO chmod 644 /etc/logrotate.d/docklite
    ok "Logrotate configured"
  fi
}

# Wait for agent health endpoint, then onboard sites that have .dkl files.
onboard_dkl_sites() {
  local port="$1" token="$2"
  local paths=() domains=()

  if [[ -d "/var/www/sites" ]]; then
    while IFS= read -r -d '' dkl_file; do
      local site_path domain
      site_path="$(dirname "$dkl_file")"
      domain="$(python3 -c "import json; d=json.load(open('$dkl_file')); print(d.get('domain','?'))" 2>/dev/null || basename "$site_path")"
      paths+=("$site_path")
      domains+=("$domain")
    done < <(find /var/www/sites -maxdepth 3 -name ".dkl" -print0 2>/dev/null)
  fi

  [[ ${#paths[@]} -eq 0 ]] && return

  echo "  Waiting for agent to be ready..."
  local ready=""
  for _ in $(seq 1 30); do
    if curl -sf "http://localhost:${port}/api/health" \
        -H "Authorization: Bearer ${token}" >/dev/null 2>&1; then
      ready="1"; break
    fi
    sleep 2
  done

  if [[ -z "$ready" ]]; then
    warn "Agent not ready — skipping onboard. Run from the web UI later."
    return
  fi
  ok "Agent is ready"

  local ok_count=0 fail_count=0
  for i in "${!paths[@]}"; do
    if curl -sf -X POST "http://localhost:${port}/api/containers/onboard" \
        -H "Authorization: Bearer ${token}" \
        -H "Content-Type: application/json" \
        -d "{\"path\": \"${paths[$i]}\"}" >/dev/null 2>&1; then
      ok "Onboarded: ${domains[$i]}"
      ok_count=$((ok_count + 1))
    else
      warn "Failed: ${domains[$i]}"
      fail_count=$((fail_count + 1))
    fi
  done
  echo -e "\n  Results: ${GREEN}${ok_count} onboarded${NC}  ${RED}${fail_count} failed${NC}"
}

print_done() {
  local port="$1" username="$2" password="$3" service="$4" token_file="$5"
  echo ""
  echo -e "${CYAN}${BOLD}╔══════════════════════════════════════════════════╗${NC}"
  echo -e "${CYAN}${BOLD}║                    All done!                     ║${NC}"
  echo -e "${CYAN}${BOLD}╚══════════════════════════════════════════════════╝${NC}"
  echo ""
  echo -e "  ${BOLD}URL:${NC}      http://$(hostname -I | awk '{print $1}' 2>/dev/null || echo localhost):${port}"
  echo -e "  ${BOLD}Username:${NC} ${username}"
  echo -e "  ${BOLD}Password:${NC} ${password}"
  echo ""
  if [[ -z "$service" ]]; then
    echo -e "  To start:  ${YELLOW}./start-fullstack.sh${NC}"
    echo -e "  To stop:   ${YELLOW}./stop-all.sh${NC}"
    echo ""
  fi
  [[ -n "$token_file" ]] && echo -e "  Token file: ${BLUE}${token_file}${NC}"
  echo ""
}

# ══════════════════════════════════════════════════════════════════════════════
# FRESH INSTALL
# ══════════════════════════════════════════════════════════════════════════════
run_fresh_install() {
  # ── install mode ────────────────────────────────────────────────────────────
  step "Install mode"
  echo "  1) Full stack  — Web GUI + Agent (recommended)"
  echo "  2) Headless    — Agent only, TUI/API access"
  echo ""
  local mode_choice=""
  while [[ "$mode_choice" != "1" && "$mode_choice" != "2" ]]; do
    echo -en "${BLUE}Choose mode${NC} ${YELLOW}[1]${NC}: "
    read -r mode_choice; mode_choice="${mode_choice:-1}"
  done
  local INSTALL_MODE="full"
  [[ "$mode_choice" == "2" ]] && INSTALL_MODE="headless"
  ok "Mode: $INSTALL_MODE"

  # ── port ────────────────────────────────────────────────────────────────────
  step "Network"
  local AGENT_PORT
  ask AGENT_PORT "Agent port" "3000"
  ok "Agent will listen on port ${AGENT_PORT}"

  # ── admin account ────────────────────────────────────────────────────────────
  step "Admin account"
  echo -e "  These credentials will be used to log into the web UI."
  echo ""
  local ADMIN_USERNAME ADMIN_PASSWORD USE_DEFAULT_PW=""
  ask ADMIN_USERNAME "Admin username" "superadmin"
  if ask_yn "Use default password (supersecretpassword123)? Change after first login." "Y"; then
    ADMIN_PASSWORD="supersecretpassword123"
    USE_DEFAULT_PW="1"
    warn "Using default password — change it after first login!"
  else
    ask_password ADMIN_PASSWORD "Admin password"
  fi
  ok "Admin account: ${ADMIN_USERNAME}"

  # ── systemd ─────────────────────────────────────────────────────────────────
  step "System service"
  local INSTALL_SERVICE=""
  if ask_yn "Install DockLite as a systemd service (auto-start on boot)?" "Y"; then
    INSTALL_SERVICE="1"
    ok "Will install systemd service"
  else
    warn "Skipping systemd — use ./start-fullstack.sh to start manually"
  fi

  # ── nginx ───────────────────────────────────────────────────────────────────
  local INSTALL_NGINX=""
  if ask_yn "Install nginx as a reverse proxy?" "Y"; then
    INSTALL_NGINX="1"
    ok "Will install nginx"
  else
    warn "Skipping nginx"
  fi

  # ── existing sites ──────────────────────────────────────────────────────────
  step "Existing sites"
  local ONBOARD_AFTER_START=""
  local found_count=0
  if [[ -d "/var/www/sites" ]]; then
    found_count=$(find /var/www/sites -maxdepth 3 -name ".dkl" 2>/dev/null | wc -l)
  fi
  if [[ "$found_count" -gt 0 ]]; then
    echo "  Found ${found_count} site(s) with .dkl manifests:"
    find /var/www/sites -maxdepth 3 -name ".dkl" 2>/dev/null | while read -r f; do
      local d; d="$(python3 -c "import json; print(json.load(open('$f')).get('domain','?'))" 2>/dev/null || basename "$(dirname "$f")")"
      echo "    • $d  ($(dirname "$f"))"
    done
    echo ""
    if ask_yn "Onboard these sites automatically after install?" "Y"; then
      ONBOARD_AFTER_START="1"
      ok "Will onboard existing sites after services start"
    else
      warn "Skipping — onboard later from the web UI"
    fi
  else
    ok "No existing sites found"
  fi

  # ── build from source ────────────────────────────────────────────────────────
  step "Build"
  local BUILD_SOURCE=""
  if command -v go >/dev/null 2>&1; then
    if ask_yn "Build Go binaries from source?" "Y"; then
      BUILD_SOURCE="1"
    fi
  else
    warn "Go not found — will use pre-built binaries"
  fi

  # ── summary ─────────────────────────────────────────────────────────────────
  echo ""
  echo -e "${CYAN}${BOLD}── Summary ──────────────────────────────────────${NC}"
  echo -e "  Mode:         ${BOLD}${INSTALL_MODE}${NC}"
  echo -e "  Agent port:   ${BOLD}${AGENT_PORT}${NC}"
  echo -e "  Admin user:   ${BOLD}${ADMIN_USERNAME}${NC}"
  echo -e "  Password:     ${BOLD}$([ -n "$USE_DEFAULT_PW" ] && echo 'supersecretpassword123 (default)' || echo '(custom)')${NC}"
  echo -e "  Systemd:      ${BOLD}$([ -n "$INSTALL_SERVICE" ] && echo 'yes' || echo 'no')${NC}"
  echo -e "  Nginx:        ${BOLD}$([ -n "$INSTALL_NGINX" ] && echo 'yes' || echo 'no')${NC}"
  echo -e "  Build source: ${BOLD}$([ -n "$BUILD_SOURCE" ] && echo 'yes' || echo 'no')${NC}"
  echo -e "${CYAN}${BOLD}─────────────────────────────────────────────────${NC}"
  echo ""
  if ! ask_yn "Proceed?" "Y"; then echo "Aborted."; exit 0; fi

  # ── run install ─────────────────────────────────────────────────────────────
  install_system_packages
  install_docker
  install_nginx_if_needed "$INSTALL_NGINX"

  step "Runtime"
  install_bun
  install_node
  [[ -n "$BUILD_SOURCE" ]] && install_go

  step "User and directories"
  local INSTALL_DIR="/opt/docklite"
  if [[ -n "$INSTALL_SERVICE" ]]; then
    if ! id -u "$DOCKLITE_USER" >/dev/null 2>&1; then
      $SUDO useradd --system --create-home --home-dir "$INSTALL_DIR" --shell /usr/sbin/nologin "$DOCKLITE_USER"
      ok "Created user: ${DOCKLITE_USER}"
    else
      ok "User already exists: ${DOCKLITE_USER}"
    fi
    getent group docker >/dev/null 2>&1 && $SUDO usermod -aG docker "$DOCKLITE_USER" || true
  fi

  if [[ "$REPO_DIR" != "$INSTALL_DIR" && -n "$INSTALL_SERVICE" ]]; then
    echo "Copying files to ${INSTALL_DIR}..."
    $SUDO mkdir -p "$INSTALL_DIR"
    $SUDO rsync -a --delete \
      --exclude node_modules --exclude .next --exclude .bun --exclude data --exclude "*.log" --exclude ".git" \
      "${REPO_DIR}/" "${INSTALL_DIR}/"
    ok "Files copied to ${INSTALL_DIR}"
  else
    INSTALL_DIR="$REPO_DIR"
    ok "Using in-place directory: ${INSTALL_DIR}"
  fi

  $SUDO mkdir -p "${INSTALL_DIR}/data" "${INSTALL_DIR}/logs" /etc/docklite /var/www/sites
  [[ -n "$INSTALL_SERVICE" ]] && $SUDO chown -R "${DOCKLITE_USER}:${DOCKLITE_USER}" "$INSTALL_DIR" /etc/docklite || true
  if [[ -n "$INSTALL_SERVICE" ]]; then
    $SUDO chown "${DOCKLITE_USER}:${DOCKLITE_USER}" /var/www/sites
  else
    $SUDO chown "${USER:-root}:${USER:-root}" /var/www/sites 2>/dev/null || true
  fi
  $SUDO chmod 755 /var/www/sites
  ok "Directories ready"

  step "Configuration"
  local DOCKLITE_TOKEN SESSION_SECRET
  DOCKLITE_TOKEN="$(openssl rand -hex 32)"
  SESSION_SECRET="$(openssl rand -hex 48)"
  local NEXTJS_URL_VALUE="http://127.0.0.1:$((AGENT_PORT + 1))"
  [[ "$INSTALL_MODE" == "headless" ]] && NEXTJS_URL_VALUE="disabled"

  cat <<EOF | $SUDO tee "$AGENT_ENV_FILE" >/dev/null
LISTEN_ADDR=:${AGENT_PORT}
NEXTJS_URL=${NEXTJS_URL_VALUE}
DOCKER_SOCKET_PATH=unix:///var/run/docker.sock
DATABASE_PATH=${INSTALL_DIR}/data/docklite.db
DOCKLITE_TOKEN=${DOCKLITE_TOKEN}
EOF
  ok "Agent env written"

  if [[ "$INSTALL_MODE" == "full" ]]; then
    cat <<EOF | $SUDO tee "$WEB_ENV_FILE" >/dev/null
NODE_ENV=production
PORT=$((AGENT_PORT + 1))
AGENT_URL=http://127.0.0.1:${AGENT_PORT}
AGENT_TOKEN=${DOCKLITE_TOKEN}
DATABASE_PATH=${INSTALL_DIR}/data/docklite.db
SESSION_SECRET=${SESSION_SECRET}
SEED_ADMIN_USERNAME=${ADMIN_USERNAME}
SEED_ADMIN_PASSWORD=${ADMIN_PASSWORD}
EOF
    ok "Web env written"
  fi

  step "Building"
  [[ "$INSTALL_MODE" == "full" ]] && build_gui "$INSTALL_DIR" "$AGENT_PORT"
  if [[ -n "$BUILD_SOURCE" ]]; then
    build_agent "$INSTALL_DIR"
    build_tui "$INSTALL_DIR"
  fi

  step "Permissions"
  write_sudoers "${DOCKLITE_USER:-$USER}"
  install_motd

  if [[ -n "$INSTALL_SERVICE" ]]; then
    step "Systemd services"
    $SUDO sed "s|__INSTALL_DIR__|${INSTALL_DIR}|g" \
      "${INSTALL_DIR}/webapp/scripts/systemd/docklite-agent.service" | \
      $SUDO tee /etc/systemd/system/docklite-agent.service >/dev/null
    if [[ "$INSTALL_MODE" == "full" ]]; then
      $SUDO sed "s|__INSTALL_DIR__|${INSTALL_DIR}|g" \
        "${INSTALL_DIR}/webapp/scripts/systemd/docklite-web.service" | \
        $SUDO tee /etc/systemd/system/docklite-web.service >/dev/null
    fi
    $SUDO systemctl daemon-reload
    if [[ "$INSTALL_MODE" == "full" ]]; then
      $SUDO systemctl enable --now docklite-web.service docklite-agent.service
      ok "docklite-web.service enabled and started"
    else
      $SUDO systemctl enable --now docklite-agent.service
    fi
    ok "docklite-agent.service enabled and started"
  fi

  if [[ -n "$ONBOARD_AFTER_START" ]]; then
    step "Onboarding existing sites"
    onboard_dkl_sites "$AGENT_PORT" "$DOCKLITE_TOKEN"
  fi

  print_done "$AGENT_PORT" "$ADMIN_USERNAME" "$ADMIN_PASSWORD" "$INSTALL_SERVICE" "$AGENT_ENV_FILE"
}

# ══════════════════════════════════════════════════════════════════════════════
# REPAIR
# ══════════════════════════════════════════════════════════════════════════════
run_repair() {
  step "Detecting existing installation"

  if ! detect_existing; then
    warn "No existing installation found at ${AGENT_ENV_FILE}"
    echo -e "  If DockLite is installed elsewhere, locate its env file and re-run."
    echo ""
    if ask_yn "Run a fresh install instead?" "Y"; then
      run_fresh_install
    fi
    return
  fi

  read_existing_config
  ok "Found installation at: ${INSTALL_DIR}"
  ok "Agent port: ${AGENT_PORT}"
  ok "Mode: ${INSTALL_MODE}"

  # Check what's actually broken / missing
  echo ""
  step "Health check"
  local agent_running="" gui_running="" agent_binary="" gui_built="" db_ok=""
  local BUN_CMD

  curl -sf "http://localhost:${AGENT_PORT}/api/health" \
    -H "Authorization: Bearer ${DOCKLITE_TOKEN}" >/dev/null 2>&1 \
    && agent_running="1" && ok "Agent is running" || fail "Agent is NOT running"

  [[ -f "${INSTALL_DIR}/bin/docklite-agent" ]] \
    && agent_binary="1" && ok "Agent binary exists" || fail "Agent binary missing"

  [[ -d "${INSTALL_DIR}/webapp/.next" ]] \
    && gui_built="1" && ok "Next.js build exists" || fail "Next.js build missing"

  [[ -f "${INSTALL_DIR}/data/docklite.db" ]] \
    && db_ok="1" && ok "Database exists" || fail "Database missing"

  BUN_CMD="$(command -v bun 2>/dev/null || echo /usr/local/bin/bun)"
  [[ -x "$BUN_CMD" ]] && ok "Bun available: $($BUN_CMD --version)" || fail "Bun not found"

  echo ""
  step "Repair options"
  echo "  Select what to repair (or just press Enter to fix everything recommended):"
  echo ""

  local DO_DEPS="" DO_GUI="" DO_AGENT="" DO_PERMS="" DO_SERVICES="" DO_ONBOARD=""

  if ask_yn "  Reinstall Node dependencies + rebuild Next.js?" "$([ -z "$gui_built" ] && echo Y || echo N)"; then
    DO_DEPS="1"; DO_GUI="1"
  fi

  if command -v go >/dev/null 2>&1 || [[ -x /usr/local/go/bin/go ]]; then
    if ask_yn "  Rebuild Go agent binary from source?" "$([ -z "$agent_binary" ] && echo Y || echo N)"; then
      DO_AGENT="1"
    fi
  fi

  if ask_yn "  Fix file permissions (/var/www/sites, database, install dir)?" "Y"; then
    DO_PERMS="1"
  fi

  if systemctl list-units --type=service 2>/dev/null | grep -q "docklite"; then
    if ask_yn "  Restart systemd services?" "$([ -z "$agent_running" ] && echo Y || echo N)"; then
      DO_SERVICES="1"
    fi
  fi

  if ask_yn "  Scan for .dkl sites and re-onboard any not currently running?" "Y"; then
    DO_ONBOARD="1"
  fi

  echo ""
  if ! ask_yn "Proceed with repair?" "Y"; then echo "Aborted."; return; fi

  # ── run selected repairs ────────────────────────────────────────────────────
  if [[ -n "$DO_DEPS" || -n "$DO_GUI" ]]; then
    step "Runtime"
    install_bun
    install_node
    step "Rebuilding GUI"
    build_gui "$INSTALL_DIR" "$AGENT_PORT"
  fi

  if [[ -n "$DO_AGENT" ]]; then
    step "Rebuilding agent"
    export PATH="/usr/local/go/bin:$PATH"
    build_agent "$INSTALL_DIR"
    build_tui "$INSTALL_DIR" 2>/dev/null || true
  fi

  if [[ -n "$DO_PERMS" ]]; then
    step "Fixing permissions"
    if id -u "$DOCKLITE_USER" >/dev/null 2>&1; then
      $SUDO chown -R "${DOCKLITE_USER}:${DOCKLITE_USER}" "$INSTALL_DIR" /etc/docklite 2>/dev/null || true
      $SUDO chown "${DOCKLITE_USER}:${DOCKLITE_USER}" /var/www/sites 2>/dev/null || true
    fi
    $SUDO chmod 755 /var/www/sites
    [[ -f "${INSTALL_DIR}/data/docklite.db" ]] && \
      $SUDO chmod 660 "${INSTALL_DIR}/data/docklite.db" || true
    write_sudoers "${DOCKLITE_USER:-$USER}"
    install_motd
    ok "Permissions fixed"
  fi

  if [[ -n "$DO_SERVICES" ]]; then
    step "Restarting services"
    if [[ "$INSTALL_MODE" == "full" ]] && systemctl list-units --type=service 2>/dev/null | grep -q "docklite-web"; then
      $SUDO systemctl restart docklite-web.service && ok "docklite-web restarted" || warn "docklite-web restart failed"
    fi
    if systemctl list-units --type=service 2>/dev/null | grep -q "docklite-agent"; then
      $SUDO systemctl restart docklite-agent.service && ok "docklite-agent restarted" || warn "docklite-agent restart failed"
    fi
    sleep 3
  fi

  if [[ -n "$DO_ONBOARD" ]]; then
    step "Re-onboarding sites"
    if [[ -z "$DOCKLITE_TOKEN" ]]; then
      warn "No token found in env file — skipping onboard"
    else
      onboard_dkl_sites "$AGENT_PORT" "$DOCKLITE_TOKEN"
    fi
  fi

  echo ""
  echo -e "${CYAN}${BOLD}╔══════════════════════════════════════════════════╗${NC}"
  echo -e "${CYAN}${BOLD}║              Repair complete!                    ║${NC}"
  echo -e "${CYAN}${BOLD}╚══════════════════════════════════════════════════╝${NC}"
  echo ""
  echo -e "  ${BOLD}URL:${NC} http://$(hostname -I | awk '{print $1}' 2>/dev/null || echo localhost):${AGENT_PORT}"
  echo ""
}

# ══════════════════════════════════════════════════════════════════════════════
# ENTRY POINT
# ══════════════════════════════════════════════════════════════════════════════
print_header

if detect_existing; then
  echo -e "  An existing DockLite installation was detected."
  echo ""
  echo "  1) Fresh install  — set up from scratch (keeps existing site files)"
  echo "  2) Repair         — fix a broken or stopped installation"
  echo ""
  ENTRY_CHOICE=""
  while [[ "$ENTRY_CHOICE" != "1" && "$ENTRY_CHOICE" != "2" ]]; do
    echo -en "${BLUE}Choose${NC} ${YELLOW}[2]${NC}: "
    read -r ENTRY_CHOICE; ENTRY_CHOICE="${ENTRY_CHOICE:-2}"
  done
  echo ""
  if [[ "$ENTRY_CHOICE" == "1" ]]; then
    handle_existing_containers
    run_fresh_install
  else
    run_repair
  fi
else
  echo -e "  No existing installation detected. Starting fresh install."
  echo -e "  Press ${YELLOW}Enter${NC} to accept defaults shown in brackets."
  echo ""
  handle_existing_containers
  run_fresh_install
fi
