'use client';

import { useCallback, useEffect, useState } from 'react';
import { Gauge } from '@phosphor-icons/react';
import { useToast } from '@/lib/hooks/useToast';

interface Limits {
  cpus: number;
  memoryMB: number;
  pids: number;
}
interface Host {
  cpus: number;
  memoryMB: number;
}

const PRESETS: { label: string; hint: string; limits: Limits }[] = [
  { label: 'Small', hint: 'A quiet site', limits: { cpus: 0.25, memoryMB: 256, pids: 100 } },
  { label: 'Medium', hint: 'A busy site', limits: { cpus: 0.5, memoryMB: 512, pids: 200 } },
  { label: 'Large', hint: 'An app or a database', limits: { cpus: 1, memoryMB: 1024, pids: 400 } },
  { label: 'No limit', hint: 'Use whatever it needs', limits: { cpus: 0, memoryMB: 0, pids: 0 } },
];

// How much of the server one container may use. Applies at once; no restart.
export default function LimitsCard({ containerId }: { containerId: string }) {
  const toast = useToast();
  const [limits, setLimits] = useState<Limits | null>(null);
  const [host, setHost] = useState<Host | null>(null);
  const [canChange, setCanChange] = useState(false);
  const [draft, setDraft] = useState({ cpus: '', memoryMB: '', pids: '' });
  const [busy, setBusy] = useState(false);

  const fill = (l: Limits) =>
    setDraft({ cpus: l.cpus ? String(l.cpus) : '', memoryMB: l.memoryMB ? String(l.memoryMB) : '', pids: l.pids ? String(l.pids) : '' });

  const load = useCallback(async () => {
    try {
      const res = await fetch(`/api/containers/${containerId}/limits`, { cache: 'no-store' });
      if (!res.ok) return;
      const data = await res.json();
      setLimits(data.limits);
      setHost(data.host);
      setCanChange(!!data.canChange);
      fill(data.limits);
    } catch {
      /* optional */
    }
  }, [containerId]);

  useEffect(() => {
    void load();
  }, [load]);

  const save = async (l: Limits) => {
    setBusy(true);
    try {
      const res = await fetch(`/api/containers/${containerId}/limits`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(l),
      });
      const data = await res.json().catch(() => ({}));
      if (!res.ok) throw new Error(data.error || 'Could not change the limits');
      setLimits(data.limits);
      fill(data.limits);
      toast.success(l.cpus || l.memoryMB || l.pids ? 'Limits applied.' : 'Limits removed.');
    } catch (err: any) {
      toast.error(err.message || 'Could not change the limits');
    } finally {
      setBusy(false);
    }
  };

  if (!limits || !host) return null;

  const num = (s: string) => (s.trim() === '' ? 0 : Number(s));
  const sentence = (l: Limits) => {
    const parts = [
      l.cpus ? `${l.cpus} of ${host.cpus} CPU core${host.cpus === 1 ? '' : 's'}` : null,
      l.memoryMB ? `${l.memoryMB} MB of ${host.memoryMB} MB memory` : null,
      l.pids ? `${l.pids} processes` : null,
    ].filter(Boolean);
    return parts.length ? `Limited to ${parts.join(', ')}.` : 'No limits: it can use as much of the server as it needs.';
  };
  const field = (label: string, key: 'cpus' | 'memoryMB' | 'pids', unit: string, hint: string) => (
    <label className="block text-xs" style={{ color: 'var(--text-secondary)' }}>
      <span className="font-bold" style={{ color: 'var(--text-primary)' }}>{label}</span> <span>({hint})</span>
      <div className="flex items-center gap-2 mt-1">
        <input
          inputMode="decimal"
          value={draft[key]}
          disabled={!canChange}
          placeholder="no limit"
          onChange={(e) => setDraft({ ...draft, [key]: e.target.value })}
          className="w-32 px-3 py-2 rounded-lg font-mono text-sm"
          style={{ background: 'var(--bg-darker)', color: 'var(--text-primary)', border: '1px solid rgba(var(--neon-purple-rgb), 0.4)' }}
        />
        <span>{unit}</span>
      </div>
    </label>
  );

  return (
    <div className="card-vapor p-6 rounded-xl">
      <h3 className="text-xl font-bold neon-text mb-1 flex items-center gap-2" style={{ color: 'var(--text-pink)' }}>
        <Gauge size={18} weight="regular" />
        Limits
      </h3>
      <p className="text-xs mb-4" style={{ color: 'var(--text-secondary)' }}>
        {sentence(limits)} Changes apply at once, without a restart.
      </p>
      {canChange ? (
        <div className="space-y-4">
          <div className="flex flex-wrap gap-2" role="group" aria-label="Quick choices">
            {PRESETS.map((p) => (
              <button key={p.label} type="button" disabled={busy} title={p.hint} onClick={() => save(p.limits)} className="btn-neon px-3 py-1 text-xs font-bold">
                {p.label}
              </button>
            ))}
          </div>
          <div className="flex flex-wrap gap-4">
            {field('CPU', 'cpus', 'cores', `this server has ${host.cpus}`)}
            {field('Memory', 'memoryMB', 'MB', `this server has ${host.memoryMB}`)}
            {field('Processes', 'pids', 'at once', 'stops runaway programs')}
          </div>
          <button
            type="button"
            disabled={busy}
            onClick={() => save({ cpus: num(draft.cpus), memoryMB: num(draft.memoryMB), pids: num(draft.pids) })}
            className="btn-neon px-4 py-2 text-sm font-bold"
          >
            {busy ? 'Working…' : 'Apply'}
          </button>
          <p className="text-xs" style={{ color: 'var(--text-secondary)' }}>
            A container that goes over its memory limit is stopped by the server and restarted, so don’t set it lower than the site needs.
          </p>
        </div>
      ) : (
        <p className="text-xs" style={{ color: 'var(--text-secondary)' }}>Only an admin can change limits.</p>
      )}
    </div>
  );
}
