# Changelog

Versions follow `MAJOR.MINOR.PATCH`. New work is listed under **Unreleased**; `scripts/release.sh` turns it into a
numbered release (and keeps `VERSION`, `webapp/package.json` and the git tag in step).

## Unreleased

### Added
- Cloudflare DNS for new sites: adding a website now creates its DNS records in Cloudflare (an A record for the site, and `www` if
  requested), proxied by default. It only creates what is missing and never overwrites an existing record unless asked.
  New endpoint `/api/dns/site` previews or applies it. Admins only.
- One-line install: `curl -fsSL .../get.sh | bash` downloads DockLite and starts the installer (add `-s -- --dry-run`
  to only look). Running it again upgrades in place.
- Demo mode (`scripts/demo.sh up`): a separate DockLite instance with fake users, example.* sites and databases for
  screenshots, testing and demos. It simulates nginx/certificate actions, hides real containers and host details, and
  real DockLites never list its containers.
- `sudo bash install.sh --dry-run`: reports what is already on the server (nginx sites, older DockLite, containers,
  site folders) and what the installer will and won't touch, changing nothing. An older install in `/opt/docklite`
  is backed up to `/var/backups/docklite/` before files are replaced.
- Licensed under the GNU AGPL v3 (`LICENSE`).
- The `docklite` command line: containers, sites, SSL, nginx, DNS, users, backups, server health, `doctor`,
  `--json` everywhere and a built-in guide (`docklite docs`).
- Shell access for server users (`docklite access grant`) and an audit log for sensitive actions; container
  start/stop/restart/delete/assign/transfer are now recorded.
- `docklite sites layout` and `docklite repair`: check and fix the `/var/www/sites/<user>/<domain>` layout.
- A site's previous owners are recorded in its `.dkl` manifest (transfers and user deletions), and kept when it is rewritten, backed up or exported.
- Adopt unregistered site folders and move orphan folders to a trash folder (API).
- Cloudflare setup that explains itself, domain import, and SSL mode controls.
- Settings pop-up, optional sidebars and a customizable top bar; container filter tabs; running-first sorting.
- `inventory.sh`: a read-only report of an existing server.
- Backups run in the background with progress and verification.
- `docklite upgrade`: re-running `install.sh` keeps your configuration.

### Fixed
- The neon glow slider (Settings → Appearance) did nothing for container and database cards, and containers had lost their glow
  entirely (their shadows used an invalid color notation). Cards now glow by default; the slider adds a bigger halo, a thicker
  tube and, near Max, a white-hot core like real neon. Off is the standard look (it was 100% before; the default is now Off).
- Settings → Appearance preview now shows a narrow site card next to a wide database card, like the real pages.
- On a server that already has a default nginx site, the dashboard's nginx entry referred to a variable that was only defined in
  a file DockLite deliberately does not write there, so nginx rejected it (and could have refused to start at its next
  restart). The entry is now self-contained, and any nginx config nginx rejects is removed again instead of left behind.
- `docklite status` always said the web GUI was "Not started" (and install printed "GUI may still be starting"), even when it
  was running: the process check used a pattern that can never match. It now detects the GUI correctly.
- `docklite` no longer needs a log out and in after install: if you were just added to the docklite group, it restarts
  itself inside the group. If an interrupted install left `/opt/docklite` read-only for the group, it repairs that itself
  (new root-helper command `fix-install-perms`).
- The one-line install left `/opt/docklite` read-only for the docklite group, so the setup wizard could not write its
  settings ("Permission denied"). The installer now sets the folder group-writable itself.
- The dashboard-domain question in setup now explains how to type it (just the hostname), that DNS must point at the server
  first, that ports 80/443 must be open, and that the dashboard is then reached only at that address. A pasted URL is
  trimmed to the hostname, and it shows what DNS currently says for the name.
- Installing over an older DockLite that runs as systemd services (`docklite-agent`, `docklite-web`): the installer now asks
  to stop and disable them first, because they run from the folder being replaced. Sites keep running (nginx and Docker
  serve them). If the backup fails, the services are started again.
- The dashboard footer and Settings showed a hard-coded "v1.0"; they now show the real version.
- The 404 page listed Containers twice; the second link now goes to Backups.
- Installing on a server that already hosts sites: DockLite no longer disables the `default` and `docklite-sites` nginx
  configs, never adds a second `default_server`, and undoes its own nginx change if the config test fails.
- Container uptime now counts from the last start, not from creation.
- Containers from an older install no longer appear to belong to unrelated users after user ids are reused.
- Terminal showed "Not connected"; pop-ups hidden under the top bar; Unicorn theme readability; backups page flash.
- The installer no longer re-owns existing site folders under `/var/www/sites`.

## 1.0.3
- Panel domain with Let's Encrypt HTTPS; the system database viewer for admins (secrets redacted); CSRF fix on
  dashboard actions; nginx catch-all renamed `docklite-default.conf`; installer and reboot fixes.

## 1.0.2
- Second round of security fixes.

## 1.0.1
- First tracked release.
