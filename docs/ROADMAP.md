# Roadmap

Versions are `MAJOR.MINOR.PATCH`. **Patch** releases (1.2.1, 1.2.2…) are fixes and small polish and ship whenever they're ready;
servers pick them up with the dashboard's Update button. **Minor** releases (1.3.0, 1.4.0…) ship when their goal below is done,
not before. **1.5.0** is the "ready to run real websites for real people" release.

> **Wording (1.20):** say **container**, not **box**. Older entries below still use "box" for the same thing.

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

## Next up (decided 2026-11-17)
In this order; each is its own release.
1. **Security review of the agent**, with the owner's goal that it is as close to perfect as it can be. Start with authentication and tokens,
   then everything that reaches files, nginx or a shell, then the root helper, then tests that try to break each. A new **Security tab** in a
   box's details tells a non-expert in plain words whether anyone is attacking it (failed logins, scanners, blocked requests, odd traffic)
   instead of making them read raw logs.
2. **Box resource limiter** (CPU, memory, process count), set from the box's details.
3. **Declutter the dashboard** and put the **Network page in plain language** (visitors, front door, site).
4. **Branded "back soon" page** (`docklite nginx maintenance on|off`).
5. **Terminal tabs** with persistent sessions. **Game servers** come next and use the same terminal as their console.
6. **Move boxes between servers**: install the agent on another server over SSH, then move a box with near-zero downtime. The two servers talk
   to each other directly (no relay), sync files and data while the old box keeps running, then a short final catch-up, the DNS switch, and the
   old box stays stopped until the owner confirms. Keys and tokens for the other server are stored encrypted and never shown again. Only after
   the security review.

## 1.5.0: production-ready
- **Stacks: isolated by default, connected on purpose.** Containers (each one a "box") no longer share one network. A
  group of boxes (a "stack") gets its own network, start/stop/restart as one, and connection strings between its
  members. Folders become cards in the grid. Decided 2026-11-11: no compatibility shim; the shared `docklite_network`
  goes away in the same release, since nothing in use relies on reaching a database by name across it. A server can't be
  called production-ready while every container can reach every database. Design notes under *Ideas* below.
- Resource limits per container (CPU, memory, process count) and non-root inside the site images, set by DockLite.
- Simpler and friendlier: declutter the dashboard; the Network page in plain language (visitors → front door → site); an activity log
  viewer; a branded "back soon" page (`docklite nginx maintenance on|off`); terminal tabs and persistent sessions.
- Security review and dependency/licence audit; documented backup-and-restore drill.
- Every flow tested on a clean server and on a server that already hosts sites.
- Docs complete; the TUI brought up to the same level as the dashboard.

## Later
Other server types beside websites (media servers and similar), more database types (MySQL proper, others on request), shareable templates ("Capsules"),
a packaged installer.

## Ideas (design notes, not scheduled)

Rough thinking for later. Nothing here is promised; each one is written so it can be built without changing how a single
server works today.

### Stacks (scheduled for 1.5.0): boxes that belong together
Words: a container is a **box**; a group of boxes is a **stack**. Docker already uses "stack" for a group of containers
deployed together, so it is the established term; and "what's your stack? nginx and Postgres" is literally what's in one.
- Two kinds of group. A **folder** is what exists now (organisation only). A **stack** is a folder that also owns a Docker
  network and has start/stop/restart for the whole group. One `kind` column on `folders`, plus a `network_name`; additive.
- Folders could show as a card in the grid (open on click) instead of a full-width section. UI only; same data.
- Every DockLite container already sits on `docklite_network`, so "connect nginx to postgres" works today but is flat. A
  stack's own network is what gives isolation (one user's site can't reach another user's database), start order (database
  before app), and connection wiring (inject `DATABASE_URL` on the next recreate).
- The principle: **isolated by default, connected on purpose.** Nothing reaches anything until the boxes are in a stack;
  the stack is what owns a network, a start order and the connection strings. The security model and the user's mental
  model are the same picture.
