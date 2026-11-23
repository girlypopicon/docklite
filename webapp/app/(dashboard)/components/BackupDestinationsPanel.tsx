'use client';

import { useCallback, useEffect, useState } from 'react';
import { CloudArrowUp, LockSimple, Plus, Trash } from '@phosphor-icons/react';
import ConfirmDeleteModal from './ConfirmDeleteModal';
import { useToast } from '@/lib/hooks/useToast';

type Dest = { id: number; name: string; type: string; enabled: number; config: string };
type Cfg = Record<string, any>;
type ServerRow = { id: number; name: string };

const TYPES: Array<{ id: string; label: string; hint: string }> = [
  { id: 's3', label: 'Bucket (R2, Backblaze B2, Amazon S3…)', hint: 'Any S3-compatible storage. Cloudflare R2 and Backblaze B2 both have a free tier.' },
  { id: 'sftp', label: 'Another server (SSH)', hint: 'Any server you can SSH into: a second DockLite server, a storage box, a spare machine.' },
  { id: 'server', label: 'A connected DockLite server', hint: 'Uses the link you already made on the Servers page. No SSH needed.' },
  { id: 'local', label: 'A folder on this server', hint: 'Another disk or mount. Add a key to lock what goes in it.' },
];

const parse = (s: string): Cfg => {
  try {
    return JSON.parse(s || '{}');
  } catch {
    return {};
  }
};

const whereOf = (d: Dest) => {
  const c = parse(d.config);
  if (d.type === 's3') return `${c.bucket}${c.prefix ? '/' + c.prefix : ''}${c.endpoint ? ' · ' + c.endpoint.replace(/^https?:\/\//, '') : ''}`;
  if (d.type === 'sftp') return `${c.user}@${c.host}${c.dir ? ':' + c.dir : ''}`;
  if (d.type === 'server') return 'a connected server';
  return c.path || 'this server’s backup folder';
};

const Field = ({ label, children, hint }: { label: string; children: React.ReactNode; hint?: string }) => (
  <label className="block">
    <span className="block text-xs font-bold mb-1" style={{ color: 'var(--text-pink)' }}>{label}</span>
    {children}
    {hint && <span className="block text-xs mt-1" style={{ color: 'var(--text-secondary)' }}>{hint}</span>}
  </label>
);

