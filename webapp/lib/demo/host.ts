// Server, Network and DNS pages. Every value here is made up (reserved example.* names, documentation
// IP ranges), so the demo never reveals or touches a real machine.
import type { DemoRequest, DemoResponse } from './api';
import type { DemoState } from './state';
import { T } from './state';

const ok = (body: any = { success: true }): DemoResponse => ({ status: 200, body });
const err = (status: number, message: string): DemoResponse => ({ status, body: { error: message } });
const iso = (ms: number) => new Date(ms).toISOString();
const GB = 1024 ** 3;
const BOOT = Date.now() - (23 * T.DAY + 6 * T.HOUR);

const svc = (name: string, kind: string, extra: Record<string, unknown> = {}) => ({
  name, kind, status: 'running', detail: 'active (running)', startedAt: iso(BOOT + 5 * 60_000),
  startSupported: true, stopSupported: true, restartSupported: true, reloadSupported: true, logsSupported: true, ...extra,
});

function logLines(target: string, n = 24): string {
  const lines: string[] = [];
  const t = (i: number) => new Date(Date.now() - (n - i) * 47_000).toISOString().replace('T', ' ').slice(0, 19);
  for (let i = 0; i < n; i++) {
    if (target === 'docklite') lines.push(`${t(i)} GET /api/containers 200 ${6 + (i % 5)}ms`);
    else if (target === 'proxy') lines.push(`203.0.113.${10 + (i % 40)} - - [${t(i)}] "GET / HTTP/2.0" 200 ${900 + i * 7} "-" "Mozilla/5.0"`);
    else lines.push(`${t(i)} demo-server systemd[1]: ${['Started Daily apt download activities', 'Finished Daily Cleanup of Temporary Directories', 'Starting Docker Application Container Engine', 'docklite-agent.service: running'][i % 4]}`);
  }
  return lines.join('\n') + '\n';
}

let dnsSeeded = false;
interface Zone { id: number; domain: string; zone_id: string; account_id: { String: string; Valid: boolean }; enabled: number; last_synced_at: { String: string; Valid: boolean }; created_at: string }
interface Rec { id: number; zone_id: number; cloudflare_record_id: { String: string; Valid: boolean }; type: string; name: string; content: string; ttl: number; priority: { Int64: number; Valid: boolean }; proxied: number; created_at: string; updated_at: string }
let zones: Zone[] = []; let records: Rec[] = []; let nextDns = 100;
const ns = (v: string | null) => ({ String: v ?? '', Valid: v !== null });

function seedDns() {
  if (dnsSeeded) return; dnsSeeded = true;
  const now = Date.now();
  zones = ['example.com', 'example.net', 'example.org'].map((domain, i) => ({
    id: i + 1, domain, zone_id: `demo${i}00000000000000000000000000`.slice(0, 32), account_id: ns('demo-account'), enabled: 1, last_synced_at: ns(iso(now - 2 * T.HOUR)), created_at: iso(now - 30 * T.DAY),
  }));
  let id = 1;
  const add = (zone: number, type: string, name: string, content: string, proxied = 0, priority: number | null = null) =>
    records.push({ id: id++, zone_id: zone, cloudflare_record_id: ns(`cf${id}`), type, name, content, ttl: 1, priority: { Int64: priority ?? 0, Valid: priority !== null }, proxied, created_at: iso(now - 30 * T.DAY), updated_at: iso(now - 2 * T.HOUR) });
  add(1, 'A', 'example.com', '203.0.113.10', 1); add(1, 'CNAME', 'www.example.com', 'example.com', 1); add(1, 'A', 'shop.example.com', '203.0.113.10', 1);
  add(1, 'A', 'blog.example.com', '203.0.113.10', 1); add(1, 'MX', 'example.com', 'mail.example.com', 0, 10); add(1, 'TXT', 'example.com', 'v=spf1 include:_spf.example.com ~all');
  add(2, 'A', 'example.net', '203.0.113.10', 1); add(2, 'A', 'api.example.net', '203.0.113.10', 0); add(2, 'A', 'portfolio.example.net', '203.0.113.10', 1);
  add(3, 'A', 'docs.example.org', '203.0.113.10', 1); add(3, 'A', 'status.example.org', '203.0.113.10', 0); add(3, 'TXT', '_dmarc.example.org', 'v=DMARC1; p=none');
}

