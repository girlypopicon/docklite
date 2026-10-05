'use client';

import Image from 'next/image';
import Link from 'next/link';
import { useRouter, usePathname } from 'next/navigation';
import { useState, useRef, useEffect } from 'react';
import { UserSession } from '@/types';
import {
  Sparkle,
  Database,
  Package,
  HardDrives,
  Globe,
  Gear,
  TerminalWindow,
  UserCircle,
  CrownSimple,
  SignOut,
  UsersThree,
} from '@phosphor-icons/react';
import { useBodyScrollLock } from '@/lib/hooks/useBodyScrollLock';

type DashboardNavProps = {
  user: UserSession;
  terminalOpen: boolean;
  onToggleTerminal: () => void;
};

export default function DashboardNav({ user, terminalOpen, onToggleTerminal }: DashboardNavProps) {
  const router = useRouter();
  const pathname = usePathname();
  const [isDropdownOpen, setIsDropdownOpen] = useState(false);
  const dropdownRef = useRef<HTMLDivElement>(null);
  const buttonRef = useRef<HTMLButtonElement>(null);

  const handleLogout = async () => {
    await fetch('/api/auth/logout', { method: 'POST' });
    // Hard redirect to ensure session is cleared
    window.location.href = '/login';
  };

  const isActive = (path: string) => pathname === path;

  // Close dropdown when clicking outside
  useBodyScrollLock(isDropdownOpen);

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
      if (event.key === 'Escape') {
        setIsDropdownOpen(false);
      }
    };

    document.addEventListener('mousedown', handleClickOutside);
    document.addEventListener('keydown', handleEscapeKey);
    

    return () => {
      document.removeEventListener('mousedown', handleClickOutside);
      document.removeEventListener('keydown', handleEscapeKey);
    };
  }, [isDropdownOpen]);

  return (
    <nav className="card-vapor border-b-2 relative overflow-visible z-[9999]" style={{ borderColor: 'rgba(var(--neon-purple-rgb), 0.3)' }}>
      {/* Animated background effect */}
      <div className="absolute inset-0 opacity-20">
        <div className="absolute inset-0 bg-gradient-to-r from-cyan-500/10 via-purple-500/10 to-pink-500/10 animate-pulse"></div>
      </div>
      
      <div className="px-4 sm:px-6 lg:px-8 relative">
        {/* Entire nav constrained to 1024px */}
        <div className="max-w-[1024px] mx-auto">
          <div className="flex justify-between items-center h-20">
            <div className="flex items-center flex-1 min-w-0 sm:-ml-[150px]">
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
              <div className="hidden sm:ml-2 sm:-ml-[150px] sm:flex sm:space-x-1 sm:-mr-2">
              <Link
                href="/"
                className={`inline-flex items-center gap-2 px-3 py-3 rounded-xl text-[15px] font-bold transition-all relative overflow-hidden ${
                  isActive('/')
                    ? 'neon-glow shadow-lg'
                    : 'hover:shadow-md'
                }`}
                style={isActive('/') ? {
                  background: 'linear-gradient(135deg, var(--neon-cyan) 0%, var(--neon-purple) 100%)',
                  color: 'var(--button-text)'
                } : {
                  color: 'var(--neon-cyan)'
                }}
              >
                <Package size={20} weight="duotone" style={{ filter: 'drop-shadow(0 0 6px rgba(var(--neon-cyan-rgb), 0.4))' }} />
                <span>Containers</span>
                {isActive('/') && (
                  <div className="absolute inset-0 bg-gradient-to-r from-cyan-400/20 to-purple-400/20 animate-pulse rounded-xl"></div>
                )}
              </Link>
              <Link
                href="/databases"
                className={`inline-flex items-center gap-2 px-3 py-3 rounded-xl text-[15px] font-bold transition-all relative overflow-hidden ${
                  isActive('/databases')
                    ? 'neon-glow shadow-lg'
                    : 'hover:shadow-md'
                }`}
                style={isActive('/databases') ? {
                  background: 'linear-gradient(135deg, var(--neon-cyan) 0%, var(--neon-purple) 100%)',
                  color: 'var(--button-text)'
                } : {
                  color: 'var(--neon-cyan)'
                }}
              >
                <Database size={20} weight="duotone" style={{ filter: 'drop-shadow(0 0 6px rgba(var(--neon-purple-rgb), 0.4))' }} />
                <span>Databases</span>
                {isActive('/databases') && (
                  <div className="absolute inset-0 bg-gradient-to-r from-cyan-400/20 to-purple-400/20 animate-pulse rounded-xl"></div>
                )}
              </Link>
              <Link
                href="/backups"
                className={`inline-flex items-center gap-2 px-3 py-3 rounded-xl text-[15px] font-bold transition-all relative overflow-hidden ${
                  isActive('/backups')
                    ? 'neon-glow shadow-lg'
                    : 'hover:shadow-md'
                }`}
                style={isActive('/backups') ? {
                  background: 'linear-gradient(135deg, var(--neon-cyan) 0%, var(--neon-purple) 100%)',
                  color: 'var(--button-text)'
                } : {
                  color: 'var(--neon-cyan)'
                }}
              >
                <HardDrives size={20} weight="duotone" style={{ filter: 'drop-shadow(0 0 6px rgba(var(--neon-green-rgb), 0.35))' }} />
                <span>Backups</span>
                {isActive('/backups') && (
                  <div className="absolute inset-0 bg-gradient-to-r from-cyan-400/20 to-purple-400/20 animate-pulse rounded-xl"></div>
                )}
              </Link>
              <Link
                href="/network"
                className={`inline-flex items-center gap-2 px-3 py-3 rounded-xl text-[15px] font-bold transition-all relative overflow-hidden ${
                  isActive('/network')
                    ? 'neon-glow shadow-lg'
                    : 'hover:shadow-md'
                }`}
                style={isActive('/network') ? {
                  background: 'linear-gradient(135deg, var(--neon-cyan) 0%, var(--neon-purple) 100%)',
                  color: 'var(--button-text)'
                } : {
                  color: 'var(--neon-cyan)'
                }}
              >
                <Globe size={20} weight="duotone" style={{ filter: 'drop-shadow(0 0 6px rgba(var(--neon-cyan-rgb), 0.4))' }} />
                <span>Network</span>
                {isActive('/network') && (
                  <div className="absolute inset-0 bg-gradient-to-r from-cyan-400/20 to-purple-400/20 animate-pulse rounded-xl"></div>
                )}
              </Link>
              <Link
                href="/server"
                className={`inline-flex items-center gap-2 px-3 py-3 rounded-xl text-[15px] font-bold transition-all relative overflow-hidden ${
                  isActive('/server')
                    ? 'neon-glow shadow-lg'
                    : 'hover:shadow-md'
                }`}
                style={isActive('/server') ? {
                  background: 'linear-gradient(135deg, var(--neon-cyan) 0%, var(--neon-purple) 100%)',
                  color: 'var(--button-text)'
                } : {
                  color: 'var(--neon-cyan)'
                }}
              >
                <Gear size={20} weight="duotone" style={{ filter: 'drop-shadow(0 0 6px rgba(var(--neon-purple-rgb), 0.4))' }} />
                <span>Server</span>
                {isActive('/server') && (
                  <div className="absolute inset-0 bg-gradient-to-r from-cyan-400/20 to-purple-400/20 animate-pulse rounded-xl"></div>
                )}
              </Link>
            </div>
            </div>

            <div className="flex items-center gap-3">
            <div className="relative z-[10000] isolate" ref={dropdownRef}>
              <button
                ref={buttonRef}
                onClick={() => setIsDropdownOpen(!isDropdownOpen)}
                className={`flex items-center justify-center p-2 transition-all hover:scale-105 card-vapor rounded-xl border ${
                  isDropdownOpen
                                              ? 'text-neon-pink border-neon-pink/60 bg-neon-pink/10'
                                              : 'text-neon-pink/70 hover:text-neon-pink border-neon-pink/20 hover:border-neon-pink/40'                }`}
              >
                <div className="w-8 h-8 rounded-full flex items-center justify-center text-xl relative" style={{ background: 'rgba(var(--neon-pink-rgb), 0.2)' }}>
                  <UserCircle size={22} weight="duotone" />
                  {user.role === 'super_admin' ? (
                    <span className="absolute -top-1 -right-1 text-xs">
                      <Sparkle size={12} weight="fill" />
                    </span>
                  ) : user.isAdmin ? (
                    <span className="absolute -top-1 -right-1 text-xs">
                      <CrownSimple size={12} weight="fill" />
                    </span>
                  ) : null}
                </div>
              </button>

              {isDropdownOpen && (
                <div
                  ref={dropdownRef}
                  className="absolute right-0 w-64 rounded-xl overflow-hidden border-2 shadow-2xl nav-user-dropdown"
                  style={{
                    background: 'linear-gradient(135deg, var(--card-bg-1) 0%, var(--card-bg-2) 100%)',
                    borderColor: 'var(--neon-pink)',
                    position: 'absolute',
                    top: '100%',
                    right: 0,
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
                    <Link
                      href="/settings"
                      className="flex items-center gap-3 px-4 py-3 text-sm font-bold transition-all group nav-dropdown-link"
                      onClick={() => setIsDropdownOpen(false)}
                    >
                      <Gear size={16} weight="duotone" className="group-hover:scale-110 transition-transform" />
                      <span>Settings</span>
                    </Link>
                    {user.isAdmin && (
                      <Link
                        href="/settings/users"
                        className="flex items-center gap-3 px-4 py-3 text-sm font-bold transition-all group nav-dropdown-link"
                        onClick={() => setIsDropdownOpen(false)}
                      >
                        <UsersThree size={16} weight="duotone" className="group-hover:scale-110 transition-transform" />
                        <span>Users</span>
                      </Link>
                    )}
                  </div>
                </div>
              )}
            </div>

            <button
              onClick={handleLogout}
              className="px-4 py-2 text-sm font-bold rounded-xl transition-all hover:scale-105 flex items-center gap-2"
              style={{
                background: 'linear-gradient(135deg, var(--status-error) 0%, var(--neon-pink) 100%)',
                color: 'var(--button-text)',
                boxShadow: '0 0 12px rgba(var(--neon-pink-rgb), 0.4)'
              }}
            >
              <SignOut size={16} weight="duotone" />
              <span className="hidden sm:inline">Logout</span>
            </button>

            <button
              onClick={onToggleTerminal}
              className="px-4 py-2 text-sm font-bold rounded-xl transition-all hover:scale-105 flex items-center gap-2"
              style={{
                background: terminalOpen
                  ? 'linear-gradient(135deg, var(--status-success) 0%, var(--status-info) 100%)'
                  : 'linear-gradient(135deg, var(--status-success) 0%, var(--neon-purple) 100%)',
                color: 'var(--button-text)',
                boxShadow: terminalOpen
                  ? '0 0 18px rgba(var(--status-info-rgb), 0.6)'
                  : '0 0 12px rgba(var(--status-success-rgb), 0.45)'
              }}
            >
              <TerminalWindow size={18} weight="duotone" />
              <span className="hidden sm:inline">Terminal</span>
            </button>
          </div>
        </div>
      </div>
      </div>
    </nav>
  );
}
