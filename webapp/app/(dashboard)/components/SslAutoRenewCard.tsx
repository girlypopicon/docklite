'use client';

import { useEffect, useState } from 'react';
import { ArrowsClockwise } from '@phosphor-icons/react';
import { useToast } from '@/lib/hooks/useToast';

/** Settings → System: automatic renewal of HTTPS certificates for all domains or none. "All" makes each site's own
 *  switch (on its HTTPS card) available; a domain switched off there stays off and doesn't affect the others. */
export default function SslAutoRenewCard() {
  const [all, setAll] = useState<boolean | null>(null);
  const [busy, setBusy] = useState(false);
  const toast = useToast();

  useEffect(() => {
    fetch('/api/ssl/autorenew')
      .then((r) => (r.ok ? r.json() : null))
      .then((d) => d && setAll(!!d.all))
      .catch(() => {});
  }, []);

  const choose = async (next: boolean) => {
    setBusy(true);
    try {
      const res = await fetch('/api/ssl/autorenew', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ enabled: next }),
      });
      if (!res.ok) throw new Error((await res.json().catch(() => ({}))).error || 'Could not change automatic renewal');
      setAll(next);
      toast.success(next ? 'Automatic renewal is on. Each domain has its own switch on its HTTPS card.' : 'Automatic renewal is off for every domain.');
    } catch (err: any) {
      toast.error(err.message || 'Could not change automatic renewal');
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="card-vapor p-6 rounded-xl">
      <h2 className="text-2xl font-bold neon-text mb-2 flex items-center gap-2" style={{ color: 'var(--text-green)' }}>
        <ArrowsClockwise size={20} weight="regular" />
        HTTPS certificate renewal
      </h2>
      <p className="text-sm mb-4" style={{ color: 'var(--text-secondary)' }}>
        Once a day DockLite renews every certificate that has less than 30 days left. If a renewal fails, the site’s HTTPS card and the
        terminal app’s “Needs attention” list say so.
      </p>
      <div className="flex gap-2" role="radiogroup" aria-label="Automatic renewal">
        {[
          { v: true, label: 'All domains' },
          { v: false, label: 'None' },
        ].map((o) => (
          <button
            key={String(o.v)}
            type="button"
            role="radio"
            aria-checked={all === o.v}
            disabled={busy || all === null}
            onClick={() => all !== o.v && choose(o.v)}
            className="px-4 py-2 rounded-lg font-bold transition-all"
            style={all === o.v ? { background: 'var(--neon-green)', color: 'var(--bg-darker)' } : { border: '1px solid rgba(var(--neon-purple-rgb), 0.4)', color: 'var(--text-primary)' }}
          >
            {o.label}
          </button>
        ))}
      </div>
      <p className="text-xs mt-3" style={{ color: 'var(--text-secondary)' }}>
        With “All domains”, you can still switch one domain off on its own card; the others keep renewing.
      </p>
    </div>
  );
}