- No migration shim. The shared `docklite_network` is removed in the same release. The update moves every existing
  container onto a per-site network of its own (a one-box stack), so a site that talked to a database by name stops
  being able to until the two are put in a box together. The release notes say so in the first line.
- A stack inside a folder: yes. A stack inside a stack: no; Docker networks don't nest.
- An nginx container inside a stack still sits behind the host nginx, so it must publish on a localhost port like any site.
- Last phase: a stack serialised to a `.dkl` is a shareable template ("Capsule"), and `docker-compose.yml` import is the
  same code path with a second input format. Needs: several services per file, any image (templates become shortcuts),
  volumes and inputs the installer asks for, and a version the agent refuses when it's too new.
- First template that needs a stack rather than one box: glowrom (the media center) on a server, playing to screens
  anywhere. The glowrom box is the brain (library, resume, auto-skip, phone remote); a small `glowrom screen` agent on the
  laptop or TV box runs mpv and plays the file over HTTP from the server, whatever cable the display uses. The stack is
  the glowrom box plus the media route on the domain. Design: `glowrom-linux/DEVNOTES.md`, 2026-11-11 entry.

- **Existing boxes that already talk (found on smoll, 2026-11-15):** `imoverit.me` reaches `imoverit-db` by name over the
  shared network, with no env var saying so. Removing the network without a shim breaks it. The update must put boxes that
  already talk into a stack together (linked in `site_databases`, or named in the site's config), or list them and stop.

### The desktop: boxes, folders and one place to make things (owner's direction, 2026-11-15)
**Groups (decided 2026-11-15).** Folders become **groups**: one flat kind of card that holds boxes, with a **Linked** checkbox in its
settings. Unticked, a group only keeps the desktop tidy. Ticked, its boxes are meant to talk to each other (a website and its
database). A box is in at most one group, groups don't nest, and there is no "Default" group: a box in none sits on the desktop.
Boxes are tracked by container name, so they stay in their group when recreated. Built in stages:
1. **Groups and the merged desktop** (1.6.0, built): the groups API and tables (`box_groups`, `/api/groups`, old folders converted
   once at start, "Default" dropped); group cards and the modal that opens them; drag a box onto another and hold to make a group;
   Containers and Databases merged into one page with database boxes among the rest; "View DockLite database" moves to Settings;
   one **New box** dialog that asks what kind (website, or a database: PostgreSQL, MySQL, MariaDB, MongoDB, Redis, SQLite).
2. **Linked groups for real** (1.7.0): a linked group gets its own private Docker network, its boxes reach each other by name, and
   connection strings are filled in for the site. Then the shared `docklite_network` goes away ("isolated by default, connected on
   purpose", see Stacks below), with boxes that already talk (a site and its linked database; `imoverit.me` and `imoverit-db` on
   smoll) put into linked groups automatically by the update, so nothing stops working.
3. More engines on request (Elasticsearch/OpenSearch, a Redis-compatible Valkey, and servers beyond websites).

*Done in the card release: the playing-card shape, the single `⋯` menu with Colour and Icon, click-to-open details, one-line names,
matching loading cards and the two-per-row phone grid. Still to do: folders as cards (no "Default" section), drag to make a
folder, the Databases tab merged away, the SSL padlock and SSL tab, one "New box" dialog, and the phone's own menu layout.*

The Containers page becomes one flat **desktop** of cards, like a phone's home screen.
- **Cards are playing-card shaped.** A box card is portrait, a little taller than the 1.4.2 card and narrower, at a
  playing card's ratio (about 5:7). A database card is the same size turned sideways, or square and shorter (try both).
- **The loading screen matches.** The placeholder "ghost" cards shown while the page loads (`PageSkeletons.tsx`) use the
  same size and layout as the real box, database and folder cards, so nothing jumps when the data arrives.
- **Folders look like boxes.** A folder is a card the same size as a box card; instead of a box's details it lists the
  boxes inside it. Click it to open a modal with those boxes. No nested sections, no full-width folder rows, and no
  "Default" folder: anything not in a folder sits on the desktop itself.
- **Make a folder the Android way:** drag one box onto another and hold for a second or two; the two become a folder.
  Dropping on a folder card adds to it; dragging out of the modal takes a box back to the desktop. Review the current
  drag and drop (`@dnd-kit`, `FolderSection.tsx`) before building this: it was written for sections, not a grid of cards.
- **Click a box to open its details.** The `⋯` menu is the only button on a card. It holds Start / Restart / Stop, View
  details, and two submenus (`›`): **Colour**, 10 to 12 shades for that card, and **Icon**, 10 to 15 icons drawn huge and
  muted (grey or the theme's quiet colour) in the card's background. The default icon is the old 1.4.1 box (`Package`).
  Folders get the same menu, colour and icon. Saved per user, like the appearance settings.
- **Names stay on one line.** A long name shrinks its font a little; an absurdly long one is cut with `…` and the full
  name shows on hover. Never two lines.
- **Say "box", not "container"**, everywhere a person reads it (page title, buttons, menus, messages).
- **SSL on the card and in the details.** A small padlock on each site's card (valid, expiring soon, expired, none),
  checked in the background. The details modal gets an SSL tab to manage that box's certificate (issue, renew, remove,
  HTTPS redirect). Most of it exists already in `SiteHttpsCard.tsx` and the `/api/ssl` endpoints.
- **One "New box" dialog** for everything: a website (static, PHP, Node, with its domain), a database (Postgres, MariaDB,
  MongoDB, Redis, SQLite), and later other server types. The Databases tab goes away; database boxes live on the desktop
  with everything else, and the DockLite system database viewer moves to Settings.
- **Phones: the desktop is the whole app.** On a phone people only manage boxes, so the boxes desktop is the first page;
  the other pages and most header buttons move into one dropdown menu; the type filters and "New box" become icon-only
  buttons; and each card shows only what a thumb needs (status, name, the `⋯` menu with Start / Restart / Stop first).
  The 1.4.2 phone layout is still crowded; this replaces it.
- Stacks (above) fit the same picture: a stack is a folder card with its own network and start/stop for the group.

### A new logo: a candle (owner's direction, 2026-11-15)
- A candle gives off light; its **flame is a heart**. Keeps the heart the brand already uses, and works at favicon size
  (a heart-shaped flame on a short stick reads at 16 px where a detailed box does not). Ideas to try beside it: a box
  (cube) whose lid opens into a heart, a heart-shaped box, sparkles around the flame for the animated version.
- Draw it as one SVG in the brand gradient (`#FF6AD5` to `#C7A4FF`), with a one-colour version for the favicon and the
  TUI splash. Replaces `dockliteiconL.png`, the favicon and the top-bar logo together.

### Unicorn theme: readable text (owner's direction, 2026-11-15)
- Keep the colours; fix the text. Much of it is pale on pastel and hard to read. Fix through the `--text-*` variables in
  `globals.css` and check with the contrast audit. The audit needs updating first: since 1.4.1 the theme is saved per
  user, and the audit only sets it in the browser, so its numbers for the light themes can't be trusted.

### Site folders: `/var/www/sites` holds only users (owner's direction, 2026-11-15)
- After a folder check, `/var/www/sites` should contain only one folder per user who has boxes, and each site folder
  inside its owner's. A folder in the wrong place that is serving a site is moved to the right place (box switched over,
  old folder removed); one that serves nothing is removed.
