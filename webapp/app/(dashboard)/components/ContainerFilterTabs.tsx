'use client';

import { useRef } from 'react';
import { Cube, Database, Globe, Package } from '@phosphor-icons/react';

export type TypeFilter = 'all' | 'sites' | 'databases' | 'other';
export type StatusFilter = 'any' | 'running' | 'stopped';

interface Props {
  type: TypeFilter;
  onType: (type: TypeFilter) => void;
  status: StatusFilter;
  onStatus: (status: StatusFilter) => void;
  counts: Record<TypeFilter, number>;
}

const TABS: { id: TypeFilter; label: string; icon: React.ReactNode }[] = [
  { id: 'all', label: 'All', icon: <Package size={18} weight="duotone" /> },
  { id: 'sites', label: 'Sites', icon: <Globe size={18} weight="duotone" /> },
  { id: 'databases', label: 'Databases', icon: <Database size={18} weight="duotone" /> },
  { id: 'other', label: 'Other', icon: <Cube size={18} weight="duotone" /> },
];

const STATUSES: { id: StatusFilter; label: string; dot?: string }[] = [
  { id: 'any', label: 'Any state' },
  { id: 'running', label: 'Running', dot: 'var(--neon-green)' },
  { id: 'stopped', label: 'Stopped', dot: 'var(--status-error)' },
];

/** Filter bar for the Containers page: type tabs with live counts, plus a Running/Stopped switch. */
export default function ContainerFilterTabs({ type, onType, status, onStatus, counts }: Props) {
  const tabRefs = useRef<Array<HTMLButtonElement | null>>([]);

  // Arrow keys move between tabs, like any tab list.
  const onKeyDown = (event: React.KeyboardEvent, index: number) => {
    let next = -1;
    if (event.key === 'ArrowRight') next = (index + 1) % TABS.length;
    else if (event.key === 'ArrowLeft') next = (index - 1 + TABS.length) % TABS.length;
    else if (event.key === 'Home') next = 0;
    else if (event.key === 'End') next = TABS.length - 1;
    if (next === -1) return;
    event.preventDefault();
    onType(TABS[next].id);
    tabRefs.current[next]?.focus();
  };

  return (
    <div className="mb-6 flex flex-wrap items-center justify-between gap-3">
      <div role="tablist" aria-label="Filter containers by type" className="flex flex-wrap gap-2">
        {TABS.map((tab, index) => {
          const selected = type === tab.id;
          return (
            <button
              key={tab.id}
              ref={(el) => {
                tabRefs.current[index] = el;
              }}
              role="tab"
              type="button"
              aria-selected={selected}
              tabIndex={selected ? 0 : -1}
              onClick={() => onType(tab.id)}
              onKeyDown={(event) => onKeyDown(event, index)}
              className={`inline-flex items-center gap-2 px-4 py-2 rounded-xl text-sm font-bold transition-all border ${
                selected ? 'neon-glow' : 'hover:shadow-md'
              }`}
              style={
                selected
                  ? {
                      background: 'linear-gradient(135deg, var(--neon-cyan) 0%, var(--neon-purple) 100%)',
                      color: 'var(--button-text)',
                      borderColor: 'transparent',
                    }
                  : { color: 'var(--neon-cyan)', borderColor: 'rgba(var(--neon-cyan-rgb), 0.35)' }
              }
            >
              {tab.icon}
              <span>{tab.label}</span>
              <span
                className="min-w-[1.4rem] px-1.5 rounded-full text-[11px] leading-5 text-center"
                style={{
                  background: selected ? 'rgba(0, 0, 0, 0.18)' : 'rgba(var(--neon-cyan-rgb), 0.15)',
                }}
              >
                {counts[tab.id]}
              </span>
            </button>
          );
        })}
      </div>

      <div role="group" aria-label="Filter containers by state" className="inline-flex rounded-xl overflow-hidden border" style={{ borderColor: 'rgba(var(--neon-purple-rgb), 0.4)' }}>
        {STATUSES.map((option) => {
          const selected = status === option.id;
          return (
            <button
              key={option.id}
              type="button"
              aria-pressed={selected}
              onClick={() => onStatus(option.id)}
              className="inline-flex items-center gap-2 px-3 py-2 text-xs font-bold transition-colors"
              style={
                selected
                  ? { background: 'rgba(var(--neon-purple-rgb), 0.3)', color: 'var(--text-primary)' }
                  : { color: 'var(--text-secondary)' }
              }
            >
              {option.dot && <span className="inline-block w-2 h-2 rounded-full" style={{ background: option.dot }} />}
              {option.label}
            </button>
          );
        })}
      </div>
    </div>
  );
}
