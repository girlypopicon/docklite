'use client';

import { useEffect, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import { ContainerInfo, BoxGroup } from '@/types';
import { DotsThree, FolderOpen, PencilSimple, LinkSimple, PaintBrush, Shapes, Trash, CaretRight, CaretLeft, Check } from '@phosphor-icons/react';
import { BOX_COLOR_RGB, boxIconFor, boxNameFontSize, groupIconFor } from '@/lib/box-styles';
import { placeMenu, type MenuPlacement } from '@/lib/menu-position';
import { containerDisplayName } from '@/lib/container-names';
import { ColourGrid, IconGrid } from './BoxStylePickers';

interface GroupCardProps {
  group: BoxGroup;
  boxes: ContainerInfo[]; // the boxes in this group that exist (a group can name a box that was deleted)
  onOpen: (group: BoxGroup) => void;
  onRename: (group: BoxGroup) => void;
  onToggleLinked: (group: BoxGroup, linked: boolean) => void;
  onStyle: (group: BoxGroup, style: { color: string; icon: string }) => void;
  onDelete: (group: BoxGroup) => void;
  onMenuOpenChange?: (open: boolean) => void;
  // A box is being dragged over this card (it would go into the group).
  highlight?: boolean;
  showOwner?: boolean;
}

type MenuView = 'main' | 'colour' | 'icon';

// Boxes shown as little tiles on the card; the last tile says how many more there are.
const MAX_TILES = 6;

/**
 * A group, as a card the same shape as a box. Instead of one box's details it lists the boxes inside. Click it to open them.
 */
export default function GroupCard({ group, boxes, onOpen, onRename, onToggleLinked, onStyle, onDelete, onMenuOpenChange, highlight, showOwner }: GroupCardProps) {
  const [menuOpen, setMenuOpen] = useState(false);
  const [view, setView] = useState<MenuView>('main');
  const [style, setStyle] = useState({ color: group.color || '', icon: group.icon || '' });
  const [menuPos, setMenuPos] = useState<MenuPlacement | null>(null);
  const menuRef = useRef<HTMLDivElement>(null);
  const buttonRef = useRef<HTMLButtonElement>(null);
  const popupRef = useRef<HTMLDivElement>(null);
  const downAt = useRef<{ x: number; y: number } | null>(null);
  const stopDnd = (e: React.SyntheticEvent) => e.stopPropagation();

  useEffect(() => {
    setStyle({ color: group.color || '', icon: group.icon || '' });
  }, [group.color, group.icon]);

  const closeMenu = () => {
    setMenuOpen(false);
    setView('main');
    onMenuOpenChange?.(false);
  };

  useEffect(() => {
    const onDown = (event: MouseEvent) => {
      const t = event.target as Node;
      if (!menuRef.current?.contains(t) && !popupRef.current?.contains(t)) {
        setMenuOpen(false);
        setView('main');
        onMenuOpenChange?.(false);
      }
    };
    document.addEventListener('mousedown', onDown);
    return () => document.removeEventListener('mousedown', onDown);
  }, [onMenuOpenChange]);

  useEffect(() => {
    if (!menuOpen || !buttonRef.current) return;
    setMenuPos(placeMenu(buttonRef.current.getBoundingClientRect()));
  }, [menuOpen]);

  const rgb = (style.color && BOX_COLOR_RGB[style.color]) || undefined;
  const BigIcon = groupIconFor(style.icon);
  const running = boxes.filter((b) => b.state === 'running').length;
  const overflow = boxes.length > MAX_TILES ? boxes.length - (MAX_TILES - 1) : 0;
  const tiles = overflow ? boxes.slice(0, MAX_TILES - 1) : boxes;

  const pick = (next: { color?: string; icon?: string }) => {
    const merged = { color: next.color ?? style.color, icon: next.icon ?? style.icon };
    setStyle(merged);
    onStyle(group, merged);
  };

  const row = (label: string, icon: React.ReactNode, onClick: () => void, opts?: { danger?: boolean; arrow?: boolean; keepOpen?: boolean; checked?: boolean }) => (
    <button
      key={label}
      type="button"
      onClick={(e) => {
        e.stopPropagation();
        if (!opts?.keepOpen) closeMenu();
        onClick();
      }}
      onPointerDown={stopDnd} onMouseDown={stopDnd} onTouchStart={stopDnd}
      className="w-full px-3 py-2 text-left text-sm font-bold transition-colors hover:bg-white/5 flex items-center gap-3"
      style={{ color: opts?.danger ? 'var(--status-error)' : 'var(--text-primary)' }}
    >
      <span className="shrink-0 flex" style={{ color: opts?.danger ? 'var(--status-error)' : 'var(--text-cyan)' }}>{icon}</span>
      <span className="flex-1 min-w-0 truncate">{label}</span>
      {opts?.arrow && <CaretRight size={14} weight="bold" className="shrink-0 opacity-60" />}
      {opts?.checked !== undefined && (
        <span
          className="w-5 h-5 rounded grid place-items-center shrink-0"
          style={{ border: '1.5px solid var(--text-cyan)', background: opts.checked ? 'var(--text-cyan)' : 'transparent' }}
          aria-hidden="true"
        >
          {opts.checked && <Check size={14} weight="bold" color="#160a1f" />}
        </span>
      )}
    </button>
  );

  const back = (title: string) => (
    <button
      type="button"
      onClick={(e) => {
        e.stopPropagation();
        setView('main');
      }}
      onPointerDown={stopDnd} onMouseDown={stopDnd} onTouchStart={stopDnd}
      className="w-full px-3 py-2 text-left text-sm font-bold flex items-center gap-2 border-b hover:bg-white/5"
      style={{ color: 'var(--text-primary)', borderColor: 'rgba(var(--neon-purple-rgb), 0.25)' }}
    >
      <CaretLeft size={14} weight="bold" />
      {title}
    </button>
  );

  const menu = (
    <div className="fixed inset-0 z-[10000]" onClick={closeMenu} onPointerDown={stopDnd} onMouseDown={stopDnd} onTouchStart={stopDnd} onPointerDownCapture={stopDnd} onMouseDownCapture={stopDnd} onTouchStartCapture={stopDnd}>
      <div
        ref={popupRef}
        className="absolute rounded-lg overflow-hidden overflow-y-auto"
        style={{
          ...menuPos,
          // Not the global .animate-slide-down class: it forces position: relative, which breaks top/bottom placement.
          animation: 'slideDown 0.2s ease-out',
          width: 232,
          maxWidth: 'calc(100vw - 24px)',
          background: 'var(--bg-darker)',
          border: '1px solid rgba(var(--neon-purple-rgb), 0.9)',
          boxShadow: '0 0 28px rgba(var(--neon-purple-rgb), 0.65)',
        }}
        onClick={(e) => e.stopPropagation()}
        onPointerDown={stopDnd} onMouseDown={stopDnd} onTouchStart={stopDnd}
        onPointerDownCapture={stopDnd} onMouseDownCapture={stopDnd} onTouchStartCapture={stopDnd}
      >
        {view === 'main' && (
          <>
            {row('Open', <FolderOpen size={16} weight="duotone" />, () => onOpen(group))}
            {row('Rename', <PencilSimple size={16} weight="duotone" />, () => onRename(group))}
            {row('Linked', <LinkSimple size={16} weight="duotone" />, () => onToggleLinked(group, !group.linked), { keepOpen: true, checked: group.linked })}
            <div className="my-1 border-t" style={{ borderColor: 'rgba(var(--neon-purple-rgb), 0.25)' }} />
            {row('Colour', <PaintBrush size={16} weight="duotone" />, () => setView('colour'), { arrow: true, keepOpen: true })}
            {row('Icon', <Shapes size={16} weight="duotone" />, () => setView('icon'), { arrow: true, keepOpen: true })}
            {row('Delete group', <Trash size={16} weight="duotone" />, () => onDelete(group), { danger: true })}
          </>
        )}
        {view === 'colour' && (
          <>
            {back('Colour')}
            <ColourGrid value={style.color} onPick={(id) => pick({ color: id })} />
          </>
        )}
        {view === 'icon' && (
          <>
            {back('Icon')}
            <IconGrid value={style.icon} onPick={(id) => pick({ icon: id })} />
          </>
        )}
      </div>
    </div>
  );

  const cVars = { '--c': rgb || 'var(--neon-purple-rgb)' } as React.CSSProperties;

  return (
    // The card sits on top of two more, fanned out behind it: a little deck, so a group reads as "several boxes".
    <div className="group relative w-full aspect-[5/7]" style={cVars}>
      <div
        className="absolute inset-0 rounded-xl pointer-events-none transition-transform duration-200 group-hover:[transform:rotate(8.5deg)_scale(.95)]"
        style={{ transform: 'rotate(6deg) scale(.95)', background: 'var(--surface-dim)', border: '1.5px solid rgba(var(--c), 0.4)', boxShadow: '0 0 12px rgba(var(--c), 0.25)' }}
        aria-hidden="true"
      />
      <div
        className="absolute inset-0 rounded-xl pointer-events-none transition-transform duration-200 group-hover:[transform:rotate(-7deg)_scale(.95)]"
        style={{ transform: 'rotate(-4.5deg) scale(.95)', background: 'var(--surface-dim)', border: '1.5px solid rgba(var(--c), 0.55)', boxShadow: '0 0 12px rgba(var(--c), 0.3)' }}
        aria-hidden="true"
      />
      <div
        className="docklite-container-card docklite-group-card docklite-neon relative w-full h-full rounded-xl cursor-pointer select-none transition-transform hover:-translate-y-0.5"
        style={{
          background: 'var(--surface-dim)',
          backdropFilter: 'blur(12px)',
          containerType: 'inline-size',
          transform: highlight ? 'scale(1.04)' : undefined,
          outline: highlight ? '3px dashed rgba(var(--c), 0.9)' : undefined,
          outlineOffset: 4,
        }}
        title={group.name}
        onPointerDown={(e) => {
          downAt.current = { x: e.clientX, y: e.clientY };
        }}
        onClick={(e) => {
          const d = downAt.current;
          if (d && Math.hypot(e.clientX - d.x, e.clientY - d.y) > 6) return;
          onOpen(group);
        }}
      >
        <div className="absolute inset-0 overflow-hidden rounded-[inherit] pointer-events-none" aria-hidden="true">
          <BigIcon weight="duotone" className="absolute -right-6 -bottom-6" style={{ width: '82%', height: 'auto', aspectRatio: '1', color: 'rgba(var(--c), 0.14)' }} />
        </div>

        <div className="relative z-10 flex flex-col h-full p-3 min-w-0">
          <div className="flex items-center justify-between gap-2">
            <span
              className="flex items-center gap-1.5 min-w-0 font-mono font-bold uppercase tracking-wider"
              style={{ color: 'var(--text-secondary)', fontSize: 'clamp(9px, 5.6cqw, 11px)' }}
            >
              {group.linked ? (
                <>
                  <LinkSimple size={13} weight="bold" className="shrink-0" style={{ color: 'var(--text-cyan)' }} />
                  <span className="truncate" style={{ color: 'var(--text-cyan)' }}>Linked</span>
                </>
              ) : (
                <span className="truncate">Group</span>
              )}
            </span>
            <div
              ref={menuRef}
              className="flex items-center gap-1 shrink-0"
              onPointerDown={stopDnd} onMouseDown={stopDnd} onTouchStart={stopDnd} onPointerDownCapture={stopDnd} onMouseDownCapture={stopDnd} onTouchStartCapture={stopDnd}
              onClick={(e) => e.stopPropagation()}
            >
              <button
                ref={buttonRef}
                type="button"
                onClick={(e) => {
                  e.stopPropagation();
                  const next = !menuOpen;
                  setMenuOpen(next);
                  if (!next) setView('main');
                  onMenuOpenChange?.(next);
                }}
                onPointerDown={stopDnd} onMouseDown={stopDnd} onTouchStart={stopDnd}
                className="docklite-square-btn"
                style={{ color: 'var(--text-primary)' }}
                title="More options"
                aria-label={`Options for group ${group.name}`}
                aria-haspopup="menu"
                aria-expanded={menuOpen}
              >
                <DotsThree size={16} weight="bold" />
              </button>
              {menuOpen && menuPos && typeof document !== 'undefined' && createPortal(menu, document.body)}
            </div>
          </div>

          <h3
            className="docklite-container-name mt-3 font-bold leading-tight truncate whitespace-nowrap"
            style={{ color: 'var(--text-primary)', fontSize: boxNameFontSize(group.name) }}
            title={group.name}
          >
            {group.name}
          </h3>
          <p className="text-xs font-mono truncate" style={{ color: 'var(--text-secondary)' }}>
            {boxes.length} {boxes.length === 1 ? 'box' : 'boxes'}
            {boxes.length > running ? ` · ${boxes.length - running} stopped` : ''}
            {showOwner && group.owner_username ? ` · ${group.owner_username}` : ''}
          </p>

          {/* The boxes inside, as little rounded squares in their own colour and icon. */}
          <ul className="mt-auto grid grid-cols-3 gap-1.5 min-w-0" aria-label={`Boxes in ${group.name}`}>
            {tiles.map((b) => {
              const tileRgb = (b.style?.color && BOX_COLOR_RGB[b.style.color]) || 'var(--c)';
              const TileIcon = boxIconFor(b.style?.icon);
              const up = b.state === 'running';
              return (
                <li
                  key={b.id}
                  title={`${containerDisplayName(b)}${up ? '' : ' (stopped)'}`}
                  className="relative aspect-square rounded-lg grid place-items-center"
                  style={{ border: `1.5px solid rgba(${tileRgb}, ${up ? 0.8 : 0.35})`, background: `rgba(${tileRgb}, ${up ? 0.14 : 0.05})`, boxShadow: up ? `0 0 8px rgba(${tileRgb}, 0.35)` : 'none', opacity: up ? 1 : 0.65 }}
                >
                  <TileIcon weight="duotone" style={{ width: '58%', height: '58%', color: `rgb(${tileRgb})` }} />
                  <span
                    className="absolute top-0.5 right-0.5 w-1.5 h-1.5 rounded-full"
                    style={{ background: up ? 'var(--neon-green)' : 'var(--status-error)' }}
                  />
                </li>
              );
            })}
            {overflow > 0 && (
              <li
                className="aspect-square rounded-lg grid place-items-center font-mono font-bold text-xs"
                style={{ border: '1.5px dashed rgba(var(--c), 0.6)', color: 'var(--text-primary)' }}
                title={`${overflow} more`}
              >
                +{overflow}
              </li>
            )}
            {boxes.length === 0 && (
              <li className="col-span-3 text-xs font-mono" style={{ color: 'var(--text-secondary)' }}>
                Empty. Drag a box here.
              </li>
            )}
          </ul>
        </div>
      </div>
    </div>
  );
}
