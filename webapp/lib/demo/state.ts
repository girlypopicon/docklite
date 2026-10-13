// The fake world a demo visitor plays in. It lives in memory (and sessionStorage, so a page
// reload in the same tab keeps it); closing the tab throws it all away.

export interface DemoUser { id: number; username: string; role: 'super_admin' | 'admin' | 'user'; managedBy: number | null; createdAt: number }
export interface DemoSite {
  id: number; domain: string; userId: number; template: 'static' | 'php' | 'node'; running: boolean;
  startedAt: number; stoppedAt: number | null; createdAt: number; includeWww: boolean; internalPort: number; hostPort: number; containerId: string; tracked: boolean;
}
export interface DemoColumn { name: string; type: string; nullable: boolean; key: string; default: string | null }
export interface DemoTable { name: string; columns: DemoColumn[]; rows: Record<string, unknown>[] }
export interface DemoDb {
  id: number; containerId: string; name: string; username: string; password: string; port: number;
  running: boolean; startedAt: number; createdAt: number; tables: DemoTable[];
}
export interface DemoBackup { id: number; jobId: number | null; targetType: 'site' | 'database'; targetId: number; path: string; sizeBytes: number; status: string; createdAt: number }
export interface DemoJob { id: number; name: string; targetType: 'site' | 'database' | 'all-sites' | 'all-databases'; targetId: number; schedule: string; enabled: boolean; createdAt: number }
export interface DemoState {
  nextId: number; nextDbId: number; users: DemoUser[]; sites: DemoSite[]; dbs: DemoDb[]; backups: DemoBackup[]; jobs: DemoJob[];
  nginx: Record<string, { enabled: boolean; config: string }>;
  certs: Record<string, { issuedAt: number; expiresAt: number }>;
  logs: Record<string, string[]>;
  shellUsers: string[];
}

const KEY = 'docklite-demo-state-v1';
const MIN = 60_000, HOUR = 3_600_000, DAY = 86_400_000;

// A small deterministic generator so ids look like Docker's but are stable.
function rng(seed: number) {
  return () => { seed |= 0; seed = (seed + 0x6d2b79f5) | 0; let t = Math.imul(seed ^ (seed >>> 15), 1 | seed); t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t; return ((t ^ (t >>> 14)) >>> 0) / 4294967296; };
}
export function fakeHex(seed: number, len = 64): string {
  const r = rng(seed * 7919 + 13); let s = '';
  while (s.length < len) s += Math.floor(r() * 16).toString(16);
  return s;
}

function cols(...defs: [string, string, boolean?, string?][]): DemoColumn[] {
  return defs.map(([name, type, nullable, key]) => ({ name, type, nullable: !!nullable, key: key || '', default: null }));
}

function seedTables(kind: string): DemoTable[] {
  if (kind === 'shop_db') {
    const names = ['Ada Lovelace', 'Grace Hopper', 'Alan Turing', 'Katherine Johnson', 'Linus Torvalds', 'Margaret Hamilton', 'Dennis Ritchie', 'Barbara Liskov'];
    return [
      { name: 'customers', columns: cols(['id', 'integer', false, 'PRIMARY KEY'], ['name', 'text'], ['email', 'text'], ['created_at', 'timestamp without time zone']),
        rows: names.map((n, i) => ({ id: i + 1, name: n, email: `${n.split(' ')[0].toLowerCase()}@example.com`, created_at: `2026-0${(i % 9) + 1}-1${i % 9} 10:${10 + i}:00` })) },
      { name: 'products', columns: cols(['id', 'integer', false, 'PRIMARY KEY'], ['name', 'text'], ['price', 'numeric'], ['stock', 'integer']),
        rows: [['Ceramic mug', 14.5, 120], ['Linen tote bag', 22, 64], ['Beeswax candle', 9.75, 210], ['Hand-dyed scarf', 38, 18], ['Notebook (A5)', 7.5, 340]].map((p, i) => ({ id: i + 1, name: p[0], price: p[1], stock: p[2] })) },
      { name: 'orders', columns: cols(['id', 'integer', false, 'PRIMARY KEY'], ['customer_id', 'integer', false, 'FOREIGN KEY'], ['total', 'numeric'], ['status', 'text']),
        rows: Array.from({ length: 12 }, (_, i) => ({ id: i + 1, customer_id: (i % 8) + 1, total: Math.round((10 + ((i * 37) % 90)) * 100) / 100, status: ['paid', 'shipped', 'delivered', 'refunded'][i % 4] })) },
    ];
  }
  if (kind === 'blog_db') {
    return [
      { name: 'posts', columns: cols(['id', 'integer', false, 'PRIMARY KEY'], ['title', 'text'], ['published', 'boolean'], ['views', 'integer']),
        rows: ['Hello, world', 'Why I self-host', 'Notes on nginx', 'A year of side projects', 'Backups you can trust'].map((t, i) => ({ id: i + 1, title: t, published: i !== 3, views: 120 + i * 311 })) },
      { name: 'comments', columns: cols(['id', 'integer', false, 'PRIMARY KEY'], ['post_id', 'integer', false, 'FOREIGN KEY'], ['author', 'text'], ['body', 'text']),
        rows: [[1, 'sam', 'Nice first post!'], [1, 'jo', 'Welcome aboard.'], [2, 'mei', 'Same here, never going back.'], [3, 'ali', 'The proxy_pass tip saved me.']].map((c, i) => ({ id: i + 1, post_id: c[0], author: c[1], body: c[2] })) },
    ];
  }
  return [
    { name: 'page_views', columns: cols(['id', 'integer', false, 'PRIMARY KEY'], ['path', 'text'], ['country', 'text'], ['seen_at', 'timestamp without time zone']),
      rows: Array.from({ length: 15 }, (_, i) => ({ id: i + 1, path: ['/', '/pricing', '/docs', '/blog'][i % 4], country: ['US', 'DE', 'JP', 'BR', 'GB'][i % 5], seen_at: `2026-10-0${(i % 6) + 1} 1${i % 10}:0${i % 6}:00` })) },
  ];
}

