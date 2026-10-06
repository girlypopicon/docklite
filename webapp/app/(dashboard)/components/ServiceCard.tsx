'use client';

import { Play, Stop, ArrowClockwise, ArrowsClockwise, Scroll, WarningCircle } from '@phosphor-icons/react';

export type ServiceKey = 'docklite' | 'proxy' | 'traefik' | 'legacy-api';
export type ServiceActionName = 'start' | 'stop' | 'restart' | 'reload';

export interface ServiceInfo {
  name: string;
  kind: string;
  status: string;
  detail: string;
  startedAt: string;
  note?: string;
  warning?: string;
  startSupported?: boolean;
  stopSupported?: boolean;
  restartSupported: boolean;
  reloadSupported: boolean;
  logsSupported: boolean;
}

interface Props {
  serviceKey: ServiceKey;
  service: ServiceInfo;
  badge: React.ReactNode;
  startedLabel?: string;
  /** "<service>:<action>" of the action currently running, if any. */
  busy: string | null;
  logsOpen: boolean;
  logsLoading: boolean;
  onAction: (service: ServiceKey, action: ServiceActionName) => void;
  onLogs: (service: ServiceKey) => void;
}

const ACTIONS: { name: ServiceActionName; label: string; busyLabel: string; icon: React.ReactNode; supported: (s: ServiceInfo) => boolean; danger?: boolean }[] = [
  { name: 'start', label: 'Start', busyLabel: 'Starting...', icon: <Play size={12} weight="fill" />, supported: (s) => Boolean(s.startSupported) },
  { name: 'stop', label: 'Stop', busyLabel: 'Stopping...', icon: <Stop size={12} weight="fill" />, supported: (s) => Boolean(s.stopSupported), danger: true },
  { name: 'restart', label: 'Restart', busyLabel: 'Restarting...', icon: <ArrowClockwise size={12} weight="bold" />, supported: (s) => s.restartSupported },
  { name: 'reload', label: 'Reload', busyLabel: 'Reloading...', icon: <ArrowsClockwise size={12} weight="bold" />, supported: (s) => s.reloadSupported },
];

/** One row of the Core Services list: status, plain-language notes, and the actions that are safe for it. */
export default function ServiceCard({ serviceKey, service, badge, startedLabel, busy, logsOpen, logsLoading, onAction, onLogs }: Props) {
  const anyBusy = busy !== null && busy.startsWith(`${serviceKey}:`);

  return (
    <div className="space-y-2">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="min-w-0">
          <div className="text-sm font-bold" style={{ color: 'var(--text-primary)' }}>{service.name}</div>
          <div className="text-xs" style={{ color: 'var(--text-secondary)' }}>{service.detail}</div>
          {startedLabel && (
            <div className="text-[11px]" style={{ color: 'var(--text-muted)' }}>{startedLabel}</div>
          )}
        </div>
        <div className="flex flex-wrap items-center gap-2">
          {badge}
          {ACTIONS.filter((a) => a.supported(service)).map((a) => (
            <button
              key={a.name}
              type="button"
              className="btn-neon px-3 py-1 text-xs font-bold inline-flex items-center gap-1 disabled:opacity-50"
              style={a.danger ? { color: 'var(--status-error)' } : undefined}
              onClick={() => onAction(serviceKey, a.name)}
              disabled={anyBusy}
            >
              {a.icon}
              {busy === `${serviceKey}:${a.name}` ? a.busyLabel : a.label}
            </button>
          ))}
          {service.logsSupported && (
            <button
              type="button"
              className="btn-neon px-3 py-1 text-xs font-bold inline-flex items-center gap-1 disabled:opacity-50"
              onClick={() => onLogs(serviceKey)}
              disabled={logsLoading && logsOpen}
            >
              <Scroll size={12} weight="duotone" />
              Logs
            </button>
          )}
        </div>
      </div>
      {service.note && (
        <div className="text-xs" style={{ color: 'var(--text-secondary)' }}>{service.note}</div>
      )}
      {service.warning && (
        <div
          className="text-xs flex items-start gap-2 p-2 rounded-lg"
          style={{ background: 'rgba(var(--status-warning-rgb), 0.12)', border: '1px solid rgba(var(--status-warning-rgb), 0.4)', color: 'var(--text-primary)' }}
        >
          <WarningCircle size={14} weight="duotone" style={{ color: 'var(--status-warning)', flexShrink: 0, marginTop: 1 }} />
          <span>{service.warning}</span>
        </div>
      )}
    </div>
  );
}
