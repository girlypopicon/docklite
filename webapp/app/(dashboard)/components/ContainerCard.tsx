'use client';

import { ContainerInfo } from '@/types';
import { useState, useRef, useEffect } from 'react';
import { createPortal } from 'react-dom';
import {
  Clock, Plug, Eye, Trash, Play, ArrowsClockwise, Stop, DotsThree, Copy, UserCircle, Folder, TerminalWindow,
  PaintBrush, Shapes, CaretRight, CaretLeft, Check,
} from '@phosphor-icons/react';
import { useToast } from '@/lib/hooks/useToast';
import { containerDisplayName } from '@/lib/container-names';
import { DB_CONTAINER_TYPES } from '@/lib/database-engines';
import { BOX_COLORS, BOX_COLOR_RGB, BOX_ICONS, boxIconFor, boxNameFontSize } from '@/lib/box-styles';

interface ContainerCardProps {
  container: ContainerInfo;
  onAction: (containerId: string, action: 'start' | 'stop' | 'restart') => void;
  onViewDetails?: (containerId: string, containerName: string) => void;
  onDelete?: (containerId: string, containerName: string) => void;
  onMenuOpenChange?: (open: boolean) => void;
  canAssign?: boolean;
  onAssign?: (containerId: string, containerName: string) => void;
  onMoveFolder?: (containerId: string, containerName: string) => void;
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
  onMoveFolder,
  onToggleTracking,
  isTracked = true,
  onStyle,
}: ContainerCardProps) {
  const [menuOpen, setMenuOpen] = useState(false);
  const [view, setView] = useState<MenuView>('main');
  const menuRef = useRef<HTMLDivElement>(null);
  const menuButtonRef = useRef<HTMLButtonElement>(null);
  const menuPopupRef = useRef<HTMLDivElement>(null);
  const [menuPos, setMenuPos] = useState<{ left: number; top?: number; bottom?: number } | null>(null);
  const downAt = useRef<{ x: number; y: number } | null>(null);
  const stopDnd = (event: React.PointerEvent) => event.stopPropagation();
  const toast = useToast();

  // What this card looks like. Starts from what's saved; changes show at once and are saved in the background.
  const [style, setStyle] = useState({ color: container.style?.color || '', icon: container.style?.icon || '' });
  useEffect(() => {
    setStyle({ color: container.style?.color || '', icon: container.style?.icon || '' });
  }, [container.style?.color, container.style?.icon]);

  const isRunning = container.state === 'running';
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
    const rect = menuButtonRef.current.getBoundingClientRect();
    const width = 232;
    const margin = 12;
    const left = Math.min(Math.max(margin, rect.right - width), window.innerWidth - width - margin);
    // Open downwards, or upwards from the button when the card is in the lower half of the screen.
    if (rect.bottom > window.innerHeight * 0.55) setMenuPos({ left, bottom: window.innerHeight - rect.top + 6 });
    else setMenuPos({ left, top: rect.bottom + 6 });
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
      onPointerDown={stopDnd}
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
      onPointerDown={stopDnd}
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
      onPointerDown={stopDnd}
      onPointerDownCapture={stopDnd}
    >
      <div
        ref={menuPopupRef}
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
        onPointerDown={stopDnd}
        onPointerDownCapture={stopDnd}
      >
        {view === 'main' && (
          <>
            {!isRunning
              ? item('Start', <Play size={16} weight="duotone" />, () => onAction(container.id, 'start'))
              : (
                <>
                  {item('Restart', <ArrowsClockwise size={16} weight="duotone" />, () => onAction(container.id, 'restart'))}
                  {item('Stop', <Stop size={16} weight="duotone" />, () => onAction(container.id, 'stop'))}
                </>
              )}
            {onViewDetails && item('View details', <Eye size={16} weight="duotone" />, () => onViewDetails(container.id, container.name))}
            {item('Open in terminal', <TerminalWindow size={16} weight="duotone" />, () =>
              window.dispatchEvent(new CustomEvent('docklite-open-terminal', { detail: { containerId: container.id, containerName: container.name } })),
            )}
            <div className="my-1 border-t" style={{ borderColor: 'rgba(var(--neon-purple-rgb), 0.25)' }} />
            {onStyle && item('Colour', <PaintBrush size={16} weight="duotone" />, () => setView('colour'), { arrow: true, keepOpen: true })}
            {onStyle && item('Icon', <Shapes size={16} weight="duotone" />, () => setView('icon'), { arrow: true, keepOpen: true })}
            {item('Copy name', <Copy size={16} weight="duotone" />, copyName)}
            {canAssign && onAssign && isSite && item('Assign to user', <UserCircle size={16} weight="duotone" />, () => onAssign(container.id, container.name))}
            {onMoveFolder && item('Move to folder', <Folder size={16} weight="duotone" />, () => onMoveFolder(container.id, container.name))}
            {onToggleTracking && item(isTracked ? 'Untrack' : 'Track', <Eye size={16} weight="duotone" />, () => onToggleTracking(container.id, isTracked), { danger: isTracked })}
            {onDelete && isSite && item('Delete', <Trash size={16} weight="duotone" />, () => onDelete(container.id, container.labels?.['docklite.domain'] || container.name), { danger: true })}
          </>
        )}

        {view === 'colour' && (
          <>
            {backRow('Colour')}
            <div className="grid grid-cols-4 gap-3 p-4" role="radiogroup" aria-label="Card colour">
              <button
                type="button"
                role="radio"
                aria-checked={!style.color}
                aria-label="Automatic colour"
                title="Automatic (by type)"
                onClick={(e) => { e.stopPropagation(); pick({ color: '' }); }}
                onPointerDown={stopDnd}
                className="h-9 w-9 rounded-full grid place-items-center text-[10px] font-bold"
                style={{ border: '2px dashed var(--text-secondary)', color: 'var(--text-secondary)', outline: !style.color ? '2px solid var(--text-primary)' : 'none', outlineOffset: 2 }}
              >
                Auto
              </button>
              {BOX_COLORS.map((c) => (
                <button
                  key={c.id}
                  type="button"
                  role="radio"
                  aria-checked={style.color === c.id}
                  aria-label={c.label}
                  title={c.label}
                  onClick={(e) => { e.stopPropagation(); pick({ color: c.id }); }}
                  onPointerDown={stopDnd}
                  className="h-9 w-9 rounded-full grid place-items-center"
                  style={{
                    background: `rgb(${c.rgb})`,
                    boxShadow: `0 0 10px rgba(${c.rgb}, 0.7)`,
                    outline: style.color === c.id ? '2px solid var(--text-primary)' : 'none',
                    outlineOffset: 2,
                  }}
                >
                  {style.color === c.id && <Check size={16} weight="bold" color="#160a1f" />}
                </button>
              ))}
            </div>
          </>
        )}

        {view === 'icon' && (
          <>
            {backRow('Icon')}
            <div className="grid grid-cols-4 gap-2 p-3" role="radiogroup" aria-label="Card icon">
              {BOX_ICONS.map(({ id, label, Icon }) => (
                <button
                  key={id || 'box'}
                  type="button"
                  role="radio"
                  aria-checked={style.icon === id}
                  aria-label={label}
                  title={label}
                  onClick={(e) => { e.stopPropagation(); pick({ icon: id }); }}
                  onPointerDown={stopDnd}
                  className="h-11 rounded-lg grid place-items-center hover:bg-white/5"
                  style={{
                    color: 'var(--text-primary)',
                    border: style.icon === id ? '2px solid var(--text-primary)' : '1px solid rgba(var(--neon-purple-rgb), 0.3)',
                  }}
                >
                  <Icon size={24} weight="duotone" />
                </button>
              ))}
            </div>
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
            className="flex items-center gap-1.5 text-[11px] font-mono font-bold uppercase tracking-wider"
            style={{ color: isRunning ? 'var(--text-green)' : 'var(--status-error)' }}
          >
            <span
              className="w-2 h-2 rounded-full"
              style={{ background: isRunning ? 'var(--neon-green)' : 'var(--status-error)', boxShadow: isRunning ? '0 0 8px var(--neon-green)' : 'none' }}
            />
            {isRunning ? 'Running' : 'Stopped'}
          </span>

          <div ref={menuRef} onPointerDown={stopDnd} onPointerDownCapture={stopDnd} onClick={(e) => e.stopPropagation()}>
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
              onPointerDown={stopDnd}
              className="w-7 h-7 rounded-full grid place-items-center transition-colors hover:bg-white/10"
              style={{ border: '1px solid rgba(var(--c), 0.6)', color: 'var(--text-primary)', background: 'transparent' }}
              title="More options"
              aria-label={`Options for ${displayName}`}
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
