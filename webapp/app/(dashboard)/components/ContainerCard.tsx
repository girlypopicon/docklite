'use client';

import { ContainerInfo } from '@/types';
import { useState, useRef, useEffect } from 'react';
import { createPortal } from 'react-dom';
import {
  Clock, Plug, Eye, Trash, Play, ArrowsClockwise, Stop, DotsThree, Copy, UserCircle, Folder, TerminalWindow,
  PaintBrush, Shapes, CaretRight, CaretLeft, FolderSimple, FolderMinus,
} from '@phosphor-icons/react';
import { useToast } from '@/lib/hooks/useToast';
import { containerDisplayName } from '@/lib/container-names';
import { DB_CONTAINER_TYPES } from '@/lib/database-engines';
import { BOX_COLOR_RGB, boxIconFor, boxNameFontSize } from '@/lib/box-styles';
import { ColourGrid, IconGrid } from './BoxStylePickers';
import { placeMenu, type MenuPlacement } from '@/lib/menu-position';

interface ContainerCardProps {
  container: ContainerInfo;
  onAction: (containerId: string, action: 'start' | 'stop' | 'restart') => void;
  onViewDetails?: (containerId: string, containerName: string) => void;
  onDelete?: (containerId: string, containerName: string) => void;
  onMenuOpenChange?: (open: boolean) => void;
  canAssign?: boolean;
  onAssign?: (containerId: string, containerName: string) => void;
  onMoveGroup?: (containerId: string, containerName: string) => void;
  // When the card is shown inside a group's window: take the box out of the group.
  onRemoveFromGroup?: (containerId: string, containerName: string) => void;
  onToggleTracking?: (containerId: string, tracked: boolean) => void;
  isTracked?: boolean;
  // Save the colour and icon this box was given (the card already shows them; this is for keeping them).
  onStyle?: (containerId: string, style: { color: string; icon: string }) => void;
}

type MenuView = 'main' | 'colour' | 'icon';

/**
 * A box, as a playing card: portrait (5:7), a big muted icon behind it, one line for its name and one `⋯` menu for
 * everything else. Click the card for its details.
 */
