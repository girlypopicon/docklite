'use client';

import { useCallback, useEffect, useMemo, useState } from 'react';
import { useSearchParams } from 'next/navigation';
import {
  Globe,
  ShieldCheck,
  Plug,
  Lock,
  Pulse,
  WifiHigh,
  WarningCircle,
  CheckCircle,
  XCircle,
  SpinnerGap,
  Stack,
  Plugs,
} from '@phosphor-icons/react';
import SslStatus from '../components/SslStatus';
import DnsPanel from './DnsPanel';
import NginxPanel from './NginxPanel';
import PortsPanel from './PortsPanel';

interface NetworkAddress {
  family: string;
  address: string;
  prefixLen: number;
  scope: string;
  label: string;
}

interface NetworkInterface {
  name: string;
  mac: string;
  mtu: number;
  state: string;
  dhcp4: string;
  dhcp6: string;
  addresses: NetworkAddress[];
}

interface NetworkRoute {
  destination: string;
  gateway: string;
  device: string;
  prefsrc: string;
  protocol: string;
}

interface ResolverInfo {
  mode: string;
  resolvConf: string;
  nameServers: string[];
  searchDomains: string[];
}

interface NetworkOverview {
  hostname: string;
  fqdn: string;
  domain: string;
  searchDomains: string[];
  primaryIPv4: string;
  primaryIPv6: string;
  defaultGateway: string;
  defaultDevice: string;
  interfaces: NetworkInterface[];
  routes: NetworkRoute[];
  resolver: ResolverInfo;
  resolverManaged: string;
}

interface FirewallStatus {
  provider: string;
  status: string;
  details: string;
}

interface OpenPort {
  proto: string;
  address: string;
  port: number;
  process: string;
  public: boolean;
}

interface DockerPort {
  hostIp: string;
  hostPort: number;
  containerPort: number;
  proto: string;
}

interface DockerExposure {
  id: string;
  name: string;
  image: string;
  managed: boolean;
  ports: DockerPort[];
}

interface FirewallResponse {
  firewall: FirewallStatus;
  openPorts: OpenPort[];
  httpOpen: boolean;
  httpsOpen: boolean;
  sshOpen: boolean;
  otherPorts: number[];
  dockerExposed: DockerExposure[];
  lastUpdated: string;
}

interface IngressResponse {
  provider: string;
  containerName: string;
  image: string;
  state: string;
  status: string;
  ports: DockerPort[];
  bindings: string[];
  entrypoints: string[];
  httpRedirect: string;
  hsts: string;
}

interface DiagnosticResult {
  name: string;
  target: string;
  status: string;
  latencyMs: number;
  detail: string;
}

interface DiagnosticsResponse {
  publicIp: string;
  results: DiagnosticResult[];
}

type NetworkTab = 'identity' | 'dns' | 'ports' | 'firewall' | 'ingress' | 'certs' | 'nginx' | 'diagnostics';

const tabs: Array<{ key: NetworkTab; label: string; icon: JSX.Element }> = [
  { key: 'identity', label: 'Identity', icon: <WifiHigh size={18} weight="duotone" /> },
  { key: 'dns', label: 'DNS', icon: <Globe size={18} weight="duotone" /> },
  { key: 'ports', label: 'Ports', icon: <Plugs size={18} weight="duotone" /> },
  { key: 'firewall', label: 'Firewall', icon: <ShieldCheck size={18} weight="duotone" /> },
  { key: 'ingress', label: 'Ingress', icon: <Plug size={18} weight="duotone" /> },
  { key: 'certs', label: 'Certificates', icon: <Lock size={18} weight="duotone" /> },
  { key: 'nginx', label: 'Nginx', icon: <Stack size={18} weight="duotone" /> },
  { key: 'diagnostics', label: 'Diagnostics', icon: <Pulse size={18} weight="duotone" /> },
];