export function seedState(now = Date.now()): DemoState {
  const mkSite = (id: number, domain: string, userId: number, template: DemoSite['template'], running: boolean, ageMs: number): DemoSite => ({
    id, domain, userId, template, running, startedAt: now - ageMs, stoppedAt: running ? null : now - 2 * HOUR, createdAt: now - ageMs - HOUR, includeWww: true,
    internalPort: template === 'node' ? 3000 : 80, hostPort: 32770 + id, containerId: fakeHex(id), tracked: true,
  });
  const mkDb = (id: number, name: string, username: string, ageMs: number): DemoDb => ({
    id, containerId: fakeHex(100 + id), name, username, password: fakeHex(200 + id, 24), port: 32780 + id, running: true,
    startedAt: now - ageMs, createdAt: now - ageMs - HOUR, tables: seedTables(name),
  });
  return {
    nextId: 20,
    nextDbId: 4,
    users: [
      { id: 1, username: 'demo', role: 'super_admin', managedBy: null, createdAt: now - 60 * DAY },
      { id: 2, username: 'alice', role: 'user', managedBy: 1, createdAt: now - 40 * DAY },
      { id: 3, username: 'bob', role: 'user', managedBy: 1, createdAt: now - 25 * DAY },
    ],
    sites: [
      mkSite(1, 'docs.example.org', 1, 'static', true, 18 * DAY + 3 * HOUR),
      mkSite(2, 'api.example.net', 3, 'node', true, 6 * DAY + 5 * HOUR),
      mkSite(3, 'portfolio.example.net', 3, 'static', true, 12 * DAY),
      mkSite(4, 'shop.example.com', 2, 'php', true, 3 * DAY + 2 * HOUR),
      mkSite(5, 'blog.example.com', 2, 'static', true, 27 * DAY),
      mkSite(6, 'status.example.org', 1, 'static', false, 40 * DAY),
    ],
    dbs: [mkDb(1, 'shop_db', 'shop_db_user', 3 * DAY), mkDb(2, 'blog_db', 'blog_db_user', 27 * DAY), mkDb(3, 'analytics', 'analytics_user', 9 * DAY)],
    backups: [
      { id: 1, jobId: null, targetType: 'site', targetId: 4, path: 'sites/site-shop.example.com-20261003-020000.tar.gz', sizeBytes: 4_812_345, status: 'success', createdAt: now - 3 * DAY },
      { id: 3, jobId: 1, targetType: 'site', targetId: 5, path: 'sites/site-blog.example.com-20261004-020000.tar.gz', sizeBytes: 2_104_221, status: 'success', createdAt: now - 2 * DAY },
      { id: 2, jobId: null, targetType: 'database', targetId: 1, path: 'databases/db-shop_db-20261003-020500.dump.gz', sizeBytes: 182_330, status: 'success', createdAt: now - 3 * DAY },
    ],
    jobs: [{ id: 1, name: 'Nightly shop backup', targetType: 'site', targetId: 4, schedule: 'daily@02:00;keep=7', enabled: true, createdAt: now - 20 * DAY }],
    nginx: {},
    certs: {},
    logs: {},
    shellUsers: ['alice'],
  };
}

let state: DemoState | null = null;

export function getState(): DemoState {
  if (state) return state;
  try {
    const raw = typeof sessionStorage !== 'undefined' ? sessionStorage.getItem(KEY) : null;
    if (raw) { state = JSON.parse(raw) as DemoState; return state; }
  } catch { /* storage can be unavailable; the demo just won't survive a reload */ }
  state = seedState();
  return state;
}

export function save(): void {
  try { if (state && typeof sessionStorage !== 'undefined') sessionStorage.setItem(KEY, JSON.stringify(state)); } catch { /* ignore */ }
}

export function resetState(): void {
  state = null;
  try { if (typeof sessionStorage !== 'undefined') sessionStorage.removeItem(KEY); } catch { /* ignore */ }
}

export const T = { MIN, HOUR, DAY };
