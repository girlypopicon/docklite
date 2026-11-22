'use client';

import { useState } from 'react';
import { Swap } from '@phosphor-icons/react';

const KINDS = [
  { id: 'static', label: 'Static website', hint: 'Plain HTML, CSS and JavaScript files (index.html).' },
  { id: 'php', label: 'PHP', hint: 'PHP pages (index.php).' },
  { id: 'node', label: 'Node.js app', hint: 'A JavaScript server that runs index.js. If your site is plain files, DockLite adds a small server for them.' },
] as const;

type Kind = (typeof KINDS)[number]['id'];

/** Run a website as another kind. The files, address and HTTPS stay; only the container it runs in is replaced
 *  (a few seconds offline), and the agent puts the old one back if that fails. */
export default function SiteTypeCard({
  containerId,
  current,
  onChanged,
}: {
  containerId: string;
  current: Kind;
  /** The site's container is new afterwards, so the caller refreshes and closes. */
  onChanged: (message: string) => void;
}) {
  const [choice, setChoice] = useState<Kind>(current);
  const [port, setPort] = useState('');
  const [confirming, setConfirming] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');

  const unchanged = choice === current && choice !== 'node';
  const label = KINDS.find((k) => k.id === choice)!.label;

  const apply = async () => {
    setBusy(true);
    setError('');
    try {
      const body: Record<string, unknown> = { template_type: choice };
      if (choice === 'node' && port.trim()) body.port = Number(port);
      const res = await fetch(`/api/containers/${containerId}/template`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      });
      const data = await res.json().catch(() => ({}));
      if (!res.ok) throw new Error(data.error || 'Could not change the site type');
      onChanged(`Now running as ${label}.${data.note ? ' ' + data.note : ''}${data.warning ? ' ' + data.warning : ''}`);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Could not change the site type');
      setConfirming(false);
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="card-vapor p-6 rounded-xl">
      <h3 className="text-xl font-bold neon-text mb-1 flex items-center gap-2" style={{ color: 'var(--text-pink)' }}>
        <Swap size={18} weight="regular" />
        Site type
      </h3>
      <p className="text-sm mb-4" style={{ color: 'var(--text-secondary)' }}>
        Run this site as another kind. Your files, address and HTTPS stay as they are.
      </p>
      <div className="space-y-2" role="radiogroup" aria-label="Site type">
        {KINDS.map((k) => (
          <label key={k.id} className="flex items-start gap-3 cursor-pointer">
            <input
              type="radio"
              name={`site-type-${containerId}`}
              checked={choice === k.id}
              onChange={() => {
                setChoice(k.id);
                setConfirming(false);
              }}
              className="mt-1"
            />
            <span>
              <span className="font-bold" style={{ color: 'var(--text-primary)' }}>
                {k.label}
                {k.id === current && <span className="ml-2 text-xs" style={{ color: 'var(--text-secondary)' }}>(now)</span>}
              </span>
              <span className="block text-xs" style={{ color: 'var(--text-secondary)' }}>
                {k.hint}
              </span>
            </span>
          </label>
        ))}
      </div>
      {choice === 'node' && (
        <label className="block mt-4 text-sm" style={{ color: 'var(--text-secondary)' }}>
          Port your app listens on (blank keeps the current one, or 3000)
          <input
            type="number"
            min={1}
            max={65535}
            value={port}
            onChange={(e) => {
              setPort(e.target.value);
              setConfirming(false);
            }}
            className="input-vapor block mt-1 px-3 py-2 w-40"
            placeholder="3000"
          />
        </label>
      )}
      {error && (
        <div className="mt-3 text-sm" role="alert" style={{ color: 'var(--text-pink)' }}>
          {error}
        </div>
      )}
      <div className="mt-4 flex items-center gap-3 flex-wrap">
        {!confirming ? (
          <button type="button" disabled={unchanged || busy} onClick={() => setConfirming(true)} className="btn-neon px-4 py-2 text-sm font-bold disabled:opacity-50">
            Switch to {label}
          </button>
        ) : (
          <>
            <button type="button" disabled={busy} onClick={apply} className="btn-neon px-4 py-2 text-sm font-bold">
              {busy ? 'Switching…' : `Yes, restart as ${label}`}
            </button>
            <button type="button" disabled={busy} onClick={() => setConfirming(false)} className="px-4 py-2 text-sm" style={{ color: 'var(--text-secondary)' }}>
              Cancel
            </button>
            <span className="text-xs" style={{ color: 'var(--text-secondary)' }}>
              The site is offline for a few seconds. If it can’t start, the old kind comes back.
            </span>
          </>
        )}
      </div>
    </div>
  );
}
