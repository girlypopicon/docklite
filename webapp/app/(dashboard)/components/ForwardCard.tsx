'use client';

import { useCallback, useEffect, useState } from 'react';
import { ArrowBendUpRight } from '@phosphor-icons/react';

interface Forward {
  domain: string;
  mode: 'redirect' | 'proxy';
}

const MODES = [
  { id: 'redirect', label: 'Redirect', hint: 'Visitors are sent to this site’s address (the address bar changes).' },
  { id: 'proxy', label: 'Proxy', hint: 'Visitors keep the new address and see this site’s pages.' },
] as const;

/** Forwarded domains: send another domain's visitors to this website. The new domain is set up like any new site (its own
 *  container, DNS and HTTPS certificate) and joins this site's linked group. Adding another is the same steps again. */
export default function ForwardCard({ containerId, domain, onForwarded }: { containerId: string; domain: string; onForwarded: (message: string) => void }) {
  const [forwards, setForwards] = useState<Forward[]>([]);
  const [newDomain, setNewDomain] = useState('');
  const [mode, setMode] = useState<'redirect' | 'proxy'>('redirect');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');

  const load = useCallback(async () => {
    try {
      const res = await fetch(`/api/containers/${containerId}/forward`);
      if (res.ok) setForwards((await res.json()).forwards || []);
    } catch {
      /* the list is optional */
    }
  }, [containerId]);

  useEffect(() => {
    void load();
  }, [load]);

  const submit = async () => {
    setBusy(true);
    setError('');
    try {
      const res = await fetch(`/api/containers/${containerId}/forward`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ domain: newDomain.trim(), mode }),
      });
      const data = await res.json().catch(() => ({}));
      if (!res.ok) throw new Error(data.error || 'Could not forward the domain');
      const dnsDone = data.dns && (data.dns.status === 'done' || data.dns.status === 'conflict');
      const dnsNote = dnsDone
        ? data.dns.status === 'conflict'
          ? ' Cloudflare already has a record for it pointing elsewhere, so I left it alone; fix it in the DNS tab.'
          : ' Its DNS in Cloudflare is set.'
        : `${data.dns?.message ? ' ' + data.dns.message : ''} Point its DNS at this server.`;
      onForwarded(
        `${data.domain} now ${data.mode}s to ${domain}.${data.group ? ` Both are in the linked group “${data.group}”.` : ''}${dnsNote}${data.warning ? ' ' + data.warning : ''}`,
      );
      setNewDomain('');
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Could not forward the domain');
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="card-vapor p-6 rounded-xl">
      <h3 className="text-xl font-bold neon-text mb-1 flex items-center gap-2" style={{ color: 'var(--text-pink)' }}>
        <ArrowBendUpRight size={18} weight="regular" />
        Forwarded domains
      </h3>
      <p className="text-sm mb-4" style={{ color: 'var(--text-secondary)' }}>
        Send another domain’s visitors to {domain}. The new domain gets its own container and HTTPS certificate, and both go in one linked group.
      </p>
      {forwards.length > 0 && (
        <ul className="mb-4 space-y-1 text-sm font-mono" aria-label="Domains forwarded here">
          {forwards.map((f) => (
            <li key={f.domain} style={{ color: 'var(--text-primary)' }}>
              {f.domain} <span style={{ color: 'var(--text-secondary)' }}>({f.mode})</span>
            </li>
          ))}
        </ul>
      )}
      <div className="space-y-3">
        <input
          value={newDomain}
          onChange={(e) => setNewDomain(e.target.value)}
          placeholder="new-name.com"
          aria-label="Domain to forward here"
          className="w-full px-3 py-2 rounded-lg font-mono text-sm"
          style={{ background: 'var(--bg-darker)', color: 'var(--text-primary)', border: '1px solid rgba(var(--neon-purple-rgb), 0.4)' }}
        />
        <div className="space-y-2" role="radiogroup" aria-label="How to forward">
          {MODES.map((m) => (
            <label key={m.id} className="flex items-start gap-3 cursor-pointer">
              <input type="radio" name={`forward-mode-${containerId}`} checked={mode === m.id} onChange={() => setMode(m.id)} className="mt-1" />
              <span>
                <span className="font-bold" style={{ color: 'var(--text-primary)' }}>{m.label}</span>
                <span className="block text-xs" style={{ color: 'var(--text-secondary)' }}>{m.hint}</span>
              </span>
            </label>
          ))}
        </div>
        {error && <p className="text-sm" style={{ color: 'var(--status-error)' }}>{error}</p>}
        <button type="button" disabled={busy || !newDomain.trim()} onClick={submit} className="btn-neon px-4 py-2 font-bold">
          {busy ? 'Setting up…' : 'Forward this domain here'}
        </button>
      </div>
    </div>
  );
}
