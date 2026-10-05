# Changelog

Versions follow `MAJOR.MINOR.PATCH`. New work is listed under **Unreleased**; `scripts/release.sh` turns it into a
numbered release (and keeps `VERSION`, `webapp/package.json` and the git tag in step).

## Unreleased

### Added
- The `docklite` command line: containers, sites, SSL, nginx, DNS, users, backups, server health, `doctor`,
  `--json` everywhere and a built-in guide (`docklite docs`).
- Shell access for server users (`docklite access grant`) and an audit log for sensitive actions; container
  start/stop/restart/delete/assign/transfer are now recorded.
- `docklite sites layout` and `docklite repair`: check and fix the `/var/www/sites/<user>/<domain>` layout.
- Adopt unregistered site folders and move orphan folders to a trash folder (API).
- Cloudflare setup that explains itself, domain import, and SSL mode controls.
- Settings pop-up, optional sidebars and a customizable top bar; container filter tabs; running-first sorting.
- `inventory.sh`: a read-only report of an existing server.
- Backups run in the background with progress and verification.
- `docklite upgrade`: re-running `install.sh` keeps your configuration.

### Fixed
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
