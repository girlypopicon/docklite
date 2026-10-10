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

## 1.3.0: take over an existing server, and backups that work
- **Adopt** existing containers and site folders (no `.dkl` needed), without touching a running site; relabel old containers to their real owner.
- A **Site folders** tab (strays, orphans, move to trash) and the cleanup tools.
- **Backups you can trust:** back up several sites and databases at once, a real scheduler (daily/weekly, keep the last N, per site or all),
  portable `.dklpkg` packages (recipe → files → image → databases), and a working **restore**.
- Everything the safe-install work promised on SMOLL: nothing moved or re-owned without an explicit step.

## 1.4.0: simpler and friendlier
- Declutter the dashboard; rewrite the Network page in plain language (visitors → front door → site).
- An activity log viewer with filters (what to show, what to record), login/logout and idle-session handling.
- A branded "back soon" page for sites that are down, and `docklite nginx maintenance on|off`.
- A browser-only public demo (`demo.docklite.net`): fake data, nothing saved, nothing real runs.
- Terminal improvements: tabs, persistent sessions, one-click `psql`.

## 1.5.0: production-ready
- Security review and dependency/licence audit; documented backup-and-restore drill.
- Every flow tested on a clean server and on a server that already hosts sites.
- Docs complete; the TUI brought up to the same level as the dashboard.

## Later
Other server types beside websites (media servers and similar), more database types (MongoDB), shareable templates ("Capsules"),
a packaged installer.
