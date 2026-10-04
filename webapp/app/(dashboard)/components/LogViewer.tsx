'use client';

import { useCallback, useEffect, useLayoutEffect, useRef, useState } from 'react';
import { ArrowDown } from '@phosphor-icons/react';

interface Props {
  text: string;
  loading?: boolean;
  emptyText?: string;
  /** Called by the Refresh button and by Live mode. */
  onRefresh?: () => void;
  className?: string;
}

const LIVE_INTERVAL_MS = 4000;
const NEAR_BOTTOM_PX = 40;

/**
 * A log box that opens on the newest line and stays there as new text
 * arrives — unless you scroll up to read something, in which case it leaves
 * you alone and offers a "Jump to latest" button. Optional Live mode
 * refreshes every few seconds.
 */
export default function LogViewer({ text, loading, emptyText = 'No logs yet.', onRefresh, className = 'max-h-64' }: Props) {
  const ref = useRef<HTMLPreElement>(null);
  // Start pinned to the bottom so the first render shows the latest output.
  const pinned = useRef(true);
  const [atBottom, setAtBottom] = useState(true);
  const [live, setLive] = useState(false);

  const scrollToBottom = useCallback(() => {
    const el = ref.current;
    if (!el) return;
    el.scrollTop = el.scrollHeight;
    pinned.current = true;
    setAtBottom(true);
  }, []);

  // New text while pinned: follow it down. Layout effect so there's no flash
  // of the top of the log before it jumps.
  useLayoutEffect(() => {
    if (pinned.current) scrollToBottom();
  }, [text, scrollToBottom]);

  const onScroll = () => {
    const el = ref.current;
    if (!el) return;
    const near = el.scrollHeight - el.scrollTop - el.clientHeight < NEAR_BOTTOM_PX;
    pinned.current = near;
    setAtBottom(near);
  };

  useEffect(() => {
    if (!live || !onRefresh) return;
    const timer = setInterval(onRefresh, LIVE_INTERVAL_MS);
    return () => clearInterval(timer);
  }, [live, onRefresh]);

  return (
    <div className="relative">
      <div className="flex items-center justify-end gap-3 mb-1 text-xs">
        {onRefresh && (
          <label className="inline-flex items-center gap-1 cursor-pointer select-none" style={{ color: 'var(--text-secondary)' }}>
            <input type="checkbox" checked={live} onChange={(e) => setLive(e.target.checked)} />
            Live
          </label>
        )}
        {onRefresh && (
          <button type="button" className="btn-neon px-3 py-1 text-xs font-bold" onClick={onRefresh}>
            {loading ? 'Loading...' : 'Refresh'}
          </button>
        )}
      </div>
      <pre
        ref={ref}
        onScroll={onScroll}
        className={`text-xs p-3 rounded-lg overflow-auto whitespace-pre-wrap break-words ${className}`}
        style={{ background: 'var(--surface-muted)', color: 'var(--text-primary)' }}
      >
        {text || (loading ? 'Loading logs...' : emptyText)}
      </pre>
      {!atBottom && (
        <button
          type="button"
          onClick={scrollToBottom}
          className="absolute right-4 bottom-3 inline-flex items-center gap-1 px-3 py-1 rounded-full text-xs font-bold"
          style={{ background: 'var(--neon-cyan)', color: 'var(--button-text)', boxShadow: '0 0 10px rgba(var(--neon-cyan-rgb), 0.5)' }}
        >
          <ArrowDown size={12} weight="bold" /> Jump to latest
        </button>
      )}
    </div>
  );
}
