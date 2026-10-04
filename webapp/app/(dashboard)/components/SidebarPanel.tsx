'use client';

import { useState, useEffect, useRef } from 'react';
import { usePathname } from 'next/navigation';
import FileManager from './FileManager';
import SchemaBrowser from '../databases/SchemaBrowser';
import { ChartLine, Scroll, Database, MagnifyingGlass, CaretLeft, CaretRight, Play, Gear, FolderOpen, SidebarSimple } from '@phosphor-icons/react';
import { useLayoutPrefs, type SidebarContent } from '@/lib/layout-prefs';
import { useSettingsModal } from '@/lib/settings-modal';

interface SidebarPanelProps {
  side: 'left' | 'right';
  defaultContent?: SidebarContent;
  mode?: 'file-browser' | 'modular';
  defaultOpen?: boolean;
  userSession?: { username: string; isAdmin: boolean } | null;
}

export default function SidebarPanel({
  side,
  mode = 'modular',
  defaultOpen = false,
  userSession = null,
}: SidebarPanelProps) {
  const isFileBrowser = mode === 'file-browser';
  const pathname = usePathname();
  const isDbEditMode = Boolean(pathname?.match(/^\/databases\/\d+\/edit/));
  // Which sidebars exist and what the right one shows is chosen in
  // Settings → Sidebars, not here.
  const { prefs } = useLayoutPrefs();
  const { openSettings } = useSettingsModal();
  const enabled = side === 'left' ? prefs.leftSidebar.enabled : prefs.rightSidebar.enabled;
  const selectedContent: SidebarContent = isFileBrowser ? 'none' : prefs.rightSidebar.content;
  const [isOpen, setIsOpen] = useState(defaultOpen);

  // Resize functionality
  const [width, setWidth] = useState<number>(20); // percentage (vw)
  const [isResizing, setIsResizing] = useState(false);
  const resizeStartX = useRef<number>(0);
  const resizeStartWidth = useRef<number>(0);

  // Load saved width from localStorage on mount
  useEffect(() => {
    const savedWidth = localStorage.getItem(`sidebar-width-${side}`);
    if (savedWidth) {
      const parsed = parseFloat(savedWidth);
      if (!isNaN(parsed) && parsed >= 10 && parsed <= 40) {
        setWidth(parsed);
      }
    }
  }, [side]);

  // Save width to localStorage when it changes
  useEffect(() => {
    localStorage.setItem(`sidebar-width-${side}`, width.toString());
  }, [width, side]);

  // Handle resize start
  const handleResizeStart = (e: React.MouseEvent) => {
    e.preventDefault();
    setIsResizing(true);
    resizeStartX.current = e.clientX;
    resizeStartWidth.current = width;
  };

  // Handle resize move
  useEffect(() => {
    if (!isResizing) return;

    // Add cursor style and prevent text selection during resize
    document.body.style.cursor = 'col-resize';
    document.body.style.userSelect = 'none';

    const handleMouseMove = (e: MouseEvent) => {
      const deltaX = e.clientX - resizeStartX.current;
      const viewportWidth = window.innerWidth;
      const deltaVw = (deltaX / viewportWidth) * 100;

      let newWidth;
      if (side === 'left') {
        newWidth = resizeStartWidth.current + deltaVw;
      } else {
        newWidth = resizeStartWidth.current - deltaVw;
      }

      // Constrain width between 10vw and 40vw
      newWidth = Math.max(10, Math.min(40, newWidth));
      setWidth(newWidth);
    };

    const handleMouseUp = () => {
      setIsResizing(false);
      document.body.style.cursor = '';
      document.body.style.userSelect = '';
    };

    document.addEventListener('mousemove', handleMouseMove);
    document.addEventListener('mouseup', handleMouseUp);

    return () => {
      document.removeEventListener('mousemove', handleMouseMove);
      document.removeEventListener('mouseup', handleMouseUp);
      document.body.style.cursor = '';
      document.body.style.userSelect = '';
    };
  }, [isResizing, side]);

  if (!enabled && !(isDbEditMode && side === 'left')) return null;

  if (isDbEditMode && side === 'left') {
    return (
      <div className="relative">
        <div
          className="fixed top-20 left-0 h-[calc(100vh-80px)] bg-gradient-to-b from-purple-900/30 to-cyan-900/30 backdrop-blur-md border-r border-purple-500/20 flex flex-col z-40"
          style={{ width: `${width}vw` }}
        >
          <div className="flex-1 overflow-auto p-4">
            <SchemaBrowser />
          </div>
        </div>
        {/* Resize handle */}
        <div
          className="fixed top-20 h-[calc(100vh-80px)] w-1 cursor-col-resize hover:w-2 transition-all z-50 group"
          style={{
            left: `${width}vw`,
            background: isResizing
              ? 'linear-gradient(180deg, var(--neon-pink) 0%, var(--neon-purple) 50%, var(--neon-cyan) 100%)'
              : 'transparent'
          }}
          onMouseDown={handleResizeStart}
        >
          <div
            className="absolute inset-0 opacity-0 group-hover:opacity-100 transition-opacity"
            style={{
              background: 'linear-gradient(180deg, var(--neon-pink) 0%, var(--neon-purple) 50%, var(--neon-cyan) 100%)',
              boxShadow: '0 0 8px rgba(var(--neon-pink-rgb), 0.6)',
            }}
          />
        </div>
      </div>
    );
  }

  const contentLabels: Record<SidebarContent, string> = {
    none: 'Empty',
    stats: 'Live Stats',
    logs: 'Container Logs',
    database: 'Database Query',
    search: 'Search',
  };

  // Toggle button when sidebar is closed
  if (!isOpen || (!isFileBrowser && selectedContent === 'none')) {
    return (
      <button
        data-side={side}
        onClick={() => {
          if (!isFileBrowser && selectedContent === 'none') {
            // Nothing chosen yet: pick what this sidebar should show.
            openSettings('sidebars');
            return;
          }
          setIsOpen(true);
        }}
        className={`docklite-sidebar-toggle fixed ${side === 'left' ? 'left-0' : 'right-0'} top-1/2 -translate-y-1/2 z-40`}
        title={!isFileBrowser && selectedContent === 'none' ? 'Choose what this sidebar shows' : `Open ${side} sidebar`}
        aria-label={!isFileBrowser && selectedContent === 'none' ? 'Choose what this sidebar shows' : `Open ${side} sidebar`}
      >
        {isDbEditMode ? <Database size={16} weight="duotone" /> : isFileBrowser ? <FolderOpen size={16} weight="duotone" /> : <SidebarSimple size={16} weight="duotone" />}
        <span className="docklite-sidebar-toggle-label">{isDbEditMode ? 'Schema' : isFileBrowser ? 'Files' : 'Panel'}</span>
      </button>
    );
  }

  return (
    <div className="relative">
      <div
        className={`docklite-sidebar-panel fixed top-20 ${side === 'left' ? 'left-0' : 'right-0'} h-[calc(100vh-80px)] bg-gradient-to-b from-purple-900/30 to-cyan-900/30 backdrop-blur-md border-${side === 'left' ? 'r' : 'l'} border-purple-500/20 flex flex-col z-40`}
        style={{ width: `${width}vw` }}
      >
        {/* Header: what's showing, and a way to change it in Settings */}
        {!isFileBrowser && (
          <div className="p-4 border-b border-purple-500/20 flex items-center justify-between gap-2">
            <div className="font-bold text-sm" style={{ color: 'var(--neon-cyan)' }}>
              {contentLabels[selectedContent]}
            </div>
            <button
              type="button"
              onClick={() => openSettings('sidebars')}
              className="inline-flex items-center gap-1 px-2 py-1 rounded-lg text-xs font-bold border transition-colors"
              style={{ borderColor: 'rgba(var(--neon-cyan-rgb), 0.4)', color: 'var(--neon-cyan)' }}
              title="Change what this sidebar shows"
            >
              <Gear size={14} weight="duotone" />
              Change
            </button>
          </div>
        )}

        {/* Content Area - simple padding, no notch */}
        <div className={`flex-1 overflow-auto ${isFileBrowser ? 'p-0' : 'p-4'}`}>
          {isFileBrowser && (isDbEditMode ? <SchemaBrowser /> : <FileManager embedded userSession={userSession} />)}
          {!isFileBrowser && selectedContent === 'stats' && <StatsContent />}
          {!isFileBrowser && selectedContent === 'logs' && <LogsContent />}
          {!isFileBrowser && selectedContent === 'database' && <DatabaseContent />}
          {!isFileBrowser && selectedContent === 'search' && <SearchContent />}
        </div>
      </div>

      {/* Resize handle with neon line on inner edge */}
      <div
        className={`docklite-sidebar-edge fixed ${side === 'left' ? '' : ''} top-20 h-[calc(100vh-80px)] w-1 cursor-col-resize hover:w-2 transition-all z-50 group`}
        style={{
          [side === 'left' ? 'left' : 'right']: `${width}vw`,
          background: isResizing
            ? 'linear-gradient(180deg, var(--neon-pink) 0%, var(--neon-purple) 50%, var(--neon-cyan) 100%)'
            : 'transparent'
        }}
        onMouseDown={handleResizeStart}
        title="Drag to resize sidebar"
      >
        <div
          className="absolute inset-0 opacity-0 group-hover:opacity-100 transition-opacity"
          style={{
            background: 'linear-gradient(180deg, var(--neon-pink) 0%, var(--neon-purple) 50%, var(--neon-cyan) 100%)',
            boxShadow: '0 0 8px rgba(var(--neon-pink-rgb), 0.6)',
          }}
        />
      </div>

      {/* Close button - on inner edge (toward main content), centered vertically */}
      <button
        onClick={() => setIsOpen(false)}
        className={`docklite-sidebar-close fixed top-1/2 -translate-y-1/2 ${side === 'left' ? '-translate-x-1/2' : 'translate-x-1/2'} z-50`}
        style={{ [side === 'left' ? 'left' : 'right']: `${width}vw` }}
        aria-label="Close sidebar"
        title="Close sidebar"
      >
        {side === 'left' ? <CaretLeft size={14} weight="bold" /> : <CaretRight size={14} weight="bold" />}
      </button>
    </div>
  );
}

