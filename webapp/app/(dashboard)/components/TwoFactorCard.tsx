'use client';

import { useCallback, useEffect, useState } from 'react';
import { ShieldCheck, Copy, DownloadSimple, WarningCircle } from '@phosphor-icons/react';

// Settings → Security: two-factor login with an authenticator app. Off → scan a QR code → type a code → on, with ten
// recovery codes shown once. Turning it off, or making new recovery codes, asks for the password and a code.

type Status = { enabled: boolean; recovery_codes_left: number; enabled_at?: string };
type Setup = { secret: string; otpauth_url: string; qr_png: string };

const box = 'px-4 py-3 rounded-lg text-sm';
const errorStyle = { background: 'rgba(var(--status-error-rgb), 0.15)', border: '1px solid rgba(var(--status-error-rgb), 0.5)', color: 'var(--status-error)' };

export default function TwoFactorCard() {
  const [status, setStatus] = useState<Status | null>(null);
  const [setup, setSetup] = useState<Setup | null>(null);
  const [codes, setCodes] = useState<string[] | null>(null);
  const [confirming, setConfirming] = useState<'disable' | 'recovery' | null>(null);
  const [code, setCode] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);

  const load = useCallback(async () => {
    try {
      const res = await fetch('/api/auth/2fa', { cache: 'no-store' });
      if (res.ok) setStatus(await res.json());
    } catch {
      /* the card just stays in its loading state */
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  const post = async (path: string, body: object) => {
    setBusy(true);
    setError('');
    try {
      const res = await fetch(path, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) });
      const data = await res.json().catch(() => ({}));
      if (!res.ok) throw new Error(data.error || 'Something went wrong');
      return data;
    } catch (err: any) {
      setError(err.message);
      return null;
    } finally {
      setBusy(false);
    }
  };

  const start = async () => {
    const data = await post('/api/auth/2fa/setup', {});
    if (data) {
      setSetup(data);
      setCode('');
    }
  };

  const enable = async (e: React.FormEvent) => {
    e.preventDefault();
    const data = await post('/api/auth/2fa/enable', { code });
    if (data) {
      setSetup(null);
      setCode('');
      setCodes(data.recovery_codes);
      load();
    }
  };

  const confirm = async (e: React.FormEvent) => {
    e.preventDefault();
    const path = confirming === 'disable' ? '/api/auth/2fa/disable' : '/api/auth/2fa/recovery-codes';
    const data = await post(path, { password, code });
    if (data) {
      if (confirming === 'recovery') setCodes(data.recovery_codes);
      setConfirming(null);
      setPassword('');
      setCode('');
      load();
    }
  };

  const codesText = (codes || []).join('\n');

  return (
    <div className="cyber-card p-4 sm:p-6 space-y-4">
      <div>
        <h3 className="text-xl font-bold flex items-center gap-2" style={{ color: 'var(--text-cyan)' }}>
          <ShieldCheck size={22} weight="duotone" />
          Two-factor login
          {status?.enabled && (
            <span className="ml-1 px-2 py-0.5 rounded-full text-xs" style={{ color: 'var(--text-green)', border: '1px solid var(--neon-green)' }}>
              On
            </span>
          )}
        </h3>
        <p className="text-sm mt-1" style={{ color: 'var(--text-secondary)' }}>
          After your password, DockLite asks for a six-digit code from an authenticator app on your phone (Google Authenticator,
          1Password, Aegis, Authy…). Someone who learns your password still can&apos;t sign in.
        </p>
      </div>

      {error && <div className={box} style={errorStyle}>{error}</div>}

      {codes && (
        <div className="space-y-3 p-4 rounded-lg" style={{ border: '1px solid var(--neon-yellow)', background: 'rgba(var(--status-warning-rgb), 0.08)' }}>
          <p className="text-sm font-bold flex items-center gap-2" style={{ color: 'var(--text-yellow)' }}>
            <WarningCircle size={18} weight="duotone" />
            Save these recovery codes now. They are shown only once.
          </p>
          <p className="text-xs" style={{ color: 'var(--text-secondary)' }}>
            Each one gets you in once if you lose your phone. Keep them somewhere safe that isn&apos;t the phone (a password manager, or on paper).
          </p>
          <div className="grid grid-cols-2 gap-x-6 gap-y-1 font-mono text-sm" style={{ color: 'var(--text-primary)' }}>
            {codes.map((c) => (
              <span key={c}>{c}</span>
            ))}
          </div>
          <div className="flex flex-wrap gap-2">
            <button type="button" className="cyber-button-sm inline-flex items-center gap-1" onClick={() => navigator.clipboard?.writeText(codesText)}>
              <Copy size={14} /> Copy
            </button>
            <a
              className="cyber-button-sm inline-flex items-center gap-1"
              href={'data:text/plain;charset=utf-8,' + encodeURIComponent('DockLite recovery codes\n\n' + codesText + '\n')}
              download="docklite-recovery-codes.txt"
            >
              <DownloadSimple size={14} /> Download
            </a>
            <button type="button" className="cyber-button-sm" onClick={() => setCodes(null)}>
              I&apos;ve saved them
            </button>
          </div>
        </div>
      )}

      {!status && <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>Loading…</p>}

      {status && !status.enabled && !setup && (
        <button type="button" className="cyber-button" onClick={start} disabled={busy}>
          Turn on two-factor login
        </button>
      )}

      {setup && (
        <form onSubmit={enable} className="space-y-4">
          <ol className="text-sm space-y-1 list-decimal pl-5" style={{ color: 'var(--text-secondary)' }}>
            <li>Open your authenticator app and add an account (usually a + button).</li>
            <li>Scan this code. Can&apos;t scan? Choose &quot;enter a key&quot; and type the key below it.</li>
            <li>Type the six-digit code the app shows.</li>
          </ol>
          <div className="flex flex-col sm:flex-row items-start gap-4">
            {/* eslint-disable-next-line @next/next/no-img-element */}
            <img src={setup.qr_png} alt="QR code for your authenticator app" width={180} height={180} className="rounded-lg bg-white p-2" />
            <div className="min-w-0 space-y-1">
              <div className="text-xs" style={{ color: 'var(--text-secondary)' }}>Key</div>
              <code className="block font-mono text-sm break-all" style={{ color: 'var(--text-primary)' }}>
                {setup.secret.replace(/(.{4})/g, '$1 ').trim()}
              </code>
            </div>
          </div>
          <div className="flex flex-wrap items-end gap-3">
            <label className="block">
              <span className="block text-xs mb-1" style={{ color: 'var(--text-secondary)' }}>Code from the app</span>
              <input
                value={code}
                onChange={(e) => setCode(e.target.value)}
                inputMode="numeric"
                autoComplete="one-time-code"
                required
                placeholder="123456"
                className="input-vapor w-40 font-mono text-center text-lg tracking-widest"
              />
            </label>
            <button type="submit" className="cyber-button" disabled={busy}>
              Turn on
            </button>
            <button type="button" className="cyber-button-sm" onClick={() => setSetup(null)} disabled={busy}>
              Cancel
            </button>
          </div>
        </form>
      )}

      {status?.enabled && !confirming && (
        <div className="space-y-3">
          <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>
            {status.recovery_codes_left} recovery {status.recovery_codes_left === 1 ? 'code' : 'codes'} left.
            {status.recovery_codes_left <= 3 && ' Make new ones soon.'}
          </p>
          <div className="flex flex-wrap gap-2">
            <button type="button" className="cyber-button-sm" onClick={() => setConfirming('recovery')}>
              New recovery codes
            </button>
            <button
              type="button"
              className="px-3 py-1.5 rounded-lg text-sm font-bold"
              style={{ color: 'var(--status-error)', border: '1px solid var(--status-error)' }}
              onClick={() => setConfirming('disable')}
            >
              Turn off
            </button>
          </div>
        </div>
      )}

      {confirming && (
        <form onSubmit={confirm} className="space-y-3">
          <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>
            {confirming === 'disable'
              ? 'To turn it off, confirm with your password and a code.'
              : 'New codes replace the old ones (the old ones stop working). Confirm with your password and a code.'}
          </p>
          <div className="flex flex-wrap items-end gap-3">
            <label className="block">
              <span className="block text-xs mb-1" style={{ color: 'var(--text-secondary)' }}>Password</span>
              <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} required className="input-vapor w-56" />
            </label>
            <label className="block">
              <span className="block text-xs mb-1" style={{ color: 'var(--text-secondary)' }}>Code (or a recovery code)</span>
              <input
                value={code}
                onChange={(e) => setCode(e.target.value)}
                autoComplete="one-time-code"
                required
                className="input-vapor w-44 font-mono text-center"
              />
            </label>
            <button type="submit" className="cyber-button" disabled={busy}>
              {confirming === 'disable' ? 'Turn off' : 'Make new codes'}
            </button>
            <button type="button" className="cyber-button-sm" onClick={() => setConfirming(null)} disabled={busy}>
              Cancel
            </button>
          </div>
        </form>
      )}
    </div>
  );
}
