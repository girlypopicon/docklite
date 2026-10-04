'use client';

import { useEffect, useState } from 'react';

type SslMode = 'off' | 'flexible' | 'full' | 'strict';

// What each Cloudflare SSL mode means for visitors and for the server.
const MODES: { value: SslMode; label: string; help: string; recommended?: boolean; warn?: boolean }[] = [
  {
    value: 'strict',
    label: 'Full (strict)',
    help: 'Encrypted all the way, and Cloudflare checks your server has a real certificate (Let’s Encrypt or a Cloudflare origin certificate).',
    recommended: true,
  },
  {
    value: 'full',
    label: 'Full',
    help: 'Encrypted all the way, but Cloudflare doesn’t check your server’s certificate. Fine while setting up; switch to strict once the server has a real one.',
  },
  {
    value: 'flexible',
    label: 'Flexible',
    help: 'Visitors see HTTPS, but Cloudflare talks to your server unencrypted. Only for servers with no certificate — and it can cause redirect loops if the server also redirects to HTTPS.',
    warn: true,
  },
  {
    value: 'off',
    label: 'Off',
    help: 'No HTTPS at all. Browsers will mark the site “Not secure”.',
    warn: true,
  },
];

export default function ZoneSslControls({ zoneId }: { zoneId: number }) {
  const [mode, setMode] = useState<SslMode | null>(null);
  const [alwaysHttps, setAlwaysHttps] = useState(false);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');

  useEffect(() => {
    let cancelled = false;
    (async () => {
      setLoading(true);
      setError('');
      try {
        const res = await fetch(`/api/dns/zones/ssl?id=${zoneId}`);
        const data = await res.json().catch(() => ({}));
        if (!res.ok) throw new Error(data.error || 'Could not load SSL settings');
        if (!cancelled) {
          setMode(data.ssl);
          setAlwaysHttps(Boolean(data.alwaysUseHttps));
        }
      } catch (err: any) {
        if (!cancelled) setError(err.message);
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [zoneId]);

  const update = async (change: { ssl?: SslMode; always_use_https?: boolean }) => {
    setSaving(true);
    setError('');
    try {
      const res = await fetch('/api/dns/zones/ssl', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ id: zoneId, ...change }),
      });
      const data = await res.json().catch(() => ({}));
      if (!res.ok) throw new Error(data.error || 'Could not change the setting');
      if (change.ssl) setMode(change.ssl);
      if (change.always_use_https !== undefined) setAlwaysHttps(change.always_use_https);
    } catch (err: any) {
      setError(err.message);
    } finally {
      setSaving(false);
    }
  };

  if (loading) return <p className="text-xs text-gray-500">Loading SSL settings…</p>;
  if (error && mode === null) return <p className="text-xs text-status-error">{error}</p>;

  return (
    <div className="space-y-3">
      <div className="text-xs font-bold text-neon-cyan">Cloudflare SSL mode — how traffic is encrypted</div>
      <div className="space-y-2">
        {MODES.map((m) => (
          <label
            key={m.value}
            className={`flex gap-3 p-2 rounded-lg cursor-pointer border ${
              mode === m.value ? 'border-neon-pink/60 bg-neon-purple/10' : 'border-transparent hover:bg-neon-purple/5'
            }`}
          >
            <input
              type="radio"
              name={`ssl-${zoneId}`}
              checked={mode === m.value}
              disabled={saving}
              onChange={() => update({ ssl: m.value })}
              className="mt-1"
            />
            <span>
              <span className="text-sm font-bold text-gray-200">{m.label}</span>
              {m.recommended && <span className="ml-2 text-[10px] font-bold text-neon-green">RECOMMENDED</span>}
              {m.warn && <span className="ml-2 text-[10px] font-bold text-status-warning">NOT RECOMMENDED</span>}
              <span className="block text-xs text-gray-400">{m.help}</span>
            </span>
          </label>
        ))}
      </div>

      <label className="flex gap-3 p-2 cursor-pointer">
        <input
          type="checkbox"
          checked={alwaysHttps}
          disabled={saving}
          onChange={(e) => update({ always_use_https: e.target.checked })}
          className="mt-1"
        />
        <span>
          <span className="text-sm font-bold text-gray-200">Always use HTTPS</span>
          <span className="block text-xs text-gray-400">
            Cloudflare sends anyone who types http:// to https://. Leave this off if the server gets its certificates
            from certbot over plain HTTP — the redirect can stop those certificates from renewing.
          </span>
        </span>
      </label>

      {error && <p className="text-xs text-status-error">{error}</p>}
    </div>
  );
}
