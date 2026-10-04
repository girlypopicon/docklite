'use client';

import { useEffect, useRef, useState } from 'react';
import { ArrowClockwise, Lightning } from '@phosphor-icons/react';
import TerminalContainerPicker from './TerminalContainerPicker';
import { Terminal } from 'xterm';
import { FitAddon } from 'xterm-addon-fit';
import 'xterm/css/xterm.css';

type XtermDrawerProps = {
  open: boolean;
  onClose: () => void;
  containerId?: string;
  containerName?: string;
  /** Called when a container is chosen from the picker in the drawer's header. */
  onSelectTarget?: (id: string, name: string) => void;
};

type ConnectionStatus = 'idle' | 'connecting' | 'connected' | 'closed' | 'error';

const STATUS_LABEL: Record<ConnectionStatus, { text: string; color: string }> = {
  idle: { text: 'Not connected', color: 'var(--text-secondary)' },
  connecting: { text: 'Connecting…', color: 'var(--status-warning)' },
  connected: { text: 'Connected', color: 'var(--neon-green)' },
  closed: { text: 'Disconnected', color: 'var(--status-error)' },
  error: { text: 'Connection failed', color: 'var(--status-error)' },
};

export default function XtermDrawer({ open, onClose, containerId, containerName, onSelectTarget }: XtermDrawerProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const terminalRef = useRef<Terminal | null>(null);
  const fitAddonRef = useRef<FitAddon | null>(null);
  const socketRef = useRef<WebSocket | null>(null);
  const inputDisposableRef = useRef<{ dispose: () => void } | null>(null);
  const [visible, setVisible] = useState(open);
  const [closing, setClosing] = useState(false);
  const [status, setStatus] = useState<ConnectionStatus>('idle');
  // Bumped by Reconnect to run the connection effect again for the same container.
  const [session, setSession] = useState(0);

  useEffect(() => {
    if (open) {
      setVisible(true);
      setClosing(false);
      return;
    }

    if (!visible) return;
    setClosing(true);
    const timer = window.setTimeout(() => {
      setVisible(false);
      setClosing(false);
    }, 300);
    return () => window.clearTimeout(timer);
  }, [open, visible]);

  useEffect(() => {
    if (!containerRef.current || terminalRef.current) return;

    const rootStyles = getComputedStyle(document.documentElement);
    const readVar = (name: string, fallback: string) => rootStyles.getPropertyValue(name).trim() || fallback;
    const successRgb = readVar('--status-success-rgb', '0, 232, 99');

    const terminal = new Terminal({
      cursorBlink: true,
      fontFamily: '"JetBrains Mono", ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace',
      fontSize: 13,
      theme: {
        background: readVar('--bg-darker', '#070312'),
        foreground: readVar('--text-primary', '#b6ffe3'),
        cursor: readVar('--status-success', '#00e863'),
        selectionBackground: `rgba(${successRgb}, 0.2)`,
      },
    });
    const fitAddon = new FitAddon();
    terminal.loadAddon(fitAddon);
    terminal.open(containerRef.current);
    fitAddon.fit();
    terminal.writeln('DockLite Terminal');
    terminal.writeln('Select a container to connect.');

    terminalRef.current = terminal;
    fitAddonRef.current = fitAddon;

    return () => {};
  }, []);

  useEffect(() => {
    if (open && fitAddonRef.current) {
      fitAddonRef.current.fit();
    }
  }, [open]);

  useEffect(() => {
    if (!open) {
      if (socketRef.current) {
        socketRef.current.close();
        socketRef.current = null;
      }
      if (inputDisposableRef.current) {
        inputDisposableRef.current.dispose();
        inputDisposableRef.current = null;
      }
      return;
    }

    if (!terminalRef.current) {
      return;
    }

    if (!containerId) {
      setStatus('idle');
      if (socketRef.current) {
        socketRef.current.close();
        socketRef.current = null;
      }
      if (inputDisposableRef.current) {
        inputDisposableRef.current.dispose();
        inputDisposableRef.current = null;
      }
      terminalRef.current.reset();
      terminalRef.current.writeln('DockLite Terminal');
      terminalRef.current.writeln('Select a container to connect.');
      return;
    }

    if (socketRef.current) {
      socketRef.current.close();
      socketRef.current = null;
    }
    if (inputDisposableRef.current) {
      inputDisposableRef.current.dispose();
      inputDisposableRef.current = null;
    }

    setStatus('connecting');
    terminalRef.current.reset();
    terminalRef.current.writeln(`Connecting to ${containerName || containerId}...`);

    if (fitAddonRef.current) {
      fitAddonRef.current.fit();
    }

    const protocol = window.location.protocol === 'https:' ? 'wss' : 'ws';
    const socket = new WebSocket(`${protocol}://${window.location.host}/api/containers/${containerId}/terminal`);
    socket.binaryType = 'arraybuffer';
    socketRef.current = socket;

    const sendResize = () => {
      if (!terminalRef.current || socket.readyState !== WebSocket.OPEN) return;
      socket.send(JSON.stringify({
        type: 'resize',
        cols: terminalRef.current.cols,
        rows: terminalRef.current.rows,
      }));
    };

    socket.addEventListener('open', () => {
      setStatus('connected');
      sendResize();
      terminalRef.current?.focus();
    });

    socket.addEventListener('message', (event) => {
      if (!terminalRef.current) return;
      if (typeof event.data === 'string') {
        terminalRef.current.write(event.data);
        return;
      }
      if (event.data instanceof ArrayBuffer) {
        terminalRef.current.write(new Uint8Array(event.data));
      }
    });

    socket.addEventListener('close', () => {
      setStatus((prev) => (prev === 'error' ? prev : 'closed'));
      if (!terminalRef.current) return;
      terminalRef.current.writeln('');
      terminalRef.current.writeln('Disconnected. Use Reconnect, or pick another container.');
    });

    socket.addEventListener('error', () => {
      setStatus('error');
      if (!terminalRef.current) return;
      terminalRef.current.writeln('');
      terminalRef.current.writeln('Could not open a shell. The container may have stopped, or it has no shell (sh/bash) installed.');
    });

    inputDisposableRef.current = terminalRef.current.onData((data) => {
      if (socket.readyState === WebSocket.OPEN) {
        socket.send(data);
      }
    });

    const handleResize = () => {
      if (!fitAddonRef.current) return;
      fitAddonRef.current.fit();
      sendResize();
    };

    window.addEventListener('resize', handleResize);
    return () => {
      window.removeEventListener('resize', handleResize);
      if (socketRef.current) {
        socketRef.current.close();
        socketRef.current = null;
      }
      if (inputDisposableRef.current) {
        inputDisposableRef.current.dispose();
        inputDisposableRef.current = null;
      }
    };
  }, [open, containerId, containerName, session]);

  if (!visible) return null;

  return (
    <div
      className={`fixed inset-0 z-[10000] ${open ? 'pointer-events-auto' : 'pointer-events-none'}`}
    >
      <div
        className={`absolute inset-0 z-0 transition-opacity duration-300 ${
          open ? 'opacity-100 pointer-events-auto' : 'opacity-0 pointer-events-none'
        }`}
        style={{ background: 'rgba(var(--text-muted-rgb), 0.45)' }}
        onClick={onClose}
      />
      <div
        className={`absolute inset-x-0 bottom-0 z-10 transition-transform duration-500 ${
          open ? 'translate-y-0' : 'translate-y-full'
        } ${open ? 'pointer-events-auto' : 'pointer-events-none'} ${closing ? 'translate-y-full' : ''}`}
        style={{ transitionTimingFunction: 'cubic-bezier(0.22, 1, 0.36, 1)' }}
      >
        <div className="mx-auto max-w-[1200px]">
          <div
            className="rounded-t-2xl border-2 shadow-2xl"
            style={{
              background: 'linear-gradient(135deg, var(--modal-bg-1) 0%, var(--modal-bg-2) 100%)',
              borderColor: 'rgba(var(--status-success-rgb), 0.4)',
              boxShadow: '0 0 30px rgba(var(--status-success-rgb), 0.25)',
            }}
          >
            <div
              className="flex items-center justify-between px-4 py-3 border-b"
              style={{ borderColor: 'rgba(var(--status-success-rgb), 0.2)' }}
            >
              <div className="flex flex-wrap items-center gap-3">
                <div className="text-sm font-bold flex items-center gap-2" style={{ color: 'var(--neon-green)' }}>
                  <Lightning size={16} weight="duotone" />
                  Terminal
                </div>
                <TerminalContainerPicker
                  currentId={containerId}
                  currentName={containerName}
                  onSelect={(id, name) => onSelectTarget?.(id, name)}
                />
                {containerId && (
                  <>
                    <span className="inline-flex items-center gap-1 text-[11px] font-bold" style={{ color: STATUS_LABEL[status].color }}>
                      <span className="inline-block w-2 h-2 rounded-full" style={{ background: STATUS_LABEL[status].color }} />
                      {STATUS_LABEL[status].text}
                    </span>
                    {(status === 'closed' || status === 'error') && (
                      <button
                        type="button"
                        onClick={() => setSession((n) => n + 1)}
                        className="inline-flex items-center gap-1 px-2 py-1 text-[11px] font-bold rounded-lg border"
                        style={{ borderColor: 'rgba(var(--status-success-rgb), 0.5)', color: 'var(--neon-green)' }}
                      >
                        <ArrowClockwise size={12} weight="bold" /> Reconnect
                      </button>
                    )}
                  </>
                )}
              </div>
              <button
                onClick={onClose}
                className="px-3 py-1 text-xs font-bold rounded-lg transition-all hover:scale-105"
                style={{
                  background: 'rgba(var(--status-error-rgb), 0.15)',
                  color: 'var(--status-error)',
                  border: '1px solid rgba(var(--status-error-rgb), 0.4)',
                }}
              >
                Close
              </button>
            </div>
            <div
              className="h-[40vh] sm:h-[45vh] border-t"
              style={{
                borderColor: 'rgba(var(--status-success-rgb), 0.2)',
                boxShadow: 'inset 0 -8px 20px rgba(0, 0, 0, 0.45)',
              }}
            >
              <div className="relative h-full w-full">
                <div ref={containerRef} className="h-full w-full" />
                {!containerId && (
                  <div className="absolute inset-0 flex flex-col items-center justify-center gap-3 text-center p-6" style={{ background: 'rgba(0, 0, 0, 0.55)' }}>
                    <div className="text-sm font-bold" style={{ color: 'var(--neon-green)' }}>Open a shell inside one of your containers</div>
                    <p className="text-xs max-w-md opacity-80" style={{ color: 'var(--text-primary)' }}>
                      You get a real command line inside the container — move around, read files, run commands — as if it were its own Linux machine.
                    </p>
                    <TerminalContainerPicker inline onSelect={(id, name) => onSelectTarget?.(id, name)} />
                  </div>
                )}
              </div>
            </div>
            <div className="h-2 w-full" style={{ background: 'linear-gradient(90deg, rgba(var(--status-success-rgb), 0.5), rgba(var(--status-info-rgb), 0.5))' }} />
          </div>
        </div>
      </div>
    </div>
  );
}
