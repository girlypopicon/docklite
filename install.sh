#!/usr/bin/env bash
# DockLite Installer — interactive setup wizard
# Usage: sudo bash install.sh   (or: curl -fsSL .../install.sh | sudo bash)
set -euo pipefail

# ══════════════════════════════════════════════════════════════════════════════
# Colors & Brand
# ══════════════════════════════════════════════════════════════════════════════

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'
BLUE='\033[0;34m'; CYAN='\033[0;36m'; BOLD='\033[1m'; DIM='\033[2m'; NC='\033[0m'
PINK='\033[38;2;255;106;213m';   CORAL='\033[38;2;255;154;139m'
GOLD='\033[38;2;255;209;102m';   MINT='\033[38;2;184;242;162m'
SKY='\033[38;2;154;208;255m';    LAVENDER='\033[38;2;199;164;255m'

REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INSTALL_DIR="/opt/docklite"
cd "$REPO_DIR"

# ══════════════════════════════════════════════════════════════════════════════
# Drawing helpers
# ══════════════════════════════════════════════════════════════════════════════

COLS=$(tput cols 2>/dev/null || echo 60)
[[ $COLS -gt 60 ]] && COLS=60

rainbow_line() {
    local chars="" i=0
    local colors=("$PINK" "$CORAL" "$GOLD" "$MINT" "$SKY" "$LAVENDER")
    while [[ $i -lt $COLS ]]; do
        chars+="${colors[$((i % 6))]}━"
        i=$((i + 1))
    done
    echo -e "${chars}${NC}"
}

banner() {
    clear
    echo ""
    echo -e "${PINK}     ____             __   __    _ __       ${NC}"
    echo -e "${CORAL}    / __ \\____  _____/ /__/ /   (_) /_____ ${NC}"
    echo -e "${GOLD}   / / / / __ \\/ ___/ //_/ /   / / __/ _ \\${NC}"
    echo -e "${MINT}  / /_/ / /_/ / /__/ ,< / /___/ / /_/  __/${NC}"
    echo -e "${SKY}  \\____/\\____/\\___/_/|_/_____/_/\\__/\\___/ ${NC}"
    echo -e "${LAVENDER}                              installer${NC}"
    echo ""
    rainbow_line
    echo ""
}

# Current step tracking
TOTAL_STEPS=0
CURRENT_STEP=0

set_total_steps() { TOTAL_STEPS=$1; CURRENT_STEP=0; }

step_header() {
    CURRENT_STEP=$((CURRENT_STEP + 1))
    echo ""
    echo -e "  ${CYAN}${BOLD}[$CURRENT_STEP/$TOTAL_STEPS]${NC} ${BOLD}$*${NC}"
    echo -e "  ${DIM}$(printf '%.0s─' $(seq 1 $((COLS - 4))))${NC}"
}

ok()   { echo -e "  ${GREEN}✓${NC} $*"; }
warn() { echo -e "  ${YELLOW}⚠${NC}  $*"; }
fail() { echo -e "  ${RED}✗${NC}  $*"; }
info() { echo -e "  ${DIM}$*${NC}"; }

spin() {
    local pid=$1 msg=$2
    local frames=('⠋' '⠙' '⠹' '⠸' '⠼' '⠴' '⠦' '⠧' '⠇' '⠏')
    local i=0
    while kill -0 "$pid" 2>/dev/null; do
        printf "\r  ${CYAN}${frames[$((i % 10))]}${NC} %s" "$msg"
        sleep 0.1
        i=$((i + 1))
    done
    wait "$pid" 2>/dev/null
    local rc=$?
    printf "\r"
    return $rc
}

ask() {
    local var="$1" prompt="$2" default="$3"
    echo -en "  ${BLUE}${prompt}${NC} ${YELLOW}[${default}]${NC}: "
    local input; read -r input
    printf -v "$var" '%s' "${input:-$default}"
}

ask_yn() {
    local prompt="$1" default="${2:-Y}"
    echo -en "  ${BLUE}${prompt}${NC} ${YELLOW}(${default}/$([ "$default" = Y ] && echo n || echo y))${NC}: "
    local input; read -r input
    input="${input:-$default}"
    [[ "${input^^}" == "Y" ]]
}

