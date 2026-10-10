'use client';

import { useState, useEffect } from 'react';
import { DesktopTower, Cube, Gear, Wrench, ArrowClockwise } from '@phosphor-icons/react';
import ShellAccessCard from '../../components/ShellAccessCard';
import UpdateCard from '../../components/UpdateCard';

export default function SystemSettingsPage() {
  const [checkingFolders, setCheckingFolders] = useState(false);
  const [folderCheckResult, setFolderCheckResult] = useState<string | null>(null);

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
      <UpdateCard />

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
