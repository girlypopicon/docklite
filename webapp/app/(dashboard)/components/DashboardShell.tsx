'use client';

import { useEffect, useState } from 'react';
import dynamic from 'next/dynamic';
import DashboardNav from '../nav';
import SidebarPanel from './SidebarPanel';
import { UserSession } from '@/types';

const XtermDrawer = dynamic(() => import('./XtermDrawer'), { ssr: false });

type DashboardShellProps = {
  user: UserSession;
  children: React.ReactNode;
};

export default function DashboardShell({ user, children }: DashboardShellProps) {
  const [terminalOpen, setTerminalOpen] = useState(false);
  const [terminalTarget, setTerminalTarget] = useState<{ id: string; name: string } | null>(null);

  useEffect(() => {
    const handler = (event: Event) => {
      const customEvent = event as CustomEvent<{ containerId: string; containerName: string }>;
      if (!customEvent.detail?.containerId) {
        return;
      }
      setTerminalTarget({
        id: customEvent.detail.containerId,
        name: customEvent.detail.containerName,
      });
      setTerminalOpen(true);
    };
    window.addEventListener('docklite-open-terminal', handler);
    return () => window.removeEventListener('docklite-open-terminal', handler);
  }, []);

  return (
    <>
      <DashboardNav
        user={user}
        terminalOpen={terminalOpen}
        onToggleTerminal={() => setTerminalOpen((prev) => !prev)}
      />

      {/* Customizable Sidebars - Overlay style, don't push content */}
      <SidebarPanel side="left" mode="file-browser" defaultOpen={false} userSession={user} />
      <SidebarPanel side="right" mode="modular" defaultContent="none" defaultOpen={false} />

      {/* Main content area - keeps existing width */}
      <main className="p-8 relative z-10">
        {children}
      </main>

      {/* Footer with system info */}
      <footer className="fixed bottom-4 right-4 text-xs font-mono opacity-40 hover:opacity-70 transition-opacity z-50">
        <div className="card-vapor px-3 py-2 rounded-lg">
          <div style={{ color: 'var(--neon-cyan)' }}>DockLite v1.0</div>
          <div style={{ color: 'var(--text-secondary)' }}>System Ready</div>
        </div>
      </footer>

      <XtermDrawer
        open={terminalOpen}
        onClose={() => setTerminalOpen(false)}
        containerId={terminalTarget?.id}
        containerName={terminalTarget?.name}
      />
    </>
  );
}
