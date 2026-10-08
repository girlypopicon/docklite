'use client';

import DnsPanel from '../../network/DnsPanel';

/** Settings → Cloudflare: connect your Cloudflare account, pick which domains DockLite manages, and set each
 *  domain's SSL mode. (The same panel as Network → DNS, where it also shows the individual DNS records.) */
export default function CloudflareSettings() {
  return (
    <div className="space-y-4">
      <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>
        Connect Cloudflare and DockLite can create a site’s DNS records for you when you add it, and show or change each
        domain’s SSL mode. The API token is stored on this server and only used to talk to Cloudflare.
      </p>
      <DnsPanel />
    </div>
  );
}