const zoneSsl: Record<number, { ssl: string; alwaysUseHttps: boolean }> = {};

export function hostApi(req: DemoRequest, s: DemoState): DemoResponse | null {
  const { method, path, query, body } = req;

  // ── Server ──
  if (method === 'GET' && path === '/api/server/stats') {
    return ok({ hostname: 'demo-server', platform: 'linux', arch: 'amd64', cpus: 8, totalMemory: 32 * GB, freeMemory: 19 * GB, uptime: (Date.now() - BOOT) / 1000, dockerVersion: '27.3.1',
      containerCount: s.sites.length + s.dbs.length, imageCount: 7, cpuUsage: +(1 + Math.random() * 4).toFixed(1), diskUsage: { free: 540 * GB, percentage: 28, total: 750 * GB, used: 210 * GB }, networkStats: { received: 41 * GB, transmitted: 38 * GB } });
  }
  if (method === 'GET' && path === '/api/server/overview') {
    return ok({ hostname: 'demo-server', os: 'Ubuntu', osVersion: '24.04.1 LTS', kernel: '6.8.0-45-generic', arch: 'x86_64', cpuCount: 8, uptime: (Date.now() - BOOT) / 1000,
      loadAvg: { one: 0.42, five: 0.37, fifteen: 0.31 }, memory: { total: 32 * GB, free: 19 * GB }, disk: { total: 750 * GB, used: 210 * GB, free: 540 * GB },
      timeSync: { status: 'synchronized', detail: 'NTP service active', timezone: 'UTC' }, clockIso: new Date().toISOString() });
  }
  if (method === 'GET' && path === '/api/server/updates') {
    return ok({ pendingUpdates: 6, securityUpdates: 2, rebootRequired: false, autoUpdates: { status: 'enabled', detail: 'unattended-upgrades is active' }, source: 'apt (demo)' });
  }
  if (method === 'GET' && path === '/api/server/services') {
    return ok({
      docker: svc('Docker', 'docker', { startSupported: false, stopSupported: false, reloadSupported: false, restartSupported: false, logsSupported: false }),
      docklite: svc('DockLite', 'docklite', { stopSupported: false, note: 'This is the program running this page. It can be restarted but not stopped from here.' }),
      dockliteSecondary: null,
      proxy: svc('nginx (web proxy)', 'proxy', { stopSupported: false, note: 'Every website goes through this. It cannot be stopped from here.' }),
      traefik: null, legacyApi: null,
    });
  }
  if (method === 'POST' && path === '/api/server/services/action') return ok({ success: true });
  if (method === 'GET' && path === '/api/server/logs') {
    return ok({ logs: logLines(query.get('target') || 'system', Math.min(Number(query.get('tail')) || 24, 60)) });
  }
  if (method === 'GET' && path === '/api/server/storage') {
    return ok({
      mounts: [{ filesystem: '/dev/nvme0n1p2', type: 'ext4', size: 750 * GB, used: 210 * GB, available: 540 * GB, usePercent: 28, mountpoint: '/' }, { filesystem: '/dev/nvme0n1p1', type: 'vfat', size: 0.5 * GB, used: 0.01 * GB, available: 0.49 * GB, usePercent: 2, mountpoint: '/boot/efi' }],
      docker: { imageCount: 7, containerCount: s.sites.length + s.dbs.length, volumeCount: 3, buildCacheCount: 12, imageSize: 2.6 * GB, containerSize: 0.3 * GB, volumeSize: 1.1 * GB, buildCacheSize: 0.8 * GB, totalSize: 4.8 * GB },
      volumes: [{ name: 'pgdata_shop', driver: 'local', mount: '/var/lib/docker/volumes/pgdata_shop/_data', size: 0.4 * GB, refCount: 1, createdAt: iso(Date.now() - 30 * T.DAY) }],
    });
  }
  if (method === 'POST' && path === '/api/server/storage/prune') return ok({ success: true });
  if (method === 'GET' && path === '/api/server/security') {
    return ok({ ssh: { status: 'running', unit: 'ssh.service', ports: ['22'] }, sudoUsers: ['demo', 'alice'], failedLogins: { count: 3, latest: iso(Date.now() - 5 * T.HOUR), source: 'journal (demo)' } });
  }
  if (method === 'GET' && path === '/api/server/ports') return ok({ agentAddr: '127.0.0.1:3000', agentPort: 3000, webUrl: 'https://demo.example.com', webPort: 443, headless: false });
  if (method === 'GET' && path === '/api/server/diagnostics') return { status: 200, body: 'DockLite demo: no diagnostics bundle is created here.\n' };

  // ── Network ──
  if (method === 'GET' && path === '/api/network/overview') {
    return ok({ hostname: 'demo-server', fqdn: 'demo-server.example.com', domain: 'example.com', searchDomains: ['example.com'], primaryIPv4: '203.0.113.10', primaryIPv6: '2001:db8::10', defaultGateway: '203.0.113.1', defaultDevice: 'eth0',
      interfaces: [
        { name: 'eth0', mac: '02:00:5e:00:53:10', mtu: 1500, state: 'UP', dhcp4: 'no', dhcp6: 'no', addresses: [{ family: 'inet', address: '203.0.113.10', prefixLen: 24, scope: 'global', label: 'eth0' }, { family: 'inet6', address: '2001:db8::10', prefixLen: 64, scope: 'global', label: '' }] },
        { name: 'docker0', mac: '02:42:ac:11:00:01', mtu: 1500, state: 'UP', dhcp4: 'no', dhcp6: 'no', addresses: [{ family: 'inet', address: '172.17.0.1', prefixLen: 16, scope: 'global', label: 'docker0' }] },
        { name: 'lo', mac: '00:00:00:00:00:00', mtu: 65536, state: 'UNKNOWN', dhcp4: 'no', dhcp6: 'no', addresses: [{ family: 'inet', address: '127.0.0.1', prefixLen: 8, scope: 'host', label: 'lo' }] },
      ],
      routes: [{ destination: 'default', gateway: '203.0.113.1', device: 'eth0', prefsrc: '', protocol: 'static' }, { destination: '172.17.0.0/16', gateway: '', device: 'docker0', prefsrc: '172.17.0.1', protocol: 'kernel' }],
      resolver: { mode: 'systemd-resolved', resolvConf: '/run/systemd/resolve/stub-resolv.conf', nameServers: ['1.1.1.1', '9.9.9.9'], searchDomains: ['example.com'] }, resolverManaged: 'systemd-resolved' });
  }
  if (method === 'GET' && path === '/api/network/firewall') {
    const sitePorts = s.sites.filter((x) => x.running).map((x) => ({ id: x.containerId, name: `docklite-site-${x.domain.replace(/\./g, '-')}`, image: 'nginx:alpine', managed: true, ports: [{ hostIp: '127.0.0.1', hostPort: x.hostPort, containerPort: x.internalPort, proto: 'tcp' }] }));
    return ok({ firewall: { provider: 'ufw', status: 'active', details: 'Status: active (demo)' },
      openPorts: [{ proto: 'tcp', address: '0.0.0.0', port: 22, process: 'sshd', public: true }, { proto: 'tcp', address: '0.0.0.0', port: 80, process: 'nginx', public: true }, { proto: 'tcp', address: '0.0.0.0', port: 443, process: 'nginx', public: true }, { proto: 'tcp', address: '127.0.0.1', port: 3000, process: 'docklite-agent', public: false }],
      httpOpen: true, httpsOpen: true, sshOpen: true, otherPorts: [], dockerExposed: sitePorts, lastUpdated: new Date().toISOString() });
  }
  if (method === 'GET' && path === '/api/network/ingress') {
    return ok({ provider: 'nginx', containerName: 'nginx (system service)', image: '', state: 'running', status: 'active', ports: [], bindings: ['0.0.0.0:80', '0.0.0.0:443'], entrypoints: ['http', 'https'], httpRedirect: 'enabled', hsts: 'enabled' });
  }
  if (method === 'GET' && path === '/api/network/public-ip') return ok({ ip: '203.0.113.10' });
  if (method === 'POST' && path === '/api/network/diagnostics') {
    return ok({ publicIp: '203.0.113.10', results: [
      { name: 'DNS resolution', target: 'example.com', status: 'ok', latencyMs: 14, detail: 'Resolved to 203.0.113.10' },
      { name: 'Internet reachability', target: '1.1.1.1', status: 'ok', latencyMs: 9, detail: 'Reply received' },
      { name: 'Docker daemon', target: 'unix:///var/run/docker.sock', status: 'ok', latencyMs: 2, detail: 'Responding' },
    ] });
  }

  // ── DNS ──
  if (path.startsWith('/api/dns/')) {
    seedDns();
    if (method === 'GET' && path === '/api/dns/config') return ok({ enabled: true, hasToken: true, accountId: 'demo-account' });
    if (path === '/api/dns/config') return ok({ message: 'Configuration saved (demo: nothing was stored)' });
    if (method === 'POST' && path === '/api/dns/cloudflare/check') return ok({ valid: true, canListZones: true, zoneCount: zones.length, canReadDns: true, canReadSettings: true });
    if (method === 'GET' && path === '/api/dns/zones') return ok({ zones });
    if (method === 'POST' && path === '/api/dns/zones/import') return ok({ imported: 0, total: zones.length });
    if (method === 'DELETE' && path === '/api/dns/zones') { const id = Number(query.get('id')); zones = zones.filter((z) => z.id !== id); records = records.filter((r) => r.zone_id !== id); return ok({ message: 'DNS zone deleted successfully' }); }
    if (method === 'POST' && path === '/api/dns/zones') { const id = nextDns++; zones.push({ id, domain: String(body?.domain ?? 'new.example.com'), zone_id: `demo${id}`.padEnd(32, '0'), account_id: ns('demo-account'), enabled: 1, last_synced_at: ns(null), created_at: iso(Date.now()) }); return ok({ id, message: 'DNS zone created successfully' }); }
    if (method === 'GET' && path === '/api/dns/records') { const z = Number(query.get('zone_id')); return ok({ records: records.filter((r) => r.zone_id === z) }); }
    if (method === 'POST' && path === '/api/dns/records') {
      const id = nextDns++; records.push({ id, zone_id: Number(body?.zone_id), cloudflare_record_id: ns(null), type: String(body?.type ?? 'A'), name: String(body?.name ?? ''), content: String(body?.content ?? ''), ttl: Number(body?.ttl) || 1, priority: { Int64: Number(body?.priority) || 0, Valid: body?.priority != null }, proxied: body?.proxied ? 1 : 0, created_at: iso(Date.now()), updated_at: iso(Date.now()) });
      return ok({ id, message: 'DNS record created successfully' });
    }
    if (method === 'PUT' && path === '/api/dns/records') { const r = records.find((x) => x.id === Number(body?.id)); if (r) Object.assign(r, { content: body?.content ?? r.content, name: body?.name ?? r.name, type: body?.type ?? r.type, proxied: body?.proxied ? 1 : 0 }); return ok({ message: 'DNS record updated successfully' }); }
    if (method === 'DELETE' && path === '/api/dns/records') { const id = Number(query.get('id')); records = records.filter((r) => r.id !== id); return ok({ message: 'DNS record deleted successfully' }); }
    if (method === 'POST' && path === '/api/dns/sync') return ok({ message: `Synced ${records.length} records from ${zones.length} zone(s) (demo: nothing was contacted)`, results: zones.map((z) => ({ zone: z.domain, records: records.filter((r) => r.zone_id === z.id).length, status: 'success' })) });
    if (method === 'GET' && path === '/api/dns/zones/ssl') { const id = Number(query.get('id')); const z = zones.find((x) => x.id === id); const v = zoneSsl[id] ?? { ssl: 'full', alwaysUseHttps: true }; return ok({ domain: z?.domain ?? '', ssl: v.ssl, alwaysUseHttps: v.alwaysUseHttps }); }
    if (method === 'PUT' && path === '/api/dns/zones/ssl') { zoneSsl[Number(body?.id)] = { ssl: body?.ssl ?? 'full', alwaysUseHttps: body?.alwaysUseHttps ?? true }; return ok({ success: true }); }
  }

  return null;
}
