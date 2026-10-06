// A pretend shell for the demo terminal. It speaks just enough of the WebSocket interface for
// XtermDrawer, echoes what you type, and answers a handful of commands. Nothing real runs.
import { getState } from './state';

type Listener = (ev: any) => void;

const OS_RELEASE = 'NAME="Alpine Linux"\nID=alpine\nVERSION_ID=3.20.3\nPRETTY_NAME="Alpine Linux v3.20"\nHOME_URL="https://alpinelinux.org/"\n';

function fsFor(kind: string, domain: string): Record<string, string | string[]> {
  const root = kind === 'php' ? '/app' : kind === 'node' ? '/app' : '/usr/share/nginx/html';
  const files: Record<string, string | string[]> = {
    '/': ['bin', 'dev', 'etc', 'home', 'lib', 'proc', 'root', 'tmp', 'usr', 'var', root.split('/')[1]].filter((v, i, a) => a.indexOf(v) === i).sort(),
    '/etc': ['hostname', 'os-release', 'passwd', 'nginx'], '/etc/os-release': OS_RELEASE, '/etc/hostname': 'demo-container\n',
    '/etc/passwd': 'root:x:0:0:root:/root:/bin/sh\nnobody:x:65534:65534:nobody:/:/sbin/nologin\n',
  };
  if (kind === 'static') {
    files['/usr'] = ['bin', 'lib', 'share']; files['/usr/share'] = ['nginx']; files['/usr/share/nginx'] = ['html']; files[root] = ['index.html', '50x.html'];
    files[`${root}/index.html`] = `<!doctype html>\n<title>${domain}</title>\n<h1>${domain}</h1>\n`;
    files[`${root}/50x.html`] = '<h1>Something went wrong</h1>\n';
  } else if (kind === 'php') {
    files[root] = ['index.php', 'composer.json']; files[`${root}/index.php`] = `<?php echo "Hello from ${domain}";\n`; files[`${root}/composer.json`] = '{ "name": "demo/shop" }\n';
  } else if (kind === 'node') {
    files[root] = ['index.js', 'package.json', 'node_modules']; files[`${root}/index.js`] = "require('http').createServer((q, r) => r.end('ok')).listen(3000);\n";
    files[`${root}/package.json`] = '{ "name": "api", "main": "index.js" }\n';
  } else {
    files['/var'] = ['lib', 'log']; files['/var/lib'] = ['postgresql']; files['/var/lib/postgresql'] = ['data']; files['/var/lib/postgresql/data'] = ['PG_VERSION', 'base', 'global'];
  }
  return files;
}

export class DemoSocket {
  static readonly CONNECTING = 0; static readonly OPEN = 1; static readonly CLOSING = 2; static readonly CLOSED = 3;
  readonly CONNECTING = 0; readonly OPEN = 1; readonly CLOSING = 2; readonly CLOSED = 3;
  readyState = 0; binaryType = 'arraybuffer'; url: string; bufferedAmount = 0; protocol = ''; extensions = '';
  private listeners: Record<string, Listener[]> = {};
  private line = ''; private cwd = '/'; private fs: Record<string, string | string[]>; private kind: string; private domain: string; private cname: string; private started: number;
  private history: string[] = [];

  constructor(url: string) {
    this.url = url;
    const id = url.match(/\/containers\/([^/]+)\/terminal/)?.[1] ?? '';
    const s = getState();
    const site = s.sites.find((x) => x.containerId === id); const db = s.dbs.find((x) => x.containerId === id);
    this.kind = site ? site.template : 'postgres'; this.domain = site?.domain ?? db?.name ?? 'demo';
    this.cname = site ? `docklite-site-${site.domain.replace(/\./g, '-')}` : `docklite-db-${(db?.name ?? 'demo').replace(/_/g, '-')}`;
    this.started = site?.startedAt ?? db?.startedAt ?? Date.now();
    this.fs = fsFor(this.kind, this.domain);
    setTimeout(() => {
      this.readyState = 1; this.emit('open', {});
      this.out(`DockLite demo shell. Commands here are simulated; nothing runs.\r\nTry: help, ls, cat /etc/os-release, ps, df\r\n\r\n${this.prompt()}`);
    }, 350);
  }

  addEventListener(type: string, fn: Listener) { (this.listeners[type] ||= []).push(fn); }
  removeEventListener(type: string, fn: Listener) { this.listeners[type] = (this.listeners[type] || []).filter((f) => f !== fn); }
  set onopen(fn: Listener) { this.addEventListener('open', fn); } set onmessage(fn: Listener) { this.addEventListener('message', fn); }
  set onclose(fn: Listener) { this.addEventListener('close', fn); } set onerror(fn: Listener) { this.addEventListener('error', fn); }
  private emit(type: string, ev: any) { (this.listeners[type] || []).slice().forEach((f) => f(ev)); }
  private out(text: string) { this.emit('message', { data: text }); }
  private prompt() { return `${this.cwd === '/' ? '/' : this.cwd} # `; }
  close() { if (this.readyState === 3) return; this.readyState = 3; this.emit('close', { code: 1000, reason: '', wasClean: true }); }

