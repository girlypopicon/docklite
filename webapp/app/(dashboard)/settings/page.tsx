'use client';

import { Tag, Cube, Lightning, Lock } from '@phosphor-icons/react';

export default function SettingsPage() {
  return (
    <div className="space-y-8">
      <div className="card-vapor p-6 rounded-xl">
        <h2 className="text-2xl font-bold neon-text mb-4" style={{ color: 'var(--neon-pink)' }}>
          General
        </h2>
        <p className="text-sm font-mono opacity-70" style={{ color: 'var(--text-secondary)' }}>
          Choose a section — appearance, sidebars, the top bar, security and system — from the Settings menu.
        </p>
      </div>

      <div className="card-vapor p-6 rounded-xl">
        <h3 className="text-xl font-bold neon-text mb-4 flex items-center gap-2" style={{ color: 'var(--neon-green)' }}>
          <Tag size={20} weight="duotone" />
          System Information
        </h3>
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
          <div className="text-center">
            <div className="text-2xl font-bold neon-text" style={{ color: 'var(--neon-cyan)' }}>
              v1.0
            </div>
            <div className="text-xs font-mono opacity-70">Version</div>
          </div>
          <div className="text-center">
            <div className="flex justify-center text-2xl font-bold neon-text" style={{ color: 'var(--neon-purple)' }}>
              <Cube size={24} weight="duotone" />
            </div>
            <div className="text-xs font-mono opacity-70">Docker</div>
          </div>
          <div className="text-center">
            <div className="flex justify-center text-2xl font-bold neon-text" style={{ color: 'var(--neon-pink)' }}>
              <Lightning size={24} weight="duotone" />
            </div>
            <div className="text-xs font-mono opacity-70">Status</div>
          </div>
          <div className="text-center">
            <div className="flex justify-center text-2xl font-bold neon-text" style={{ color: 'var(--neon-yellow)' }}>
              <Lock size={24} weight="duotone" />
            </div>
            <div className="text-xs font-mono opacity-70">Secure</div>
          </div>
        </div>
      </div>
    </div>
  );
}