export default function ContainerCard({
  container,
  onAction,
  onViewDetails,
  onDelete,
  onMenuOpenChange,
  canAssign,
  onAssign,
  onMoveGroup,
  onRemoveFromGroup,
  onToggleTracking,
  isTracked = true,
  onStyle,
}: ContainerCardProps) {
  const [menuOpen, setMenuOpen] = useState(false);
  const [view, setView] = useState<MenuView>('main');
  const menuRef = useRef<HTMLDivElement>(null);
  const menuButtonRef = useRef<HTMLButtonElement>(null);
  const menuPopupRef = useRef<HTMLDivElement>(null);
  const [menuPos, setMenuPos] = useState<MenuPlacement | null>(null);
  const downAt = useRef<{ x: number; y: number } | null>(null);
  const stopDnd = (event: React.SyntheticEvent) => event.stopPropagation();
  const toast = useToast();

  // What this card looks like. Starts from what's saved; changes show at once and are saved in the background.
  const [style, setStyle] = useState({ color: container.style?.color || '', icon: container.style?.icon || '' });
  useEffect(() => {
    setStyle({ color: container.style?.color || '', icon: container.style?.icon || '' });
  }, [container.style?.color, container.style?.icon]);

  const isRunning = container.state === 'running';
  // A SQLite database is a file inside a site, not a container: no start/stop, terminal, colour or grouping.
  const isVirtual = (container.labels?.['docklite.type'] || '') === 'sqlite';
  const displayName = containerDisplayName(container);
  // Docker lists a published port once per address family (IPv4 and IPv6); show each mapping once.
  const ports = Array.from(new Set((container.ports || '').split(',').map((p) => p.trim()).filter(Boolean))).join(', ');

  const containerType = container.labels?.['docklite.type'] || 'other';
  const isSite = ['static', 'php', 'node'].includes(containerType);
  const isDatabase = DB_CONTAINER_TYPES.includes(containerType);
  // The card's colour: the one picked, or one by what the box is. `--c` ("r, g, b") drives the glow and the icon.
  const neonClass = isSite ? 'docklite-neon-pink' : isDatabase ? 'docklite-neon-green' : 'docklite-neon-cyan';
  const pickedRgb = style.color ? BOX_COLOR_RGB[style.color] : undefined;
  const BigIcon = boxIconFor(style.icon);

  const closeMenu = () => {
    setMenuOpen(false);
    setView('main');
    onMenuOpenChange?.(false);
  };

  useEffect(() => {
    function handleClickOutside(event: MouseEvent) {
      const target = event.target as Node;
      if (!menuRef.current?.contains(target) && !menuPopupRef.current?.contains(target)) {
        setMenuOpen(false);
        setView('main');
        onMenuOpenChange?.(false);
      }
    }
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, [onMenuOpenChange]);

  useEffect(() => {
    if (!menuOpen || !menuButtonRef.current) return;
    setMenuPos(placeMenu(menuButtonRef.current.getBoundingClientRect()));
  }, [menuOpen]);

  const pick = (next: { color?: string; icon?: string }) => {
    const merged = { color: next.color ?? style.color, icon: next.icon ?? style.icon };
    setStyle(merged);
    onStyle?.(container.id, merged);
  };

  const copyName = async () => {
    let copied = false;
    try {
      if (navigator.clipboard?.writeText) {
        await navigator.clipboard.writeText(container.name);
        copied = true;
      } else {
        const input = document.createElement('textarea');
        input.value = container.name;
        input.style.position = 'fixed';
        input.style.opacity = '0';
        document.body.appendChild(input);
        input.select();
        copied = document.execCommand('copy');
        document.body.removeChild(input);
      }
    } catch {
      copied = false;
    }
    if (copied) toast.success('Copied name');
    else toast.error('Copy failed');
  };

  // One menu row. Closing is the default; the colour and icon rows move to their own view instead.
  const item = (label: string, icon: React.ReactNode, onClick: () => void, opts?: { danger?: boolean; arrow?: boolean; keepOpen?: boolean }) => (
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
    </button>
  );

  const backRow = (title: string) => (
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
    <div
      className="fixed inset-0 z-[10000]"
      onClick={closeMenu}
      onPointerDown={stopDnd} onMouseDown={stopDnd} onTouchStart={stopDnd}
      onPointerDownCapture={stopDnd} onMouseDownCapture={stopDnd} onTouchStartCapture={stopDnd}
    >
      <div
        ref={menuPopupRef}
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
            {onViewDetails && item('View details', <Eye size={16} weight="duotone" />, () => onViewDetails(container.id, container.name))}
            {!isVirtual && item('Open in terminal', <TerminalWindow size={16} weight="duotone" />, () =>
              window.dispatchEvent(new CustomEvent('docklite-open-terminal', { detail: { containerId: container.id, containerName: container.name } })),
            )}
            <div className="my-1 border-t" style={{ borderColor: 'rgba(var(--neon-purple-rgb), 0.25)' }} />
            {onStyle && !isVirtual && item('Colour', <PaintBrush size={16} weight="duotone" />, () => setView('colour'), { arrow: true, keepOpen: true })}
            {onStyle && !isVirtual && item('Icon', <Shapes size={16} weight="duotone" />, () => setView('icon'), { arrow: true, keepOpen: true })}
            {item('Copy name', <Copy size={16} weight="duotone" />, copyName)}
            {canAssign && onAssign && isSite && item('Assign to user', <UserCircle size={16} weight="duotone" />, () => onAssign(container.id, container.name))}
            {onRemoveFromGroup && item('Take out of group', <FolderMinus size={16} weight="duotone" />, () => onRemoveFromGroup(container.id, container.name))}
            {onMoveGroup && !isVirtual && item('Move to group', <FolderSimple size={16} weight="duotone" />, () => onMoveGroup(container.id, container.name))}
            {onToggleTracking && !isVirtual && item(isTracked ? 'Untrack' : 'Track', <Eye size={16} weight="duotone" />, () => onToggleTracking(container.id, isTracked), { danger: isTracked })}
            {onDelete && isSite && item('Delete', <Trash size={16} weight="duotone" />, () => onDelete(container.id, container.labels?.['docklite.domain'] || container.name), { danger: true })}
          </>
        )}

        {view === 'colour' && (
          <>
            {backRow('Colour')}
            <ColourGrid value={style.color} onPick={(id) => pick({ color: id })} />
          </>
        )}

        {view === 'icon' && (
          <>
            {backRow('Icon')}
            <IconGrid value={style.icon} onPick={(id) => pick({ icon: id })} />
          </>
        )}
      </div>
    </div>
  );

  return (
    <div
      className={`docklite-container-card docklite-neon ${pickedRgb ? '' : neonClass} group relative w-full aspect-[5/7] rounded-xl cursor-pointer select-none transition-transform hover:-translate-y-0.5`}
      style={{
        ...(pickedRgb ? ({ '--c': pickedRgb } as React.CSSProperties) : {}),
        background: 'var(--surface-dim)',
        backdropFilter: 'blur(12px)',
        opacity: isRunning ? 1 : 0.8,
        containerType: 'inline-size', // lets the name size itself to the card's width
      }}
      title={container.name}
      onPointerDown={(e) => {
        downAt.current = { x: e.clientX, y: e.clientY };
      }}
      onClick={(e) => {
        // A drag ends in a click too; only a real tap opens the details.
        const d = downAt.current;
        if (d && Math.hypot(e.clientX - d.x, e.clientY - d.y) > 6) return;
        onViewDetails?.(container.id, container.name);
      }}
    >
      {/* The big icon, muted, bleeding off the bottom right corner. Clipped by its own box so the glow isn't. */}
      <div className="absolute inset-0 overflow-hidden rounded-[inherit] pointer-events-none" aria-hidden="true">
        <BigIcon
          weight="duotone"
          className="absolute -right-6 -bottom-6"
          style={{ width: '82%', height: 'auto', aspectRatio: '1', color: 'rgba(var(--c), 0.16)' }}
        />
      </div>

      <div className="relative z-10 flex flex-col h-full p-3 min-w-0">
        <div className="flex items-center justify-between gap-2">
          <span
            className="flex items-center gap-1.5 min-w-0 font-mono font-bold uppercase tracking-wider"
            style={{ color: isRunning ? 'var(--text-green)' : 'var(--status-error)', fontSize: 'clamp(9px, 5.6cqw, 11px)' }}
          >
            <span
              className="w-2 h-2 rounded-full shrink-0"
              style={{ background: isRunning ? 'var(--neon-green)' : 'var(--status-error)', boxShadow: isRunning ? '0 0 8px var(--neon-green)' : 'none' }}
            />
            <span className="truncate">{isVirtual ? 'File' : isRunning ? 'Running' : 'Stopped'}</span>
          </span>

          <div
            ref={menuRef}
            className="flex items-center gap-1 shrink-0"
            onPointerDown={stopDnd} onMouseDown={stopDnd} onTouchStart={stopDnd} onPointerDownCapture={stopDnd} onMouseDownCapture={stopDnd} onTouchStartCapture={stopDnd}
            onClick={(e) => e.stopPropagation()}
          >
            {!isVirtual && (
              <>
                <button
                  type="button"
                  onClick={(e) => { e.stopPropagation(); onAction(container.id, isRunning ? 'stop' : 'start'); }}
                  onPointerDown={stopDnd} onMouseDown={stopDnd} onTouchStart={stopDnd}
                  className="docklite-square-btn"
                  style={{ color: isRunning ? 'var(--status-error)' : 'var(--text-green)' }}
                  title={isRunning ? 'Stop' : 'Start'}
                  aria-label={`${isRunning ? 'Stop' : 'Start'} ${displayName}`}
                >
                  {isRunning ? <Stop size={13} weight="fill" /> : <Play size={13} weight="fill" />}
                </button>
                {isRunning && (
                  <button
                    type="button"
                    onClick={(e) => { e.stopPropagation(); onAction(container.id, 'restart'); }}
                    onPointerDown={stopDnd} onMouseDown={stopDnd} onTouchStart={stopDnd}
                    className="docklite-square-btn"
                    style={{ color: 'var(--text-yellow)' }}
                    title="Restart"
                    aria-label={`Restart ${displayName}`}
                  >
                    <ArrowsClockwise size={13} weight="bold" />
                  </button>
                )}
              </>
            )}
            <button
              ref={menuButtonRef}
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
              aria-label={`Options for ${displayName}`}
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
          style={{ color: 'var(--text-primary)', fontSize: boxNameFontSize(displayName) }}
          title={container.name}
        >
          {displayName}
        </h3>
        <p className="text-xs font-mono truncate" style={{ color: 'var(--text-secondary)' }}>
          {container.image.split(':')[0]}
          {container.owner_username ? ` · ${container.owner_username}` : ''}
        </p>

        <div className="mt-auto space-y-1 text-xs font-mono min-w-0">
          <div className="flex items-center gap-1.5 min-w-0" style={{ color: 'var(--text-secondary)' }}>
            <Clock size={14} weight="duotone" className="shrink-0" />
            <span className="truncate">{isRunning ? container.uptime || 'Running' : 'Not running'}</span>
          </div>
          {ports && (
            <div className="flex items-center gap-1.5 min-w-0" style={{ color: 'var(--text-purple)' }}>
              <Plug size={14} weight="duotone" className="shrink-0" />
              <span className="truncate">{ports}</span>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
