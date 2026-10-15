// The fake DockLite API. Every /api request the dashboard makes in demo mode lands here and is
// answered from lib/demo/state.ts. Nothing leaves the browser; nothing is real.
import { fakeHex, getState, save, T, type DemoDb, type DemoSite, type DemoState } from './state';
import { hostApi } from './host';
import { filesApi } from './files';

export interface DemoRequest { method: string; path: string; query: URLSearchParams; body: any }
export interface DemoResponse { status: number; body: any }

const ok = (body: any = { success: true }): DemoResponse => ({ status: 200, body });
const err = (status: number, message: string): DemoResponse => ({ status, body: { error: message } });
const iso = (ms: number) => new Date(ms).toISOString();
const IMAGES = { static: 'nginx:alpine', php: 'webdevops/php-nginx:8.2-alpine', node: 'node:20-alpine' } as const;
const MAX_SITES = 24;

export function formatUptime(ms: number): string {
  const s = Math.max(0, Math.floor(ms / 1000));
  if (s < 60) return `${s}s`;
  const m = Math.floor(s / 60);
  if (m < 60) return `${m}m`;
  const h = Math.floor(m / 60);
  if (h < 24) return `${h}h ${m % 60}m`;
  return `${Math.floor(h / 24)}d`;
}

function dockerStatus(running: boolean, since: number): string {
  const ago = formatUptime(Date.now() - since);
  return running ? `Up ${ago}` : `Exited (0) ${ago} ago`;
}

const siteName = (d: string) => `docklite-site-${d.replace(/\./g, '-')}`;
const dbName = (n: string) => `docklite-db-${n.replace(/_/g, '-')}`;
const username = (s: DemoState, id: number | null) => s.users.find((u) => u.id === id)?.username ?? 'demo';

function siteInfo(s: DemoState, site: DemoSite) {
  const since = site.running ? site.startedAt : site.stoppedAt ?? site.createdAt;
  return {
    id: site.containerId, name: siteName(site.domain), image: IMAGES[site.template],
    created: Math.floor(site.createdAt / 1000), state: site.running ? 'running' : 'exited', status: dockerStatus(site.running, since),
    uptime: site.running ? formatUptime(Date.now() - site.startedAt) : '-',
    ports: site.running ? `${site.hostPort}->${site.internalPort}` : '',
    labels: {
      'docklite.managed': 'true', 'docklite.type': site.template, 'docklite.domain': site.domain, 'docklite.site.id': String(site.id),
      'docklite.user.id': String(site.userId), 'docklite.include_www': String(site.includeWww), 'docklite.internal_port': String(site.internalPort),
    },
    owner_username: username(s, site.userId),
  };
}

function dbInfo(db: DemoDb) {
  return {
    id: db.containerId, name: dbName(db.name), image: 'postgres:16-alpine', created: Math.floor(db.createdAt / 1000),
    state: db.running ? 'running' : 'exited', status: dockerStatus(db.running, db.startedAt), uptime: db.running ? formatUptime(Date.now() - db.startedAt) : '-',
    ports: db.running ? `${db.port}->5432, ${db.port}->5432` : '',
    labels: { 'docklite.managed': 'true', 'docklite.type': 'postgres', 'docklite.database': db.name, 'docklite.username': db.username, 'docklite.password': db.password },
  };
}

const allContainers = (s: DemoState) => [...s.sites.map((x) => siteInfo(s, x)), ...s.dbs.map(dbInfo)];

function findContainer(s: DemoState, id: string): { site?: DemoSite; db?: DemoDb } | null {
  const site = s.sites.find((x) => x.containerId === id || x.containerId.startsWith(id));
  if (site) return { site };
  const db = s.dbs.find((x) => x.containerId === id || x.containerId.startsWith(id));
  return db ? { db } : null;
}

function logsFor(s: DemoState, found: { site?: DemoSite; db?: DemoDb }): string {
  const now = Date.now();
  const stamp = (offset: number) => new Date(now - offset).toISOString().replace('T', ' ').slice(0, 19);
  const lines: string[] = [];
  if (found.db) {
    lines.push(`${stamp(9e6)} UTC [1] LOG:  database system is ready to accept connections`);
    for (let i = 0; i < 6; i++) lines.push(`${stamp(6e5 - i * 9e4)} UTC [${40 + i}] LOG:  checkpoint complete: wrote ${12 + i * 3} buffers (0.1%)`);
  } else if (found.site) {
    const { template, domain } = found.site;
    if (template === 'node') {
      lines.push(`[${stamp(5e6)}] server listening on :3000`);
      for (let i = 0; i < 8; i++) lines.push(`[${stamp(4e5 - i * 4e4)}] GET /api/items 200 ${3 + i}ms`);
    } else {
      const ua = '"Mozilla/5.0 (X11; Linux x86_64)"';
      for (let i = 0; i < 10; i++) lines.push(`172.21.0.1 - - [${stamp(5e5 - i * 4e4)}] "GET ${['/', '/style.css', '/favicon.ico', '/about'][i % 4]} HTTP/1.1" ${i === 2 ? 404 : 200} ${600 + i * 13} "-" ${ua}`);
    }
    lines.unshift(`${stamp(9e6)} [notice] ${domain}: ready to handle connections`);
  }
  return (s.logs[found.site?.containerId ?? found.db?.containerId ?? ''] ?? []).concat(lines).join('\n') + '\n';
}

