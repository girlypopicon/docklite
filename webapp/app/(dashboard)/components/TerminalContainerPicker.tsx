'use client';

import { useEffect, useMemo, useRef, useState } from 'react';
import { CaretDown, MagnifyingGlass } from '@phosphor-icons/react';
import { containerKind, KIND_ORDER, sortContainers, type ContainerKind } from '@/lib/container-sort';
import type { ContainerInfo } from '@/types';

interface Props {
  currentId?: string;
  currentName?: string;
  onSelect: (id: string, name: string) => void;
  /** Show the list directly instead of behind a button (used by the empty state). */
  inline?: boolean;
}

const GROUP_LABELS: Record<ContainerKind, string> = { site: 'Sites', database: 'Databases', other: 'Other' };

/** Friendly name for a row: the site's domain when it has one. */
function displayName(container: ContainerInfo) {
  return container.labels?.['docklite.domain'] || container.name;
}

/**
 * "Which container do you want a shell in?" — a searchable list grouped as
 * sites, databases, other; running containers first and selectable, stopped
 * ones shown dimmed (a shell needs a running container).
 */
export default function TerminalContainerPicker({ currentId, currentName, onSelect, inline = false }: Props) {
  const [openState, setOpen] = useState(false);
  const open = inline || openState;
  const [containers, setContainers] = useState<ContainerInfo[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [query, setQuery] = useState('');
  const rootRef = useRef<HTMLDivElement>(null);
  const searchRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (!open) return;
    let cancelled = false;
    (async () => {
      setLoading(true);
      setError('');
      try {
        const res = await fetch('/api/containers/all');
        if (!res.ok) throw new Error('Could not load your containers');
        const data = await res.json();
        if (!cancelled) setContainers(data.containers || []);
      } catch (err: any) {
        if (!cancelled) setError(err.message || 'Could not load your containers');
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    searchRef.current?.focus();
    return () => {
      cancelled = true;
    };
  }, [open]);

  useEffect(() => {
    if (!open || inline) return;
    const onPointer = (event: MouseEvent) => {
      if (rootRef.current && !rootRef.current.contains(event.target as Node)) setOpen(false);
    };
    const onKey = (event: KeyboardEvent) => {
      if (event.key === 'Escape') setOpen(false);
    };
    document.addEventListener('mousedown', onPointer);
    document.addEventListener('keydown', onKey);
    return () => {
      document.removeEventListener('mousedown', onPointer);
      document.removeEventListener('keydown', onKey);
    };
  }, [open, inline]);

  const groups = useMemo(() => {
    const q = query.trim().toLowerCase();
    const matching = sortContainers(containers).filter(
      (c) => !q || displayName(c).toLowerCase().includes(q) || c.name.toLowerCase().includes(q) || c.image.toLowerCase().includes(q)
    );
    return (Object.keys(KIND_ORDER) as ContainerKind[])
      .sort((a, b) => KIND_ORDER[a] - KIND_ORDER[b])
      .map((kind) => ({ kind, items: matching.filter((c) => containerKind(c) === kind) }))
      .filter((group) => group.items.length > 0);
  }, [containers, query]);

  const runningCount = containers.filter((c) => c.state === 'running').length;

  const choose = (container: ContainerInfo) => {
    if (container.state !== 'running') return;
    onSelect(container.id, displayName(container));
    setOpen(false);
    setQuery('');
  };

  return (
    <div className="relative" ref={rootRef}>
      {!inline && <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        aria-haspopup="listbox"
        aria-expanded={open}
        className="inline-flex items-center gap-2 px-3 py-1.5 rounded-lg text-xs font-bold border"
        style={{ borderColor: 'rgba(var(--status-success-rgb), 0.5)', color: 'var(--neon-green)', background: 'rgba(var(--status-success-rgb), 0.08)' }}
      >
        {currentName ? currentName : 'Choose a container…'}
        <CaretDown size={12} weight="bold" />
      </button>}

      {open && (
        <div
          className={`${inline ? 'w-[min(26rem,90vw)]' : 'absolute left-0 bottom-full mb-2 w-[min(26rem,90vw)] z-20'} rounded-xl border-2 shadow-2xl overflow-hidden text-left`}
          style={{ background: 'linear-gradient(135deg, var(--modal-bg-1) 0%, var(--modal-bg-2) 100%)', borderColor: 'rgba(var(--status-success-rgb), 0.5)' }}
          role="listbox"
          aria-label="Containers"
        >
          <div className="p-2 border-b flex items-center gap-2" style={{ borderColor: 'rgba(var(--status-success-rgb), 0.25)' }}>
            <MagnifyingGlass size={14} weight="bold" style={{ color: 'var(--neon-green)' }} />
            <input
              ref={searchRef}
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder={`Search ${containers.length || ''} containers…`}
              className="flex-1 bg-transparent outline-none text-xs font-mono"
              style={{ color: 'var(--text-primary)' }}
              aria-label="Search containers"
            />
            <span className="text-[10px] opacity-70 whitespace-nowrap">{runningCount} running</span>
          </div>

          <div className={`${inline ? 'max-h-40' : 'max-h-72'} overflow-y-auto docklite-scroll p-1`}>
            {loading && <div className="p-3 text-xs opacity-70">Loading…</div>}
            {error && <div className="p-3 text-xs" style={{ color: 'var(--status-error)' }}>{error}</div>}
            {!loading && !error && groups.length === 0 && <div className="p-3 text-xs opacity-70">No containers match.</div>}
            {groups.map((group) => (
              <div key={group.kind} className="mb-1">
                <div className="px-2 pt-2 pb-1 text-[10px] font-bold uppercase tracking-wider opacity-60">{GROUP_LABELS[group.kind]}</div>
                {group.items.map((c) => {
                  const running = c.state === 'running';
                  return (
                    <button
                      key={c.id}
                      type="button"
                      role="option"
                      aria-selected={c.id === currentId}
                      disabled={!running}
                      onClick={() => choose(c)}
                      title={running ? `Open a shell in ${c.name}` : 'Start this container first — a shell needs it running'}
                      className="w-full text-left px-2 py-1.5 rounded-lg flex items-center gap-2 transition-colors disabled:opacity-40 disabled:cursor-not-allowed hover:enabled:bg-white/10"
                      style={c.id === currentId ? { background: 'rgba(var(--status-success-rgb), 0.15)' } : undefined}
                    >
                      <span className="inline-block w-2 h-2 rounded-full flex-shrink-0" style={{ background: running ? 'var(--neon-green)' : 'var(--status-error)' }} />
                      <span className="min-w-0">
                        <span className="block text-xs font-bold truncate" style={{ color: 'var(--text-primary)' }}>{displayName(c)}</span>
                        <span className="block text-[10px] opacity-60 truncate font-mono">{c.image}</span>
                      </span>
                    </button>
                  );
                })}
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
