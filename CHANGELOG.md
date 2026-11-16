# Changelog

Versions follow `MAJOR.MINOR.PATCH`. New work is listed under **Unreleased**; `scripts/release.sh` turns it into a
numbered release (and keeps `VERSION`, `webapp/package.json` and the git tag in step).

## Unreleased

## 1.6.1 (2026-11-16)

### Changed
- **Group cards look like a little deck**: two more cards fan out behind the group, and the boxes inside show as rounded squares in
  their own colour and icon (green dot running, red stopped) instead of a text list.
- **Start/stop and restart are back on box cards**, in a row at the top right next to the status: one button that switches between
  Start and Stop, a Restart button while the box is running, and the `⋯` menu. All three are rounded squares now, not circles.
- **Drag a box onto another box and let go to make a group**: no more holding it there first.

### Fixed
- The `⋯` menu on a card could run off the bottom of the screen. It now opens on the side with more room and scrolls inside it
  when the window is short.

## 1.6.0 (2026-11-16)

This release is **1.6.0**: groups, and one page for every kind of box.

### Added
- **Groups replace folders.** A group is a card the same shape as a box; instead of one box's details it lists the boxes inside, and
  clicking it opens them in a window. There is no more "Default" section and no folders inside folders: a box that is in no group
  simply sits on the desktop. Your old folders are turned into groups the first time DockLite starts (nested ones become top-level
  groups, "Default" goes away, and what was in it is on the desktop). Each group has a **Linked** switch for boxes that belong
  together, like a website and its database (it only marks the group for now; what linking does comes in a later release), and a
  colour and an icon like any box.
- **Make a group by dragging.** Drag a box onto a group and it goes in. Drag a box onto another box and hold it there for a moment
  and they become a new group (you name it). On a phone, use a box's `⋯` menu → **Move to group**, which can also make a new group.
- **Databases are boxes.** PostgreSQL, MySQL, MariaDB, MongoDB and Redis databases sit on the Boxes page with everything else, and a
  SQLite database is a card too. Click one for how to connect, the table editor, a download, a new password and delete; they all
  used to be on the Databases page.
- **One "New box" window** that asks what kind of box you want: a website, or a database (PostgreSQL, MySQL, MariaDB, MongoDB, Redis
  or SQLite), each with a plain-words description.
- **MySQL** is now a database type beside PostgreSQL, MariaDB, MongoDB, Redis and SQLite: its own box (MySQL 8.4), listening on this
  server only, with a checked dump for backups and `docklite databases create <name> --type mysql`.

### Changed
- **The Databases page is gone** (the address still works and takes you to Boxes). "View DockLite database", the look inside
  DockLite's own data, is now in Settings → System.
- The Containers page is now called **Boxes**, and "New container" and "New folder" are **New box** and **New group**.
- Escape closes the new windows.

## 1.5.0 (2026-11-15)

This release is **1.5.0**: two-factor login, a page for stopped boxes, and the box cards redone as playing cards.

### Added
- **A DockLite page for stopped boxes.** When a site's box is stopped, visitors used to get nginx's plain "502 Bad Gateway". They
  now see a page in DockLite's colours: "This website's box isn't running right now", with a line saying who to contact. It is sent
  as a 503 ("back soon") so search engines don't treat the site as gone, it checks again every minute, and it shows nothing about
  your server. A box that is running but answers with an error of its own keeps its own error page.
- **Set the contact line in Settings → System → Page for stopped boxes** (a name, an email address or a link), with a preview. One
  page is shared by every site, so changing it changes all of them.
- **Existing sites get it too.** DockLite adds one `include` line to each site's nginx file, in the server block that names the
  site (HTTPS ones as well), when it starts and once a minute after that. Each file is backed up first, nginx's config is tested,
  and the file is put back if nginx refuses it. New sites are created with the line already in place. Anything that isn't one of
  your sites (the DockLite dashboard itself, hand-written proxies, a block that already has its own 502 page) is left alone.
- **Pick a colour and an icon for any box** from its `⋯` menu: 12 colours (or Auto, which colours by type as before) and 15
  icons (the box is the default). The colour is the card's glow and the icon is drawn huge behind it. It shows at once and is kept
  on the server, so everyone who can see the box sees the same card.
- **Two-factor login.** In Settings → Security you can turn on a six-digit code from an authenticator app (Google Authenticator,
  1Password, Aegis, Authy…): scan the QR code, type one code to confirm, and from then on signing in asks for the code after the
  password. Nothing is sent to any outside service; your phone and the server work the codes out separately. Each code works
  once. Ten one-time **recovery codes** are shown when you turn it on (for a lost phone, saved or copied from the same screen);
  you can make new ones, or turn two-factor off, with your password and a code.
