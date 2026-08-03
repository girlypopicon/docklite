# DockLite Development Worklog

This document tracks the active development work being done on DockLite. It is updated as work progresses.

---

## Current State

- Agent: rebuilt and running (Go binary in `bin/docklite-agent`)
- GUI: Next.js on bun, running on port 3002
- Stack: started via `./start-fullstack.sh`
- Default credentials: `superadmin` / `supersecretpassword123` (fresh installs)
- Runtime: switched from npm to bun (1.3.10)

---

## Session 1 — Stack Setup & Critical Bug Fixes

### Stack & Auth
- Found and fixed rate-limited login (too many attempts) by resetting DB password and restarting agent
- Changed default seed password from `admin` → `supersecretpassword123` in `webapp/lib/db.ts`
- Switched entire stack from npm to bun (`bun install` in 1.82s vs npm's ~30s)
- Updated `start-fullstack.sh` to use bun for all operations

### Token & Session Persistence
- `start-fullstack.sh` now reads/writes `.docklite-token` and `.docklite-session` to persist across restarts
- Both files are `chmod 600`
- `stop-all.sh` rewritten to kill only by PID files — no longer kills unrelated processes

### Container Creation Bugs Fixed (6 total)
1. **Wrong nginx port for static/PHP** — was using user-specified port instead of internal port 80
2. **Context timeout during image pull** — 30s context expired before large images pulled; switched to 15-minute background context (`ctxWithLongTimeout()`)
3. **Double dockerContext** — consolidated to single long-lived context in `createContainer`
4. **Node container startup** — changed from `npm start` → `node index.js` (removes npm dependency inside container)
5. **`/var/www/sites` permissions** — fixed in `install.sh` to be owned by docklite user
6. **Port binding race condition** — `getContainerHostPort` now retries 5× with 500ms gaps

### Container Port Bug Fixed (Session 2)
- **Random port on restart** — Docker re-draws from the ephemeral port range on every container start because `HostPort: "0"` was set at creation. `handleLifecycle` never updated nginx after start/restart.
- Fix: after any start/restart action, inspect container labels (`docklite.domain`, `docklite.include_www`, `docklite.internal_port`), detect new host port, and rewrite the nginx upstream config automatically.

### `.gitignore` Cleanup
- Added `bin/docklite-agent`, `bin/docklite-tui`, `.docklite-token`, `.docklite-session`

---

## Session 1 — Feature Work

### `.dkl` Manifest & `.dklpkg` Export/Import System
- `.dkl` — JSON manifest written to each site directory at container creation time; describes image, ports, env, domain, template type
- `.dklpkg` — gzipped tar archive (manifest + all site files) for portable site backups
- New endpoints:
  - `GET /api/containers/scan` — walk `/var/www/sites` for `.dkl` files, return registered/unregistered status
  - `POST /api/containers/onboard` — read `.dkl`, create DB record + Docker container from existing files
  - `POST /api/containers/import` — multipart upload of `.dklpkg`, extract, validate, provision
  - `GET /api/containers/<id>/export` — stream `.dklpkg` download
- New files: `go-app/internal/handlers/onboard.go`, `go-app/internal/handlers/pkgexport.go`

### Install Wizard (`install.sh` rewrite)
- Entry point detects existing install and routes to fresh or repair flow
- **Fresh install flow**: mode selection, port, admin account creation, systemd service, nginx, site scanning, build options
- **Repair flow**: health check shows what's broken; selective fixes for deps, binary rebuild, permissions, service restart, site onboarding
- Both flows call `onboard_dkl_sites()` to auto-register any `.dkl` sites found on disk

### MOTD Welcome Screen
- `webapp/scripts/motd/10-docklite` — bash script showing live agent status, version, container counts, web UI URL
- Installs to `/etc/update-motd.d/10-docklite` via both install flows
- Reads config from `/etc/docklite/docklite-agent.env`

---

## Session 2 — SSL Patch & Test Infrastructure

### SSL v2 (ACME/lego-based)
Applied `docklite-ssl-testing.patch` (which was corrupt — wrong line counts throughout). Extracted files manually and applied changes.

**New files added:**
- `go-app/internal/acme/manager.go` — ACME manager using lego v4 library; HTTP-01 and DNS-01 (Cloudflare) challenge support; certificate storage via `FileStore`
- `go-app/internal/acme/manager_test.go`
- `go-app/internal/handlers/ssl_v2.go` — `SSLManager` wrapper; 5 new HTTP handlers
- `SSL.md` — SSL management documentation

**New SSL v2 endpoints:**
| Endpoint | Method | Description |
|---|---|---|
| `/api/ssl/v2/status` | GET | Certificate status (ACME + certbot fallback) |
| `/api/ssl/v2/issue` | POST | Issue certificate via lego |
| `/api/ssl/v2/renew` | POST | Renew certificate |
| `/api/ssl/v2/delete` | POST | Delete/revoke certificate |
| `/api/ssl/v2/check-renewal` | GET/POST | Check renewal status |

**`sslManager` field** added to `Handlers` struct in `registry.go` (nil-safe — falls back to certbot if not initialised).

**go.mod** — added `github.com/go-acme/lego/v4 v4.17.4`; ran `go mod tidy`

### Test Infrastructure
Applied test files from patch (all were truncated, required fixes):

**New test files:**
- `go-app/internal/testhelpers/helpers.go` — `AssertEqual`, `AssertNoError`, `AssertTrue`, `TempDir`, `TempFile`, test DB helpers
- `go-app/internal/store/sqlite_test.go` — store lifecycle, WAL mode, concurrent writes
- `go-app/internal/store/users_test.go` — user CRUD tests
- `go-app/internal/handlers/ssl_test.go` — domain validation, cert parsing, SSL status
- `go-app/internal/handlers/handlers_test.go` — health, JSON helpers, templates, folder sorting
- `webapp/__tests__/db.test.ts` — database operation tests (Vitest)
- `webapp/__tests__/api.test.ts` — API integration tests (Vitest)
- `webapp/vitest.config.ts` — Vitest configuration
- `TESTING.md` — testing documentation

**Makefile** — added `test`, `test-go`, `test-web`, `test-coverage`, `install-test-deps` targets

### Bugs Fixed in Patch Code
The patch had several bugs (likely due to truncation or version mismatches):

| Bug | Fix |
|---|---|
| `certificate.ObtainRequest.CSR` doesn't exist in lego v4 | Removed manual CSR generation; lego generates keys internally |
| `h.sslManager` undefined | Added `sslManager *SSLManager` field to `Handlers` struct |
| `AssertEqual` panicked on `[]byte` comparison | Switched to `reflect.DeepEqual` |
| `isValidDomain` accepted bare TLDs, double hyphens, `localhost` | Tightened regex (require 2+ labels) + `strings.Contains(domain, "--")` check |
| `TempFile` deleted file before test used it | Fixed with `t.Cleanup()` instead of `defer cleanup()` |
| SQLite concurrent writes failed with `SQLITE_BUSY` | Added `db.SetMaxOpenConns(1)` to serialize writes |
| `NewSQLiteStore` couldn't create new DB files (`mode=rw`) | Changed DSN to `mode=rwc` |
| `sudo cat` test failed in non-sudo env | Added `sudo -n true` check; test skips gracefully |

**All Go tests pass:** `ok internal/acme`, `ok internal/handlers`, `ok internal/store`

---

## Pending / Next Steps

- [ ] Initialise `sslManager` in `main.go` using `SSL_EMAIL`, `SSL_PRODUCTION`, `CLOUDFLARE_API_TOKEN` env vars (currently always nil, falls back to certbot)
- [ ] Run webapp tests (`make test-web`) — requires `bun install` in webapp and vitest dependency
- [ ] Commit all changes to git (large number of modified/new files)
- [ ] Test full install flow on a clean Ubuntu machine
- [ ] Test repair flow on an existing installation
- [ ] Test `.dklpkg` export/import with a live site
- [ ] Verify MOTD displays correctly on SSH login

---

## Key Environment Details

| Item | Value |
|---|---|
| Repo root | `/home/DOCKLITE_GEM_TEST` |
| Agent port | 3000 |
| GUI port | 3002 |
| DB | `data/docklite.db` (SQLite) |
| Bun version | 1.3.10 |
| Go version | 1.22 |
| Default admin user | `superadmin` |
