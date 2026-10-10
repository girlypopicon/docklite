#!/usr/bin/env bash
# Sandbox test for the root helper's self-update: no root, no network, nothing outside a temp folder.
# It builds a fake GitHub repository with releases, a fake installer and a fake health endpoint, then
# runs the real `update-job` code against them: success, failed install with rollback, missing release.
set -uo pipefail
HELPER="$(cd "$(dirname "$0")/.." && pwd)/docklite-helper"
T="$(mktemp -d)"; trap 'kill $SRV 2>/dev/null; rm -rf "$T"' EXIT
pass=0; failn=0
check() { if [[ "$2" == "$3" ]]; then echo "  ok   $1"; pass=$((pass+1)); else echo "  FAIL $1 (wanted '$3', got '$2')"; failn=$((failn+1)); fi; }

# fake releases
R="$T/origin"; mkdir -p "$R"; git -C "$R" init -q
mkrelease() { # version installer-body
    echo "$1" > "$R/VERSION"; printf '#!/usr/bin/env bash\n%s\n' "$2" > "$R/install.sh"
    git -C "$R" add -A; git -C "$R" -c user.email=t@t -c user.name=t commit -qm "v$1"; git -C "$R" tag "v$1"
}
GOOD='echo "$VERSION_FILE_CONTENT_PLACEHOLDER" >/dev/null; cp "$(dirname "$0")/VERSION" "$INSTALL_DIR/VERSION"'
mkrelease 1.0.0 "$GOOD"
mkrelease 1.1.0 "$GOOD"
mkrelease 1.2.0 "$GOOD"
mkrelease 1.3.0 'echo "the new version changed the database" > "$INSTALL_DIR/data/docklite.db"; echo junk > "$INSTALL_DIR/data/docklite.db-wal"; echo "pretend the build blew up"; exit 1'

# a fake DockLite that answers like the real one
PORT=$((20000 + RANDOM % 10000))
printf 'HTTP/1.1 200 OK\r\nContent-Length: 2\r\n\r\nok' > /dev/null
python3 -m http.server "$PORT" --bind 127.0.0.1 --directory "$T" >/dev/null 2>&1 & SRV=$!
mkdir -p "$T/api" "$T/login"; echo ok > "$T/api/health"; echo ok > "$T/login/index.html"; : > "$T/login.html"
sleep 1

# the helper with its fixed paths pointed into the sandbox
H="$T/helper.sh"
sed -e "s#^UPDATE_STATE=.*#UPDATE_STATE=$T/state/update-state#" -e "s#^UPDATE_LOG=.*#UPDATE_LOG=$T/log/update.log#" \
    -e "s#^UPDATE_SRC=.*#UPDATE_SRC=$T/src#" -e "s#^UPDATE_REPO=.*#UPDATE_REPO=file://$R#" -e "s#^INSTALL_DIR=.*#INSTALL_DIR=$T/opt#" \
    -e "s#/var/lib/docklite#$T/state#g" -e "s#/var/log/docklite#$T/log#g" -e "s#/var/backups/docklite#$T/backups#g" \
    -e "s#/var/lock/docklite-update.lock#$T/lock#g" -e "s#/var/lock#$T#g" \
    -e 's#chown docklite:docklite#true #g' -e 's#sleep 5; waited=$((waited + 5))#sleep 1; waited=$((waited + 1))#' "$HELPER" > "$H"
# the real helper wants "/login" to answer 200; the python server serves the folder
sed -i 's#/login"#/login/"#' "$H"
mkdir -p "$T/opt/data" "$T/state" "$T/log"; echo "1.1.0" > "$T/opt/VERSION"; echo "AGENT_PORT=$PORT" > "$T/opt/.docklite.conf"; echo db > "$T/opt/data/docklite.db"
export INSTALL_DIR="$T/opt"

state() { cut -d' ' -f1,2 "$T/state/update-state"; }

echo "1. update 1.1.0 -> v1.2.0 (works)"
bash "$H" update-job v1.2.0; check "exit code" "$?" "0"
check "state" "$(state)" "success v1.2.0"
check "version installed" "$(cat "$T/opt/VERSION")" "1.2.0"
check "data backed up" "$(ls "$T"/backups/pre-update-1.1.0-to-1.2.0-*.tgz 2>/dev/null | wc -l | tr -d ' ')" "1"

echo "2. update to v1.3.0 whose install fails -> rolls back to 1.2.0"
bash "$H" update-job v1.3.0; check "exit code" "$?" "1"
check "state" "$(state)" "rolled-back v1.3.0"
check "still on the old version" "$(cat "$T/opt/VERSION")" "1.2.0"
check "database put back as it was before the update" "$(cat "$T/opt/data/docklite.db")" "db"
check "failed version's leftover journal removed" "$([ -e "$T/opt/data/docklite.db-wal" ] && echo present || echo gone)" "gone"
check "settings kept" "$(grep -c AGENT_PORT "$T/opt/.docklite.conf")" "1"

echo "3. a release that doesn't exist -> nothing changes"
bash "$H" update-job v9.9.9; check "exit code" "$?" "1"
check "state" "$(state)" "failed v9.9.9"
check "version untouched" "$(cat "$T/opt/VERSION")" "1.2.0"

echo "4. bad tags never get past validation"
for bad in "main" "v1.2" "1.2.3" 'v1.2.3;id' 'v1.2.3 v1.2.4' '../v1.2.3' ""; do
    bash "$H" update-start "$bad" >/dev/null 2>&1; r=$?
    check "rejects '$bad'" "$([ $r -ne 0 ] && echo rejected || echo ACCEPTED)" "rejected"
done

echo; echo "$pass passed, $failn failed"; [[ $failn -eq 0 ]]