# ══════════════════════════════════════════════════════════════════════════════
# Dependency checks & installers
# ══════════════════════════════════════════════════════════════════════════════

SUDO=""
[[ "${EUID}" -ne 0 ]] && SUDO="sudo"

need_install=()

detect_deps() {
    step_header "Checking your system"
    echo ""

    # Docker
    if command -v docker >/dev/null 2>&1 && docker info >/dev/null 2>&1; then
        ok "Docker $(docker --version 2>/dev/null | grep -oP '\d+\.\d+\.\d+' | head -1)"
    else
        fail "Docker not found or not running"
        need_install+=("docker")
    fi

    # Node.js
    if command -v node >/dev/null 2>&1; then
        local node_ver; node_ver=$(node -v 2>/dev/null)
        ok "Node.js ${node_ver}"
    else
        fail "Node.js not found"
        need_install+=("node")
    fi

    # Go
    local go_bin=""
    if command -v go >/dev/null 2>&1; then
        go_bin="$(command -v go)"
    elif [[ -x /usr/local/go/bin/go ]]; then
        go_bin="/usr/local/go/bin/go"
    elif [[ -x "$HOME/.local/go/bin/go" ]]; then
        go_bin="$HOME/.local/go/bin/go"
    fi
    if [[ -n "$go_bin" ]]; then
        ok "Go $($go_bin version 2>/dev/null | grep -oP 'go\d+\.\d+\.\d+' | head -1)"
    else
        fail "Go not found"
        need_install+=("go")
    fi

    # PM2
    if command -v pm2 >/dev/null 2>&1; then
        ok "PM2 $(pm2 --version 2>/dev/null)"
    else
        fail "PM2 not found"
        need_install+=("pm2")
    fi

    # nginx
    if command -v nginx >/dev/null 2>&1; then
        ok "Nginx $(nginx -v 2>&1 | grep -oP '\d+\.\d+\.\d+' | head -1)"
    else
        fail "Nginx not found"
        need_install+=("nginx")
    fi

    # Agent binary
    if [[ -x "${REPO_DIR}/bin/docklite-agent" ]]; then
        ok "Agent binary found"
    else
        info "Agent binary will be built"
        need_install+=("build-agent")
    fi

    # Webapp
    if [[ -d "${REPO_DIR}/webapp/.next" ]]; then
        ok "Webapp is built"
    else
        info "Webapp will be built"
        need_install+=("build-webapp")
    fi

    echo ""
}

