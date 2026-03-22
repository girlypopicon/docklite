#!/usr/bin/env bash
set -euo pipefail

INSTALL_DIR="${INSTALL_DIR:-/opt/docklite}"
BRANCH="${BRANCH:-main}"
LOG_FILE="${LOG_FILE:-/var/log/docklite/update.log}"
PID_FILE="${PID_FILE:-/tmp/docklite-update.pid}"
GO_BIN="${GO_BIN:-/usr/local/go/bin}"

mkdir -p "$(dirname "$LOG_FILE")"
echo $$ > "$PID_FILE"

log() { echo "[$(date -u +%Y-%m-%dT%H:%M:%SZ)] $*" | tee -a "$LOG_FILE"; }

cleanup() {
    rm -f "$PID_FILE"
}
trap cleanup EXIT

log "=== DockLite update started (branch: $BRANCH) ==="

# 1. Pull latest code
log "Fetching latest code from origin/$BRANCH..."
git -C "$INSTALL_DIR" fetch origin
COMMITS_BEHIND=$(git -C "$INSTALL_DIR" rev-list HEAD.."origin/$BRANCH" --count 2>/dev/null || echo "0")
if [[ "$COMMITS_BEHIND" -eq 0 ]]; then
    log "Already up to date."
else
    log "Pulling $COMMITS_BEHIND new commit(s)..."
    git -C "$INSTALL_DIR" reset --hard "origin/$BRANCH"
    log "Code updated to $(git -C "$INSTALL_DIR" rev-parse --short HEAD)."
fi

# 2. Install Node dependencies
log "Installing Node dependencies..."
cd "$INSTALL_DIR/webapp"
bun install --frozen-lockfile

# 3. Build Next.js app
log "Building web app..."
AGENT_URL="http://127.0.0.1:3000" bun run build
log "Web app built successfully."

# 4. Build Go agent
log "Building Go agent..."
cd "$INSTALL_DIR/go-app"
if PATH="$GO_BIN:$PATH" go build -o "$INSTALL_DIR/bin/docklite-agent.new" ./cmd/docklite-agent; then
    mv "$INSTALL_DIR/bin/docklite-agent.new" "$INSTALL_DIR/bin/docklite-agent"
    log "Go agent built successfully."
else
    log "ERROR: Go agent build failed, keeping existing binary"
    rm -f "$INSTALL_DIR/bin/docklite-agent.new"
    exit 1
fi

# 5. Restart services
log "Restarting docklite-web.service..."
sudo systemctl restart docklite-web.service
log "docklite-web restarted."

log "Scheduling docklite-agent.service restart..."
# Use --no-block so this returns before systemd kills us (we're in the agent's cgroup)
sudo systemctl restart --no-block docklite-agent.service

log "=== Update complete — agent restarting ==="
