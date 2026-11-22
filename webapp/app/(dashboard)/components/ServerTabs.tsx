'use client';

import { Plus } from '@phosphor-icons/react';

export interface RemoteServer {
  id: number;
  name: string;
  url: string;
}

/** One tab for this server and one for each connected server. The tab bar only appears once another server is connected. */
export default function ServerTabs({
  servers,
  active,
  onSelect,
  onAdd,
}: {
  servers: RemoteServer[];
  active: number | null;
  onSelect: (id: number | null) => void;
  onAdd: () => void;
}) {
  const tab = (selected: boolean) =>
    `px-4 py-2 font-bold transition-colors whitespace-nowrap ${selected ? 'border-b-2 border-neon-pink text-accent-pink' : 'text-ink-muted hover:text-accent-cyan'}`;
  return (
    <div className="flex flex-wrap items-center gap-2 border-b border-neon-purple/30 mb-6" role="tablist" aria-label="Servers">
      <button type="button" role="tab" aria-selected={active === null} className={tab(active === null)} onClick={() => onSelect(null)}>
        This server
      </button>
      {servers.map((s) => (
        <button key={s.id} type="button" role="tab" aria-selected={active === s.id} className={tab(active === s.id)} onClick={() => onSelect(s.id)} title={s.url}>
          {s.name}
        </button>
      ))}
      <button type="button" className="px-3 py-2 text-sm inline-flex items-center gap-1 text-ink-muted hover:text-accent-cyan" onClick={onAdd}>
        <Plus size={14} weight="regular" /> Add server
      </button>
    </div>
  );
}
