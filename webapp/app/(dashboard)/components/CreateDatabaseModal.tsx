'use client';

import { useState } from 'react';
import { ContainerInfo } from '@/types';
import { WarningCircle } from '@phosphor-icons/react';
import CloseButton from './CloseButton';
import { DB_ENGINES, type DbType } from '@/lib/database-engines';
import { containerDisplayName } from '@/lib/container-names';

interface CreateDatabaseModalProps {
  type: DbType;
  sites: ContainerInfo[];
  onBack: () => void;
  onClose: () => void;
  onCreated: () => void;
}

interface ConnectionInfo {
  type: string;
  host?: string;
  port?: number;
  database?: string;
  username?: string;
  password?: string;
  url?: string;
  container_url?: string;
}

/** The form for one kind of database; after it is made, the details to connect with (the password is shown only now). */
export default function CreateDatabaseModal({ type, sites, onBack, onClose, onCreated }: CreateDatabaseModalProps) {
  const engine = DB_ENGINES.find((e) => e.id === type)!;
  const [name, setName] = useState('');
  const [username, setUsername] = useState('docklite');
  const [password, setPassword] = useState('');
  const [siteId, setSiteId] = useState('');
  const [path, setPath] = useState('/app/data/db.sqlite');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [info, setInfo] = useState<ConnectionInfo | null>(null);

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError('');
    try {
      const payload: Record<string, unknown> = { name: name.trim(), type };
      if (type === 'sqlite') {
        payload.container_id = siteId;
        payload.db_path = path;
      } else {
        if (engine.hasUser) payload.username = username;
        payload.password = password || undefined;
      }
      const res = await fetch('/api/databases', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(payload) });
      const data = await res.json().catch(() => ({}));
      if (!res.ok) throw new Error(data.error || 'Could not create the database');
      onCreated();
      setInfo(data.connection || { type });
    } catch (err: any) {
      setError(err.message || 'Could not create the database');
    } finally {
      setBusy(false);
    }
  };

  const shell = (children: React.ReactNode) => (
    <div className="fixed inset-0 z-[10000] flex items-center justify-center p-3 sm:p-4 bg-black/75 backdrop-blur-sm" onClick={info ? undefined : onClose}>
      <div
        className="card-vapor w-full max-w-xl max-h-[92vh] overflow-y-auto p-4 sm:p-6 relative"
        style={{ background: 'linear-gradient(135deg, var(--modal-bg-1) 0%, var(--modal-bg-2) 100%)', border: '2px solid var(--modal-border)', boxShadow: '0 0 30px var(--modal-shadow)' }}
        onClick={(e) => e.stopPropagation()}
      >
        <CloseButton onClick={onClose} className="absolute top-3 right-3" />
        {children}
      </div>
    </div>
  );

  if (info) {
    const rows: [string, string | number | undefined][] = [
      ['Host (from this server)', info.host || 'localhost'],
      ['Port', info.port],
      ...(info.type !== 'redis' ? ([['Database', info.database], ['Username', info.username]] as [string, string | undefined][]) : []),
      ['Password', info.password],
    ];
    return shell(
      <>
        <h2 className="text-2xl font-bold pr-10" style={{ color: 'var(--text-primary)' }}>{engine.label} database is ready</h2>
        <dl className="mt-4 space-y-2 text-sm font-mono">
          {rows.filter(([, v]) => v !== undefined && v !== '').map(([k, v]) => (
            <div key={k} className="flex justify-between gap-4">
              <dt style={{ color: 'var(--text-cyan)' }}>{k}</dt>
              <dd className="break-all text-right select-all" style={{ color: k === 'Password' ? 'var(--text-pink)' : 'var(--text-primary)' }}>{String(v)}</dd>
            </div>
          ))}
        </dl>
        {info.url && (
          <div className="mt-4 space-y-3 text-xs font-mono" style={{ color: 'var(--text-secondary)' }}>
            <div>
              <div style={{ color: 'var(--text-cyan)' }}>From this server:</div>
              <div className="break-all select-all" style={{ color: 'var(--text-primary)' }}>{info.url}</div>
            </div>
            {info.container_url && (
              <div>
                <div style={{ color: 'var(--text-cyan)' }}>From one of your sites (another box on this server):</div>
                <div className="break-all select-all" style={{ color: 'var(--text-primary)' }}>{info.container_url}</div>
              </div>
            )}
          </div>
        )}
        {type !== 'sqlite' && (
          <p className="mt-4 text-xs flex items-center gap-2" style={{ color: 'var(--text-yellow)' }}>
            <WarningCircle size={14} weight="duotone" /> Save the password now. It will not be shown again.
          </p>
        )}
        <button type="button" onClick={onClose} className="mt-5 cyber-button">Got it</button>
      </>,
    );
  }

  return shell(
    <form onSubmit={submit} className="space-y-4">
      <div className="pr-10">
        <button type="button" onClick={onBack} className="text-xs underline" style={{ color: 'var(--text-secondary)' }}>← All kinds of box</button>
        <h2 className="text-2xl font-bold mt-1" style={{ color: 'var(--text-primary)' }}>New {engine.label} database</h2>
        <p className="text-sm mt-1" style={{ color: 'var(--text-secondary)' }}>{engine.blurb}</p>
      </div>

      <label className="block">
        <span className="block text-xs mb-1" style={{ color: 'var(--text-secondary)' }}>Name</span>
        <input value={name} onChange={(e) => setName(e.target.value)} required pattern="[A-Za-z0-9_\-]+" title="Letters, numbers, - and _" className="input-vapor w-full" placeholder="shop_db" />
      </label>

      {type === 'sqlite' ? (
        <>
          <label className="block">
            <span className="block text-xs mb-1" style={{ color: 'var(--text-secondary)' }}>Which website does it live in?</span>
            <select value={siteId} onChange={(e) => setSiteId(e.target.value)} required className="input-vapor w-full">
              <option value="">Choose a website…</option>
              {sites.map((s) => (
                <option key={s.id} value={s.id}>{containerDisplayName(s)}</option>
              ))}
            </select>
          </label>
          <label className="block">
            <span className="block text-xs mb-1" style={{ color: 'var(--text-secondary)' }}>File path inside the website</span>
            <input value={path} onChange={(e) => setPath(e.target.value)} required className="input-vapor w-full font-mono" />
          </label>
        </>
      ) : (
        <>
          {engine.hasUser && (
            <label className="block">
              <span className="block text-xs mb-1" style={{ color: 'var(--text-secondary)' }}>Username</span>
              <input value={username} onChange={(e) => setUsername(e.target.value)} className="input-vapor w-full" />
            </label>
          )}
          <label className="block">
            <span className="block text-xs mb-1" style={{ color: 'var(--text-secondary)' }}>Password (leave empty for a strong random one)</span>
            <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} autoComplete="new-password" className="input-vapor w-full" />
          </label>
        </>
      )}

      {error && <p className="text-sm" style={{ color: 'var(--status-error)' }}>{error}</p>}
      <div className="flex gap-3">
        <button type="button" onClick={onBack} disabled={busy} className="flex-1 px-4 py-2 rounded-lg font-bold border-2" style={{ borderColor: 'var(--surface-dim)', color: 'var(--text-primary)' }}>
          Back
        </button>
        <button type="submit" disabled={busy || (type === 'sqlite' && !siteId)} className="flex-1 cyber-button">
          {busy ? 'Creating…' : 'Create database'}
        </button>
      </div>
    </form>,
  );
}
