package handlers

// nodeStarterIndexJS is the index.js DockLite adds (only when the site has none, and only when asked) so a site made of
// plain files can run as a Node.js app. It has no dependencies, serves only the folder's own files, and never serves
// hidden files (.dkl and friends), itself or package.json.
const nodeStarterIndexJS = `// Starter server added by DockLite: it serves this folder's files (index.html, style.css, app.js, fonts...) so a site that
// was static files can run as a Node.js app. Replace it with your own server whenever you like.
// No dependencies. Hidden files (.dkl and friends), this file and package.json are never served.
const http = require('http');
const fs = require('fs');
const path = require('path');

const ROOT = __dirname;
const PORT = Number(process.env.PORT) || 3000;
const HIDDEN = new Set(['index.js', 'package.json', 'package-lock.json']);
const TYPES = {
  '.html': 'text/html; charset=utf-8',
  '.css': 'text/css; charset=utf-8',
  '.js': 'text/javascript; charset=utf-8',
  '.svg': 'image/svg+xml',
  '.woff2': 'font/woff2',
  '.woff': 'font/woff',
  '.png': 'image/png',
  '.jpg': 'image/jpeg',
  '.jpeg': 'image/jpeg',
  '.gif': 'image/gif',
  '.webp': 'image/webp',
  '.ico': 'image/x-icon',
  '.json': 'application/json; charset=utf-8',
  '.txt': 'text/plain; charset=utf-8',
};

function resolve(urlPath) {
  let rel;
  try {
    rel = decodeURIComponent(urlPath.split('?')[0].split('#')[0]);
  } catch {
    return null;
  }
  if (rel.includes('\0')) return null;
  if (rel.endsWith('/')) rel += 'index.html';
  const file = path.normalize(path.join(ROOT, rel));
  if (file !== ROOT && !file.startsWith(ROOT + path.sep)) return null;
  const parts = path.relative(ROOT, file).split(path.sep);
  if (parts.some((p) => p.startsWith('.'))) return null;
  if (parts.length === 1 && HIDDEN.has(parts[0])) return null;
  return file;
}

http
  .createServer((req, res) => {
    if (req.method !== 'GET' && req.method !== 'HEAD') {
      res.writeHead(405, { Allow: 'GET, HEAD' }).end();
      return;
    }
    const file = resolve(req.url || '/');
    if (!file) {
      res.writeHead(404).end('Not found');
      return;
    }
    fs.stat(file, (err, st) => {
      if (err || !st.isFile()) {
        res.writeHead(404, { 'Content-Type': 'text/plain; charset=utf-8' }).end('Not found');
        return;
      }
      res.writeHead(200, {
        'Content-Type': TYPES[path.extname(file).toLowerCase()] || 'application/octet-stream',
        'Content-Length': st.size,
        'X-Content-Type-Options': 'nosniff',
        'Cache-Control': 'public, max-age=300',
      });
      if (req.method === 'HEAD') return res.end();
      fs.createReadStream(file).pipe(res);
    });
  })
  .listen(PORT, '0.0.0.0', () => console.log('listening on ' + PORT));
`
