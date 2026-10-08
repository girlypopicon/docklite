'use client';

import { useEffect, useState } from 'react';
import { Lock, LockOpen, WarningCircle } from '@phosphor-icons/react';
import ZoneSslControls from '../network/ZoneSslControls';
import { useToast } from '@/lib/hooks/useToast';

interface CertInfo {
  domain: string;
  domains?: string[];
  hasSSL: boolean;
  expiryDate?: string | null;
  daysUntilExpiry?: number | null;
  status: string;
}

interface DnsRecordPreview {
  name: string;
  type: string;
  content: string;
  action: string;
  existing?: string;
}

interface DnsPreview {
  status: string;
  message?: string;
  records?: DnsRecordPreview[];
}

interface CloudflareInfo {
  zoneId: number;
  zoneDomain: string;
  ssl: string;
  alwaysUseHttps: boolean;
}

const CF_MODE_TEXT: Record<string, string> = {
  strict: 'Full (strict) — encrypted all the way, with a checked certificate. Best.',
  full: 'Full — encrypted all the way, but your server’s certificate isn’t checked.',
  flexible: 'Flexible — Cloudflare talks to this server unencrypted.',
  off: 'Off — no HTTPS through Cloudflare.',
};

function certMatches(cert: CertInfo, domain: string) {
  const names = [cert.domain, ...(cert.domains || [])];
  return names.includes(domain) || names.includes(`www.${domain}`);
}

/** HTTPS status for a website container: its certificate and, when the
 *  domain is on Cloudflare, Cloudflare's SSL mode. Read-only. */
