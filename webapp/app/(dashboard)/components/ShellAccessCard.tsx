'use client';

import { useCallback, useEffect, useState } from 'react';
import { TerminalWindow, UserMinus, UserPlus, WarningCircle } from '@phosphor-icons/react';
import { useToast } from '@/lib/hooks/useToast';

/**
 * Which server users can run `docklite` in their own SSH session with full
 * admin rights. Super admin only: the card disappears for everyone else.
 */
export default function ShellAccessCard() {
  const toast = useToast();
  const [members, setMembers] = useState<string[] | null>(null);
  const [allowed, setAllowed] = useState(true);
  const [username, setUsername] = useState('');
  const [busy, setBusy] = useState(false);

  const load = useCallback(async () => {
    try {
      const res = await fetch('/api/system/shell-access');
      if (res.status === 403) {
        setAllowed(false);
        return;
      }
      if (!res.ok) throw new Error();
      setMembers((await res.json()).members || []);
    } catch {
      setMembers([]);
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  const change = async (name: string, action: 'grant' | 'revoke') => {
    const what =
      action === 'grant'
        ? `Give ${name} admin control of DockLite from the server's command line?\n\nThey will be able to read the server's master token, so treat this like giving them root for DockLite.`
        : `Remove ${name}'s admin shell access to DockLite?`;
    if (!window.confirm(what)) return;
    setBusy(true);
    try {
      const res = await fetch('/api/system/shell-access', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username: name, action }),
      });
      const data = await res.json().catch(() => ({}));
      if (!res.ok) throw new Error(data.error || 'The change failed');
      toast.success(action === 'grant' ? `${name} now has shell access — they need to log out and in once` : `${name}'s shell access removed`);
      setUsername('');
      await load();
    } catch (err: any) {
      toast.error(err.message || 'The change failed');
    } finally {
      setBusy(false);
    }
  };

  if (!allowed) return null;

  return (
    <div className="card-vapor p-6 rounded-xl">
      <h3 className="text-xl font-bold neon-text mb-2 flex items-center gap-2" style={{ color: 'var(--neon-green)' }}>
        <TerminalWindow size={20} weight="duotone" />
        Shell access
      </h3>
      <p className="text-sm mb-4" style={{ color: 'var(--text-secondary)' }}>
        People who log in to this server with their own account can run <code>docklite</code> from the command line
        using their DockLite login (<code>docklite login</code>) with no setup here. Adding someone below goes
        further: they can run it with <b>full admin rights and no login</b>.
      </p>
      <div className="text-xs flex items-start gap-2 p-3 rounded-lg mb-4" style={{ background: 'rgba(var(--status-warning-rgb), 0.12)', border: '1px solid rgba(var(--status-warning-rgb), 0.4)' }}>
        <WarningCircle size={16} weight="duotone" style={{ color: 'var(--status-warning)', flexShrink: 0 }} />
        <span>This lets them read the server&apos;s master token, so it is like giving them root for DockLite. Only add people you trust. Every change is written to the audit log.</span>
      </div>

      <div className="space-y-2 mb-4">
        {members === null ? (
          <div className="text-sm opacity-70">Loading…</div>
        ) : members.length === 0 ? (
          <div className="text-sm opacity-70">No one has admin shell access yet.</div>
        ) : (
          members.map((name) => (
            <div key={name} className="flex items-center justify-between p-2 rounded-lg" style={{ background: 'var(--surface-muted)' }}>
              <span className="font-mono text-sm">{name}</span>
              <button
                type="button"
                disabled={busy}
                onClick={() => change(name, 'revoke')}
                className="inline-flex items-center gap-1 px-2 py-1 text-xs font-bold rounded-lg disabled:opacity-50"
                style={{ color: 'var(--status-error)' }}
              >
                <UserMinus size={14} weight="bold" /> Remove
              </button>
            </div>
          ))
        )}
      </div>

      <form
        className="flex flex-wrap gap-2"
        onSubmit={(e) => {
          e.preventDefault();
          if (username.trim()) change(username.trim(), 'grant');
        }}
      >
        <input
          value={username}
          onChange={(e) => setUsername(e.target.value)}
          placeholder="Server username (e.g. alice)"
          className="input-vapor px-3 py-2 text-sm flex-1 min-w-[12rem]"
          autoComplete="off"
          aria-label="Server username"
        />
        <button type="submit" disabled={busy || !username.trim()} className="btn-neon px-4 py-2 text-sm font-bold inline-flex items-center gap-2 disabled:opacity-50">
          <UserPlus size={16} weight="bold" /> Give shell access
        </button>
      </form>
    </div>
  );
}
