#!/usr/bin/env bash
# DockLite one-line installer.
#
#   curl -fsSL https://raw.githubusercontent.com/girlypopicon/docklite/main/get.sh | bash
#
# Look first, change nothing:
#   curl -fsSL https://raw.githubusercontent.com/girlypopicon/docklite/main/get.sh | bash -s -- --dry-run
#
# What it does: makes sure git is present, downloads DockLite into /usr/local/src/docklite (or updates it
# there on a re-run), then starts install.sh, which asks its questions at your keyboard. Running the same
# command again later upgrades DockLite and keeps your data and settings.
#
# Options:  --dry-run         report what is on this server and what would happen; change nothing
#           --ref NAME        install a branch or release tag instead of main (also: DOCKLITE_REF)
#           anything else is passed on to install.sh
# Run it as yourself (no sudo): it asks for your sudo password itself, only for the steps that need it.
# Piping into `sudo bash` breaks the installer's questions, so it is refused.
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

    # `curl | sudo bash` runs everything inside sudo's own pseudo-terminal, which gets none of your keystrokes
    # (not even Ctrl+C) when the script comes from a pipe. So: run as yourself, and let this script call sudo.
    if [ -n "${SUDO_USER:-}" ] && [ ! -t 0 ]; then
        die "Run it without sudo, so the installer can read your answers:  curl -fsSL <this url> | bash"
    fi

    local SUDO=""
    if [ "$(id -u)" -ne 0 ]; then
        if [ "$dry" -eq 0 ]; then
            command -v sudo >/dev/null 2>&1 || die "sudo is needed to install. Install it or run as root."
            say "DockLite installs system packages and services, so it needs sudo. You may be asked for your password."
            sudo -v || die "Could not get sudo access."
            SUDO="sudo"
        else
            # A dry run only reads; keep its download in a temp folder and skip installing git for it.
            src="$(mktemp -d)/docklite"
        fi
    fi

    if ! command -v git >/dev/null 2>&1; then
        [ -n "$SUDO" ] || [ "$(id -u)" -eq 0 ] || die "git is missing; install it (sudo apt install git) and run again."
        say "Installing git..."
        $SUDO apt-get update -qq >/dev/null 2>&1 || true
        $SUDO apt-get install -y -qq git ca-certificates >/dev/null 2>&1 || die "Could not install git."
    fi

    if [ -d "$src/.git" ]; then
        say "Updating DockLite in $src (ref: $ref)..."
        $SUDO git -C "$src" fetch --quiet --depth 1 origin "$ref" || die "Could not fetch '$ref' from $repo"
        $SUDO git -C "$src" checkout --quiet --force FETCH_HEAD
    else
        say "Downloading DockLite (ref: $ref)..."
        $SUDO mkdir -p "$(dirname "$src")"
        $SUDO git clone --quiet --depth 1 --branch "$ref" "$repo" "$src" || die "Could not download '$ref' from $repo"
    fi
    say "Got $($SUDO git -C "$src" describe --tags --always 2>/dev/null || $SUDO git -C "$src" rev-parse --short HEAD)"
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
    exec $SUDO bash "$src/install.sh" ${args[@]+"${args[@]}"}
}

main "$@"
