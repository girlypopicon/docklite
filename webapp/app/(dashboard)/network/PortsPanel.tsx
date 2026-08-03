'use client';

import { useEffect, useState } from 'react';
import {
  ArrowRight,
  Globe,
  HouseLine,
  LockSimple,
  PlugsConnected,
  Question,
  SpinnerGap,
  Warning,
} from '@phosphor-icons/react';

interface ServicePorts {
  agentAddr: string;
  agentPort: number;
  webUrl: string;
  webPort: number;
  headless: boolean;
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

interface Props {
  openPorts: OpenPort[];
  dockerExposed: DockerExposure[];
}

function addrLabel(address: string): { label: string; icon: JSX.Element; color: string } {
  if (address === '0.0.0.0' || address === '*' || address === '::') {
    return {
      label: 'All interfaces — internet-facing',
      icon: <Globe size={12} weight="duotone" />,
      color: 'var(--status-warning)',
    };
  }
  if (address === '127.0.0.1' || address === '::1') {
    return {
      label: 'Localhost only — not reachable from outside',
      icon: <HouseLine size={12} weight="duotone" />,
      color: 'var(--neon-cyan)',
    };
  }
  return {
    label: address,
    icon: <LockSimple size={12} weight="duotone" />,
    color: 'var(--text-secondary)',
  };
}

function isDockLitePort(port: number, services: ServicePorts | null): string | null {
  if (!services) return null;
  if (port === services.agentPort) return 'DockLite Agent API';
  if (port === services.webPort) return 'DockLite Web UI';
  return null;
}

export default function PortsPanel({ openPorts, dockerExposed }: Props) {
  const [services, setServices] = useState<ServicePorts | null>(null);
  const [loading, setLoading] = useState(true);
  const [currentPort, setCurrentPort] = useState<number | null>(null);

  useEffect(() => {
    if (typeof window !== 'undefined') {
      const p = parseInt(window.location.port);
      setCurrentPort(isNaN(p) ? (window.location.protocol === 'https:' ? 443 : 80) : p);
    }
    fetch('/api/server/ports')
      .then((r) => r.json())
      .then((d) => setServices(d))
      .catch(() => {})
      .finally(() => setLoading(false));
  }, []);

  return (
    <div className="space-y-6">
      {/* DockLite Services */}
      <div className="card-vapor p-6 rounded-xl">
        <h2 className="text-lg font-bold mb-1" style={{ color: 'var(--neon-cyan)' }}>
          DockLite Services
        </h2>
        <p className="text-xs font-mono mb-4" style={{ color: 'var(--text-secondary)' }}>
          The ports DockLite itself is running on.
        </p>
        {loading ? (
          <div className="flex items-center gap-2 text-sm" style={{ color: 'var(--text-secondary)' }}>
            <SpinnerGap size={14} className="animate-spin" />
            Loading…
          </div>
        ) : services ? (
          <div className="space-y-3">
            <div
              className="flex items-center justify-between rounded-lg px-4 py-3"
              style={{ background: 'rgba(var(--neon-cyan-rgb), 0.07)', border: '1px solid rgba(var(--neon-cyan-rgb), 0.2)' }}
            >
              <div>
                <div className="font-bold text-sm" style={{ color: 'var(--neon-cyan)' }}>
                  Agent API
                  {' '}
                  <span className="font-mono">:{services.agentPort}</span>
                </div>
                <div className="text-xs mt-0.5" style={{ color: 'var(--text-secondary)' }}>
                  Handles all Docker operations and serves the web UI. Every request from your browser goes through here.
                </div>
              </div>
              <span
                className="text-xs font-mono px-2 py-1 rounded-full ml-4 shrink-0"
                style={{ background: 'rgba(var(--neon-cyan-rgb), 0.15)', color: 'var(--neon-cyan)' }}
              >
                :{services.agentPort}
              </span>
            </div>

            {!services.headless && (
              <div
                className="flex items-center justify-between rounded-lg px-4 py-3"
                style={{ background: 'rgba(var(--neon-purple-rgb), 0.07)', border: '1px solid rgba(var(--neon-purple-rgb), 0.2)' }}
              >
                <div>
                  <div className="font-bold text-sm flex items-center gap-2" style={{ color: 'var(--neon-purple)' }}>
                    Web UI
                    {' '}
                    <span className="font-mono">:{services.webPort}</span>
                    {currentPort === services.webPort && (
                      <span
                        className="text-[10px] px-2 py-0.5 rounded-full"
                        style={{ background: 'rgba(var(--neon-green-rgb), 0.15)', color: 'var(--neon-green)' }}
                      >
                        ← you are here
                      </span>
                    )}
                  </div>
                  <div className="text-xs mt-0.5" style={{ color: 'var(--text-secondary)' }}>
                    Internal Next.js server. Not exposed publicly — the Agent proxies web traffic to it. You don&apos;t need to open this port.
                  </div>
                </div>
                <span
                  className="text-xs font-mono px-2 py-1 rounded-full ml-4 shrink-0"
                  style={{ background: 'rgba(var(--neon-purple-rgb), 0.15)', color: 'var(--neon-purple)' }}
                >
                  :{services.webPort}
                </span>
              </div>
            )}

            {services.headless && (
              <div className="text-xs font-mono" style={{ color: 'var(--text-secondary)' }}>
                Running in headless mode — no web UI, TUI/API access only.
              </div>
            )}

            <div
              className="text-xs font-mono rounded-lg px-4 py-3 flex items-start gap-2"
              style={{ background: 'rgba(var(--neon-cyan-rgb), 0.04)', color: 'var(--text-secondary)' }}
            >
              <PlugsConnected size={14} className="shrink-0 mt-0.5" style={{ color: 'var(--neon-cyan)' }} />
              <span>
                To access DockLite from your browser, use{' '}
                <span className="font-bold" style={{ color: 'var(--text-primary)' }}>
                  http://&lt;your-server-ip&gt;:{services.agentPort}
                </span>
                {' '} — that&apos;s the only port you need to expose.
              </span>
            </div>
          </div>
        ) : (
          <div className="flex items-center gap-2 text-xs" style={{ color: 'var(--status-error)' }}>
            <Warning size={14} />
            Could not load service port info.
          </div>
        )}
      </div>

      {/* Address guide */}
      <div className="card-vapor p-6 rounded-xl">
        <h2 className="text-lg font-bold mb-1" style={{ color: 'var(--neon-pink)' }}>
          What do those IPs mean?
        </h2>
        <p className="text-xs font-mono mb-4" style={{ color: 'var(--text-secondary)' }}>
          Every port is &quot;bound&quot; to an IP address — this controls who can reach it.
        </p>
        <div className="space-y-2 text-sm font-mono">
          <div className="flex items-start gap-3">
            <span
              className="inline-flex items-center gap-1 px-2 py-0.5 rounded text-xs shrink-0"
              style={{ background: 'rgba(var(--status-warning-rgb), 0.12)', color: 'var(--status-warning)' }}
            >
              <Globe size={11} /> 0.0.0.0
            </span>
            <span style={{ color: 'var(--text-secondary)' }}>
              Bound on <strong style={{ color: 'var(--text-primary)' }}>all network interfaces</strong> — reachable from the internet if your firewall allows it.
            </span>
          </div>
          <div className="flex items-start gap-3">
            <span
              className="inline-flex items-center gap-1 px-2 py-0.5 rounded text-xs shrink-0"
              style={{ background: 'rgba(var(--neon-cyan-rgb), 0.1)', color: 'var(--neon-cyan)' }}
            >
              <HouseLine size={11} /> 127.0.0.1
            </span>
            <span style={{ color: 'var(--text-secondary)' }}>
              Bound on <strong style={{ color: 'var(--text-primary)' }}>localhost only</strong> — only processes on this server can reach it. Safe from the internet.
            </span>
          </div>
          <div className="flex items-start gap-3">
            <span
              className="inline-flex items-center gap-1 px-2 py-0.5 rounded text-xs shrink-0"
              style={{ background: 'rgba(var(--text-secondary-rgb, 128, 128, 128), 0.1)', color: 'var(--text-secondary)' }}
            >
              <LockSimple size={11} /> specific IP
            </span>
            <span style={{ color: 'var(--text-secondary)' }}>
              Bound to a <strong style={{ color: 'var(--text-primary)' }}>specific interface</strong> — only reachable via that IP address.
            </span>
          </div>
        </div>
      </div>

      {/* Listening ports */}
      <div className="card-vapor p-6 rounded-xl">
        <h2 className="text-lg font-bold mb-1" style={{ color: 'var(--neon-cyan)' }}>
          All Listening Ports
        </h2>
        <p className="text-xs font-mono mb-4" style={{ color: 'var(--text-secondary)' }}>
          Every port this server is currently listening on.
        </p>
        {openPorts.length === 0 ? (
          <div className="text-xs font-mono" style={{ color: 'var(--text-secondary)' }}>No listening ports detected.</div>
        ) : (
          <div className="space-y-2">
            {openPorts.map((p, i) => {
              const addr = addrLabel(p.address);
              const dkl = isDockLitePort(p.port, services);
              return (
                <div
                  key={`${p.port}-${i}`}
                  className="flex flex-wrap items-center gap-3 rounded-lg px-4 py-2.5 text-sm font-mono"
                  style={{ background: 'rgba(var(--neon-purple-rgb), 0.05)', border: '1px solid rgba(var(--neon-purple-rgb), 0.12)' }}
                >
                  <span className="font-bold w-14" style={{ color: 'var(--neon-cyan)' }}>
                    :{p.port}
                  </span>
                  <span className="text-xs" style={{ color: 'var(--text-secondary)' }}>
                    {p.proto.toUpperCase()}
                  </span>
                  <span
                    className="inline-flex items-center gap-1 text-xs px-2 py-0.5 rounded"
                    style={{ background: 'rgba(0,0,0,0.15)', color: addr.color }}
                  >
                    {addr.icon}
                    {addr.label}
                  </span>
                  {p.process && (
                    <span className="text-xs" style={{ color: 'var(--text-secondary)' }}>
                      {p.process}
                    </span>
                  )}
                  {dkl && (
                    <span
                      className="text-xs px-2 py-0.5 rounded-full ml-auto"
                      style={{ background: 'rgba(var(--neon-cyan-rgb), 0.12)', color: 'var(--neon-cyan)' }}
                    >
                      {dkl}
                    </span>
                  )}
                </div>
              );
            })}
          </div>
        )}
      </div>

      {/* Container port mappings */}
      <div className="card-vapor p-6 rounded-xl">
        <h2 className="text-lg font-bold mb-1" style={{ color: 'var(--neon-pink)' }}>
          Container Port Mappings
        </h2>
        <p className="text-xs font-mono mb-4" style={{ color: 'var(--text-secondary)' }}>
          For each container: the <strong style={{ color: 'var(--text-primary)' }}>host (outside) port</strong> your server listens on, mapped to the{' '}
          <strong style={{ color: 'var(--text-primary)' }}>container (inside) port</strong> the app uses internally.
        </p>
        {dockerExposed.length === 0 ? (
          <div className="text-xs font-mono" style={{ color: 'var(--text-secondary)' }}>No containers exposing host ports.</div>
        ) : (
          <div className="space-y-4">
            {dockerExposed.map((c) => (
              <div
                key={c.id}
                className="rounded-xl p-4"
                style={{ border: '1px solid rgba(var(--neon-purple-rgb), 0.2)', background: 'rgba(var(--neon-purple-rgb), 0.04)' }}
              >
                <div className="flex items-center gap-2 mb-3">
                  <span className="font-bold text-sm" style={{ color: 'var(--neon-cyan)' }}>{c.name}</span>
                  {c.managed ? (
                    <span
                      className="text-[10px] px-2 py-0.5 rounded-full"
                      style={{ background: 'rgba(var(--neon-cyan-rgb), 0.1)', color: 'var(--neon-cyan)' }}
                    >
                      DockLite managed
                    </span>
                  ) : (
                    <span
                      className="text-[10px] px-2 py-0.5 rounded-full"
                      style={{ background: 'rgba(var(--status-warning-rgb), 0.1)', color: 'var(--status-warning)' }}
                    >
                      External
                    </span>
                  )}
                </div>
                {c.ports.length === 0 ? (
                  <div className="text-xs font-mono" style={{ color: 'var(--text-secondary)' }}>No published ports.</div>
                ) : (
                  <div className="space-y-2">
                    {c.ports.map((port, idx) => {
                      const addr = addrLabel(port.hostIp);
                      return (
                        <div
                          key={idx}
                          className="flex flex-wrap items-center gap-2 text-xs font-mono rounded-lg px-3 py-2"
                          style={{ background: 'rgba(var(--neon-cyan-rgb), 0.05)' }}
                        >
                          {/* Outside */}
                          <div className="flex flex-col items-start">
                            <span className="text-[10px] mb-0.5" style={{ color: 'var(--text-secondary)' }}>
                              Host port (outside)
                            </span>
                            <span className="font-bold" style={{ color: 'var(--neon-cyan)' }}>
                              :{port.hostPort}
                            </span>
                            <span
                              className="inline-flex items-center gap-1 text-[10px] mt-0.5"
                              style={{ color: addr.color }}
                            >
                              {addr.icon} {port.hostIp === '0.0.0.0' ? 'internet-facing' : port.hostIp}
                            </span>
                          </div>
                          <ArrowRight size={14} style={{ color: 'var(--text-secondary)' }} />
                          {/* Inside */}
                          <div className="flex flex-col items-start">
                            <span className="text-[10px] mb-0.5" style={{ color: 'var(--text-secondary)' }}>
                              Container port (inside)
                            </span>
                            <span className="font-bold" style={{ color: 'var(--neon-purple)' }}>
                              :{port.containerPort}
                            </span>
                            <span className="text-[10px] mt-0.5" style={{ color: 'var(--text-secondary)' }}>
                              {port.proto}
                            </span>
                          </div>
                        </div>
                      );
                    })}
                  </div>
                )}
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
