'use client';

import { useEffect, useState } from 'react';
import { Globe } from '@phosphor-icons/react';
import { useToast } from '@/lib/hooks/useToast';

interface Panel {
  domain: string;
  enabled: boolean;
  https: boolean;
  url: string;
  fallback_url: string;
}

/** Settings → System: the domain DockLite's own dashboard is reached at (like smoll.docklite.net). Turning it off removes
 *  the nginx entry and the dashboard is at the server's address and port again. */
export default function PanelDomainCard() {
  const [panel, setPanel] = useState<Panel | null>(null);
  const [domain, setDomain] = useState('');
  const [busy, setBusy] = useState(false);
  const toast = useToast();

  useEffect(() => {
    fetch('/api/panel-domain')
      .then((r) => (r.ok ? r.json() : null))
      .then((d: Panel | null) => {
        if (d) {
          setPanel(d);
          setDomain(d.domain);
        }
      })
      .catch(() => {});
  }, []);

  const save = async (body: Record<string, unknown>, done: string) => {
    setBusy(true);
    try {
      const res = await fetch('/api/panel-domain', { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) });
      const data = await res.json().catch(() => ({}));
      if (!res.ok) throw new Error(data.error || 'Could not change the domain');
      setPanel(data);
      setDomain(data.domain);
      toast.success(done);
    } catch (err: any) {
      toast.error(err.message || 'Could not change the domain');
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="card-vapor p-6 rounded-xl">
      <h2 className="text-2xl font-bold neon-text mb-2 flex items-center gap-2" style={{ color: 'var(--text-cyan)' }}>
        <Globe size={20} weight="regular" />
        Linked domain
      </h2>
      <p className="text-sm mb-4" style={{ color: 'var(--text-secondary)' }}>
        The address you open DockLite at, like smoll.docklite.net. Its DNS must point at this server.
      </p>
      {!panel ? (
        <p className="text-xs" style={{ color: 'var(--text-secondary)' }}>Loading…</p>
      ) : (
        <div className="space-y-3">
          <div className="text-sm" style={{ color: 'var(--text-primary)' }}>
            Status:{' '}
            <span className="font-bold">
              {panel.domain === '' ? 'no domain set' : panel.enabled ? `on${panel.https ? ' (HTTPS)' : ' (plain http; run docklite ssl issue ' + panel.domain + ' for HTTPS)'}` : 'off'}
            </span>
          </div>
          <div className="text-sm" style={{ color: 'var(--text-secondary)' }}>
            Open DockLite at <span className="font-mono" style={{ color: 'var(--text-primary)' }}>{panel.url}</span>
            {panel.enabled && (
              <>
                {' '}
                · without the domain: <span className="font-mono">{panel.fallback_url}</span>
              </>
            )}
            {!panel.enabled && <> (the server’s address and port).</>}
          </div>
          <div className="flex flex-wrap gap-2 items-center">
            <input
              value={domain}
              onChange={(e) => setDomain(e.target.value)}
              placeholder="smoll.docklite.net"
              aria-label="Domain for the dashboard"
              className="px-3 py-2 rounded-lg font-mono text-sm flex-1 min-w-[12rem]"
              style={{ background: 'var(--bg-darker)', color: 'var(--text-primary)', border: '1px solid rgba(var(--neon-purple-rgb), 0.4)' }}
            />
            <button type="button" disabled={busy || !domain.trim()} className="btn-neon px-4 py-2 font-bold" onClick={() => save({ domain: domain.trim(), enabled: true }, `DockLite is now at ${domain.trim()}`)}>
              {panel.domain && panel.domain !== domain.trim() ? 'Change and turn on' : 'Save and turn on'}
            </button>
            {panel.domain && (
              <button type="button" disabled={busy} className="px-4 py-2 font-bold rounded-lg" style={{ border: '1px solid rgba(var(--neon-purple-rgb), 0.4)', color: 'var(--text-primary)' }}
                onClick={() => save({ enabled: !panel.enabled }, panel.enabled ? 'Domain turned off. DockLite is at the server’s address and port.' : 'Domain turned on.')}>
                {panel.enabled ? 'Turn off' : 'Turn on'}
              </button>
            )}
          </div>
        </div>
      )}
    </div>
  );
}
