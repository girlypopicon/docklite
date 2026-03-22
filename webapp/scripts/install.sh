#!/usr/bin/env bash
set -euo pipefail

REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
INSTALL_DIR="${INSTALL_DIR:-/opt/docklite}"
DOCKLITE_USER="${DOCKLITE_USER:-docklite}"
DOCKLITE_GROUP="${DOCKLITE_GROUP:-docklite}"
NODE_MAJOR="${NODE_MAJOR:-20}"
GO_VERSION="${GO_VERSION:-1.22.6}"
AGENT_PORT="${AGENT_PORT:-3000}"
GUI_PORT="${GUI_PORT:-$((AGENT_PORT + 1))}"
INSTALL_MODE="${INSTALL_MODE:-full}"
if [[ "${HEADLESS:-}" == "1" ]]; then
  INSTALL_MODE="headless"
fi

SUDO=""
if [[ "${EUID}" -ne 0 ]]; then
  SUDO="sudo"
fi

if [[ -f /etc/os-release ]]; then
  . /etc/os-release
  if [[ "${ID:-}" != "ubuntu" ]]; then
    echo "Warning: this installer is tested on Ubuntu; continuing anyway." >&2
  fi
fi

echo "Installing system packages..."
$SUDO apt-get update -y
$SUDO apt-get install -y \
  ca-certificates \
  curl \
  git \
  rsync \
  openssl \
  unzip \
  build-essential \
  pkg-config \
  libsqlite3-dev

install_node() {
  local current_major="0"
  if command -v node >/dev/null 2>&1; then
    current_major="$(node -p "process.versions.node.split('.')[0]")"
  fi
  if [[ "${current_major}" -lt "${NODE_MAJOR}" ]]; then
    echo "Installing Node.js ${NODE_MAJOR}.x (required for native addon compilation)..."
    curl -fsSL "https://deb.nodesource.com/setup_${NODE_MAJOR}.x" | $SUDO bash -
    $SUDO apt-get install -y nodejs
  fi
}

install_bun() {
  if command -v bun >/dev/null 2>&1; then
    echo "bun already installed: $(bun --version)"
    return
  fi
  echo "Installing bun..."
  local arch
  arch="$(uname -m)"
  case "${arch}" in
    x86_64) arch="x64" ;;
    aarch64) arch="aarch64" ;;
    *) echo "Unsupported arch for bun: ${arch}" >&2; exit 1 ;;
  esac
  local url="https://github.com/oven-sh/bun/releases/latest/download/bun-linux-${arch}.zip"
  curl -fsSL "$url" -o /tmp/bun.zip
  unzip -o /tmp/bun.zip -d /tmp/bun-install >/dev/null
  $SUDO install -m 0755 /tmp/bun-install/bun-linux-${arch}/bun /usr/local/bin/bun
  rm -rf /tmp/bun.zip /tmp/bun-install
  echo "bun installed: $(bun --version)"
}

install_go() {
  local need_go="true"
  if command -v go >/dev/null 2>&1; then
    local current
    current="$(go env GOVERSION | sed 's/^go//')"
    local major minor
    major="$(echo "${current}" | cut -d. -f1)"
    minor="$(echo "${current}" | cut -d. -f2)"
    if [[ "${major}" -gt 1 ]] || [[ "${major}" -eq 1 && "${minor}" -ge 22 ]]; then
      need_go="false"
    fi
  fi
  if [[ "${need_go}" == "true" ]]; then
    echo "Installing Go ${GO_VERSION}..."
    local arch
    arch="$(uname -m)"
    case "${arch}" in
      x86_64) arch="amd64" ;;
      aarch64) arch="arm64" ;;
      *) echo "Unsupported architecture: ${arch}" >&2; exit 1 ;;
    esac
    curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-${arch}.tar.gz" -o /tmp/go.tgz
    $SUDO rm -rf /usr/local/go
    $SUDO tar -C /usr/local -xzf /tmp/go.tgz
    rm -f /tmp/go.tgz
  fi
}

install_node
install_go
install_bun

if ! id -u "${DOCKLITE_USER}" >/dev/null 2>&1; then
  echo "Creating system user ${DOCKLITE_USER}..."
  $SUDO useradd --system --create-home --home-dir "${INSTALL_DIR}" --shell /usr/sbin/nologin "${DOCKLITE_USER}"
fi

if getent group docker >/dev/null 2>&1; then
  $SUDO usermod -aG docker "${DOCKLITE_USER}" || true
fi

if [[ "${REPO_DIR}" != "${INSTALL_DIR}" ]]; then
  echo "Copying repo to ${INSTALL_DIR}..."
  $SUDO mkdir -p "${INSTALL_DIR}"
  $SUDO rsync -a --delete \
    --exclude node_modules \
    --exclude .next \
    --exclude .bun \
    --exclude .git \
    --exclude data \
    --exclude "*.log" \
    "${REPO_DIR}/" "${INSTALL_DIR}/"
fi

$SUDO mkdir -p "${INSTALL_DIR}/data" /etc/docklite
$SUDO chown -R "${DOCKLITE_USER}:${DOCKLITE_GROUP}" "${INSTALL_DIR}" /etc/docklite

# Create the sites root and make the docklite user the sole owner.
# All OS-level permissions flow through this user; DockLite enforces
# per-user access within its own application layer.
$SUDO mkdir -p /var/www/sites
$SUDO chown "${DOCKLITE_USER}:${DOCKLITE_GROUP}" /var/www/sites
$SUDO chmod 755 /var/www/sites

