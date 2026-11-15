'use client';

import { useState, useEffect } from 'react';
import CloseButton from './CloseButton';
import { ClockCounterClockwise, WarningCircle } from '@phosphor-icons/react';
import BackupPicker, { BackupSelection, selectionIsEmpty } from './BackupPicker';

interface AddBackupJobModalProps {
  destinations: any[];
  onClose: () => void;
  onSuccess: () => void;
  initialJob?: any;
  mode?: 'create' | 'edit';
  // Shown in the page itself (the Backups page's first screen when there is no schedule yet) instead of as a dialog.
  inline?: boolean;
}

const WEEKDAYS = [
  { value: 'sun', label: 'Sunday' },
  { value: 'mon', label: 'Monday' },
  { value: 'tue', label: 'Tuesday' },
  { value: 'wed', label: 'Wednesday' },
  { value: 'thu', label: 'Thursday' },
  { value: 'fri', label: 'Friday' },
  { value: 'sat', label: 'Saturday' },
];

type ScheduleKind = 'daily' | 'weekly' | 'hours';

// The schedule is stored as text in the job's "frequency": daily@03:00, weekly@sun@03:00, 6 (hours), with ";keep=7" for
// "keep only the newest 7". Older jobs (daily, hourly, every-3-days, ...) are read too; see go-app/internal/backup/schedule.go.
function parseFrequency(freq: string | undefined): { kind: ScheduleKind; time: string; weekday: string; hours: number; keep: number } {
  const out = { kind: 'daily' as ScheduleKind, time: '03:00', weekday: 'sun', hours: 6, keep: 7 };
  if (!freq) return out;
  const [main, opts = ''] = freq.split(';');
  const keep = /keep=(\d+)/.exec(opts);
  if (keep) out.keep = Number(keep[1]);
  const parts = main.split('@');
  if (parts[0] === 'weekly') {
    out.kind = 'weekly';
    if (parts.length === 3) {
      out.weekday = parts[1];
      out.time = parts[2];
    }
  } else if (parts[0] === 'daily') {
    if (parts[1]) out.time = parts[1];
  } else if (parts[0] === 'hourly') {
    out.kind = 'hours';
    out.hours = 1;
  } else {
    const n = Number(parts[0].replace(/^every-/, '').replace(/-hours$/, ''));
    if (n > 0) {
      out.kind = 'hours';
      out.hours = n;
    }
  }
  return out;
}

// The agent sends a job's destination as destination_id; older code here read "destination".
function jobDestination(job: any): number | undefined {
  return job?.destination_id ?? job?.destination;
}

