'use client';

import { useEffect, useState } from 'react';
import { formatBytes } from '@/lib/database-engines';

export type InfoKind = 'server' | 'network' | 'databases' | 'ssl';

const TITLES: Record<InfoKind, string> = { server: 'Server', network: 'Network', databases: 'Databases', ssl: 'HTTPS certificates' };

async function get(serverId: number, path: string) {
  const res = await fetch(`/api/servers/${serverId}/proxy${path}`);
  if (!res.ok) throw new Error((await res.json().catch(() => ({}))).error || `Could not read ${path}`);
  return res.json();
}

const Row = ({ k, v }: { k: string; v: React.ReactNode }) => (
  <div className="flex justify-between gap-4 py-1 text-sm border-b border-neon-purple/10">
    <span style={{ color: 'var(--text-secondary)' }}>{k}</span>
    <span className="font-mono text-right break-all" style={{ color: 'var(--text-primary)' }}>{v}</span>
  </div>
);

function uptime(sec: number) {
  const d = Math.floor(sec / 86400), h = Math.floor((sec % 86400) / 3600), m = Math.floor((sec % 3600) / 60);
  return d > 0 ? `${d} days, ${h} hours` : h > 0 ? `${h} hours, ${m} minutes` : `${m} minutes`;
}

/** A read-only window onto one view of another server: its system, network, databases or certificates. */
export default function RemoteInfoModal({ serverId, serverName, kind, onClose }: { serverId: number; serverName: string; kind: InfoKind; onClose: () => void }) {
  const [data, setData] = useState<any>(null);
  const [error, setError] = useState('');

  useEffect(() => {
    let off = false;
    (async () => {
      try {
        if (kind === 'server') {
          const [ov, sv] = await Promise.all([get(serverId, '/server/overview').catch(() => null), get(serverId, '/summary')]);
          if (!off) setData({ ov, sv });
        } else if (kind === 'network') {
          const [ov, fw, ing] = await Promise.all([get(serverId, '/network/overview'), get(serverId, '/network/firewall').catch(() => null), get(serverId, '/network/ingress').catch(() => null)]);
          if (!off) setData({ ov, fw, ing });
        } else if (kind === 'databases') {
          const d = await get(serverId, '/databases/stats');
          if (!off) setData(d);
        } else {
          const d = await get(serverId, '/ssl/status');
          if (!off) setData(d);
        }
      } catch (e) {
        if (!off) setError(e instanceof Error ? e.message : 'Could not read the server');
      }
    })();
    return () => {
      off = true;
    };
  }, [serverId, kind]);

  let body: React.ReactNode = <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>Loading…</p>;
  if (error) body = <p className="text-sm" style={{ color: 'var(--status-error)' }}>{error}</p>;
  else if (data && kind === 'server') {
    const { ov, sv } = data;
    body = (
      <div>
        {ov && (
          <>
            <Row k="Server name" v={ov.hostname} />
            <Row k="System" v={`${ov.os} ${ov.osVersion || ''}`} />
            <Row k="Processors" v={ov.cpuCount} />
            <Row k="Running for" v={uptime(ov.uptime)} />
            <Row k="Load" v={ov.loadAvg ? `${ov.loadAvg.One?.toFixed?.(2) ?? ov.loadAvg.one} ${ov.loadAvg.Five?.toFixed?.(2) ?? ov.loadAvg.five}` : '-'} />
            {ov.memory && <Row k="Memory used" v={`${formatBytes(ov.memory.Total - ov.memory.Free)} of ${formatBytes(ov.memory.Total)}`} />}
            {ov.disk && <Row k="Disk used" v={`${formatBytes(ov.disk.Used)} of ${formatBytes(ov.disk.Total)}`} />}
          </>
        )}
        <Row k="Containers running" v={sv.containersRunning ?? sv.ContainersRunning ?? '-'} />
        <Row k="Containers stopped" v={sv.containersStopped ?? sv.ContainersStopped ?? '-'} />
      </div>
    );
  } else if (data && kind === 'network') {
    const { ov, fw, ing } = data;
    body = (
      <div className="space-y-4">
        <div>
          <Row k="Address" v={ov.primaryIPv4 || '-'} />
          <Row k="Gateway" v={ov.defaultGateway || '-'} />
          <Row k="Web proxy" v={ing ? `${ing.provider || 'nginx'} ${ing.state || ing.status || ''}` : '-'} />
          <Row k="Firewall" v={fw ? `${fw.firewall?.status || 'unknown'} (${fw.firewall?.provider || 'none'})` : '-'} />
        </div>
        {fw?.openPorts?.length > 0 && (
          <div>
            <div className="text-sm font-bold mb-1" style={{ color: 'var(--text-cyan)' }}>Open ports</div>
            {fw.openPorts.slice(0, 20).map((p: any) => (
              <Row key={`${p.proto}${p.port}`} k={`${p.port}/${p.proto}`} v={`${p.process || ''}${p.public ? ' · public' : ''}`} />
            ))}
          </div>
        )}
      </div>
    );
  } else if (data && kind === 'databases') {
    const dbs: any[] = data.databases || [];
    body = dbs.length === 0 ? <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>No databases on this server.</p> : (
      <div>
        {dbs.map((d) => (
          <Row key={`${d.type}${d.name}`} k={`${d.name} (${d.type})`} v={`${d.used_by ? 'used by ' + d.used_by + ' · ' : ''}${typeof d.size === 'number' ? formatBytes(d.size) : ''}`} />
        ))}
      </div>
    );
  } else if (data && kind === 'ssl') {
    const certs: any[] = data.allCerts || [];
    body = certs.length === 0 ? <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>No certificates on this server.</p> : (
      <div>
        {certs.map((c) => {
          const failed = c.renewal && !c.renewal.ok;
          return (
            <Row
              key={c.domain}
              k={c.domain}
              v={
                <span style={failed ? { color: 'var(--status-error)' } : undefined}>
                  {c.daysUntilExpiry != null ? `${c.daysUntilExpiry} days left` : c.status}
                  {failed ? ' · renewal failed' : c.autoRenew ? ' · auto-renews' : ' · auto-renew off'}
                </span>
              }
            />
          );
        })}
      </div>
    );
  }

  return (
    <div className="fixed inset-0 bg-black/80 flex items-center justify-center z-[10000] p-4" onClick={onClose}>
      <div className="cyber-card max-w-xl w-full max-h-[85vh] overflow-y-auto p-6" role="dialog" aria-label={`${TITLES[kind]} on ${serverName}`} onClick={(e) => e.stopPropagation()}>
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-2xl font-bold neon-text-pink">{TITLES[kind]} · {serverName}</h2>
          <button onClick={onClose} className="transition-colors" style={{ color: 'var(--text-secondary)' }} aria-label="Close">✕</button>
        </div>
        {body}
      </div>
    </div>
  );
}
