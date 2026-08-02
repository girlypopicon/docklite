'use client';

import { useCallback, useEffect, useState } from 'react';
import { ArrowRight, Globe, ShieldCheck, HardDrive, Cube, SpinnerGap, ArrowsClockwise, Anchor, CloudArrowUp } from '@phosphor-icons/react';

interface NginxSite {
  domain: string;
  templateType: string;
  enabled: boolean;
  hasConfig: boolean;
}

interface DockerPort {
  hostIp: string;
  hostPort: number;
  containerPort: number;
  proto: string;
}

interface DockerContainer {
  id: string;
  name: string;
  image: string;
  managed: boolean;
  ports: DockerPort[];
}

interface OpenPort {
  proto: string;
  address: string;
  port: number;
  process: string;
  public: boolean;
}

export default function NginxFlowPanel() {
  const [sites, setSites] = useState<NginxSite[]>([]);
  const [containers, setContainers] = useState<DockerContainer[]>([]);
  const [openPorts, setOpenPorts] = useState<OpenPort[]>([]);
  const [loading, setLoading] = useState(true);
  const [nginxStatus, setNginxStatus] = useState<'active' | 'inactive' | 'unknown'>('unknown');

  const fetchData = useCallback(async () => {
    setLoading(true);
    try {
      const [sitesRes, firewallRes, servicesRes] = await Promise.all([
        fetch('/api/nginx/sites'),
        fetch('/api/network/firewall'),
        fetch('/api/server/services'),
      ]);

      if (sitesRes.ok) {
        const data = await sitesRes.json();
        setSites(data.sites || []);
      }

      if (firewallRes.ok) {
        const data = await firewallRes.json();
        setContainers(data.dockerExposed || []);
        setOpenPorts(data.openPorts || []);
      }

      if (servicesRes.ok) {
        const svcData = await servicesRes.json();
        const proxyStatus = svcData.proxy?.status?.toLowerCase();
        setNginxStatus(['running', 'active'].includes(proxyStatus) ? 'active' : 'inactive');
      }
    } catch {
      // ignore
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { fetchData(); }, [fetchData]);

  const statusDot = (status: 'online' | 'offline' | 'unknown') => {
    const colors = { online: 'var(--neon-green)', offline: 'var(--status-error)', unknown: 'var(--text-secondary)' };
    return (
      <span
        className="inline-block w-2 h-2 rounded-full flex-shrink-0"
        style={{ backgroundColor: colors[status], boxShadow: status === 'online' ? `0 0 6px ${colors[status]}` : 'none' }}
      />
    );
  };

  const PortBadge = ({ port, type, label }: { port: number | string; type: 'nginx' | 'docker'; label?: string }) => {
    const isNginx = type === 'nginx';
    const bg = isNginx ? 'rgba(var(--neon-green-rgb, 0,255,100), 0.1)' : 'rgba(var(--neon-cyan-rgb), 0.1)';
    const color = isNginx ? 'var(--neon-green)' : 'var(--neon-cyan)';
    const borderColor = isNginx ? 'rgba(var(--neon-green-rgb, 0,255,100), 0.3)' : 'rgba(var(--neon-cyan-rgb), 0.3)';
    const Icon = isNginx ? ShieldCheck : Anchor;

    return (
      <div className="flex flex-col items-center gap-0.5">
        {label && <div className="text-[9px] font-mono" style={{ color: 'var(--text-secondary)' }}>{label}</div>}
        <div
          className="text-[10px] font-mono font-bold px-2 py-0.5 rounded-full flex items-center gap-1"
          style={{ background: bg, color, border: `1px solid ${borderColor}` }}
        >
          <Icon size={10} weight="bold" />
          :{port}
        </div>
      </div>
    );
  };

  const Arrow = () => (
    <div className="flex items-center mx-0.5 flex-shrink-0">
      <div className="h-[2px] w-4 rounded" style={{ background: 'linear-gradient(90deg, var(--neon-cyan), var(--neon-purple))' }} />
      <ArrowRight size={10} style={{ color: 'var(--neon-cyan)' }} />
    </div>
  );

  const NodeBox = ({ label, sublabel, color, status, icon, small }: {
    label: string; sublabel?: string; color: string; status: 'online' | 'offline' | 'unknown';
    icon: 'globe' | 'nginx' | 'agent' | 'gui' | 'container'; small?: boolean;
  }) => {
    const icons = {
      globe: <Globe size={small ? 20 : 24} weight="duotone" style={{ color }} />,
      nginx: <ShieldCheck size={small ? 20 : 24} weight="duotone" style={{ color }} />,
      agent: <HardDrive size={small ? 20 : 24} weight="duotone" style={{ color }} />,
      gui: <CloudArrowUp size={small ? 20 : 24} weight="duotone" style={{ color }} />,
      container: <Cube size={small ? 20 : 24} weight="duotone" style={{ color }} />,
    };

    return (
      <div
        className={`flex flex-col items-center gap-0.5 rounded-xl border flex-shrink-0 ${small ? 'px-3 py-2 min-w-[80px]' : 'px-4 py-3 min-w-[100px]'}`}
        style={{
          background: 'var(--surface-muted)',
          borderColor: `color-mix(in srgb, ${color} 40%, transparent)`,
          boxShadow: `0 0 12px color-mix(in srgb, ${color} 10%, transparent)`,
        }}
      >
        {icons[icon]}
        <div className={`font-bold ${small ? 'text-[10px]' : 'text-xs'}`} style={{ color }}>{label}</div>
        {sublabel && (
          <div className="text-[9px] font-mono text-center max-w-[90px] truncate" style={{ color: 'var(--text-secondary)' }}>{sublabel}</div>
        )}
        <div className="flex items-center gap-1">
          {statusDot(status)}
          <span className="text-[9px]" style={{ color: 'var(--text-secondary)' }}>{status}</span>
        </div>
      </div>
    );
  };

  const domainFromName = (name: string) => {
    return name.replace(/^docklite-site-/, '').replace(/-com$/, '.com').replace(/-org$/, '.org').replace(/-net$/, '.net').replace(/-io$/, '.io').replace(/-/g, '-');
  };

  const findNginxSite = (container: DockerContainer) => {
    const domain = domainFromName(container.name);
    return sites.find(s => domain.includes(s.domain.replace(/\./g, '-')) || s.domain === domain);
  };

  if (loading) {
    return (
      <div className="card-vapor p-6 rounded-xl">
        <div className="text-sm font-mono flex items-center gap-2" style={{ color: 'var(--text-secondary)' }}>
          <SpinnerGap size={14} className="animate-spin" />
          Loading traffic flow...
        </div>
      </div>
    );
  }

  const managedContainers = containers.filter(c => c.managed);
  const unmanagedContainers = containers.filter(c => !c.managed && c.ports.length > 0);

  return (
    <div className="card-vapor p-6 rounded-xl">
      <div className="flex items-center justify-between mb-6">
        <div>
          <h2 className="text-xl font-bold neon-text flex items-center gap-2" style={{ color: 'var(--neon-cyan)' }}>
            <ArrowsClockwise size={18} weight="duotone" />
            Traffic Flow
          </h2>
          <p className="text-xs font-mono mt-1" style={{ color: 'var(--text-secondary)' }}>
            How requests route through nginx and Docker to your services.
          </p>
        </div>
        <button onClick={fetchData} className="btn-neon px-3 py-1 text-xs font-bold">Refresh</button>
      </div>

      {/* DockLite core flow */}
      <div className="mb-6">
        <div className="text-[10px] font-bold tracking-widest mb-3" style={{ color: 'var(--neon-purple)' }}>DOCKLITE DASHBOARD</div>
        <div className="flex items-center gap-1 overflow-x-auto pb-2">
          <NodeBox label="Internet" sublabel="browser" icon="globe" color="var(--neon-pink)" status="online" />
          <Arrow />
          <PortBadge port={80} type="nginx" label="HTTP" />
          <Arrow />
          <NodeBox label="Nginx" sublabel="reverse proxy" icon="nginx" color="var(--neon-green)" status={nginxStatus === 'active' ? 'online' : 'offline'} />
          <Arrow />
          <PortBadge port={3000} type="nginx" label="proxy_pass" />
          <Arrow />
          <NodeBox label="Agent" sublabel="docklite-agent" icon="agent" color="var(--neon-cyan)" status="online" />
          <Arrow />
          <PortBadge port={3001} type="docker" label="internal" />
          <Arrow />
          <NodeBox label="Web GUI" sublabel="Next.js" icon="gui" color="var(--neon-purple)" status="online" />
        </div>
      </div>

      {/* Managed container flows */}
      {managedContainers.length > 0 && (
        <div className="mb-6">
          <div className="text-[10px] font-bold tracking-widest mb-3" style={{ color: 'var(--neon-purple)' }}>HOSTED SITES</div>
          <div className="space-y-3">
            {managedContainers.map((container) => {
              const site = findNginxSite(container);
              const domain = site?.domain || domainFromName(container.name);
              const hostPort = container.ports[0]?.hostPort;
              const containerPort = container.ports[0]?.containerPort;

              return (
                <div key={container.id} className="flex items-center gap-1 overflow-x-auto pb-2">
                  <NodeBox label={domain} sublabel="visitor" icon="globe" color="var(--neon-pink)" status="online" small />
                  <Arrow />
                  <PortBadge port={80} type="nginx" />
                  <Arrow />
                  <NodeBox
                    label="Nginx"
                    sublabel={domain}
                    icon="nginx"
                    color="var(--neon-green)"
                    status={site?.enabled ? 'online' : 'unknown'}
                    small
                  />
                  <Arrow />
                  <PortBadge port={hostPort || '?'} type="nginx" label="proxy_pass" />
                  <Arrow />
                  <PortBadge port={containerPort || '?'} type="docker" label="container" />
                  <Arrow />
                  <NodeBox
                    label={container.name.replace('docklite-site-', '')}
                    sublabel={container.image}
                    icon="container"
                    color="var(--neon-cyan)"
                    status="online"
                    small
                  />
                </div>
              );
            })}
          </div>
        </div>
      )}

      {/* Unmanaged containers with exposed ports */}
      {unmanagedContainers.length > 0 && (
        <div className="mb-6">
          <div className="text-[10px] font-bold tracking-widest mb-3" style={{ color: 'var(--status-warning)' }}>EXTERNAL CONTAINERS (not managed by DockLite)</div>
          <div className="space-y-3">
            {unmanagedContainers.map((container) => (
              <div key={container.id} className="flex items-center gap-1 overflow-x-auto pb-2">
                <NodeBox
                  label={container.name}
                  sublabel={container.image}
                  icon="container"
                  color="var(--status-warning)"
                  status="online"
                  small
                />
                <div className="flex flex-wrap gap-1 ml-2">
                  {container.ports.filter((p, i, arr) => arr.findIndex(x => x.hostPort === p.hostPort) === i).map((p, i) => (
                    <div key={i} className="flex items-center gap-0.5">
                      <PortBadge port={p.hostPort} type="docker" label="host" />
                      <Arrow />
                      <PortBadge port={p.containerPort} type="docker" label="container" />
                    </div>
                  ))}
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      {managedContainers.length === 0 && unmanagedContainers.length === 0 && (
        <div className="text-xs font-mono" style={{ color: 'var(--text-secondary)' }}>
          No containers with exposed ports. Create a site from the dashboard to see its traffic flow.
        </div>
      )}

      {/* Legend */}
      <div className="mt-4 pt-4 border-t border-white/10">
        <div className="flex flex-wrap gap-4 text-[10px]" style={{ color: 'var(--text-secondary)' }}>
          <span className="flex items-center gap-1">
            <ShieldCheck size={12} weight="bold" style={{ color: 'var(--neon-green)' }} />
            nginx port
          </span>
          <span className="flex items-center gap-1">
            <Anchor size={12} weight="bold" style={{ color: 'var(--neon-cyan)' }} />
            docker port
          </span>
          <span className="flex items-center gap-1">{statusDot('online')} online</span>
          <span className="flex items-center gap-1">{statusDot('offline')} offline</span>
          <span className="flex items-center gap-1">{statusDot('unknown')} unknown</span>
        </div>
      </div>
    </div>
  );
}