// Placeholder components for each content type
function StatsContent() {
  return (
    <div className="text-sm font-mono">
                  <div className="mb-4 text-neon-cyan font-bold flex items-center gap-2">        <ChartLine size={16} weight="duotone" />
        Live Stats
      </div>
      <p className="text-xs opacity-70 mb-4">System metrics</p>
      <div className="space-y-4">
        <div className="card-vapor p-3 rounded-lg">
          <div className="text-xs opacity-70 mb-1">CPU Usage</div>
          <div className="text-2xl font-bold" style={{ color: 'var(--neon-green)' }}>24%</div>
        </div>
        <div className="card-vapor p-3 rounded-lg">
          <div className="text-xs opacity-70 mb-1">Memory</div>
          <div className="text-2xl font-bold" style={{ color: 'var(--neon-cyan)' }}>3.2 GB</div>
        </div>
        <div className="card-vapor p-3 rounded-lg">
          <div className="text-xs opacity-70 mb-1">Containers</div>
          <div className="text-2xl font-bold" style={{ color: 'var(--neon-pink)' }}>12</div>
        </div>
      </div>
    </div>
  );
}

function LogsContent() {
  return (
    <div className="text-sm font-mono">
                  <div className="mb-4 text-neon-cyan font-bold flex items-center gap-2">        <Scroll size={16} weight="duotone" />
        Container Logs
      </div>
      <div className="bg-black/50 p-3 rounded-lg text-xs space-y-1 font-mono" style={{ color: 'var(--neon-green)' }}>
        <div>[2025-12-30 04:00:00] Container started</div>
        <div>[2025-12-30 04:00:01] Listening on port 80</div>
        <div>[2025-12-30 04:00:02] Ready to accept connections</div>
        <div className="opacity-50">...</div>
      </div>
    </div>
  );
}

