#!/usr/bin/env bash
# DockLite one-line installer.
#
#   curl -fsSL https://raw.githubusercontent.com/girlypopicon/docklite/main/get.sh | sudo bash
#
# Look first, change nothing:
#   curl -fsSL https://raw.githubusercontent.com/girlypopicon/docklite/main/get.sh | sudo bash -s -- --dry-run
#
# What it does: makes sure git is present, downloads DockLite into /usr/local/src/docklite (or updates it
# there on a re-run), then starts install.sh, which asks its questions at your keyboard. Running the same
# command again later upgrades DockLite and keeps your data and settings.
#
# Options:  --dry-run         report what is on this server and what would happen; change nothing
#           --ref NAME        install a branch or release tag instead of main (also: DOCKLITE_REF)
#           anything else is passed on to install.sh
# Prefer to read the code first? git clone https://github.com/girlypopicon/docklite.git && sudo bash docklite/install.sh
#
# Everything is inside main() so a download that is cut off halfway can never run a half-script.
set -euo pipefail

main() {
    local repo="${DOCKLITE_REPO:-https://github.com/girlypopicon/docklite.git}"
    local ref="${DOCKLITE_REF:-main}"
    local src="${DOCKLITE_SRC:-/usr/local/src/docklite}"
    local dry=0 args=()

    while [ $# -gt 0 ]; do
        case "$1" in
            --ref)   ref="${2:?--ref needs a branch or tag name}"; shift 2 ;;
            --ref=*) ref="${1#*=}"; shift ;;
            --dry-run) dry=1; args+=("$1"); shift ;;
            *) args+=("$1"); shift ;;
        esac
    done

    say()  { printf '  %s\n' "$*"; }
    die()  { printf '  ✗ %s\n' "$*" >&2; exit 1; }

    echo
    say "DockLite installer"
    echo

    command -v apt-get >/dev/null 2>&1 || die "This installer supports Ubuntu and Debian (it needs apt)."
    if [ "$dry" -eq 0 ] && [ "$(id -u)" -ne 0 ]; then
        die "Run it with sudo:  curl -fsSL <this url> | sudo bash"
    fi

    # A dry run only reads, so it works without root and leaves nothing behind but a temp folder.
    if [ "$dry" -eq 1 ] && [ "$(id -u)" -ne 0 ]; then
        src="$(mktemp -d)/docklite"
    fi

    if ! command -v git >/dev/null 2>&1; then
        [ "$(id -u)" -eq 0 ] || die "git is missing; install it (sudo apt install git) and run again."
        say "Installing git..."
        apt-get update -qq >/dev/null 2>&1 || true
        apt-get install -y -qq git ca-certificates >/dev/null 2>&1 || die "Could not install git."
    fi

    if [ -d "$src/.git" ]; then
        say "Updating DockLite in $src (ref: $ref)..."
        git -C "$src" fetch --quiet --depth 1 origin "$ref" || die "Could not fetch '$ref' from $repo"
        git -C "$src" checkout --quiet --force FETCH_HEAD
    else
        say "Downloading DockLite (ref: $ref)..."
        mkdir -p "$(dirname "$src")"
        git clone --quiet --depth 1 --branch "$ref" "$repo" "$src" || die "Could not download '$ref' from $repo"
    fi
    say "Got $(git -C "$src" describe --tags --always 2>/dev/null || git -C "$src" rev-parse --short HEAD)"
    [ -f "$src/install.sh" ] || die "install.sh is missing from the download."

    # We were piped into bash, so stdin is the script. The installer asks questions, so give it the keyboard.
    if [ ! -t 0 ]; then
        if [ -r /dev/tty ] && { : 2>/dev/null </dev/tty; } 2>/dev/null; then
            exec </dev/tty
        elif [ "$dry" -eq 0 ]; then
            die "The installer asks questions and needs a terminal. Log in over SSH and run the command there."
        fi
    fi

    echo
    exec bash "$src/install.sh" ${args[@]+"${args[@]}"}
}

main "$@"
