'use client';

import { useEffect, useState } from 'react';
import { useToast } from '@/lib/hooks/useToast';

type TemplateType = 'static' | 'php' | 'node';

interface DnsPreview {
  status: 'not-configured' | 'no-zone' | 'no-ip' | 'ready' | 'done' | 'conflict' | 'error';
  zone?: string;
  ip?: string;
  message?: string;
  records?: { name: string; type: string; content: string; proxied: boolean; action: string; existing?: string }[];
}

interface AddContainerModalProps {
  onClose: () => void;
  onCreated: () => void;
}

export default function AddContainerModal({ onClose, onCreated }: AddContainerModalProps) {
  const [domain, setDomain] = useState('');
  const [templateType, setTemplateType] = useState<TemplateType>('static');
  const [codePath, setCodePath] = useState('');
  const [port, setPort] = useState(3000);
  const [portTouched, setPortTouched] = useState(false);
  const [includeWww, setIncludeWww] = useState(true);
  const [loading, setLoading] = useState(false);
  const [cfDns, setCfDns] = useState(true);
  const [cfProxied, setCfProxied] = useState(true);
  const [dnsPreview, setDnsPreview] = useState<DnsPreview | null>(null); // null: not an admin / not checked yet
  const toast = useToast();

  // Ask what Cloudflare would do for this domain (admins only; a 403 just hides the section).
  useEffect(() => {
    const name = domain.trim().toLowerCase();
    if (!name.includes('.')) {
      setDnsPreview(null);
      return;
    }
    let cancelled = false;
    const timer = setTimeout(async () => {
      try {
        const res = await fetch(`/api/dns/site?domain=${encodeURIComponent(name)}&www=${includeWww ? 1 : 0}&proxied=${cfProxied ? 1 : 0}`);
        if (!res.ok) {
          if (!cancelled) setDnsPreview(null);
          return;
        }
        const data = await res.json();
        if (!cancelled) setDnsPreview(data);
      } catch {
        if (!cancelled) setDnsPreview(null);
      }
    }, 600);
    return () => {
      cancelled = true;
      clearTimeout(timer);
    };
  }, [domain, includeWww, cfProxied]);

  useEffect(() => {
    if (templateType !== 'node' || portTouched) return;
    const loadSuggestedPort = async () => {
      try {
        const res = await fetch('/api/ports/suggest?type=node');
        if (!res.ok) return;
        const data = await res.json();
        if (typeof data.port === 'number') {
          setPort(data.port);
        }
      } catch {
        // Ignore suggestion failures; keep default
      }
    };
    loadSuggestedPort();
  }, [templateType, portTouched]);

  const handleSubmit = async () => {
    if (!domain.trim()) {
      toast.error('Domain is required');
      return;
    }
    setLoading(true);
    try {
      const res = await fetch('/api/containers', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          domain: domain.trim(),
          template_type: templateType,
          code_path: codePath.trim() || undefined,
          port: templateType === 'node' ? Number(port) || 3000 : undefined,
          include_www: includeWww,
          cloudflare_dns: dnsPreview && (dnsPreview.status === 'ready' || dnsPreview.status === 'conflict') ? cfDns : false,
          cloudflare_proxied: cfProxied,
        }),
      });
      const data = await res.json().catch(() => ({}));
      if (!res.ok) {
        throw new Error(data.error || 'Failed to create container');
      }
      const warning = data.warning;
      const dns = data.dns as DnsPreview | undefined;
      if (dns && dns.status === 'done') {
        toast.success(`Site created, and its DNS records were added in Cloudflare (${dns.zone}).`);
      } else if (dns && dns.status === 'conflict') {
        toast.error(`Site created, but DNS was left alone: ${dns.message || 'a record already points somewhere else.'}`);
      } else if (dns && (dns.status === 'error' || dns.status === 'no-ip')) {
        toast.error(`Site created, but Cloudflare DNS wasn’t set up: ${dns.message || dns.status}`);
      } else if (warning) {
        toast.success(`Container created. Note: ${warning}`);
      } else {
        toast.success('Container created with nginx proxy configured');
      }
      onCreated();
      onClose();
    } catch (err: any) {
      toast.error(err.message || 'Failed to create container');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="fixed inset-0 bg-black/80 backdrop-blur-sm flex items-center justify-center z-[10000] p-4">
      <div className="card-vapor max-w-2xl w-full p-6 rounded-2xl border-2 border-neon-purple/40">
        <div className="flex items-start justify-between gap-3 mb-4">
          <div>
                          <div className="text-xs uppercase tracking-[0.2em] text-neon-purple/70">Create Container</div>            <div className="text-2xl font-bold text-neon-cyan">New Site</div>
          </div>
          <button onClick={onClose} className="btn-neon px-3 py-1 text-sm font-bold">✕ Close</button>
        </div>

        <div className="space-y-4">
          <div>
            <label className="text-sm font-bold text-neon-cyan/80 block mb-2">Domain</label>
            <input
              className="input-vapor w-full px-3 py-2 font-mono"
              placeholder="example.com"
              value={domain}
              onChange={(e) => setDomain(e.target.value)}
            />
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div>
              <label className="text-sm font-bold text-neon-cyan/80 block mb-2">Template</label>
              <select
                className="input-vapor w-full px-3 py-2"
                value={templateType}
                onChange={(e) => setTemplateType(e.target.value as TemplateType)}
              >
                <option value="static">Static (nginx)</option>
                <option value="php">PHP (php-nginx)</option>
                <option value="node">Node.js</option>
              </select>
            </div>
            <div>
              <label className="text-sm font-bold text-neon-cyan/80 block mb-2">Code Path (optional)</label>
              <input
                className="input-vapor w-full px-3 py-2 font-mono"
                placeholder="/var/www/sites/username/example.com"
                value={codePath}
                onChange={(e) => setCodePath(e.target.value)}
              />
            </div>
          </div>

          {templateType === 'node' && (
            <div>
              <label className="text-sm font-bold text-neon-cyan/80 block mb-2">Node Internal Port</label>
              <input
                type="number"
                className="input-vapor w-full px-3 py-2 font-mono"
                value={port}
                onChange={(e) => {
                  setPort(Number(e.target.value));
                  setPortTouched(true);
                }}
                min={1}
              />
            </div>
          )}

          <div className="flex items-center gap-2">
            <input
              id="include-www"
              type="checkbox"
              className="w-4 h-4 accent-cyan-400"
              checked={includeWww}
              onChange={(e) => setIncludeWww(e.target.checked)}
            />
                              <label htmlFor="include-www" className="text-sm text-neon-purple">              Request SSL for <code>www.{domain || 'example.com'}</code> too
            </label>
          </div>

          {dnsPreview && (
            <div className="rounded-lg p-3 text-sm space-y-2" style={{ border: '1px solid rgba(var(--neon-purple-rgb), 0.35)' }}>
              <div className="font-bold" style={{ color: 'var(--neon-cyan)' }}>Cloudflare DNS</div>
              {dnsPreview.status === 'not-configured' && (
                <p className="text-xs" style={{ color: 'var(--text-secondary)' }}>
                  Cloudflare isn’t connected, so you’ll point this domain’s DNS at the server yourself. Connect it under Settings → Cloudflare and DockLite can do this step for you.
                </p>
              )}
              {dnsPreview.status === 'no-zone' && (
                <p className="text-xs" style={{ color: 'var(--text-secondary)' }}>
                  This domain isn’t in your Cloudflare account yet, or you haven’t imported it. Import your domains under Settings → Cloudflare.
                </p>
              )}
              {(dnsPreview.status === 'no-ip' || dnsPreview.status === 'error') && (
                <p className="text-xs" style={{ color: 'var(--status-warning)' }}>{dnsPreview.message}</p>
              )}
              {(dnsPreview.status === 'ready' || dnsPreview.status === 'conflict') && (
                <>
                  <label className="flex items-center gap-2 cursor-pointer">
                    <input type="checkbox" className="w-4 h-4 accent-cyan-400" checked={cfDns} onChange={(e) => setCfDns(e.target.checked)} />
                    <span>Set up its DNS in Cloudflare for me</span>
                  </label>
                  {cfDns && (
                    <>
                      <ul className="text-xs font-mono space-y-1" style={{ color: 'var(--text-secondary)' }}>
                        {dnsPreview.records?.map((r) => (
                          <li key={r.type + r.name}>
                            {r.action === 'exists' && <span style={{ color: 'var(--neon-green)' }}>✓ already set: </span>}
                            {r.action === 'create' && <span style={{ color: 'var(--neon-cyan)' }}>+ will add: </span>}
                            {r.action === 'conflict' && <span style={{ color: 'var(--status-warning)' }}>! left alone: </span>}
                            {r.type} {r.name} → {r.content}
                            {r.action === 'conflict' && r.existing ? ` (there is already ${r.existing})` : ''}
                          </li>
                        ))}
                      </ul>
                      <label className="flex items-center gap-2 text-xs cursor-pointer" style={{ color: 'var(--text-secondary)' }}>
                        <input type="checkbox" className="w-4 h-4 accent-cyan-400" checked={cfProxied} onChange={(e) => setCfProxied(e.target.checked)} />
                        <span>Proxy through Cloudflare (orange cloud: hides your server’s IP, adds HTTPS and caching)</span>
                      </label>
                      <p className="text-xs" style={{ color: 'var(--text-secondary)' }}>
                        Existing records are never changed. If one already points somewhere else, it’s left alone and you’ll be told.
                      </p>
                    </>
                  )}
                </>
              )}
            </div>
          )}

          <div className="flex justify-end gap-3 pt-2">
            <button
              onClick={onClose}
              className="btn-neon px-4 py-2 font-bold"
              disabled={loading}
            >
              Cancel
            </button>
            <button
              onClick={handleSubmit}
              className="btn-neon px-6 py-2 font-bold"
              disabled={loading}
            >
              {loading ? 'Creating...' : 'Create'}
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
