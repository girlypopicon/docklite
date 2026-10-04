import {
  Archive,
  ArrowsHorizontal,
  Database,
  Gear,
  Globe,
  HardDrives,
  Package,
  SignOut,
  TerminalWindow,
  UserCircle,
} from '@phosphor-icons/react';

// Icons and routes for the top bar items, shared by the bar and its editor.
// (Labels and descriptions live in lib/layout-prefs.ts.)

export const TOP_BAR_ICONS: Record<string, (size: number) => React.ReactNode> = {
  containers: (size) => <Package size={size} weight="duotone" />,
  databases: (size) => <Database size={size} weight="duotone" />,
  backups: (size) => <Archive size={size} weight="duotone" />,
  network: (size) => <Globe size={size} weight="duotone" />,
  // A server rack, so the cog can mean Settings and only Settings.
  server: (size) => <HardDrives size={size} weight="duotone" />,
  terminal: (size) => <TerminalWindow size={size} weight="duotone" />,
  settings: (size) => <Gear size={size} weight="duotone" />,
  account: (size) => <UserCircle size={size} weight="duotone" />,
  logout: (size) => <SignOut size={size} weight="duotone" />,
  spacer: (size) => <ArrowsHorizontal size={size} weight="duotone" />,
};

export const TOP_BAR_LINKS: Record<string, string> = {
  containers: '/',
  databases: '/databases',
  backups: '/backups',
  network: '/network',
  server: '/server',
};
