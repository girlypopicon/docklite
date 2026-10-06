# Changelog

Versions follow `MAJOR.MINOR.PATCH`. New work is listed under **Unreleased**; `scripts/release.sh` turns it into a
numbered release (and keeps `VERSION`, `webapp/package.json` and the git tag in step).

## Unreleased

### Added
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
