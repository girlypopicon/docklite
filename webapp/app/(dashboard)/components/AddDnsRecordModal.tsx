'use client';

import { useState, useEffect } from 'react';
import { Globe, X, Info, Cloud } from '@phosphor-icons/react';

interface AddDnsRecordModalProps {
  zones: any[];
  selectedZone?: number | null;
  onClose: () => void;
  onSuccess: () => void;
}

const RECORD_TYPES = [
  { value: 'A', label: 'A', description: 'IPv4 address' },
  { value: 'AAAA', label: 'AAAA', description: 'IPv6 address' },
  { value: 'CNAME', label: 'CNAME', description: 'Canonical name' },
  { value: 'MX', label: 'MX', description: 'Mail exchange' },
  { value: 'TXT', label: 'TXT', description: 'Text record' },
  { value: 'NS', label: 'NS', description: 'Name server' },
  { value: 'SRV', label: 'SRV', description: 'Service record' },
  { value: 'CAA', label: 'CAA', description: 'Certificate authority' },
];

export default function AddDnsRecordModal({ zones, selectedZone, onClose, onSuccess }: AddDnsRecordModalProps) {
  const [zoneId, setZoneId] = useState<number>(selectedZone || zones[0]?.id || 0);
  const [recordType, setRecordType] = useState('A');
  const [name, setName] = useState('');
  const [content, setContent] = useState('');
  const [ttl, setTtl] = useState(1); // 1 = Auto
  const [priority, setPriority] = useState<number | ''>('');
  const [proxied, setProxied] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  // Update zone if selectedZone changes
  useEffect(() => {
    if (selectedZone) {
      setZoneId(selectedZone);
    }
  }, [selectedZone]);

  const showPriority = recordType === 'MX' || recordType === 'SRV';
  const showProxied = recordType === 'A' || recordType === 'AAAA' || recordType === 'CNAME';

  const getPlaceholder = () => {
    switch (recordType) {
      case 'A':
        return '192.0.2.1';
      case 'AAAA':
        return '2001:0db8::1';
      case 'CNAME':
        return 'example.com';
      case 'MX':
        return 'mail.example.com';
      case 'TXT':
        return 'v=spf1 include:_spf.example.com ~all';
      case 'NS':
        return 'ns1.example.com';
      case 'SRV':
        return '0 5 5060 sipserver.example.com';
      case 'CAA':
        return '0 issue "letsencrypt.org"';
      default:
        return '';
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    setLoading(true);

    try {
      const selectedZoneData = zones.find(z => z.id === zoneId);
      if (!selectedZoneData) {
        throw new Error('Zone not found');
      }

      // Create record in Cloudflare and locally
      const createRes = await fetch(`/api/dns/records`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          zone_id: zoneId,
          type: recordType,
          name: name.trim() || '@',
          content: content.trim(),
          ttl: ttl,
          priority: showPriority && priority !== '' ? Number(priority) : null,
          proxied: showProxied ? (proxied ? 1 : 0) : 0,
        })
      });

      const data = await createRes.json();

      if (!createRes.ok) {
        throw new Error(data.error || 'Failed to create DNS record');
      }

      onSuccess();
      onClose();
    } catch (err: any) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  const selectedZoneData = zones.find(z => z.id === zoneId);

  return (
    <div className="fixed inset-0 z-[10000] flex items-center justify-center p-4 bg-black/70 backdrop-blur-sm">
      <div
        className="card-vapor max-w-2xl w-full p-6 relative max-h-[90vh] overflow-y-auto"
        style={{
          background: 'linear-gradient(135deg, var(--modal-bg-1) 0%, var(--modal-bg-2) 100%)',
          border: '2px solid var(--neon-purple)',
          boxShadow: '0 0 30px rgba(var(--neon-purple-rgb), 0.5)',
        }}
      >
        {/* Close button */}
        <button
          onClick={onClose}
          className="absolute top-4 right-4 p-2 rounded-lg transition-all hover:scale-110"
          style={{
            background: 'rgba(var(--status-error-rgb), 0.2)',
            border: '1px solid var(--status-error)',
          }}
        >
          <X size={20} color="var(--status-error)" weight="bold" />
        </button>

        {/* Header */}
        <div className="mb-6">
          <h2 className="text-2xl font-bold neon-text flex items-center gap-3" style={{ color: 'var(--neon-cyan)' }}>
            <Globe size={32} weight="duotone" color="var(--status-success)" />
            Add DNS Record
          </h2>
          <p className="text-sm mt-2" style={{ color: 'var(--text-secondary)' }}>
            Create a new DNS record in Cloudflare
          </p>
        </div>

        {error && (
          <div
            className="mb-4 p-3 rounded-lg border"
            style={{
              background: 'rgba(var(--status-error-rgb), 0.1)',
              border: '1px solid var(--status-error)',
              color: 'var(--status-error)',
            }}
          >
            {error}
          </div>
        )}

        <form onSubmit={handleSubmit} className="space-y-4">
          {/* Zone Selection */}
          <div>
            <label className="block text-sm font-bold mb-2" style={{ color: 'var(--neon-pink)' }}>
              DNS Zone
            </label>
            <select
              value={zoneId}
              onChange={(e) => setZoneId(Number(e.target.value))}
              className="input-vapor w-full"
              required
              disabled={loading || zones.length === 0}
            >
              {zones.map((zone) => (
                <option key={zone.id} value={zone.id}>
                  {zone.domain}
                </option>
              ))}
            </select>
          </div>

          {/* Record Type */}
          <div>
            <label className="block text-sm font-bold mb-2" style={{ color: 'var(--neon-pink)' }}>
              Record Type
            </label>
            <select
              value={recordType}
              onChange={(e) => setRecordType(e.target.value)}
              className="input-vapor w-full"
              required
              disabled={loading}
            >
              {RECORD_TYPES.map((type) => (
                <option key={type.value} value={type.value}>
                  {type.label} - {type.description}
                </option>
              ))}
            </select>
          </div>

          {/* Name */}
          <div>
            <label className="block text-sm font-bold mb-2" style={{ color: 'var(--neon-pink)' }}>
              Name
            </label>
            <div className="flex items-center gap-2">
              <input
                type="text"
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="@ or subdomain"
                className="input-vapor flex-1 font-mono text-sm"
                disabled={loading}
              />
              <span className="text-sm" style={{ color: 'var(--text-secondary)' }}>
                .{selectedZoneData?.domain || ''}
              </span>
            </div>
            <p className="text-xs mt-1" style={{ color: 'var(--text-secondary)' }}>
              Use @ for root domain, or enter a subdomain like &quot;www&quot; or &quot;blog&quot;
            </p>
          </div>

          {/* Content */}
          <div>
            <label className="block text-sm font-bold mb-2" style={{ color: 'var(--neon-pink)' }}>
              Content
            </label>
            <input
              type="text"
              value={content}
              onChange={(e) => setContent(e.target.value)}
              placeholder={getPlaceholder()}
              className="input-vapor w-full font-mono text-sm"
              required
              disabled={loading}
            />
            <p className="text-xs mt-1" style={{ color: 'var(--text-secondary)' }}>
              {recordType === 'A' && 'IPv4 address (e.g., 192.0.2.1)'}
              {recordType === 'AAAA' && 'IPv6 address (e.g., 2001:0db8::1)'}
              {recordType === 'CNAME' && 'Target domain (e.g., example.com)'}
              {recordType === 'MX' && 'Mail server domain'}
              {recordType === 'TXT' && 'Text value (e.g., SPF, DKIM, verification codes)'}
              {recordType === 'NS' && 'Name server domain'}
            </p>
          </div>

          <div className="grid grid-cols-2 gap-4">
            {/* TTL */}
            <div>
              <label className="block text-sm font-bold mb-2" style={{ color: 'var(--neon-pink)' }}>
                TTL (seconds)
              </label>
              <select
                value={ttl}
                onChange={(e) => setTtl(Number(e.target.value))}
                className="input-vapor w-full"
                disabled={loading}
              >
                <option value={1}>Auto</option>
                <option value={60}>1 minute</option>
                <option value={300}>5 minutes</option>
                <option value={900}>15 minutes</option>
                <option value={1800}>30 minutes</option>
                <option value={3600}>1 hour</option>
                <option value={7200}>2 hours</option>
                <option value={18000}>5 hours</option>
                <option value={43200}>12 hours</option>
                <option value={86400}>1 day</option>
              </select>
            </div>

            {/* Priority (for MX/SRV) */}
            {showPriority && (
              <div>
                <label className="block text-sm font-bold mb-2" style={{ color: 'var(--neon-pink)' }}>
                  Priority
                </label>
                <input
                  type="number"
                  value={priority}
                  onChange={(e) => setPriority(e.target.value === '' ? '' : Number(e.target.value))}
                  placeholder="10"
                  min="0"
                  className="input-vapor w-full"
                  required
                  disabled={loading}
                />
              </div>
            )}
          </div>

          {/* Proxied (for A/AAAA/CNAME) */}
          {showProxied && (
            <div className="flex items-center gap-3 p-3 rounded-lg" style={{ background: 'rgba(var(--status-warning-rgb), 0.1)' }}>
              <input
                type="checkbox"
                id="proxied"
                checked={proxied}
                onChange={(e) => setProxied(e.target.checked)}
                className="w-4 h-4"
                disabled={loading}
              />
              <div className="flex-1">
                <label htmlFor="proxied" className="text-sm font-bold flex items-center gap-2" style={{ color: 'var(--status-warning)' }}>
                  <Cloud size={16} weight="duotone" />
                  Proxy through Cloudflare
                </label>
                <p className="text-xs mt-1" style={{ color: 'var(--text-secondary)' }}>
                  Enable Cloudflare&apos;s CDN, DDoS protection, and SSL (orange cloud)
                </p>
              </div>
            </div>
          )}

          <div className="flex gap-3 pt-4">
            <button
              type="button"
              onClick={onClose}
              disabled={loading}
              className="flex-1 px-4 py-3 rounded-lg font-bold transition-all hover:scale-105"
              style={{
                background: 'rgba(var(--text-muted-rgb), 0.1)',
                border: '1px solid rgba(var(--text-muted-rgb), 0.2)',
                color: 'var(--text-secondary)',
              }}
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={loading || !content.trim() || zones.length === 0}
              className="flex-1 px-4 py-3 rounded-lg font-bold transition-all hover:scale-105 disabled:opacity-50 disabled:cursor-not-allowed"
              style={{
                background: 'linear-gradient(135deg, var(--neon-cyan) 0%, var(--neon-purple) 100%)',
                color: 'var(--button-text)',
                boxShadow: '0 0 20px rgba(var(--status-success-rgb), 0.4)',
              }}
            >
              {loading ? 'Creating...' : 'Create Record'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