function inspect(s: DemoState, found: { site?: DemoSite; db?: DemoDb }) {
  const site = found.site, db = found.db;
  const info: any = site ? siteInfo(s, site) : dbInfo(db!);
  const running = site ? site.running : db!.running;
  const started = site ? site.startedAt : db!.startedAt;
  const port = site ? site.hostPort : db!.port;
  const internal = site ? site.internalPort : 5432;
  return {
    container: {
      created: iso(site ? site.createdAt : db!.createdAt), id: info.id, image: info.image, labels: info.labels, name: info.name,
      env: site ? [`TERM=xterm`, `LANG=C.UTF-8`, `DOCKLITE_DOMAIN=${site.domain}`] : [`POSTGRES_DB=${db!.name}`, `POSTGRES_USER=${db!.username}`, 'PGDATA=/var/lib/postgresql/data'],
      mounts: [{ Type: 'bind', Source: site ? `/var/www/sites/${username(s, site.userId)}/${site.domain}` : `/var/lib/docklite/db/${db!.name}`, Destination: site ? (site.template === 'php' ? '/app' : site.template === 'node' ? '/app' : '/usr/share/nginx/html') : '/var/lib/postgresql/data', Mode: 'rw', RW: true, Propagation: 'rprivate' }],
      networkSettings: { gateway: '', ipAddress: '', networks: { docklite_network: { Gateway: '172.21.0.1', IPAddress: `172.21.0.${(site?.id ?? 20 + db!.id) + 2}`, IPPrefixLen: 16 } }, ports: { [`${internal}/tcp`]: running ? [{ HostIp: '127.0.0.1', HostPort: String(port) }] : null } },
      resources: { cpuQuota: 0, cpuShares: 0, memory: 0, memorySwap: 0 }, restartPolicy: { MaximumRetryCount: 0, Name: 'unless-stopped' },
      state: { Status: running ? 'running' : 'exited', Running: running, Paused: false, Restarting: false, OOMKilled: false, Dead: false, Pid: running ? 4100 + (site?.id ?? 0) : 0, ExitCode: 0, Error: '', StartedAt: iso(started), FinishedAt: running ? '0001-01-01T00:00:00Z' : iso(site?.stoppedAt ?? Date.now()) },
    },
  };
}

