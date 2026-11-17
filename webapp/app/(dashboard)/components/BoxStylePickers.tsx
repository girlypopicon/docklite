'use client';

import { Check } from '@phosphor-icons/react';
import { BOX_COLORS, BOX_ICONS } from '@/lib/box-styles';

// The colour and icon pickers inside a card's `⋯` menu, shared by box cards and group cards.

const stop = (e: React.SyntheticEvent) => e.stopPropagation();

export function ColourGrid({ value, onPick }: { value: string; onPick: (id: string) => void }) {
  return (
    <div className="grid grid-cols-4 gap-3 p-4" role="radiogroup" aria-label="Card colour">
      <button
        type="button"
        role="radio"
        aria-checked={!value}
        aria-label="Automatic colour"
        title="Automatic"
        onClick={(e) => {
          e.stopPropagation();
          onPick('');
        }}
        onPointerDown={stop} onMouseDown={stop} onTouchStart={stop}
        className="h-9 w-9 rounded-full grid place-items-center text-[10px] font-bold"
        style={{ border: '2px dashed var(--text-secondary)', color: 'var(--text-secondary)', outline: !value ? '2px solid var(--text-primary)' : 'none', outlineOffset: 2 }}
      >
        Auto
      </button>
      {BOX_COLORS.map((c) => (
        <button
          key={c.id}
          type="button"
          role="radio"
          aria-checked={value === c.id}
          aria-label={c.label}
          title={c.label}
          onClick={(e) => {
            e.stopPropagation();
            onPick(c.id);
          }}
          onPointerDown={stop} onMouseDown={stop} onTouchStart={stop}
          className="h-9 w-9 rounded-full grid place-items-center"
          style={{
            background: `rgb(${c.rgb})`,
            boxShadow: `0 0 10px rgba(${c.rgb}, 0.7)`,
            outline: value === c.id ? '2px solid var(--text-primary)' : 'none',
            outlineOffset: 2,
          }}
        >
          {value === c.id && <Check size={16} weight="bold" color="#160a1f" />}
        </button>
      ))}
    </div>
  );
}

export function IconGrid({ value, onPick }: { value: string; onPick: (id: string) => void }) {
  return (
    <div className="grid grid-cols-4 gap-2 p-3" role="radiogroup" aria-label="Card icon">
      {BOX_ICONS.map(({ id, label, Icon }) => (
        <button
          key={id || 'default'}
          type="button"
          role="radio"
          aria-checked={value === id}
          aria-label={label}
          title={label}
          onClick={(e) => {
            e.stopPropagation();
            onPick(id);
          }}
          onPointerDown={stop} onMouseDown={stop} onTouchStart={stop}
          className="h-11 rounded-lg grid place-items-center hover:bg-white/5"
          style={{
            color: 'var(--text-primary)',
            border: value === id ? '2px solid var(--text-primary)' : '1px solid rgba(var(--neon-purple-rgb), 0.3)',
          }}
        >
          <Icon size={24} weight="regular" />
        </button>
      ))}
    </div>
  );
}