- **Lost phone and no recovery codes?** An admin can use **Reset two-factor login** in Settings → Users (or
  `docklite users 2fa-reset <name> --yes` on the server) and the person signs in with their password and sets it up again.
  `docklite login` asks for the code too (`--code` to give it up front).

### Changed
- **Box cards are now playing cards.** Portrait (5 by 7), with a big muted icon behind them, one line for the name and a status dot
  with the uptime and port. A name takes the biggest size that fits on one line; a very long one is cut with "…" and the full
  name shows when you hover. Two boxes fit side by side on a phone. The loading placeholders are the same shape, so nothing jumps
  when the page fills in.
- **Click a box to open its details.** The buttons on the card are gone; one `⋯` menu holds everything: Start / Restart / Stop,
  View details, Open in terminal, Colour, Icon, Copy name, Assign, Move to folder, Track and Delete.

### Fixed
- The Security page said a new password needs 6 characters; it needs 10, and now says so.
- `docklite sites folders adopt` printed the wrong names in its confirmation message.

## 1.4.3 (2026-11-15)

### Added
- **Site folders panel (Settings → System) and `docklite sites folders`**: what is really under `/var/www/sites`, compared with
  what DockLite knows. Folders left behind by deleted sites, transfers or hand copies can be adopted as sites (owner and type
  chosen on the spot) or moved to the trash; the trash is listed with where each entry came from and when it goes, and can be
  emptied, one entry or all. The old "User Folder Check" button lives in the same panel.
- **The trash empties itself.** Folders moved to `/var/www/sites/.trash` (by a transfer, a layout move, a user deletion or the
  panel) are deleted for good 30 days later (`DOCKLITE_TRASH_KEEP_DAYS` changes that; `0` keeps them until emptied by hand).
  Before, nothing ever emptied it, so "cleaned up" folders only moved out of sight.
- **Backups are now one `.litpkg` file, whatever you back up**: one site, a site plus its databases, one database, several sites, all
  sites, all databases, or everything. A `.litpkg` is a plain `.tar.gz` (site folders under `sites/<domain>/files/`, each database's
  dump under `databases/`, and a `manifest.json` saying what is inside), so it unpacks with `tar` and needs no DockLite to read.
  Site exports also download as `.litpkg` now (the file was called `.dklpkg` before); the old name and the long spelling
  `.litepkg` still restore and import.
- **The Backups page starts with the schedule.** Most servers need one schedule that backs up everything, so that is the
  first screen: with no schedule yet, the form is right there with Everything already chosen. **Run backup now** (top of the
  page) offers Everything, All sites, All databases, or Choose… to tick sites and databases one by one. Past backups are under
  **Restore points**.
- **The backup picker**: a list of your sites with a **Site** and a **Databases** checkbox on each row, and the databases that
  don't belong to a site. Schedules and Run backup now use the same picker; "Everything" on a schedule includes sites and
  databases added later.
- **Databases can be linked to a site** (from the picker), so "this site and its databases" knows what to include. SQLite
  databases are linked to their site automatically.
- **Restore one site out of any package**, from the Restore tab or `docklite backups restore <file> --from <domain>`. Older
  `.tar.gz` backups still restore as before.
- **From the terminal:** `docklite backups create` and `backups schedule` take `--with-databases`, `--database <name>`,
  `--all-sites`, `--all-databases` and `--all`, and several domains at once.

### Changed
- **Moving a site to another user really moves it.** The old folder is moved to the trash even when it belongs to root (an
  older install or a hand-made site): the root helper moves it when the agent can't. Deleting a user now moves each of their
  sites' folders to the new owner (`/var/www/sites/<new owner>/<domain>`, container switched over, old folder trashed) instead
  of leaving the files under the deleted user's name, and removes the deleted user's folder. `docklite sites layout --apply`
  also trashes the old folders instead of keeping them. The moved folder is made fully writable for the new owner (owned by
  the docklite user, group-writable, setgid folders, default ACL): before, the copy kept the old folder's modes, so files an
  older site or its container had written as read-only for the group stayed that way and the new owner could not save them.
- Existing schedules keep working and now make packages: an "all sites" schedule makes one package per run instead of one file
  per site, and "keep the newest N" counts packages. `docklite backups schedule --all` now means all sites and all databases.