export default function SiteHttpsCard({ domain }: { domain: string }) {
  const [cert, setCert] = useState<CertInfo | null>(null);
  const [cloudflare, setCloudflare] = useState<CloudflareInfo | null>(null);
  const [loading, setLoading] = useState(true);
  const [dns, setDns] = useState<DnsPreview | null>(null);
  const [dnsBusy, setDnsBusy] = useState(false);
  const [showSsl, setShowSsl] = useState(false);
  const toast = useToast();

  const loadDns = async () => {
    try {
      const res = await fetch(`/api/dns/site?domain=${encodeURIComponent(domain)}&www=1`);
      if (res.ok) setDns(await res.json());
    } catch {
      /* optional */
    }
  };

  const setUpDns = async (overwrite: boolean) => {
    setDnsBusy(true);
    try {
      const res = await fetch('/api/dns/site', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ domain, include_www: true, overwrite }),
      });
      const data = await res.json().catch(() => ({}));
      if (!res.ok) throw new Error(data.error || 'Could not set up DNS');
      if (data.status === 'done') toast.success('DNS records added in Cloudflare.');
      else toast.error(data.message || 'Cloudflare DNS was not changed.');
      await loadDns();
    } catch (err: any) {
      toast.error(err.message || 'Could not set up DNS');
    } finally {
      setDnsBusy(false);
    }
  };

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const res = await fetch('/api/ssl/status');
        if (res.ok) {
          const data = await res.json();
          const all: CertInfo[] = [...(data.sites || []), ...(data.allCerts || [])];
          const found = all.find((c) => c.hasSSL && certMatches(c, domain)) || null;
          if (!cancelled) setCert(found);
        }
      } catch {
        /* certificate status is best-effort */
      }

      try {
        const zonesRes = await fetch('/api/dns/zones');
        if (zonesRes.ok) {
          const zones: { id: number; domain: string }[] = (await zonesRes.json()).zones || [];
          // The zone is the domain itself or the closest parent (blog.example.com → example.com).
          const zone = zones
            .filter((z) => domain === z.domain || domain.endsWith(`.${z.domain}`))
            .sort((a, b) => b.domain.length - a.domain.length)[0];
          if (zone) {
            const sslRes = await fetch(`/api/dns/zones/ssl?id=${zone.id}`);
            if (sslRes.ok) {
              const ssl = await sslRes.json();
              if (!cancelled) setCloudflare({ zoneId: zone.id, zoneDomain: zone.domain, ssl: ssl.ssl, alwaysUseHttps: ssl.alwaysUseHttps });
              if (!cancelled) loadDns();
            }
          }
        }
      } catch {
        /* Cloudflare is optional */
      }
      if (!cancelled) setLoading(false);
    })();
    return () => {
      cancelled = true;
    };
  }, [domain]);

  let icon = <LockOpen size={18} weight="duotone" style={{ color: 'var(--status-warning)' }} />;
  let headline = 'No certificate on this server';
  let detail = 'Visitors can only reach this site over plain http:// unless Cloudflare provides HTTPS in front of it.';
  if (cert) {
    const days = cert.daysUntilExpiry ?? null;
    const date = cert.expiryDate ? new Date(cert.expiryDate).toLocaleDateString() : null;
    if (days !== null && days < 0) {
      icon = <WarningCircle size={18} weight="duotone" style={{ color: 'var(--status-error)' }} />;
      headline = 'Certificate expired';
      detail = `It expired on ${date}. Browsers will show a security warning until it’s renewed.`;
    } else if (days !== null && days < 30) {
      icon = <WarningCircle size={18} weight="duotone" style={{ color: 'var(--status-warning)' }} />;
      headline = `Certificate expires in ${days} day${days === 1 ? '' : 's'}`;
      detail = `Valid until ${date}. Let’s Encrypt certificates normally renew themselves about 30 days before this.`;
    } else {
      icon = <Lock size={18} weight="duotone" style={{ color: 'var(--neon-green)' }} />;
      headline = 'HTTPS certificate is valid';
      detail = date ? `Valid until ${date}${days !== null ? ` (${days} days)` : ''}; it renews automatically.` : 'Certificate installed.';
    }
  }

  return (
    <div className="card-vapor p-4 rounded-xl space-y-3">
      <div className="text-sm font-bold" style={{ color: 'var(--neon-cyan)' }}>
        HTTPS for {domain}
      </div>
      {loading ? (
        <p className="text-xs" style={{ color: 'var(--text-secondary)' }}>Checking…</p>
      ) : (
        <>
          <div className="flex gap-3">
            <div className="pt-0.5">{icon}</div>
            <div>
              <div className="text-sm font-bold" style={{ color: 'var(--text-primary)' }}>{headline}</div>
              <div className="text-xs" style={{ color: 'var(--text-secondary)' }}>{detail}</div>
            </div>
          </div>
          {cloudflare && (
            <div className="text-xs border-t border-neon-purple/20 pt-3" style={{ color: 'var(--text-secondary)' }}>
              <span className="font-bold" style={{ color: 'var(--text-primary)' }}>Cloudflare ({cloudflare.zoneDomain}): </span>
              {CF_MODE_TEXT[cloudflare.ssl] || cloudflare.ssl}
              {cloudflare.alwaysUseHttps ? ' http:// visitors are sent to https://.' : ''}
              <div className="mt-2">
                <button type="button" className="underline" style={{ color: 'var(--neon-cyan)' }} onClick={() => setShowSsl((v) => !v)}>
                  {showSsl ? 'Hide SSL settings' : 'Change Cloudflare SSL settings'}
                </button>
              </div>
              {showSsl && <div className="mt-3"><ZoneSslControls zoneId={cloudflare.zoneId} /></div>}
            </div>
          )}
          {cloudflare && dns && dns.records && (
            <div className="text-xs border-t border-neon-purple/20 pt-3 space-y-2" style={{ color: 'var(--text-secondary)' }}>
              <span className="font-bold" style={{ color: 'var(--text-primary)' }}>DNS in Cloudflare: </span>
              {dns.records.every((r) => r.action === 'exists') ? (
                <span style={{ color: 'var(--neon-green)' }}>✓ this site’s records are set up.</span>
              ) : (
                <>
                  <ul className="font-mono space-y-1 mt-1">
                    {dns.records.map((r) => (
                      <li key={r.type + r.name}>
                        {r.action === 'exists' ? '✓' : r.action === 'conflict' ? '!' : '+'} {r.type} {r.name} → {r.content}
                        {r.action === 'conflict' && r.existing ? ` (currently ${r.existing})` : ''}
                      </li>
                    ))}
                  </ul>
                  <div className="flex gap-2 flex-wrap">
                    {dns.records.some((r) => r.action === 'create') && (
                      <button type="button" disabled={dnsBusy} onClick={() => setUpDns(false)} className="btn-neon px-3 py-1 text-xs font-bold">
                        {dnsBusy ? 'Working…' : 'Create the missing records'}
                      </button>
                    )}
                    {dns.records.some((r) => r.action === 'conflict') && (
                      <button
                        type="button"
                        disabled={dnsBusy}
                        onClick={() => { if (window.confirm('Change the existing record so it points at this server? Visitors will go to this server after it spreads.')) setUpDns(true); }}
                        className="btn-neon px-3 py-1 text-xs font-bold"
                      >
                        Point it at this server
                      </button>
                    )}
                  </div>
                </>
              )}
            </div>
          )}
        </>
      )}
    </div>
  );
}
