'use client';

import { useCallback, useEffect, useState } from 'react';
import { ShieldCheck, ArrowCounterClockwise, WarningCircle } from '@phosphor-icons/react';
import { useToast } from '@/lib/hooks/useToast';

interface BackupFile {
  name: string;
  size: number;
  modified_at: string;
}

interface Inspection {
  domain?: string;
  problem?: string;
  files: number;
  bytes: number;
  verified: boolean;
  skipped?: string[];
}

function formatSize(bytes: number) {
  if (bytes < 1024) return `${bytes} B`;
  const units = ['KB', 'MB', 'GB', 'TB'];
  let value = bytes / 1024;
  let i = 0;
  while (value >= 1024 && i < units.length - 1) {
    value /= 1024;
    i++;
  }
  return `${value.toFixed(1)} ${units[i]}`;
}

// "sites/site-example.com-20261011-030000.tar.gz" -> "example.com"
function siteName(file: string) {
  const base = file.split('/').pop() || file;
  const m = /^site-(.+)-\d{8}-\d{6}\.tar\.gz$/.exec(base);
  return m ? m[1] : base;
}

export default function RestoreSiteBackups({ focusPath }: { focusPath?: string }) {
  const toast = useToast();
  const [files, setFiles] = useState<BackupFile[]>([]);
  const [loading, setLoading] = useState(true);
  const [picked, setPicked] = useState<BackupFile | null>(null);
  const [inspection, setInspection] = useState<Inspection | null>(null);
  const [inspectError, setInspectError] = useState('');
  const [working, setWorking] = useState(false);
  const [done, setDone] = useState<{ domain: string; files: number; safety?: string } | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const res = await fetch('/api/backups/local', { cache: 'no-store' });
      const data = res.ok ? await res.json() : { files: [] };
      setFiles((data.files || []).filter((f: BackupFile) => f.name.startsWith('sites/') && f.name.endsWith('.tar.gz')));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  // Arriving from a backup's own Restore button: open that backup's confirmation right away.
  useEffect(() => {
    if (!focusPath || files.length === 0) return;
    const norm = focusPath.replace(/\\/g, '/');
    const match = files.find((f) => norm.endsWith('/' + f.name) || norm === f.name);
    if (match) choose(match);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [focusPath, files]);

  const choose = async (file: BackupFile) => {
    setPicked(file);
    setInspection(null);
    setInspectError('');
    setDone(null);
    try {
      const res = await fetch(`/api/backups/inspect?file=${encodeURIComponent(file.name)}`, { cache: 'no-store' });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || 'Could not read this backup');
      setInspection(data);
    } catch (err: any) {
      setInspectError(err.message || 'Could not read this backup');
    }
  };

  const restore = async () => {
    if (!picked) return;
    setWorking(true);
    try {
      const res = await fetch('/api/backups/restore', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ file: picked.name }),
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || 'Restore failed');
      setDone({ domain: data.domain, files: data.files, safety: data.safety_backup });
      setPicked(null);
      toast.success(`Restored ${data.domain}`);
    } catch (err: any) {
      toast.error(err.message || 'Restore failed');
    } finally {
      setWorking(false);
    }
  };

  const canRestore = !!inspection && !!inspection.domain && inspection.verified && !working;

  return (
    <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
      <div className="lg:col-span-2 card-vapor p-6 border border-neon-purple/30">
        <h2 className="text-xl font-bold text-accent-cyan flex items-center gap-2">
          <ShieldCheck size={22} weight="duotone" /> Restore a site
        </h2>
        <p className="text-sm text-ink-muted mt-2">
          Pick a backup to put a site back the way it was. Before anything changes, DockLite saves what the site has now, so a restore can itself be undone.
        </p>

        {done && (
          <div className="mt-4 rounded-lg p-3 text-sm" style={{ border: '1px solid var(--status-success)' }}>
            <div className="font-bold text-accent-green">Restored {done.domain} ({done.files} files).</div>
            {done.safety && <div className="text-xs text-ink-muted mt-1 break-all">What was there before is saved at {done.safety}</div>}
          </div>
        )}

        <div className="mt-4 space-y-2">
          {loading ? (
            <div className="text-sm text-ink-muted">Loading backups…</div>
          ) : files.length === 0 ? (
            <div className="text-sm text-ink-muted">
              No site backups on this server yet. Make one from the Backups tab (choose “Local”), or set up a schedule.
            </div>
          ) : (
            files.map((f) => (
              <div
                key={f.name}
                className="flex flex-wrap items-center justify-between gap-3 p-3 rounded-lg bg-surface-muted border border-neon-purple/20"
              >
                <div>
                  <div className="font-bold text-accent-pink">{siteName(f.name)}</div>
                  <div className="text-xs text-ink-muted">
                    {new Date(f.modified_at).toLocaleString()} • {formatSize(f.size)}
                  </div>
                </div>
                <button onClick={() => choose(f)} className="cyber-button-sm flex items-center gap-1">
                  <ArrowCounterClockwise size={14} weight="duotone" /> Restore…
                </button>
              </div>
            ))
          )}
        </div>
      </div>

      <div className="card-vapor p-6 border border-neon-purple/20">
        <h3 className="text-lg font-bold text-accent-pink">Good to know</h3>
        <ul className="text-sm text-ink-muted mt-3 space-y-2 list-disc pl-5">
          <li>Restoring replaces the site&apos;s files with the backup&apos;s. Files added since the backup are removed (they stay in the safety copy).</li>
          <li>The site&apos;s container and settings aren&apos;t touched, only its files.</li>
          <li>A backup that fails its check, or that has been changed since it was made, is refused.</li>
        </ul>
      </div>

      {picked && (
        <div className="fixed inset-0 z-[10000] flex items-center justify-center p-4 bg-black/70 backdrop-blur-sm">
          <div className="card-vapor max-w-lg w-full p-6 rounded-2xl" style={{ background: 'linear-gradient(135deg, var(--modal-bg-1) 0%, var(--modal-bg-2) 100%)', border: '2px solid var(--modal-border)', boxShadow: '0 0 30px var(--modal-shadow)' }}>
            <h3 className="text-xl font-bold text-accent-cyan">Restore {siteName(picked.name)}?</h3>
            <div className="mt-3 text-sm text-ink-muted space-y-2">
              {!inspection && !inspectError && <div>Checking the backup…</div>}
              {inspectError && (
                <div className="flex items-start gap-2" style={{ color: 'var(--status-error)' }}>
                  <WarningCircle size={18} weight="duotone" /> {inspectError}
                </div>
              )}
              {inspection && (
                <>
                  <div>
                    This backup has <strong>{inspection.files} files</strong> ({formatSize(inspection.bytes)}), made{' '}
                    {new Date(picked.modified_at).toLocaleString()}.
                  </div>
                  {inspection.domain ? (
                    <div>
                      It will <strong>replace the files of {inspection.domain}</strong> with these. What the site has now is saved first.
                    </div>
                  ) : (
                    <div style={{ color: 'var(--status-error)' }}>{inspection.problem}</div>
                  )}
                  {!inspection.verified && (
                    <div style={{ color: 'var(--status-error)' }}>
                      This file doesn&apos;t match what was recorded when it was made, so it was changed or damaged. It won&apos;t be used.
                    </div>
                  )}
                  {inspection.skipped && inspection.skipped.length > 0 && (
                    <div className="text-xs">Left out: {inspection.skipped.slice(0, 3).join(', ')}{inspection.skipped.length > 3 ? '…' : ''}</div>
                  )}
                </>
              )}
            </div>
            <div className="flex justify-end gap-3 mt-6">
              <button onClick={() => setPicked(null)} className="btn-neon px-4 py-2 font-bold" disabled={working}>
                Cancel
              </button>
              <button onClick={restore} className="btn-neon px-5 py-2 font-bold" disabled={!canRestore}>
                {working ? 'Restoring…' : 'Restore now'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