TOKEN_FILE="/etc/docklite/docklite-agent.env"
WEB_FILE="/etc/docklite/docklite-web.env"
NEXTJS_URL_VALUE="http://127.0.0.1:${GUI_PORT}"
if [[ "${INSTALL_MODE}" == "headless" ]]; then
  NEXTJS_URL_VALUE="disabled"
fi

if [[ ! -f "${TOKEN_FILE}" ]]; then
  DOCKLITE_TOKEN="$(openssl rand -hex 32)"
  cat <<EOF | $SUDO tee "${TOKEN_FILE}" >/dev/null
LISTEN_ADDR=:${AGENT_PORT}
NEXTJS_URL=${NEXTJS_URL_VALUE}
DOCKER_SOCKET_PATH=unix:///var/run/docker.sock
DATABASE_PATH=${INSTALL_DIR}/data/docklite.db
DOCKLITE_TOKEN=${DOCKLITE_TOKEN}
EOF
fi

if [[ "${INSTALL_MODE}" != "headless" && ! -f "${WEB_FILE}" ]]; then
  SESSION_SECRET="$(openssl rand -hex 48)"
  DOCKLITE_TOKEN="$(grep -E '^DOCKLITE_TOKEN=' "${TOKEN_FILE}" | cut -d= -f2)"
  cat <<EOF | $SUDO tee "${WEB_FILE}" >/dev/null
NODE_ENV=production
PORT=${GUI_PORT}
AGENT_URL=http://127.0.0.1:${AGENT_PORT}
AGENT_TOKEN=${DOCKLITE_TOKEN}
DATABASE_PATH=${INSTALL_DIR}/data/docklite.db
SESSION_SECRET=${SESSION_SECRET}
EOF
fi

echo "Installing Node dependencies..."
$SUDO -u "${DOCKLITE_USER}" bash -lc "cd '${INSTALL_DIR}/webapp' && bun install"

if [[ "${INSTALL_MODE}" != "headless" ]]; then
  echo "Building Next.js..."
  $SUDO -u "${DOCKLITE_USER}" bash -lc "cd '${INSTALL_DIR}/webapp' && AGENT_URL=http://127.0.0.1:${AGENT_PORT} bun run build"
fi

echo "Building agent..."
$SUDO -u "${DOCKLITE_USER}" bash -lc "cd '${INSTALL_DIR}' && mkdir -p bin && cd go-app && PATH=/usr/local/go/bin:/usr/bin:/bin go build -o ../bin/docklite-agent ./cmd/docklite-agent"

echo "Installing systemd services..."
$SUDO sed "s|__INSTALL_DIR__|${INSTALL_DIR}|g" "${INSTALL_DIR}/webapp/scripts/systemd/docklite-agent.service" | \
  $SUDO tee /etc/systemd/system/docklite-agent.service >/dev/null

$SUDO systemctl daemon-reload
if [[ "${INSTALL_MODE}" != "headless" ]]; then
  $SUDO sed "s|__INSTALL_DIR__|${INSTALL_DIR}|g" "${INSTALL_DIR}/webapp/scripts/systemd/docklite-web.service" | \
    $SUDO tee /etc/systemd/system/docklite-web.service >/dev/null
  $SUDO systemctl daemon-reload
  $SUDO systemctl enable --now docklite-web.service docklite-agent.service
else
  $SUDO systemctl enable --now docklite-agent.service
fi

echo "Configuring permissions..."
$SUDO mkdir -p /etc/sudoers.d
cat <<EOF | $SUDO tee /etc/sudoers.d/docklite >/dev/null
# Service management
${DOCKLITE_USER} ALL=(root) NOPASSWD: /usr/bin/systemctl restart docklite-agent.service, /usr/bin/systemctl restart docklite-web.service
# Nginx management
${DOCKLITE_USER} ALL=(root) NOPASSWD: /usr/sbin/nginx -t
${DOCKLITE_USER} ALL=(root) NOPASSWD: /usr/sbin/nginx -s reload
${DOCKLITE_USER} ALL=(root) NOPASSWD: /usr/bin/tee /etc/nginx/sites-available/*
${DOCKLITE_USER} ALL=(root) NOPASSWD: /usr/bin/ln -sf /etc/nginx/sites-available/* /etc/nginx/sites-enabled/*
${DOCKLITE_USER} ALL=(root) NOPASSWD: /usr/bin/rm -f /etc/nginx/sites-available/*
${DOCKLITE_USER} ALL=(root) NOPASSWD: /usr/bin/rm -f /etc/nginx/sites-enabled/*
# SSL management (certbot)
${DOCKLITE_USER} ALL=(root) NOPASSWD: /usr/bin/certbot
# Reading Let's Encrypt certs
${DOCKLITE_USER} ALL=(root) NOPASSWD: /usr/bin/cat /etc/letsencrypt/live/*/fullchain.pem
${DOCKLITE_USER} ALL=(root) NOPASSWD: /usr/bin/ls /etc/letsencrypt/live
EOF
$SUDO chmod 440 /etc/sudoers.d/docklite
# Clean up old file if it exists
$SUDO rm -f /etc/sudoers.d/docklite-update

echo "Done."
if [[ "${INSTALL_MODE}" != "headless" ]]; then
  echo "DockLite is running on http://localhost:${AGENT_PORT} (agent proxy)."
else
  echo "DockLite agent is running on http://localhost:${AGENT_PORT} (headless mode)."
fi
