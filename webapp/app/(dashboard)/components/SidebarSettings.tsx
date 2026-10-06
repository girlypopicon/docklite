'use client';

import { ChartLine, Cube, Database, FolderOpen, MagnifyingGlass, ProhibitInset, Terminal } from '@phosphor-icons/react';
import { useLayoutPrefs, type SidebarContent } from '@/lib/layout-prefs';

const CONTENT_CHOICES: { value: SidebarContent; label: string; description: string; icon: React.ReactNode }[] = [
  { value: 'none', label: 'Empty', description: 'Keep the sidebar closed until you pick something.', icon: <ProhibitInset size={22} weight="duotone" /> },
  { value: 'stats', label: 'Live stats', description: 'CPU and memory for your running containers, updating as you work.', icon: <ChartLine size={22} weight="duotone" /> },
  { value: 'logs', label: 'Container logs', description: 'Follow a container’s log output while you browse other pages.', icon: <Terminal size={22} weight="duotone" /> },
  { value: 'database', label: 'Database query', description: 'Run quick queries against a database from anywhere.', icon: <Database size={22} weight="duotone" /> },
  { value: 'search', label: 'Search', description: 'Find containers, sites and databases by name.', icon: <MagnifyingGlass size={22} weight="duotone" /> },
];

function Switch({ checked, onChange, label }: { checked: boolean; onChange: (value: boolean) => void; label: string }) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      aria-label={label}
      onClick={() => onChange(!checked)}
      className="relative inline-flex h-7 w-12 flex-shrink-0 items-center rounded-full transition-colors"
      style={{ background: checked ? 'var(--status-success)' : 'var(--surface-dim)', border: '1px solid rgba(var(--neon-purple-rgb), 0.4)' }}
    >
      <span
        className="inline-block h-5 w-5 rounded-full transition-transform"
        style={{ background: 'var(--button-text)', transform: checked ? 'translateX(24px)' : 'translateX(3px)' }}
      />
    </button>
  );
}

export default function SidebarSettings() {
  const { prefs, update } = useLayoutPrefs();
  const { leftSidebar, rightSidebar } = prefs;

  return (
    <div className="space-y-8">
      <div>
        <h2 className="text-2xl font-bold neon-text flex items-center gap-2" style={{ color: 'var(--neon-cyan)' }}>
          Sidebars
        </h2>
        <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>
          Sidebars slide over the page from the left and right edges. Turn one off and its handle disappears completely.
          Changes apply instantly.
        </p>
      </div>

      <div className="card-vapor p-6 rounded-xl space-y-3">
        <div className="flex items-center justify-between gap-4">
          <div>
            <div className="font-bold flex items-center gap-2" style={{ color: 'var(--neon-green)' }}>
              <FolderOpen size={20} weight="duotone" /> Left sidebar — file browser
            </div>
            <div className="text-sm opacity-70">Browse and edit your sites’ files without leaving the page you’re on.</div>
          </div>
          <Switch
            checked={leftSidebar.enabled}
            label="Show the left sidebar"
            onChange={(enabled) => update({ leftSidebar: { enabled } })}
          />
        </div>
      </div>

      <div className="card-vapor p-6 rounded-xl space-y-5">
        <div className="flex items-center justify-between gap-4">
          <div>
            <div className="font-bold flex items-center gap-2" style={{ color: 'var(--neon-pink)' }}>
              <Cube size={20} weight="duotone" /> Right sidebar
            </div>
            <div className="text-sm opacity-70">A flexible panel — you choose what lives in it.</div>
          </div>
          <Switch
            checked={rightSidebar.enabled}
            label="Show the right sidebar"
            onChange={(enabled) => update({ rightSidebar: { ...rightSidebar, enabled } })}
          />
        </div>

        <div className={rightSidebar.enabled ? '' : 'opacity-40 pointer-events-none'} aria-disabled={!rightSidebar.enabled}>
          <div className="font-bold mb-3 text-sm" style={{ color: 'var(--neon-cyan)' }}>What should it show?</div>
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            {CONTENT_CHOICES.map((choice) => {
              const selected = rightSidebar.content === choice.value;
              return (
                <button
                  key={choice.value}
                  type="button"
                  onClick={() => update({ rightSidebar: { ...rightSidebar, content: choice.value } })}
                  className="text-left p-4 rounded-lg border-2 transition-all flex gap-3"
                  style={
                    selected
                      ? { borderColor: 'var(--neon-cyan)', background: 'rgba(var(--neon-cyan-rgb), 0.1)' }
                      : { borderColor: 'rgba(var(--neon-purple-rgb), 0.2)' }
                  }
                >
                  <span style={{ color: selected ? 'var(--neon-cyan)' : 'var(--text-secondary)' }}>{choice.icon}</span>
                  <span>
                    <span className="block font-bold">{choice.label}</span>
                    <span className="block text-xs opacity-70">{choice.description}</span>
                  </span>
                </button>
              );
            })}
          </div>
        </div>
      </div>
    </div>
  );
}
