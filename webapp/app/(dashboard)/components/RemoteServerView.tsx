'use client';

import { useCallback, useEffect, useState } from 'react';
import { ArrowsClockwise, Play, Stop, ArrowClockwise } from '@phosphor-icons/react';
import { useToast } from '@/lib/hooks/useToast';
import type { ContainerInfo } from '@/types';
import type { RemoteServer } from './ServerTabs';
import RemoteInfoModal, { type InfoKind } from './RemoteInfoModal';
import ConfirmModal from './ConfirmModal';

const VIEWS: { kind: InfoKind; label: string }[] = [
  { kind: 'server', label: 'Server' },
  { kind: 'network', label: 'Network' },
  { kind: 'databases', label: 'Databases' },
  { kind: 'ssl', label: 'HTTPS' },
];

/** Another server's tab: its containers, with buttons that open its server, network, database and certificate views in a
 *  window. Everything goes through this server (the hub), which holds the token; the browser never sees it. */
export default function RemoteServerView({ server, onRemoved }: { server: RemoteServer; onRemoved: () => void }) {
  const [containers, setContainers] = useState<ContainerInfo[] | null>(null);
  const [error, setError] = useState('');
  const [view, setView] = useState<InfoKind | null>(null);
  const [confirmRemove, setConfirmRemove] = useState(false);
  const toast = useToast();
  const base = `/api/servers/${server.id}/proxy`;

  const load = useCallback(async () => {
    setError('');
    try {
      const res = await fetch(`${base}/containers`);
      const data = await res.json().catch(() => ({}));
      if (!res.ok) throw new Error(data.error || `Could not reach ${server.name}`);
      setContainers(data.boxes || data.containers || []);
    } catch (e) {
      setContainers(null);
      setError(e instanceof Error ? e.message : `Could not reach ${server.name}`);
    }
  }, [base, server.name]);

  useEffect(() => {
    setContainers(null);
    void load();
  }, [load]);

  const act = async (c: ContainerInfo, action: 'start' | 'stop' | 'restart') => {
    try {
      const res = await fetch(`${base}/containers/${c.id}/${action}`, { method: 'POST' });
      if (!res.ok) throw new Error((await res.json().catch(() => ({}))).error || `Could not ${action} it`);
      toast.success(`${c.labels?.['docklite.domain'] || c.name}: ${action === 'stop' ? 'stopped' : action === 'start' ? 'started' : 'restarted'}`);
      void load();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : `Could not ${action} it`);
    }
  };

  const remove = async () => {
    const res = await fetch(`/api/servers/${server.id}`, { method: 'DELETE' });
    if (res.ok) {
      toast.success(`${server.name} is no longer shown here. It was not changed.`);
      onRemoved();
    } else toast.error('Could not remove the server');
  };

  return (
    <div>
      <div className="flex flex-wrap items-center justify-between gap-3 mb-6">
        <div>
          <h1 className="text-3xl font-bold neon-text" style={{ color: 'var(--text-cyan)' }}>{server.name}</h1>
          <p className="text-xs font-mono" style={{ color: 'var(--text-secondary)' }}>{server.url}</p>
        </div>
        <div className="flex flex-wrap gap-2">
          {VIEWS.map((v) => (
            <button key={v.kind} type="button" className="btn-neon px-4 py-2 font-bold" onClick={() => setView(v.kind)}>
              {v.label}
            </button>
          ))}
          <button type="button" className="px-3 py-2" aria-label="Refresh" onClick={() => void load()} style={{ color: 'var(--text-secondary)' }}>
            <ArrowsClockwise size={18} weight="regular" />
          </button>
          <button type="button" className="px-3 py-2 text-sm" onClick={() => setConfirmRemove(true)} style={{ color: 'var(--text-secondary)' }}>
            Remove
          </button>
        </div>
      </div>

      {error && (
        <div className="card-vapor p-6 rounded-xl text-center">
          <p className="font-bold mb-3" style={{ color: 'var(--status-error)' }}>{error}</p>
          <button type="button" onClick={() => void load()} className="btn-neon px-4 py-2 font-bold">Try again</button>
        </div>
      )}
      {!error && containers === null && <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>Loading…</p>}
      {containers && containers.length === 0 && <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>Nothing runs on {server.name} yet.</p>}
      {containers && containers.length > 0 && (
        <ul className="grid gap-3 grid-cols-[repeat(auto-fill,minmax(min(100%,260px),1fr))]">
          {containers.map((c) => {
            const running = c.state === 'running';
            return (
              <li key={c.id} className="card-vapor p-4 rounded-xl flex flex-col gap-2">
                <div className="font-bold break-all" style={{ color: 'var(--text-primary)' }}>{c.labels?.['docklite.domain'] || c.name}</div>
                <div className="text-xs font-mono" style={{ color: 'var(--text-secondary)' }}>
                  {c.labels?.['docklite.type'] || c.image} · <span style={{ color: running ? 'var(--text-green)' : 'var(--status-warning)' }}>{c.status}</span>
                </div>
                <div className="flex gap-2 mt-1">
                  {running ? (
                    <button type="button" aria-label={`Stop ${c.name}`} className="px-2 py-1" onClick={() => act(c, 'stop')}><Stop size={16} weight="regular" /></button>
                  ) : (
                    <button type="button" aria-label={`Start ${c.name}`} className="px-2 py-1" onClick={() => act(c, 'start')}><Play size={16} weight="regular" /></button>
                  )}
                  <button type="button" aria-label={`Restart ${c.name}`} className="px-2 py-1" onClick={() => act(c, 'restart')}><ArrowClockwise size={16} weight="regular" /></button>
                </div>
              </li>
            );
          })}
        </ul>
      )}

      {view && <RemoteInfoModal serverId={server.id} serverName={server.name} kind={view} onClose={() => setView(null)} />}
      {confirmRemove && (
        <ConfirmModal
          title={`Remove ${server.name}?`}
          message={`${server.name} will no longer be shown here. Nothing on it is changed. To cut it off completely, also revoke the "hub: ..." token in its token list.`}
          confirmText="Remove"
          onConfirm={() => {
            setConfirmRemove(false);
            void remove();
          }}
          onCancel={() => setConfirmRemove(false)}
        />
      )}
    </div>
  );
}