function DatabaseContent() {
  return (
    <div className="text-sm font-mono">
                  <div className="mb-4 text-neon-cyan font-bold flex items-center gap-2">        <Database size={16} weight="duotone" />
        Database Query
      </div>
      <textarea
        className="w-full h-32 p-2 rounded-lg text-xs font-mono mb-2"
        style={{
          background: 'var(--surface-muted)',
          border: '2px solid var(--neon-purple)',
          color: 'var(--text-primary)',
        }}
        placeholder="SELECT * FROM users;"
      />
      <button
        className="btn-neon w-full py-2 text-sm font-bold"
      >
        <span className="inline-flex items-center gap-2">
          <Play size={14} weight="duotone" />
          Execute Query
        </span>
      </button>
    </div>
  );
}

function SearchContent() {
  return (
    <div className="text-sm font-mono">
                  <div className="mb-4 text-neon-cyan font-bold flex items-center gap-2">        <MagnifyingGlass size={16} weight="duotone" />
        Search
      </div>
      <input
        type="text"
        className="input-vapor w-full px-3 py-2 text-sm mb-4"
        placeholder="Search containers, sites..."
        style={{
          background: 'var(--surface-muted)',
          border: '2px solid var(--neon-cyan)',
        }}
      />
      <div className="text-xs opacity-70">No results</div>
    </div>
  );
}
