'use client';

import { DesktopTower, Cube, Gear } from '@phosphor-icons/react';
import ShellAccessCard from '../../components/ShellAccessCard';
import UpdateCard from '../../components/UpdateCard';
import SiteFoldersPanel from '../../components/SiteFoldersPanel';

export default function SystemSettingsPage() {
  return (
    <div className="space-y-8">
      <div>
        <h2 className="text-2xl font-bold neon-text flex items-center gap-2" style={{ color: 'var(--text-cyan)' }}>
          <DesktopTower size={20} weight="duotone" />
          System Settings
        </h2>
        <p className="text-sm font-mono" style={{ color: 'var(--text-secondary)' }}>
          Docker daemon configuration
        </p>
      </div>

      {/* Docker Status */}
      <div className="card-vapor p-6 rounded-xl">
        <h2 className="text-2xl font-bold neon-text mb-4 flex items-center gap-2" style={{ color: 'var(--text-green)' }}>
          <Cube size={20} weight="duotone" />
          Docker Connection
        </h2>
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="flex items-center gap-4">
            <div className="w-4 h-4 shrink-0 rounded-full animate-pulse bg-green-500"></div>
            <div>
              <div className="font-bold text-lg">Connected</div>
              <div className="text-sm opacity-70" style={{ color: 'var(--text-secondary)' }}>
                Docker daemon status
              </div>
            </div>
          </div>
          <div className="sm:text-right min-w-0">
            <div className="text-sm font-mono opacity-70 break-all">Socket: /var/run/docker.sock</div>
            <div className="text-xs opacity-50">Version: API 1.41+</div>
          </div>
        </div>
      </div>

      {/* Configuration Options */}
      <div className="card-vapor p-6 rounded-xl">
        <h2 className="text-2xl font-bold neon-text mb-6 flex items-center gap-2" style={{ color: 'var(--text-yellow)' }}>
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

      {/* Site folders: strays, adoption, trash */}
      <SiteFoldersPanel />

      <ShellAccessCard />
    </div>
  );
}
