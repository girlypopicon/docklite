'use client';

import { useState } from 'react';
import { ArrowSquareOut, CheckCircle, XCircle, WarningCircle } from '@phosphor-icons/react';

interface TokenCheck {
  valid: boolean;
  canListZones: boolean;
  zoneCount: number;
  canReadDns: boolean;
  canReadSettings: boolean;
  error?: string;
}

interface CloudflareConfig {
  enabled: boolean;
  hasToken: boolean;
}

const TOKEN_PAGE = 'https://dash.cloudflare.com/profile/api-tokens';

// The permissions DockLite needs, in the words Cloudflare's token form uses.
const PERMISSIONS = [
  { scope: 'Zone', item: 'Zone', level: 'Read', why: 'see your domains so DockLite can import them' },
  { scope: 'Zone', item: 'DNS', level: 'Edit', why: 'read and change DNS records (and prove domain ownership for certificates)' },
  { scope: 'Zone', item: 'Zone Settings', level: 'Edit', why: 'show and change each domain’s SSL mode and HTTPS redirect' },
];

function CheckRow({ ok, label }: { ok: boolean; label: string }) {
  return (
    <li className="flex items-center gap-2 text-sm">
      {ok ? (
        <CheckCircle size={16} weight="fill" style={{ color: 'var(--neon-green)' }} />
      ) : (
        <XCircle size={16} weight="fill" style={{ color: 'var(--status-error)' }} />
      )}
      <span className={ok ? 'text-gray-300' : 'text-gray-400'}>{label}</span>
    </li>
  );
}

