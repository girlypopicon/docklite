'use client';

import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { Gear } from '@phosphor-icons/react';

const tabs = [
  { id: 'general', label: 'General', href: '/settings' },
  { id: 'password', label: 'Security', href: '/settings/password' },
  { id: 'users', label: 'Users', href: '/settings/users' },
  { id: 'system', label: 'System', href: '/settings/system' },
  { id: 'appearance', label: 'Appearance', href: '/settings/appearance' },
  { id: 'sidebars', label: 'Sidebars', href: '/settings/sidebars' },
  { id: 'topbar', label: 'Top bar', href: '/settings/topbar' },
];

export default function SettingsLayout({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();

  return (
    <div className="max-w-[1400px] mx-auto space-y-6">
      <div>
        <h1 className="text-3xl font-bold neon-text mb-2 flex items-center gap-3" style={{ color: 'var(--neon-cyan)' }}>
          <Gear size={24} weight="duotone" />
          Settings
        </h1>
        <p className="text-sm font-mono" style={{ color: 'var(--text-secondary)' }}>
          Configure your DockLite experience
        </p>
      </div>

      <div className="flex flex-wrap gap-2 border-b border-neon-purple/30">
        {tabs.map((tab) => {
          const isActive =
            tab.href === '/settings'
              ? pathname === '/settings'
              : pathname.startsWith(tab.href);

          return (
            <Link
              key={tab.id}
              href={tab.href}
              className={`px-4 py-2 font-bold transition-colors ${
                isActive
                  ? 'border-b-2 border-neon-pink text-neon-pink'
                  : 'text-gray-400 hover:text-neon-cyan'
              }`}
            >
              {tab.label}
            </Link>
          );
        })}
      </div>

      {children}
    </div>
  );
}
