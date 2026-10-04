'use client';

import { useState } from 'react';
import { HardDrives, X } from '@phosphor-icons/react';

interface AddBackupDestinationModalProps {
  onClose: () => void;
  onSuccess: () => void;
}

type DestinationType = 'local';

export default function AddBackupDestinationModal({ onClose, onSuccess }: AddBackupDestinationModalProps) {
  const [name, setName] = useState('');
  const [type] = useState<DestinationType>('local');
  const [enabled, setEnabled] = useState(true);

  // Local config
  const [path, setPath] = useState('/var/backups/docklite');

  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    setLoading(true);

    try {
      const config = { path };

      const res = await fetch('/api/backups/destinations', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          name: name.trim(),
          type,
          config,
          enabled: enabled ? 1 : 0
        })
      });

      const data = await res.json();

      if (!res.ok) {
        throw new Error(data.error || 'Failed to create backup destination');
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
        className="card-vapor max-w-2xl w-full p-6 relative max-h-[90vh] overflow-y-auto"
        style={{
          background: 'linear-gradient(135deg, var(--modal-bg-1) 0%, var(--modal-bg-2) 100%)',
          border: '2px solid var(--modal-border)',
          boxShadow: '0 0 30px var(--modal-shadow)',
        }}
      >
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

        <div className="mb-6">
          <h2 className="text-2xl font-bold neon-text flex items-center gap-3" style={{ color: 'var(--neon-cyan)' }}>
            <HardDrives size={32} weight="duotone" color="var(--status-success)" />
            Add Backup Destination
          </h2>
          <p className="text-sm mt-2" style={{ color: 'var(--text-secondary)' }}>
            Configure where backups will be stored
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
              Destination Name
            </label>
            <input
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="My Backup Storage"
              className="input-vapor w-full"
              required
              disabled={loading}
            />
          </div>

          <div>
            <label className="block text-sm font-bold mb-2" style={{ color: 'var(--neon-pink)' }}>
              Storage Type
            </label>
            <div className="input-vapor w-full flex items-center">
              Local Storage
            </div>
            <p className="text-xs mt-1" style={{ color: 'var(--text-secondary)' }}>
              Scheduled backups only support local storage in V1.
            </p>
          </div>

          <div>
            <label className="block text-sm font-bold mb-2" style={{ color: 'var(--neon-pink)' }}>
              Local Path
            </label>
            <input
              type="text"
              value={path}
              onChange={(e) => setPath(e.target.value)}
              placeholder="/var/backups/docklite"
              className="input-vapor w-full font-mono text-sm"
              required
              disabled={loading}
            />
            <p className="text-xs mt-1" style={{ color: 'var(--text-secondary)' }}>
              Directory where backups will be stored on the server.
            </p>
          </div>

          <div className="flex items-center gap-3 p-3 rounded-lg" style={{ background: 'rgba(var(--status-success-rgb), 0.1)' }}>
            <input
              type="checkbox"
              id="enabled"
              checked={enabled}
              onChange={(e) => setEnabled(e.target.checked)}
              className="w-4 h-4"
              disabled={loading}
            />
            <label htmlFor="enabled" className="text-sm font-bold" style={{ color: 'var(--neon-green)' }}>
              Enable this destination
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
              disabled={loading || !name.trim()}
              className="flex-1 px-4 py-3 rounded-lg font-bold transition-all hover:scale-105 disabled:opacity-50 disabled:cursor-not-allowed"
              style={{
                background: 'linear-gradient(135deg, var(--neon-cyan) 0%, var(--neon-purple) 100%)',
                color: 'var(--button-text)',
                boxShadow: '0 0 20px rgba(var(--status-success-rgb), 0.4)',
              }}
            >
              {loading ? 'Creating...' : 'Create Destination'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
