#!/usr/bin/env bash
# Sandbox test for the root helper's site-folder commands (site-trash, site-purge):
# fake /var/www/sites tree in a temp folder, no root, nothing outside it touched.
set -uo pipefail
HELPER="$(cd "$(dirname "$0")/.." && pwd)/docklite-helper"
T="$(mktemp -d)"; trap 'rm -rf "$T"' EXIT
pass=0; failn=0
check() { if [[ "$2" == "$3" ]]; then echo "  ok   $1"; pass=$((pass+1)); else echo "  FAIL $1 (wanted '$3', got '$2')"; failn=$((failn+1)); fi; }

S="$T/var/www/sites"; mkdir -p "$S/alice/old.example.com/assets" "$S/stray.example.com" "$S/outside"
echo hello > "$S/alice/old.example.com/index.html"; echo a > "$S/alice/old.example.com/assets/app.js"
echo x > "$S/stray.example.com/index.php"
ln -s "$S/outside" "$S/alice/link.example.com"

H="$T/helper.sh"
# chown needs root; everything else runs as is.
sed -e "s#SITES_ROOT=/var/www/sites#SITES_ROOT=$S#" -e 's#^\( *\)chown #\1true chown #' "$HELPER" > "$H"
run() { bash "$H" "$@" 2>&1; }

echo "1. site-trash moves a folder into .trash under the given name"
run site-trash alice/old.example.com 20260101-120000-alice__old.example.com >/dev/null; check "exit ok" "$?" "0"
check "old place is empty" "$([[ -e "$S/alice/old.example.com" ]] && echo there || echo gone)" "gone"
check "files arrived" "$(cat "$S/.trash/20260101-120000-alice__old.example.com/assets/app.js")" "a"
run site-trash stray.example.com 20260101-120001-stray.example.com >/dev/null; check "top-level folder too" "$?" "0"

echo "2. site-trash refuses anything that is not a plain site folder"
out="$(run site-trash ../etc 20260101-120000-x)"; check "no .." "${out#docklite-helper: }" "invalid site folder: ../etc"
out="$(run site-trash .trash 20260101-120000-x)"; check "no hidden" "${out#docklite-helper: }" "invalid site folder: .trash"
out="$(run site-trash alice/link.example.com 20260101-120000-alice__link.example.com)"; check "no symlink" "${out#docklite-helper: }" "no such site folder: alice/link.example.com"
check "symlink target untouched" "$([[ -d "$S/outside" ]] && echo there)" "there"
out="$(run site-trash alice/a/b 20260101-120000-x)"; check "no deeper paths" "${out#docklite-helper: }" "invalid site folder: alice/a/b"
out="$(run site-trash alice junk-name)"; check "trash name must carry a stamp" "${out#docklite-helper: }" "invalid trash entry: junk-name"
out="$(run site-trash alice 20260101-120000-alice__old.example.com)"; check "never overwrites a trash entry" "${out#docklite-helper: }" "already in the trash: 20260101-120000-alice__old.example.com"
check "alice still there" "$([[ -d "$S/alice" ]] && echo there)" "there"

echo "3. site-purge deletes one trash entry and nothing else"
run site-purge 20260101-120000-alice__old.example.com >/dev/null; check "exit ok" "$?" "0"
check "entry gone" "$([[ -e "$S/.trash/20260101-120000-alice__old.example.com" ]] && echo there || echo gone)" "gone"
check "other entry kept" "$(cat "$S/.trash/20260101-120001-stray.example.com/index.php")" "x"
out="$(run site-purge 20260101-120000-alice__old.example.com)"; check "missing entry is an error" "${out#docklite-helper: }" "not in the trash: 20260101-120000-alice__old.example.com"
out="$(run site-purge ../alice)"; check "no paths" "${out#docklite-helper: }" "invalid trash entry: ../alice"
out="$(run site-purge 20260101-120000-../../alice)"; check "no .. inside a name" "${out#docklite-helper: }" "invalid trash entry: 20260101-120000-../../alice"
ln -s "$S/alice" "$S/.trash/20260101-120002-alice"
out="$(run site-purge 20260101-120002-alice)"; check "no symlinked entries" "${out#docklite-helper: }" "not in the trash: 20260101-120002-alice"
check "alice still there after the symlink trick" "$([[ -d "$S/alice" ]] && echo there)" "there"

echo; echo "$pass passed, $failn failed"
[[ $failn -eq 0 ]]
