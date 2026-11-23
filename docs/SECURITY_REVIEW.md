# Security review

A running record of the agent security review (roadmap, "Next up" #1). Each pass lists what was looked at, what was found, what was
fixed (with the test that proves it) and what is still open. Newest first.

## Pass 1 (2026-11-22): authentication, tokens, sessions, nginx editing

**Looked at:** login (`auth_login.go`), API tokens (`tokens.go`), the session cookie (`delegation.go`, `registry.go` `Auth()`), CSRF and
WebSocket origin checks, the route table (every route is behind `Auth()`, except login), the file manager's path checks, backup restore
extraction, the root helper's argument checks, and the raw nginx editor.

### Fixed in this pass
| # | Finding | Why it mattered | Fix | Test |
|---|---|---|---|---|
| 1 | **Any `admin` could write raw nginx config** (`PUT /api/nginx/sites/{domain}`), with no checks, and a config nginx rejected stayed on disk. | nginx's master process is root. `access_log /etc/cron.d/x` writes a root-owned file with request-controlled text; `include`, `load_module`, `*_temp_path` do similar. That is admin → root. A rejected config also broke every later reload for every site. | Only `super_admin` may edit by hand; the config is checked against a short list of safe directives and paths (`nginx_validate.go`); a config nginx rejects is rolled back to the previous one. | `nginx_validate_test.go` |
| 2 | **Token scopes were accepted and ignored.** | A token created as `["read"]` acted as a full admin, so nobody could rely on a "limited" token. | Scopes are `read`, `write` or `*`; anything else is refused when the token is made; a token without write cannot use POST/PUT/PATCH/DELETE. | `TestScopesAreValidated…` |
| 3 | **Login limits only counted one IP + username pair.** | Password spraying (many usernames from one address) and guessing one account from many addresses were not limited; the limiter table could grow without bound. | Added a per-address limit (40 failures / 5 min) and a per-username limit (20 / 15 min), with the table kept bounded (`login_limits.go`). | `TestSprayingManyUsernames…`, `TestOneAccountGuessed…` |
| 4 | **Unknown usernames answered faster than real ones.** | The difference in time showed which usernames exist. | A hash of a random password is checked for an unknown user too. | (timing; covered by review) |
| 5 | **Sessions survived a password change** (up to 24 h). | Someone who stole a session kept access after the owner changed the password. | The session carries a fingerprint of the password hash; a different one ends the session (`passwordVersion`). Older cookies without it keep working until they expire (≤ 24 h). | `TestPasswordChangeEndsOlderSessions` |
| 6 | **Tokens issued at login never expired by default**, unlike tokens made on the token page (90 days). | A leaked CLI token was valid forever. | Login tokens expire after 90 days unless `expires_at` is given. | `TestLoginTokensExpireByDefault` |

### Looked at and found sound
- Every route except `/api/auth/login` is behind `Auth()`; mutating routes carry the CSRF/origin check.
- The WebSocket terminal checks the request's Origin against its host; the terminal is limited to containers the caller may access.
- The file manager resolves symlinks before checking that a path stays under `/var/www/sites` (and under the user's own folder for non-admins).
- Backup extraction rejects `..` and symlinks that leave the destination.
- The root helper validates names, domains, paths and users before acting, and uses `mktemp` + `mv` for writes.
- The session cookie is `HttpOnly`, `SameSite=Lax`, `Secure` behind HTTPS; its signature is compared in constant time; the user and role are re-read from the database on every request.

### Still open (next passes)
- **Files API race:** a path is checked, then used; a site's own code (running as root in its container) could swap a symlink in between. Open with `O_NOFOLLOW` / re-check after open.
- **Login rate limits are in memory** (a restart forgets them); the delegation secret is the bootstrap token, so anyone holding it can forge any session.
- **`docklite-helper site-write`** writes whatever nginx config the agent hands it (no `nginx -t` + rollback inside the helper); the agent's own checks are now the only gate.
- **Containers** run as root inside; resource limits and non-root images are on the roadmap.
- **Hub tokens** (multi-server) are admin-level and never expire by design; they are revocable, but a read-only hub token would be safer now that scopes work.
- Webapp (Next.js) routes, the update path (`system_update.go`), the Docker create options for admin-supplied mounts, and uploads still need a pass.
- The planned **Security tab** in a container's details (failed logins, scanners, blocked requests).
