# DockLite Implementation TODO

## P0 - Critical

- [x] Harden update execution path in `go-app/internal/handlers/system_update.go`.
  - Validate update script ownership and permissions before execution.
  - Add integrity check (hash/signature allowlist) for update script.
  - Add structured security/audit logging for update runs.

- [ ] Keep debug features disabled by default.
  - Ensure startup scripts do not force debug flags.
  - Consider route-registration-level gating for debug routes in `go-app/internal/api/router.go`.

## P1 - High Priority

- [ ] Enforce secure database permissions (600) in all setup/start scripts.
  - `start-agent.sh`
  - `start-fullstack.sh`
  - `init-db.sh`

- [ ] Remove weak session secret fallback in full-stack startup.
  - Avoid predictable default secret values.
  - Ensure `SESSION_SECRET` is always strong (provided or generated securely).

- [x] Add CSRF + Origin validation for state-changing API routes.
  - Add middleware for `POST`, `PUT`, `PATCH`, `DELETE`.
  - Add token issuance/validation flow for web client calls.

- [x] Strengthen cookie security policy.
  - Keep insecure-cookie bypass test-only.
  - Verify secure behavior behind reverse proxies.

- [ ] Add Docker socket startup validation.
  - Verify socket path exists and is a socket.
  - Warn on world-writable socket permissions.

- [ ] Remove token leakage in startup output.
  - Do not print full/partial tokens.
  - Prefer secure token file handoff with mode 600.

## P2 - Medium Priority

- [ ] Add default token expiration and lifecycle policy in `go-app/internal/handlers/tokens.go`.
  - Default TTL for new tokens (e.g., 90 days).
  - Explicit admin override for non-expiring tokens.

- [ ] Persist login rate limiting.
  - Move in-memory limiter to durable storage or add robust cleanup/backoff.

- [ ] Add audit logging for security-sensitive actions.
  - Login success/failure.
  - Token create/revoke.
  - User/role changes.
  - Destructive operations (delete/prune/update).

- [ ] Add command timeouts/safer execution wrappers for privileged commands.
  - `ssl.go`, `nginx.go`, and other shell execution paths.

- [ ] Expand automated tests.
  - Auth middleware (`registry.go`, login flow).
  - File access boundaries (`files.go`).
  - Token lifecycle behavior (`tokens.go`).

- [ ] Dependency and vulnerability maintenance.
  - Review/upgrade Go deps in `go-app/go.mod`.
  - Review/upgrade Node deps in `webapp/package.json`.
  - Add CI security scanning.

## Phased Plan

### Phase 1 - Security Defaults (completed)

- [x] Create implementation TODO backlog.
- [x] Harden startup scripts:
  - DB permissions changed to 600.
  - Removed forced debug default in full-stack startup.
  - Replaced predictable session-secret fallback with secure generation.
  - Reduced token exposure in script output.
- [x] Validate shell behavior and startup paths end-to-end.

### Phase 2 - Auth Boundary Hardening (in progress)

- [x] Implement CSRF/origin protections:
  - Created CSRF middleware in `go-app/internal/handlers/csrf.go`
  - Added `/api/csrf-token` endpoint for optional CSRF token generation
  - Added origin validation for security-conscious clients
  - Note: Token-based API auth doesn't require CSRF (no browser cookies involved)
- [x] Add default token TTL policy (90 days) in `go-app/internal/handlers/tokens.go`:
  - Tokens now automatically expire 90 days after creation if no explicit expiry is set
  - Admin can still override with explicit `expires_at` when creating tokens
- [x] Tighten secure-cookie behavior and proxy checks.
- [x] Add rate limiting for token creation/revocation.

### Phase 3 - Operational Safety

- [ ] Harden system update command path.
- [ ] Add command timeout wrappers for privileged handlers.
- [ ] Improve structured errors for operational endpoints.

### Phase 4 - Reliability and Observability

- [ ] Implement persistent rate limiting.
- [ ] Add audit logging pipeline and storage.
- [ ] Expand diagnostics for startup and auth flows.

### Phase 5 - Tests and Upgrades

- [ ] Add targeted Go tests for critical security paths.
- [ ] Add CI checks for vulnerabilities and dependency drift.
- [ ] Complete controlled dependency updates.
