# Primer: revamping the DockLite GitHub presence

You are picking up a documentation / presentation task for **DockLite**, a self-hosted web-hosting and Docker
control panel. This file is everything the previous Claude session knew that you can't see in the code. Read it
fully before touching anything. The goal of the task: make the GitHub repo understandable, honest and attractive
to a stranger who lands on it. Nobody outside the owner has really used the software yet, so the README is the
whole first impression.

## 0. Ground rules (read first)

- **Owner:** Stella (GitHub `sgauth0`, also the `girlypopicon` account). Use they/them if you need a pronoun.
- **Do not push, merge, tag, or change repo settings.** Work on a branch, commit locally, and tell Stella what
  to review. Stella merges PRs themself. Commits end with the Co-Authored-By line your harness gives you.
- **Never type passwords, tokens or API keys anywhere, and never put them in files, commits or screenshots.**
  If you need to be logged in to the dashboard, ask Stella to log in; do not log in for them.
- **Privacy:** real domains, IPs, usernames and site names from Stella's servers must not appear in anything
  public (README, screenshots, docs). See "Screenshots" below.
- **Don't invent features.** Only document what exists. The roadmap section lists what is *planned*; label it so.
- Ask before deleting files from the repo root, even the obviously stale ones (see section 4).

## 1. What DockLite is

A control panel that turns one Linux box into a web host: create a site (static / PHP / Node) for a domain and
DockLite runs it in a Docker container, puts nginx in front, gets a Let's Encrypt certificate (HTTP or via
Cloudflare DNS), manages databases (Postgres containers), backups, DNS records, users with roles, and gives you a
browser dashboard, a command line, and a terminal UI. The longer-term idea: DockLite **owns** `/var/www/sites/`
and fully manages the Linux hosting side ("a self-hosted web host in a box"). It is the owner's main product and
they hope to monetize it eventually, so tone: confident, plain-language, not corporate.

Brand: neon / retro "vaporwave" look, a rainbow gradient (`#FF6AD5 #FF9A8B #FFD166 #B8F2A2 #9AD0FF #C7A4FF`,
see `BRAND.md`), a unicorn theme exists in the GUI. A future rename around the word **"Capsules"** is being
considered (a site/app packaged to move between servers). Do **not** rebrand now; just don't make that harder.

## 2. Architecture in one minute

```
Browser -> nginx (:80/:443) -> Go agent (:3000, API + reverse proxy) -> Docker
                                   |-> Next.js GUI (:3001+)
                                   '-> SQLite (data/docklite.db), shared by agent and GUI
```
- `go-app/` Go agent (API, Docker SDK, nginx/cert management via a root helper) and the `docklite` CLI
  (`go-app/cmd/docklite`, built to `bin/docklite-cli`, embedded guide in `cmd/docklite/docs/CLI.md`).
- `webapp/` Next.js 14 dashboard (Tailwind, xterm.js terminal, Phosphor icons). Pages in `webapp/app/(dashboard)/`.
- `docklite` (bash launcher: setup wizard, `upgrade`, `repair`, panel domain), `install.sh`, `uninstall.sh`,
  `docklite-helper` (the only root-privileged piece, a validated allowlist of operations), `inventory.sh`
  (read-only server report).
- Runs as the unprivileged `docklite` user under PM2. Installed in `/opt/docklite`. Sites in
  `/var/www/sites/<user>/<domain>/`. Config in `/opt/docklite/.docklite.conf` (never print it).
- Roles: `super_admin`, `admin`, `user`. Bearer tokens for API/CLI; cookie sessions for the GUI.
- TUI: a standalone Bubble Tea client. Its source is NOT in this repo (only binaries / `cli-repo/`); don't
  document its internals.

Existing, accurate technical docs: `CLAUDE.md` (architecture + API cheat sheet), `go-app/cmd/docklite/docs/CLI.md`
(the CLI guide; run `docklite docs`), `SSL.md`, `DATABASESPEC.md`. Verify any claim you reuse against the code.

## 3. Feature inventory (what actually works today)

Check each against the code or the live dashboard before claiming it:
- Sites: create static/PHP/Node, per-site container, nginx vhost, HTTPS (Let's Encrypt; Cloudflare DNS token for
  wildcard/proxied setups), `.dkl` manifest per site, file manager API, site transfer between users.
- Databases: Postgres containers, in-dashboard inspector/editor, per-user permissions.
- Backups: site and database backups with async progress and verification (restore UI is being finished).
- DNS: Cloudflare zones import and record editing; SSL mode controls.
- Server page: services (nginx, DockLite, Docker, Traefik), logs, health; web terminal into a container.
- Users and roles, audit log (write-only so far), "shell access" for server users via the `docklite` group.
- CLI: `docklite doctor`, `containers`, `sites`, `ssl`, `nginx`, `dns`, `users`, `backups`, `access`, always
  `--json`, refuses destructive actions without `--yes`. Written to be driven by AI assistants too.