export default function CloudflareSetup({ config, onSaved }: { config: CloudflareConfig | null; onSaved: () => void }) {
  const [apiToken, setApiToken] = useState('');
  const [check, setCheck] = useState<TokenCheck | null>(null);
  const [busy, setBusy] = useState<'check' | 'save' | 'toggle' | null>(null);
  const [message, setMessage] = useState<{ kind: 'ok' | 'error'; text: string } | null>(null);

  const runCheck = async () => {
    setBusy('check');
    setMessage(null);
    try {
      const res = await fetch('/api/dns/cloudflare/check', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ api_token: apiToken.trim() }),
      });
      const data = await res.json().catch(() => ({}));
      if (!res.ok) throw new Error(data.error || 'Check failed');
      setCheck(data);
    } catch (err: any) {
      setCheck(null);
      setMessage({ kind: 'error', text: err.message });
    } finally {
      setBusy(null);
    }
  };

  const save = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!apiToken.trim()) return;
    setBusy('save');
    setMessage(null);
    try {
      const res = await fetch('/api/dns/config', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ api_token: apiToken.trim(), enabled: true }),
      });
      const data = await res.json().catch(() => ({}));
      if (!res.ok) throw new Error(data.error || 'Saving failed');
      setApiToken('');
      setMessage({ kind: 'ok', text: 'Token saved. Next: go to the Zones tab and click “Import from Cloudflare”.' });
      onSaved();
    } catch (err: any) {
      setMessage({ kind: 'error', text: err.message });
    } finally {
      setBusy(null);
    }
  };

  const toggleEnabled = async () => {
    setBusy('toggle');
    try {
      await fetch('/api/dns/config', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ enabled: !config?.enabled }),
      });
      onSaved();
    } finally {
      setBusy(null);
    }
  };

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-xl font-bold text-neon-cyan mb-1">Connect Cloudflare</h2>
        <p className="text-sm text-gray-400">
          DockLite uses a Cloudflare <span className="text-gray-300">API token</span> — a password that only allows
          the specific things you tick. It takes about two minutes.
        </p>
      </div>

      <div className="bg-dark-bg/50 p-4 rounded-lg flex flex-wrap items-center justify-between gap-3">
        <div className="text-sm text-gray-400">
          Status:{' '}
          {config?.hasToken ? (
            config.enabled ? (
              <span className="text-neon-green">✓ Connected</span>
            ) : (
              <span className="text-gray-500">Token saved, integration turned off</span>
            )
          ) : (
            <span className="text-status-warning inline-flex items-center gap-1">
              <WarningCircle size={14} weight="duotone" /> Not connected yet
            </span>
          )}
        </div>
        {config?.hasToken && (
          <div className="flex gap-2">
            <button type="button" className="cyber-button-sm" onClick={runCheck} disabled={busy !== null}>
              {busy === 'check' ? 'Checking…' : 'Check saved token'}
            </button>
            <button type="button" className="cyber-button-sm" onClick={toggleEnabled} disabled={busy !== null}>
              {config.enabled ? 'Turn off' : 'Turn on'}
            </button>
          </div>
        )}
      </div>

      <ol className="space-y-4 text-sm text-gray-300 list-decimal list-inside">
        <li>
          Open{' '}
          <a href={TOKEN_PAGE} target="_blank" rel="noopener noreferrer" className="text-neon-pink hover:underline inline-flex items-center gap-1">
            Cloudflare → My Profile → API Tokens <ArrowSquareOut size={12} />
          </a>{' '}
          and click <b>Create Token</b>.
        </li>
        <li>
          Next to <b>Edit zone DNS</b>, click <b>Use template</b>.
        </li>
        <li>
          Under <b>Permissions</b>, make sure these three rows exist (the template gives you the DNS one; use{' '}
          <b>+ Add more</b> for the others):
          <div className="mt-2 overflow-x-auto">
            <table className="text-xs w-full">
              <tbody>
                {PERMISSIONS.map((p) => (
                  <tr key={p.item} className="border-t border-neon-purple/20">
                    <td className="py-1.5 pr-3 font-mono whitespace-nowrap text-neon-cyan">
                      {p.scope} → {p.item} → {p.level}
                    </td>
                    <td className="py-1.5 text-gray-400">so DockLite can {p.why}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </li>
        <li>
          Under <b>Zone Resources</b>, choose <b>Include → All zones</b> (or pick just the domains DockLite should manage).
        </li>
        <li>
          Click <b>Continue to summary</b>, then <b>Create Token</b>. Copy the token right away — Cloudflare only shows it once.
        </li>
        <li>Paste it below, click <b>Check token</b> to see what it can do, then <b>Save</b>.</li>
      </ol>

      <form onSubmit={save} className="space-y-3">
        <label className="block text-sm font-bold text-neon-cyan">Cloudflare API token</label>
        <input
          type="password"
          value={apiToken}
          onChange={(e) => {
            setApiToken(e.target.value);
            setCheck(null);
          }}
          placeholder={config?.hasToken ? 'Paste a new token to replace the saved one' : 'Paste your token here'}
          className="input-vapor w-full font-mono"
          autoComplete="off"
          disabled={busy !== null}
        />
        <p className="text-xs text-gray-500">Stored on this server only; DockLite never shows it again.</p>
        <div className="flex flex-wrap gap-2">
          <button type="button" className="cyber-button-sm" onClick={runCheck} disabled={busy !== null || !apiToken.trim()}>
            {busy === 'check' ? 'Checking…' : 'Check token'}
          </button>
          <button type="submit" className="cyber-button" disabled={busy !== null || !apiToken.trim()}>
            {busy === 'save' ? 'Saving…' : 'Save'}
          </button>
        </div>
      </form>

      {check && (
        <div className="bg-dark-bg/50 p-4 rounded-lg space-y-2">
          <div className="text-sm font-bold text-neon-cyan">What this token can do</div>
          <ul className="space-y-1">
            <CheckRow ok={check.valid} label="Cloudflare accepts the token" />
            <CheckRow
              ok={check.canListZones}
              label={check.canListZones ? `See your domains (${check.zoneCount} found)` : 'See your domains — needs Zone → Zone → Read'}
            />
            <CheckRow ok={check.canReadDns} label={check.canReadDns ? 'Read DNS records' : 'Read DNS records — needs Zone → DNS → Edit'} />
            <CheckRow
              ok={check.canReadSettings}
              label={check.canReadSettings ? 'Read SSL settings' : 'Read SSL settings — needs Zone → Zone Settings → Edit'}
            />
          </ul>
          {check.error && <p className="text-xs text-status-warning">{check.error}</p>}
          <p className="text-xs text-gray-500">
            “Edit” permissions can’t be tested without changing something, so DockLite only checks reading here.
          </p>
        </div>
      )}

      {message && (
        <p className={`text-sm ${message.kind === 'ok' ? 'text-neon-green' : 'text-status-error'}`}>{message.text}</p>
      )}
    </div>
  );
}
