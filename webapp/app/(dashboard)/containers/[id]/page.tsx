'use client';

import { useEffect, useState, useCallback } from 'react';
import { useParams, useRouter } from 'next/navigation';
import Link from 'next/link';
import {
  Lightning,
  WarningCircle,
  ArrowLeft,
  Package,
  ChartBar,
  NotePencil,
  Wrench,
  Globe,
  Info,
  Database,
  LockOpen,
  Lock,
  ArrowClockwise,
  Plug,
} from '@phosphor-icons/react';

interface ContainerStats {
  cpu: number;
  memory: {
    used: number;
    total: number;
    percentage: number;
  };
}

interface ContainerDetails {
  id: string;
  name: string;
  image: string;
  created: string;
  state: {
    Status: string;
    Running: boolean;
    StartedAt: string;
    FinishedAt: string;
  };
  env: string[];
  labels: Record<string, string>;
  mounts: any[];
  networkSettings: {
    networks: Record<string, any>;
    ports: Record<string, any>;
    ipAddress: string;
    gateway: string;
  };
  restartPolicy: {
    Name: string;
    MaximumRetryCount: number;
  };
  resources: {
    memory: number;
    memorySwap: number;
    cpuShares: number;
    cpuQuota: number;
  };
}

export default function ContainerDetailPage() {
  const params = useParams();
  const router = useRouter();
  const containerId = params.id as string;

  const [details, setDetails] = useState<ContainerDetails | null>(null);
  const [stats, setStats] = useState<ContainerStats | null>(null);
  const [logs, setLogs] = useState<string>('');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [activeTab, setActiveTab] = useState<'overview' | 'logs' | 'env' | 'network'>('overview');
  const [autoRefreshLogs, setAutoRefreshLogs] = useState(true);
  const [logLines, setLogLines] = useState(100);

  // Fetch container details
  const fetchDetails = useCallback(async () => {
    try {
      const res = await fetch(`/api/containers/${containerId}/inspect`);
      if (!res.ok) throw new Error('Failed to fetch container details');
      const data = await res.json();
      setDetails(data.container);
    } catch (err: any) {
      setError(err.message);
    }
  }, [containerId]);

  // Fetch container stats
  const fetchStats = useCallback(async () => {
    try {
      const res = await fetch(`/api/containers/${containerId}/stats`);
      if (res.ok) {
        const data = await res.json();
        setStats(data.stats);
      }
    } catch (err) {
      // Stats might not be available if container is stopped
      setStats(null);
    }
  }, [containerId]);

  // Fetch container logs
  const fetchLogs = useCallback(async () => {
    try {
      const res = await fetch(`/api/containers/${containerId}/logs?tail=${logLines}`);
      if (!res.ok) throw new Error('Failed to fetch logs');
      const data = await res.json();
      setLogs(data.logs);
    } catch (err: any) {
      console.error('Error fetching logs:', err);
    }
  }, [containerId, logLines]);

  // Initial load
  useEffect(() => {
    const loadData = async () => {
      await fetchDetails();
      await fetchStats();
      await fetchLogs();
      setLoading(false);
    };
    loadData();
  }, [fetchDetails, fetchStats, fetchLogs]);

  // Auto-refresh stats (every 2 seconds)
  useEffect(() => {
    const interval = setInterval(fetchStats, 2000);
    return () => clearInterval(interval);
  }, [fetchStats]);

  // Auto-refresh logs (every 5 seconds if enabled)
  useEffect(() => {
    if (!autoRefreshLogs) return;
    const interval = setInterval(fetchLogs, 5000);
    return () => clearInterval(interval);
  }, [autoRefreshLogs, fetchLogs]);

  const formatBytes = (bytes: number) => {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return `${(bytes / Math.pow(k, i)).toFixed(2)} ${sizes[i]}`;
  };

  const formatDate = (dateStr: string) => {
    return new Date(dateStr).toLocaleString();
  };

  if (loading) {
    return (
      <div className="text-center py-16">
        <div className="flex justify-center mb-4 animate-float">
          <Lightning size={48} weight="duotone" color="var(--neon-cyan)" />
        </div>
        <div className="text-2xl font-bold neon-text animate-pulse" style={{ color: 'var(--text-cyan)' }}>
          Loading container details...
        </div>
      </div>
    );
  }

  if (error || !details) {
    return (
      <div className="text-center py-16">
        <div className="flex justify-center mb-4">
          <WarningCircle size={48} weight="duotone" color="var(--status-error)" />
        </div>
        <div className="text-xl font-bold mb-4" style={{ color: 'var(--status-error)' }}>
          {error || 'Container not found'}
        </div>
        <Link href="/" className="btn-neon px-6 py-3 inline-flex items-center gap-2">
          <ArrowLeft size={16} weight="bold" />
          Back to Dashboard
        </Link>
      </div>
    );
  }

  const isRunning = details.state.Running;

  return (
    <div>
      {/* Header */}
      <div className="flex items-center justify-between mb-8">
        <div className="flex items-center gap-4">
          <Link href="/" className="text-2xl hover:scale-110 transition-transform inline-flex items-center">
            <ArrowLeft size={20} weight="bold" />
          </Link>
          <div>
            <h1 className="text-4xl font-bold neon-text flex items-center gap-2" style={{ color: 'var(--text-cyan)' }}>
              <Package size={24} weight="duotone" />
              {details.name}
            </h1>
            <p className="text-sm font-mono mt-1" style={{ color: 'var(--text-secondary)' }}>
              {details.image}
            </p>
          </div>
        </div>
        <span
          className="px-4 py-2 rounded-full text-sm font-bold"
          style={{
            background: isRunning ? 'rgba(var(--status-success-rgb), 0.2)' : 'rgba(var(--status-error-rgb), 0.2)',
            color: isRunning ? 'var(--text-green)' : 'var(--status-error)',
            border: `2px solid ${isRunning ? 'var(--neon-green)' : 'var(--status-error)'}`,
          }}
        >
          {isRunning ? '● RUNNING' : '○ STOPPED'}
        </span>
      </div>

      {/* Tabs */}
      <div className="flex gap-2 mb-6 overflow-x-auto">
        {[
          { id: 'overview', label: 'Overview', icon: <ChartBar size={16} weight="duotone" /> },
          { id: 'logs', label: 'Logs', icon: <NotePencil size={16} weight="duotone" /> },
          { id: 'env', label: 'Environment', icon: <Wrench size={16} weight="duotone" /> },
          { id: 'network', label: 'Network', icon: <Globe size={16} weight="duotone" /> },
        ].map((tab) => (
          <button
            key={tab.id}
            onClick={() => setActiveTab(tab.id as any)}
            className={`px-6 py-3 rounded-xl font-bold transition-all ${
              activeTab === tab.id
                ? 'bg-gradient-to-r from-cyan-500 to-purple-500 text-gray-900 neon-glow'
                : 'card-vapor text-accent-cyan hover:text-accent-cyan/80'
            }`}
          >
            <span className="mr-2 inline-flex">{tab.icon}</span>
            {tab.label}
          </button>
        ))}
      </div>

      {/* Overview Tab */}
      {activeTab === 'overview' && (
        <div className="space-y-6">
          {/* Real-time Stats */}
          {isRunning && stats && (
            <div className="card-vapor p-6 rounded-xl">
              <h2 className="text-2xl font-bold neon-text mb-6 flex items-center gap-2" style={{ color: 'var(--text-pink)' }}>
                <ChartBar size={20} weight="duotone" />
                Real-time Resources
              </h2>
              <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                {/* CPU */}
                <div>
                  <div className="flex items-center justify-between mb-2">
                    <span className="font-bold" style={{ color: 'var(--text-cyan)' }}>
                      CPU Usage
                    </span>
                    <span className="text-2xl font-bold" style={{ color: 'var(--text-yellow)' }}>
                      {(stats.cpu ?? 0).toFixed(2)}%
                    </span>
                  </div>
                  <div className="w-full h-4 bg-gray-800 rounded-full overflow-hidden">
                    <div
                      className="h-full bg-gradient-to-r from-cyan-500 to-purple-500 transition-all duration-300"
                      style={{ width: `${Math.min(stats.cpu ?? 0, 100)}%` }}
                    ></div>
                  </div>
                </div>

                {/* Memory */}
                <div>
                  <div className="flex items-center justify-between mb-2">
                    <span className="font-bold" style={{ color: 'var(--text-cyan)' }}>
                      Memory Usage
                    </span>
                    <span className="text-2xl font-bold" style={{ color: 'var(--text-pink)' }}>
                      {(stats.memory?.percentage ?? 0).toFixed(2)}%
                    </span>
                  </div>
                  <div className="w-full h-4 bg-gray-800 rounded-full overflow-hidden mb-2">
                    <div
                      className="h-full bg-gradient-to-r from-pink-500 to-purple-500 transition-all duration-300"
                      style={{ width: `${Math.min(stats.memory?.percentage ?? 0, 100)}%` }}
                    ></div>
                  </div>
                  <div className="text-sm font-mono" style={{ color: 'var(--text-secondary)' }}>
                    {formatBytes(stats.memory?.used ?? 0)} / {formatBytes(stats.memory?.total ?? 0)}
                  </div>
                </div>
              </div>
            </div>
          )}

          {/* Container Info */}
          <div className="card-vapor p-6 rounded-xl">
            <h2 className="text-2xl font-bold neon-text mb-6 flex items-center gap-2" style={{ color: 'var(--text-pink)' }}>
              <Info size={20} weight="duotone" />
              Container Information
            </h2>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4 font-mono text-sm">
              <div>
                <span className="opacity-70">ID:</span>
                <div className="font-bold" style={{ color: 'var(--text-cyan)' }}>
                  {details.id.substring(0, 12)}
                </div>
              </div>
              <div>
                <span className="opacity-70">Created:</span>
                <div className="font-bold">{formatDate(details.created)}</div>
              </div>
              <div>
                <span className="opacity-70">Started:</span>
                <div className="font-bold">
                  {details.state.StartedAt ? formatDate(details.state.StartedAt) : 'N/A'}
                </div>
              </div>
              <div>
                <span className="opacity-70">Restart Policy:</span>
                <div className="font-bold" style={{ color: 'var(--text-green)' }}>
                  {details.restartPolicy.Name || 'none'}
                </div>
              </div>
            </div>
          </div>

          {/* Mounts */}
          {details.mounts.length > 0 && (
            <div className="card-vapor p-6 rounded-xl">
            <h2 className="text-2xl font-bold neon-text mb-6 flex items-center gap-2" style={{ color: 'var(--text-pink)' }}>
              <Database size={20} weight="duotone" />
              Volumes & Mounts
            </h2>
              <div className="space-y-3">
                {details.mounts.map((mount: any, idx: number) => (
                  <div key={idx} className="p-4 bg-purple-900/20 rounded-lg">
                    <div className="font-mono text-sm">
                      <div className="mb-2">
                        <span className="opacity-70">Source:</span>
                        <div className="font-bold text-accent-cyan">{mount.Source}</div>
                      </div>
                      <div className="mb-2">
                        <span className="opacity-70">Destination:</span>
                        <div className="font-bold text-accent-pink">{mount.Destination}</div>
                      </div>
                      <div className="flex gap-4 text-xs">
                        <span className={mount.RW ? 'text-status-success' : 'text-status-error'}>
                          {mount.RW ? (
                            <span className="inline-flex items-center gap-2">
                              <LockOpen size={12} weight="duotone" />
                              Read/Write
                            </span>
                          ) : (
                            <span className="inline-flex items-center gap-2">
                              <Lock size={12} weight="duotone" />
                              Read-Only
                            </span>
                          )}
                        </span>
                        <span className="opacity-70">Type: {mount.Type}</span>
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>
      )}

      {/* Logs Tab */}
      {activeTab === 'logs' && (
        <div className="card-vapor p-6 rounded-xl">
          <div className="flex items-center justify-between mb-6">
            <h2 className="text-2xl font-bold neon-text flex items-center gap-2" style={{ color: 'var(--text-pink)' }}>
              <NotePencil size={20} weight="duotone" />
              Container Logs
            </h2>
            <div className="flex items-center gap-4">
              <label className="flex items-center gap-2 text-sm">
                <input
                  type="checkbox"
                  checked={autoRefreshLogs}
                  onChange={(e) => setAutoRefreshLogs(e.target.checked)}
                  className="w-4 h-4"
                />
                <span>Auto-refresh (5s)</span>
              </label>
              <select
                value={logLines}
                onChange={(e) => {
                  setLogLines(parseInt(e.target.value));
                  fetchLogs();
                }}
                className="input-vapor px-3 py-2 text-sm"
              >
                <option value="50">50 lines</option>
                <option value="100">100 lines</option>
                <option value="200">200 lines</option>
                <option value="500">500 lines</option>
                <option value="1000">1000 lines</option>
              </select>
              <button onClick={fetchLogs} className="btn-neon px-4 py-2 text-sm inline-flex items-center gap-2">
                <ArrowClockwise size={14} weight="duotone" />
                Refresh
              </button>
            </div>
          </div>
          <div
            className="bg-black/50 p-4 rounded-lg font-mono text-xs overflow-x-auto"
            style={{ maxHeight: '600px', overflowY: 'auto' }}
          >
            <pre className="whitespace-pre-wrap" style={{ color: 'var(--text-green)' }}>
              {logs || 'No logs available'}
            </pre>
          </div>
        </div>
      )}

      {/* Environment Tab */}
      {activeTab === 'env' && (
        <div className="card-vapor p-6 rounded-xl">
          <h2 className="text-2xl font-bold neon-text mb-6 flex items-center gap-2" style={{ color: 'var(--text-pink)' }}>
            <Wrench size={20} weight="duotone" />
            Environment Variables
          </h2>
          {details.env.length === 0 ? (
            <div className="text-center py-8 opacity-60">No environment variables set</div>
          ) : (
            <div className="space-y-2">
              {details.env.map((envVar: string, idx: number) => {
                const [key, ...valueParts] = envVar.split('=');
                const value = valueParts.join('=');
                return (
                  <div key={idx} className="p-4 bg-purple-900/20 rounded-lg font-mono text-sm">
                    <div className="flex items-start gap-4">
                      <span className="font-bold min-w-[200px]" style={{ color: 'var(--text-cyan)' }}>
                        {key}
                      </span>
                      <span className="flex-1 break-all" style={{ color: 'var(--text-pink)' }}>
                        {value || '(empty)'}
                      </span>
                    </div>
                  </div>
                );
              })}
            </div>
          )}
        </div>
      )}

      {/* Network Tab */}
      {activeTab === 'network' && (
        <div className="space-y-6">
          {/* Network Info */}
          <div className="card-vapor p-6 rounded-xl">
            <h2 className="text-2xl font-bold neon-text mb-6 flex items-center gap-2" style={{ color: 'var(--text-pink)' }}>
              <Globe size={20} weight="duotone" />
              Network Configuration
            </h2>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4 font-mono text-sm mb-6">
              <div>
                <span className="opacity-70">IP Address:</span>
                <div className="font-bold" style={{ color: 'var(--text-cyan)' }}>
                  {details.networkSettings.ipAddress || 'N/A'}
                </div>
              </div>
              <div>
                <span className="opacity-70">Gateway:</span>
                <div className="font-bold" style={{ color: 'var(--text-cyan)' }}>
                  {details.networkSettings.gateway || 'N/A'}
                </div>
              </div>
            </div>

            {/* Networks */}
            <h3 className="text-xl font-bold neon-text mb-4" style={{ color: 'var(--text-cyan)' }}>
              Networks
            </h3>
            <div className="space-y-3">
              {Object.entries(details.networkSettings.networks).map(([name, network]: [string, any]) => (
                <div key={name} className="p-4 bg-purple-900/20 rounded-lg">
                  <div className="font-bold mb-2" style={{ color: 'var(--text-pink)' }}>
                    {name}
                  </div>
                  <div className="font-mono text-xs space-y-1">
                    <div>
                      <span className="opacity-70">IP: </span>
                      <span className="text-accent-cyan">{network.IPAddress || 'N/A'}</span>
                    </div>
                    <div>
                      <span className="opacity-70">Gateway: </span>
                      <span className="text-accent-cyan">{network.Gateway || 'N/A'}</span>
                    </div>
                    <div>
                      <span className="opacity-70">MAC: </span>
                      <span className="text-accent-cyan">{network.MacAddress || 'N/A'}</span>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          </div>

          {/* Port Mappings */}
          <div className="card-vapor p-6 rounded-xl">
            <h2 className="text-2xl font-bold neon-text mb-6 flex items-center gap-2" style={{ color: 'var(--text-pink)' }}>
              <Plug size={20} weight="duotone" />
              Port Mappings
            </h2>
            {Object.keys(details.networkSettings.ports).length === 0 ? (
              <div className="text-center py-8 opacity-60">No port mappings</div>
            ) : (
              <div className="space-y-2">
                {Object.entries(details.networkSettings.ports).map(([containerPort, hostBindings]: [string, any]) => (
                  <div key={containerPort} className="p-4 bg-purple-900/20 rounded-lg font-mono text-sm">
                    <div className="flex items-center gap-4">
                      <span className="font-bold" style={{ color: 'var(--text-cyan)' }}>
                        Container: {containerPort}
                      </span>
                      <span style={{ color: 'var(--text-purple)' }}>→</span>
                      <span className="font-bold" style={{ color: 'var(--text-pink)' }}>
                        Host: {hostBindings && hostBindings[0]?.HostPort ? hostBindings[0].HostPort : 'Not mapped'}
                      </span>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  );
}