- "Removed" means moved to `.trash`, which empties itself after 30 days (1.4.3), so a mistake can be undone for a month.
  Deleting at once is possible but leaves no way back.

### Hosting as a business: one-click websites (owner's direction, 2026-11-15)
The idea for making DockLite pay: DockLite as a website host where making a site is one click.
- **A WYSIWYG site editor**, so a customer builds the site without touching files. Starts from the ioi profile builder idea
  (see *Webrings* below).
- **Buy the domain while making the site.** Search a name, pay, and the site, DNS and certificate are set up in one go.
  The plan is to do this through Cloudflare. Check first: the note under *Webrings* says Cloudflare's registrar API isn't
  open for new registrations to most accounts, and lists reseller alternatives (Porkbun, Namecheap, OpenSRS, IONOS).
- **Link an imoverit account.** A customer with a DockLite domain can have `imoverit.me` point at their own domain
  instead of their `ioi/username` page, so it is one address for both.
- Payments make DockLite a business (merchant account, refunds, support, terms). Needs the multi-tenant hardening under
  *Public hosts* before strangers' sites run on the same server.

### Stopped boxes: later ideas
- **Retired on purpose.** A per-box switch for a site its owner has stopped deliberately: a different message ("this site has
  closed"), or none (nginx drops the connection). Today every stopped box shows the same "isn't running" page.
- **Per-site contact.** The contact line is server-wide. A host with several customers may want each site's own owner shown.

### Apache as a choice under the static and PHP templates
- A "web server" dropdown (nginx or Apache) on the static and PHP templates, not extra templates. Swaps the image inside the
  container (`httpd:alpine`, `php:apache`); the host nginx proxies to it exactly as before.
- Turn on `AllowOverride All` for the site root, otherwise `.htaccess` (the reason people want Apache) does nothing.

### Databases page revamp
- Engines are one row each in `docker/engines.go`; MySQL proper is nearly free. SQLite is a file, not a server, so it needs
  its own path (lives in the site folder, schema browser points at the file).
- Group the page by family rather than by product: relational, document, key-value.

### More than one server (fleet)
Install DockLite on a second server, sign in with the same account, see the containers from both, and choose which server
each one runs on. Pieces that already exist and would carry most of it:
- **`.litpkg` is the transport.** Moving a site is export on one server, import on the other; a package already holds the
  files, the databases and the `.dkl`.
- **Cloudflare DNS is the switch.** The DNS integration can already write a site's A record; moving a site ends with
  pointing the record at the other server's IP. Later, Cloudflare Tunnels would let a second server with no public IP join.
- **The agent API is the remote control.** The TUI already drives an agent over the bearer-token API; the dashboard could
  talk to several agents the same way, with a `servers` table and a server id on each container.

Design notes:
- **The hub is a list of addresses, and that list is a `.dklnet` file.** Fleet nodes are entered on the main dashboard by
  IP or hostname, or by opening a `.dklnet` file: plain JSON, one entry per node (address, name, public key, maybe a
  label like "eu-2"). The hub keeps it as its `servers` table, and can export it again. No discovery protocol, no cluster
  membership dance. Not needed for two servers; for a big site spread over many, the fleet definition is a file you can
  keep in a repo, hand to a colleague, or open on a fresh hub. Same family as `.dkl` (one site) and `.litpkg` (one package).
- **Nodes don't need the GUI.** A fleet node is the agent on its own (`NEXTJS_URL=disabled` already does this), driven by
  the hub's dashboard, the TUI or the CLI. The GUI runs once, on the hub.
- **Superadmin is the keystone.** Every node trusts the hub's superadmin; that account ties the fleet together and is for
  big emergencies only. Admin is for ordinary emergencies. Day to day, people use their own account, because it does
  everything they need. Nobody signs in as superadmin or admin to get work done.
- **Pairing with a generated key, not a shared password.** At join time DockLite generates a keypair; the dashboard shows
  a one-time join code, the second server pastes it, and the two exchange public keys. SSH is a good wire for it: `rsync`
  over SSH moves large site folders without going through the API, and the `docklite` user owning `/var/www/sites` on both
  ends means no root is involved. Agent-to-agent API calls can sign with the same key.
- **One primary for accounts.** The first server owns users and sessions; a secondary trusts the primary's sessions rather
  than keeping its own user table in step. Avoids two sources of truth for passwords and roles.
- **Independent servers with a shared dashboard, not an orchestrator.** No Swarm, no Kubernetes, no shared state between
  the Docker daemons. Each server keeps working on its own if the others go away; the fleet view is a convenience on top.
- **Single-server mode stays exactly as it is.** A server that never joins a fleet runs the same code with an empty
  `servers` table.

### Mail
- Two different jobs. **Sending** (contact forms, password resets) is easy and is what most sites need: an SMTP relay
  setting per site or per server, pointed at any provider, and `MAIL_*` env injected into containers. **Receiving**
  (mailboxes, IMAP) is the hardest thing to self-host: port 25 is blocked on many VPSs, reverse DNS must match, and IP
  reputation takes months.
- A mail "engine" for the full thing: one container like `docker-mailserver` (Postfix, Dovecot, Rspamd) in the engine
  table, same as a database. DockLite's value is the wiring people get wrong: MX, SPF, DKIM, DMARC and autoconfig records
  written through the Cloudflare integration, and the certificate shared with the host nginx.
- Start with sending. Offer receiving only with a pre-flight that checks port 25, reverse DNS and that the domain's zone is
  in DockLite, and says plainly what is missing.

### Proxy management
- Keep nginx. Swapping it for Traefik or Caddy would throw away the vhost generation, the SSL stack and the upstream sync.
  The gap is the editing experience, not the proxy.
- Routes, not config files. A per-site view of what the host nginx does for that domain: upstream (container and port),
  www handling, redirects, path-based routes to other containers (a stack's services), custom headers, basic auth, rate
  limits, the maintenance page. Each is a form field that regenerates the vhost; the raw config stays available under
  "advanced" for people who want it.
- Stacks plug in here: a stack exposes one or more services as routes under a domain.

### Terminal UI
- Done: the TUI source now lives in this repo (`cli-repo/`), the installer builds it, and it has its own Update screen.
  Still to do: browsing tables in a database (needs the database login, which the TUI has), and the steps below.
- Parity with the dashboard, screen by screen: sites (create, SSL, DNS), backups (picker, restore), users, server health,
  logs that stream. Lists must scroll inside the panel (fixed in the containers, databases and files screens), and every
  screen must survive a narrow terminal.
- The TUI is first-class, not a fallback: on a fleet node without the GUI it is the interface.

### Neon controls
- Today there is one glow slider (0 to 200 percent) plus theme, text size and animations, in `webapp/lib/appearance.ts`.
- Keep adding granularity until it feels right: separate intensity for borders, text and fills; per-colour toggles (pink,
  cyan, purple, green, yellow); glow radius vs brightness; a "calm" preset and a "maximum" preset; the same knobs in the
  TUI's colour scheme. Every step must keep the contrast audit (`scripts/contrast-audit.js`) at zero failures.

### Public hosts: donated capacity for small things
Pitch: "Got extra server space? Let someone run their glowrom app on it." Later, not now. Someone with spare room on their DockLite server opens it up, and a person who just wants a small thing (a media server,
a blog, a game server) gets an account there without owning a server. From the guest's side it is a borrowed fleet node.
- **Start as a directory, not a marketplace.** Hosts opt in and say what they offer (how much, what's allowed, where they
  are, how to reach them); guests apply for an account; the host decides. Trust stays between people, the way Fediverse
  instances and community game servers work. The automatic "pick a host, deploy" flow comes later, if ever.
- **Not before multi-tenant hardening.** DockLite trusts its users today; a public host runs strangers' containers. Needs,
  in order: per-user networks (from stacks), quotas the host sets (CPU, memory, disk, bandwidth, container count),
  rootless Docker or gVisor with an image allowlist and no privileged containers, no path from a guest to the Docker
  socket or another user's files, and an outside security review of all of it.
- **Abuse is the host's problem, so give them the tools.** A kill switch per guest, an abuse report address on the listing,
  delisting from the directory, and plain terms the guest accepts. Media servers are the sharpest case: a stranger
  streaming a pirated library from a volunteer's IP is the volunteer's legal exposure. Say so in the host guide.
- **What DockLite can promise early.** The account model, the per-user folders and the directory itself are cheap. What it
  must not promise is that it's safe to host strangers until the hardening list above is done.

### Webrings, names and the quiet social network
**Why:** to get people off the computer, off the phone and outside. Moving away from endless scroll is the start. So the
reader is **finite by design**: it has a bottom ("you're caught up", then nothing), no notifications (you go look when you
feel like it; it never comes to you), time as the only sort order (nothing promoted, nothing competing), an optional daily
or weekly digest, and sources you add on purpose with a cap if you want one ("at most three a day from this one").
At the bottom, a friendly note: you can go back to the top and read them all again, because your friends deserve it,
and the second pass earns you a second reaction. The only "more" the reader ever offers is the same posts again, pointed
back at the people you already chose. Every later choice in this section is judged against that.

Someone types the name they want, sees if it's free, pays a couple of dollars, and has their own site on their own
domain, built with the ioi profile builder if they don't want to touch files, and connected to other people's sites.
- **Rings first.** A webring is a member list plus a previous / next / random widget: a small file (`.dklring`, to keep the
  family going) and a few lines on each member site. Cheap, no protocol work, and it makes the connected feeling real on
  day one.
- **Standards, not a protocol of our own.** Every profile site publishes an h-card (this is a person), an RSS feed, and a
  Webmention endpoint (when one site links to another, the other finds out). Those three turn a ring of profile sites into
  a slow, quiet social network with no server in the middle. ActivityPub (what Mastodon speaks) is the later step if the
  sites should appear in Fediverse feeds.
- **Buying the name.** A domain, not a TLD (a TLD is `.com` itself and is not for sale). Needs a registrar reseller API
  (Porkbun, Namecheap, OpenSRS; Cloudflare's registrar API doesn't open new registrations to most accounts), with
  Cloudflare taking over DNS afterwards through the existing integration. Payments make DockLite a service with a
  merchant account, refunds and support: a business decision, not a feature, so it goes last.
- Order: rings and the three standards (free, no permission needed), then the profile-builder on-ramp, then the registrar
  flow once there is a reason to run a business around it.
- **A reader, not a feed.** "There are 25 posts total ever" is the design constraint: nothing to rank, nothing real-time. A
  page that checks your friends' sites once an hour is the whole engine. Pull in other sources if you want; or just friends.
- **Friends are domains, not accounts.** A friend is any address that publishes a feed: an ioi site, a hand-written blog, a
  Mastodon profile (every Mastodon profile already has an RSS feed at its address plus `.rss`). Same reader for all.
- **Alongside Mastodon, not instead.** Consume it first (free, via RSS); later let Mastodon users follow an ioi site back
  (ActivityPub). Never a replacement for what the Fediverse is doing.
- **Two addresses, one site.** `ioi.me/username` is the home everyone gets; `username.com` is the same site once they buy
  a name. One vhost with two server names and a canonical link, the way www and non-www already work. No migration.
- **Registrar candidates.** IONOS (dollar-domain bundles; the loss leader is autorenew on the extras, and a reseller's math
  is the same: a few renewals pay for the ones that lapse; has a developer API for domains and DNS), Porkbun, Namecheap,
  OpenSRS. DNS hands off to Cloudflare afterwards either way.
- **Curating with a small model call, in your own words.** Instead of "at most three from this source", a person types what
  they want ("the ones about climbing, or anyone who sounds like they need help") and a small call picks from the new
  posts since last refresh. **The hard rule: it never learns.** No engagement signal, no counting the seconds a dog
  picture was on screen so the feed can become all dogs. The only input is the words the person typed; if they want
  dogs, they type "dogs" and get the dogs their friends actually posted. Also: the person can read their instructions
  back; it runs on pull over a handful of posts, so it
  narrows the pile and never adds to it; each pick says why; the rest stay one tap away under "the rest"; it is optional
  and per source, and the default is still everything, in order. Hosted or local model is a per-server setting, like a
  database engine; no key configured, no feature.
