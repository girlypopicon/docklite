'use client';

import { useState } from 'react';
import type { RemoteServer } from './ServerTabs';

/** Connect another DockLite server. It must already have DockLite installed (the agent is enough; the dashboard and terminal
 *  app there are optional). On that server run `docklite share`, then paste the code here. */
export default function AddServerModal({ onClose, onAdded }: { onClose: () => void; onAdded: (s: RemoteServer) => void }) {
  const [join, setJoin] = useState('');
  const [name, setName] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');

  const submit = async () => {
    setBusy(true);
    setError('');
    try {
      const res = await fetch('/api/servers', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ join: join.trim(), name: name.trim() }),
      });
      const data = await res.json().catch(() => ({}));
      if (!res.ok) throw new Error(data.error || 'Could not add the server');
      onAdded(data.server);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Could not add the server');
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="fixed inset-0 bg-black/80 flex items-center justify-center z-[10000] p-4" onClick={onClose}>
      <div className="cyber-card max-w-lg w-full p-6" role="dialog" aria-label="Add server" onClick={(e) => e.stopPropagation()}>
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-2xl font-bold neon-text-pink">Add server</h2>
          <button onClick={onClose} className="transition-colors" style={{ color: 'var(--text-secondary)' }} aria-label="Close">✕</button>
        </div>
        <ol className="text-sm mb-4 space-y-1 list-decimal pl-5" style={{ color: 'var(--text-secondary)' }}>
          <li>Install DockLite on the other server. The agent alone is enough; the dashboard and terminal app there are optional.</li>
          <li>
            On that server run <code className="font-mono" style={{ color: 'var(--text-cyan)' }}>docklite share</code>. It prints a join code.
          </li>
          <li>Paste the code here.</li>
        </ol>
        <label className="block mb-3">
          <span className="block text-sm font-bold mb-2 text-accent-cyan">Join code</span>
          <textarea value={join} onChange={(e) => setJoin(e.target.value)} rows={3} placeholder="dlj1...." className="input-vapor w-full font-mono text-xs" />
        </label>
        <label className="block mb-4">
          <span className="block text-sm font-bold mb-2 text-accent-cyan">Name (optional)</span>
          <input value={name} onChange={(e) => setName(e.target.value)} placeholder="smoll" className="input-vapor w-full" />
        </label>
        {error && <p className="text-sm mb-3" style={{ color: 'var(--status-error)' }}>{error}</p>}
        <div className="flex gap-2 justify-end">
          <button type="button" onClick={onClose} className="px-4 py-2 font-bold" style={{ color: 'var(--text-secondary)' }}>Cancel</button>
          <button type="button" disabled={busy || !join.trim()} onClick={submit} className="btn-neon px-4 py-2 font-bold">
            {busy ? 'Connecting…' : 'Add server'}
          </button>
        </div>
      </div>
    </div>
  );
}