/** The Destinations tab: where backups can go, how to add one, and how to lock backups with a key. */
export default function BackupDestinationsPanel() {
  const toast = useToast();
  const [dests, setDests] = useState<Dest[]>([]);
  const [servers, setServers] = useState<ServerRow[]>([]);
  const [adding, setAdding] = useState(false);
  const [testing, setTesting] = useState<number | null>(null);
  const [results, setResults] = useState<Record<number, { ok: boolean; text: string }>>({});
  const [confirm, setConfirm] = useState<Dest | null>(null);

  const load = useCallback(async () => {
    const [d, s] = await Promise.all([fetch('/api/backups/destinations', { cache: 'no-store' }), fetch('/api/servers', { cache: 'no-store' })]);
    if (d.ok) setDests((await d.json()).destinations || []);
    if (s.ok) setServers((await s.json()).servers || []);
  }, []);
  useEffect(() => {
    void load().catch(() => {});
  }, [load]);

  const test = async (d: Dest) => {
    setTesting(d.id);
    try {
      const res = await fetch('/api/backups/destinations/test', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ id: d.id }) });
      const data = await res.json();
      setResults((r) => ({ ...r, [d.id]: { ok: !!data.ok, text: data.ok ? data.message : data.error || data.message } }));
    } catch (e) {
      setResults((r) => ({ ...r, [d.id]: { ok: false, text: (e as Error).message } }));
    } finally {
      setTesting(null);
    }
  };

  const remove = async () => {
    if (!confirm) return;
    const res = await fetch(`/api/backups/destinations?id=${confirm.id}`, { method: 'DELETE' });
    if (!res.ok) toast.error('Could not remove it');
    setConfirm(null);
    await load();
  };

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <p className="text-sm max-w-2xl" style={{ color: 'var(--text-secondary)' }}>
          Backups are made on this server, then sent where you choose. For safety keep a copy somewhere else: if this server is lost, backups stored only on it are lost too.
        </p>
        <button className="cyber-button inline-flex items-center gap-2" onClick={() => setAdding(true)}>
          <Plus size={16} weight="bold" /> Add a destination
        </button>
      </div>

      {dests.length === 0 && <p className="py-8 text-center" style={{ color: 'var(--text-secondary)' }}>Backups go to a folder on this server until you add another place.</p>}
      <ul className="space-y-3">
        {dests.map((d) => {
          const c = parse(d.config);
          const r = results[d.id];
          return (
            <li key={d.id} className="card-vapor p-4 rounded-xl">
              <div className="flex flex-wrap items-center gap-3 justify-between">
                <div className="min-w-0">
                  <div className="font-bold flex items-center gap-2" style={{ color: 'var(--text-primary)' }}>
                    <CloudArrowUp size={18} style={{ color: 'var(--text-cyan)' }} /> {d.name}
                    {c.encrypt && (
                      <span className="inline-flex items-center gap-1 text-xs font-mono px-2 py-0.5 rounded-full" style={{ color: 'var(--text-green)', border: '1px solid var(--text-green)' }}>
                        <LockSimple size={11} weight="bold" /> Locked
                      </span>
                    )}
                  </div>
                  <div className="text-xs font-mono truncate" style={{ color: 'var(--text-secondary)' }}>
                    {TYPES.find((t) => t.id === d.type)?.label.split(' (')[0] || d.type} · {whereOf(d)}
                  </div>
                </div>
                <div className="flex gap-2">
                  {d.type !== 'local' && (
                    <button className="btn-neon px-3 py-1 text-xs font-bold" disabled={testing === d.id} onClick={() => test(d)}>
                      {testing === d.id ? 'Testing…' : 'Test'}
                    </button>
                  )}
                  <button className="btn-neon px-3 py-1 text-xs font-bold" aria-label={`Remove ${d.name}`} onClick={() => setConfirm(d)}>
                    <Trash size={14} />
                  </button>
                </div>
              </div>
              {r && (
                <p className="text-xs mt-2" style={{ color: r.ok ? 'var(--text-green)' : 'var(--status-error)' }} role="status">
                  {r.text}
                </p>
              )}
            </li>
          );
        })}
      </ul>

      {adding && <AddDestination servers={servers} onClose={() => setAdding(false)} onAdded={() => { setAdding(false); void load(); }} />}
      {confirm && (
        <ConfirmDeleteModal
          title="Remove destination"
          message="Backups already sent there stay where they are, but DockLite will no longer be able to reach them. Remove it?"
          itemName={confirm.name}
          onConfirm={remove}
          onCancel={() => setConfirm(null)}
          loading={false}
        />
      )}
    </div>
  );
}