- Safer adoption of existing servers: `inventory.sh`, `docklite sites layout` (normalize folders), `docklite repair`.
- Appearance: themes, customizable sidebar/top bar.

**Planned / not built (label as roadmap, never as features):** backup scheduler + restore UI, `.dklpkg` bundles
(tar.gz with manifest + files, optionally image and DB dump), adopt/cleanup GUI ("Site folders"), activity log
viewer, MongoDB/other database types, template catalog (media-server style capsules), TUI revamp, mail hosting
via providers, packaged installer (a 2.0 idea).

## 4. State of the repo (what's wrong with it today)

The repo root is cluttered with leftovers from earlier phases. Candidates to archive/remove **after asking
Stella**: `COMPLETE_STACK.md`, `WIRING_COMPLETE.md`, `WORKLOG.md`, `GEMINI.md`, `AGENTS.md` (check it's still
useful), `SSL-TESTING-PATCH-README.md`, `docklite-ssl-testing.patch`, `check-db.js`, `dkl-migrate.py`,
`test-phase1-validation.sh`, `start-*.sh`/`stop-all.sh` (superseded by the `docklite` launcher; check first),
`CONNECTNEWSITE.md`, `TODO.md` (some items stale). The README (`README.md`, ~330 lines) is an old
"Complete Distribution" document: wrong clone URL and quick start, mentions systemd/`./start-fullstack.sh`
flows that predate the PM2 installer, and says nothing about the CLI, backups, or the safety story.
Also: `VERSION` says 1.1.0 while the latest git tag is v1.0.3; remotes are `origin`
(`sgauth0/docklite-new`) and `girlypop` (`girlypopicon/docklite`, the canonical public repo). Confirm with
Stella which URL the README should use. There is no LICENSE file, `.github/` folder, issue templates, or CI badge;
suggest them, but ask before choosing a license.

## 5. Suggested plan

1. Create a branch (e.g. `docs/github-revamp`). Run `git status`; don't disturb other branches.
2. Read `CLAUDE.md`, `INSTALL.md`, `install.sh`, `docklite` (launcher), and the CLI guide. Verify the install
   flow is `sudo bash install.sh`, then the printed dashboard address.
3. Rewrite `README.md`: one-line pitch, a hero screenshot, "why" (own your hosting, safe on existing servers),
   feature list (only section 3 "works today"), 3-step install, first-site walkthrough, CLI taste (a few
   commands), architecture diagram, safety model (unprivileged user + validated root helper), roadmap, license.
   Keep it scannable; no emoji walls.
4. Consolidate docs under `docs/` (INSTALL, CLI, SSL, architecture). Leave stubs or redirects where files move.
5. Add repo hygiene: LICENSE (ask), `.github/ISSUE_TEMPLATE`, a short `CONTRIBUTING.md`, topics/description text
   for Stella to paste into GitHub settings (you can't change them).
6. Screenshots (below), referenced from the README via `docs/img/`.
7. Hand back a summary and the exact git commands for Stella to review and push.

## 6. Screenshots (careful)

The live dev server is `https://xxl.docklite.net` (the "XXL" box). It runs the newest build but **contains real
site names, domains, usernames and IPs**. You must not publish those.
- Preferred: ask Stella to log in in the browser you control (you cannot and must not type their password), or
  better, spin up a **demo instance** with fake data (domains like `example.com`, `demo.example.net`, users
  `alice`/`bob`) and screenshot that. A clean demo beats redaction.
- If you must use the real server: blur/black-box every real domain, IP, username, email and token before
  saving; look at the *whole* image (sidebar, headers, URLs in the address bar, terminal text, log lines).
  Crop out the browser URL bar. Re-open each final image and inspect it before committing.
- Shots worth taking: dashboard/containers (sites + the DB tiles), a container detail page, SSL/HTTPS card,
  database inspector, backups page, appearance themes (default + unicorn), the CLI `docklite doctor` output in a
  terminal, the install wizard. Save as PNG, ~1600px wide, in `docs/img/`.
- Existing images in `webapp/public/screenshots/` (`docklite1-5.png`) are old; inspect them for private data
  before reusing, and replace if unsure.

## 7. Style

Plain, warm, specific. Say what it does in the first sentence. Show commands in fenced `bash` blocks, one per
block. Don't promise what's on the roadmap. Prefer concrete examples over adjectives. Keep claims verifiable.