export default function AddBackupJobModal({ destinations, onClose, onSuccess, initialJob, mode = 'create', inline = false }: AddBackupJobModalProps) {
  const [destinationId, setDestinationId] = useState(jobDestination(initialJob) || destinations[0]?.id || 0);
  // Schedules made before packages have no selection of their own; the agent works one out from their old target.
  const [selection, setSelection] = useState<BackupSelection>(initialJob?.selection_data || { all_sites: true, all_databases: true });
  const initialSchedule = parseFrequency(initialJob?.frequency);
  const [kind, setKind] = useState<ScheduleKind>(initialSchedule.kind);
  const [time, setTime] = useState(initialSchedule.time);
  const [weekday, setWeekday] = useState(initialSchedule.weekday);
  const [everyHours, setEveryHours] = useState(initialSchedule.hours);
  const [keep, setKeep] = useState(initialSchedule.keep);
  const [enabled, setEnabled] = useState(initialJob ? initialJob.enabled === 1 : true);

  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  useEffect(() => {
    if (!initialJob) return;
    setDestinationId(jobDestination(initialJob) || destinations[0]?.id || 0);
    if (initialJob.selection_data) setSelection(initialJob.selection_data);
    const sched = parseFrequency(initialJob.frequency);
    setKind(sched.kind);
    setTime(sched.time);
    setWeekday(sched.weekday);
    setEveryHours(sched.hours);
    setKeep(sched.keep);
    setEnabled(initialJob.enabled === 1);
  }, [initialJob, destinations]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    if (selectionIsEmpty(selection)) {
      setError('Tick at least one site or database');
      return;
    }
    setLoading(true);

    try {
      const isEdit = mode === 'edit' && initialJob?.id;
      const payload: any = {
        destination_id: destinationId,
        selection,
        frequency:
          (kind === 'daily' ? `daily@${time}` : kind === 'weekly' ? `weekly@${weekday}@${time}` : String(everyHours)) +
          (keep > 0 ? `;keep=${keep}` : ''),
        retention_days: 0, // old-backup cleanup is by count now ("keep the newest N"), not by age
        enabled: enabled ? 1 : 0,
      };

      if (isEdit) {
        payload.id = initialJob.id;
      }

      const res = await fetch('/api/backups/jobs', {
        method: isEdit ? 'PUT' : 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload)
      });

      const data = await res.json();

      if (!res.ok) {
        throw new Error(data.error || 'Failed to create backup job');
      }

      onSuccess();
      onClose();
    } catch (err: any) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  const title = inline ? 'Back up automatically' : mode === 'edit' ? 'Edit Schedule' : 'Create Schedule';
  const body = (
      <>
        <div className="mb-6">
          <h2 className="text-2xl font-bold neon-text flex items-center gap-3" style={{ color: 'var(--text-cyan)' }}>
            <ClockCounterClockwise size={32} weight="duotone" color="var(--status-success)" />
            {title}
          </h2>
          <p className="text-sm mt-2" style={{ color: 'var(--text-secondary)' }}>
            {inline
              ? 'Pick what to protect and when. Everything is ticked to start with; most servers only ever need this one schedule.'
              : 'Schedule automated backups for your sites and databases'}
          </p>
        </div>

        {error && (
          <div
            className="mb-4 p-3 rounded-lg border"
            style={{
              background: 'rgba(var(--status-error-rgb), 0.1)',
              border: '1px solid var(--status-error)',
              color: 'var(--status-error)',
            }}
          >
            {error}
          </div>
        )}

        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="block text-sm font-bold mb-2" style={{ color: 'var(--text-pink)' }}>
              Backup Destination
            </label>
            <select
              value={destinationId}
              onChange={(e) => setDestinationId(Number(e.target.value))}
              className="input-vapor w-full"
              required
              disabled={loading || destinations.length === 0}
            >
              {destinations.map((dest) => (
                <option key={dest.id} value={dest.id}>
                  {dest.name} ({dest.type.toUpperCase()})
                </option>
              ))}
            </select>
            {destinations.length === 0 && (
              <p className="text-xs mt-1 flex items-center gap-2" style={{ color: 'var(--status-warning)' }}>
                <WarningCircle size={14} weight="duotone" />
                No destinations configured. A local destination will be used automatically.
              </p>
            )}
          </div>

          <div>
            <label className="block text-sm font-bold mb-2" style={{ color: 'var(--text-pink)' }}>
              What to back up
            </label>
            <BackupPicker value={selection} onChange={setSelection} disabled={loading} forSchedule />
          </div>

          <div className="space-y-3">
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
              <div>
                <label className="block text-sm font-bold mb-2" style={{ color: 'var(--text-pink)' }}>
                  How often
                </label>
                <select value={kind} onChange={(e) => setKind(e.target.value as ScheduleKind)} className="input-vapor w-full" disabled={loading}>
                  <option value="daily">Every day</option>
                  <option value="weekly">Every week</option>
                  <option value="hours">Every few hours</option>
                </select>
              </div>
              {kind === 'weekly' && (
                <div>
                  <label className="block text-sm font-bold mb-2" style={{ color: 'var(--text-pink)' }}>
                    On
                  </label>
                  <select value={weekday} onChange={(e) => setWeekday(e.target.value)} className="input-vapor w-full" disabled={loading}>
                    {WEEKDAYS.map((d) => (
                      <option key={d.value} value={d.value}>
                        {d.label}
                      </option>
                    ))}
                  </select>
                </div>
              )}
              {kind !== 'hours' ? (
                <div>
                  <label className="block text-sm font-bold mb-2" style={{ color: 'var(--text-pink)' }}>
                    At (server time)
                  </label>
                  <input type="time" value={time} onChange={(e) => setTime(e.target.value || '03:00')} className="input-vapor w-full" required disabled={loading} />
                </div>
              ) : (
                <div>
                  <label className="block text-sm font-bold mb-2" style={{ color: 'var(--text-pink)' }}>
                    Every
                  </label>
                  <select value={everyHours} onChange={(e) => setEveryHours(Number(e.target.value))} className="input-vapor w-full" disabled={loading}>
                    {[1, 2, 3, 4, 6, 8, 12].map((h) => (
                      <option key={h} value={h}>
                        {h === 1 ? 'hour' : `${h} hours`}
                      </option>
                    ))}
                  </select>
                </div>
              )}
            </div>
            <div>
              <label className="block text-sm font-bold mb-2" style={{ color: 'var(--text-pink)' }}>
                Keep the newest
              </label>
              <select value={keep} onChange={(e) => setKeep(Number(e.target.value))} className="input-vapor w-full" disabled={loading}>
                {[1, 3, 5, 7, 14, 30, 60, 90].map((n) => (
                  <option key={n} value={n}>
                    {n} {n === 1 ? 'backup' : 'backups'}
                  </option>
                ))}
                <option value={0}>All of them (never delete)</option>
              </select>
              <p className="text-xs mt-1" style={{ color: 'var(--text-secondary)' }}>
                After each backup, older ones beyond this number are deleted so the disk doesn&apos;t fill up. If the server was off at the chosen time, the backup runs when it&apos;s back.
              </p>
            </div>
          </div>

          <div className="flex items-center gap-3 p-3 rounded-lg" style={{ background: 'rgba(var(--status-success-rgb), 0.1)' }}>
            <input
              type="checkbox"
              id="enabled"
              checked={enabled}
              onChange={(e) => setEnabled(e.target.checked)}
              className="w-4 h-4"
              disabled={loading}
            />
            <label htmlFor="enabled" className="text-sm font-bold" style={{ color: 'var(--text-green)' }}>
              Enable this backup job
            </label>
          </div>

          <div className="flex gap-3 pt-4">
            {!inline && <button
              type="button"
              onClick={onClose}
              disabled={loading}
              className="flex-1 px-4 py-3 rounded-lg font-bold transition-all hover:scale-105"
              style={{
                background: 'rgba(var(--text-muted-rgb), 0.1)',
                border: '1px solid rgba(var(--text-muted-rgb), 0.2)',
                color: 'var(--text-secondary)',
              }}
            >
              Cancel
            </button>}
            <button
              type="submit"
              disabled={loading || destinations.length === 0}
              className="flex-1 px-4 py-3 rounded-lg font-bold transition-all hover:scale-105 disabled:opacity-50 disabled:cursor-not-allowed"
              style={{
                background: 'linear-gradient(135deg, var(--neon-cyan) 0%, var(--neon-purple) 100%)',
                color: 'var(--button-text)',
                boxShadow: '0 0 20px rgba(var(--status-success-rgb), 0.4)',
              }}
            >
              {loading ? (mode === 'edit' ? 'Saving...' : 'Creating...') : mode === 'edit' ? 'Save Schedule' : inline ? 'Start backing up' : 'Create Schedule'}
            </button>
          </div>
        </form>
      </>
  );

  if (inline) return <div className="card-vapor p-4 sm:p-6 border border-neon-purple/40">{body}</div>;

  return (
    <div className="fixed inset-0 z-[10000] flex items-center justify-center p-4 bg-black/70 backdrop-blur-sm">
      <div
        className="card-vapor max-w-2xl w-full p-4 sm:p-6 relative max-h-[90vh] overflow-y-auto"
        style={{
          background: 'linear-gradient(135deg, var(--modal-bg-1) 0%, var(--modal-bg-2) 100%)',
          border: '2px solid var(--modal-border)',
          boxShadow: '0 0 30px var(--modal-shadow)',
        }}
      >
        <CloseButton onClick={onClose} className="absolute top-4 right-4" />
        {body}
      </div>
    </div>
  );
}