function AddDestination({ servers, onClose, onAdded }: { servers: ServerRow[]; onClose: () => void; onAdded: () => void }) {
  const toast = useToast();
  const [type, setType] = useState('s3');
  const [name, setName] = useState('');
  const [f, setF] = useState<Cfg>({ region: 'auto', port: 22 });
  const [lock, setLock] = useState<'off' | 'new' | 'mine'>('new');
  const [myKey, setMyKey] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [keys, setKeys] = useState<{ recipient: string; identity: string } | null>(null);
  const [saved, setSaved] = useState(false);
  const [hostKey, setHostKey] = useState('');
  const set = (k: string) => (e: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) => setF((p) => ({ ...p, [k]: e.target.value }));

  const create = async (recipient: string) => {
    const config: Cfg = { ...f, port: Number(f.port) || 0, server_id: Number(f.server_id) || 0 };
    if (recipient) {
      config.encrypt = true;
      config.recipient = recipient;
    }
    const res = await fetch('/api/backups/destinations', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ name, type, config }) });
    const data = await res.json().catch(() => ({}));
    if (!res.ok) throw new Error(data.error || 'Could not add it');
    toast.success('Destination added. Press Test to check it works.');
    if (data.host_key) {
      setHostKey(data.host_key);
      return false;
    }
    return true;
  };

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    setBusy(true);
    try {
      if (lock === 'new') {
        // The private key is shown first and nothing is saved until they say they kept it.
        const res = await fetch('/api/backups/keys', { method: 'POST' });
        if (!res.ok) throw new Error('Could not make a key');
        setKeys(await res.json());
      } else if (await create(lock === 'mine' ? myKey.trim() : '')) {
        onAdded();
      }
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setBusy(false);
    }
  };

  const confirmKeySaved = async () => {
    if (!keys) return;
    setBusy(true);
    try {
      if (await create(keys.recipient)) onAdded();
      setKeys(null);
    } catch (err) {
      setError((err as Error).message);
      setKeys(null);
    } finally {
      setBusy(false);
    }
  };

  const downloadKey = () => {
    if (!keys) return;
    const blob = new Blob([`# DockLite backup key for ${name}\n# public key: ${keys.recipient}\n${keys.identity}\n`], { type: 'text/plain' });
    const a = document.createElement('a');
    a.href = URL.createObjectURL(blob);
    a.download = `docklite-backup-key-${name.replace(/[^a-z0-9]+/gi, '-').toLowerCase()}.txt`;
    a.click();
    URL.revokeObjectURL(a.href);
  };

  return (
    <div className="fixed inset-0 z-[9000] flex items-center justify-center p-3 sm:p-6 bg-black/75 backdrop-blur-sm" onClick={() => !busy && onClose()}>
      <div
        className="card-vapor w-full max-w-xl max-h-[92vh] overflow-y-auto p-5 sm:p-6"
        style={{ background: 'linear-gradient(135deg, var(--modal-bg-1) 0%, var(--modal-bg-2) 100%)', border: '2px solid var(--modal-border)' }}
        onClick={(e) => e.stopPropagation()}
        role="dialog"
        aria-label="Add a backup destination"
      >
        {hostKey ? (
          <div className="space-y-4">
            <h2 className="text-xl font-bold" style={{ color: 'var(--text-primary)' }}>Destination added</h2>
            <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>
              DockLite saved this server&apos;s identity so nothing else can pretend to be it. If you know what it should be, check it matches:
            </p>
            <code className="block text-xs break-all p-3 rounded-lg" style={{ border: '1px solid var(--modal-border)' }}>{hostKey}</code>
            <button className="cyber-button" onClick={onAdded}>Done</button>
          </div>
        ) : keys ? (
          <div className="space-y-4">
            <h2 className="text-xl font-bold" style={{ color: 'var(--text-primary)' }}>Save your backup key</h2>
            <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>
              Backups sent to <b>{name}</b> are locked with this key. <b>DockLite does not keep the private half</b>, so if you lose it, those backups can never be opened. Store it in a password manager.
            </p>
            <textarea readOnly rows={3} value={keys.identity} className="input-vapor w-full font-mono text-xs" onFocus={(e) => e.currentTarget.select()} aria-label="Private backup key" />
            <div className="flex flex-wrap gap-2">
              <button className="btn-neon px-3 py-1 text-xs font-bold" onClick={() => navigator.clipboard?.writeText(keys.identity).then(() => toast.success('Copied'))}>Copy</button>
              <button className="btn-neon px-3 py-1 text-xs font-bold" onClick={downloadKey}>Download as a file</button>
            </div>
            <label className="flex items-start gap-2 text-sm" style={{ color: 'var(--text-primary)' }}>
              <input type="checkbox" checked={saved} onChange={(e) => setSaved(e.target.checked)} className="mt-1" />
              I have saved the key somewhere safe.
            </label>
            <button className="cyber-button" disabled={!saved || busy} onClick={confirmKeySaved}>{busy ? 'Adding…' : 'Add the destination'}</button>
          </div>
        ) : (
          <form onSubmit={submit} className="space-y-4">
            <h2 className="text-xl font-bold" style={{ color: 'var(--text-primary)' }}>Add a destination</h2>
            <Field label="What kind" hint={TYPES.find((t) => t.id === type)?.hint}>
              <select value={type} onChange={(e) => setType(e.target.value)} className="input-vapor w-full">
                {TYPES.map((t) => (
                  <option key={t.id} value={t.id}>{t.label}</option>
                ))}
              </select>
            </Field>
            <Field label="Name"><input required value={name} onChange={(e) => setName(e.target.value)} className="input-vapor w-full" placeholder="Cloudflare R2" /></Field>

            {type === 's3' && (
              <>
                <Field label="Endpoint" hint="R2: https://<account id>.r2.cloudflarestorage.com. Backblaze: https://s3.<region>.backblazeb2.com. Leave empty for Amazon S3.">
                  <input value={f.endpoint || ''} onChange={set('endpoint')} className="input-vapor w-full" />
                </Field>
                <div className="grid grid-cols-2 gap-3">
                  <Field label="Bucket"><input required value={f.bucket || ''} onChange={set('bucket')} className="input-vapor w-full" /></Field>
                  <Field label="Region" hint='"auto" for R2'><input value={f.region || ''} onChange={set('region')} className="input-vapor w-full" /></Field>
                </div>
                <Field label="Folder in the bucket (optional)"><input value={f.prefix || ''} onChange={set('prefix')} className="input-vapor w-full" placeholder="docklite" /></Field>
                <div className="grid grid-cols-2 gap-3">
                  <Field label="Access key"><input required autoComplete="off" value={f.access_key || ''} onChange={set('access_key')} className="input-vapor w-full" /></Field>
                  <Field label="Secret key"><input required type="password" autoComplete="new-password" value={f.secret_key || ''} onChange={set('secret_key')} className="input-vapor w-full" /></Field>
                </div>
              </>
            )}
            {type === 'sftp' && (
              <>
                <div className="grid grid-cols-3 gap-3">
                  <div className="col-span-2"><Field label="Server address"><input required value={f.host || ''} onChange={set('host')} className="input-vapor w-full" placeholder="203.0.113.9" /></Field></div>
                  <Field label="Port"><input type="number" value={f.port ?? 22} onChange={set('port')} className="input-vapor w-full" /></Field>
                </div>
                <div className="grid grid-cols-2 gap-3">
                  <Field label="User"><input required value={f.user || ''} onChange={set('user')} className="input-vapor w-full" /></Field>
                  <Field label="Folder there"><input value={f.dir || ''} onChange={set('dir')} className="input-vapor w-full" placeholder="/srv/docklite-backups" /></Field>
                </div>
                <Field label="Password"><input type="password" autoComplete="new-password" value={f.password || ''} onChange={set('password')} className="input-vapor w-full" /></Field>
                <Field label="…or a private key (paste it; one without a passphrase)">
                  <textarea rows={3} value={f.private_key || ''} onChange={set('private_key')} className="input-vapor w-full font-mono text-xs" />
                </Field>
              </>
            )}
            {type === 'server' && (
              <Field label="Server" hint={servers.length === 0 ? 'No servers connected yet. Add one on the Servers page first.' : undefined}>
                <select required value={f.server_id || ''} onChange={(e) => setF((p) => ({ ...p, server_id: e.target.value }))} className="input-vapor w-full">
                  <option value="">Choose a server…</option>
                  {servers.map((s) => (
                    <option key={s.id} value={s.id}>{s.name}</option>
                  ))}
                </select>
              </Field>
            )}
            {type === 'local' && (
              <Field label="Folder" hint="Leave empty to use this server's own backup folder."><input value={f.path || ''} onChange={set('path')} className="input-vapor w-full" placeholder="/mnt/backups" /></Field>
            )}

            <fieldset className="rounded-lg p-3 space-y-2" style={{ border: '1px solid var(--modal-border)' }}>
              <legend className="px-1 text-sm font-bold flex items-center gap-1" style={{ color: 'var(--text-green)' }}><LockSimple size={14} weight="bold" /> Lock backups</legend>
              <p className="text-xs" style={{ color: 'var(--text-secondary)' }}>
                Scrambles every backup before it leaves this server, so whoever stores it can&apos;t read your sites or databases. You will need the key to restore.
              </p>
              {([['new', 'Lock them: make a new key for me'], ['mine', 'Lock them with a key I already have'], ['off', 'Don’t lock them']] as const).map(([v, l]) => (
                <label key={v} className="flex items-center gap-2 text-sm" style={{ color: 'var(--text-primary)' }}>
                  <input type="radio" name="lock" checked={lock === v} onChange={() => setLock(v)} /> {l}
                </label>
              ))}
              {lock === 'mine' && (
                <input required value={myKey} onChange={(e) => setMyKey(e.target.value)} className="input-vapor w-full font-mono text-xs" placeholder="Public key, starts with age1…" aria-label="Public backup key" />
              )}
            </fieldset>

            {error && <div className="text-sm rounded-lg p-3" style={{ color: 'var(--status-error)', border: '1px solid var(--status-error)' }} role="alert">{error}</div>}
            <div className="flex gap-3">
              <button type="submit" className="cyber-button" disabled={busy}>{busy ? 'Working…' : lock === 'new' ? 'Next: make my key' : 'Add the destination'}</button>
              <button type="button" className="btn-neon px-4 py-2 text-sm font-bold" onClick={onClose} disabled={busy}>Cancel</button>
            </div>
          </form>
        )}
      </div>
    </div>
  );
}