export default function NetworkPage() {
  const searchParams = useSearchParams();
  const requestedTab = searchParams.get('tab') as NetworkTab | null;
  const [activeTab, setActiveTab] = useState<NetworkTab>('identity');
  const [overview, setOverview] = useState<NetworkOverview | null>(null);
  const [firewall, setFirewall] = useState<FirewallResponse | null>(null);
  const [ingress, setIngress] = useState<IngressResponse | null>(null);
  const [publicIp, setPublicIp] = useState<string>('');
  const [diagnostics, setDiagnostics] = useState<DiagnosticsResponse | null>(null);
  const [loadingOverview, setLoadingOverview] = useState(true);
  const [loadingFirewall, setLoadingFirewall] = useState(true);
  const [loadingIngress, setLoadingIngress] = useState(true);
  const [runningDiagnostics, setRunningDiagnostics] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [showRoutes, setShowRoutes] = useState(false);

  useEffect(() => {
    if (requestedTab && tabs.some((tab) => tab.key === requestedTab)) {
      setActiveTab(requestedTab);
    }
  }, [requestedTab]);

  const fetchOverview = useCallback(async () => {
    setLoadingOverview(true);
    try {
      const res = await fetch('/api/network/overview');
      if (!res.ok) throw new Error('Failed to load network overview');
      const data = await res.json();
      setOverview(data);
    } catch (err: any) {
      setError(err.message || 'Failed to load network overview');
    } finally {
      setLoadingOverview(false);
    }
  }, []);

  const fetchFirewall = useCallback(async () => {
    setLoadingFirewall(true);
    try {
      const res = await fetch('/api/network/firewall');
      if (!res.ok) throw new Error('Failed to load firewall status');
      const data = await res.json();
      setFirewall(data);
    } catch (err: any) {
      setError(err.message || 'Failed to load firewall status');
    } finally {
      setLoadingFirewall(false);
    }
  }, []);

  const fetchIngress = useCallback(async () => {
    setLoadingIngress(true);
    try {
      const res = await fetch('/api/network/ingress');
      if (!res.ok) throw new Error('Failed to load ingress info');
      const data = await res.json();
      setIngress(data);
    } catch (err: any) {
      setError(err.message || 'Failed to load ingress info');
    } finally {
      setLoadingIngress(false);
    }
  }, []);

  const fetchPublicIp = useCallback(async () => {
    try {
      const res = await fetch('/api/network/public-ip');
      if (!res.ok) return;
      const data = await res.json();
      setPublicIp(data.ip || '');
    } catch {
      setPublicIp('');
    }
  }, []);

  const runDiagnostics = async () => {
    setRunningDiagnostics(true);
    try {
      const res = await fetch('/api/network/diagnostics', { method: 'POST' });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || 'Diagnostics failed');
      setDiagnostics(data);
    } catch (err: any) {
      setError(err.message || 'Diagnostics failed');
    } finally {
      setRunningDiagnostics(false);
    }
  };

  useEffect(() => {
    fetchOverview();
    fetchFirewall();
    fetchIngress();
    fetchPublicIp();
  }, [fetchOverview, fetchFirewall, fetchIngress, fetchPublicIp]);

  const resolverSummary = useMemo(() => {
    if (!overview?.resolver) return 'No resolver data.';
    const servers = overview.resolver.nameServers.length
      ? overview.resolver.nameServers.join(', ')
      : 'No resolvers detected';
    const search = overview.resolver.searchDomains.length
      ? overview.resolver.searchDomains.join(', ')
      : 'None';
    return `${servers} • search: ${search}`;
  }, [overview]);

  return (
    <div className="max-w-[1400px] mx-auto space-y-6">
      <div className="flex flex-wrap items-center justify-between gap-4">
        <div>
          <h1 className="text-3xl font-bold neon-text flex items-center gap-2" style={{ color: 'var(--neon-cyan)' }}>
            <Globe size={26} weight="duotone" />
            Network
          </h1>
          <p className="text-xs font-mono" style={{ color: 'var(--text-secondary)' }}>
            Identity, addressing, DNS, firewall exposure, ingress, and diagnostics.
          </p>
        </div>
        {error && (
          <div className="text-xs font-mono" style={{ color: 'var(--status-error)' }}>
            {error}
          </div>
        )}
      </div>

      <div className="flex flex-wrap gap-2 border-b border-neon-purple/30">
        {tabs.map((tab) => (
          <button
            key={tab.key}
            onClick={() => setActiveTab(tab.key)}
            className={`px-4 py-2 font-bold transition-colors ${
              activeTab === tab.key
                ? 'border-b-2 border-neon-pink text-neon-pink'
                : 'text-gray-400 hover:text-neon-cyan'
            }`}
          >
            <span className="inline-flex items-center gap-2">
              {tab.icon}
              {tab.label}
            </span>
          </button>
        ))}
      </div>

      {activeTab === 'identity' && (
        <div className="space-y-6">
          {loadingOverview ? (
            <div className="text-sm font-mono flex items-center gap-2" style={{ color: 'var(--text-secondary)' }}>
              <SpinnerGap size={14} weight="duotone" className="animate-spin" />
              Loading network identity...
            </div>
          ) : overview ? (
            <>
              <div className="grid grid-cols-1 xl:grid-cols-3 gap-6">
                <div className="card-vapor p-6 rounded-xl">
                  <h2 className="text-lg font-bold mb-4" style={{ color: 'var(--neon-cyan)' }}>
                    Identity
                  </h2>
                  <div className="space-y-3 text-sm font-mono">
                    <div className="flex justify-between gap-3">
                      <span style={{ color: 'var(--text-secondary)' }}>Hostname</span>
                      <span>{overview.hostname}</span>
                    </div>
                    <div className="flex justify-between gap-3">
                      <span style={{ color: 'var(--text-secondary)' }}>FQDN</span>
                      <span>{overview.fqdn || '—'}</span>
                    </div>
                    <div className="flex justify-between gap-3">
                      <span style={{ color: 'var(--text-secondary)' }}>Domain</span>
                      <span>{overview.domain || '—'}</span>
                    </div>
                    <div className="flex justify-between gap-3">
                      <span style={{ color: 'var(--text-secondary)' }}>Search</span>
                      <span>{overview.searchDomains.length ? overview.searchDomains.join(', ') : '—'}</span>
                    </div>
                  </div>
                </div>

                <div className="card-vapor p-6 rounded-xl">
                  <h2 className="text-lg font-bold mb-4" style={{ color: 'var(--neon-pink)' }}>
                    Primary Addressing
                  </h2>
                  <div className="space-y-3 text-sm font-mono">
                    <div className="flex justify-between gap-3">
                      <span style={{ color: 'var(--text-secondary)' }}>Primary IPv4</span>
                      <span>{overview.primaryIPv4 || '—'}</span>
                    </div>
                    <div className="flex justify-between gap-3">
                      <span style={{ color: 'var(--text-secondary)' }}>Primary IPv6</span>
                      <span>{overview.primaryIPv6 || '—'}</span>
                    </div>
                    <div className="flex justify-between gap-3">
                      <span style={{ color: 'var(--text-secondary)' }}>Default Gateway</span>
                      <span>{overview.defaultGateway || '—'}</span>
                    </div>
                    <div className="flex justify-between gap-3">
                      <span style={{ color: 'var(--text-secondary)' }}>Default Interface</span>
                      <span>{overview.defaultDevice || '—'}</span>
                    </div>
                    <div className="flex justify-between gap-3">
                      <span style={{ color: 'var(--text-secondary)' }}>Public IP</span>
                      <span>{publicIp || '—'}</span>
                    </div>
                  </div>
                </div>

                <div className="card-vapor p-6 rounded-xl">
                  <h2 className="text-lg font-bold mb-4" style={{ color: 'var(--neon-purple)' }}>
                    Resolver
                  </h2>
                  <div className="space-y-3 text-sm font-mono">
                    <div className="flex justify-between gap-3">
                      <span style={{ color: 'var(--text-secondary)' }}>Mode</span>
                      <span>{overview.resolver.mode}</span>
                    </div>
                    <div className="flex justify-between gap-3">
                      <span style={{ color: 'var(--text-secondary)' }}>Resolv.conf</span>
                      <span className="truncate max-w-[220px]">{overview.resolver.resolvConf}</span>
                    </div>
                    <div className="text-xs" style={{ color: 'var(--text-secondary)' }}>
                      {resolverSummary}
                    </div>
                  </div>
                </div>
              </div>

              <div className="card-vapor p-6 rounded-xl">
                <div className="flex items-center justify-between mb-4">
                  <h2 className="text-lg font-bold" style={{ color: 'var(--neon-cyan)' }}>
                    Interfaces
                  </h2>
                  <button
                    type="button"
                    onClick={() => setShowRoutes((prev) => !prev)}
                    className="text-xs font-bold px-3 py-1 rounded-full"
                    style={{
                      background: 'rgba(var(--neon-cyan-rgb), 0.1)',
                      border: '1px solid rgba(var(--neon-cyan-rgb), 0.3)',
                      color: 'var(--neon-cyan)',
                    }}
                  >
                    {showRoutes ? 'Hide routes' : 'Show routes'}
                  </button>
                </div>
                <div className="overflow-x-auto">
                  <table className="w-full text-xs font-mono">
                    <thead>
                      <tr style={{ borderBottom: '1px solid rgba(var(--neon-purple-rgb), 0.4)' }}>
                        <th className="text-left py-2">Interface</th>
                        <th className="text-left py-2">Addresses</th>
                        <th className="text-left py-2">DHCP</th>
                        <th className="text-left py-2">MTU</th>
                        <th className="text-left py-2">State</th>
                      </tr>
                    </thead>
                    <tbody>
                      {overview.interfaces.map((iface) => (
                        <tr key={iface.name} className="border-t border-purple-500/10">
                          <td className="py-2 pr-2">
                            <div className="font-bold" style={{ color: 'var(--neon-cyan)' }}>{iface.name}</div>
                            <div className="text-[10px] opacity-70">{iface.mac}</div>
                          </td>
                          <td className="py-2 pr-2">
                            {iface.addresses.length === 0 ? '—' : (
                              <div className="space-y-1">
                                {iface.addresses.map((addr) => (
                                  <div key={`${iface.name}-${addr.address}-${addr.family}`}>
                                    <span>{addr.address}/{addr.prefixLen}</span>
                                    <span className="ml-2 text-[10px] opacity-70">{addr.family}</span>
                                  </div>
                                ))}
                              </div>
                            )}
                          </td>
                          <td className="py-2 pr-2">
                            <div>IPv4: {iface.dhcp4}</div>
                            <div>IPv6: {iface.dhcp6}</div>
                          </td>
                          <td className="py-2 pr-2">{iface.mtu}</td>
                          <td className="py-2 pr-2">{iface.state}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>

                {showRoutes && (
                  <div className="mt-6">
                    <h3 className="text-sm font-bold mb-3" style={{ color: 'var(--neon-pink)' }}>
                      Routes
                    </h3>
                    <div className="overflow-x-auto">
                      <table className="w-full text-xs font-mono">
                        <thead>
                          <tr style={{ borderBottom: '1px solid rgba(var(--neon-purple-rgb), 0.4)' }}>
                            <th className="text-left py-2">Destination</th>
                            <th className="text-left py-2">Gateway</th>
                            <th className="text-left py-2">Device</th>
                            <th className="text-left py-2">Protocol</th>
                          </tr>
                        </thead>
                        <tbody>
                          {overview.routes.map((route, index) => (
                            <tr key={`${route.destination}-${index}`} className="border-t border-purple-500/10">
                              <td className="py-2 pr-2">{route.destination || '—'}</td>
                              <td className="py-2 pr-2">{route.gateway || '—'}</td>
                              <td className="py-2 pr-2">{route.device || '—'}</td>
                              <td className="py-2 pr-2">{route.protocol || '—'}</td>
                            </tr>
                          ))}
                        </tbody>
                      </table>
                    </div>
                  </div>
                )}
              </div>
            </>
          ) : (
            <div className="text-sm font-mono" style={{ color: 'var(--text-secondary)' }}>
              No network identity data available.
            </div>
          )}
        </div>
      )}

      {activeTab === 'dns' && (
        <DnsPanel />
      )}


      {activeTab === 'ports' && firewall && (
        <PortsPanel openPorts={firewall.openPorts} dockerExposed={firewall.dockerExposed} />
      )}
      {activeTab === 'ports' && !firewall && loadingFirewall && (
        <div className="flex items-center gap-2 text-sm font-mono" style={{ color: 'var(--text-secondary)' }}>
          <SpinnerGap size={14} className="animate-spin" />
          Loading port data…
        </div>
      )}

      {activeTab === 'firewall' && (
        <div className="space-y-6">
          {loadingFirewall ? (
            <div className="text-sm font-mono flex items-center gap-2" style={{ color: 'var(--text-secondary)' }}>
              <SpinnerGap size={14} weight="duotone" className="animate-spin" />
              Loading firewall status...
            </div>
          ) : firewall ? (
            <>
              <div className="grid grid-cols-1 xl:grid-cols-3 gap-6">
                <div className="card-vapor p-6 rounded-xl">
                  <h2 className="text-lg font-bold mb-4" style={{ color: 'var(--neon-cyan)' }}>
                    Firewall Status
                  </h2>
                  <div className="space-y-3 text-sm font-mono">
                    <div className="flex justify-between">
                      <span style={{ color: 'var(--text-secondary)' }}>Provider</span>
                      <span>{firewall.firewall.provider}</span>
                    </div>
                    <div className="flex justify-between">
                      <span style={{ color: 'var(--text-secondary)' }}>Status</span>
                      <span>{firewall.firewall.status}</span>
                    </div>
                    <div className="text-xs" style={{ color: 'var(--text-secondary)' }}>
                      {firewall.firewall.details ? firewall.firewall.details.split('\n')[0] : 'No details available'}
                    </div>
                  </div>
                </div>

                <div className="card-vapor p-6 rounded-xl">
                  <h2 className="text-lg font-bold mb-4" style={{ color: 'var(--neon-pink)' }}>
                    Inbound Overview
                  </h2>
                  <div className="space-y-3 text-sm font-mono">
                    <div className="flex items-center justify-between">
                      <span style={{ color: 'var(--text-secondary)' }}>HTTP :80</span>
                      <span style={{ color: firewall.httpOpen ? 'var(--neon-green)' : 'var(--status-error)' }}>
                        {firewall.httpOpen ? 'Open' : 'Closed'}
                      </span>
                    </div>
                    <div className="flex items-center justify-between">
                      <span style={{ color: 'var(--text-secondary)' }}>HTTPS :443</span>
                      <span style={{ color: firewall.httpsOpen ? 'var(--neon-green)' : 'var(--status-error)' }}>
                        {firewall.httpsOpen ? 'Open' : 'Closed'}
                      </span>
                    </div>
                    <div className="flex items-center justify-between">
                      <span style={{ color: 'var(--text-secondary)' }}>SSH :22</span>
                      <span style={{ color: firewall.sshOpen ? 'var(--neon-green)' : 'var(--status-error)' }}>
                        {firewall.sshOpen ? 'Open' : 'Closed'}
                      </span>
                    </div>
                    <div className="text-xs" style={{ color: 'var(--text-secondary)' }}>
                      Other exposed ports: {firewall.otherPorts.length ? firewall.otherPorts.join(', ') : 'none'}
                    </div>
                  </div>
                </div>

                <div className="card-vapor p-6 rounded-xl">
                  <h2 className="text-lg font-bold mb-4" style={{ color: 'var(--neon-purple)' }}>
                    Exposure Risk
                  </h2>
                  <div className="text-xs font-mono" style={{ color: 'var(--text-secondary)' }}>
                    {firewall.dockerExposed.length} containers publishing host ports.
                  </div>
                  <div className="mt-3 text-sm">
                    {firewall.otherPorts.length > 0 ? (
                      <span className="inline-flex items-center gap-2" style={{ color: 'var(--status-warning)' }}>
                        <WarningCircle size={14} weight="duotone" />
                        Review non-standard open ports.
                      </span>
                    ) : (
                      <span className="inline-flex items-center gap-2" style={{ color: 'var(--neon-green)' }}>
                        <CheckCircle size={14} weight="duotone" />
                        No unexpected open ports detected.
                      </span>
                    )}
                  </div>
                </div>
              </div>

              <div className="card-vapor p-6 rounded-xl">
                <h2 className="text-lg font-bold mb-4" style={{ color: 'var(--neon-cyan)' }}>
                  Listening Ports
                </h2>
                <div className="overflow-x-auto">
                  <table className="w-full text-xs font-mono">
                    <thead>
                      <tr style={{ borderBottom: '1px solid rgba(var(--neon-purple-rgb), 0.4)' }}>
                        <th className="text-left py-2">Port</th>
                        <th className="text-left py-2">Protocol</th>
                        <th className="text-left py-2">Address</th>
                        <th className="text-left py-2">Process</th>
                        <th className="text-left py-2">Public</th>
                      </tr>
                    </thead>
                    <tbody>
                      {firewall.openPorts.length === 0 ? (
                        <tr>
                          <td colSpan={5} className="py-4 text-center opacity-70">No listening ports detected.</td>
                        </tr>
                      ) : firewall.openPorts.map((port, index) => (
                        <tr key={`${port.port}-${index}`} className="border-t border-purple-500/10">
                          <td className="py-2 pr-2">{port.port}</td>
                          <td className="py-2 pr-2">{port.proto}</td>
                          <td className="py-2 pr-2">{port.address}</td>
                          <td className="py-2 pr-2">{port.process || '—'}</td>
                          <td className="py-2 pr-2">
                            {port.public ? (
                              <span style={{ color: 'var(--status-error)' }}>Public</span>
                            ) : (
                              <span style={{ color: 'var(--text-secondary)' }}>Local</span>
                            )}
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              </div>

              <div className="card-vapor p-6 rounded-xl">
                <h2 className="text-lg font-bold mb-4" style={{ color: 'var(--neon-pink)' }}>
                  Container Port Mapping
                </h2>
                <div className="space-y-3">
                  {firewall.dockerExposed.length === 0 ? (
                    <div className="text-xs font-mono" style={{ color: 'var(--text-secondary)' }}>
                      No containers exposing host ports.
                    </div>
                  ) : firewall.dockerExposed.map((container) => (
                    <div key={container.id} className="border border-purple-500/20 rounded-lg p-4">
                      <div className="flex items-center justify-between">
                        <div>
                          <div className="font-bold" style={{ color: 'var(--neon-cyan)' }}>{container.name}</div>
                          <div className="text-[11px] opacity-70">{container.image}</div>
                        </div>
                        {container.managed ? (
                          <span className="text-[10px] px-2 py-1 rounded-full" style={{ background: 'rgba(var(--neon-cyan-rgb), 0.1)', color: 'var(--neon-cyan)' }}>
                            DockLite managed
                          </span>
                        ) : (
                          <span className="text-[10px] px-2 py-1 rounded-full" style={{ background: 'rgba(var(--status-warning-rgb), 0.12)', color: 'var(--status-warning)' }}>
                            External
                          </span>
                        )}
                      </div>
                      <div className="mt-2 flex flex-wrap gap-2">
                        {container.ports.map((port, index) => (
                          <span
                            key={`${container.id}-${index}`}
                            className="text-xs font-mono px-2 py-1 rounded-full"
                            style={{
                              background: 'rgba(var(--neon-purple-rgb), 0.15)',
                              color: 'var(--neon-purple)'
                            }}
                          >
                            {port.hostIp}:{port.hostPort} → {port.containerPort}/{port.proto}
                          </span>
                        ))}
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            </>
          ) : (
            <div className="text-sm font-mono" style={{ color: 'var(--text-secondary)' }}>
              No firewall data available.
            </div>
          )}
        </div>
      )}

      {activeTab === 'ingress' && (
        <div className="space-y-6">
          {loadingIngress ? (
            <div className="text-sm font-mono flex items-center gap-2" style={{ color: 'var(--text-secondary)' }}>
              <SpinnerGap size={14} weight="duotone" className="animate-spin" />
              Loading ingress details...
            </div>
          ) : ingress ? (
            <>
              <div className="grid grid-cols-1 xl:grid-cols-3 gap-6">
                <div className="card-vapor p-6 rounded-xl">
                  <h2 className="text-lg font-bold mb-4" style={{ color: 'var(--neon-cyan)' }}>
                    Reverse Proxy
                  </h2>
                  <div className="space-y-3 text-sm font-mono">
                    <div className="flex justify-between gap-3">
                      <span style={{ color: 'var(--text-secondary)' }}>Provider</span>
                      <span>{ingress.provider}</span>
                    </div>
                    <div className="flex justify-between gap-3">
                      <span style={{ color: 'var(--text-secondary)' }}>Container</span>
                      <span>{ingress.containerName || '—'}</span>
                    </div>
                    <div className="flex justify-between gap-3">
                      <span style={{ color: 'var(--text-secondary)' }}>Image</span>
                      <span>{ingress.image || '—'}</span>
                    </div>
                    <div className="flex justify-between gap-3">
                      <span style={{ color: 'var(--text-secondary)' }}>State</span>
                      <span>{ingress.state || '—'}</span>
                    </div>
                  </div>
                </div>

                <div className="card-vapor p-6 rounded-xl">
                  <h2 className="text-lg font-bold mb-4" style={{ color: 'var(--neon-pink)' }}>
                    Bindings
                  </h2>
                  <div className="space-y-2 text-sm font-mono">
                    {ingress.bindings.length === 0 ? (
                      <div className="text-xs" style={{ color: 'var(--text-secondary)' }}>
                        No bindings detected.
                      </div>
                    ) : ingress.bindings.map((binding) => (
                      <div key={binding}>{binding}</div>
                    ))}
                  </div>
                  <div className="text-xs mt-3" style={{ color: 'var(--text-secondary)' }}>
                    Entrypoints: {ingress.entrypoints.length ? ingress.entrypoints.join(', ') : '—'}
                  </div>
                </div>

                <div className="card-vapor p-6 rounded-xl">
                  <h2 className="text-lg font-bold mb-4" style={{ color: 'var(--neon-purple)' }}>
                    TLS + Redirect
                  </h2>
                  <div className="space-y-2 text-sm font-mono">
                    <div className="flex justify-between">
                      <span style={{ color: 'var(--text-secondary)' }}>HTTP → HTTPS</span>
                      <span>{ingress.httpRedirect}</span>
                    </div>
                    <div className="flex justify-between">
                      <span style={{ color: 'var(--text-secondary)' }}>HSTS</span>
                      <span>{ingress.hsts}</span>
                    </div>
                  </div>
                  <div className="text-xs mt-3" style={{ color: 'var(--text-secondary)' }}>
                    Read-only view (reverse proxy config).
                  </div>
                </div>
              </div>

              <div className="card-vapor p-6 rounded-xl">
                <h2 className="text-lg font-bold mb-4" style={{ color: 'var(--neon-cyan)' }}>
                  Port Exposure
                </h2>
                <div className="flex flex-wrap gap-2">
                  {ingress.ports.length === 0 ? (
                    <span className="text-xs font-mono" style={{ color: 'var(--text-secondary)' }}>
                      No published ports.
                    </span>
                  ) : ingress.ports.map((port, index) => (
                    <span
                      key={`${port.hostIp}-${port.hostPort}-${index}`}
                      className="text-xs font-mono px-3 py-1 rounded-full"
                      style={{ background: 'rgba(var(--neon-cyan-rgb), 0.12)', color: 'var(--neon-cyan)' }}
                    >
                      {port.hostIp}:{port.hostPort} → {port.containerPort}/{port.proto}
                    </span>
                  ))}
                </div>
              </div>
            </>
          ) : (
            <div className="text-sm font-mono" style={{ color: 'var(--text-secondary)' }}>
              No ingress data available.
            </div>
          )}
        </div>
      )}

      {activeTab === 'certs' && (
        <SslStatus />
      )}

      {activeTab === 'nginx' && (
        <NginxPanel />
      )}

      {activeTab === 'diagnostics' && (
        <div className="space-y-6">
          <div className="card-vapor p-6 rounded-xl">
            <div className="flex flex-wrap items-center justify-between gap-3">
              <div>
                <h2 className="text-lg font-bold" style={{ color: 'var(--neon-cyan)' }}>
                  Connectivity Diagnostics
                </h2>
                <p className="text-xs font-mono" style={{ color: 'var(--text-secondary)' }}>
                  Ping gateway, public DNS, resolve domains, HTTPS probe, and port checks.
                </p>
              </div>
              <button
                onClick={runDiagnostics}
                disabled={runningDiagnostics}
                className="btn-neon px-4 py-2 text-sm font-bold disabled:opacity-50"
              >
                {runningDiagnostics ? (
                  <span className="inline-flex items-center gap-2">
                    <SpinnerGap size={14} weight="duotone" className="animate-spin" />
                    Running…
                  </span>
                ) : (
                  <span className="inline-flex items-center gap-2">
                    <Pulse size={14} weight="duotone" />
                    Run Diagnostics
                  </span>
                )}
              </button>
            </div>
            <div className="mt-4 text-xs font-mono" style={{ color: 'var(--text-secondary)' }}>
              Public IP: {publicIp || diagnostics?.publicIp || '—'}
            </div>
          </div>

          <div className="card-vapor p-6 rounded-xl">
            <h2 className="text-lg font-bold mb-4" style={{ color: 'var(--neon-pink)' }}>
              Results
            </h2>
            {!diagnostics || diagnostics.results.length === 0 ? (
              <div className="text-sm font-mono" style={{ color: 'var(--text-secondary)' }}>
                Run diagnostics to see latency and resolver status.
              </div>
            ) : (
              <div className="overflow-x-auto">
                <table className="w-full text-xs font-mono">
                  <thead>
                    <tr style={{ borderBottom: '1px solid rgba(var(--neon-purple-rgb), 0.4)' }}>
                      <th className="text-left py-2">Test</th>
                      <th className="text-left py-2">Target</th>
                      <th className="text-left py-2">Status</th>
                      <th className="text-left py-2">Latency</th>
                      <th className="text-left py-2">Detail</th>
                    </tr>
                  </thead>
                  <tbody>
                    {diagnostics.results.map((result, index) => (
                      <tr key={`${result.name}-${index}`} className="border-t border-purple-500/10">
                        <td className="py-2 pr-2">{result.name}</td>
                        <td className="py-2 pr-2">{result.target}</td>
                        <td className="py-2 pr-2">
                          {result.status === 'ok' ? (
                            <span className="inline-flex items-center gap-1" style={{ color: 'var(--neon-green)' }}>
                              <CheckCircle size={12} weight="duotone" />
                              OK
                            </span>
                          ) : (
                            <span className="inline-flex items-center gap-1" style={{ color: 'var(--status-error)' }}>
                              <XCircle size={12} weight="duotone" />
                              FAIL
                            </span>
                          )}
                        </td>
                        <td className="py-2 pr-2">{result.latencyMs ? `${result.latencyMs}ms` : '—'}</td>
                        <td className="py-2 pr-2">{result.detail || '—'}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  );
}
