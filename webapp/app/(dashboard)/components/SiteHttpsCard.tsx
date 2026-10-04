'use client';

import { useEffect, useState } from 'react';
import { Lock, LockOpen, WarningCircle } from '@phosphor-icons/react';

interface CertInfo {
  domain: string;
  domains?: string[];
  hasSSL: boolean;
  expiryDate?: string | null;
  daysUntilExpiry?: number | null;
  status: string;
}

interface CloudflareInfo {
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
              if (!cancelled) setCloudflare({ zoneDomain: zone.domain, ssl: ssl.ssl, alwaysUseHttps: ssl.alwaysUseHttps });
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
              <div className="mt-1">Change it under Network → DNS → Domains → SSL settings.</div>
            </div>
          )}
        </>
      )}
    </div>
  );
}