install_docker() {
    step_header "Installing Docker"
    if command -v docker >/dev/null 2>&1; then
        ok "Already installed"; return 0
    fi
    (curl -fsSL https://get.docker.com | $SUDO sh) >/dev/null 2>&1 &
    spin $! "Installing Docker engine..." && ok "Docker installed" || { fail "Docker install failed"; return 1; }
    $SUDO usermod -aG docker "${SUDO_USER:-$USER}" 2>/dev/null || true
    ok "Added $(whoami) to docker group"
}

install_node() {
    step_header "Installing Node.js"
    if command -v node >/dev/null 2>&1; then
        ok "Already installed: $(node -v)"; return 0
    fi
    (curl -fsSL https://deb.nodesource.com/setup_22.x | $SUDO bash - >/dev/null 2>&1 && \
        $SUDO apt-get install -y -q nodejs >/dev/null 2>&1) &
    spin $! "Installing Node.js 22..." && ok "Node.js installed: $(node -v)" || { fail "Node install failed"; return 1; }
}

install_go() {
    step_header "Installing Go"
    local go_bin=""
    command -v go >/dev/null 2>&1 && go_bin="$(command -v go)"
    [[ -z "$go_bin" && -x /usr/local/go/bin/go ]] && go_bin="/usr/local/go/bin/go"
    [[ -z "$go_bin" && -x "$HOME/.local/go/bin/go" ]] && go_bin="$HOME/.local/go/bin/go"
    if [[ -n "$go_bin" ]]; then
        ok "Already installed"; return 0
    fi

    local arch; arch="$(uname -m)"
    [[ "$arch" == "x86_64" ]] && arch="amd64"
    [[ "$arch" == "aarch64" ]] && arch="arm64"

    local install_dir="/usr/local"
    if [[ -n "$SUDO" ]] && ! sudo -n true 2>/dev/null; then
        install_dir="$HOME/.local"
    fi

    (curl -fsSL "https://go.dev/dl/go1.22.10.linux-${arch}.tar.gz" -o /tmp/go.tgz && \
        if [[ "$install_dir" == "/usr/local" ]]; then
            $SUDO rm -rf /usr/local/go && $SUDO tar -C /usr/local -xzf /tmp/go.tgz
        else
            mkdir -p "$install_dir" && rm -rf "${install_dir}/go" && tar -C "$install_dir" -xzf /tmp/go.tgz
        fi && rm -f /tmp/go.tgz) &
    spin $! "Installing Go 1.22..." && ok "Go installed to ${install_dir}/go" || { fail "Go install failed"; return 1; }

    export PATH="${install_dir}/go/bin:$PATH"
}

install_pm2() {
    step_header "Installing PM2"
    if command -v pm2 >/dev/null 2>&1; then
        ok "Already installed"; return 0
    fi
    (npm install -g pm2 >/dev/null 2>&1) &
    spin $! "Installing PM2..." && ok "PM2 installed" || { fail "PM2 install failed"; return 1; }
}

install_nginx() {
    step_header "Installing Nginx"
    if command -v nginx >/dev/null 2>&1; then
        ok "Already installed"; return 0
    fi
    ($SUDO apt-get install -y -q nginx >/dev/null 2>&1) &
    spin $! "Installing Nginx..." && ok "Nginx installed" || { fail "Nginx install failed"; return 1; }
}

install_system_packages() {
    step_header "System packages"
    ($SUDO apt-get update -qq >/dev/null 2>&1 && \
        $SUDO apt-get install -y -q ca-certificates curl git openssl build-essential pkg-config >/dev/null 2>&1) &
    spin $! "Updating packages..." && ok "System packages ready" || warn "Some packages may have failed"
}

# ══════════════════════════════════════════════════════════════════════════════
# Build steps
# ══════════════════════════════════════════════════════════════════════════════

find_go() {
    command -v go 2>/dev/null || echo "${HOME}/.local/go/bin/go" || echo "/usr/local/go/bin/go"
}

build_agent() {
    step_header "Building agent"
    if [[ -x "${REPO_DIR}/bin/docklite-agent" ]]; then
        ok "Agent binary already exists"; return 0
    fi
    local go_bin; go_bin=$(find_go)
    mkdir -p "${REPO_DIR}/bin"
    (cd "${REPO_DIR}/go-app" && "$go_bin" build -o ../bin/docklite-agent ./cmd/docklite-agent 2>&1) &
    spin $! "Compiling Go agent..." && ok "Agent built ($(du -h "${REPO_DIR}/bin/docklite-agent" | cut -f1))" || { fail "Agent build failed"; return 1; }
}

build_webapp() {
    step_header "Building web app"

    if [[ ! -d "${REPO_DIR}/webapp/node_modules" ]]; then
        (cd "${REPO_DIR}/webapp" && npm install --silent 2>&1) &
        spin $! "Installing dependencies..." && ok "Dependencies installed" || { fail "npm install failed"; return 1; }
    else
        ok "Dependencies already installed"
    fi

    if [[ ! -d "${REPO_DIR}/webapp/.next" ]]; then
        (cd "${REPO_DIR}/webapp" && npm run build 2>&1) &
        spin $! "Building Next.js app..." && ok "Webapp built" || { fail "Build failed"; return 1; }
    else
        ok "Webapp already built"
    fi
}

install_to_opt() {
    step_header "Installing to ${INSTALL_DIR}"
    $SUDO mkdir -p "${INSTALL_DIR}"
    $SUDO rsync -a --delete \
        --exclude '.git' \
        --exclude '.bun' \
        "${REPO_DIR}/" "${INSTALL_DIR}/"
    $SUDO chown -R docklite:docklite "${INSTALL_DIR}"
    ok "App installed to ${INSTALL_DIR}"
}

# ══════════════════════════════════════════════════════════════════════════════
# Sudoers & symlink
# ══════════════════════════════════════════════════════════════════════════════

setup_user_and_dirs() {
    step_header "User & directories"

    if id docklite >/dev/null 2>&1; then
        ok "User 'docklite' already exists"
    else
        $SUDO groupadd -f docklite 2>/dev/null || true
        if ! $SUDO useradd --system --create-home --home-dir /opt/docklite \
            --shell /usr/sbin/nologin -g docklite docklite 2>&1; then
            fail "Could not create 'docklite' user — run with: sudo bash install.sh"
            exit 1
        fi
        ok "Created system user 'docklite'"
    fi

    if ! id docklite >/dev/null 2>&1; then
        fail "User 'docklite' does not exist — cannot continue"
        exit 1
    fi

    $SUDO usermod -aG docker docklite 2>/dev/null || true
    ok "User 'docklite' in docker group"

    $SUDO mkdir -p /var/www/sites
    $SUDO chown -R docklite:docklite /var/www/sites
    $SUDO chmod 775 /var/www/sites
    ok "Site directory: /var/www/sites (owned by docklite)"

    $SUDO mkdir -p "${INSTALL_DIR}/data" "${INSTALL_DIR}/logs"
    $SUDO chown -R docklite:docklite "${INSTALL_DIR}/data" "${INSTALL_DIR}/logs" "${INSTALL_DIR}/bin"
    ok "Data/logs directories ready"
}

setup_sudoers() {
    step_header "Permissions"

    local calling_user="${SUDO_USER:-$USER}"
    local pm2_path
    pm2_path="$(command -v pm2 2>/dev/null || echo "/usr/bin/pm2")"
    # setup_nginx_default (run by `docklite setup`, which executes as
    # calling_user, not docklite — see install.sh's handoff) needs the same
    # NOPASSWD grants as docklite itself, or it silently no-ops.
    $SUDO tee /etc/sudoers.d/docklite >/dev/null <<EOF
docklite ALL=(ALL) NOPASSWD: /usr/sbin/nginx, /usr/bin/tee /etc/nginx/sites-available/*, /usr/bin/ln -sf /etc/nginx/sites-available/* /etc/nginx/sites-enabled/*, /usr/bin/rm -f /etc/nginx/sites-enabled/*, /usr/bin/certbot, /usr/bin/ls /etc/letsencrypt/live, /usr/bin/ls /etc/letsencrypt/live/*, /usr/bin/cat /etc/letsencrypt/live/*, /usr/bin/openssl
${calling_user} ALL=(ALL) NOPASSWD: /usr/sbin/nginx, /usr/bin/tee /etc/nginx/sites-available/*, /usr/bin/ln -sf /etc/nginx/sites-available/* /etc/nginx/sites-enabled/*, /usr/bin/rm -f /etc/nginx/sites-enabled/*, /usr/bin/certbot, /usr/bin/ls /etc/letsencrypt/live, /usr/bin/ls /etc/letsencrypt/live/*, /usr/bin/cat /etc/letsencrypt/live/*, /usr/bin/openssl
${calling_user} ALL=(ALL) NOPASSWD: ${pm2_path}
${calling_user} ALL=(docklite) NOPASSWD: ALL
EOF
    $SUDO chmod 440 /etc/sudoers.d/docklite
    ok "Sudoers rules installed for docklite user"
}

install_symlink() {
    step_header "System integration"
    $SUDO ln -sf "${INSTALL_DIR}/docklite" /usr/local/bin/docklite
    ok "Installed ${BOLD}docklite${NC} command"
    info "Run 'docklite' from anywhere to manage DockLite"
}

# ══════════════════════════════════════════════════════════════════════════════
# Confirmation screen
# ══════════════════════════════════════════════════════════════════════════════

show_install_plan() {
    echo ""
    echo -e "  ${BOLD}Installation Plan${NC}"
    echo ""

    if [[ ${#need_install[@]} -eq 0 ]]; then
        echo -e "  ${GREEN}Everything is already installed!${NC}"
        echo -e "  ${DIM}We'll just configure and start DockLite.${NC}"
    else
        echo -e "  ${DIM}The following will be installed:${NC}"
        echo ""
        for dep in "${need_install[@]}"; do
            case "$dep" in
                docker)        echo -e "    ${GOLD}•${NC} Docker engine" ;;
                node)          echo -e "    ${GOLD}•${NC} Node.js 22" ;;
                go)            echo -e "    ${GOLD}•${NC} Go 1.22" ;;
                pm2)           echo -e "    ${GOLD}•${NC} PM2 process manager" ;;
                nginx)         echo -e "    ${GOLD}•${NC} Nginx web server" ;;
                build-agent)   echo -e "    ${GOLD}•${NC} Build agent binary (Go)" ;;
                build-webapp)  echo -e "    ${GOLD}•${NC} Build web dashboard (Next.js)" ;;
            esac
        done
    fi

    echo ""
    echo -e "  ${DIM}Then we'll:${NC}"
    echo -e "    ${MINT}•${NC} Create ${BOLD}docklite${NC} system user"
    echo -e "    ${MINT}•${NC} Install app to ${BOLD}/opt/docklite${NC}"
    echo -e "    ${MINT}•${NC} Set up /var/www/sites directory"
    echo -e "    ${MINT}•${NC} Set up PM2 for process management"
    echo -e "    ${MINT}•${NC} Configure nginx reverse proxy"
    echo -e "    ${MINT}•${NC} Install ${BOLD}docklite${NC} command"
    echo -e "    ${MINT}•${NC} Start DockLite services"
    echo ""
}

# ══════════════════════════════════════════════════════════════════════════════
# Main wizard flow
# ══════════════════════════════════════════════════════════════════════════════

main() {
    banner

    echo -e "  ${BOLD}Welcome to the DockLite installer!${NC}"
    echo ""
    echo -e "  ${DIM}This wizard will install all dependencies and get${NC}"
    echo -e "  ${DIM}DockLite running on your server.${NC}"
    echo ""

    # ── check sudo ──
    if [[ "${EUID}" -ne 0 ]]; then
        echo -e "  ${YELLOW}⚠${NC}  This installer needs sudo for system packages."
        echo ""
        if ! sudo -v 2>/dev/null; then
            echo -e "  ${RED}✗${NC}  Could not get sudo access."
            echo -e "  ${DIM}Run with: sudo bash install.sh${NC}"
            exit 1
        fi
        ok "sudo access confirmed"
        echo ""
    fi

    # ── detect ──
    detect_deps

    # ── show plan ──
    show_install_plan

    if ! ask_yn "Ready to install?" "Y"; then
        echo ""
        echo -e "  ${DIM}Installation cancelled.${NC}"
        echo ""
        exit 0
    fi

    # ── count steps ──
    local total=0
    [[ " ${need_install[*]} " == *" docker "* ]] && total=$((total + 1))
    [[ " ${need_install[*]} " == *" node "* ]] && total=$((total + 1))
    [[ " ${need_install[*]} " == *" go "* ]] && total=$((total + 1))
    [[ " ${need_install[*]} " == *" pm2 "* ]] && total=$((total + 1))
    [[ " ${need_install[*]} " == *" nginx "* ]] && total=$((total + 1))
    # Always: system packages, user+dirs, build agent, build webapp, install to opt, sudoers, symlink, handoff
    total=$((total + 8))
    set_total_steps $total

    # ── install deps ──
    install_system_packages

    for dep in "${need_install[@]}"; do
        case "$dep" in
            docker)  install_docker ;;
            node)    install_node ;;
            go)      install_go ;;
            pm2)     install_pm2 ;;
            nginx)   install_nginx ;;
        esac
    done

    # ── user & directories ──
    setup_user_and_dirs

    # ── build ──
    build_agent
    build_webapp

    # ── install to /opt/docklite ──
    install_to_opt

    # ── system setup ──
    setup_sudoers
    install_symlink

    # ── handoff ──
    step_header "Launching DockLite setup"
    echo ""
    echo -e "  ${DIM}Dependencies are installed. Now let's configure DockLite.${NC}"
    echo ""
    rainbow_line
    echo ""

    sleep 1

    # Hand off to docklite setup — run as the calling user so they
    # can interact with PM2, but the ecosystem config tells PM2 to
    # run the actual processes as the docklite user.
    local target_user="${SUDO_USER:-$USER}"
    if [[ "$target_user" != "root" ]]; then
        exec sudo -u "$target_user" "${INSTALL_DIR}/docklite" setup
    else
        exec "${INSTALL_DIR}/docklite" setup
    fi
}

main "$@"
