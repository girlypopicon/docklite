# DockLite SSL & Testing Patch

This patch contains all the SSL and testing improvements made to the docklite-new repository.

## What's Included

### 1. ACME SSL Management (lego-based)
- **`go-app/internal/acme/manager.go`** - Core ACME manager with lego library
- **`go-app/internal/acme/manager_test.go`** - ACME manager tests
- **`go-app/internal/handlers/ssl_v2.go`** - SSL v2 API handlers

### 2. Test Infrastructure
- **`go-app/internal/testhelpers/helpers.go`** - Common test utilities
- **`go-app/internal/store/sqlite_test.go`** - SQLite store tests
- **`go-app/internal/store/users_test.go`** - User store tests
- **`go-app/internal/handlers/ssl_test.go`** - SSL validation tests
- **`go-app/internal/handlers/handlers_test.go`** - Handler utility tests
- **`webapp/__tests__/db.test.ts`** - Database tests
- **`webapp/__tests__/api.test.ts`** - API integration tests

### 3. Documentation
- **`TESTING.md`** - Testing documentation
- **`SSL.md`** - SSL management documentation

### 4. Configuration Changes
- **`go-app/go.mod`** - Added lego dependency
- **`go-app/internal/api/router.go`** - Added v2 SSL endpoints
- **`Makefile`** - Added test targets

## How to Apply

### Option 1: Git Apply (Recommended)

```bash
cd /path/to/docklite-new
git apply docklite-ssl-testing.patch
```

### Option 2: Patch Command

```bash
cd /path/to/docklite-new
patch -p1 < docklite-ssl-testing.patch
```

### Option 3: Manual Application

If the patch doesn't apply cleanly, you can manually create the files from the patch content.

## After Applying

1. **Install Go dependencies:**
   ```bash
   cd go-app && go mod tidy
   ```

2. **Run Go tests:**
   ```bash
   make test-go
   ```

3. **Install webapp dependencies:**
   ```bash
   cd webapp && bun install
   ```

4. **Run webapp tests:**
   ```bash
   make test-web
   ```

## New API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/ssl/v2/status` | GET | Get certificate status |
| `/api/ssl/v2/issue` | POST | Issue new certificate |
| `/api/ssl/v2/renew` | POST | Renew certificate |
| `/api/ssl/v2/delete` | POST | Delete/revoke certificate |
| `/api/ssl/v2/check-renewal` | GET/POST | Check renewal status |

## Environment Variables

```bash
SSL_EMAIL=admin@example.com
SSL_PRODUCTION=false  # Use staging by default
SSL_CERT_DIR=/var/lib/docklite/certs
CLOUDFLARE_API_TOKEN=your-token  # For DNS-01 challenge
SSL_PREFER_DNS=true
```

## Test Commands

```bash
# All tests
make test

# Go tests only
make test-go

# Webapp tests only
make test-web

# With coverage
make test-coverage
```
