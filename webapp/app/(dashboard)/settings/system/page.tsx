'use client';

import { useState, useEffect, useRef } from 'react';
import { DesktopTower, Cube, Gear, Wrench, ArrowClockwise, ArrowFatUp, CheckCircle, WarningCircle } from '@phosphor-icons/react';
import ShellAccessCard from '../../components/ShellAccessCard';

type UpdateStatus = {
  version: string;
  gitHash: string;
  branch: string;
  commitsBehind: number;
  updateAvailable: boolean;
  updateRunning: boolean;
  lastUpdated: string;
  log: string[];
};

export default function SystemSettingsPage() {
  const [checkingFolders, setCheckingFolders] = useState(false);
  const [folderCheckResult, setFolderCheckResult] = useState<string | null>(null);

  const [updateStatus, setUpdateStatus] = useState<UpdateStatus | null>(null);
  const [statusLoading, setStatusLoading] = useState(false);
  const [updating, setUpdating] = useState(false);
  const [updateError, setUpdateError] = useState<string | null>(null);
  const logRef = useRef<HTMLDivElement>(null);
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const fetchStatus = async () => {
    setStatusLoading(true);
    try {
      const res = await fetch('/api/system/update/status');
      if (res.ok) setUpdateStatus(await res.json());
    } finally {
      setStatusLoading(false);
    }
  };

  useEffect(() => {
    fetchStatus();
  }, []);

  useEffect(() => {
    if (logRef.current) {
      logRef.current.scrollTop = logRef.current.scrollHeight;
    }
  }, [updateStatus?.log]);

  useEffect(() => {
    if (updating || updateStatus?.updateRunning) {
      pollRef.current = setInterval(fetchStatus, 3000);
    } else {
      if (pollRef.current) clearInterval(pollRef.current);
    }
    return () => { if (pollRef.current) clearInterval(pollRef.current); };
  }, [updating, updateStatus?.updateRunning]);

  const handleUpdate = async () => {
    setUpdating(true);
    setUpdateError(null);
    try {
      const res = await fetch('/api/system/update/run', { method: 'POST' });
      if (!res.ok) {
        const d = await res.json();
        setUpdateError(d.error || 'Failed to start update');
        setUpdating(false);
        return;
      }
      await fetchStatus();
    } catch (err: any) {
      setUpdateError(err.message);
      setUpdating(false);
    }
  };

  const handleCheckFolders = async () => {
    setCheckingFolders(true);
    setFolderCheckResult(null);

    try {
      const res = await fetch('/api/system/check-folders', { method: 'POST' });
      const data = await res.json();

      if (res.ok) {
        setFolderCheckResult(`✓ Success: Checked ${data.userCount} users`);
      } else {
        setFolderCheckResult(`✗ Error: ${data.error}`);
      }
    } catch (err: any) {
      setFolderCheckResult(`✗ Error: ${err.message}`);
    } finally {
      setCheckingFolders(false);
    }
  };

  return (
    <div className="space-y-8">
      <div>
        <h2 className="text-2xl font-bold neon-text flex items-center gap-2" style={{ color: 'var(--neon-cyan)' }}>
          <DesktopTower size={20} weight="duotone" />
          System Settings
        </h2>
        <p className="text-sm font-mono" style={{ color: 'var(--text-secondary)' }}>
          Docker daemon configuration
        </p>
      </div>

      {/* Docker Status */}
      <div className="card-vapor p-6 rounded-xl">
        <h2 className="text-2xl font-bold neon-text mb-4 flex items-center gap-2" style={{ color: 'var(--neon-green)' }}>
          <Cube size={20} weight="duotone" />
          Docker Connection
        </h2>
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-4">
            <div className="w-4 h-4 rounded-full animate-pulse bg-green-500"></div>
            <div>
              <div className="font-bold text-lg">Connected</div>
              <div className="text-sm opacity-70" style={{ color: 'var(--text-secondary)' }}>
                Docker daemon status
              </div>
            </div>
          </div>
          <div className="text-right">
            <div className="text-sm font-mono opacity-70">Socket: /var/run/docker.sock</div>
            <div className="text-xs opacity-50">Version: API 1.41+</div>
          </div>
        </div>
      </div>

      {/* Configuration Options */}
      <div className="card-vapor p-6 rounded-xl">
        <h2 className="text-2xl font-bold neon-text mb-6 flex items-center gap-2" style={{ color: 'var(--neon-yellow)' }}>
          <Gear size={20} weight="duotone" />
          Configuration
        </h2>
        <div className="space-y-4">
          <div className="flex items-center justify-between p-4 rounded-lg border border-purple-500/20">
            <div>
              <div className="font-bold">Auto-refresh containers</div>
              <div className="text-sm opacity-70">Automatically refresh container status every 10 seconds</div>
            </div>
            <button className="px-4 py-2 rounded-lg font-bold transition-all" style={{ background: 'var(--neon-green)', color: 'var(--bg-darker)' }}>
              ON
            </button>
          </div>
          <div className="flex items-center justify-between p-4 rounded-lg border border-purple-500/20">
            <div>
              <div className="font-bold">Container logs retention</div>
              <div className="text-sm opacity-70">Keep container logs for 7 days</div>
            </div>
            <button className="px-4 py-2 rounded-lg font-bold transition-all" style={{ background: 'var(--neon-purple)', color: 'var(--button-text)' }}>
              7 days
            </button>
          </div>
        </div>
      </div>

      {/* DockLite Updates */}
      <div className="card-vapor p-6 rounded-xl">
        <div className="flex items-center justify-between mb-6">
          <h2 className="text-2xl font-bold neon-text flex items-center gap-2" style={{ color: 'var(--neon-cyan)' }}>
            <ArrowFatUp size={20} weight="duotone" />
            DockLite Updates
          </h2>
          <button
            onClick={fetchStatus}
            disabled={statusLoading}
            className="p-2 rounded-lg transition-all hover:scale-105 disabled:opacity-50"
            style={{ color: 'var(--text-secondary)' }}
            title="Refresh status"
          >
            <ArrowClockwise size={16} weight="duotone" className={statusLoading ? 'animate-spin' : ''} />
          </button>
        </div>

        {updateStatus && (
          <>
            <div className="grid grid-cols-2 sm:grid-cols-4 gap-4 mb-6">
              <div className="p-3 rounded-lg" style={{ background: 'var(--surface-muted)' }}>
                <div className="text-xs opacity-60 mb-1">Version</div>
                <div className="font-bold font-mono" style={{ color: 'var(--neon-cyan)' }}>{updateStatus.version}</div>
              </div>
              <div className="p-3 rounded-lg" style={{ background: 'var(--surface-muted)' }}>
                <div className="text-xs opacity-60 mb-1">Commit</div>
                <div className="font-bold font-mono" style={{ color: 'var(--neon-purple)' }}>{updateStatus.gitHash || '—'}</div>
              </div>
              <div className="p-3 rounded-lg" style={{ background: 'var(--surface-muted)' }}>
                <div className="text-xs opacity-60 mb-1">Branch</div>
                <div className="font-bold font-mono" style={{ color: 'var(--neon-pink)' }}>{updateStatus.branch || '—'}</div>
              </div>
              <div className="p-3 rounded-lg" style={{ background: 'var(--surface-muted)' }}>
                <div className="text-xs opacity-60 mb-1">Behind</div>
                <div className="font-bold font-mono" style={{ color: updateStatus.updateAvailable ? 'var(--neon-yellow)' : 'var(--neon-green)' }}>
                  {updateStatus.commitsBehind} commit{updateStatus.commitsBehind !== 1 ? 's' : ''}
                </div>
              </div>
            </div>

            <div className="flex items-center gap-4 mb-6">
              {updateStatus.updateAvailable ? (
                <div className="flex items-center gap-2 text-sm font-bold" style={{ color: 'var(--neon-yellow)' }}>
                  <WarningCircle size={18} weight="duotone" />
                  {updateStatus.commitsBehind} update{updateStatus.commitsBehind !== 1 ? 's' : ''} available
                </div>
              ) : (
                <div className="flex items-center gap-2 text-sm font-bold" style={{ color: 'var(--neon-green)' }}>
                  <CheckCircle size={18} weight="duotone" />
                  Up to date
                </div>
              )}

              <button
                onClick={handleUpdate}
                disabled={updating || updateStatus.updateRunning}
                className="px-6 py-2 rounded-lg font-bold transition-all hover:scale-105 disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-2"
                style={{
                  background: (updating || updateStatus.updateRunning)
                    ? 'rgba(var(--text-muted-rgb), 0.3)'
                    : 'linear-gradient(135deg, var(--neon-cyan) 0%, var(--neon-purple) 100%)',
                  color: 'var(--button-text)',
                }}
              >
                <ArrowFatUp size={16} weight="duotone" className={(updating || updateStatus.updateRunning) ? 'animate-bounce' : ''} />
                {(updating || updateStatus.updateRunning) ? 'Updating...' : 'Update Now'}
              </button>
            </div>

            {updateError && (
              <div className="mb-4 px-4 py-2 rounded-lg text-sm font-mono" style={{ background: 'rgba(var(--status-error-rgb), 0.15)', color: 'var(--status-error)', border: '1px solid var(--status-error)' }}>
                {updateError}
              </div>
            )}

            {updateStatus.log && updateStatus.log.length > 0 && (
              <div>
                <div className="text-xs font-bold mb-2 opacity-60">Update log</div>
                <div
                  ref={logRef}
                  className="rounded-lg p-3 font-mono text-xs overflow-y-auto max-h-56 space-y-0.5"
                  style={{ background: 'var(--bg-darker)', color: 'var(--text-secondary)' }}
                >
                  {updateStatus.log.map((line, i) => (
                    <div key={i} style={{ color: line.includes('error') || line.includes('Error') ? 'var(--status-error)' : line.includes('===') ? 'var(--neon-cyan)' : undefined }}>
                      {line}
                    </div>
                  ))}
                </div>
                {updateStatus.lastUpdated && (
                  <div className="text-xs opacity-40 mt-1">Last updated: {new Date(updateStatus.lastUpdated).toLocaleString()}</div>
                )}
              </div>
            )}
          </>
        )}

        {!updateStatus && statusLoading && (
          <div className="text-sm opacity-50 flex items-center gap-2">
            <ArrowClockwise size={14} className="animate-spin" /> Checking for updates...
          </div>
        )}
      </div>

      {/* Maintenance Tools */}
      <div className="card-vapor p-6 rounded-xl">
        <h2 className="text-2xl font-bold neon-text mb-6 flex items-center gap-2" style={{ color: 'var(--neon-pink)' }}>
          <Wrench size={20} weight="duotone" />
          Maintenance Tools
        </h2>
        <div className="space-y-4">
          <div className="p-4 rounded-lg border border-purple-500/20">
            <div className="flex items-start justify-between mb-4">
              <div>
                <div className="font-bold text-lg mb-2">User Folder Check</div>
                <div className="text-sm opacity-70" style={{ color: 'var(--text-secondary)' }}>
                  Ensures all users have their home directories created in /var/www/sites/
                  <br />
                  This runs automatically on startup, but you can trigger it manually here.
                </div>
              </div>
            </div>
            <div className="flex items-center gap-4">
              <button
                onClick={handleCheckFolders}
                disabled={checkingFolders}
                className="px-6 py-3 rounded-lg font-bold transition-all hover:scale-105 disabled:opacity-50 disabled:cursor-not-allowed"
                style={{
                  background: checkingFolders
                    ? 'rgba(var(--text-muted-rgb), 0.3)'
                    : 'linear-gradient(135deg, var(--neon-cyan) 0%, var(--neon-purple) 100%)',
                  color: 'var(--button-text)',
                }}
              >
                {checkingFolders ? (
                  <span className="inline-flex items-center gap-2">
                    <ArrowClockwise size={16} weight="duotone" className="animate-spin" />
                    Checking...
                  </span>
                ) : (
                  <span className="inline-flex items-center gap-2">
                    <ArrowClockwise size={16} weight="duotone" />
                    Run Folder Check
                  </span>
                )}
              </button>
              {folderCheckResult && (
                <div
                  className="px-4 py-2 rounded-lg font-mono text-sm"
                  style={{
                    background: folderCheckResult.startsWith('✓')
                      ? 'rgba(var(--status-success-rgb), 0.2)'
                      : 'rgba(var(--status-error-rgb), 0.2)',
                    color: folderCheckResult.startsWith('✓')
                      ? 'var(--neon-green)'
                      : 'var(--status-error)',
                    border: `1px solid ${folderCheckResult.startsWith('✓') ? 'var(--neon-green)' : 'var(--status-error)'}`,
                  }}
                >
                  {folderCheckResult}
                </div>
              )}
            </div>
          </div>
        </div>
      </div>

      <ShellAccessCard />
    </div>
  );
}
