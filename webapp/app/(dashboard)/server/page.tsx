'use client';

import { useCallback, useEffect, useMemo, useState } from 'react';
import {
  DesktopTower,
  Info,
  Gauge,
  Clock,
  HardDrives,
  ShieldCheck,
  Gear,
  Package,
  TerminalWindow,
  DownloadSimple,
  WarningCircle,
  CheckCircle,
  ArrowClockwise,
  SpinnerGap,
} from '@phosphor-icons/react';
import NginxFlowPanel from './NginxFlowPanel';

type LoadAvg = {
  one: number;
  five: number;
  fifteen: number;
};

type MemorySummary = {
  total: number;
  free: number;
};

type DiskSummary = {
  total: number;
  used: number;
  free: number;
};

type TimeSync = {
  status: string;
  detail: string;
  timezone: string;
};

type OverviewResponse = {
  hostname: string;
  os: string;
  osVersion: string;
  kernel: string;
  arch: string;
  cpuCount: number;
  uptime: number;
  loadAvg: LoadAvg;
  memory: MemorySummary;
  disk: DiskSummary;
  timeSync: TimeSync;
  clockIso: string;
};

type AutoUpdates = {
  status: string;
  detail: string;
};

type UpdatesResponse = {
  pendingUpdates: number;
  securityUpdates: number;
  rebootRequired: boolean;
  autoUpdates: AutoUpdates;
  source: string;
};

type ServiceStatus = {
  name: string;
  kind: string;
  status: string;
  detail: string;
  startedAt: string;
  restartSupported: boolean;
  reloadSupported: boolean;
  logsSupported: boolean;
};

type ServicesResponse = {
  docker: ServiceStatus;
  docklite: ServiceStatus | null;
  dockliteSecondary: ServiceStatus | null;
  proxy: ServiceStatus | null;
};

type MountUsage = {
  filesystem: string;
  type: string;
  size: number;
  used: number;
  available: number;
  usePercent: number;
  mountpoint: string;
};

type DockerUsage = {
  imageCount: number;
  containerCount: number;
  volumeCount: number;
  buildCacheCount: number;
  imageSize: number;
  containerSize: number;
  volumeSize: number;
  buildCacheSize: number;
  totalSize: number;
};

type DockerVolume = {
  name: string;
  driver: string;
  mount: string;
  size: number;
  refCount: number;
  createdAt: string;
};

type StorageResponse = {
  mounts: MountUsage[];
  docker: DockerUsage;
  volumes: DockerVolume[];
};

type SSHStatus = {
  status: string;
  unit: string;
  ports: string[];
};

type FailedLogins = {
  count: number;
  latest: string;
  source: string;
};

type SecurityResponse = {
  ssh: SSHStatus;
  sudoUsers: string[];
  failedLogins: FailedLogins;
};

