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
scripts/build-public-demo.sh            # writes dist/demo (about 220 files, 13 MB)
scripts/build-public-demo.sh --serve    # also serves it on http://127.0.0.1:3300 to look at
```

The build runs in a scratch copy of `webapp/`, so it never touches an installed build. It needs `npm install` to
have been run in `webapp/` once.

## Serve it with nginx

Copy `dist/demo/` to the server and point a virtual host at it:

```nginx
server {
    listen 80;
    server_name demo.docklite.net;
    root /var/www/demo.docklite.net;
    index index.html;

    location / { try_files $uri $uri/ /404.html; }
    location /_next/static/ { expires 1y; add_header Cache-Control "public, immutable"; }

    add_header X-Content-Type-Options nosniff always;
    add_header Referrer-Policy no-referrer always;
}
```

Behind Cloudflare, use SSL mode "Full" with a certificate on the origin, or "Flexible" if the origin only speaks
HTTP. Any static host (Cloudflare Pages, GitHub Pages, S3) also works: upload the folder as is.

## Keeping it current

Rebuild and re-copy after UI changes. When the dashboard gains a new `/api` call, add an answer for it in
`webapp/lib/demo/api.ts` (or `host.ts`); calls that aren't answered show "not available in the demo" and log
`[demo] not implemented` in the browser console, which makes gaps easy to find.