- A site or database that can't be backed up no longer fails the whole backup: it is left out, and the backup says what is missing.

### Fixed
- **Transferring a site to another user no longer drops its HTTPS.** certbot writes the HTTPS block into the site's own nginx
  file; the transfer re-pointed nginx by rewriting that file from the plain template whenever the port had not changed (which
  it never does any more), so the site fell back to HTTP only. The file is now only ever re-pointed, never rewritten, when it
  carries a certificate; the same guard covers every other path that writes a site's nginx file. To bring a site back that
  already lost its HTTPS this way: Containers → the site → HTTPS → enable again (or `docklite ssl issue <domain>`); the
  certificate is still on the server and certbot just puts it back.
- The dashboard's "Assign to user" now says when the old folder could not be tidied away, instead of reporting plain success.
- Two backups of the same thing made in the same second no longer overwrite each other.
- A backup that was running when DockLite stopped is now marked failed instead of showing "in progress" forever.
- Editing a schedule showed the wrong destination.

## 1.4.2 (2026-11-11)

### Changed
- **Container cards are smaller and named the way you know them.** A site's card is titled with its domain (`example.com`) instead of
  Docker's name (`docklite-site-example-com`), and a database's with its database name. Each card is about a third of its old height, so
  far more fit on a screen. The Docker name is still there: hover the title, open Details, or use "Copy name".
- **Database cards are smaller too**, with the type, size, address and date on two short lines.

### Fixed
- **The dashboard fits a phone screen.** The top bar was wider than a phone, so every page loaded zoomed out, and the page links were
  hidden with no other way to reach them. On phones the bar is compact (Log out is in the account menu) and the pages get a row of their
  own. Pages use the full width of the screen instead of wide margins, header buttons no longer cut off their labels, and the
  Files / Panel side tabs sit low on the screen edge as small icons instead of covering the page's filters.
- **The File Browser works on phones.** It used to open a fifth of the screen wide, with its buttons pushed out of sight. On a phone it now
  takes the screen (tap outside it, or the arrow, to close), its buttons wrap onto a second line, and each file has a "⋯" button for
  Move, Copy and Delete, which were right-click only before.
- **Long folders in the File Browser scroll to the end.** The panel used to run under a phone browser's toolbar, hiding the last files, and
  its layout relied on sizing some browsers handle differently. The list now scrolls on its own, below a header that stays in view.
- **Settings on a phone:** the Users list shows its actions button without scrolling sideways (and the button is big enough to tap), the
  Docker Connection and Top bar rows no longer squash their text, and the Update card's links are easier to hit.
- **A cleaner update log.** Updating no longer prints a stray "info: No menu item" error, PM2's process table, or the whole
  site-folder report into the update log (one line now points to `docklite sites layout`).
- **A container's port is listed once**, not twice (Docker reports it for IPv4 and IPv6).

## 1.4.1 (2026-10-26)

### Fixed
- **Servers on 1.2.6 or 1.3.0 can update with the Update button again.** Those versions' updater sends the installer's output through a
  log filter that fails on common locales ("Invalid collation character"), so the installer stopped at once and the update rolled back
  or left DockLite stopped. The installer now notices it was started by one of those updaters and writes straight to the update log.
- **The Update card no longer spins forever after an update from the command line.** If an update job was stopped, or DockLite was
  reinstalled by hand, the card believed the old "running" note for 45 minutes. It now checks whether an update is really running, and
  shows the update as finished (or as not finished) as soon as it isn't.
- **Your theme and other Appearance settings stay put.** They used to live only in the browser, so they could be lost (after an update,
  cleared site data, another browser or computer). They are now saved to your DockLite account and follow you everywhere. Pick your look
  once more after updating to 1.4.1 and it stays from then on.

## 1.4.0 (2026-10-17)

### Added
- **More kinds of database: MariaDB, MongoDB and Redis**, alongside PostgreSQL and SQLite. The Create Database form has a picker that says in
  plain words what each is for (MongoDB and Redis are the NoSQL ones). Each gets its own container, a password, and a persistent data
  volume. The new three listen on this server only (never the open internet) and your sites reach them by name over DockLite's private
  network; the form shows ready-to-paste connection URLs for both. Cards show a type badge and size.
- **Backups for the new types.** MariaDB, MongoDB and Redis dumps use each engine's own tool, and every dump is read back and checked
  before it counts as a backup (a cut-off dump is refused). Restoring databases is the next release.
- **From the terminal:** `docklite databases types | create <name> --type mariadb|mongodb|redis|postgres | delete <name> --yes`, and
  `databases list` now shows each database's type and size.

