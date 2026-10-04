'use client';

import Image from 'next/image';
import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { useState, useRef, useEffect } from 'react';
import { UserSession } from '@/types';
import { Sparkle, TerminalWindow, UserCircle, CrownSimple, SignOut, UsersThree, Gear } from '@phosphor-icons/react';
import { useBodyScrollLock } from '@/lib/hooks/useBodyScrollLock';
import { useLayoutPrefs, TOP_BAR_ITEMS } from '@/lib/layout-prefs';
import { useSettingsModal } from '@/lib/settings-modal';
import { TOP_BAR_ICONS, TOP_BAR_LINKS } from './components/topBarItems';

type DashboardNavProps = {
  user: UserSession;
  terminalOpen: boolean;
  onToggleTerminal: () => void;
};

const LABELS = Object.fromEntries(TOP_BAR_ITEMS.map((item) => [item.id, item.label]));

export default function DashboardNav({ user, terminalOpen, onToggleTerminal }: DashboardNavProps) {
  const pathname = usePathname();
  const { prefs } = useLayoutPrefs();
  const { openSettings } = useSettingsModal();
  const [isDropdownOpen, setIsDropdownOpen] = useState(false);
  const dropdownRef = useRef<HTMLDivElement>(null);
  const buttonRef = useRef<HTMLButtonElement>(null);

  const handleLogout = async () => {
    await fetch('/api/auth/logout', { method: 'POST' });
    // Hard redirect to ensure session is cleared
    window.location.href = '/login';
  };

  const isActive = (path: string) => pathname === path;

  useBodyScrollLock(isDropdownOpen);

  // Close the account menu on outside click or Escape.
  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      const target = event.target as Node;
      const isClickOnButton = buttonRef.current && buttonRef.current.contains(target);
      const isClickOnDropdown = dropdownRef.current && dropdownRef.current.contains(target);
      if (!isClickOnButton && !isClickOnDropdown && isDropdownOpen) {
        setIsDropdownOpen(false);
      }
    };
    const handleEscapeKey = (event: KeyboardEvent) => {
      if (event.key === 'Escape') setIsDropdownOpen(false);
    };
    document.addEventListener('mousedown', handleClickOutside);
    document.addEventListener('keydown', handleEscapeKey);
    return () => {
      document.removeEventListener('mousedown', handleClickOutside);
      document.removeEventListener('keydown', handleEscapeKey);
    };
  }, [isDropdownOpen]);

  // Anchor the account menu to the side of the bar the button is on, so it
  // never opens off-screen wherever the user puts it.
  const firstSpacer = prefs.topBar.indexOf('spacer');
  const accountOnRight = firstSpacer !== -1 && prefs.topBar.indexOf('account') > firstSpacer;

  const roleBadge =
    user.role === 'super_admin' ? (
      <Sparkle size={12} weight="fill" />
    ) : user.isAdmin ? (
      <CrownSimple size={12} weight="fill" />
    ) : null;

  const renderItem = (id: string, index: number) => {
    if (TOP_BAR_LINKS[id]) {
      const href = TOP_BAR_LINKS[id];
      const active = isActive(href);
      return (
        <Link
          key={`${id}-${index}`}
          href={href}
          className={`hidden sm:inline-flex items-center gap-2 px-3 py-3 rounded-xl text-[15px] font-bold transition-all relative overflow-hidden ${
            active ? 'neon-glow shadow-lg' : 'hover:shadow-md'
          }`}
          style={
            active
              ? { background: 'linear-gradient(135deg, var(--neon-cyan) 0%, var(--neon-purple) 100%)', color: 'var(--button-text)' }
              : { color: 'var(--neon-cyan)' }
          }
        >
          {TOP_BAR_ICONS[id](20)}
          <span>{LABELS[id]}</span>
          {active && <div className="absolute inset-0 bg-gradient-to-r from-cyan-400/20 to-purple-400/20 animate-pulse rounded-xl"></div>}
        </Link>
      );
    }

    switch (id) {
      case 'spacer':
        return <div key={`spacer-${index}`} className="flex-1 min-w-2" aria-hidden="true" />;

      case 'terminal':
        return (
          <button
            key={id}
            onClick={onToggleTerminal}
            className="px-4 py-2 text-sm font-bold rounded-xl transition-all hover:scale-105 flex items-center gap-2"
            style={{
              background: terminalOpen
                ? 'linear-gradient(135deg, var(--status-success) 0%, var(--status-info) 100%)'
                : 'linear-gradient(135deg, var(--status-success) 0%, var(--neon-purple) 100%)',
              color: 'var(--button-text)',
              boxShadow: terminalOpen ? '0 0 18px rgba(var(--status-info-rgb), 0.6)' : '0 0 12px rgba(var(--status-success-rgb), 0.45)',
            }}
          >
            <TerminalWindow size={18} weight="duotone" />
            <span className="hidden sm:inline">Terminal</span>
          </button>
        );

      case 'settings':
        return (
          <button
            key={id}
            type="button"
            onClick={() => openSettings()}
            aria-label="Settings"
            title="Settings"
            className="flex items-center justify-center p-2 rounded-xl border transition-all hover:scale-105 card-vapor"
            style={{ color: 'var(--neon-purple)', borderColor: 'rgba(var(--neon-purple-rgb), 0.35)' }}
          >
            <Gear size={22} weight="duotone" />
          </button>
        );

      case 'logout':
        return (
          <button
            key={id}
            onClick={handleLogout}
            className="px-4 py-2 text-sm font-bold rounded-xl transition-all hover:scale-105 flex items-center gap-2"
            style={{
              background: 'linear-gradient(135deg, var(--status-error) 0%, var(--neon-pink) 100%)',
              color: 'var(--button-text)',
              boxShadow: '0 0 12px rgba(var(--neon-pink-rgb), 0.4)',
            }}
          >
            <SignOut size={16} weight="duotone" />
            <span className="hidden sm:inline">Logout</span>
          </button>
        );

      case 'account':
        return (
          <div key={id} className="relative z-[10000] isolate" ref={dropdownRef}>
            <button
              ref={buttonRef}
              onClick={() => setIsDropdownOpen(!isDropdownOpen)}
              aria-label="Account menu"
              aria-expanded={isDropdownOpen}
              className={`flex items-center justify-center p-2 transition-all hover:scale-105 card-vapor rounded-xl border ${
                isDropdownOpen
                  ? 'text-neon-pink border-neon-pink/60 bg-neon-pink/10'
                  : 'text-neon-pink/70 hover:text-neon-pink border-neon-pink/20 hover:border-neon-pink/40'
              }`}
            >
              <div className="w-8 h-8 rounded-full flex items-center justify-center text-xl relative" style={{ background: 'rgba(var(--neon-pink-rgb), 0.2)' }}>
                <UserCircle size={22} weight="duotone" />
                {roleBadge && <span className="absolute -top-1 -right-1 text-xs">{roleBadge}</span>}
              </div>
            </button>

            {isDropdownOpen && (
              <div
                className="absolute w-64 rounded-xl overflow-hidden border-2 shadow-2xl nav-user-dropdown"
                style={{
                  background: 'linear-gradient(135deg, var(--card-bg-1) 0%, var(--card-bg-2) 100%)',
                  borderColor: 'var(--neon-pink)',
                  top: '100%',
                  [accountOnRight ? 'right' : 'left']: 0,
                  marginTop: '0.5rem',
                  zIndex: 999999,
                }}
              >
                <div className="p-4 border-b" style={{ borderColor: 'rgba(var(--neon-pink-rgb), 0.3)' }}>
                  <div className="font-bold flex items-center gap-2" style={{ color: 'var(--neon-pink)' }}>
                    {user.username}
                    {user.role === 'super_admin' ? (
                      <span className="text-sm flex items-center gap-1">
                        <CrownSimple size={14} weight="fill" />
                        <Sparkle size={12} weight="fill" />
                      </span>
                    ) : user.isAdmin ? (
                      <span className="text-sm">
                        <CrownSimple size={14} weight="fill" />
                      </span>
                    ) : null}
                  </div>
                  <div className="text-xs opacity-70" style={{ color: 'var(--text-secondary)' }}>
                    {user.role === 'super_admin' ? 'Superadmin' : user.isAdmin ? 'Administrator' : 'User'}
                  </div>
                </div>
                <div className="py-2">
                  <button
                    type="button"
                    className="w-full flex items-center gap-3 px-4 py-3 text-sm font-bold transition-all group nav-dropdown-link text-left"
                    onClick={() => {
                      setIsDropdownOpen(false);
                      openSettings();
                    }}
                  >
                    <Gear size={16} weight="duotone" className="group-hover:scale-110 transition-transform" />
                    <span>Settings</span>
                  </button>
                  {user.isAdmin && (
                    <button
                      type="button"
                      className="w-full flex items-center gap-3 px-4 py-3 text-sm font-bold transition-all group nav-dropdown-link text-left"
                      onClick={() => {
                        setIsDropdownOpen(false);
                        openSettings('users');
                      }}
                    >
                      <UsersThree size={16} weight="duotone" className="group-hover:scale-110 transition-transform" />
                      <span>Users</span>
                    </button>
                  )}
                  <button
                    type="button"
                    className="w-full flex items-center gap-3 px-4 py-3 text-sm font-bold transition-all group nav-dropdown-link text-left"
                    onClick={handleLogout}
                  >
                    <SignOut size={16} weight="duotone" className="group-hover:scale-110 transition-transform" />
                    <span>Log out</span>
                  </button>
                </div>
              </div>
            )}
          </div>
        );

      default:
        return null;
    }
  };

  return (
    <nav className="card-vapor border-b-2 relative overflow-visible z-[9999]" style={{ borderColor: 'rgba(var(--neon-purple-rgb), 0.3)' }}>
      {/* Animated background effect */}
      <div className="absolute inset-0 opacity-20">
        <div className="absolute inset-0 bg-gradient-to-r from-cyan-500/10 via-purple-500/10 to-pink-500/10 animate-pulse"></div>
      </div>

      <div className="px-4 sm:px-6 lg:px-8 relative">
        <div className="max-w-[1400px] mx-auto flex items-center gap-3 h-20">
          <div className="flex-shrink-0 flex items-center gap-2 group">
            <Image
              src="/dockliteiconL.png"
              alt="DockLite logo"
              width={30}
              height={30}
              className="group-hover:scale-110 transition-transform"
              style={{ filter: 'drop-shadow(0 0 6px rgba(var(--neon-cyan-rgb), 0.6))' }}
            />
            <h1 className="docklite-logo text-4xl font-bold neon-text group-hover:scale-105 transition-transform">
              <span className="docklite-logo-dock">Dock</span>
              <span className="docklite-logo-lite">Lite</span>
            </h1>
            <Sparkle
              size={24}
              weight="duotone"
              color="var(--neon-pink)"
              className="opacity-70 group-hover:opacity-100 transition-opacity"
              style={{ filter: 'drop-shadow(0 0 6px rgba(var(--neon-pink-rgb), 0.6))' }}
            />
          </div>

          <div className="flex-1 min-w-0 flex items-center gap-2 sm:gap-1">{prefs.topBar.map(renderItem)}</div>
        </div>
      </div>
    </nav>
  );
}
