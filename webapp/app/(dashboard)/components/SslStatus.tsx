'use client';

import { useEffect, useState } from 'react';
import {
  Lock,
  CheckCircle,
  WarningCircle,
  XCircle,
  Circle,
  ArrowClockwise,
  SpinnerGap,
  Plus,
  Trash,
} from '@phosphor-icons/react';
import { useToast } from '@/lib/hooks/useToast';

interface SslCert {
  domain: string;
  domains?: string[];
  hasSSL: boolean;
  expiryDate: string | null;
  daysUntilExpiry: number | null;
  status: 'valid' | 'expiring' | 'expired' | 'none' | 'unknown';
}

interface SslMeta {
  provider?: string;
  certCount?: number;
  managedCount?: number;
}

export default function SslStatus() {
  const [managedCerts, setManagedCerts] = useState<SslCert[]>([]);
  const [allCerts, setAllCerts] = useState<SslCert[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [lastChecked, setLastChecked] = useState<Date | null>(null);
  const [refreshing, setRefreshing] = useState(false);
  const [meta, setMeta] = useState<SslMeta | null>(null);
  const [showAllModal, setShowAllModal] = useState(false);
  const [showIssueModal, setShowIssueModal] = useState(false);
  const [actionLoading, setActionLoading] = useState<string | null>(null);
  const toast = useToast();

  const fetchSslStatus = async () => {
    setRefreshing(true);
    try {
      const res = await fetch('/api/ssl/status');
      if (!res.ok) throw new Error('Failed to fetch SSL status');
      const data = await res.json();
      setManagedCerts(data.sites || []);
      setAllCerts(data.allCerts || []);
      setMeta(data.meta || null);
      setError('');
      setLastChecked(new Date());
    } catch (err: any) {
      setError(err.message);
    } finally {
      setLoading(false);
      setRefreshing(false);
    }
  };

  const issueSsl = async (domain: string, includeWww: boolean, email: string) => {
    setActionLoading(domain);
    try {
      const res = await fetch('/api/ssl/issue', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ domain, includeWww, email }),
      });
      const data = await res.json().catch(() => ({}));
      if (!res.ok) throw new Error(data.detail || data.error || 'Failed to issue SSL');
      toast.success(`SSL certificate issued for ${domain}`);
      fetchSslStatus();
    } catch (err: any) {
      toast.error(err.message || 'Failed to issue SSL');
    } finally {
      setActionLoading(null);
    }
  };

  const renewSsl = async (domain: string) => {
    setActionLoading(domain);
    try {
      const res = await fetch('/api/ssl/renew', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ domain }),
      });
      const data = await res.json().catch(() => ({}));
      if (!res.ok) throw new Error(data.detail || data.error || 'Failed to renew SSL');
      toast.success(`SSL certificate renewed for ${domain}`);
      fetchSslStatus();
    } catch (err: any) {
      toast.error(err.message || 'Failed to renew SSL');
    } finally {
      setActionLoading(null);
    }
  };

  const deleteSsl = async (domain: string) => {
    if (!confirm(`Delete SSL certificate for ${domain}? This cannot be undone.`)) return;
    setActionLoading(domain);
    try {
      const res = await fetch('/api/ssl/delete', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ domain }),
      });
      const data = await res.json().catch(() => ({}));
      if (!res.ok) throw new Error(data.detail || data.error || 'Failed to delete SSL');
      toast.success(`SSL certificate deleted for ${domain}`);
      fetchSslStatus();
    } catch (err: any) {
      toast.error(err.message || 'Failed to delete SSL');
    } finally {
      setActionLoading(null);
    }
  };

  useEffect(() => {
    fetchSslStatus();
    const interval = setInterval(fetchSslStatus, 5 * 60 * 1000);
    return () => clearInterval(interval);
  }, []);

  useEffect(() => {
    if (showAllModal || showIssueModal) {
      document.body.style.overflow = 'hidden';
    } else {
      document.body.style.overflow = 'unset';
    }
    return () => { document.body.style.overflow = 'unset'; };
  }, [showAllModal, showIssueModal]);

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'valid': return 'var(--neon-green)';
      case 'expiring': return 'var(--status-warning)';
      case 'expired': return 'var(--status-error)';
      case 'unknown': return 'var(--text-secondary)';
      default: return 'var(--text-secondary)';
    }
  };

  const getStatusIcon = (status: string) => {
    switch (status) {
      case 'valid': return <CheckCircle size={14} weight="duotone" />;
      case 'expiring': return <WarningCircle size={14} weight="duotone" />;
      case 'expired': return <XCircle size={14} weight="duotone" />;
      default: return <Circle size={14} weight="duotone" />;
    }
  };

  const formatExpiryDate = (date: string | null, days: number | null) => {
    if (!date) return 'Unknown';
    const expiryDate = new Date(date);
    const formatted = expiryDate.toLocaleDateString('en-US', {
      year: 'numeric', month: 'short', day: 'numeric',
    });
    if (days !== null) return `${formatted} (${days}d)`;
    return formatted;
  };

  return (
    <>
      <div className="card-vapor p-6">
        <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3 mb-6">
          <div>
            <h2 className="text-2xl font-bold neon-text flex items-center gap-2" style={{ color: 'var(--neon-cyan)' }}>
              <Lock size={20} weight="duotone" />
              SSL Certificates
            </h2>
            <div className="text-xs font-mono mt-1 space-y-1" style={{ color: 'var(--text-secondary)' }}>
              <div>Last checked: {lastChecked ? lastChecked.toLocaleTimeString() : '—'}</div>
              {meta?.provider && <div>Provider: {meta.provider}</div>}
              {typeof meta?.managedCount === 'number' && <div>Managed sites: {meta.managedCount}</div>}
              {typeof meta?.certCount === 'number' && <div>Total certs: {meta.certCount}</div>}
            </div>
            {error && (
              <div className="text-xs font-bold mt-1 flex items-center gap-2" style={{ color: 'var(--status-error)' }}>
                <XCircle size={14} weight="duotone" />
                {error}
              </div>
            )}
          </div>
          <div className="flex gap-2">
            <button
              onClick={() => setShowIssueModal(true)}
              className="btn-neon px-4 py-2 text-sm font-bold"
            >
              <span className="inline-flex items-center gap-2">
                <Plus size={14} weight="bold" />
                Issue SSL
              </span>
            </button>
            <button
              onClick={fetchSslStatus}
              disabled={refreshing}
              className="btn-neon px-4 py-2 text-sm font-bold disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {refreshing ? (
                <span className="inline-flex items-center gap-2">
                  <SpinnerGap size={14} weight="duotone" className="animate-spin" />
                  Refreshing…
                </span>
              ) : (
                <span className="inline-flex items-center gap-2">
                  <ArrowClockwise size={14} weight="duotone" />
                  Refresh
                </span>
              )}
            </button>
          </div>
        </div>

        {loading ? (
          <div className="text-sm font-mono flex items-center gap-2" style={{ color: 'var(--text-secondary)' }}>
            <SpinnerGap size={14} weight="duotone" className="animate-spin" />
            Loading SSL status...
          </div>
        ) : (() => {
          const managedDomains = new Set(managedCerts.map(c => c.domain));
          const otherCerts = allCerts.filter(c => !managedDomains.has(c.domain));
          const combinedCerts = [
            ...managedCerts.map(c => ({ ...c, source: 'managed' as const })),
            ...otherCerts.map(c => ({ ...c, source: 'server' as const })),
          ];

          return combinedCerts.length === 0 ? (
            <div className="text-sm font-mono" style={{ color: 'var(--text-secondary)' }}>
              No certificates found. Create a site and issue an SSL certificate to get started.
            </div>
          ) : (
          <div className="overflow-x-auto">
            <table className="w-full font-mono text-sm">
              <thead>
                <tr style={{ borderBottom: '2px solid var(--neon-purple)' }}>
                  <th className="text-left py-3 px-4" style={{ color: 'var(--neon-pink)' }}>DOMAIN</th>
                  <th className="text-left py-3 px-4" style={{ color: 'var(--neon-pink)' }}>SOURCE</th>
                  <th className="text-left py-3 px-4" style={{ color: 'var(--neon-pink)' }}>STATUS</th>
                  <th className="text-left py-3 px-4" style={{ color: 'var(--neon-pink)' }}>EXPIRES</th>
                  <th className="text-left py-3 px-4" style={{ color: 'var(--neon-pink)' }}>ACTIONS</th>
                </tr>
              </thead>
              <tbody>
                {combinedCerts.map((cert) => (
                  <tr
                    key={`${cert.source}-${cert.domain}`}
                    className="hover:bg-white/5 transition-colors"
                    style={{ borderBottom: '1px solid rgba(var(--text-muted-rgb), 0.1)' }}
                  >
                    <td className="py-3 px-4">
                      <div style={{ color: 'var(--neon-cyan)' }}>{cert.domain}</div>
                      {cert.domains && cert.domains.length > 1 && (
                        <div className="text-[10px] mt-1" style={{ color: 'var(--text-secondary)' }}>
                          {cert.domains.join(', ')}
                        </div>
                      )}
                    </td>
                    <td className="py-3 px-4">
                      {cert.source === 'managed' ? (
                        <span className="text-[10px] font-bold px-2 py-1 rounded-full" style={{ background: 'rgba(var(--neon-cyan-rgb), 0.1)', color: 'var(--neon-cyan)', border: '1px solid rgba(var(--neon-cyan-rgb), 0.3)' }}>
                          DockLite
                        </span>
                      ) : (
                        <span className="text-[10px] font-bold px-2 py-1 rounded-full" style={{ background: 'rgba(var(--neon-purple-rgb), 0.1)', color: 'var(--neon-purple)', border: '1px solid rgba(var(--neon-purple-rgb), 0.3)' }}>
                          Server
                        </span>
                      )}
                    </td>
                    <td className="py-3 px-4">
                      <span className="inline-flex items-center gap-1" style={{ color: getStatusColor(cert.status) }}>
                        {getStatusIcon(cert.status)} {cert.status.toUpperCase()}
                      </span>
                    </td>
                    <td className="py-3 px-4" style={{ color: 'var(--text-secondary)' }}>
                      {cert.hasSSL ? formatExpiryDate(cert.expiryDate, cert.daysUntilExpiry) : 'No SSL'}
                    </td>
                    <td className="py-3 px-4">
                      <div className="flex gap-2">
                        {cert.hasSSL ? (
                          <>
                            <button
                              onClick={() => renewSsl(cert.domain)}
                              disabled={actionLoading === cert.domain}
                              className="btn-neon px-3 py-1 text-xs font-bold disabled:opacity-50"
                            >
                              {actionLoading === cert.domain ? (
                                <SpinnerGap size={12} className="animate-spin" />
                              ) : (
                                'Renew'
                              )}
                            </button>
                            <button
                              onClick={() => deleteSsl(cert.domain)}
                              disabled={actionLoading === cert.domain}
                              className="px-3 py-1 text-xs font-bold rounded border transition-colors"
                              style={{
                                borderColor: 'var(--status-error)',
                                color: 'var(--status-error)',
                              }}
                            >
                              <Trash size={12} weight="bold" />
                            </button>
                          </>
                        ) : (
                          <button
                            onClick={() => issueSsl(cert.domain, true, '')}
                            disabled={actionLoading === cert.domain}
                            className="btn-neon px-3 py-1 text-xs font-bold disabled:opacity-50"
                          >
                            {actionLoading === cert.domain ? (
                              <SpinnerGap size={12} className="animate-spin" />
                            ) : (
                              'Issue SSL'
                            )}
                          </button>
                        )}
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          );
        })()}
      </div>

      {showIssueModal && <IssueSslModal onClose={() => setShowIssueModal(false)} onIssue={issueSsl} loading={!!actionLoading} />}

    </>
  );
}

function IssueSslModal({ onClose, onIssue, loading }: { onClose: () => void; onIssue: (domain: string, includeWww: boolean, email: string) => Promise<void>; loading: boolean }) {
  const [domain, setDomain] = useState('');
  const [includeWww, setIncludeWww] = useState(true);
  const [email, setEmail] = useState('');

  const handleSubmit = async () => {
    if (!domain.trim()) return;
    await onIssue(domain.trim(), includeWww, email.trim());
    onClose();
  };

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center p-4"
      style={{ backgroundColor: 'var(--modal-backdrop, rgba(0, 0, 0, 0.8))' }}
      onClick={onClose}
    >
      <div className="card-vapor max-w-lg w-full p-6 rounded-2xl border-2 border-neon-purple/40" onClick={(e) => e.stopPropagation()}>
        <div className="flex items-start justify-between gap-3 mb-4">
          <div>
            <div className="text-xs uppercase tracking-[0.2em]" style={{ color: 'var(--neon-purple)' }}>SSL Certificate</div>
            <div className="text-2xl font-bold" style={{ color: 'var(--neon-cyan)' }}>Issue New Certificate</div>
          </div>
          <button onClick={onClose} className="btn-neon px-3 py-1 text-sm font-bold">✕</button>
        </div>

        <div className="space-y-4">
          <div>
            <label className="text-sm font-bold block mb-2" style={{ color: 'var(--neon-cyan)' }}>Domain</label>
            <input
              className="input-vapor w-full px-3 py-2 font-mono"
              placeholder="example.com"
              value={domain}
              onChange={(e) => setDomain(e.target.value)}
            />
          </div>

          <div>
            <label className="text-sm font-bold block mb-2" style={{ color: 'var(--neon-cyan)' }}>Email (optional)</label>
            <input
              className="input-vapor w-full px-3 py-2 font-mono"
              placeholder="admin@example.com"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
            />
            <div className="text-xs mt-1" style={{ color: 'var(--text-secondary)' }}>
              For Let&apos;s Encrypt renewal notifications. Leave blank to skip.
            </div>
          </div>

          <div className="flex items-center gap-2">
            <input
              id="ssl-include-www"
              type="checkbox"
              className="w-4 h-4 accent-cyan-400"
              checked={includeWww}
              onChange={(e) => setIncludeWww(e.target.checked)}
            />
            <label htmlFor="ssl-include-www" className="text-sm" style={{ color: 'var(--neon-purple)' }}>
              Include www.{domain || 'example.com'}
            </label>
          </div>

          <div className="flex justify-end gap-3 pt-2">
            <button onClick={onClose} className="btn-neon px-4 py-2 font-bold" disabled={loading}>Cancel</button>
            <button onClick={handleSubmit} className="btn-neon px-6 py-2 font-bold" disabled={loading || !domain.trim()}>
              {loading ? (
                <span className="inline-flex items-center gap-2">
                  <SpinnerGap size={14} className="animate-spin" /> Issuing…
                </span>
              ) : 'Issue Certificate'}
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
