'use client';

import { useState } from 'react';
import { Globe, X, Lightbulb } from '@phosphor-icons/react';

interface AddDnsZoneModalProps {
  onClose: () => void;
  onSuccess: () => void;
}

export default function AddDnsZoneModal({ onClose, onSuccess }: AddDnsZoneModalProps) {
  const [domain, setDomain] = useState('');
  const [zoneId, setZoneId] = useState('');
  const [autoImport, setAutoImport] = useState(true);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    setLoading(true);

    try {
      const res = await fetch('/api/dns/zones', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          domain: domain.trim(),
          zone_id: zoneId.trim(),
          auto_import: autoImport
        })
      });

      const data = await res.json();

      if (!res.ok) {
        throw new Error(data.error || 'Failed to create DNS zone');
      }

      // If auto-import is enabled, trigger sync
      if (autoImport) {
        await fetch('/api/dns/sync', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ zone_id: data.id })
        });
      }

      onSuccess();
      onClose();
    } catch (err: any) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="fixed inset-0 z-[10000] flex items-center justify-center p-4 bg-black/70 backdrop-blur-sm">
      <div
        className="card-vapor max-w-lg w-full p-6 relative"
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
            Add DNS Zone
          </h2>
          <p className="text-sm mt-2" style={{ color: 'var(--text-secondary)' }}>
            Connect a Cloudflare DNS zone to DockLite
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
          <div>
            <label className="block text-sm font-bold mb-2" style={{ color: 'var(--neon-pink)' }}>
              Domain Name
            </label>
            <input
              type="text"
              value={domain}
              onChange={(e) => setDomain(e.target.value)}
              placeholder="example.com"
              className="input-vapor w-full"
              required
              disabled={loading}
            />
            <p className="text-xs mt-1" style={{ color: 'var(--text-secondary)' }}>
              The domain name managed in Cloudflare
            </p>
          </div>

          <div>
            <label className="block text-sm font-bold mb-2" style={{ color: 'var(--neon-pink)' }}>
              Cloudflare Zone ID
            </label>
            <input
              type="text"
              value={zoneId}
              onChange={(e) => setZoneId(e.target.value)}
              placeholder="a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6"
              className="input-vapor w-full font-mono text-sm"
              required
              disabled={loading}
            />
            <p className="text-xs mt-1" style={{ color: 'var(--text-secondary)' }}>
              Find this in your Cloudflare dashboard under the domain overview
            </p>
          </div>

          <div className="flex items-center gap-3 p-3 rounded-lg" style={{ background: 'rgba(var(--status-success-rgb), 0.1)' }}>
            <input
              type="checkbox"
              id="autoImport"
              checked={autoImport}
              onChange={(e) => setAutoImport(e.target.checked)}
              className="w-4 h-4"
              disabled={loading}
            />
            <label htmlFor="autoImport" className="text-sm font-bold" style={{ color: 'var(--neon-green)' }}>
              Auto-import DNS records after adding zone
            </label>
          </div>

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
              disabled={loading || !domain.trim() || !zoneId.trim()}
              className="flex-1 px-4 py-3 rounded-lg font-bold transition-all hover:scale-105 disabled:opacity-50 disabled:cursor-not-allowed"
              style={{
                background: 'linear-gradient(135deg, var(--neon-cyan) 0%, var(--neon-purple) 100%)',
                color: 'var(--button-text)',
                boxShadow: '0 0 20px rgba(var(--status-success-rgb), 0.4)',
              }}
            >
              {loading ? 'Adding...' : 'Add Zone'}
            </button>
          </div>
        </form>

        {/* Help text */}
        <div className="mt-6 p-4 rounded-lg" style={{ background: 'rgba(var(--status-success-rgb), 0.05)', border: '1px solid rgba(var(--status-success-rgb), 0.2)' }}>
          <p className="text-xs font-bold mb-2 flex items-center gap-2" style={{ color: 'var(--neon-green)' }}>
            <Lightbulb size={14} weight="duotone" />
            How to find your Zone ID:
          </p>
          <ol className="text-xs space-y-1" style={{ color: 'var(--text-secondary)' }}>
            <li>1. Go to your Cloudflare dashboard</li>
            <li>2. Select your domain</li>
            <li>3. Scroll down on the Overview page</li>
            <li>4. Copy the Zone ID from the right sidebar</li>
          </ol>
        </div>
      </div>
    </div>
  );
}
