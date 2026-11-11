'use client';

import { ContainerInfo } from '@/types';
import { useState, useRef, useEffect } from 'react';
import { createPortal } from 'react-dom';
import Link from 'next/link';
import { Clock, Plug, Eye, Trash, Play, ArrowsClockwise, Stop, DotsThree, Copy, UserCircle, Folder, TerminalWindow } from '@phosphor-icons/react';
import { useToast } from '@/lib/hooks/useToast';
import { containerDisplayName } from '@/lib/container-names';

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
}

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
}: ContainerCardProps) {
  const [menuOpen, setMenuOpen] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);
  const menuButtonRef = useRef<HTMLButtonElement>(null);
  const menuPopupRef = useRef<HTMLDivElement>(null);
  const [menuPosition, setMenuPosition] = useState<{ top: number; left: number } | null>(null);
  const stopDnd = (event: React.PointerEvent) => event.stopPropagation();
  const toast = useToast();

  const isRunning = container.state === 'running';
  const statusColor = isRunning ? 'var(--neon-green)' : 'var(--status-error)';
  const displayName = containerDisplayName(container);
  // Docker lists a published port once per address family (IPv4 and IPv6); show each mapping once.
  const ports = Array.from(new Set((container.ports || '').split(',').map((p) => p.trim()).filter(Boolean))).join(', ');

  // Determine container type from labels
  const containerType = container.labels?.['docklite.type'] || 'other';
  const isSite = ['static', 'php', 'node'].includes(containerType);
  const isDatabase = ['postgres', 'mariadb', 'mongodb', 'redis'].includes(containerType);

  // The card's color. Glow comes from the .docklite-neon classes, which take an "r, g, b" so alpha works.
  const shadowColor = isSite
    ? 'var(--neon-pink)'
    : isDatabase
    ? 'var(--neon-green)'
    : 'var(--neon-cyan)';
  const neonClass = isSite ? 'docklite-neon-pink' : isDatabase ? 'docklite-neon-green' : 'docklite-neon-cyan';
  const textColor = isSite ? 'var(--text-pink)' : isDatabase ? 'var(--text-green)' : 'var(--text-cyan)';
  const rgbVar = isSite ? 'var(--neon-pink-rgb)' : isDatabase ? 'var(--neon-green-rgb)' : 'var(--neon-cyan-rgb)';
  const actionStyle = {
    background: 'transparent',
    border: `1px solid ${shadowColor}`,
    color: textColor,
    boxShadow: `0 0 4px rgba(${rgbVar}, 0.45)`,
  };

  // Close menu when clicking outside
  useEffect(() => {
    function handleClickOutside(event: MouseEvent) {
      const target = event.target as Node;
      const inButton = menuRef.current?.contains(target);
      const inMenu = menuPopupRef.current?.contains(target);
      if (!inButton && !inMenu) {
        setMenuOpen(false);
        onMenuOpenChange?.(false);
      }
    }
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, [onMenuOpenChange]);

  useEffect(() => {
    if (!menuOpen || !menuButtonRef.current) return;
    const rect = menuButtonRef.current.getBoundingClientRect();
    const menuWidth = 200;
    const margin = 12;
    const maxLeft = window.innerWidth - menuWidth - margin;
    const left = Math.min(Math.max(margin, rect.right - menuWidth), maxLeft);
    setMenuPosition({
      top: rect.bottom + 8,
      left,
    });
  }, [menuOpen]);

  return (
    <div
      className={`docklite-container-card docklite-neon ${neonClass} p-3 rounded-xl transition-all group relative min-h-[132px] h-full flex flex-col`}
      style={{
        background: 'var(--surface-dim)',
        backdropFilter: 'blur(12px)',
        overflow: 'visible',
      }}
    >
      {/* 3-Dot Menu - Top right */}
      <div
        className="absolute top-2 right-2 z-20 pointer-events-auto"
        ref={menuRef}
        onPointerDown={stopDnd}
        onPointerDownCapture={stopDnd}
      >
        <button
          ref={menuButtonRef}
          onClick={(e) => {
            e.stopPropagation();
            const nextOpen = !menuOpen;
            setMenuOpen(nextOpen);
            onMenuOpenChange?.(nextOpen);
          }}
          onPointerDown={stopDnd}
          className="w-9 h-9 flex items-center justify-center rounded-lg text-sm font-bold transition-all"
          style={{
            background: 'transparent',
            border: `1px solid ${shadowColor}`,
            color: textColor,
            boxShadow: `
              0 0 5px rgba(${rgbVar}, 0.85),
              0 0 10px rgba(${rgbVar}, 0.38),
              inset 0 0 5px rgba(${rgbVar}, 0.25)
            `
          }}
          title="More options"
          aria-label="More options"
        >
          <DotsThree size={16} weight="bold" />
        </button>

        {/* Dropdown Menu */}
        {menuOpen && menuPosition && typeof document !== 'undefined' &&
          createPortal(
            <div
              className="fixed inset-0 z-[10000]"
              onClick={() => {
                setMenuOpen(false);
                onMenuOpenChange?.(false);
              }}
              onPointerDown={stopDnd}
              onPointerDownCapture={stopDnd}
            >
              <div
                ref={menuPopupRef}
                className="absolute rounded-lg overflow-hidden animate-slide-down"
                style={{
                  top: menuPosition.top,
                  left: menuPosition.left,
                  background: 'var(--bg-darker)',
                  border: '1px solid rgba(var(--neon-purple-rgb), 0.9)',
                  boxShadow: '0 0 28px rgba(var(--neon-purple-rgb), 0.65)',
                  width: '200px',
                  maxWidth: 'calc(100vw - 24px)',
                }}
                onClick={(e) => e.stopPropagation()}
                onPointerDown={stopDnd}
                onPointerDownCapture={stopDnd}
              >
              {onViewDetails && (
                <button
                  onClick={(e) => {
                    e.stopPropagation();
                    setMenuOpen(false);
                    onMenuOpenChange?.(false);
                    onViewDetails(container.id, container.name);
                  }}
                  onPointerDown={stopDnd}
                  className="w-full px-4 py-3 text-left text-sm font-bold transition-all hover:bg-white/5 flex items-center gap-3"
                  style={{ color: 'var(--text-cyan)' }}
                >
                  <Eye size={16} weight="duotone" />
                  View Details
                </button>
              )}
              <button
                onClick={(e) => {
                  e.stopPropagation();
                  setMenuOpen(false);
                  onMenuOpenChange?.(false);
                  window.dispatchEvent(new CustomEvent('docklite-open-terminal', {
                    detail: {
                      containerId: container.id,
                      containerName: container.name,
                    },
                  }));
                }}
                onPointerDown={stopDnd}
                className="w-full px-4 py-3 text-left text-sm font-bold transition-all hover:bg-white/5 flex items-center gap-3"
                style={{ color: 'var(--text-green)' }}
              >
                <TerminalWindow size={16} weight="duotone" />
                Open in Terminal
              </button>
              <button
                onClick={async (e) => {
                  e.stopPropagation();
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
                      input.style.pointerEvents = 'none';
                      document.body.appendChild(input);
                      input.focus();
                      input.select();
                      copied = document.execCommand('copy');
                      document.body.removeChild(input);
                    }
                  } catch (err) {
                    copied = false;
                  }

                  if (copied) {
                    toast.success('Copied name');
                  } else {
                    toast.error('Copy failed');
                  }

                  setMenuOpen(false);
                  onMenuOpenChange?.(false);
                }}
                onPointerDown={stopDnd}
                className="w-full px-4 py-3 text-left text-sm font-bold transition-all hover:bg-white/5 flex items-center gap-3"
                style={{ color: 'var(--text-purple)' }}
              >
                <Copy size={16} weight="duotone" />
                Copy name
              </button>
              {canAssign && onAssign && isSite && (
                <button
                  onClick={(e) => {
                    e.stopPropagation();
                    setMenuOpen(false);
                    onMenuOpenChange?.(false);
                    onAssign(container.id, container.name);
                  }}
                  onPointerDown={stopDnd}
                  className="w-full px-4 py-3 text-left text-sm font-bold transition-all flex items-center gap-3"
                  style={{ color: 'var(--text-green)' }}
                  onMouseEnter={(e) => e.currentTarget.style.background = 'var(--surface-dim)'}
                  onMouseLeave={(e) => e.currentTarget.style.background = 'transparent'}
                >
                  <UserCircle size={16} weight="duotone" />
                  Assign to user
                </button>
              )}
              {onMoveFolder && (
                <button
                  onClick={(e) => {
                    e.stopPropagation();
                    setMenuOpen(false);
                    onMenuOpenChange?.(false);
                    onMoveFolder(container.id, container.name);
                  }}
                  onPointerDown={stopDnd}
                  className="w-full px-4 py-3 text-left text-sm font-bold transition-all flex items-center gap-3"
                  style={{ color: 'var(--text-cyan)' }}
                  onMouseEnter={(e) => e.currentTarget.style.background = 'var(--surface-dim)'}
                  onMouseLeave={(e) => e.currentTarget.style.background = 'transparent'}
                >
                  <Folder size={16} weight="duotone" />
                  Move to folder
                </button>
              )}
              {onToggleTracking && (
                <button
                  onClick={(e) => {
                    e.stopPropagation();
                    setMenuOpen(false);
                    onMenuOpenChange?.(false);
                    onToggleTracking(container.id, isTracked);
                  }}
                  onPointerDown={stopDnd}
                  className="w-full px-4 py-3 text-left text-sm font-bold transition-all flex items-center gap-3"
                  style={{ color: isTracked ? 'var(--status-error)' : 'var(--text-green)' }}
                  onMouseEnter={(e) => e.currentTarget.style.background = 'var(--surface-dim)'}
                  onMouseLeave={(e) => e.currentTarget.style.background = 'transparent'}
                >
                  <Eye size={16} weight="duotone" />
                  {isTracked ? 'Untrack' : 'Track'}
                </button>
              )}
              {onDelete && isSite && (
                <button
                  onClick={(e) => {
                    e.stopPropagation();
                    setMenuOpen(false);
                    onMenuOpenChange?.(false);
                    onDelete(container.id, container.labels?.['docklite.domain'] || container.name);
                  }}
                  onPointerDown={stopDnd}
                  className="w-full px-4 py-3 text-left text-sm font-bold transition-all flex items-center gap-3"
                  style={{ color: 'var(--status-error)' }}
                  onMouseEnter={(e) => e.currentTarget.style.background = 'rgba(var(--status-error-rgb), 0.2)'}
                  onMouseLeave={(e) => e.currentTarget.style.background = 'transparent'}
                >
                  <Trash size={16} weight="duotone" />
                  Delete
                </button>
              )}
              </div>
            </div>,
            document.body
          )}
      </div>


      {/* Title: status dot, then the name people know it by (a site's domain), then what it runs */}
      <div className="flex items-start gap-2 pr-10 min-w-0">
        <span
          className="mt-1.5 shrink-0 w-2.5 h-2.5 rounded-full"
          role="img"
          aria-label={isRunning ? 'Running' : 'Stopped'}
          title={isRunning ? 'Running' : 'Stopped'}
          style={{ background: statusColor, boxShadow: isRunning ? `0 0 8px ${statusColor}` : 'none' }}
        />
        <div className="min-w-0 flex-1">
          <h3
            className="docklite-container-name font-bold text-base leading-snug line-clamp-2"
            style={{ color: 'var(--text-cyan)', overflowWrap: 'anywhere' }}
            title={container.name}
          >
            {/* Let a long domain wrap at its dots rather than mid-word */}
            {displayName.split('.').map((part, i) => (
              <span key={i}>{i > 0 && <><wbr />.</>}{part}</span>
            ))}
          </h3>
          <p className="text-xs font-mono truncate" style={{ color: 'var(--text-secondary)' }}>
            {container.image.split(':')[0]}
            {container.owner_username ? ` · ${container.owner_username}` : ''}
          </p>
        </div>
      </div>

      {/* Uptime and port on one line */}
      <div className="flex flex-wrap items-center gap-x-3 gap-y-1 mt-2 mb-3 text-xs font-mono min-w-0">
        <span className="flex items-center gap-1 min-w-0" style={{ color: isRunning ? 'var(--text-secondary)' : 'var(--status-error)' }}>
          <Clock size={14} weight="duotone" className="shrink-0" />
          <span className="truncate">{isRunning ? container.uptime || 'Running' : 'Stopped'}</span>
        </span>
        {ports && (
          <span className="flex items-center gap-1 min-w-0" style={{ color: 'var(--text-purple)' }}>
            <Plug size={14} weight="duotone" className="shrink-0" />
            <span className="truncate">{ports}</span>
          </span>
        )}
      </div>

      {/* Actions */}
      <div className="flex gap-2 mt-auto">
        {!isRunning ? (
          <button
            onClick={(e) => {
              e.stopPropagation();
              onAction(container.id, 'start');
            }}
            onPointerDown={stopDnd}
            className="flex-1 h-9 px-3 rounded-lg text-sm font-bold transition-all group/btn flex items-center justify-center gap-1"
            style={actionStyle}
            title="Start container"
          >
            <Play size={16} weight="duotone" />
            <span>Start</span>
          </button>
        ) : (
          <>
            <button
              onClick={(e) => {
                e.stopPropagation();
                onAction(container.id, 'restart');
              }}
              onPointerDown={stopDnd}
              className="flex-1 h-9 px-3 rounded-lg text-sm font-bold transition-all group/btn flex items-center justify-center"
              style={actionStyle}
              title="Restart container"
              aria-label="Restart container"
            >
              <ArrowsClockwise size={16} weight="duotone" className="group-hover/btn:rotate-180 transition-transform duration-500" />
            </button>
            <button
              onClick={(e) => {
                e.stopPropagation();
                onAction(container.id, 'stop');
              }}
              onPointerDown={stopDnd}
              className="flex-1 h-9 px-3 rounded-lg text-sm font-bold transition-all group/btn flex items-center justify-center"
              style={actionStyle}
              title="Stop container"
              aria-label="Stop container"
            >
              <Stop size={16} weight="duotone" />
            </button>
          </>
        )}
        <button
          onClick={(e) => {
            e.stopPropagation();
            onViewDetails?.(container.id, container.name);
          }}
          onPointerDown={stopDnd}
          className="flex-1 h-9 px-3 rounded-lg text-sm font-bold transition-all group/btn flex items-center justify-center"
          style={actionStyle}
          title="View container details"
          aria-label="View container details"
        >
          <Eye size={16} weight="duotone" />
        </button>
      </div>
    </div>
  );
}
