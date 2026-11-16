'use client';

import { useEffect, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import { ContainerInfo, BoxGroup } from '@/types';
import { DotsThree, FolderOpen, PencilSimple, LinkSimple, PaintBrush, Shapes, Trash, CaretRight, CaretLeft, Check } from '@phosphor-icons/react';
import { BOX_COLOR_RGB, boxNameFontSize, groupIconFor } from '@/lib/box-styles';
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

const MAX_LISTED = 5;

/**
 * A group, as a card the same shape as a box. Instead of one box's details it lists the boxes inside. Click it to open them.
 */
export default function GroupCard({ group, boxes, onOpen, onRename, onToggleLinked, onStyle, onDelete, onMenuOpenChange, highlight, showOwner }: GroupCardProps) {
  const [menuOpen, setMenuOpen] = useState(false);
  const [view, setView] = useState<MenuView>('main');
  const [style, setStyle] = useState({ color: group.color || '', icon: group.icon || '' });
  const [menuPos, setMenuPos] = useState<{ left: number; top?: number; bottom?: number } | null>(null);
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
    const rect = buttonRef.current.getBoundingClientRect();
    const width = 232;
    const left = Math.min(Math.max(12, rect.right - width), window.innerWidth - width - 12);
    if (rect.bottom > window.innerHeight * 0.55) setMenuPos({ left, bottom: window.innerHeight - rect.top + 6 });
    else setMenuPos({ left, top: rect.bottom + 6 });
  }, [menuOpen]);

  const rgb = (style.color && BOX_COLOR_RGB[style.color]) || undefined;
  const BigIcon = groupIconFor(style.icon);
  const running = boxes.filter((b) => b.state === 'running').length;
  const listed = boxes.slice(0, MAX_LISTED);

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
        className="absolute rounded-lg overflow-hidden overflow-y-auto animate-slide-down"
        style={{
          ...menuPos,
          width: 232,
          maxWidth: 'calc(100vw - 24px)',
          maxHeight: 'calc(100vh - 24px)',
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

  return (
    <div
      className="docklite-container-card docklite-group-card docklite-neon group relative w-full aspect-[5/7] rounded-xl cursor-pointer select-none transition-transform hover:-translate-y-0.5"
      style={{
        ...({ '--c': rgb || 'var(--neon-purple-rgb)' } as React.CSSProperties),
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
        <BigIcon weight="duotone" className="absolute -right-6 -bottom-6" style={{ width: '82%', height: 'auto', aspectRatio: '1', color: 'rgba(var(--c), 0.16)' }} />
      </div>

      <div className="relative z-10 flex flex-col h-full p-3 min-w-0">
        <div className="flex items-center justify-between gap-2">
          <span className="flex items-center gap-1.5 text-[11px] font-mono font-bold uppercase tracking-wider" style={{ color: 'var(--text-secondary)' }}>
            {group.linked ? (
              <>
                <LinkSimple size={13} weight="bold" style={{ color: 'var(--text-cyan)' }} />
                <span style={{ color: 'var(--text-cyan)' }}>Linked</span>
              </>
            ) : (
              <>Group</>
            )}
          </span>
          <div ref={menuRef} onPointerDown={stopDnd} onMouseDown={stopDnd} onTouchStart={stopDnd} onPointerDownCapture={stopDnd} onMouseDownCapture={stopDnd} onTouchStartCapture={stopDnd} onClick={(e) => e.stopPropagation()}>
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
              className="w-7 h-7 rounded-full grid place-items-center transition-colors hover:bg-white/10"
              style={{ border: '1px solid rgba(var(--c), 0.6)', color: 'var(--text-primary)', background: 'transparent' }}
              title="More options"
              aria-label={`Options for group ${group.name}`}
              aria-haspopup="menu"
              aria-expanded={menuOpen}
            >
              <DotsThree size={18} weight="bold" />
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

        <ul className="mt-3 space-y-1 text-xs font-mono min-w-0" aria-label={`Boxes in ${group.name}`}>
          {listed.map((b) => (
            <li key={b.id} className="flex items-center gap-1.5 min-w-0" style={{ color: 'var(--text-primary)' }}>
              <span
                className="w-1.5 h-1.5 rounded-full shrink-0"
                style={{ background: b.state === 'running' ? 'var(--neon-green)' : 'var(--status-error)' }}
              />
              <span className="truncate">{containerDisplayName(b)}</span>
            </li>
          ))}
          {boxes.length > MAX_LISTED && (
            <li className="pl-3" style={{ color: 'var(--text-secondary)' }}>
              +{boxes.length - MAX_LISTED} more
            </li>
          )}
          {boxes.length === 0 && (
            <li style={{ color: 'var(--text-secondary)' }}>Empty. Drag a box here.</li>
          )}
        </ul>
      </div>
    </div>
  );
}