  send(data: string | ArrayBuffer) {
    if (this.readyState !== 1 || typeof data !== 'string') return;
    if (data.startsWith('{')) return; // resize message
    for (const ch of data) this.key(ch);
  }

  private key(ch: string) {
    if (ch === '\r') { this.out('\r\n'); const l = this.line.trim(); this.line = ''; if (l) { this.history.push(l); this.run(l); } else this.out(this.prompt()); return; }
    if (ch === '\x7f' || ch === '\b') { if (this.line) { this.line = this.line.slice(0, -1); this.out('\b \b'); } return; }
    if (ch === '\x03') { this.line = ''; this.out('^C\r\n' + this.prompt()); return; }
    if (ch === '\x0c') { this.out('\x1b[2J\x1b[H' + this.prompt() + this.line); return; }
    if (ch === '\x04') { this.out('\r\nlogout\r\n'); this.close(); return; }
    if (ch === '\x1b' || ch.charCodeAt(0) < 32) return;
    this.line += ch; this.out(ch);
  }

  private abs(p: string) {
    const parts = (p.startsWith('/') ? p : `${this.cwd}/${p}`).split('/').filter(Boolean); const out: string[] = [];
    for (const x of parts) { if (x === '..') out.pop(); else if (x !== '.') out.push(x); }
    return '/' + out.join('/');
  }

  private run(cmdline: string) {
    const [cmd, ...args] = cmdline.split(/\s+/); const w: string[] = [];
    const say = (t: string) => w.push(t.replace(/\n/g, '\r\n'));
    switch (cmd) {
      case 'help': say('Available (simulated): ls cd pwd cat echo whoami id hostname uname date uptime ps df free env clear exit\n' + (this.kind === 'postgres' ? 'psql\n' : this.kind === 'node' ? 'node -v\n' : this.kind === 'php' ? 'php -v\n' : 'nginx -v\n')); break;
      case 'pwd': say(this.cwd + '\n'); break;
      case 'whoami': say('root\n'); break;
      case 'id': say('uid=0(root) gid=0(root) groups=0(root)\n'); break;
      case 'hostname': say(this.cname.slice(0, 12) + '\n'); break;
      case 'uname': say(args[0] === '-a' ? 'Linux demo-container 6.8.0 #1 SMP x86_64 Linux\n' : 'Linux\n'); break;
      case 'date': say(new Date().toUTCString() + '\n'); break;
      case 'uptime': say(` ${new Date().toISOString().slice(11, 19)} up ${Math.max(1, Math.floor((Date.now() - this.started) / 60000))} min,  load average: 0.00, 0.01, 0.00\n`); break;
      case 'echo': say(args.join(' ') + '\n'); break;
      case 'clear': w.push('\x1b[2J\x1b[H'); break;
      case 'exit': case 'logout': this.out('logout\r\n'); this.close(); return;
      case 'cd': { const t = this.abs(args[0] || '/'); if (Array.isArray(this.fs[t]) || t === '/') this.cwd = t; else say(`cd: can't cd to ${args[0]}: No such file or directory\n`); break; }
      case 'ls': {
        const t = this.abs(args.filter((a) => !a.startsWith('-'))[0] || this.cwd); const v = this.fs[t];
        if (Array.isArray(v)) say(v.join('  ') + '\n'); else if (typeof v === 'string') say(t.split('/').pop() + '\n'); else say(`ls: ${args[0]}: No such file or directory\n`); break;
      }
      case 'cat': {
        if (!args[0]) { say('cat: missing file\n'); break; }
        const v = this.fs[this.abs(args[0])]; if (typeof v === 'string') say(v); else say(`cat: ${args[0]}: ${Array.isArray(v) ? 'Is a directory' : 'No such file or directory'}\n`); break;
      }
      case 'ps': say('PID   USER     TIME  COMMAND\n    1 root      0:00  ' + (this.kind === 'node' ? 'node index.js' : this.kind === 'postgres' ? 'postgres' : 'nginx: master process') + '\n   17 root      0:00  /bin/sh\n   29 root      0:00  ps\n'); break;
      case 'df': say('Filesystem           Size      Used Available Use% Mounted on\noverlay            699.0G    120.3G    578.7G  17% /\ntmpfs               64.0M         0     64.0M   0% /dev\n'); break;
      case 'free': say('              total        used        free\nMem:       24561248      812344    18490112\nSwap:       2097148           0     2097148\n'); break;
      case 'env': say(`HOSTNAME=${this.cname.slice(0, 12)}\nHOME=/root\nTERM=xterm\nPATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\n`); break;
      case 'nginx': say('nginx version: nginx/1.27.2\n'); break;
      case 'node': say('v20.18.0\n'); break;
      case 'php': say('PHP 8.2.30 (cli) (built: demo)\n'); break;
      case 'psql': say('psql (16.4)\nIn the demo, use the Databases page to browse tables.\n'); break;
      default: say(`sh: ${cmd}: not found (this is a simulated shell; type help)\n`);
    }
    this.out(w.join('') + this.prompt());
  }
}
