'use client';

import { useCallback, useEffect, useState } from 'react';
import { ClockCounterClockwise, LockSimple, Play } from '@phosphor-icons/react';
import CloseButton from './CloseButton';
import { useEscape } from '@/lib/hooks/useEscape';
import { useToast } from '@/lib/hooks/useToast';

interface Destination {
  id: number;
  name: string;
  type: string;
  enabled: number;
  config?: string;
}

interface Plan {
  id: number;
  what?: string;
  schedule_text?: string;
  destination_name?: string;
  next_run_at?: string;
  enabled: number;
  selection_data?: { containers?: string[]; groups?: number[] };
}

const TYPE_NAMES: Record<string, string> = { local: 'this server', s3: 'bucket', sftp: 'another server', server: 'DockLite server' };
const WEEKDAYS = [
  ['sun', 'Sunday'], ['mon', 'Monday'], ['tue', 'Tuesday'], ['wed', 'Wednesday'], ['thu', 'Thursday'], ['fri', 'Friday'], ['sat', 'Saturday'],
];

const isLocked = (d: Destination) => {
  try {
    return !!JSON.parse(d.config || '{}').encrypt;
  } catch {
    return false;
  }
};

/** Backups are an admin thing: the destinations list answers 403 to anyone else. */
export function useCanManageBackups() {
  const [can, setCan] = useState(false);
  useEffect(() => {
    let live = true;
    fetch('/api/backups/destinations', { cache: 'no-store' })
      .then((r) => live && setCan(r.ok))
      .catch(() => {});
    return () => {
      live = false;
    };
  }, []);
  return can;
}

interface Props {
  // A container is backed up by name; a group by id (so containers added to it later are included).
  kind: 'container' | 'group';
  name: string;
  groupId?: number;
  onClose: () => void;
}

