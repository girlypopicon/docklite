# SSL Certificate Management

DockLite includes built-in SSL certificate management using ACME (Let's Encrypt) with support for both HTTP-01 and DNS-01 challenge types.

## Overview

The SSL system uses [lego](https://github.com/go-acme/lego), a pure Go ACME client library, which provides:

- **No external dependencies** - No need to install certbot
- **No sudo required** - Runs as the application user
- **Fully testable** - Can mock ACME servers for unit tests
- **Multiple challenge types** - HTTP-01 and DNS-01
- **100+ DNS providers** - Cloudflare, Route53, DigitalOcean, etc.

## Features

| Feature | Description |
|---------|-------------|
| **Automatic Issuance** | Issue certificates with a single API call |
| **Automatic Renewal** | Certificates auto-renew before expiry |
| **DNS Challenge** | Issue certs without port 80 (via Cloudflare) |
| **HTTP Challenge** | Standard HTTP-01 challenge support |
| **Certificate Storage** | Automatic PEM file management |
| **Status Monitoring** | Track certificate expiry status |

## Configuration

### Environment Variables

```bash
# ACME Account Email (required)
SSL_EMAIL=admin@example.com

# Use production Let's Encrypt (default: false, uses staging)
SSL_PRODUCTION=true

# Certificate storage directory
SSL_CERT_DIR=/var/lib/docklite/certs

# Cloudflare API Token (for DNS-01 challenge)
CLOUDFLARE_API_TOKEN=your-api-token

# Prefer DNS challenge over HTTP
SSL_PREFER_DNS=true
```

### Cloudflare DNS Integration

To use DNS-01 challenge (recommended for internal/development servers):

1. Create a Cloudflare API token with DNS edit permissions
2. Configure the token in DockLite:
   ```bash
   export CLOUDFLARE_API_TOKEN=your-token-here
   export SSL_PREFER_DNS=true
   ```

## API Endpoints

### v2 ACME Endpoints (Recommended)

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/ssl/v2/status` | GET | Get certificate status |
| `/api/ssl/v2/issue` | POST | Issue new certificate |
| `/api/ssl/v2/renew` | POST | Renew certificate |
| `/api/ssl/v2/delete` | POST | Delete/revoke certificate |
| `/api/ssl/v2/check-renewal` | GET/POST | Check renewal status |

### Issue Certificate

```bash
curl -X POST http://localhost:3000/api/ssl/v2/issue \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "domain": "example.com",
    "includeWww": true,
    "method": "dns"
  }'
```

### Check Certificate Status

```bash
curl http://localhost:3000/api/ssl/v2/status \
  -H "Authorization: Bearer $TOKEN"
```

## Certificate Storage

Certificates are stored in PEM format:

```
/var/lib/docklite/certs/
├── example.com.crt    # Certificate chain
├── example.com.key    # Private key
└── ...
```

## Fallback to Certbot

If the ACME manager is not configured, DockLite falls back to certbot:

1. Ensure certbot is installed: `apt install certbot python3-certbot-nginx`
2. Certificates are stored in `/etc/letsencrypt/live/`

The API endpoints are compatible between ACME and certbot modes.

## Development & Testing

### Using Staging Environment

By default, DockLite uses Let's Encrypt staging (for development):

```bash
# Staging (default) - Rate limits are relaxed, certs not trusted by browsers
SSL_PRODUCTION=false

# Production - Real certificates, strict rate limits
SSL_PRODUCTION=true
```

## Rate Limits

### Let's Encrypt Production

- **Certificates per domain**: 50 per week
- **Certificates per account**: 300 per week
- **Failed validations**: 5 failures per hour per account

### Let's Encrypt Staging

- **Certificates per domain**: 30,000 per week
- **Certificates per account**: 30,000 per week

## Architecture

```
┌─────────────────┐
│   API Request   │
│  /api/ssl/v2/*  │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  SSL Handler    │
│ (ssl_v2.go)     │
└────────┬────────┘
         │
         ▼
┌─────────────────┐     ┌──────────────────┐
│  SSL Manager    │────▶│  ACME Manager    │
│                 │     │  (lego)          │
└────────┬────────┘     └────────┬─────────┘
         │                       │
         ▼                       ▼
┌─────────────────┐     ┌──────────────────┐
│  File Store     │     │  Let's Encrypt   │
│  (PEM files)    │     │  ACME Server     │
└─────────────────┘     └──────────────────┘
```