### Changed
- **The page a brand-new site shows is now DockLite-themed** (dark, rainbow-edged, readable on a phone) instead of plain white, for static, PHP
  and Node sites alike. It also stops printing the server's folder path to the public, which the old page did. It loads nothing from other
  websites, asks search engines not to index it, and only appears on sites that have no page yet; existing sites are untouched.

### Fixed
- **A database whose container had been removed couldn't be deleted** (error 500, and the entry stayed in the list forever). It now clears
  the entry and any leftover data.
- Creating a MariaDB or MongoDB database with the same name as one whose data was left behind used to silently reuse the old data and
  ignore the new password. It now says so and refuses.

## 1.3.4 (2026-10-15)

### Fixed
- **Dialogs in the Unicorn theme were see-through**, so the blurred page showed through their text (New Container, Restore, the
  certificate dialog). They are now a solid surface, and the New Container dialog's headings and labels use the theme's readable text
  colours in every theme.
- **Log windows are readable in every theme.** They are solid terminal-style panes instead of see-through glass (in Unicorn the page
  behind showed through the text). The glass look stays on panels; it just isn't used where you have to read.
- **A proper Close button.** Dialogs had a tiny, plain ×; they now share one round, clearly visible button in the theme's own button colours.
- **Container logs no longer start every line with garbage characters** on containers that have no terminal attached (most of them).
  Docker wraps those logs in a binary frame that DockLite printed as text; it is now decoded.

## 1.3.3 (2026-10-15)

### Fixed
- The public demo's **File Browser** (the FILES tab) was empty and showed errors. It now has a pretend disk: browse each user's and
  site's folders, open and edit files, create, upload, move, copy, download and delete, all inside the visitor's tab.

## 1.3.2 (2026-10-13)

### Added
- **A public demo of the dashboard** (for demo.docklite.net) that runs entirely in the visitor's browser on made-up data: every page
  works, including restoring a backup, schedules, the terminal and DNS, and nothing real can be touched because nothing real is there.
  Built as plain static files by `scripts/build-public-demo.sh` (it refuses to build if the output mentions anything private) and
  attached to each release as `docklite-demo.tar.gz`. See `docs/PUBLIC_DEMO.md`.

## 1.3.1 (2026-10-12)

### Fixed
- **The Update button stopped right after "Downloading and installing" on some servers** ("Invalid collation character"). The step that
  tidies the update log used a character range some locales reject. It now works on raw bytes in every locale, and if it can't run
  for any reason the log is shown untidied instead of stopping the update.

## 1.3.0 (2026-10-12)

### Added
- **Restore a site from a backup.** The Restore tab lists your site backups; pick one, see what's in it, and put the site back. Before
  anything changes DockLite saves what the site has now (so a restore can itself be undone), checks the backup is intact and hasn't been
  altered, refuses anything that could write outside the site's folder, and swaps the files in place so the running site keeps its
  folder. Each backup row's Restore button opens the same step. Restoring databases comes in a later version.
- **Schedules with a time of day and "keep the newest N".** "Every day at 03:00, keep the last 7" instead of "every 24 hours, keep
  for 30 days". If the server was off at the chosen time the backup runs when it's back. Old schedules keep working. The form now
  defaults to All Sites.
- **Backups from the terminal:** `docklite backups files | create | inspect | restore | schedule | schedules | unschedule`.

### Fixed
- One site failing to back up no longer stops the others, and no longer makes the schedule retry every few minutes: the failure is
  recorded on that backup and the schedule moves on.

## 1.2.6 (2026-10-11)

### Fixed
- **The Update button's installer stopped its own update job.** The installer shuts down "older DockLite services" so they don't fight
  the new one, and it counted the update's own temporary service as one, so the update killed itself right after starting. The
  update's service is now left alone.
- **A killed update no longer blocks the next one for 45 minutes.** DockLite now checks that an update is really still running
  instead of trusting the last note it wrote.

### Changed
- **Release notes in the Update card look like notes.** Headings, bullets and bold text are shown properly instead of as raw `###` and `**` symbols.
- **The update log is tidy no matter which version runs the update.** Colour codes, spinner frames and repeated lines are removed both
  when the log is written and when the dashboard shows it, so each step reads as one line.

## 1.2.5 (2026-10-11)

### Fixed
- **The dashboard's Update button failed on the build step and left the panel stopped (502 Bad Gateway).** The update runs as a
  background job that has no home folder set, and the installer stopped when it needed one. The installer now copes, and so does
  the update job. If an update ever fails, the old version is brought back as before.

