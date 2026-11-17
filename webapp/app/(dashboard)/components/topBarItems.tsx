import {
  Archive,
  ArrowsHorizontal,
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
  containers: (size) => <Package size={size} weight="regular" />,
  backups: (size) => <Archive size={size} weight="regular" />,
  network: (size) => <Globe size={size} weight="regular" />,
  // A server rack, so the cog can mean Settings and only Settings.
  server: (size) => <HardDrives size={size} weight="regular" />,
  terminal: (size) => <TerminalWindow size={size} weight="regular" />,
  settings: (size) => <Gear size={size} weight="regular" />,
  account: (size) => <UserCircle size={size} weight="regular" />,
  logout: (size) => <SignOut size={size} weight="regular" />,
  spacer: (size) => <ArrowsHorizontal size={size} weight="regular" />,
};

export const TOP_BAR_LINKS: Record<string, string> = {
  containers: '/',
  backups: '/backups',
  network: '/network',
  server: '/server',
};
