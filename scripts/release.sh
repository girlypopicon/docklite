#!/usr/bin/env bash
# DockLite versions and releases. Keeps VERSION, webapp/package.json, CHANGELOG.md and the git tag in step.
#
#   scripts/release.sh status                  where are we? flags anything out of step
#   scripts/release.sh prepare patch|minor|major|X.Y.Z
#                                              on a branch: bump the files and commit "Release vX.Y.Z" (no tag yet)
#   scripts/release.sh tag [--push]            on main, after that branch is merged: tag the release (and push the tag)
#   scripts/release.sh publish                 create the GitHub release (with notes from CHANGELOG.md) for the current version
#   add --dry-run to prepare to preview it
#
# Why tags matter: the dashboard's Update button looks for the newest release on GitHub and installs exactly that
# tag. A version without a pushed tag is invisible to every server.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."

cmd="${1:-status}"; shift || true
dry=0; push=0
for a in "$@"; do case "$a" in --dry-run) dry=1 ;; --push) push=1 ;; esac; done

cur="$(tr -d '[:space:]' < VERSION)"
pkg="$(node -p "require('./webapp/package.json').version" 2>/dev/null || echo "?")"
last_tag="$(git describe --tags --abbrev=0 2>/dev/null || echo none)"
since=0; [[ "$last_tag" != none ]] && since="$(git rev-list --count "${last_tag}..HEAD")"
unrel="$(awk '/^## Unreleased/{f=1;next} /^## /{f=0} f&&/^- /{n++} END{print n+0}' CHANGELOG.md)"
remote="$(git remote -v | awk '/girlypopicon\/docklite(\.git)? \(push\)/{print $1; exit}')"
remote="${remote:-origin}"

section() { # the CHANGELOG text for one version, for release notes
    awk -v v="$1" '$0 ~ "^## "v"( |$)" {f=1; next} /^## /{f=0} f' CHANGELOG.md | sed -e :a -e '/^\n*$/{$d;N;ba' -e '}'
}

status() {
    echo "VERSION file:        $cur"
    echo "webapp/package.json: $pkg"
    echo "latest git tag:      $last_tag   ($since commits since)"
    echo "unreleased changelog entries: $unrel"
    local rc=0
    [[ "$cur" == "$pkg" ]] || { echo "!! VERSION and package.json disagree"; rc=1; }
    if [[ "$since" -gt 0 && "$unrel" -eq 0 && "v$cur" == "$last_tag" ]]; then
        echo "!! $since commits since $last_tag but nothing under '## Unreleased' in CHANGELOG.md"; rc=1
    fi
    if [[ "$last_tag" != none && "v$cur" != "$last_tag" && "$unrel" -eq 0 ]]; then
        echo "-- $cur is prepared but not tagged yet: merge it, then run: scripts/release.sh tag --push"
    fi
    [[ "$rc" -eq 0 ]] && echo "ok"
    return $rc
}

case "$cmd" in
status|check) status; exit $? ;;

prepare)
    target="${1:-}"
    [[ "$cur" == "$pkg" ]] || { echo "VERSION ($cur) and package.json ($pkg) disagree; fix first" >&2; exit 1; }
    IFS=. read -r MA MI PA <<<"$cur"
    case "$target" in
        patch) new="$MA.$MI.$((PA+1))" ;;
        minor) new="$MA.$((MI+1)).0" ;;
        major) new="$((MA+1)).0.0" ;;
        [0-9]*.[0-9]*.[0-9]*) new="$target" ;;
        *) echo "usage: scripts/release.sh prepare patch|minor|major|X.Y.Z [--dry-run]" >&2; exit 2 ;;
    esac
    [[ "$unrel" -gt 0 ]] || { echo "Nothing under '## Unreleased' in CHANGELOG.md; add entries first" >&2; exit 1; }
    git rev-parse -q --verify "refs/tags/v$new" >/dev/null && { echo "tag v$new already exists" >&2; exit 1; }
    echo "Preparing v$new (was $cur, $unrel changelog entries)"
    [[ "$dry" -eq 1 ]] && { echo "(dry run; nothing changed)"; exit 0; }
    [[ -z "$(git status --porcelain)" ]] || { echo "Working tree not clean; commit or stash first" >&2; exit 1; }
    echo "$new" > VERSION
    node -e "const f='webapp/package.json',fs=require('fs');const j=JSON.parse(fs.readFileSync(f));j.version='$new';fs.writeFileSync(f,JSON.stringify(j,null,2)+'\n')"
    # Unreleased becomes the new version's section, and a fresh empty Unreleased goes on top.
    sed -i "s/^## Unreleased.*/## Unreleased\n\n## $new ($(date +%Y-%m-%d))/" CHANGELOG.md
    git add VERSION webapp/package.json CHANGELOG.md
    git commit -q -m "Release v$new"
    echo "Done: version is now $new on branch '$(git branch --show-current)'. After it is merged to main:"
    echo "  scripts/release.sh tag --push && scripts/release.sh publish"
    ;;

tag)
    [[ "$cur" == "$pkg" ]] || { echo "VERSION and package.json disagree" >&2; exit 1; }
    [[ -z "$(git status --porcelain)" ]] || { echo "Working tree not clean" >&2; exit 1; }
    branch="$(git branch --show-current)"
    [[ "$branch" == "main" ]] || { echo "Tag from main after the release branch is merged (you are on '$branch')." >&2; exit 1; }
    git rev-parse -q --verify "refs/tags/v$cur" >/dev/null && { echo "v$cur is already tagged" >&2; exit 1; }
    git tag -a "v$cur" -m "DockLite v$cur"
    echo "Tagged v$cur at $(git rev-parse --short HEAD)."
    if [[ "$push" -eq 1 ]]; then git push "$remote" "v$cur"; echo "Pushed v$cur to $remote."; else echo "Publish it:  git push $remote v$cur"; fi
    ;;

publish)
    command -v gh >/dev/null || { echo "gh (GitHub CLI) is needed" >&2; exit 1; }
    git rev-parse -q --verify "refs/tags/v$cur" >/dev/null || { echo "Tag v$cur doesn't exist yet; run: scripts/release.sh tag --push" >&2; exit 1; }
    notes="$(section "$cur")"
    [[ -n "$notes" ]] || notes="DockLite $cur"
    gh release create "v$cur" --repo girlypopicon/docklite --title "DockLite $cur" --notes "$notes" --verify-tag
    echo "Published release v$cur."
    ;;

*) echo "usage: scripts/release.sh status|prepare|tag|publish" >&2; exit 2 ;;
esac
