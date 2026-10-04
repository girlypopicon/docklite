'use client';

import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import { Gear, Lock, Palette, Rows, SidebarSimple, UsersThree, Wrench, X } from '@phosphor-icons/react';
import { useBodyScrollLock } from '@/lib/hooks/useBodyScrollLock';
import {
  InSettingsModalContext,
  SettingsControlContext,
  type SettingsTab,
} from '@/lib/settings-modal';
import GeneralSettings from '../settings/page';
import PasswordSettings from '../settings/password/page';
import UsersSettings from '../settings/users/page';
import SystemSettings from '../settings/system/page';
import AppearanceSettings from '../settings/appearance/page';
import SidebarSettings from './SidebarSettings';
import TopBarSettings from './TopBarSettings';

interface TabDef {
  id: SettingsTab;
  label: string;
  icon: React.ReactNode;
  adminOnly?: boolean;
  render: () => React.ReactNode;
}

const TABS: TabDef[] = [
  { id: 'general', label: 'General', icon: <Gear size={18} weight="duotone" />, render: () => <GeneralSettings /> },
  { id: 'appearance', label: 'Appearance', icon: <Palette size={18} weight="duotone" />, render: () => <AppearanceSettings /> },
  { id: 'sidebars', label: 'Sidebars', icon: <SidebarSimple size={18} weight="duotone" />, render: () => <SidebarSettings /> },
  { id: 'topbar', label: 'Top bar', icon: <Rows size={18} weight="duotone" />, render: () => <TopBarSettings /> },
  { id: 'security', label: 'Security', icon: <Lock size={18} weight="duotone" />, render: () => <PasswordSettings /> },
  { id: 'users', label: 'Users', icon: <UsersThree size={18} weight="duotone" />, adminOnly: true, render: () => <UsersSettings /> },
  { id: 'system', label: 'System', icon: <Wrench size={18} weight="duotone" />, adminOnly: true, render: () => <SystemSettings /> },
];

/**
 * Provides openSettings()/closeSettings() to the whole dashboard and renders
 * the Settings modal over whatever page is showing. Nothing navigates, so
 * closing it leaves you exactly where you were.
 */
export function SettingsModalProvider({ isAdmin, children }: { isAdmin: boolean; children: React.ReactNode }) {
  const [open, setOpen] = useState(false);
  const [tab, setTab] = useState<SettingsTab>('general');
  const returnFocusTo = useRef<HTMLElement | null>(null);

  const openSettings = useCallback((nextTab?: SettingsTab) => {
    returnFocusTo.current = document.activeElement as HTMLElement | null;
    if (nextTab) setTab(nextTab);
    setOpen(true);
  }, []);

  const closeSettings = useCallback(() => {
    setOpen(false);
    // Put keyboard focus back where it was before the modal opened.
    returnFocusTo.current?.focus?.();
  }, []);

  const control = useMemo(() => ({ openSettings, closeSettings }), [openSettings, closeSettings]);

  return (
    <SettingsControlContext.Provider value={control}>
      {children}
      {open && <SettingsModal tab={tab} onTab={setTab} isAdmin={isAdmin} onClose={closeSettings} />}
    </SettingsControlContext.Provider>
  );
}

function SettingsModal({
  tab,
  onTab,
  isAdmin,
  onClose,
}: {
  tab: SettingsTab;
  onTab: (tab: SettingsTab) => void;
  isAdmin: boolean;
  onClose: () => void;
}) {
  useBodyScrollLock();
  const dialogRef = useRef<HTMLDivElement>(null);
  const tabs = TABS.filter((t) => !t.adminOnly || isAdmin);
  const active = tabs.find((t) => t.id === tab) || tabs[0];

  useEffect(() => {
    dialogRef.current?.focus();
    const onKey = (event: KeyboardEvent) => {
      if (event.key === 'Escape' && !event.defaultPrevented) onClose();
    };
    document.addEventListener('keydown', onKey);
    return () => document.removeEventListener('keydown', onKey);
  }, [onClose]);

  return createPortal(
    // z-[9995]: above the page, below the top bar (z-[9999]) and the modals
    // that pages like Users open from inside it (z-[10000]).
    // Same backdrop classes as every other modal in the app, so each theme's
    // own modal-backdrop styling (e.g. Unicorn's soft blur) applies here too.
    <div
      className="fixed inset-0 z-[9995] bg-black/80 backdrop-blur-lg flex items-center justify-center p-3 sm:p-6"
      onMouseDown={(event) => {
        if (event.target === event.currentTarget) onClose();
      }}
    >
      <div
        ref={dialogRef}
        role="dialog"
        aria-modal="true"
        aria-label="Settings"
        tabIndex={-1}
        // Panel styled like the other modals (card-vapor neon-border + the same
        // inline gradient) so themes that restyle card-vapor, like Unicorn,
        // keep their readable colors instead of getting dark text on dark.
        className="card-vapor neon-border w-full max-w-6xl h-[min(92vh,860px)] mt-16 flex flex-col rounded-2xl outline-none overflow-hidden"
        style={{
          background: 'linear-gradient(135deg, var(--modal-bg-1) 0%, var(--modal-bg-2) 100%)',
          border: '2px solid rgba(var(--neon-cyan-rgb), 0.5)',
        }}
      >
        <div className="flex items-center justify-between px-6 py-4 border-b" style={{ borderColor: 'rgba(var(--neon-purple-rgb), 0.3)' }}>
          <h1 className="text-xl font-bold neon-text flex items-center gap-2" style={{ color: 'var(--neon-cyan)' }}>
            <Gear size={22} weight="duotone" />
            Settings
          </h1>
          <button type="button" onClick={onClose} aria-label="Close settings" className="p-2 rounded-lg hover:bg-white/10">
            <X size={20} weight="bold" />
          </button>
        </div>

        <div className="flex-1 min-h-0 flex flex-col md:flex-row">
          <nav
            className="md:w-52 flex-shrink-0 flex md:flex-col gap-1 p-3 overflow-x-auto md:overflow-y-auto border-b md:border-b-0 md:border-r"
            style={{ borderColor: 'rgba(var(--neon-purple-rgb), 0.3)' }}
            aria-label="Settings sections"
          >
            {tabs.map((t) => (
              <button
                key={t.id}
                type="button"
                onClick={() => onTab(t.id)}
                aria-current={t.id === active.id ? 'page' : undefined}
                className="flex items-center gap-2 px-3 py-2 rounded-lg text-sm font-bold text-left whitespace-nowrap transition-colors"
                style={
                  t.id === active.id
                    ? { background: 'rgba(var(--neon-pink-rgb), 0.15)', color: 'var(--neon-pink)' }
                    : { color: 'var(--text-secondary)' }
                }
              >
                {t.icon}
                {t.label}
              </button>
            ))}
          </nav>

          <div className="flex-1 min-w-0 overflow-y-auto p-6">
            <InSettingsModalContext.Provider value={true}>{active.render()}</InSettingsModalContext.Provider>
          </div>
        </div>
      </div>
    </div>,
    document.body
  );
}
