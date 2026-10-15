// The demo's file browser: a small fake file system under /var/www/sites/<user>/<domain>, kept in the
// visitor's tab like the rest of the demo. Edit, create, upload, move, copy and delete all work, but only on
// this pretend disk.
import type { DemoRequest, DemoResponse } from './api';
import { save, type DemoState, type FsNode } from './state';

const BASE = '/var/www/sites';
const MAX_NODES = 600;
const MAX_FILE = 64 * 1024;

const ok = (body: any = { success: true }): DemoResponse => ({ status: 200, body });
const err = (status: number, message: string): DemoResponse => ({ status, body: { error: message } });

function seedFiles(template: string, domain: string): Record<string, string | null> {
  const title = domain;
  if (template === 'php') {
    return {
      'index.php': `<?php\nrequire __DIR__ . '/config.php';\n?>\n<!doctype html>\n<title><?= SITE_NAME ?></title>\n<h1><?= SITE_NAME ?></h1>\n<p>Welcome to the shop.</p>\n`,
      'config.php': `<?php\ndefine('SITE_NAME', '${title}');\ndefine('CURRENCY', 'USD');\n`,
      'composer.json': `{\n  "name": "demo/${domain}",\n  "require": { "php": ">=8.1" }\n}\n`,
      'includes/': null, 'includes/header.php': '<header><nav>Shop · Cart · Contact</nav></header>\n',
      'uploads/': null,
    };
  }
  if (template === 'node') {
    return {
      'index.js': `const http = require('http');\nconst items = [{ id: 1, name: 'First' }, { id: 2, name: 'Second' }];\n\nhttp.createServer((req, res) => {\n  res.setHeader('Content-Type', 'application/json');\n  res.end(JSON.stringify(items));\n}).listen(3000, () => console.log('listening on :3000'));\n`,
      'package.json': `{\n  "name": "${domain}",\n  "version": "1.0.0",\n  "main": "index.js",\n  "scripts": { "start": "node index.js" }\n}\n`,
      'README.md': `# ${title}\n\nA tiny JSON API running in a DockLite container.\n`,
      'routes/': null, 'routes/items.js': "module.exports = (req, res) => res.end('[]');\n",
    };
  }
  return {
    'index.html': `<!doctype html>\n<html lang="en">\n<head>\n  <meta charset="utf-8">\n  <title>${title}</title>\n  <link rel="stylesheet" href="style.css">\n</head>\n<body>\n  <h1>${title}</h1>\n  <p>This page is served by DockLite.</p>\n  <a href="about.html">About</a>\n</body>\n</html>\n`,
    'about.html': `<!doctype html>\n<title>About ${title}</title>\n<h1>About</h1>\n<p>Edit this file from the File Browser.</p>\n`,
    'style.css': `body {\n  font-family: system-ui, sans-serif;\n  max-width: 40rem;\n  margin: 4rem auto;\n  color: #222;\n}\n`,
    'assets/': null, 'assets/logo.svg': '<svg xmlns="http://www.w3.org/2000/svg" width="64" height="64"><circle cx="32" cy="32" r="28" fill="#ff6ad5"/></svg>\n',
  };
}

/** Make sure the fake disk has a folder for every user and site, and none for sites that were deleted. */
function ensureFs(s: DemoState): Record<string, FsNode> {
  const fs = (s.fs ??= { [BASE]: { dir: true } });
  const seeded = (s.fsSites ??= []);
  const wanted = new Set<string>();
  for (const u of s.users) fs[`${BASE}/${u.username}`] ??= { dir: true };
  for (const site of s.sites) {
    const owner = s.users.find((u) => u.id === site.userId)?.username ?? 'demo';
    const dir = `${BASE}/${owner}/${site.domain}`;
    wanted.add(dir);
    if (!seeded.includes(dir)) {
      seeded.push(dir);
      fs[`${BASE}/${owner}`] ??= { dir: true };
      fs[dir] = { dir: true };
      for (const [rel, content] of Object.entries(seedFiles(site.template, site.domain))) {
        if (rel.endsWith('/')) fs[`${dir}/${rel.slice(0, -1)}`] = { dir: true };
        else fs[`${dir}/${rel}`] = { dir: false, content: content ?? '' };
      }
    }
  }
  // A deleted (or moved) site takes its folder with it.
  s.fsSites = seeded.filter((dir) => {
    if (wanted.has(dir)) return true;
    for (const p of Object.keys(fs)) if (p === dir || p.startsWith(dir + '/')) delete fs[p];
    return false;
  });
  return fs;
}

/** Turn what the UI sends (absolute or relative) into a path inside the fake disk, or null if it escapes. */
function resolve(p: unknown): string | null {
  let raw = String(p ?? '').trim() || BASE;
  if (!raw.startsWith('/')) raw = `${BASE}/${raw}`;
  const out: string[] = [];
  for (const part of raw.split('/')) {
    if (!part || part === '.') continue;
    if (part === '..') { out.pop(); continue; }
    out.push(part);
  }
  const path = '/' + out.join('/');
  return path === BASE || path.startsWith(BASE + '/') ? path : null;
}

const parent = (p: string) => p.slice(0, p.lastIndexOf('/')) || '/';
const baseName = (p: string) => p.slice(p.lastIndexOf('/') + 1);
const under = (fs: Record<string, FsNode>, dir: string) => Object.keys(fs).filter((p) => p === dir || p.startsWith(dir + '/'));
const validName = (n: string) => !!n && n.length <= 80 && !/[\\/\0]/.test(n) && n !== '.' && n !== '..';
// Site folders belong to the site; delete the site itself from the Containers page.
const isProtected = (s: DemoState, p: string) => p === BASE || p.split('/').length <= 6 && (s.fsSites ?? []).includes(p) || parent(p) === BASE;

