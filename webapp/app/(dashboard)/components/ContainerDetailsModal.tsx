'use client';

import { useEffect, useState } from 'react';
import {
  Lightning,
  WarningCircle,
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
import { useBodyScrollLock } from '@/lib/hooks/useBodyScrollLock';

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

interface Props {
  containerId: string;
  containerName: string;
  onClose: () => void;
}

export default function ContainerDetailsModal({ containerId, containerName, onClose }: Props) {
  useBodyScrollLock();
  const [details, setDetails] = useState<ContainerDetails | null>(null);
  const [stats, setStats] = useState<ContainerStats | null>(null);
  const [logs, setLogs] = useState<string>('');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [activeTab, setActiveTab] = useState<'overview' | 'logs' | 'env' | 'network'>('overview');
  const [autoRefreshLogs, setAutoRefreshLogs] = useState(true);
  const [logLines, setLogLines] = useState(100);

  // Fetch container details
  const fetchDetails = async () => {
    try {
      const res = await fetch(`/api/containers/${containerId}/inspect`);
      if (!res.ok) {
        const errorData = await res.json().catch(() => ({ error: 'Unknown error' }));
        throw new Error(errorData.error || 'Failed to fetch container details');
      }
      const data = await res.json();
      if (!data.container) {
        throw new Error('Invalid response from server');
      }
      setDetails(data.container);
    } catch (err: any) {
      console.error('Error fetching container details:', err);
      setError(err.message || 'Failed to load container details');
    }
  };

  // Fetch container stats
  const fetchStats = async () => {
    try {
      const res = await fetch(`/api/containers/${containerId}/stats`);
      if (res.ok) {
        const data = await res.json();
        // Handle both formats: agent-client transformed and raw agent response
        const rawStats = data.stats;
        if (rawStats && typeof rawStats === 'object') {
          // Transform to consistent format
          const cpu = Number(rawStats.cpu ?? rawStats.cpuUsage ?? rawStats.CPUUsage ?? 0);
          const memUsed = Number(rawStats.memory?.used ?? rawStats.memoryUsage ?? rawStats.MemoryUsage ?? 0);
          const memTotal = Number(rawStats.memory?.total ?? rawStats.memoryLimit ?? rawStats.MemoryLimit ?? 0);
          const memPct = Number(rawStats.memory?.percentage ?? rawStats.memoryPct ?? rawStats.MemoryPct ?? 0);
          
          setStats({
            cpu: cpu,
            memory: {
              used: memUsed,
              total: memTotal,
              percentage: memPct,
            },
          });
        } else {
          setStats(null);
        }
      } else {
        setStats(null);
      }
    } catch (err) {
      console.error('Stats fetch error:', err);
      setStats(null);
    }
  };

  // Fetch container logs
  const fetchLogs = async () => {
    try {
      const res = await fetch(`/api/containers/${containerId}/logs?tail=${logLines}`);
      if (!res.ok) throw new Error('Failed to fetch logs');
      const data = await res.json();
      setLogs(data.logs);
    } catch (err: any) {
      console.error('Error fetching logs:', err);
    }
  };

  // Initial load
  useEffect(() => {
    const loadData = async () => {
      await fetchDetails();
      await fetchStats();
      await fetchLogs();
      setLoading(false);
    };
    loadData();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [containerId]);

  // Auto-refresh stats (every 2 seconds)
  useEffect(() => {
    const interval = setInterval(fetchStats, 2000);
    return () => clearInterval(interval);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [containerId]);

  // Auto-refresh logs (every 5 seconds if enabled)
  useEffect(() => {
    if (!autoRefreshLogs) return;
    const interval = setInterval(fetchLogs, 5000);
    return () => clearInterval(interval);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [containerId, autoRefreshLogs, logLines]);

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

  if (loading || !details) {
    return (
      <div className="fixed inset-0 bg-black/80 backdrop-blur-lg z-[9999] flex items-center justify-center p-4" onClick={onClose}>
        <div className="text-center" onClick={(e) => e.stopPropagation()}>
          <div className="flex justify-center mb-4 animate-float">
            <Lightning size={48} weight="duotone" color="var(--neon-cyan)" />
          </div>
          <div className="text-2xl font-bold neon-text animate-pulse" style={{ color: 'var(--neon-cyan)' }}>
            Loading...
          </div>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="fixed inset-0 bg-black/80 backdrop-blur-lg z-[9999] flex items-center justify-center p-4" onClick={onClose}>
        <div className="card-vapor p-8 max-w-md" onClick={(e) => e.stopPropagation()}>
          <div className="flex justify-center mb-4 text-center">
            <WarningCircle size={48} weight="duotone" color="var(--status-error)" />
          </div>
          <div className="text-xl font-bold mb-4 text-center" style={{ color: 'var(--status-error)' }}>
            {error}
          </div>
          <button onClick={onClose} className="btn-neon w-full py-3">
            Close
          </button>
        </div>
      </div>
    );
  }

  const isRunning = details.state.Running;

  return (
    <div className="fixed inset-0 bg-black/80 backdrop-blur-lg z-[9999] flex items-center justify-center p-4 overflow-y-auto" onClick={onClose}>
      <div
        className="card-vapor neon-border max-w-7xl w-full max-h-[90vh] overflow-y-auto p-6 rounded-2xl my-8"
        onClick={(e) => e.stopPropagation()}
        style={{
          background: 'linear-gradient(135deg, var(--modal-bg-1) 0%, var(--modal-bg-2) 100%)',
          border: '2px solid rgba(var(--neon-cyan-rgb), 0.5)',
        }}
      >
        {/* Header */}
        <div className="flex items-center justify-between mb-6 pb-4 border-b" style={{ borderColor: 'rgba(var(--neon-cyan-rgb), 0.2)' }}>
          <div className="flex items-center gap-4">
            <div>
              <h2 className="text-3xl font-bold neon-text flex items-center gap-2" style={{ color: 'var(--neon-cyan)' }}>
                <Package size={22} weight="duotone" />
                {details.name}
              </h2>
              <p className="text-sm font-mono mt-1" style={{ color: 'var(--text-secondary)' }}>
                {details.image}
              </p>
            </div>
          </div>
          <div className="flex items-center gap-4">
            <span
              className="px-4 py-2 rounded-full text-sm font-bold"
              style={{
                background: isRunning ? 'rgba(var(--status-success-rgb), 0.2)' : 'rgba(var(--status-error-rgb), 0.2)',
                color: isRunning ? 'var(--neon-green)' : 'var(--status-error)',
                border: `2px solid ${isRunning ? 'var(--neon-green)' : 'var(--status-error)'}`,
              }}
            >
              {isRunning ? '● RUNNING' : '○ STOPPED'}
            </span>
            <button
              onClick={onClose}
              className="text-3xl hover:scale-110 transition-transform"
              style={{ color: 'var(--neon-pink)' }}
            >
              ×
            </button>
          </div>
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
              className={`px-6 py-3 rounded-xl font-bold transition-all whitespace-nowrap ${
                activeTab === tab.id
                  ? 'neon-glow'
                  : 'card-vapor'
              }`}
              style={activeTab === tab.id ? {
                background: 'linear-gradient(135deg, var(--neon-cyan) 0%, var(--neon-purple) 100%)',
                color: 'var(--button-text)'
              } : {
                color: 'var(--neon-cyan)'
              }}
            >
              <span className="mr-2 inline-flex">{tab.icon}</span>
              {tab.label}
            </button>
          ))}
        </div>

        {/* Tab Content */}
        <div className="space-y-6">
          {/* Overview Tab */}
          {activeTab === 'overview' && (
            <>
              {/* Real-time Stats */}
              {isRunning && stats && stats.cpu !== undefined && (
                <div className="card-vapor p-6 rounded-xl">
                  <h3 className="text-xl font-bold neon-text mb-4 flex items-center gap-2" style={{ color: 'var(--neon-pink)' }}>
                    <ChartBar size={18} weight="duotone" />
                    Real-time Resources
                  </h3>
                  <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                    {/* CPU */}
                    <div>
                      <div className="flex items-center justify-between mb-2">
                        <span className="font-bold" style={{ color: 'var(--neon-cyan)' }}>
                          CPU Usage
                        </span>
                        <span className="text-2xl font-bold" style={{ color: 'var(--neon-yellow)' }}>
                          {(stats.cpu).toFixed(2)}%
                        </span>
                      </div>
                      <div className="w-full h-4 rounded-full overflow-hidden" style={{ background: 'var(--surface-dim)' }}>
                        <div
                          className="h-full transition-all duration-300"
                          style={{
                            width: `${Math.min(stats.cpu, 100)}%`,
                            background: 'linear-gradient(135deg, var(--neon-cyan) 0%, var(--neon-purple) 100%)'
                          }}
                        ></div>
                      </div>
                    </div>

                    {/* Memory */}
                    <div>
                      <div className="flex items-center justify-between mb-2">
                        <span className="font-bold" style={{ color: 'var(--neon-cyan)' }}>
                          Memory Usage
                        </span>
                        <span className="text-2xl font-bold" style={{ color: 'var(--neon-pink)' }}>
                          {(stats.memory?.percentage ?? 0).toFixed(2)}%
                        </span>
                      </div>
                      <div className="w-full h-4 rounded-full overflow-hidden mb-2" style={{ background: 'var(--surface-dim)' }}>
                        <div
                          className="h-full transition-all duration-300"
                          style={{
                            width: `${Math.min(stats.memory?.percentage ?? 0, 100)}%`,
                            background: 'linear-gradient(135deg, var(--neon-pink) 0%, var(--neon-purple) 100%)'
                          }}
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
                <h3 className="text-xl font-bold neon-text mb-4 flex items-center gap-2" style={{ color: 'var(--neon-pink)' }}>
                  <Info size={18} weight="duotone" />
                  Container Information
                </h3>
                <div className="grid grid-cols-1 md:grid-cols-2 gap-4 font-mono text-sm">
                  <div>
                    <span className="opacity-70">ID:</span>
                    <div className="font-bold" style={{ color: 'var(--neon-cyan)' }}>
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
                    <div className="font-bold" style={{ color: 'var(--neon-green)' }}>
                      {details.restartPolicy.Name || 'none'}
                    </div>
                  </div>
                </div>
              </div>

              {/* Mounts */}
              {details.mounts.length > 0 && (
                <div className="card-vapor p-6 rounded-xl">
                  <h3 className="text-xl font-bold neon-text mb-4 flex items-center gap-2" style={{ color: 'var(--neon-pink)' }}>
                    <Database size={18} weight="duotone" />
                    Volumes & Mounts
                  </h3>
                  <div className="space-y-3">
                    {details.mounts.map((mount: any, idx: number) => (
                      <div key={idx} className="p-4 rounded-lg" style={{ background: 'rgba(var(--neon-purple-rgb), 0.1)' }}>
                        <div className="font-mono text-sm">
                          <div className="mb-2">
                            <span className="opacity-70">Source:</span>
                            <div className="font-bold break-all" style={{ color: 'var(--neon-cyan)' }}>{mount.Source}</div>
                          </div>
                          <div className="mb-2">
                            <span className="opacity-70">Destination:</span>
                            <div className="font-bold" style={{ color: 'var(--neon-pink)' }}>{mount.Destination}</div>
                          </div>
                          <div className="flex gap-4 text-xs">
                            <span style={{ color: mount.RW ? 'var(--status-success)' : 'var(--status-error)' }}>
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
            </>
          )}

          {/* Logs Tab */}
          {activeTab === 'logs' && (
            <div className="card-vapor p-6 rounded-xl">
              <div className="flex flex-wrap items-center justify-between mb-6 gap-4">
                <h3 className="text-xl font-bold neon-text flex items-center gap-2" style={{ color: 'var(--neon-pink)' }}>
                  <NotePencil size={18} weight="duotone" />
                  Container Logs
                </h3>
                <div className="flex items-center gap-4 flex-wrap">
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
                  </select>
                  <button onClick={fetchLogs} className="btn-neon px-4 py-2 text-sm inline-flex items-center gap-2">
                    <ArrowClockwise size={14} weight="duotone" />
                    Refresh
                  </button>
                </div>
              </div>
              <div
                className="bg-black/50 p-4 rounded-lg font-mono text-xs overflow-x-auto"
                style={{ maxHeight: '500px', overflowY: 'auto' }}
              >
                <pre className="whitespace-pre-wrap" style={{ color: 'var(--neon-green)' }}>
                  {logs || 'No logs available'}
                </pre>
              </div>
            </div>
          )}

          {/* Environment Tab */}
          {activeTab === 'env' && (
            <div className="card-vapor p-6 rounded-xl">
              <h3 className="text-xl font-bold neon-text mb-4 flex items-center gap-2" style={{ color: 'var(--neon-pink)' }}>
                <Wrench size={18} weight="duotone" />
                Environment Variables
              </h3>
              {details.env.length === 0 ? (
                <div className="text-center py-8 opacity-60">No environment variables set</div>
              ) : (
                <div className="space-y-2 max-h-[500px] overflow-y-auto">
                  {details.env.map((envVar: string, idx: number) => {
                    const [key, ...valueParts] = envVar.split('=');
                    const value = valueParts.join('=');
                    return (
                      <div key={idx} className="p-4 rounded-lg font-mono text-sm" style={{ background: 'rgba(var(--neon-purple-rgb), 0.1)' }}>
                        <div className="flex flex-col sm:flex-row sm:items-start gap-2 sm:gap-4">
                          <span className="font-bold min-w-[200px]" style={{ color: 'var(--neon-cyan)' }}>
                            {key}
                          </span>
                          <span className="flex-1 break-all" style={{ color: 'var(--neon-pink)' }}>
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
            <>
              <div className="card-vapor p-6 rounded-xl">
                <h3 className="text-xl font-bold neon-text mb-4 flex items-center gap-2" style={{ color: 'var(--neon-pink)' }}>
                  <Globe size={18} weight="duotone" />
                  Network Configuration
                </h3>
                <div className="grid grid-cols-1 md:grid-cols-2 gap-4 font-mono text-sm mb-6">
                  <div>
                    <span className="opacity-70">IP Address:</span>
                    <div className="font-bold" style={{ color: 'var(--neon-cyan)' }}>
                      {details.networkSettings.ipAddress || 'N/A'}
                    </div>
                  </div>
                  <div>
                    <span className="opacity-70">Gateway:</span>
                    <div className="font-bold" style={{ color: 'var(--neon-cyan)' }}>
                      {details.networkSettings.gateway || 'N/A'}
                    </div>
                  </div>
                </div>

                <h4 className="text-lg font-bold neon-text mb-3" style={{ color: 'var(--neon-cyan)' }}>
                  Networks
                </h4>
                <div className="space-y-3">
                  {Object.entries(details.networkSettings.networks).map(([name, network]: [string, any]) => (
                    <div key={name} className="p-4 rounded-lg" style={{ background: 'rgba(var(--neon-purple-rgb), 0.1)' }}>
                      <div className="font-bold mb-2" style={{ color: 'var(--neon-pink)' }}>
                        {name}
                      </div>
                      <div className="font-mono text-xs space-y-1">
                        <div>
                          <span className="opacity-70">IP: </span>
                          <span style={{ color: 'var(--neon-cyan)' }}>{network.IPAddress || 'N/A'}</span>
                        </div>
                        <div>
                          <span className="opacity-70">Gateway: </span>
                          <span style={{ color: 'var(--neon-cyan)' }}>{network.Gateway || 'N/A'}</span>
                        </div>
                        <div>
                          <span className="opacity-70">MAC: </span>
                          <span style={{ color: 'var(--neon-cyan)' }}>{network.MacAddress || 'N/A'}</span>
                        </div>
                      </div>
                    </div>
                  ))}
                </div>
              </div>

              <div className="card-vapor p-6 rounded-xl">
                <h3 className="text-xl font-bold neon-text mb-4 flex items-center gap-2" style={{ color: 'var(--neon-pink)' }}>
                  <Plug size={18} weight="duotone" />
                  Port Mappings
                </h3>
                {Object.keys(details.networkSettings.ports).length === 0 ? (
                  <div className="text-center py-8 opacity-60">No port mappings</div>
                ) : (
                  <div className="space-y-2">
                    {Object.entries(details.networkSettings.ports).map(([containerPort, hostBindings]: [string, any]) => (
                      <div key={containerPort} className="p-4 rounded-lg font-mono text-sm" style={{ background: 'rgba(var(--neon-purple-rgb), 0.1)' }}>
                        <div className="flex items-center gap-4">
                          <span className="font-bold" style={{ color: 'var(--neon-cyan)' }}>
                            Container: {containerPort}
                          </span>
                          <span style={{ color: 'var(--neon-purple)' }}>→</span>
                          <span className="font-bold" style={{ color: 'var(--neon-pink)' }}>
                            Host: {hostBindings && hostBindings[0]?.HostPort ? hostBindings[0].HostPort : 'Not mapped'}
                          </span>
                        </div>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            </>
          )}
        </div>
      </div>
    </div>
  );
}
