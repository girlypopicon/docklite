#!/usr/bin/env bash
# DockLite version helper. Keeps VERSION, webapp/package.json and CHANGELOG.md in step.
#
#   scripts/release.sh status              where are we? (version, last tag, unreleased work)
#   scripts/release.sh patch|minor|major   cut a release (or give an exact X.Y.Z)
#   add --dry-run to see what would happen without changing anything
#
# A release: moves "## Unreleased" in CHANGELOG.md under the new version, writes the version
# into VERSION and webapp/package.json, commits "Release vX.Y.Z" and tags it locally.
# It never pushes; it prints the commands to do that.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."

cmd="${1:-status}"; dry=0
[[ "${2:-}" == "--dry-run" || "${1:-}" == "--dry-run" ]] && dry=1

cur="$(tr -d '[:space:]' < VERSION)"
pkg="$(node -p "require('./webapp/package.json').version" 2>/dev/null || echo "?")"
last_tag="$(git describe --tags --abbrev=0 2>/dev/null || echo none)"
since=0; [[ "$last_tag" != none ]] && since="$(git rev-list --count "${last_tag}..HEAD")"
unrel="$(awk '/^## Unreleased/{f=1;next} /^## /{f=0} f&&/^- /{n++} END{print n+0}' CHANGELOG.md)"

status() {
  echo "VERSION file:        $cur"
  echo "webapp/package.json: $pkg"
  echo "latest git tag:      $last_tag   ($since commits since)"
  echo "unreleased changelog entries: $unrel"
  local rc=0
  [[ "$cur" == "$pkg" ]] || { echo "!! VERSION and package.json disagree"; rc=1; }
  if [[ "$last_tag" != none && "v$cur" != "$last_tag" && "$since" -eq 0 ]]; then
    echo "!! HEAD is the latest tag but VERSION says $cur"; rc=1
  fi
  if [[ "$since" -gt 0 && "$unrel" -eq 0 ]]; then
    echo "!! $since commits since $last_tag but nothing under '## Unreleased' in CHANGELOG.md"; rc=1
  fi
  [[ "$rc" -eq 0 ]] && echo "ok"
  return $rc
}

if [[ "$cmd" == "status" || "$cmd" == "check" ]]; then status; exit $?; fi

[[ "$cur" == "$pkg" ]] || { echo "VERSION ($cur) and package.json ($pkg) disagree; fix first" >&2; exit 1; }
IFS=. read -r MA MI PA <<<"$cur"
case "$cmd" in
  patch) new="$MA.$MI.$((PA+1))";;
  minor) new="$MA.$((MI+1)).0";;
  major) new="$((MA+1)).0.0";;
  [0-9]*.[0-9]*.[0-9]*) new="$cmd";;
  *) echo "usage: scripts/release.sh status|patch|minor|major|X.Y.Z [--dry-run]" >&2; exit 2;;
esac
# A branch may already be ahead (VERSION bumped early): releasing that same number is fine.
[[ "$new" != "$cur" || "$last_tag" != "v$cur" ]] || { echo "v$cur is already tagged" >&2; exit 1; }
[[ "$unrel" -gt 0 ]] || { echo "Nothing under '## Unreleased' in CHANGELOG.md; add entries first" >&2; exit 1; }
git rev-parse -q --verify "refs/tags/v$new" >/dev/null && { echo "tag v$new exists" >&2; exit 1; }

echo "Releasing v$new (was $cur, $unrel changelog entries)"
if [[ "$dry" -eq 1 ]]; then echo "(dry run; nothing changed)"; exit 0; fi

[[ -z "$(git status --porcelain)" ]] || { echo "Working tree not clean; commit or stash first" >&2; exit 1; }
branch="$(git branch --show-current)"
[[ "$branch" == "main" ]] || echo "note: you are on '$branch', not main. Tag from main after merging if you can." >&2

echo "$new" > VERSION
node -e "const f='webapp/package.json',fs=require('fs');const j=JSON.parse(fs.readFileSync(f));j.version='$new';fs.writeFileSync(f,JSON.stringify(j,null,2)+'\n')"
sed -i "s/^## Unreleased.*/## $new ($(date +%Y-%m-%d))/" CHANGELOG.md
git add VERSION webapp/package.json CHANGELOG.md
git commit -q -m "Release v$new"
git tag -a "v$new" -m "DockLite v$new"
echo "Done: committed and tagged v$new locally. To publish:"
echo "  git push <remote> $branch && git push <remote> v$new"