export function filesApi(req: DemoRequest, s: DemoState): DemoResponse | null {
  const { method, path, query, body } = req;
  if (!path.startsWith('/api/files')) return null;
  const fs = ensureFs(s);

  if (method === 'GET' && path === '/api/files') {
    const dir = resolve(query.get('path'));
    if (!dir) return err(403, 'path outside base directory');
    if (!fs[dir]) return err(404, 'folder not found');
    if (!fs[dir].dir) return err(400, 'not a folder');
    const entries = Object.keys(fs).filter((p) => parent(p) === dir && p !== dir).map((p) => ({ name: baseName(p), isDirectory: fs[p].dir }))
      .sort((a, b) => (a.isDirectory === b.isDirectory ? a.name.localeCompare(b.name) : a.isDirectory ? -1 : 1));
    return ok({ entries });
  }

  if (method === 'GET' && path === '/api/files/content') {
    const p = resolve(query.get('path')); const node = p ? fs[p] : null;
    if (!p) return err(403, 'path outside base directory');
    if (!node || node.dir) return err(404, 'file not found');
    return ok({ content: node.content ?? '' });
  }

  if (method === 'GET' && path === '/api/files/download') {
    const p = resolve(query.get('path')); const node = p ? fs[p] : null;
    if (!p || !node || node.dir) return err(404, 'file not found');
    return { status: 200, body: node.content ?? '' };
  }

  if (method === 'POST' && path === '/api/files/content') {
    const p = resolve(body?.filePath ?? body?.path);
    if (!p) return err(403, 'path outside base directory');
    if (!fs[parent(p)]?.dir) return err(404, 'folder not found');
    if (fs[p]?.dir) return err(400, 'that is a folder');
    const content = String(body?.content ?? '');
    if (content.length > MAX_FILE) return err(400, 'The demo keeps files under 64 KB');
    if (!fs[p] && Object.keys(fs).length >= MAX_NODES) return err(400, 'The demo disk is full. Press Reset to start fresh.');
    fs[p] = { dir: false, content }; save(); return ok();
  }

  if (method === 'POST' && path === '/api/files/create') {
    const dir = resolve(body?.basePath); const name = String(body?.name ?? '').trim();
    if (!dir) return err(403, 'path outside base directory');
    if (!fs[dir]?.dir) return err(404, 'folder not found');
    if (!validName(name)) return err(400, 'Use a simple name without slashes');
    const target = `${dir}/${name}`;
    if (fs[target]) return err(409, `${name} already exists`);
    if (Object.keys(fs).length >= MAX_NODES) return err(400, 'The demo disk is full. Press Reset to start fresh.');
    fs[target] = body?.type === 'folder' ? { dir: true } : { dir: false, content: '' }; save(); return ok();
  }

  if (method === 'POST' && path === '/api/files/upload') {
    const dir = resolve(body?.path); const file = body?.file;
    if (!dir || !fs[dir]?.dir || !file?.name) return err(400, 'Choose a file');
    if (!validName(file.name)) return err(400, 'Use a simple file name');
    if (Object.keys(fs).length >= MAX_NODES) return err(400, 'The demo disk is full. Press Reset to start fresh.');
    // Only small text files are kept; anything else is recorded by name so the listing still looks right.
    fs[`${dir}/${file.name}`] = { dir: false, content: typeof file.text === 'string' ? file.text : `(demo: ${file.name}, ${file.size} bytes; real contents are not stored)\n` };
    save(); return ok({ success: true, name: file.name });
  }

  if (method === 'DELETE' && path === '/api/files/delete') {
    const p = resolve(body?.path);
    if (!p || !fs[p]) return err(404, 'not found');
    if (isProtected(s, p)) return err(400, 'This is a site folder. Delete the site from the Containers page instead.');
    for (const q of under(fs, p)) delete fs[q];
    save(); return ok();
  }

  if (method === 'POST' && path === '/api/files/rename') {
    const p = resolve(body?.path); const name = String(body?.new_name ?? '').trim();
    if (!p || !fs[p]) return err(404, 'not found');
    if (isProtected(s, p)) return err(400, 'This is a site folder; it can not be renamed here.');
    if (!validName(name)) return err(400, 'Use a simple name without slashes');
    const target = `${parent(p)}/${name}`;
    if (fs[target]) return err(409, `${name} already exists`);
    for (const q of under(fs, p)) { fs[target + q.slice(p.length)] = fs[q]; delete fs[q]; }
    save(); return ok();
  }

  if (method === 'POST' && path === '/api/files/transfer') {
    const src = resolve(body?.sourcePath); const dest = resolve(body?.targetDir); const move = body?.action === 'move';
    if (!src || !dest) return err(403, 'path outside base directory');
    if (!fs[src]) return err(404, 'source not found');
    if (!fs[dest]?.dir) return err(404, 'target folder not found');
    if (dest === src || dest.startsWith(src + '/')) return err(400, 'Cannot put a folder inside itself');
    if (move && isProtected(s, src)) return err(400, 'This is a site folder; it can not be moved here.');
    const target = `${dest}/${baseName(src)}`;
    if (fs[target]) return err(409, `${baseName(src)} already exists there`);
    const items = under(fs, src);
    if (Object.keys(fs).length + items.length >= MAX_NODES) return err(400, 'The demo disk is full. Press Reset to start fresh.');
    for (const q of items) { fs[target + q.slice(src.length)] = { ...fs[q] }; if (move) delete fs[q]; }
    save(); return ok({ success: true, targetPath: target });
  }

  return null;
}
