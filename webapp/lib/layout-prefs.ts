'use client';

import { useCallback, useSyncExternalStore } from 'react';

// How the dashboard is laid out — which sidebars exist, what the right one
// shows, and what is on the top bar and in what order. Stored per browser in
// localStorage; every component reads it through useLayoutPrefs(), so a
// change made in Settings shows up everywhere immediately.

export type SidebarContent = 'none' | 'stats' | 'logs' | 'database' | 'search';

export interface LayoutPrefs {
  leftSidebar: { enabled: boolean };
  rightSidebar: { enabled: boolean; content: SidebarContent };
  /** Ordered top bar items; 'spacer' entries (may repeat) push what follows to the right. */
  topBar: string[];
}

export interface TopBarItemDef {
  id: string;
  label: string;
  description: string;
  /** Items that can't be removed, so you can never lock yourself out of Settings or your account. */
  required?: boolean;
  /** May appear more than once. */
  repeatable?: boolean;
}

export const TOP_BAR_ITEMS: TopBarItemDef[] = [
  { id: 'containers', label: 'Containers', description: 'Your sites and containers' },
  { id: 'databases', label: 'Databases', description: 'Database containers and the system database' },
  { id: 'backups', label: 'Backups', description: 'Back up and restore sites and databases' },
  { id: 'network', label: 'Network', description: 'Ports, nginx, DNS and SSL' },
  { id: 'server', label: 'Server', description: 'Host health, services, logs and updates' },
  { id: 'terminal', label: 'Terminal', description: 'Open the terminal drawer' },
  { id: 'settings', label: 'Settings', description: 'The settings cog', required: true },
  { id: 'account', label: 'Account', description: 'Your account menu', required: true },
  { id: 'logout', label: 'Log out', description: 'A log out button (also in the account menu)' },
  { id: 'spacer', label: 'Flexible space', description: 'Pushes the items after it to the right', repeatable: true },
];

export const DEFAULT_LAYOUT: LayoutPrefs = {
  leftSidebar: { enabled: true },
  rightSidebar: { enabled: true, content: 'none' },
  topBar: ['containers', 'databases', 'backups', 'network', 'server', 'spacer', 'terminal', 'settings', 'account', 'logout'],
};

const STORAGE_KEY = 'docklite-layout';
const CHANGE_EVENT = 'docklite-layout-change';
const CONTENTS: SidebarContent[] = ['none', 'stats', 'logs', 'database', 'search'];
const KNOWN_ITEMS = new Set(TOP_BAR_ITEMS.map((i) => i.id));
const REPEATABLE = new Set(TOP_BAR_ITEMS.filter((i) => i.repeatable).map((i) => i.id));
const REQUIRED = TOP_BAR_ITEMS.filter((i) => i.required).map((i) => i.id);

/** Stable unique keys for drag-and-drop lists: spacers repeat, so number them. */
export function withKeys(ids: string[]) {
  const seen: Record<string, number> = {};
  return ids.map((id) => {
    seen[id] = (seen[id] || 0) + 1;
    return { id, key: seen[id] === 1 ? id : `${id}#${seen[id]}` };
  });
}

/** Repairs anything odd in stored data (older versions, hand edits). */
export function normalizeLayout(raw: unknown): LayoutPrefs {
  const r = (raw && typeof raw === 'object' ? raw : {}) as any;
  const content = CONTENTS.includes(r.rightSidebar?.content) ? r.rightSidebar.content : DEFAULT_LAYOUT.rightSidebar.content;

  let topBar: string[] = Array.isArray(r.topBar)
    ? r.topBar.filter((id: unknown) => typeof id === 'string' && KNOWN_ITEMS.has(id))
    : [...DEFAULT_LAYOUT.topBar];
  // No duplicates except repeatable items.
  topBar = topBar.filter((id, index) => REPEATABLE.has(id) || topBar.indexOf(id) === index);
  // Required items always come back, so Settings and Account can't be lost.
  for (const id of REQUIRED) {
    if (!topBar.includes(id)) topBar.push(id);
  }
  return {
    leftSidebar: { enabled: r.leftSidebar?.enabled !== false },
    rightSidebar: { enabled: r.rightSidebar?.enabled !== false, content },
    topBar,
  };
}

// useSyncExternalStore needs a stable snapshot object between changes, so the
// parsed value is cached against the raw string it came from.
let cachedRaw: string | null | undefined;
let cachedValue: LayoutPrefs = DEFAULT_LAYOUT;

function readStored(): string | null {
  try {
    return localStorage.getItem(STORAGE_KEY);
  } catch {
    return null;
  }
}

function getSnapshot(): LayoutPrefs {
  const raw = readStored();
  if (raw !== cachedRaw) {
    cachedRaw = raw;
    try {
      cachedValue = raw ? normalizeLayout(JSON.parse(raw)) : DEFAULT_LAYOUT;
    } catch {
      cachedValue = DEFAULT_LAYOUT;
    }
  }
  return cachedValue;
}

function getServerSnapshot(): LayoutPrefs {
  return DEFAULT_LAYOUT;
}

function subscribe(onChange: () => void) {
  window.addEventListener(CHANGE_EVENT, onChange);
  window.addEventListener('storage', onChange); // other tabs
  return () => {
    window.removeEventListener(CHANGE_EVENT, onChange);
    window.removeEventListener('storage', onChange);
  };
}

export function saveLayout(next: LayoutPrefs) {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(normalizeLayout(next)));
  } catch {
    /* private mode: the change still applies until the page reloads */
    cachedRaw = undefined;
    cachedValue = normalizeLayout(next);
  }
  window.dispatchEvent(new Event(CHANGE_EVENT));
}

export function useLayoutPrefs() {
  const prefs = useSyncExternalStore(subscribe, getSnapshot, getServerSnapshot);
  const update = useCallback((change: Partial<LayoutPrefs>) => saveLayout({ ...getSnapshot(), ...change }), []);
  const reset = useCallback(() => saveLayout(DEFAULT_LAYOUT), []);
  return { prefs, update, reset };
}