// ── A tiny read-only SQL engine for the database editor ────────────────────────────────────────
function runSql(db: DemoDb, sqlRaw: string): any {
  const sql = sqlRaw.trim().replace(/;+\s*$/, '');
  const select = sql.match(/^select\s+(.+?)\s+from\s+"?([a-z_][a-z0-9_]*)"?(?:\s+where\s+"?([a-z_][a-z0-9_]*)"?\s*(=|ilike|like)\s*'([^']*)')?(?:\s+limit\s+(\d+))?$/i);
  if (select) {
    const [, colsRaw, tableName, whereCol, op, whereVal, limit] = select;
    const table = db.tables.find((t) => t.name === tableName);
    if (!table) return { status: 400, body: { error: `relation "${tableName}" does not exist` } };
    let rows = table.rows;
    if (whereCol) {
      rows = rows.filter((r) => {
        const v = String(r[whereCol] ?? '');
        return op === '=' ? v === whereVal : new RegExp('^' + whereVal.replace(/[.*+?^${}()|[\]\\]/g, '\\$&').replace(/%/g, '.*') + '$', 'i').test(v);
      });
    }
    if (/^count\(\*\)$/i.test(colsRaw.trim())) return ok({ type: 'select', rows: [{ count: rows.length }], columns: ['count'] });
    const wanted = colsRaw.trim() === '*' ? null : colsRaw.split(',').map((c) => c.trim().replace(/"/g, ''));
    let out = rows.map((r) => (wanted ? Object.fromEntries(wanted.map((c) => [c, r[c]])) : { ...r }));
    if (limit) out = out.slice(0, Number(limit));
    return ok({ type: 'select', rows: out, columns: out.length ? Object.keys(out[0]) : wanted ?? table.columns.map((c) => c.name) });
  }
  if (/^select\s+version\(\)$/i.test(sql)) return ok({ type: 'select', rows: [{ version: 'PostgreSQL 16.4 (demo)' }], columns: ['version'] });
  if (/^(insert|update|delete|create|alter|drop|truncate)\b/i.test(sql)) {
    return ok({ type: 'command', output: 'OK (demo: this was not run, and nothing was changed)' });
  }
  return ok({ type: 'command', output: 'Demo mode understands simple queries such as: SELECT * FROM <table> LIMIT 10' });
}

// ── Route table ──────────────────────────────────────────────────────────────────────────────────
type Handler = (r: DemoRequest, m: RegExpMatchArray, s: DemoState) => DemoResponse | Promise<DemoResponse>;
const routes: [string, RegExp, Handler][] = [];
const on = (method: string, path: RegExp, h: Handler) => routes.push([method, path, h]);
const ID = '([^/]+)';

// auth
on('GET', /^\/api\/auth\/me$/, () => ok({ user: { isAdmin: true, role: 'super_admin', userId: 1, username: 'demo' } }));
on('POST', /^\/api\/auth\/(login|logout)$/, () => ok());

// containers
const folderOf = (s: DemoState, containers: any[]) => ({
  id: 1, user_id: 1, name: 'Default', parent_folder_id: null, depth: 0, position: 0, created_at: iso(Date.now() - 60 * T.DAY), children: [], containers,
});
on('GET', /^\/api\/containers$/, (_r, _m, s) => { const list = allContainers(s); return ok({ folders: [folderOf(s, list)], totalContainers: list.length }); });
on('GET', /^\/api\/containers\/all$/, (_r, _m, s) => ok({ containers: allContainers(s).map((c) => ({ ...c, tracked: true, hasManifest: true })) }));
on('POST', /^\/api\/containers$/, (r, _m, s) => {
  const { domain, template_type, include_www, port, user_id } = r.body ?? {};
  const d = String(domain ?? '').trim().toLowerCase();
  if (!/^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)+$/.test(d)) return err(400, 'Enter a domain like example.com');
  if (s.sites.some((x) => x.domain === d)) return err(400, `${d} already exists`);
  if (s.sites.length >= MAX_SITES) return err(400, 'The demo is limited to a few sites. Delete one first.');
  const template: DemoSite['template'] = ['static', 'php', 'node'].includes(template_type) ? template_type : 'static';
  const id = s.nextId++;
  s.sites.push({ id, domain: d, userId: Number(user_id) || 1, template, running: true, startedAt: Date.now(), stoppedAt: null, createdAt: Date.now(), includeWww: include_www !== false, internalPort: template === 'node' ? Number(port) || 3000 : 80, hostPort: 32770 + id, containerId: fakeHex(id), tracked: true });
  save();
  return { status: 200, body: { success: true, site_id: id } };
});
on('GET', new RegExp(`^/api/containers/${ID}/inspect$`), (_r, m, s) => { const f = findContainer(s, m[1]); return f ? ok(inspect(s, f)) : err(404, 'container not found'); });
on('GET', new RegExp(`^/api/containers/${ID}/stats$`), (_r, m, s) => {
  const f = findContainer(s, m[1]); if (!f) return err(404, 'container not found');
  const running = f.site ? f.site.running : f.db!.running;
  const mem = (f.db ? 62 : 24 + (f.site?.id ?? 0) * 6) * 1024 * 1024 + Math.floor(Math.random() * 2e6);
  const limit = 8 * 1024 ** 3;
  return ok({ stats: { cpuUsage: running ? +(Math.random() * 1.4).toFixed(2) : 0, memoryUsage: running ? mem : 0, memoryLimit: limit, memoryPct: running ? (mem / limit) * 100 : 0 } });
});
on('GET', new RegExp(`^/api/containers/${ID}/logs$`), (_r, m, s) => { const f = findContainer(s, m[1]); return f ? ok({ logs: logsFor(s, f) }) : err(404, 'container not found'); });
on('POST', new RegExp(`^/api/containers/${ID}/(start|stop|restart)$`), (_r, m, s) => {
  const f = findContainer(s, m[1]); if (!f) return err(404, 'container not found');
  const t = f.site ?? f.db!;
  if (m[2] === 'stop') { t.running = false; if (f.site) f.site.stoppedAt = Date.now(); } else { t.running = true; t.startedAt = Date.now(); if (f.site) f.site.stoppedAt = null; }
  s.logs[t.containerId] = [...(s.logs[t.containerId] ?? []), `${new Date().toISOString().replace('T', ' ').slice(0, 19)} container ${m[2]}ed (demo)`];
  save(); return ok();
});
on('DELETE', new RegExp(`^/api/containers/${ID}/delete$`), (_r, m, s) => {
  const f = findContainer(s, m[1]); if (!f) return err(404, 'container not found');
  if (f.site) s.sites = s.sites.filter((x) => x !== f.site); else s.dbs = s.dbs.filter((x) => x !== f.db);
  save(); return ok();
});
on('POST', new RegExp(`^/api/containers/${ID}/(assign|transfer)$`), (r, m, s) => {
  const f = findContainer(s, m[1]); const uid = Number(r.body?.user_id ?? r.body?.new_user_id);
  if (!f?.site || !s.users.some((u) => u.id === uid)) return err(400, 'Choose a user');
  f.site.userId = uid; save(); return ok({ success: true, new_user: username(s, uid) });
});
on('POST', new RegExp(`^/api/containers/${ID}/(track|untrack|claim|write-manifest)$`), () => ok());
on('POST', /^\/api\/containers\/(scan|onboard)$/, () => ok({ sites: [] }));
on('GET', /^\/api\/containers\/scan$/, () => ok({ sites: [] }));
on('GET', /^\/api\/ports\/suggest$/, (_r, _m, s) => ok({ port: 3000 + s.sites.length }));

// folders (one fixed "Default" folder)
on('GET', /^\/api\/folders$/, () => ok({ folders: [{ id: 1, user_id: 1, name: 'Default', parent_folder_id: null, depth: 0, position: 0, created_at: iso(Date.now() - 60 * T.DAY) }] }));
on('POST', /^\/api\/folders/, () => err(400, 'Folders are read-only in the demo'));
on('PUT', /^\/api\/folders/, () => err(400, 'Folders are read-only in the demo'));
on('DELETE', /^\/api\/folders/, () => err(400, 'Folders are read-only in the demo'));

// databases
const sizeOf = (db: DemoDb) => db.tables.reduce((n, t) => n + t.rows.length, 0) * 8192 + 262144;
const category = (n: number) => (n < 100_000 ? 'empty' : n < 5_000_000 ? 'small' : n < 100_000_000 ? 'medium' : 'large');
on('GET', /^\/api\/databases$/, (_r, _m, s) => ok({ databases: s.dbs.map((d) => ({ id: d.containerId, name: d.name, type: '', port: d.port, username: d.username, password: d.password, status: dockerStatus(d.running, d.startedAt) })) }));
on('GET', /^\/api\/databases\/stats$/, (_r, _m, s) => ok({
  databases: s.dbs.map((d) => ({ container_id: d.containerId, created_at: iso(d.createdAt), id: d.id, name: d.name, postgres_port: d.port, size: sizeOf(d), sizeCategory: category(sizeOf(d)), username: d.username })),
  dockliteDb: { path: '/opt/docklite/data/docklite.db', size: 184320, tables: 15 },
}));
on('POST', /^\/api\/databases$/, (r, _m, s) => {
  const name = String(r.body?.name ?? '').trim().replace(/[^a-zA-Z0-9_]/g, '_').toLowerCase();
  if (!name) return err(400, 'Give the database a name');
  if (s.dbs.some((d) => d.name === name)) return err(400, `${name} already exists`);
  if (s.dbs.length >= 12) return err(400, 'The demo is limited to a few databases. Delete one first.');
  if ((s.nextDbId ?? 4) > 60) return err(400, 'The demo has made enough databases. Press Reset to start fresh.');
  const id = (s.nextDbId = (s.nextDbId ?? 4) + 1) - 1;
  const db: DemoDb = { id, containerId: fakeHex(100 + id), name, username: r.body?.username || `${name}_user`, password: r.body?.password || fakeHex(id, 20), port: 32780 + id, running: true, startedAt: Date.now(), createdAt: Date.now(), tables: [] };
  s.dbs.push(db); save();
  return { status: 201, body: { database: { id, name, port: db.port, username: db.username, password: db.password } } };
});
const dbById = (s: DemoState, id: string) => s.dbs.find((d) => String(d.id) === id);
on('GET', new RegExp(`^/api/databases/${ID}$`), (_r, m, s) => { const d = dbById(s, m[1]); return d ? ok({ database: { id: d.id, name: d.name, type: 'postgres', container_id: d.containerId, postgres_port: d.port, created_at: iso(d.createdAt) } }) : err(400, 'invalid database id'); });
on('DELETE', new RegExp(`^/api/databases/${ID}$`), (_r, m, s) => { const d = dbById(s, m[1]); if (!d) return err(404, 'database not found'); s.dbs = s.dbs.filter((x) => x !== d); save(); return ok(); });
on('PUT', new RegExp(`^/api/databases/${ID}$`), () => ok({ success: true, message: 'Database credentials updated (demo)' }));
on('POST', new RegExp(`^/api/databases/${ID}/schema$`), (_r, m, s) => { const d = dbById(s, m[1]); return d ? ok({ tables: d.tables.map((t) => ({ name: t.name, columns: t.columns })) }) : err(404, 'database not found'); });
on('POST', new RegExp(`^/api/databases/${ID}/table$`), (r, m, s) => {
  const d = dbById(s, m[1]); const t = d?.tables.find((x) => x.name === r.body?.table);
  return t ? ok({ columns: t.columns.map(({ name, type, nullable }) => ({ name, type, nullable })), rows: t.rows }) : err(404, 'table not found');
});
on('POST', new RegExp(`^/api/databases/${ID}/query$`), (r, m, s) => { const d = dbById(s, m[1]); return d ? runSql(d, String(r.body?.sql ?? '')) : err(404, 'database not found'); });
on('POST', new RegExp(`^/api/databases/${ID}/update-rows$`), (r, m, s) => {
  const d = dbById(s, m[1]); const t = d?.tables.find((x) => x.name === r.body?.table);
  if (!t) return err(404, 'table not found');
  for (const change of (r.body?.rows ?? []) as Record<string, unknown>[]) {
    const row = t.rows.find((x) => String(x.id) === String(change.id));
    if (row) Object.assign(row, change);
  }
  save(); return ok();
});
on('POST', new RegExp(`^/api/databases/${ID}/download$`), (_r, m, s) => {
  const d = dbById(s, m[1]); if (!d) return err(404, 'database not found');
  const dump = `-- DockLite demo dump of ${d.name}\n` + d.tables.map((t) => `-- table ${t.name}: ${t.rows.length} rows\n`).join('');
  return { status: 200, body: dump };
});
on('GET', /^\/api\/db$/, (_r, _m, s) => {
  const table = (name: string, schema: [string, string, number][], data: any[]) => ({ name, count: data.length, data, schema: schema.map(([n, type, pk], cid) => ({ cid, name: n, type, notnull: pk, dflt_value: null, pk })) });
  return ok({ dbInfo: [
    table('users', [['id', 'INTEGER', 1], ['username', 'TEXT', 0], ['role', 'TEXT', 0], ['created_at', 'DATETIME', 0]], s.users.map((u) => ({ id: u.id, username: u.username, role: u.role, created_at: iso(u.createdAt) }))),
    table('sites', [['id', 'INTEGER', 1], ['domain', 'TEXT', 0], ['user_id', 'INTEGER', 0], ['template_type', 'TEXT', 0]], s.sites.map((x) => ({ id: x.id, domain: x.domain, user_id: x.userId, template_type: x.template }))),
    table('databases', [['id', 'INTEGER', 1], ['name', 'TEXT', 0], ['type', 'TEXT', 0]], s.dbs.map((x) => ({ id: x.id, name: x.name, type: 'postgres' }))),
    table('backups', [['id', 'INTEGER', 1], ['target_type', 'TEXT', 0], ['status', 'TEXT', 0]], s.backups.map((x) => ({ id: x.id, target_type: x.targetType, status: x.status }))),
  ] });
});
on('POST', /^\/api\/db\/cleanup$/, () => ok());

// users
const userRow = (s: DemoState, u: DemoState['users'][number]) => ({
  id: u.id, username: u.username, password_hash: '', is_admin: u.role === 'user' ? 0 : 1, role: u.role, is_super_admin: u.role === 'super_admin' ? 1 : 0,
  managed_by: { Int64: u.managedBy ?? 0, Valid: u.managedBy !== null }, created_at: iso(u.createdAt),
});
on('GET', /^\/api\/users$/, (_r, _m, s) => ok({ users: s.users.map((u) => userRow(s, u)) }));
on('POST', /^\/api\/users$/, (r, _m, s) => {
  const name = String(r.body?.username ?? '').trim();
  if (!/^[a-zA-Z0-9_-]{2,32}$/.test(name)) return err(400, 'Usernames are 2 to 32 letters, numbers, - or _');
  if (s.users.some((u) => u.username === name)) return err(400, 'Username already exists');
  if (s.users.length >= 12) return err(400, 'The demo is limited to a few users');
  const id = s.nextId++;
  s.users.push({ id, username: name, role: r.body?.isAdmin ? 'admin' : 'user', managedBy: 1, createdAt: Date.now() }); save();
  return { status: 201, body: { user: { id, username: name, isAdmin: !!r.body?.isAdmin } } };
});
on('DELETE', /^\/api\/users$/, (r, _m, s) => {
  const id = Number(r.query.get('id')); const u = s.users.find((x) => x.id === id);
  if (!u) return err(404, 'User not found'); if (u.id === 1) return err(400, 'You cannot delete your own account');
  s.sites.forEach((x) => { if (x.userId === id) x.userId = 1; }); s.users = s.users.filter((x) => x !== u); save(); return ok();
});
on('POST', /^\/api\/users\/password$/, () => ok());

// backups
const WEEKDAYS: Record<string, string> = { sun: 'Sunday', mon: 'Monday', tue: 'Tuesday', wed: 'Wednesday', thu: 'Thursday', fri: 'Friday', sat: 'Saturday' };
// The schedule text, e.g. "daily@03:00;keep=7", in plain words (the real agent does the same in Go).
function describeSchedule(freq: string): string {
  const [main, opts = ''] = freq.split(';');
  const keep = /keep=(\d+)/.exec(opts);
  const parts = main.split('@');
  let when: string;
  if (parts[0] === 'daily') when = parts[1] ? `every day at ${parts[1]}` : 'every day';
  else if (parts[0] === 'weekly') when = parts.length === 3 ? `every ${WEEKDAYS[parts[1]] ?? parts[1]} at ${parts[2]}` : 'every week';
  else if (parts[0] === 'hourly') when = 'every hour';
  else { const n = Number(parts[0].replace(/^every-/, '').replace(/-hours$/, '')); when = n > 0 ? `every ${n} hours` : freq; }
  return keep && Number(keep[1]) > 0 ? `${when}, keeping the last ${keep[1]}` : when;
}
const keepOf = (freq: string) => Number(/keep=(\d+)/.exec(freq)?.[1] ?? 0);
const backupRow = (b: DemoState['backups'][number]) => ({ id: b.id, job_id: b.jobId, destination_id: 1, target_type: b.targetType, target_id: b.targetId, backup_path: `/var/backups/docklite/${b.path}`, size_bytes: b.sizeBytes, status: b.status, error_message: null, created_at: iso(b.createdAt) });
const jobRow = (j: DemoState['jobs'][number]) => ({
  id: j.id, destination_id: 1, target_type: j.targetType, target_id: ['all-sites', 'all-databases'].includes(j.targetType) ? null : j.targetId,
  frequency: j.schedule, retention_days: 0, enabled: j.enabled ? 1 : 0, last_run_at: iso(Date.now() - 9 * T.HOUR), next_run_at: iso(Date.now() + 15 * T.HOUR), created_at: iso(j.createdAt),
  schedule_text: describeSchedule(j.schedule), schedule_valid: true, keep_last: keepOf(j.schedule),
});
const stamp = () => new Date().toISOString().replace(/[-:T]/g, '').slice(0, 8) + '-' + new Date().toISOString().replace(/[-:T]/g, '').slice(8, 14);
const targetName = (s: DemoState, targetType: string, targetId: number) =>
  targetType === 'site' ? s.sites.find((x) => x.id === targetId)?.domain ?? 'site' : s.dbs.find((x) => x.id === targetId)?.name ?? 'database';
const addBackup = (s: DemoState, targetType: 'site' | 'database', targetId: number, jobId: number | null, keep = 0) => {
  const name = targetName(s, targetType, targetId);
  const id = s.nextId++;
  const b = { id, jobId, targetType, targetId, path: targetType === 'site' ? `sites/site-${name}-${stamp()}.tar.gz` : `databases/db-${name}-${stamp()}.dump.gz`, sizeBytes: 150_000 + Math.floor(Math.random() * 4_000_000), status: 'success', createdAt: Date.now() };
  s.backups.unshift(b);
  if (keep > 0) { // "keep the newest N" of this site/database
    const same = s.backups.filter((x) => x.targetType === targetType && x.targetId === targetId && x.status === 'success');
    const drop = new Set(same.slice(keep).map((x) => x.id));
    s.backups = s.backups.filter((x) => !drop.has(x.id));
  }
  save(); return b;
};
const siteBackupFiles = (s: DemoState) => s.backups.filter((b) => b.status === 'success' || b.status === 'completed').map((b) => ({ name: b.path, size: b.sizeBytes, modified_at: iso(b.createdAt) }));
const fileCount = (sizeBytes: number) => 24 + (sizeBytes % 260);
on('GET', /^\/api\/backups$/, (_r, _m, s) => ok({ backups: s.backups.map(backupRow) }));
on('GET', /^\/api\/backups\/local$/, (_r, _m, s) => ok({ path: '/var/backups/docklite', files: siteBackupFiles(s) }));
on('DELETE', /^\/api\/backups\/local$/, (r, _m, s) => { s.backups = s.backups.filter((b) => b.path !== r.query.get('file')); save(); return ok(); });
on('DELETE', /^\/api\/backups$/, (r, _m, s) => { s.backups = s.backups.filter((b) => String(b.id) !== r.query.get('id')); save(); return ok(); });
on('GET', /^\/api\/backups\/inspect$/, (r, _m, s) => {
  const b = s.backups.find((x) => x.path === r.query.get('file') && x.targetType === 'site');
  if (!b) return err(400, "that isn't a site backup file");
  return ok({ domain: targetName(s, 'site', b.targetId), files: fileCount(b.sizeBytes), bytes: b.sizeBytes * 2, verified: true, skipped: [], created_at: iso(b.createdAt) });
});
on('POST', /^\/api\/backups\/restore$/, (r, _m, s) => {
  const b = s.backups.find((x) => x.path === r.body?.file && x.targetType === 'site');
  if (!b) return err(422, "that isn't a site backup file");
  const domain = targetName(s, 'site', b.targetId);
  const safety = `safety/before-restore-${domain}-${stamp()}.tar.gz`;
  s.backups.unshift({ id: s.nextId++, jobId: null, targetType: 'site', targetId: b.targetId, path: safety, sizeBytes: Math.round(b.sizeBytes * 0.9), status: 'success', createdAt: Date.now() });
  save();
  return ok({ domain, files: fileCount(b.sizeBytes), bytes: b.sizeBytes * 2, safety_backup: `/var/backups/docklite/${safety}`, skipped: [], message: `Restored ${domain}. What was there before was saved first, so this can be undone.` });
});
on('POST', /^\/api\/backups\/export$/, (r, _m, s) => {
  const domain = String(r.body?.domain ?? '').toLowerCase();
  const targetType = r.body?.target_type === 'database' ? 'database' : 'site';
  const targetId = domain ? s.sites.find((x) => x.domain === domain)?.id ?? 0 : Number(r.body?.target_id);
  if (!targetId) return err(404, `there is no site called ${domain}`);
  const b = addBackup(s, targetType, targetId, null);
  return ok({ success: true, backup_id: b.id });
});
on('GET', /^\/api\/backups\/jobs$/, (_r, _m, s) => ok({ jobs: s.jobs.map(jobRow) }));
const VALID_FREQ = /^(hourly|daily(@([01]\d|2[0-3]):[0-5]\d)?|weekly(@(sun|mon|tue|wed|thu|fri|sat)@([01]\d|2[0-3]):[0-5]\d)?|every-3-days|monthly|\d+|every-\d+-hours)(;keep=\d+)?$/;
on('POST', /^\/api\/backups\/jobs$/, (r, _m, s) => {
  const freq = String(r.body?.frequency ?? '');
  if (!VALID_FREQ.test(freq)) return err(400, `unrecognised schedule "${freq}"`);
  const id = s.nextId++; const t = r.body?.target_type;
  const targetType = ['database', 'all-sites', 'all-databases'].includes(t) ? t : 'site';
  s.jobs.push({ id, name: 'Scheduled backup', targetType, targetId: Number(r.body?.target_id) || 1, schedule: freq, enabled: r.body?.enabled !== 0, createdAt: Date.now() });
  save(); return ok({ id, message: 'Backup job created successfully' });
});
on('PUT', /^\/api\/backups\/jobs$/, (r, _m, s) => {
  const j = s.jobs.find((x) => x.id === Number(r.body?.id)); if (!j) return err(404, 'schedule not found');
  if (r.body?.frequency !== undefined) { if (!VALID_FREQ.test(String(r.body.frequency))) return err(400, `unrecognised schedule "${r.body.frequency}"`); j.schedule = String(r.body.frequency); }
  if (r.body?.enabled !== undefined) j.enabled = !!r.body.enabled;
  if (r.body?.target_type) j.targetType = r.body.target_type;
  if (r.body?.target_id !== undefined && r.body.target_id !== null) j.targetId = Number(r.body.target_id);
  save(); return ok();
});
on('DELETE', /^\/api\/backups\/jobs$/, (r, _m, s) => { s.jobs = s.jobs.filter((j) => String(j.id) !== r.query.get('id')); save(); return ok(); });
on('POST', /^\/api\/backups\/trigger$/, (r, _m, s) => {
  const j = s.jobs.find((x) => x.id === Number(r.body?.job_id)); if (!j) return err(404, 'schedule not found');
  const keep = keepOf(j.schedule);
  if (j.targetType === 'all-sites') s.sites.forEach((x) => addBackup(s, 'site', x.id, j.id, keep));
  else if (j.targetType === 'all-databases') s.dbs.forEach((x) => addBackup(s, 'database', x.id, j.id, keep));
  else addBackup(s, j.targetType as 'site' | 'database', j.targetId, j.id, keep);
  return ok({ message: 'Backup job triggered successfully' });
});
// Automatic HTTPS (the real agent waits for DNS, then asks Let's Encrypt); the demo has nothing pending.
on('GET', /^\/api\/ssl\/auto$/, () => ok({ jobs: [], email: '' }));
on('POST', /^\/api\/ssl\/auto$/, () => ok({ started: [], jobs: [] }));
on('GET', /^\/api\/backups\/destinations$/, () => ok({ destinations: [{ id: 1, name: 'Local Server', type: 'local', config: '{"path":"/var/backups/docklite"}', enabled: 1, created_at: iso(Date.now() - 60 * T.DAY) }] }));

// SSL + nginx
const certStatus = (s: DemoState, domain: string) => {
  const c = s.certs[domain]; if (!c) return { domain, domains: null, hasSSL: false, expiryDate: null, daysUntilExpiry: null, status: 'none' };
  const days = Math.floor((c.expiresAt - Date.now()) / T.DAY);
  return { domain, domains: [domain, `www.${domain}`], hasSSL: true, expiryDate: iso(c.expiresAt), daysUntilExpiry: days, status: days < 14 ? 'expiring' : 'valid' };
};
on('GET', /^\/api\/ssl\/status$/, (_r, _m, s) => {
  const sites = s.sites.map((x) => certStatus(s, x.domain));
  return ok({ allCerts: null, meta: { certCount: Object.keys(s.certs).length, managedCount: sites.length, provider: 'certbot' }, sites });
});
on('POST', /^\/api\/ssl\/(issue|renew)$/, (r, _m, s) => {
  const d = String(r.body?.domain ?? ''); if (!s.sites.some((x) => x.domain === d)) return err(404, 'site not found');
  s.certs[d] = { issuedAt: Date.now(), expiresAt: Date.now() + 90 * T.DAY }; save();
  return ok({ success: true, message: 'Certificate issued (demo: no real certificate was requested)' });
});
on('POST', /^\/api\/ssl\/delete$/, (r, _m, s) => { delete s.certs[String(r.body?.domain ?? '')]; save(); return ok(); });
on('GET', /^\/api\/nginx\/sites$/, (_r, _m, s) => ok({ sites: s.sites.map((x) => ({ domain: x.domain, templateType: x.template, enabled: true, hasConfig: true })) }));
const nginxConf = (s: DemoState, domain: string) => {
  const site = s.sites.find((x) => x.domain === domain);
  const saved = s.nginx[domain]; if (saved) return saved.config;
  const port = site?.hostPort ?? 8080;
  return `# Managed by DockLite (demo)\nserver {\n    listen 80;\n    server_name ${domain} www.${domain};\n\n    location / {\n        proxy_pass http://127.0.0.1:${port};\n        proxy_set_header Host $host;\n        proxy_set_header X-Real-IP $remote_addr;\n    }\n}\n`;
};
on('GET', new RegExp(`^/api/nginx/sites/${ID}$`), (_r, m, s) => { const d = decodeURIComponent(m[1]); return ok({ domain: d, config: nginxConf(s, d), content: nginxConf(s, d), enabled: true }); });
on('PUT', new RegExp(`^/api/nginx/sites/${ID}$`), (r, m, s) => { s.nginx[decodeURIComponent(m[1])] = { enabled: true, config: String(r.body?.config ?? r.body?.content ?? '') }; save(); return ok({ success: true }); });
on('POST', /^\/api\/nginx\/(test|reload)$/, () => ok({ success: true, output: 'nginx: configuration file test is successful (demo)' }));

// settings
on('POST', /^\/api\/system\/check-folders$/, () => ok({ success: true, checked: 3, created: 0 }));
on('GET', /^\/api\/system\/shell-access$/, (_r, _m, s) => ok({ members: s.shellUsers }));
on('POST', /^\/api\/system\/shell-access$/, (r, _m, s) => {
  const u = String(r.body?.username ?? '').trim(); if (!u) return err(400, 'Enter a username');
  s.shellUsers = r.body?.action === 'revoke' ? s.shellUsers.filter((x) => x !== u) : Array.from(new Set([...s.shellUsers, u])); save(); return ok({ success: true });
});
on('GET', /^\/api\/system\/update\/status$/, () => ok({ updateAvailable: false, currentVersion: process.env.NEXT_PUBLIC_DOCKLITE_VERSION ?? '', latestVersion: process.env.NEXT_PUBLIC_DOCKLITE_VERSION ?? '', running: false }));
on('POST', /^\/api\/system\/update\/run$/, () => err(403, 'Updates are disabled in the demo'));

export async function handleDemoRequest(req: DemoRequest): Promise<DemoResponse | null> {
  const s = getState();
  for (const [method, re, handler] of routes) {
    if (method !== req.method) continue;
    const m = req.path.match(re);
    if (m) return handler(req, m, s);
  }
  const files = filesApi(req, s);
  if (files) return files;
  const host = hostApi(req, s);
  if (host) return host;
  if (typeof console !== 'undefined') console.warn('[demo] not implemented:', req.method, req.path);
  return err(400, 'This part of DockLite is not available in the demo');
}
