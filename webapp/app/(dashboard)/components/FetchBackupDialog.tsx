'use client';

import { useState } from 'react';
import { LockSimple } from '@phosphor-icons/react';
import CloseButton from './CloseButton';
import { useEscape } from '@/lib/hooks/useEscape';

interface Props {
  backupId: number;
  path: string; // where the backup is: s3://…, sftp://…, server://…, or a file ending in .age
  onReady: (file: string) => void;
  onClose: () => void;
}

/** A backup on another destination, or a locked one, has to come to this server (and be unlocked) before it can be restored. */
export default function FetchBackupDialog({ backupId, path, onReady, onClose }: Props) {
  useEscape(onClose);
  const locked = path.endsWith('.age');
  const remote = path.includes('://');
  const [key, setKey] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');

  const go = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError('');
    try {
      const res = await fetch('/api/backups/fetch', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ backup_id: backupId, identity: key }) });
      const data = await res.json().catch(() => ({}));
      if (!res.ok) throw new Error(data.error || 'Could not get the backup');
      onReady(data.file);
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="fixed inset-0 z-[9000] flex items-center justify-center p-3 bg-black/75 backdrop-blur-sm" onClick={() => !busy && onClose()}>
      <form
        onSubmit={go}
        className="card-vapor w-full max-w-lg p-5 sm:p-6 relative space-y-4"
        style={{ background: 'linear-gradient(135deg, var(--modal-bg-1) 0%, var(--modal-bg-2) 100%)', border: '2px solid var(--modal-border)' }}
        onClick={(e) => e.stopPropagation()}
        role="dialog"
        aria-label="Get this backup"
      >
        <CloseButton onClick={onClose} className="absolute top-3 right-3" />
        <h2 className="text-xl font-bold pr-10 flex items-center gap-2" style={{ color: 'var(--text-primary)' }}>
          {locked && <LockSimple size={22} style={{ color: 'var(--text-green)' }} />} Get this backup first
        </h2>
        <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>
          {remote ? 'This backup is stored elsewhere, so DockLite will copy it here' : 'This backup is locked, so DockLite will unlock a copy'}
          {remote && locked ? ' and unlock it' : ''}. The copy is deleted an hour later.
        </p>
        {locked && (
          <label className="block">
            <span className="block text-xs font-bold mb-1" style={{ color: 'var(--text-pink)' }}>Your private backup key</span>
            <textarea required rows={3} value={key} onChange={(e) => setKey(e.target.value)} className="input-vapor w-full font-mono text-xs" placeholder="AGE-SECRET-KEY-1…" autoComplete="off" />
            <span className="block text-xs mt-1" style={{ color: 'var(--text-secondary)' }}>Used for this restore only. It is not saved.</span>
          </label>
        )}
        {error && <div className="text-sm rounded-lg p-3" style={{ color: 'var(--status-error)', border: '1px solid var(--status-error)' }} role="alert">{error}</div>}
        <button type="submit" className="cyber-button" disabled={busy}>{busy ? 'Getting it…' : 'Get it and continue'}</button>
      </form>
    </div>
  );
}
