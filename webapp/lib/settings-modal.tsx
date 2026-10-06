'use client';

import { createContext, useContext } from 'react';

export type SettingsTab = 'general' | 'appearance' | 'sidebars' | 'topbar' | 'security' | 'users' | 'system';

interface SettingsControl {
  /** Open the Settings modal over the current page, optionally on a tab. */
  openSettings: (tab?: SettingsTab) => void;
  closeSettings: () => void;
  /** True while Settings → Top bar is showing: the real top bar becomes editable in place. */
  editingTopBar: boolean;
}

export const SettingsControlContext = createContext<SettingsControl | null>(null);

/** True inside the Settings modal body, so pages shown there can behave
 *  differently from when they're opened as full pages (e.g. not navigate away). */
export const InSettingsModalContext = createContext(false);

export function useSettingsModal(): SettingsControl {
  const ctx = useContext(SettingsControlContext);
  if (!ctx) {
    // Outside the dashboard shell (shouldn't happen); fall back to the full page.
    return {
      openSettings: () => {
        window.location.href = '/settings';
      },
      closeSettings: () => {},
      editingTopBar: false,
    };
  }
  return ctx;
}

export function useInSettingsModal() {
  return useContext(InSettingsModalContext);
}
