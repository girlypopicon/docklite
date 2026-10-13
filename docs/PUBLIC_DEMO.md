# The public demo (demo.docklite.net)

A try-it-yourself copy of the dashboard that needs no server behind it. Every page works, but everything the
visitor sees is made up and lives only in their browser tab; closing the tab throws it away.

## How it works

- The dashboard is built with `NEXT_PUBLIC_DOCKLITE_DEMO=1` and exported as plain static files (`output: 'export'`).
- `webapp/lib/demo/` replaces `fetch('/api/...')` and the terminal's WebSocket with fakes, so no request ever
  leaves the browser. `state.ts` holds the fake world (users, sites, databases, backups), `api.ts` and `host.ts`
  answer the API calls, and `shell.ts` is a pretend shell.
- State is kept in memory and in `sessionStorage`, so a page reload in the same tab keeps your changes and closing
  the tab (or pressing **Reset** in the banner) clears them. Nothing is sent anywhere.
- Nothing real can happen: creating a site, issuing a certificate, running a backup or a SQL statement only
  changes the fake world. The demo can't reach Docker, nginx, files or a database because none of it is there.
- Normal builds contain none of this code (the flag is resolved at build time and the demo modules are dropped).

## Build

```bash
scripts/build-public-demo.sh              # writes dist/demo (about 230 files, 13 MB)
scripts/build-public-demo.sh --package    # also writes dist/docklite-demo.tar.gz (7 MB) to hand to a web server
scripts/build-public-demo.sh --serve      # also serves it on http://127.0.0.1:3300 to look at
```

The build runs in a scratch copy of `webapp/`, so it never touches an installed build. It needs `npm install` to have been run
in `webapp/` once. Before it finishes it scans the output and **refuses to produce it** if it finds this machine's username, a
`/home/` path, a public-looking IP address or a real email address (add your own words with `DEMO_FORBIDDEN='word|other'`).

Each release publishes the packaged demo as `docklite-demo.tar.gz` on its GitHub release, so a web server never needs Node or a build.

## Put it on a web server (nginx)

No Node, no database: it is static files. On the server that hosts `demo.docklite.net`:

```bash
sudo mkdir -p /var/www/demo.docklite.net
curl -fsSL https://github.com/girlypopicon/docklite/releases/latest/download/docklite-demo.tar.gz \
  | sudo tar -xz -C /var/www/demo.docklite.net
```

and a server block that answers only for that name (never `default_server`, so other hostnames can't reach it):

```nginx
server {
    listen 443 ssl;
    http2 on;
    server_name demo.docklite.net;
    ssl_certificate     /etc/letsencrypt/live/<your-cert-name>/fullchain.pem;   # the certificate that covers demo.docklite.net
    ssl_certificate_key /etc/letsencrypt/live/<your-cert-name>/privkey.pem;

    root /var/www/demo.docklite.net;
    index index.html;

    location / { try_files $uri $uri/ /404.html; }
    location /_next/static/ { expires 1y; add_header Cache-Control "public, immutable"; }

    add_header X-Content-Type-Options nosniff always;
    add_header Referrer-Policy no-referrer always;
}
```

Test with `sudo nginx -t`, then `sudo systemctl reload nginx` (a reload, not a restart: other sites keep serving). Behind
Cloudflare, the SSL mode **Full** works with any valid or self-signed origin certificate, and **Full (strict)** needs one that
covers `demo.docklite.net` (a `*.docklite.net` certificate does). A 525 from Cloudflare means nothing on the origin answers
TLS for that name yet; this server block fixes it. To update the demo, run the `curl | tar` line again (it overwrites the files).

Any static host (Cloudflare Pages, GitHub Pages, S3) also works: upload the unpacked folder as is.

## Keeping it current

Rebuild and re-copy after UI changes. When the dashboard gains a new `/api` call, add an answer for it in
`webapp/lib/demo/api.ts` (or `host.ts`); calls that aren't answered show "not available in the demo" and log
`[demo] not implemented` in the browser console, which makes gaps easy to find.
