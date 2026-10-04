'use client';

import {
  DndContext,
  KeyboardSensor,
  PointerSensor,
  closestCenter,
  useSensor,
  useSensors,
  type DragEndEvent,
} from '@dnd-kit/core';
import { SortableContext, arrayMove, sortableKeyboardCoordinates, useSortable, verticalListSortingStrategy } from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';
import { ArrowDown, ArrowUp, DotsSixVertical, Plus, X } from '@phosphor-icons/react';
import { DEFAULT_LAYOUT, TOP_BAR_ITEMS, useLayoutPrefs } from '@/lib/layout-prefs';
import { TOP_BAR_ICONS } from './topBarItems';

const DEFS = Object.fromEntries(TOP_BAR_ITEMS.map((item) => [item.id, item]));

/** Stable unique keys for the sortable list: spacers repeat, so number them. */
function withKeys(ids: string[]) {
  const seen: Record<string, number> = {};
  return ids.map((id) => {
    seen[id] = (seen[id] || 0) + 1;
    return { id, key: seen[id] === 1 ? id : `${id}#${seen[id]}` };
  });
}

function Row({
  entry,
  index,
  count,
  onMove,
  onRemove,
}: {
  entry: { id: string; key: string };
  index: number;
  count: number;
  onMove: (from: number, to: number) => void;
  onRemove: (index: number) => void;
}) {
  const def = DEFS[entry.id];
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({ id: entry.key });

  return (
    <li
      ref={setNodeRef}
      style={{
        transform: CSS.Transform.toString(transform),
        transition,
        opacity: isDragging ? 0.6 : 1,
        borderColor: 'rgba(var(--neon-purple-rgb), 0.3)',
        background: 'var(--surface-muted)',
      }}
      className="flex items-center gap-3 p-3 rounded-lg border"
    >
      <button
        type="button"
        {...attributes}
        {...listeners}
        aria-label={`Drag ${def.label} to reorder`}
        className="cursor-grab active:cursor-grabbing p-1 opacity-70 hover:opacity-100"
        style={{ touchAction: 'none' }}
      >
        <DotsSixVertical size={20} weight="bold" />
      </button>
      <span style={{ color: 'var(--neon-cyan)' }}>{TOP_BAR_ICONS[entry.id]?.(20)}</span>
      <span className="flex-1 min-w-0">
        <span className="block font-bold text-sm">{def.label}</span>
        <span className="block text-xs opacity-70 truncate">{def.description}</span>
      </span>
      <button
        type="button"
        onClick={() => onMove(index, index - 1)}
        disabled={index === 0}
        aria-label={`Move ${def.label} earlier`}
        className="p-1.5 rounded disabled:opacity-30"
      >
        <ArrowUp size={16} weight="bold" />
      </button>
      <button
        type="button"
        onClick={() => onMove(index, index + 1)}
        disabled={index === count - 1}
        aria-label={`Move ${def.label} later`}
        className="p-1.5 rounded disabled:opacity-30"
      >
        <ArrowDown size={16} weight="bold" />
      </button>
      {def.required ? (
        <span className="text-[10px] font-bold opacity-60 w-14 text-center" title="Always stays on the bar so you can't lock yourself out">
          ALWAYS
        </span>
      ) : (
        <button
          type="button"
          onClick={() => onRemove(index)}
          aria-label={`Remove ${def.label} from the top bar`}
          className="p-1.5 rounded w-14 flex justify-center"
          style={{ color: 'var(--status-error)' }}
        >
          <X size={16} weight="bold" />
        </button>
      )}
    </li>
  );
}

export default function TopBarSettings() {
  const { prefs, update } = useLayoutPrefs();
  const entries = withKeys(prefs.topBar);
  const onBar = new Set(prefs.topBar);
  const available = TOP_BAR_ITEMS.filter((item) => item.repeatable || !onBar.has(item.id));

  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 5 } }),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates })
  );

  const move = (from: number, to: number) => {
    if (to < 0 || to >= prefs.topBar.length) return;
    update({ topBar: arrayMove(prefs.topBar, from, to) });
  };
  const remove = (index: number) => update({ topBar: prefs.topBar.filter((_, i) => i !== index) });
  const add = (id: string) => update({ topBar: [...prefs.topBar, id] });

  const onDragEnd = (event: DragEndEvent) => {
    const { active, over } = event;
    if (!over || active.id === over.id) return;
    const from = entries.findIndex((e) => e.key === active.id);
    const to = entries.findIndex((e) => e.key === over.id);
    if (from >= 0 && to >= 0) move(from, to);
  };

  return (
    <div className="space-y-8">
      <div>
        <h2 className="text-2xl font-bold neon-text" style={{ color: 'var(--neon-cyan)' }}>
          Top bar
        </h2>
        <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>
          Build your own bar. Drag items (or use the arrows) to reorder them, remove what you don’t use, and add things
          back from the shelf below. A <b>flexible space</b> pushes everything after it to the right. Watch the real bar
          at the top of the page change as you go.
        </p>
      </div>

      <div className="card-vapor p-6 rounded-xl space-y-4">
        <div className="font-bold text-sm" style={{ color: 'var(--neon-pink)' }}>On your bar (left to right)</div>
        <DndContext sensors={sensors} collisionDetection={closestCenter} onDragEnd={onDragEnd}>
          <SortableContext items={entries.map((e) => e.key)} strategy={verticalListSortingStrategy}>
            <ul className="space-y-2">
              {entries.map((entry, index) => (
                <Row key={entry.key} entry={entry} index={index} count={entries.length} onMove={move} onRemove={remove} />
              ))}
            </ul>
          </SortableContext>
        </DndContext>
      </div>

      <div className="card-vapor p-6 rounded-xl space-y-4">
        <div className="font-bold text-sm" style={{ color: 'var(--neon-green)' }}>Shelf — click to add</div>
        {available.length === 0 ? (
          <p className="text-sm opacity-70">Everything is already on your bar.</p>
        ) : (
          <div className="flex flex-wrap gap-2">
            {available.map((item) => (
              <button
                key={item.id}
                type="button"
                onClick={() => add(item.id)}
                className="inline-flex items-center gap-2 px-3 py-2 rounded-lg border text-sm font-bold transition-colors"
                style={{ borderColor: 'rgba(var(--neon-cyan-rgb), 0.4)', color: 'var(--neon-cyan)' }}
                title={item.description}
              >
                <Plus size={14} weight="bold" />
                {TOP_BAR_ICONS[item.id]?.(16)}
                {item.label}
              </button>
            ))}
          </div>
        )}
      </div>

      <div className="text-center">
        <button type="button" className="btn-neon px-6 py-2 font-bold" onClick={() => update({ topBar: [...DEFAULT_LAYOUT.topBar] })}>
          Restore the default bar
        </button>
      </div>
    </div>
  );
}