const formatUptime = (seconds: number) => {
  if (!seconds) return '—';
  const days = Math.floor(seconds / 86400);
  const hours = Math.floor((seconds % 86400) / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  if (days > 0) return `${days}d ${hours}h`;
  if (hours > 0) return `${hours}h ${minutes}m`;
  return `${minutes}m`;
};

const formatBytes = (bytes: number) => {
  if (!bytes) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${Math.round((bytes / Math.pow(k, i)) * 100) / 100} ${sizes[i]}`;
};

const formatDateTime = (value: string) => {
  if (!value) return '—';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString();
};

const statusBadge = (status: string) => {
  const normalized = (status || '').toLowerCase();
  if (['running', 'active', 'up'].includes(normalized)) {
    return <span className="badge-running">RUNNING</span>;
  }
  if (['inactive', 'dead', 'exited', 'failed', 'unavailable'].includes(normalized)) {
    return <span className="badge-stopped">STOPPED</span>;
  }
  return (
    <span
      className="text-xs font-bold px-3 py-1 rounded-full"
      style={{ border: '1px solid rgba(var(--text-muted-rgb), 0.2)', color: 'var(--text-secondary)' }}
    >
      {status ? status.toUpperCase() : 'UNKNOWN'}
    </span>
  );
};

export default function ServerPage() {
  const [overview, setOverview] = useState<OverviewResponse | null>(null);
  const [updates, setUpdates] = useState<UpdatesResponse | null>(null);
  const [services, setServices] = useState<ServicesResponse | null>(null);
  const [storage, setStorage] = useState<StorageResponse | null>(null);
  const [security, setSecurity] = useState<SecurityResponse | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [accessDenied, setAccessDenied] = useState(false);

  const [loadingOverview, setLoadingOverview] = useState(true);
  const [loadingUpdates, setLoadingUpdates] = useState(true);
  const [loadingServices, setLoadingServices] = useState(true);
  const [loadingStorage, setLoadingStorage] = useState(true);
  const [loadingSecurity, setLoadingSecurity] = useState(true);

  const [serviceAction, setServiceAction] = useState<string | null>(null);
  const [serviceLogsTarget, setServiceLogsTarget] = useState<'docklite' | 'proxy' | null>(null);
  const [serviceLogs, setServiceLogs] = useState('');
  const [loadingServiceLogs, setLoadingServiceLogs] = useState(false);

  const [systemLogs, setSystemLogs] = useState('');
  const [dockliteLogs, setDockliteLogs] = useState('');
  const [loadingSystemLogs, setLoadingSystemLogs] = useState(false);
  const [loadingDockliteLogs, setLoadingDockliteLogs] = useState(false);

  const fetchOverview = useCallback(async () => {
    if (accessDenied) return;
    setLoadingOverview(true);
    try {
      const res = await fetch('/api/server/overview');
      if (res.status === 403) {
        setAccessDenied(true);
        return;
      }
      if (!res.ok) throw new Error('Failed to load server overview');
      setOverview(await res.json());
    } catch (err: any) {
      setError(err.message || 'Failed to load server overview');
    } finally {
      setLoadingOverview(false);
    }
  }, [accessDenied]);

  const fetchUpdates = useCallback(async () => {
    if (accessDenied) return;
    setLoadingUpdates(true);
    try {
      const res = await fetch('/api/server/updates');
      if (res.status === 403) {
        setAccessDenied(true);
        return;
      }
      if (!res.ok) throw new Error('Failed to load updates status');
      setUpdates(await res.json());
    } catch (err: any) {
      setError(err.message || 'Failed to load updates status');
    } finally {
      setLoadingUpdates(false);
    }
  }, [accessDenied]);

  const fetchServices = useCallback(async () => {
    if (accessDenied) return;
    setLoadingServices(true);
    try {
      const res = await fetch('/api/server/services');
      if (res.status === 403) {
        setAccessDenied(true);
        return;
      }
      if (!res.ok) throw new Error('Failed to load services');
      setServices(await res.json());
    } catch (err: any) {
      setError(err.message || 'Failed to load services');
    } finally {
      setLoadingServices(false);
    }
  }, [accessDenied]);

  const fetchStorage = useCallback(async () => {
    if (accessDenied) return;
    setLoadingStorage(true);
    try {
      const res = await fetch('/api/server/storage');
      if (res.status === 403) {
        setAccessDenied(true);
        return;
      }
      if (!res.ok) throw new Error('Failed to load storage');
      setStorage(await res.json());
    } catch (err: any) {
      setError(err.message || 'Failed to load storage');
    } finally {
      setLoadingStorage(false);
    }
  }, [accessDenied]);

  const fetchSecurity = useCallback(async () => {
    if (accessDenied) return;
    setLoadingSecurity(true);
    try {
      const res = await fetch('/api/server/security');
      if (res.status === 403) {
        setAccessDenied(true);
        return;
      }
      if (!res.ok) throw new Error('Failed to load security status');
      setSecurity(await res.json());
    } catch (err: any) {
      setError(err.message || 'Failed to load security status');
    } finally {
      setLoadingSecurity(false);
    }
  }, [accessDenied]);

  useEffect(() => {
    fetchOverview();
    fetchUpdates();
    fetchServices();
    fetchStorage();
    fetchSecurity();
  }, [fetchOverview, fetchUpdates, fetchServices, fetchStorage, fetchSecurity]);

  const memoryUsedPercent = useMemo(() => {
    if (!overview || overview.memory.total === 0) return 0;
    return Math.round(((overview.memory.total - overview.memory.free) / overview.memory.total) * 100);
  }, [overview]);

  const diskUsedPercent = useMemo(() => {
    if (!overview || overview.disk.total === 0) return 0;
    return Math.round((overview.disk.used / overview.disk.total) * 100);
  }, [overview]);

  const handleServiceAction = async (service: 'docklite' | 'proxy', action: 'restart' | 'reload') => {
    setServiceAction(`${service}:${action}`);
    try {
      const res = await fetch('/api/server/services/action', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ service, action }),
      });
      if (res.status === 403) {
        setAccessDenied(true);
        return;
      }
      if (!res.ok) throw new Error('Service action failed');
      await fetchServices();
    } catch (err: any) {
      setError(err.message || 'Service action failed');
    } finally {
      setServiceAction(null);
    }
  };

  const handleServiceLogs = async (target: 'docklite' | 'proxy') => {
    setServiceLogsTarget(target);
    setLoadingServiceLogs(true);
    try {
      const res = await fetch(`/api/server/logs?target=${target}&tail=200`);
      if (res.status === 403) {
        setAccessDenied(true);
        return;
      }
      if (!res.ok) throw new Error('Failed to load service logs');
      const data = await res.json();
      setServiceLogs(data.logs || '');
    } catch (err: any) {
      setError(err.message || 'Failed to load service logs');
    } finally {
      setLoadingServiceLogs(false);
    }
  };

  const handleSystemLogs = async () => {
    setLoadingSystemLogs(true);
    try {
      const res = await fetch('/api/server/logs?target=system&tail=200');
      if (res.status === 403) {
        setAccessDenied(true);
        return;
      }
      if (!res.ok) throw new Error('Failed to load system logs');
      const data = await res.json();
      setSystemLogs(data.logs || '');
    } catch (err: any) {
      setError(err.message || 'Failed to load system logs');
    } finally {
      setLoadingSystemLogs(false);
    }
  };

  const handleDockliteLogs = async () => {
    setLoadingDockliteLogs(true);
    try {
      const res = await fetch('/api/server/logs?target=docklite&tail=200');
      if (res.status === 403) {
        setAccessDenied(true);
        return;
      }
      if (!res.ok) throw new Error('Failed to load DockLite logs');
      const data = await res.json();
      setDockliteLogs(data.logs || '');
    } catch (err: any) {
      setError(err.message || 'Failed to load DockLite logs');
    } finally {
      setLoadingDockliteLogs(false);
    }
  };

  const handlePrune = async (target: 'images' | 'build-cache') => {
    const confirmLabel = target === 'images' ? 'prune unused images' : 'prune build cache';
    if (!window.confirm(`Are you sure you want to ${confirmLabel}?`)) return;
    try {
      const res = await fetch('/api/server/storage/prune', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ target }),
      });
      if (res.status === 403) {
        setAccessDenied(true);
        return;
      }
      if (!res.ok) throw new Error('Prune failed');
      await fetchStorage();
    } catch (err: any) {
      setError(err.message || 'Prune failed');
    }
  };

  const downloadDiagnostics = () => {
    window.location.href = '/api/server/diagnostics';
  };

  return (
    <div className="max-w-[1400px] mx-auto">
      <div className="sm:flex sm:items-center sm:justify-between mb-6">
        <div>
          <h1 className="text-3xl font-bold neon-text flex items-center gap-2" style={{ color: 'var(--neon-green)' }}>
            <DesktopTower size={22} weight="duotone" />
            Server
          </h1>
          <p className="mt-1 text-sm font-mono" style={{ color: 'var(--text-secondary)' }}>
            Host overview, updates, services, and diagnostics.
          </p>
        </div>
      </div>

      {accessDenied && (
        <div className="card-vapor p-6 rounded-xl flex items-center gap-3 mb-6" style={{ borderColor: 'rgba(var(--text-muted-rgb), 0.15)' }}>
          <ShieldCheck size={22} weight="duotone" style={{ color: 'var(--neon-cyan)' }} />
          <div>
            <div className="text-lg font-bold" style={{ color: 'var(--text-primary)' }}>Admin access required</div>
            <div className="text-sm" style={{ color: 'var(--text-secondary)' }}>
              The Server tab is restricted to admin users. Switch to an admin account to view host status and controls.
            </div>
          </div>
        </div>
      )}

      {error && (
        <div className="mb-6 card-vapor p-4 rounded-xl flex items-center gap-3" style={{ borderColor: 'rgba(var(--status-error-rgb), 0.6)' }}>
          <WarningCircle size={18} weight="duotone" style={{ color: 'var(--status-error)' }} />
          <span className="text-sm" style={{ color: 'var(--text-primary)' }}>{error}</span>
        </div>
      )}

      {accessDenied ? null : (
        <>
          <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        <div className="card-vapor p-6 rounded-xl">
          <h2 className="text-xl font-bold mb-4 neon-text flex items-center gap-2" style={{ color: 'var(--neon-cyan)' }}>
            <Info size={18} weight="duotone" />
            Overview
          </h2>
          {loadingOverview || !overview ? (
            <div className="text-sm flex items-center gap-2" style={{ color: 'var(--text-secondary)' }}>
              <SpinnerGap size={16} className="animate-spin" /> Loading overview...
            </div>
          ) : (
            <dl className="space-y-3">
              <div className="flex justify-between items-center">
                <dt className="text-sm font-bold" style={{ color: 'var(--neon-purple)' }}>HOSTNAME</dt>
                <dd className="text-sm font-mono" style={{ color: 'var(--text-primary)' }}>{overview.hostname}</dd>
              </div>
              <div className="flex justify-between items-center">
                <dt className="text-sm font-bold" style={{ color: 'var(--neon-purple)' }}>OS</dt>
                <dd className="text-sm font-mono" style={{ color: 'var(--text-primary)' }}>{overview.os || '—'} {overview.osVersion}</dd>
              </div>
              <div className="flex justify-between items-center">
                <dt className="text-sm font-bold" style={{ color: 'var(--neon-purple)' }}>KERNEL</dt>
                <dd className="text-sm font-mono" style={{ color: 'var(--text-primary)' }}>{overview.kernel || '—'}</dd>
              </div>
              <div className="flex justify-between items-center">
                <dt className="text-sm font-bold" style={{ color: 'var(--neon-purple)' }}>UPTIME</dt>
                <dd className="text-sm font-mono" style={{ color: 'var(--neon-green)' }}>{formatUptime(overview.uptime)}</dd>
              </div>
              <div className="flex justify-between items-center">
                <dt className="text-sm font-bold" style={{ color: 'var(--neon-purple)' }}>TIME SYNC</dt>
                <dd className="text-sm font-mono" style={{ color: 'var(--text-primary)' }}>
                  {overview.timeSync.status} ({overview.timeSync.timezone || 'UTC'})
                </dd>
              </div>
              <div className="flex justify-between items-center">
                <dt className="text-sm font-bold" style={{ color: 'var(--neon-purple)' }}>CLOCK</dt>
                <dd className="text-sm font-mono" style={{ color: 'var(--text-primary)' }}>{formatDateTime(overview.clockIso)}</dd>
              </div>
            </dl>
          )}
        </div>

        <div className="card-vapor p-6 rounded-xl">
          <h2 className="text-xl font-bold mb-4 neon-text flex items-center gap-2" style={{ color: 'var(--neon-pink)' }}>
            <Gauge size={18} weight="duotone" />
            Performance
          </h2>
          {loadingOverview || !overview ? (
            <div className="text-sm flex items-center gap-2" style={{ color: 'var(--text-secondary)' }}>
              <SpinnerGap size={16} className="animate-spin" /> Loading metrics...
            </div>
          ) : (
            <div className="space-y-4">
              <div className="flex justify-between items-center">
                <span className="text-sm font-bold" style={{ color: 'var(--neon-purple)' }}>CPU</span>
                <span className="text-sm font-mono" style={{ color: 'var(--text-primary)' }}>
                  {overview.cpuCount} cores
                </span>
              </div>
              <div className="flex justify-between items-center">
                <span className="text-sm font-bold" style={{ color: 'var(--neon-purple)' }}>LOAD</span>
                <span className="text-sm font-mono" style={{ color: 'var(--text-primary)' }}>
                  {overview.loadAvg.one.toFixed(2)} / {overview.loadAvg.five.toFixed(2)} / {overview.loadAvg.fifteen.toFixed(2)}
                </span>
              </div>
              <div>
                <div className="flex justify-between items-center mb-2">
                  <span className="text-sm font-bold" style={{ color: 'var(--neon-purple)' }}>MEMORY</span>
                  <span className="text-xs font-mono" style={{ color: 'var(--text-secondary)' }}>
                    {formatBytes(overview.memory.total - overview.memory.free)} / {formatBytes(overview.memory.total)}
                  </span>
                </div>
                <div className="overflow-hidden h-3 rounded-lg" style={{ background: 'var(--surface-muted)', border: '1px solid rgba(var(--neon-pink-rgb), 0.3)' }}>
                  <div
                    className="h-full transition-all duration-500"
                    style={{
                      width: `${memoryUsedPercent}%`,
                      background: 'linear-gradient(90deg, var(--neon-pink) 0%, var(--neon-purple) 100%)',
                      boxShadow: '0 0 10px var(--neon-pink)'
                    }}
                  ></div>
                </div>
              </div>
              <div>
                <div className="flex justify-between items-center mb-2">
                  <span className="text-sm font-bold" style={{ color: 'var(--neon-purple)' }}>DISK</span>
                  <span className="text-xs font-mono" style={{ color: 'var(--text-secondary)' }}>
                    {formatBytes(overview.disk.used)} / {formatBytes(overview.disk.total)}
                  </span>
                </div>
                <div className="overflow-hidden h-3 rounded-lg" style={{ background: 'var(--surface-muted)', border: '1px solid rgba(var(--status-success-rgb), 0.3)' }}>
                  <div
                    className="h-full transition-all duration-500"
                    style={{
                      width: `${diskUsedPercent}%`,
                      background: 'linear-gradient(90deg, var(--neon-green) 0%, var(--neon-cyan) 100%)',
                      boxShadow: '0 0 10px var(--neon-green)'
                    }}
                  ></div>
                </div>
              </div>
            </div>
          )}
        </div>

        <div className="card-vapor p-6 rounded-xl">
          <h2 className="text-xl font-bold mb-4 neon-text flex items-center gap-2" style={{ color: 'var(--neon-green)' }}>
            <Clock size={18} weight="duotone" />
            Updates
          </h2>
          {loadingUpdates || !updates ? (
            <div className="text-sm flex items-center gap-2" style={{ color: 'var(--text-secondary)' }}>
              <SpinnerGap size={16} className="animate-spin" /> Loading updates...
            </div>
          ) : (
            <div className="space-y-4">
              <div className="flex justify-between items-center">
                <span className="text-sm font-bold" style={{ color: 'var(--neon-purple)' }}>PENDING</span>
                <span className="text-sm font-mono" style={{ color: 'var(--text-primary)' }}>{updates.pendingUpdates}</span>
              </div>
              <div className="flex justify-between items-center">
                <span className="text-sm font-bold" style={{ color: 'var(--neon-purple)' }}>SECURITY</span>
                <span className="text-sm font-mono" style={{ color: 'var(--text-primary)' }}>{updates.securityUpdates}</span>
              </div>
              <div className="flex items-center gap-2">
                {updates.rebootRequired ? (
                  <WarningCircle size={16} weight="duotone" style={{ color: 'var(--status-error)' }} />
                ) : (
                  <CheckCircle size={16} weight="duotone" style={{ color: 'var(--neon-green)' }} />
                )}
                <span className="text-sm" style={{ color: 'var(--text-primary)' }}>
                  {updates.rebootRequired ? 'Reboot required' : 'No reboot needed'}
                </span>
              </div>
              <div>
                <div className="text-xs font-bold" style={{ color: 'var(--neon-purple)' }}>AUTO-UPDATES</div>
                <div className="text-sm font-mono" style={{ color: 'var(--text-primary)' }}>{updates.autoUpdates.status}</div>
                <div className="text-xs" style={{ color: 'var(--text-secondary)' }}>{updates.autoUpdates.detail}</div>
              </div>
            </div>
          )}
        </div>
      </div>

          <div className="mt-8">
            <NginxFlowPanel />
          </div>

          <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 mt-8">
        <div className="card-vapor p-6 rounded-xl">
          <h2 className="text-xl font-bold mb-4 neon-text flex items-center gap-2" style={{ color: 'var(--neon-cyan)' }}>
            <Gear size={18} weight="duotone" />
            Core Services
          </h2>
          {loadingServices || !services ? (
            <div className="text-sm flex items-center gap-2" style={{ color: 'var(--text-secondary)' }}>
              <SpinnerGap size={16} className="animate-spin" /> Loading services...
            </div>
          ) : (
            <div className="space-y-4">
              <div className="flex items-center justify-between">
                <div>
                  <div className="text-sm font-bold" style={{ color: 'var(--text-primary)' }}>{services.docker.name}</div>
                  <div className="text-xs" style={{ color: 'var(--text-secondary)' }}>{services.docker.detail}</div>
                </div>
                {statusBadge(services.docker.status)}
              </div>

              {services.docklite ? (
                <div className="border-t border-white/10 pt-4">
                  <div className="flex items-start justify-between">
                    <div>
                      <div className="text-sm font-bold" style={{ color: 'var(--text-primary)' }}>{services.docklite.name}</div>
                      <div className="text-xs" style={{ color: 'var(--text-secondary)' }}>{services.docklite.detail}</div>
                      {services.docklite.startedAt && (
                        <div className="text-xs" style={{ color: 'var(--text-secondary)' }}>
                          Started {formatDateTime(services.docklite.startedAt)}
                        </div>
                      )}
                    </div>
                    <div className="flex items-center gap-2">
                      {statusBadge(services.docklite.status)}
                      {services.docklite.restartSupported && (
                        <button
                          className="btn-neon px-3 py-1 text-xs font-bold"
                          onClick={() => handleServiceAction('docklite', 'restart')}
                          disabled={serviceAction === 'docklite:restart'}
                        >
                          {serviceAction === 'docklite:restart' ? 'Restarting...' : 'Restart'}
                        </button>
                      )}
                      {services.docklite.logsSupported && (
                        <button
                          className="btn-neon px-3 py-1 text-xs font-bold"
                          onClick={() => handleServiceLogs('docklite')}
                          disabled={loadingServiceLogs && serviceLogsTarget === 'docklite'}
                        >
                          Logs
                        </button>
                      )}
                    </div>
                  </div>
                </div>
              ) : (
                <div className="border-t border-white/10 pt-4 text-sm" style={{ color: 'var(--text-secondary)' }}>
                  DockLite service not detected.
                </div>
              )}

              {services.dockliteSecondary && (
                <div className="border-t border-white/10 pt-4">
                  <div className="flex items-center justify-between">
                    <div>
                      <div className="text-sm font-bold" style={{ color: 'var(--text-primary)' }}>
                        {services.dockliteSecondary.name} (secondary)
                      </div>
                      <div className="text-xs" style={{ color: 'var(--text-secondary)' }}>{services.dockliteSecondary.detail}</div>
                    </div>
                    {statusBadge(services.dockliteSecondary.status)}
                  </div>
                </div>
              )}

              {services.proxy ? (
                <div className="border-t border-white/10 pt-4">
                  <div className="flex items-start justify-between">
                    <div>
                      <div className="text-sm font-bold" style={{ color: 'var(--text-primary)' }}>{services.proxy.name}</div>
                      <div className="text-xs" style={{ color: 'var(--text-secondary)' }}>{services.proxy.detail}</div>
                      {services.proxy.startedAt && (
                        <div className="text-xs" style={{ color: 'var(--text-secondary)' }}>
                          Started {formatDateTime(services.proxy.startedAt)}
                        </div>
                      )}
                    </div>
                    <div className="flex items-center gap-2">
                      {statusBadge(services.proxy.status)}
                      {services.proxy.restartSupported && (
                        <button
                          className="btn-neon px-3 py-1 text-xs font-bold"
                          onClick={() => handleServiceAction('proxy', 'restart')}
                          disabled={serviceAction === 'proxy:restart'}
                        >
                          {serviceAction === 'proxy:restart' ? 'Restarting...' : 'Restart'}
                        </button>
                      )}
                      {services.proxy.reloadSupported && (
                        <button
                          className="btn-neon px-3 py-1 text-xs font-bold"
                          onClick={() => handleServiceAction('proxy', 'reload')}
                          disabled={serviceAction === 'proxy:reload'}
                        >
                          {serviceAction === 'proxy:reload' ? 'Reloading...' : 'Reload'}
                        </button>
                      )}
                      {services.proxy.logsSupported && (
                        <button
                          className="btn-neon px-3 py-1 text-xs font-bold"
                          onClick={() => handleServiceLogs('proxy')}
                          disabled={loadingServiceLogs && serviceLogsTarget === 'proxy'}
                        >
                          Logs
                        </button>
                      )}
                    </div>
                  </div>
                </div>
              ) : (
                <div className="border-t border-white/10 pt-4 text-sm" style={{ color: 'var(--text-secondary)' }}>
                  Proxy service not detected.
                </div>
              )}

              {serviceLogsTarget && (
                <div className="mt-4">
                  <div className="text-xs font-bold mb-2" style={{ color: 'var(--neon-purple)' }}>
                    {serviceLogsTarget.toUpperCase()} LOGS
                  </div>
                  <pre
                    className="text-xs p-3 rounded-lg overflow-auto max-h-48"
                    style={{ background: 'var(--surface-muted)', color: 'var(--text-primary)' }}
                  >
                    {loadingServiceLogs ? 'Loading logs...' : serviceLogs || 'No logs available'}
                  </pre>
                </div>
              )}
            </div>
          )}
        </div>

        <div className="card-vapor p-6 rounded-xl">
          <h2 className="text-xl font-bold mb-4 neon-text flex items-center gap-2" style={{ color: 'var(--neon-purple)' }}>
            <ShieldCheck size={18} weight="duotone" />
            Host Security
          </h2>
          {loadingSecurity || !security ? (
            <div className="text-sm flex items-center gap-2" style={{ color: 'var(--text-secondary)' }}>
              <SpinnerGap size={16} className="animate-spin" /> Loading security status...
            </div>
          ) : (
            <div className="space-y-4">
              <div>
                <div className="text-sm font-bold" style={{ color: 'var(--neon-purple)' }}>SSH</div>
                <div className="flex items-center gap-2 mt-1">
                  {statusBadge(security.ssh.status)}
                  <span className="text-xs" style={{ color: 'var(--text-secondary)' }}>{security.ssh.unit || 'unknown'}</span>
                </div>
                <div className="text-xs mt-2" style={{ color: 'var(--text-secondary)' }}>
                  Ports: {security.ssh.ports.join(', ')}
                </div>
              </div>
              <div>
                <div className="text-sm font-bold" style={{ color: 'var(--neon-purple)' }}>SUDO USERS</div>
                <div className="text-xs mt-1" style={{ color: 'var(--text-secondary)' }}>
                  {security.sudoUsers.length > 0 ? security.sudoUsers.join(', ') : 'No sudo users detected'}
                </div>
              </div>
              <div>
                <div className="text-sm font-bold" style={{ color: 'var(--neon-purple)' }}>FAILED LOGINS</div>
                <div className="text-xs mt-1" style={{ color: 'var(--text-secondary)' }}>
                  {security.failedLogins.count} recent failures ({security.failedLogins.source})
                </div>
                {security.failedLogins.latest && (
                  <div className="text-xs mt-1" style={{ color: 'var(--text-secondary)' }}>
                    Latest: {security.failedLogins.latest}
                  </div>
                )}
              </div>
            </div>
          )}
        </div>
      </div>

          <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 mt-8">
        <div className="card-vapor p-6 rounded-xl">
          <h2 className="text-xl font-bold mb-4 neon-text flex items-center gap-2" style={{ color: 'var(--neon-cyan)' }}>
            <HardDrives size={18} weight="duotone" />
            Storage
          </h2>
          {loadingStorage || !storage ? (
            <div className="text-sm flex items-center gap-2" style={{ color: 'var(--text-secondary)' }}>
              <SpinnerGap size={16} className="animate-spin" /> Loading storage...
            </div>
          ) : (
            <div className="space-y-3">
              <div className="text-xs font-bold" style={{ color: 'var(--neon-purple)' }}>MOUNTS</div>
              <div>
                <table className="w-full text-left text-xs">
                  <thead style={{ color: 'var(--text-secondary)' }}>
                    <tr>
                      <th className="py-2">Mount</th>
                      <th className="py-2">Used</th>
                      <th className="py-2">Available</th>
                      <th className="py-2">Use%</th>
                    </tr>
                  </thead>
                  <tbody>
                    {storage.mounts.map((mount) => (
                      <tr key={`${mount.mountpoint}-${mount.filesystem}`} className="border-t border-white/5">
                        <td className="py-2 text-xs font-mono" style={{ color: 'var(--text-primary)' }}>{mount.mountpoint}</td>
                        <td className="py-2 text-xs" style={{ color: 'var(--text-secondary)' }}>{formatBytes(mount.used)}</td>
                        <td className="py-2 text-xs" style={{ color: 'var(--text-secondary)' }}>{formatBytes(mount.available)}</td>
                        <td className="py-2 text-xs" style={{ color: 'var(--text-secondary)' }}>{mount.usePercent}%</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          )}
        </div>

        <div className="card-vapor p-6 rounded-xl">
          <h2 className="text-xl font-bold mb-4 neon-text flex items-center gap-2" style={{ color: 'var(--neon-pink)' }}>
            <Package size={18} weight="duotone" />
            Docker Storage
          </h2>
          {loadingStorage || !storage ? (
            <div className="text-sm flex items-center gap-2" style={{ color: 'var(--text-secondary)' }}>
              <SpinnerGap size={16} className="animate-spin" /> Loading Docker usage...
            </div>
          ) : (
            <div className="space-y-4">
              <div className="grid grid-cols-2 gap-3">
                <div>
                  <div className="text-xs font-bold" style={{ color: 'var(--neon-purple)' }}>IMAGES</div>
                  <div className="text-sm" style={{ color: 'var(--text-primary)' }}>{storage.docker.imageCount} ({formatBytes(storage.docker.imageSize)})</div>
                </div>
                <div>
                  <div className="text-xs font-bold" style={{ color: 'var(--neon-purple)' }}>CONTAINERS</div>
                  <div className="text-sm" style={{ color: 'var(--text-primary)' }}>{storage.docker.containerCount} ({formatBytes(storage.docker.containerSize)})</div>
                </div>
                <div>
                  <div className="text-xs font-bold" style={{ color: 'var(--neon-purple)' }}>VOLUMES</div>
                  <div className="text-sm" style={{ color: 'var(--text-primary)' }}>{storage.docker.volumeCount} ({formatBytes(storage.docker.volumeSize)})</div>
                </div>
                <div>
                  <div className="text-xs font-bold" style={{ color: 'var(--neon-purple)' }}>BUILD CACHE</div>
                  <div className="text-sm" style={{ color: 'var(--text-primary)' }}>{storage.docker.buildCacheCount} ({formatBytes(storage.docker.buildCacheSize)})</div>
                </div>
              </div>

              <div className="flex flex-wrap gap-2">
                <button className="btn-neon px-3 py-1 text-xs font-bold" onClick={() => handlePrune('images')}>
                  Prune Images
                </button>
                <button className="btn-neon px-3 py-1 text-xs font-bold" onClick={() => handlePrune('build-cache')}>
                  Prune Build Cache
                </button>
              </div>

              <div>
                <div className="text-xs font-bold mb-2" style={{ color: 'var(--neon-purple)' }}>VOLUMES (READ-ONLY)</div>
                <div className="text-xs" style={{ color: 'var(--text-secondary)' }}>
                  {storage.volumes.length === 0 ? (
                    <div>No volumes detected.</div>
                  ) : (
                    <ul className="space-y-2">
                      {storage.volumes.map((volume) => (
                        <li key={volume.name} className="border-b border-white/5 pb-2">
                          <div className="font-mono" style={{ color: 'var(--text-primary)' }}>{volume.name}</div>
                          <div>{formatBytes(volume.size)} • {volume.driver} • refs {volume.refCount}</div>
                        </li>
                      ))}
                    </ul>
                  )}
                </div>
              </div>
            </div>
          )}
        </div>
      </div>

          <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 mt-8">
        <div className="card-vapor p-6 rounded-xl">
          <h2 className="text-xl font-bold mb-4 neon-text flex items-center gap-2" style={{ color: 'var(--neon-cyan)' }}>
            <TerminalWindow size={18} weight="duotone" />
            Logs
          </h2>
          <div className="space-y-4">
            <div>
              <div className="flex items-center justify-between">
                <div className="text-xs font-bold" style={{ color: 'var(--neon-purple)' }}>SYSTEM LOGS</div>
                <button className="btn-neon px-3 py-1 text-xs font-bold" onClick={handleSystemLogs}>
                  {loadingSystemLogs ? 'Loading...' : 'Refresh'}
                </button>
              </div>
              <pre
                className="text-xs p-3 rounded-lg overflow-auto max-h-48 mt-2"
                style={{ background: 'var(--surface-muted)', color: 'var(--text-primary)' }}
              >
                {systemLogs || 'No system logs loaded.'}
              </pre>
            </div>
            <div>
              <div className="flex items-center justify-between">
                <div className="text-xs font-bold" style={{ color: 'var(--neon-purple)' }}>DOCKLITE LOGS</div>
                <button className="btn-neon px-3 py-1 text-xs font-bold" onClick={handleDockliteLogs}>
                  {loadingDockliteLogs ? 'Loading...' : 'Refresh'}
                </button>
              </div>
              <pre
                className="text-xs p-3 rounded-lg overflow-auto max-h-48 mt-2"
                style={{ background: 'var(--surface-muted)', color: 'var(--text-primary)' }}
              >
                {dockliteLogs || 'No DockLite logs loaded.'}
              </pre>
            </div>
          </div>
        </div>

        <div className="card-vapor p-6 rounded-xl">
          <h2 className="text-xl font-bold mb-4 neon-text flex items-center gap-2" style={{ color: 'var(--neon-green)' }}>
            <DownloadSimple size={18} weight="duotone" />
            Diagnostics
          </h2>
          <div className="space-y-4">
            <div className="text-sm" style={{ color: 'var(--text-secondary)' }}>
              Generate a diagnostics bundle with system + DockLite logs, Docker info, and host metadata.
            </div>
            <div className="text-xs" style={{ color: 'var(--text-secondary)' }}>
              Stored at <span className="font-mono">/var/backups/docklite/diagnostics</span> (keeps last 5).
            </div>
            <button className="btn-neon px-4 py-2 text-sm font-bold inline-flex items-center gap-2" onClick={downloadDiagnostics}>
              <ArrowClockwise size={16} weight="duotone" />
              Download diagnostics bundle
            </button>
          </div>
        </div>
          </div>
        </>
      )}
    </div>
  );
}
