#!/usr/bin/env bash
# DockLite demo mode: a separate, throwaway DockLite instance full of fake data, for screenshots,
# testing and demos. It never touches the real nginx, certificates or your real containers:
#   scripts/demo.sh up       build, start the demo (agent :3100, dashboard :3102) and seed it
#   scripts/demo.sh status   is it running? where to open it
#   scripts/demo.sh login    print the demo login (the password lives in a private file)
#   scripts/demo.sh down     stop it and remove its containers (add --wipe to delete its data too)
# Everything lives in ~/docklite-demo (override with DOCKLITE_DEMO_DIR). Demo containers are tagged with
# the label docklite.demo=1, so a real DockLite never lists them and the demo never lists real ones.
set -euo pipefail

REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DIR="${DOCKLITE_DEMO_DIR:-$HOME/docklite-demo}"
AGENT_PORT="${DOCKLITE_DEMO_AGENT_PORT:-3100}"
GUI_PORT="${DOCKLITE_DEMO_GUI_PORT:-3102}"
# Site files live outside $HOME so screenshots of mount paths don't contain your username.
SITES="${DOCKLITE_DEMO_SITES_DIR:-/tmp/docklite-demo/sites}"
API="http://127.0.0.1:${AGENT_PORT}"

die() { echo "demo: $*" >&2; exit 1; }
alive() { [[ -f "$1" ]] && kill -0 "$(cat "$1")" 2>/dev/null; }
secret() { [[ -f "$DIR/$1" ]] || { umask 077; openssl rand -hex "${2:-24}" > "$DIR/$1"; }; cat "$DIR/$1"; }

demo_containers() { docker ps -aq --filter label=docklite.demo=1; }

up() {
    shift || true
    mkdir -p "$DIR"/{bin,data,logs,backups} "$SITES"
    chmod 700 "$DIR"
    local token session pass
    token="$(secret .token)"; session="$(secret .session 48)"; pass="$(secret .demo-password 12)"

    cp "$REPO/VERSION" "$DIR/VERSION"   # a real install has VERSION next to bin/; the demo's agent runs from here
    echo "Building the agent..."
    (cd "$REPO/go-app" && go build -o "$DIR/bin/docklite-agent" ./cmd/docklite-agent)
    # The demo has its own build folder, so it never touches (or needs write access to) an installed build.
    if [[ ! -f "$REPO/webapp/.next-demo/BUILD_ID" || "${1:-}" == "--rebuild" ]]; then
        echo "Building the dashboard (a minute or two)..."
        (cd "$REPO/webapp" && NEXT_DIST_DIR=.next-demo npm run build >"$DIR/logs/build.log" 2>&1) \
            || die "dashboard build failed; see $DIR/logs/build.log"
    fi

    if ! alive "$DIR/gui.pid"; then
        # exec so the recorded pid is node itself; detach from our output so callers don't wait on it
        ( cd "$REPO/webapp" && exec env PORT="$GUI_PORT" HOSTNAME=127.0.0.1 NODE_ENV=production \
            DATABASE_PATH="$DIR/data/docklite.db" AGENT_URL="$API" AGENT_TOKEN="$token" SESSION_SECRET="$session" \
            SEED_ADMIN_USERNAME=demo SEED_ADMIN_PASSWORD="$pass" DOCKLITE_DEMO=1 DOCKLITE_SITES_DIR="$SITES" NEXT_DIST_DIR=.next-demo \
            node node_modules/.bin/next start -H 127.0.0.1 -p "$GUI_PORT" ) >"$DIR/logs/gui.log" 2>&1 </dev/null &
        echo $! > "$DIR/gui.pid"
    fi
    # the dashboard creates the database schema and the demo admin on first start
    local i; for i in $(seq 1 60); do
        [[ -s "$DIR/data/docklite.db" ]] && sqlite3 -readonly "$DIR/data/docklite.db" "select 1 from users limit 1" 2>/dev/null | grep -q 1 && break
        sleep 1
    done
    # A pretend Cloudflare (example.* zones) so the Cloudflare screens work without any real account or token.
    if ! alive "$DIR/cf.pid"; then
        ( exec python3 "$REPO/scripts/demo_cloudflare.py" 3199 ) >"$DIR/logs/cloudflare.log" 2>&1 </dev/null &
        echo $! > "$DIR/cf.pid"
        sleep 1
    fi
    if ! alive "$DIR/agent.pid"; then
        ( exec env DOCKLITE_CLOUDFLARE_API="http://127.0.0.1:3199" DOCKLITE_GITHUB_API="http://127.0.0.1:3199" DOCKLITE_PUBLIC_IP="203.0.113.10" LISTEN_ADDR="127.0.0.1:${AGENT_PORT}" DATABASE_PATH="$DIR/data/docklite.db" DOCKLITE_TOKEN="$token" \
            NEXTJS_URL="http://127.0.0.1:${GUI_PORT}" DOCKLITE_DEMO=1 DOCKLITE_SITES_DIR="$SITES" \
            BACKUP_BASE_DIR="$DIR/backups" "$DIR/bin/docklite-agent" ) >"$DIR/logs/agent.log" 2>&1 </dev/null &
        echo $! > "$DIR/agent.pid"
    fi
    for i in $(seq 1 30); do curl -sf -o /dev/null -H "Authorization: Bearer $token" "$API/api/containers" && break; sleep 1; done
    curl -sf -o /dev/null -H "Authorization: Bearer $token" "$API/api/containers" || die "the demo agent didn't start; see $DIR/logs/agent.log"

    DEMO_API="$API" DEMO_TOKEN="$token" DEMO_SITES="$SITES" python3 "$REPO/scripts/demo_seed.py"
    status
}

status() {
    if alive "$DIR/agent.pid" && alive "$DIR/gui.pid"; then
        echo "Demo is running: dashboard http://127.0.0.1:${AGENT_PORT} (login: scripts/demo.sh login)"
        echo "  $(demo_containers | wc -l) demo containers; data in $DIR"
    else
        echo "Demo is not running."
    fi
}

login() { [[ -f "$DIR/.demo-password" ]] || die "no demo yet; run: scripts/demo.sh up"; echo "username: demo"; echo "password: $(cat "$DIR/.demo-password")"; }

down() {
    local f; for f in agent gui cf; do
        if alive "$DIR/$f.pid"; then kill "$(cat "$DIR/$f.pid")" 2>/dev/null || true; fi
        rm -f "$DIR/$f.pid"
    done
    local ids; ids="$(demo_containers)"
    [[ -n "$ids" ]] && docker rm -f $ids >/dev/null
    echo "Demo stopped; demo containers removed."
    if [[ "${1:-}" == "--wipe" ]]; then rm -rf "$DIR" "$(dirname "$SITES")"; echo "Demo data deleted."; fi
}

case "${1:-}" in
    up) up "$@" ;;
    status) status ;;
    login) login ;;
    down) down "${2:-}" ;;
    *) sed -n '2,8p' "$0"; exit 2 ;;
esac
