#!/usr/bin/env bash
# Builds the public DockLite demo (demo.docklite.net) as plain static files in dist/demo/.
# Everything the demo shows is generated in the visitor's browser from fake data, so the result is
# just HTML, JS and CSS: serve the folder with nginx (or any static host); no Node, no database.
#
#   scripts/build-public-demo.sh            build into dist/demo
#   scripts/build-public-demo.sh --serve    also serve it on http://127.0.0.1:3300 for a look
set -euo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT="$REPO/dist/demo"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

[[ -d "$REPO/webapp/node_modules" ]] || { echo "Run 'npm install' in webapp/ first." >&2; exit 1; }

# Build in a scratch copy so the real webapp folder (and any installed build in it) is never touched.
rsync -a --exclude node_modules --exclude .next --exclude '.next-*' --exclude out "$REPO/webapp/" "$WORK/webapp/"
ln -s "$REPO/webapp/node_modules" "$WORK/webapp/node_modules"
# Developer-only pages have no place in a public demo.
rm -rf "$WORK/webapp/app/debug" "$WORK/webapp/app/debug-auth" "$WORK/webapp/app/emergency"
(cd "$WORK/webapp" && DOCKLITE_DEMO_EXPORT=1 NEXT_PUBLIC_DOCKLITE_DEMO=1 npm run build)

rm -rf "$OUT"; mkdir -p "$(dirname "$OUT")"
mv "$WORK/webapp/out" "$OUT"
echo "Built $(find "$OUT" -type f | wc -l) files ($(du -sh "$OUT" | cut -f1)) in $OUT"

if [[ "${1:-}" == "--serve" ]]; then
    echo "Serving on http://127.0.0.1:3300 (Ctrl-C to stop)"
    cd "$OUT" && exec python3 -m http.server 3300 --bind 127.0.0.1
fi