### Changed
- **The update log is readable.** It used to be full of colour codes and spinner frames; it now shows one clean line per step.

## 1.2.4 (2026-10-11)

### Added
- **New sites get their HTTPS certificate automatically.** When you create a site, DockLite waits until the domain's DNS points at
  the server (or at Cloudflare), then requests the certificate itself and retries politely if something isn't ready yet. No more
  issuing certs by hand. Progress and a "Try now" button show in the site's details; untick the box in New Site to opt out.
  From the terminal: `docklite ssl pending`, `docklite ssl retry <domain>|--all`, `docklite ssl email`.

## 1.2.3 (2026-10-10)

### Added
- `scripts/contrast-audit.js`: measures text contrast in every theme on a running demo, so theme changes are checked by numbers.

### Changed
- **Themes are readable.** Text now has its own colours per theme instead of reusing the pale neon colours meant for borders and glows.
  In Corpo, Corpo Blue and Unicorn about 40% of the text on the main pages was below the readable standard (some almost invisible,
  e.g. pale green on cream); it is now zero on the pages checked. The Corpo themes use deeper fills with white button text.

### Fixed
- **Site folders weren't writable by the docklite group**, so anyone logged in to the server (or an assistant working in their shell)
  had to run `sudo chmod -R g+w` on each new site before they could edit it. DockLite asked for group-writable folders, but the
  server's default umask silently removed the permission. Folders and files DockLite creates are now explicitly group-writable
  (folders also keep the docklite group for new files); existing sites are fixed once automatically, and
  `docklite sites permissions [domain]` does it on demand. It only adds group access: owners and other users are untouched,
  nothing becomes world-writable, and links are never followed. The installer also adds the standard `acl` package and a default
  ACL, so files created later in a site folder by a person or assistant (whatever their shell's umask) stay editable by the
  dashboard's file editor too.
- **Transferring a site to another user is safer.** It copies the folder first, switches the container over only once the new one is
  running, puts the old container back if anything fails, and moves the old folder to the trash instead of deleting it. It never
  copies from or deletes outside `/var/www/sites`, and refuses to overwrite a target folder that already has files.

## 1.2.2 (2026-10-10)

### Fixed
- **The installer quit silently on servers that have websites but no default nginx site**, so upgrades never ran (the version stayed
  the same). It now finishes, and if it ever stops unexpectedly it says at which line instead of just disappearing.

## 1.2.1 (2026-10-10)

### Added
- `docklite update`: check for a newer DockLite (`--check`) and install it (`--yes`) from the command line. It follows the update
  through the restart and exits non-zero if it failed or rolled back. Needs no sudo password, so a person or an assistant logged in
  to the server can run it.
- A written roadmap (`docs/ROADMAP.md`): what 1.2.x, 1.3, 1.4 and 1.5 are for.

### Fixed
- A failed update now puts your **data** back as well as the old program, so the old version never meets a database the new one
  already changed. (The pre-update backup is restored while DockLite is stopped.)

## 1.2.0 (2026-10-08)

### Added
- A working Update button (Settings → System). It checks GitHub for the newest release, shows what's new, and updates with one
  click: it backs up your data first, installs exactly that release, checks DockLite came back, and goes back to the previous
  version by itself if it didn't. Your sites keep running throughout. `install.sh --yes` runs the installer without questions.
- Settings → Cloudflare: connect your Cloudflare account, import your domains and set each domain's SSL mode from Settings
  (it was only under Network → DNS). The New Site form shows exactly which DNS records will be created, and a site's details
  show whether its DNS exists in Cloudflare, with buttons to create it and to change the zone's SSL mode.
- Demo mode includes a pretend Cloudflare (example.* zones), so these screens work without a real account.
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
- **Sites showed 502 after a restart or reboot.** Site containers were given a random port that changed every time they
  started, while nginx had the old number written in. DockLite now keeps nginx in step by itself: when the agent starts,
  whenever Docker starts a managed container, and once a minute it finds each site's real port and fixes the matching
  `proxy_pass` (in `sites-available`, `conf.d` and multi-site files, including aliases of a site), reloading nginx only if
  `nginx -t` accepts it. New sites get a fixed port from 20000-29999 (`DOCKLITE_PORT_RANGE`), so their port never changes.
  Per-server workarounds like a port-sync script and timer are no longer needed.
- The old Update button could never work (it looked for git history that installs don't have, and for a script that didn't
  exist). It's been replaced.
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
