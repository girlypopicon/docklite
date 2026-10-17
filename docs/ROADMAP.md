# Roadmap

Versions are `MAJOR.MINOR.PATCH`. **Patch** releases (1.2.1, 1.2.2…) are fixes and small polish and ship whenever they're ready;
servers pick them up with the dashboard's Update button. **Minor** releases (1.3.0, 1.4.0…) ship when their goal below is done,
not before. **1.5.0** is the "ready to run real websites for real people" release.

## 1.2.x: steady and safe (now)
Fixes found by running DockLite on a live server, plus proving the update path.
- Prove the Update button on a live server (backup, install, health check, automatic rollback of code *and* data).
- Contrast fixes for the Unicorn and Corpo Blue themes.
- Safer site transfer (never delete outside `/var/www/sites`, keep the old folder until the new container is confirmed).
- Anything else that breaks on a real server.

## 1.3.x: backups you can restore, and a public demo (shipped)
- Restore a site from a backup (safety copy first, checked before and after), schedules with a time of day and "keep the newest N",
  `docklite backups ...`, and the browser-only public demo at `demo.docklite.net`.

## 1.4.x: more databases, and backups that cover them
- **1.4.0: MariaDB, MongoDB and Redis** beside PostgreSQL and SQLite (each in its own container, loopback-only, with connection strings
  for your sites), and a dump of each that is checked before it counts as a backup.
- **1.4.1: restore databases** (all engines, with a safety dump first) and **one-domain backups**: a site AND its databases together,
  everything at once, or just one domain's database. Needs a link between a site and its databases. Replaces the checkbox grid with
  three plain choices.
- **1.4.2: off-site backups** to any S3-compatible storage (Amazon S3, Backblaze B2, Cloudflare R2, Wasabi, MinIO).
- **1.4.3: take over an existing server.** Adopt existing containers and site folders (no `.dkl` needed) without touching a running
  site, plus a Site folders tab (strays, orphans, move to trash).

## 1.5.0: production-ready
- Simpler and friendlier: declutter the dashboard; the Network page in plain language (visitors → front door → site); an activity log
  viewer; a branded "back soon" page (`docklite nginx maintenance on|off`); terminal tabs and persistent sessions.
- Security review and dependency/licence audit; documented backup-and-restore drill.
- Every flow tested on a clean server and on a server that already hosts sites.
- Docs complete; the TUI brought up to the same level as the dashboard.

## Later
Other server types beside websites (media servers and similar), more database types (MySQL proper, others on request), shareable templates ("Capsules"),
a packaged installer.
