# Using DockLite from the command line

DockLite is a control panel for a server: it runs websites and databases in Docker containers, puts nginx in
front of them, gets HTTPS certificates, and manages backups and DNS. Everything the web dashboard does can also
be done with the `docklite` command. This guide is written for people **and for AI assistants** (Claude and
others) that have been given a shell on a DockLite server: read it once and you can operate DockLite safely.

Print this guide any time with `docklite docs`. List every command as JSON with `docklite commands --json`.

## Start here

1. `docklite doctor` — checks that DockLite, Docker, nginx and certificates are healthy and says how to fix
   anything that isn't. Run it first; it also tells you which account you are using.
2. `docklite containers list` — what is running (sites, databases, other), running ones first.
3. `docklite help <command>` — usage and examples for one command.

## Two kinds of command

- **Launcher commands** (`docklite setup | upgrade | start-all | stop | restart | logs | tui | status`) manage the
  DockLite program itself. They run **on the server** and usually need `sudo`.
- **Everything else** (`containers`, `sites`, `ssl`, `nginx`, `dns`, `server`, `users`, `backups`, `doctor`,
  `access`...) talks to DockLite's API. These work on the server **and from any other machine** once you have
  logged in to the server's address.

## Getting access (how the CLI finds credentials)

The CLI uses the first of these that applies:

1. `--token` / `--host` flags, or the `DOCKLITE_TOKEN` / `DOCKLITE_HOST` environment variables.
2. **A saved login:** `docklite login` asks for your DockLite username and password and saves a personal token in
   `~/.config/docklite/config.json`. You can then do whatever *your DockLite account* may do (user, admin or
   super admin). To log in to a remote server: `docklite --host https://panel.example.com login`.
3. **Admin shell access:** if you are on the server and your server user was added with
   `docklite access grant <your-user>` (it joins the `docklite` group), the CLI reads the server's own config and
   acts as a full DockLite admin with no login. Treat this like root for DockLite.

`docklite whoami` shows which of these is in use. Tokens are never printed by any command.

## Output and exit codes

- Add `--json` to any command for machine-readable output (one JSON document on stdout). Without it you get
  readable tables. **Parse `--json`, never the tables.**
- Errors go to stderr as `error: ...` (or `{"error":...,"code":N}` with `--json`).
- Exit codes: `0` ok · `1` failed · `2` wrong usage · `3` not logged in / not allowed · `4` not found ·
  `5` the server refused (already exists, invalid) · `6` can't reach DockLite · `7` destructive command needs `--yes`.

## Names you can use

Wherever a command takes a container, you can give its **name**, a site's **domain** (`example.com`), a full id,
or a unique id prefix. If a name is ambiguous the command stops and lists the matches; it never guesses.

## Safety rules (especially for AI assistants)

DockLite usually hosts **live websites and databases**. Be careful and be conservative:

- **Look before you change.** Run the read-only command (`containers list`, `doctor`, `ssl status`,
  `server services`) and confirm the target before any change.
- **Destructive commands** are marked in `docklite commands --json` (`"destructive": true`) and in the reference
  below. Run interactively they ask "y/N". Run by a script or assistant they **refuse with exit code 7 unless you
  pass `--yes`**. Only add `--yes` when the person you are working for has clearly asked for that exact action on
  that exact target. If you get exit code 7, tell them what you wanted to do and ask.
- **Never stop nginx/the web proxy or DockLite itself** unless explicitly told to; every site goes down with it.
  (`server service proxy stop` is refused by design; stopping Traefik, which is normally unused, is fine.)
- **Backups before big changes** where you can; `docklite backups list` shows what exists.
- Don't paste tokens, passwords or the contents of `/opt/docklite/.docklite.conf` into chat, tickets or commits.
- Prefer small steps and re-check with `docklite doctor` afterwards.

## Common tasks

| I want to... | Run |
|---|---|
| Check everything is healthy | `docklite doctor` |
| See what's running | `docklite containers list` (add `--kind site`, `--running`) |
| Restart a site | `docklite containers restart example.com` |
| Read a site's logs | `docklite containers logs example.com --tail 200` |
| Create a website | `docklite sites create example.com` (`--type php` or `--type node --port 3000`) |
| Get HTTPS for a site | `docklite ssl issue example.com --www --email me@example.com` (DNS must point here first) |
| See certificates and expiry | `docklite ssl status` |
| Check/reload nginx | `docklite nginx test` then `docklite nginx reload` |
| Why is the server slow? | `docklite server overview`, `docklite server services`, `docklite server logs system` |
| Add a DockLite account | `docklite users create alice` (add `--admin`; use `--password-stdin` in scripts) |
| Let a server user run docklite | `docklite access grant alice --yes` (they must log out and in once) |
| Remove that access | `docklite access revoke alice --yes` |
| See all backups | `docklite backups list` |
| Check sites follow `/var/www/sites/<user>/<domain>` | `docklite sites layout` (report only) |
| Move sites into that layout | `docklite sites layout --apply --yes` (copies; old folders are kept) |
| See if a newer DockLite exists | `docklite update --check` |
| Update DockLite (sites keep running) | `docklite update --yes` (waits until it's running; rolls back by itself if it fails) |
| Stop an unused service | `docklite server service traefik stop --yes` |

## Concepts

- **Site**: a website DockLite runs in a container, from a folder under `/var/www/sites/<user>/<domain>/`.
  Types: `static` (HTML), `php`, `node`. nginx sends the domain's traffic to it.
- **Kind**: every container is a `site`, a `database`, or `other` (things DockLite did not create).
- **Admin**: DockLite accounts are `user`, `admin` or `super_admin`. Users see only their own things.
- **Root helper**: DockLite never runs as root. Anything needing root (nginx files, certificates) goes through
  `/usr/local/sbin/docklite-helper`, which only accepts a short list of validated operations.

## When something goes wrong

- `docklite doctor` first. Exit code `6` means DockLite isn't reachable: on the server run `docklite start-all`,
  or check `--host`.
- Exit code `3`: run `docklite login`, or ask an admin for access.
- A site is down: `docklite containers list`, then `docklite containers logs <site>`, then
  `docklite server logs proxy`.
- Certificate problems: `docklite ssl status`; a renewal can be blocked while another certificate task runs;
  wait a minute and retry.