/** Make a backup plan for one container or one group: where it goes, when, and how many to keep. */
export default function BackupPlanModal({ kind, name, groupId, onClose }: Props) {
  useEscape(onClose);
  const toast = useToast();
  const [destinations, setDestinations] = useState<Destination[]>([]);
  const [plans, setPlans] = useState<Plan[]>([]);
  const [destinationId, setDestinationId] = useState(0);
  const [when, setWhen] = useState<'daily' | 'weekly'>('daily');
  const [time, setTime] = useState('02:00');
  const [weekday, setWeekday] = useState('sun');
  const [keep, setKeep] = useState(7);
  const [busy, setBusy] = useState<'' | 'plan' | 'now'>('');
  const [error, setError] = useState('');

  const selection = kind === 'group' ? { groups: [groupId] } : { containers: [name] };

  const load = useCallback(async () => {
    try {
      const [d, j] = await Promise.all([
        fetch('/api/backups/destinations', { cache: 'no-store' }),
        fetch('/api/backups/jobs', { cache: 'no-store' }),
      ]);
      if (d.ok) {
        const list: Destination[] = ((await d.json()).destinations || []).filter((x: Destination) => x.enabled);
        setDestinations(list);
        setDestinationId((cur) => cur || list[0]?.id || 0);
      }
      if (j.ok) {
        const jobs: Plan[] = (await j.json()).jobs || [];
        setPlans(
          jobs.filter((p) =>
            kind === 'group' ? p.selection_data?.groups?.includes(groupId as number) : p.selection_data?.containers?.includes(name),
          ),
        );
      }
    } catch {
      /* the page still works without the list */
    }
  }, [kind, name, groupId]);

  useEffect(() => {
    void load();
  }, [load]);

  const post = async (url: string, body: unknown) => {
    const res = await fetch(url, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) });
    const data = await res.json().catch(() => ({}));
    if (!res.ok) throw new Error(data.error || 'Something went wrong');
    return data;
  };

  const createPlan = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy('plan');
    setError('');
    try {
      const frequency = (when === 'daily' ? `daily@${time}` : `weekly@${weekday}@${time}`) + (keep > 0 ? `;keep=${keep}` : '');
      await post('/api/backups/jobs', { destination_id: destinationId, selection, frequency, retention_days: 0, enabled: 1 });
      toast.success('Backup plan created.');
      await load();
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setBusy('');
    }
  };

  const runNow = async () => {
    setBusy('now');
    setError('');
    try {
      await post('/api/backups/export', { selection, delivery: 'local', destination_id: destinationId });
      toast.success('Backup started. Follow it on the Backups page.');
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setBusy('');
    }
  };

  const chosen = destinations.find((d) => d.id === destinationId);
  const noun = kind === 'group' ? 'group' : 'container';

  return (
    <div className="fixed inset-0 z-[9100] flex items-center justify-center p-3 sm:p-6 bg-black/75 backdrop-blur-sm" onClick={onClose}>
      <div
        className="card-vapor w-full max-w-lg max-h-[92vh] overflow-y-auto p-5 sm:p-6 relative"
        style={{ background: 'linear-gradient(135deg, var(--modal-bg-1) 0%, var(--modal-bg-2) 100%)', border: '2px solid var(--modal-border)', boxShadow: '0 0 30px var(--modal-shadow)' }}
        onClick={(e) => e.stopPropagation()}
        role="dialog"
        aria-label={`Back up ${name}`}
      >
        <CloseButton onClick={onClose} className="absolute top-3 right-3" />
        <h2 className="text-xl font-bold pr-10 flex items-center gap-2" style={{ color: 'var(--text-primary)' }}>
          <ClockCounterClockwise size={24} style={{ color: 'var(--text-cyan)' }} /> Back up {name}
        </h2>
        <p className="text-sm mt-1 mb-4" style={{ color: 'var(--text-secondary)' }}>
          {kind === 'group'
            ? 'Everything in this group goes into one backup file, including containers you add to it later.'
            : 'A website comes with its databases. A database container is saved as its own dump.'}
        </p>

        {plans.length > 0 && (
          <div className="mb-4 rounded-lg p-3 text-sm" style={{ border: '1px solid var(--modal-border)' }}>
            <div className="font-bold mb-1" style={{ color: 'var(--text-primary)' }}>Plans for this {noun}</div>
            <ul className="space-y-1" style={{ color: 'var(--text-secondary)' }}>
              {plans.map((p) => (
                <li key={p.id}>
                  {p.schedule_text}
                  {p.destination_name ? ` → ${p.destination_name}` : ''}
                  {p.enabled === 0 ? ' (paused)' : ''}
                </li>
              ))}
            </ul>
            <p className="text-xs mt-2" style={{ color: 'var(--text-secondary)' }}>Change or remove plans on the Backups page.</p>
          </div>
        )}

        <form onSubmit={createPlan} className="space-y-4">
          <label className="block">
            <span className="block text-sm font-bold mb-1" style={{ color: 'var(--text-pink)' }}>Where it goes</span>
            <select value={destinationId} onChange={(e) => setDestinationId(Number(e.target.value))} className="input-vapor w-full">
              {destinations.length === 0 && <option value={0}>This server</option>}
              {destinations.map((d) => (
                <option key={d.id} value={d.id}>
                  {d.name} ({TYPE_NAMES[d.type] || d.type}){isLocked(d) ? ' – locked' : ''}
                </option>
              ))}
            </select>
            {chosen && isLocked(chosen) && (
              <span className="text-xs mt-1 flex items-center gap-1" style={{ color: 'var(--text-green)' }}>
                <LockSimple size={12} weight="bold" /> Locked with your backup key before it leaves this server.
              </span>
            )}
            {chosen && chosen.type === 'local' && !isLocked(chosen) && destinations.length > 1 && (
              <span className="text-xs mt-1 block" style={{ color: 'var(--status-warning)' }}>
                This keeps the backup on this server only. Add a bucket or second server on the Backups page to keep a copy elsewhere.
              </span>
            )}
          </label>

          <div className="flex flex-wrap gap-3 items-end">
            <label className="block">
              <span className="block text-sm font-bold mb-1" style={{ color: 'var(--text-pink)' }}>How often</span>
              <select value={when} onChange={(e) => setWhen(e.target.value as 'daily' | 'weekly')} className="input-vapor">
                <option value="daily">Every day</option>
                <option value="weekly">Every week</option>
              </select>
            </label>
            {when === 'weekly' && (
              <select value={weekday} onChange={(e) => setWeekday(e.target.value)} className="input-vapor" aria-label="Day of the week">
                {WEEKDAYS.map(([v, l]) => (
                  <option key={v} value={v}>{l}</option>
                ))}
              </select>
            )}
            <label className="block">
              <span className="block text-sm font-bold mb-1" style={{ color: 'var(--text-pink)' }}>At</span>
              <input type="time" value={time} onChange={(e) => setTime(e.target.value)} className="input-vapor" required />
            </label>
            <label className="block">
              <span className="block text-sm font-bold mb-1" style={{ color: 'var(--text-pink)' }}>Keep the newest</span>
              <input type="number" min={0} max={1000} value={keep} onChange={(e) => setKeep(Number(e.target.value))} className="input-vapor w-24" />
            </label>
          </div>
          <p className="text-xs" style={{ color: 'var(--text-secondary)' }}>
            Server time. Older backups are deleted so only the newest {keep > 0 ? keep : 'few'} stay{keep === 0 ? ' (0 keeps them all)' : ''}.
          </p>

          {error && (
            <div className="text-sm rounded-lg p-3" style={{ color: 'var(--status-error)', border: '1px solid var(--status-error)' }} role="alert">
              {error}
            </div>
          )}

          <div className="flex flex-wrap gap-3">
            <button type="submit" className="cyber-button inline-flex items-center gap-2" disabled={busy !== ''}>
              <ClockCounterClockwise size={16} /> {busy === 'plan' ? 'Creating…' : 'Create plan'}
            </button>
            <button type="button" className="btn-neon px-4 py-2 text-sm font-bold inline-flex items-center gap-2" onClick={runNow} disabled={busy !== ''}>
              <Play size={16} /> {busy === 'now' ? 'Starting…' : 'Back up now'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
