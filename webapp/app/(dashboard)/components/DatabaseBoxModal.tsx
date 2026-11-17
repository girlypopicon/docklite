'use client';

import { useState } from 'react';
import { useRouter } from 'next/navigation';
import { ContainerInfo } from '@/types';
import CloseButton from './CloseButton';
import { useEscape } from '@/lib/hooks/useEscape';
import { DB_ENGINES, dbLabel, formatBytes, type DatabaseRecord } from '@/lib/database-engines';

interface DatabaseBoxModalProps {
  db: DatabaseRecord;
  container?: ContainerInfo; // the database's box (a SQLite file has none)
  onClose: () => void;
  onChanged: () => void;
  onOpenContainer?: (container: ContainerInfo) => void;
}

type Panel = 'overview' | 'editor' | 'download' | 'credentials' | 'delete';

const INNER_PORT: Record<string, number> = { postgres: 5432, mysql: 3306, mariadb: 3306, mongodb: 27017, redis: 6379 };

/**
 * What the Databases page used to do, in the window of a database's box: how to connect, open the table editor, download a
 * copy, change the password, and delete it.
 */
export default function DatabaseBoxModal({ db, container, onClose, onChanged, onOpenContainer }: DatabaseBoxModalProps) {
  useEscape(onClose);
  const router = useRouter();
  const type = db.type || 'postgres';
  const engine = DB_ENGINES.find((e) => e.id === type);
  const hasUser = engine?.hasUser ?? true;
  const [panel, setPanel] = useState<Panel>('overview');
  const [username, setUsername] = useState(db.username || 'docklite');
  const [password, setPassword] = useState('');
  const [gzip, setGzip] = useState(true);
  const [confirmText, setConfirmText] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [note, setNote] = useState('');

  const go = (p: Panel) => {
    setPanel(p);
    setError('');
    setNote('');
  };

  const call = async (fn: () => Promise<void>) => {
    setBusy(true);
    setError('');
    try {
      await fn();
    } catch (err: any) {
      setError(err.message || 'Something went wrong');
    } finally {
      setBusy(false);
    }
  };

  const openEditor = () => {
    if (hasUser && (!username || !password)) {
      setError('Enter the database username and password.');
      return;
    }
    sessionStorage.setItem(`docklite-db-edit-${db.id}`, JSON.stringify({ username, password }));
    onClose();
    router.push(`/databases/${db.id}/edit`);
  };

  const download = () =>
    call(async () => {
      if (hasUser && (!username || !password)) throw new Error('Enter the database username and password.');
      const res = await fetch(`/api/databases/${db.id}/download`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username, password, gzip }),
      });
      if (!res.ok) throw new Error((await res.json().catch(() => ({}))).error || 'Could not download the database');
      const blob = await res.blob();
      const match = (res.headers.get('Content-Disposition') || '').match(/filename="([^"]+)"/i);
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = match?.[1] || `docklite-${db.name}.dump${gzip ? '.gz' : ''}`;
      document.body.appendChild(a);
      a.click();
      document.body.removeChild(a);
      URL.revokeObjectURL(url);
      setNote('Download started.');
    });

  const changeCredentials = () =>
    call(async () => {
      const res = await fetch(`/api/databases/${db.id}`, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username, password }),
      });
      if (!res.ok) throw new Error((await res.json().catch(() => ({}))).error || 'Could not change the credentials');
      setPassword('');
      setNote('Credentials changed.');
      onChanged();
    });

  const remove = () =>
    call(async () => {
      const res = await fetch(`/api/databases/${db.id}`, { method: 'DELETE' });
      if (!res.ok) throw new Error((await res.json().catch(() => ({}))).error || 'Could not delete the database');
      onChanged();
      onClose();
    });

  const tab = (id: Panel, label: string) => (
    <button
      key={id}
      type="button"
      onClick={() => go(id)}
      className="px-3 py-1.5 rounded-lg text-sm font-bold whitespace-nowrap"
      style={{
        color: id === 'delete' ? 'var(--status-error)' : 'var(--text-primary)',
        border: panel === id ? '1px solid var(--text-cyan)' : '1px solid rgba(var(--neon-purple-rgb), 0.3)',
        background: panel === id ? 'rgba(var(--neon-cyan-rgb), 0.12)' : 'transparent',
      }}
    >
      {label}
    </button>
  );

  const creds = (
    <div className="grid gap-3 sm:grid-cols-2">
      {hasUser && (
        <label className="block">
          <span className="block text-xs mb-1" style={{ color: 'var(--text-secondary)' }}>Username</span>
          <input value={username} onChange={(e) => setUsername(e.target.value)} className="input-vapor w-full" />
        </label>
      )}
      <label className="block">
        <span className="block text-xs mb-1" style={{ color: 'var(--text-secondary)' }}>{panel === 'credentials' ? 'New password' : 'Password'}</span>
        <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} autoComplete="off" className="input-vapor w-full" />
      </label>
    </div>
  );

  const host = container?.name;
  const inner = INNER_PORT[type];

  return (
    <div className="fixed inset-0 z-[9500] flex items-center justify-center p-3 sm:p-6 bg-black/75 backdrop-blur-sm" onClick={onClose}>
      <div
        className="card-vapor w-full max-w-2xl max-h-[92vh] overflow-y-auto p-4 sm:p-6 relative"
        style={{ background: 'linear-gradient(135deg, var(--modal-bg-1) 0%, var(--modal-bg-2) 100%)', border: '2px solid var(--modal-border)', boxShadow: '0 0 30px var(--modal-shadow)' }}
        onClick={(e) => e.stopPropagation()}
        role="dialog"
        aria-label={`Database ${db.name}`}
      >
        <CloseButton onClick={onClose} className="absolute top-3 right-3" />
        <div className="pr-12">
          <h2 className="text-2xl font-bold truncate" style={{ color: 'var(--text-primary)' }}>{db.name}</h2>
          <p className="text-sm mt-1" style={{ color: 'var(--text-secondary)' }}>
            {dbLabel(type)} · {formatBytes(db.size)}{db.created_at ? ` · made ${new Date(db.created_at).toLocaleDateString()}` : ''}
          </p>
        </div>

        <div className="mt-4 flex gap-2 overflow-x-auto pb-1">
          {tab('overview', 'Connect')}
          {type !== 'redis' && tab('editor', 'Tables')}
          {tab('download', 'Download')}
          {type !== 'sqlite' && tab('credentials', 'Password')}
          {tab('delete', 'Delete')}
        </div>

        <div className="mt-5">
          {panel === 'overview' && (
            <div className="space-y-4 text-sm font-mono" style={{ color: 'var(--text-secondary)' }}>
              {type === 'sqlite' ? (
                <div>
                  <div style={{ color: 'var(--text-cyan)' }}>File</div>
                  <div className="break-all select-all" style={{ color: 'var(--text-primary)' }}>{db.db_path}</div>
                  <p className="mt-2 font-sans text-xs">A SQLite database is a file inside one of your websites. Your site opens it directly; there is no server to connect to.</p>
                </div>
              ) : (
                <>
                  <div>
                    <div style={{ color: 'var(--text-cyan)' }}>From this server</div>
                    <div className="select-all" style={{ color: 'var(--text-primary)' }}>localhost:{db.postgres_port}</div>
                  </div>
                  {host && inner && (
                    <div>
                      <div style={{ color: 'var(--text-cyan)' }}>From one of your websites (another box on this server)</div>
                      <div className="select-all" style={{ color: 'var(--text-primary)' }}>{host}:{inner}</div>
                    </div>
                  )}
                  {hasUser && (
                    <div>
                      <div style={{ color: 'var(--text-cyan)' }}>Username</div>
                      <div style={{ color: 'var(--text-primary)' }}>{db.username || 'docklite'}</div>
                    </div>
                  )}
                  <p className="font-sans text-xs">The password was shown when you made it. If it&apos;s lost, set a new one under Password.</p>
                </>
              )}
              {container && onOpenContainer && (
                <button type="button" className="cyber-button-sm" onClick={() => onOpenContainer(container)}>Logs, stats and the box itself</button>
              )}
            </div>
          )}

          {(panel === 'editor' || panel === 'download' || panel === 'credentials') && (
            <div className="space-y-4">
              <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>
                {panel === 'editor' && (type === 'sqlite' ? 'Open the table editor. A SQLite file has no login, so there is nothing to enter.' : 'Open the table editor. It needs the database login.')}
                {panel === 'download' && (type === 'sqlite' ? 'Download a copy of the database file.' : 'Download a copy of this database. It needs the database login.')}
                {panel === 'credentials' && 'Set a new password for this database. Sites that use the old one need updating.'}
              </p>
              {type !== 'sqlite' && creds}
              {panel === 'download' && (
                <label className="flex items-center gap-2 text-sm" style={{ color: 'var(--text-primary)' }}>
                  <input type="checkbox" checked={gzip} onChange={(e) => setGzip(e.target.checked)} /> Compress it (.gz)
                </label>
              )}
              <button
                type="button"
                className="cyber-button"
                disabled={busy}
                onClick={panel === 'editor' ? openEditor : panel === 'download' ? download : changeCredentials}
              >
                {busy ? 'Working…' : panel === 'editor' ? 'Open table editor' : panel === 'download' ? 'Download' : 'Change password'}
              </button>
            </div>
          )}

          {panel === 'delete' && (
            <div className="space-y-4">
              <p className="text-sm" style={{ color: 'var(--status-error)' }}>
                This deletes the database and everything in it. It cannot be undone. Type <b>{db.name}</b> to confirm.
              </p>
              <input value={confirmText} onChange={(e) => setConfirmText(e.target.value)} className="input-vapor w-full" placeholder={db.name} />
              <button type="button" disabled={busy || confirmText !== db.name} onClick={remove} className="px-4 py-2 rounded-lg font-bold" style={{ color: 'var(--status-error)', border: '1px solid var(--status-error)' }}>
                {busy ? 'Deleting…' : 'Delete this database'}
              </button>
            </div>
          )}

          {error && <p className="mt-3 text-sm" style={{ color: 'var(--status-error)' }}>{error}</p>}
          {note && <p className="mt-3 text-sm" style={{ color: 'var(--text-green)' }}>{note}</p>}
        </div>
      </div>
    </div>
  );
}
