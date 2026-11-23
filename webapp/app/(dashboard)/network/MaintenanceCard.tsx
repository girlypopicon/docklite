'use client';

import { useCallback, useEffect, useState } from 'react';
import { Hourglass } from '@phosphor-icons/react';
import { useToast } from '@/lib/hooks/useToast';

// "Back soon" mode: visitors see a branded page (HTTP 503) instead of the site. Instant, nothing reloads.
export default function MaintenanceCard({ domains }: { domains: string[] }) {
  const toast = useToast();
  const [active, setActive] = useState<string[]>([]);
  const [message, setMessage] = useState('');
  const [defaultMessage, setDefaultMessage] = useState('');
  const [target, setTarget] = useState('all');
  const [busy, setBusy] = useState(false);
  const [allowed, setAllowed] = useState(true);

  const load = useCallback(async () => {
    try {
      const res = await fetch('/api/nginx/maintenance', { cache: 'no-store' });
      if (!res.ok) {
        setAllowed(false);
        return;
      }
      const data = await res.json();
      setActive(data.active || []);
      setMessage(data.message || '');
      setDefaultMessage(data.defaultMessage || '');
    } catch {
      setAllowed(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const send = async (domain: string, on: boolean) => {
    if (on && !window.confirm(`Show the “back soon” page to visitors of ${domain === 'all' ? 'every site' : domain}? They won’t see the site until you turn it off.`)) return;
    setBusy(true);
    try {
      const res = await fetch('/api/nginx/maintenance', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ domain, on, message }),
      });
      const data = await res.json().catch(() => ({}));
      if (!res.ok) throw new Error(data.error || 'Could not change back-soon mode');
      setActive(data.active || []);
      toast.success(on ? 'Back-soon page is showing.' : 'Live again.');
    } catch (e: any) {
      toast.error(e.message || 'Could not change back-soon mode');
    } finally {
      setBusy(false);
    }
  };

  if (!allowed) return null;
  const isOn = (d: string) => active.includes(d === 'all' ? 'ALL' : d);

  return (
    <div className="card-vapor p-4 rounded-xl space-y-3">
      <div className="flex items-center gap-2" style={{ color: 'var(--text-cyan)' }}>
        <Hourglass size={18} weight="regular" />
        <span className="font-bold text-sm">“Back soon” page</span>
      </div>
      <p className="text-xs" style={{ color: 'var(--text-secondary)' }}>
        {active.length === 0
          ? 'Off: every site is live. Turn it on while you work on a site, and visitors see a friendly page instead of errors.'
          : `On for: ${active.map((a) => (a === 'ALL' ? 'every site' : a)).join(', ')}.`}
      </p>
      <textarea
        value={message}
        onChange={(e) => setMessage(e.target.value)}
        maxLength={300}
        rows={2}
        placeholder={defaultMessage}
        aria-label="What visitors read"
        className="w-full px-3 py-2 rounded-lg text-sm"
        style={{ background: 'var(--bg-darker)', color: 'var(--text-primary)', border: '1px solid rgba(var(--neon-purple-rgb), 0.4)' }}
      />
      <div className="flex flex-wrap items-center gap-2">
        <select
          value={target}
          onChange={(e) => setTarget(e.target.value)}
          aria-label="Which site"
          className="px-3 py-2 rounded-lg text-sm"
          style={{ background: 'var(--bg-darker)', color: 'var(--text-primary)', border: '1px solid rgba(var(--neon-purple-rgb), 0.4)' }}
        >
          <option value="all">Every site</option>
          {domains.map((d) => (
            <option key={d} value={d}>{d}</option>
          ))}
        </select>
        {isOn(target) ? (
          <button type="button" disabled={busy} onClick={() => send(target, false)} className="btn-neon px-4 py-2 text-sm font-bold">
            {busy ? 'Working…' : 'Turn off (go live)'}
          </button>
        ) : (
          <button type="button" disabled={busy} onClick={() => send(target, true)} className="btn-neon px-4 py-2 text-sm font-bold">
            {busy ? 'Working…' : 'Turn on'}
          </button>
        )}
      </div>
    </div>
  );
}
