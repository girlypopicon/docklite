'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import { ArrowClockwise, ArrowFatUp, CheckCircle, WarningCircle, ArrowSquareOut } from '@phosphor-icons/react';

interface UpdateStatus {
  version: string;
  latestVersion: string;
  latestTag: string;
  updateAvailable: boolean;
  notes: string;
  releaseUrl: string;
  checkError?: string;
  updateRunning: boolean;
  state: 'idle' | 'running' | 'success' | 'failed' | 'rolled-back';
  stateTag?: string;
  stateMessage?: string;
  log: string[];
  lastUpdated: string;
}

/** Settings → System: shows the installed and newest DockLite versions and updates with one button.
 *  The update runs on the server as its own job (DockLite restarts partway through), so this card keeps
 *  asking for progress and simply waits through the restart. */
export default function UpdateCard() {
  const [status, setStatus] = useState<UpdateStatus | null>(null);
  const [loading, setLoading] = useState(false);
  const [starting, setStarting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [reconnecting, setReconnecting] = useState(false);
  const [target, setTarget] = useState<string | null>(null); // the version we asked for, e.g. "1.2.0"
  const [showLog, setShowLog] = useState(false);
  const logRef = useRef<HTMLDivElement>(null);
  const waiting = starting || status?.updateRunning || reconnecting;

  const load = useCallback(async (refresh = false) => {
    setLoading(true);
    try {
      const res = await fetch(`/api/system/update/status${refresh ? '?refresh=1' : ''}`, { cache: 'no-store' });
      if (res.ok) {
        setStatus(await res.json());
        setReconnecting(false);
      } else if (res.status === 403) {
        setError('Only admins can see update information.');
      } else {
        throw new Error(`status ${res.status}`);
      }
    } catch {
      // While updating, DockLite restarts: that is expected, so keep waiting instead of showing an error.
      setReconnecting(true);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  // Poll while an update is in flight, including through the restart.
  useEffect(() => {
    if (!waiting) return;
    const id = setInterval(() => load(), 3000);
    return () => clearInterval(id);
  }, [waiting, load]);

  // When the new version is up, reload so the browser picks up the new dashboard.
  useEffect(() => {
    if (status?.state === 'success' && target && status.version === target) {
      const id = setTimeout(() => window.location.reload(), 2500);
      return () => clearTimeout(id);
    }
  }, [status?.state, status?.version, target]);

  useEffect(() => {
    if (logRef.current) logRef.current.scrollTop = logRef.current.scrollHeight;
  }, [status?.log, showLog]);

  const startUpdate = async () => {
    if (!status) return;
    const ok = window.confirm(
      `Update DockLite from ${status.version} to ${status.latestVersion}?\n\n` +
        'Your sites keep running. The dashboard restarts for about a minute and this page updates by itself.\n' +
        'A safety backup of your DockLite data is made first, and if the new version does not start properly it goes back automatically.'
    );
    if (!ok) return;
    setError(null);
    setStarting(true);
    setTarget(status.latestVersion);
    setShowLog(true);
    try {
      const res = await fetch('/api/system/update/run', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({}) });
      const data = await res.json().catch(() => ({}));
      if (!res.ok) throw new Error(data.error || 'Could not start the update');
      await load();
    } catch (err: any) {
      setError(err.message || 'Could not start the update');
      setTarget(null);
    } finally {
      setStarting(false);
    }
  };

  const finishedOk = status?.state === 'success' && !!target && status.version === target;
  const failed = !waiting && (status?.state === 'failed' || status?.state === 'rolled-back') && !!target;

  return (
    <div className="card-vapor p-6 rounded-xl">
      <div className="flex items-center justify-between mb-6">
        <h2 className="text-2xl font-bold neon-text flex items-center gap-2" style={{ color: 'var(--neon-cyan)' }}>
          <ArrowFatUp size={20} weight="duotone" />
          DockLite Updates
        </h2>
        <button
          onClick={() => load(true)}
          disabled={loading || !!waiting}
          className="p-2 rounded-lg transition-all hover:scale-105 disabled:opacity-50"
          style={{ color: 'var(--text-secondary)' }}
          title="Check for updates"
        >
          <ArrowClockwise size={16} weight="duotone" className={loading ? 'animate-spin' : ''} />
        </button>
      </div>

      {!status && !error && (
        <div className="text-sm opacity-60 flex items-center gap-2">
          <ArrowClockwise size={14} className="animate-spin" /> {reconnecting ? 'Waiting for DockLite to come back…' : 'Checking for updates…'}
        </div>
      )}

      {status && (
        <>
          <div className="grid grid-cols-2 gap-4 mb-5">
            <div className="p-3 rounded-lg" style={{ background: 'var(--surface-muted)' }}>
              <div className="text-xs opacity-60 mb-1">Installed</div>
              <div className="font-bold font-mono" style={{ color: 'var(--neon-cyan)' }}>{status.version}</div>
            </div>
            <div className="p-3 rounded-lg" style={{ background: 'var(--surface-muted)' }}>
              <div className="text-xs opacity-60 mb-1">Newest release</div>
              <div className="font-bold font-mono" style={{ color: status.updateAvailable ? 'var(--neon-yellow)' : 'var(--neon-green)' }}>
                {status.latestVersion || '—'}
              </div>
            </div>
          </div>

          {waiting ? (
            <div className="mb-4 p-3 rounded-lg text-sm" style={{ border: '1px solid var(--neon-cyan)', color: 'var(--text-primary)' }}>
              <div className="font-bold flex items-center gap-2" style={{ color: 'var(--neon-cyan)' }}>
                <ArrowClockwise size={16} className="animate-spin" />
                {reconnecting ? 'DockLite is restarting…' : `Updating to ${target || status.latestVersion}…`}
              </div>
              <div className="text-xs mt-1" style={{ color: 'var(--text-secondary)' }}>
                {reconnecting
                  ? 'This is expected. This page reconnects by itself in a moment. Your sites are not affected.'
                  : 'Downloading and building the new version takes a few minutes. Keep this page open, or come back later.'}
              </div>
            </div>
          ) : finishedOk ? (
            <div className="mb-4 flex items-center gap-2 text-sm font-bold" style={{ color: 'var(--neon-green)' }}>
              <CheckCircle size={18} weight="duotone" /> Updated to {status.version}. Reloading…
            </div>
          ) : failed ? (
            <div className="mb-4 p-3 rounded-lg text-sm" style={{ border: '1px solid var(--status-warning)' }}>
              <div className="font-bold flex items-center gap-2" style={{ color: 'var(--status-warning)' }}>
                <WarningCircle size={18} weight="duotone" />
                {status.state === 'rolled-back' ? 'The update did not work, so DockLite went back to the version that did.' : 'The update did not finish.'}
              </div>
              <div className="text-xs mt-1" style={{ color: 'var(--text-secondary)' }}>{status.stateMessage}</div>
            </div>
          ) : status.checkError ? (
            <div className="mb-4 flex items-center gap-2 text-sm" style={{ color: 'var(--status-warning)' }}>
              <WarningCircle size={18} weight="duotone" /> Couldn’t check for updates: {status.checkError}
            </div>
          ) : status.version === 'unknown' ? (
            <div className="mb-4 flex items-center gap-2 text-sm" style={{ color: 'var(--status-warning)' }}>
              <WarningCircle size={18} weight="duotone" /> Couldn’t read the installed version, so I can’t tell whether you’re up to date.
            </div>
          ) : status.updateAvailable ? (
            <div className="mb-4 flex items-center gap-2 text-sm font-bold" style={{ color: 'var(--neon-yellow)' }}>
              <WarningCircle size={18} weight="duotone" /> Version {status.latestVersion} is available
            </div>
          ) : (
            <div className="mb-4 flex items-center gap-2 text-sm font-bold" style={{ color: 'var(--neon-green)' }}>
              <CheckCircle size={18} weight="duotone" /> You have the newest version
            </div>
          )}

          {status.updateAvailable && !waiting && !finishedOk && status.notes && (
            <div className="mb-4">
              <div className="text-xs font-bold mb-2 opacity-60">What’s new in {status.latestVersion}</div>
              <div className="rounded-lg p-3 text-xs whitespace-pre-wrap max-h-48 overflow-y-auto" style={{ background: 'var(--bg-darker)', color: 'var(--text-secondary)' }}>
                {status.notes}
              </div>
            </div>
          )}

          <div className="flex items-center gap-3 flex-wrap">
            {status.updateAvailable && !finishedOk && (
              <button
                onClick={startUpdate}
                disabled={!!waiting}
                className="px-6 py-2 rounded-lg font-bold transition-all hover:scale-105 disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-2"
                style={{
                  background: waiting ? 'rgba(var(--text-muted-rgb), 0.3)' : 'linear-gradient(135deg, var(--neon-cyan) 0%, var(--neon-purple) 100%)',
                  color: 'var(--button-text)',
                }}
              >
                <ArrowFatUp size={16} weight="duotone" className={waiting ? 'animate-bounce' : ''} />
                {waiting ? 'Updating…' : `Update to ${status.latestVersion}`}
              </button>
            )}
            {status.releaseUrl && (
              <a href={status.releaseUrl} target="_blank" rel="noreferrer" className="text-xs underline flex items-center gap-1" style={{ color: 'var(--neon-cyan)' }}>
                Release notes <ArrowSquareOut size={12} />
              </a>
            )}
            {status.log && status.log.length > 0 && (
              <button type="button" className="text-xs underline" style={{ color: 'var(--text-secondary)' }} onClick={() => setShowLog((v) => !v)}>
                {showLog ? 'Hide update log' : 'Show update log'}
              </button>
            )}
          </div>

          {error && (
            <div className="mt-4 px-4 py-2 rounded-lg text-sm" style={{ background: 'rgba(var(--status-error-rgb), 0.15)', color: 'var(--status-error)', border: '1px solid var(--status-error)' }}>
              {error}
            </div>
          )}

          {showLog && status.log && status.log.length > 0 && (
            <div className="mt-4">
              <div ref={logRef} className="rounded-lg p-3 font-mono text-xs overflow-y-auto max-h-56 space-y-0.5" style={{ background: 'var(--bg-darker)', color: 'var(--text-secondary)' }}>
                {status.log.map((line, i) => (
                  <div key={i} style={{ color: /fail|error/i.test(line) ? 'var(--status-error)' : undefined }}>{line}</div>
                ))}
              </div>
              {status.lastUpdated && <div className="text-xs opacity-40 mt-1">Last run: {new Date(status.lastUpdated).toLocaleString()}</div>}
            </div>
          )}
        </>
      )}

      {!status && error && (
        <div className="px-4 py-2 rounded-lg text-sm" style={{ color: 'var(--status-error)', border: '1px solid var(--status-error)' }}>{error}</div>
      )}
    </div>
  );
}
