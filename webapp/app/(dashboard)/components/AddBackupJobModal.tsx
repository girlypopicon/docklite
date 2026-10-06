'use client';

import { useState, useEffect } from 'react';
import { ClockCounterClockwise, X, WarningCircle } from '@phosphor-icons/react';

interface AddBackupJobModalProps {
  destinations: any[];
  onClose: () => void;
  onSuccess: () => void;
  initialJob?: any;
  mode?: 'create' | 'edit';
}

type TargetType = 'all-sites' | 'site' | 'all-databases' | 'database';

const TARGET_TYPES = [
  { value: 'all-sites', label: 'All Sites', description: 'Backup all sites' },
  { value: 'site', label: 'Specific Site', description: 'Backup one site' },
  { value: 'all-databases', label: 'All Databases', description: 'Backup all databases' },
  { value: 'database', label: 'Specific Database', description: 'Backup one database' },
];

const FREQUENCIES = [
  { value: 'hourly', label: 'Every Hour' },
  { value: 'every-6-hours', label: 'Every 6 Hours' },
  { value: 'every-12-hours', label: 'Every 12 Hours' },
  { value: 'daily', label: 'Daily (midnight)' },
  { value: 'every-3-days', label: 'Every 3 Days' },
  { value: 'weekly', label: 'Weekly (Sunday)' },
  { value: 'monthly', label: 'Monthly (1st)' },
];

export default function AddBackupJobModal({ destinations, onClose, onSuccess, initialJob, mode = 'create' }: AddBackupJobModalProps) {
  const [destinationId, setDestinationId] = useState(initialJob?.destination || destinations[0]?.id || 0);
  const [targetType, setTargetType] = useState<TargetType>(initialJob?.target_type || 'all-databases');
  const [targetId, setTargetId] = useState<number | ''>(initialJob?.target_id ?? '');
  const [frequency, setFrequency] = useState(initialJob?.frequency || 'daily');
  const [retentionDays, setRetentionDays] = useState(initialJob?.retention_days || 30);
  const [enabled, setEnabled] = useState(initialJob ? initialJob.enabled === 1 : true);

  const [databases, setDatabases] = useState<any[]>([]);
  const [sites, setSites] = useState<any[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  // Load databases if needed
  useEffect(() => {
    if (targetType === 'database') {
      loadDatabases();
    }
    if (targetType === 'site') {
      loadSites();
    }
  }, [targetType]);

  useEffect(() => {
    if (!initialJob) return;
    setDestinationId(initialJob.destination || destinations[0]?.id || 0);
    setTargetType(initialJob.target_type || 'all-databases');
    setTargetId(initialJob.target_id ?? '');
    setFrequency(initialJob.frequency || 'daily');
    setRetentionDays(initialJob.retention_days || 30);
    setEnabled(initialJob.enabled === 1);
  }, [initialJob, destinations]);

  const loadDatabases = async () => {
    try {
      const res = await fetch('/api/databases/stats');
      const data = await res.json();
      const normalized = (data.databases || []).map((db: any) => ({
        id: Number(db.id),
        name: db.name,
      }));
      setDatabases(normalized);
      if (normalized.length > 0) {
        setTargetId((current) => (current === '' ? normalized[0].id : current));
      }
    } catch (err) {
      console.error('Error loading databases:', err);
    }
  };

  const loadSites = async () => {
    try {
      const res = await fetch('/api/containers/all');
      const data = await res.json();
      const found = (data.containers || [])
        .map((container: any) => {
          const siteId = container.labels?.['docklite.site.id'];
          if (!siteId) return null;
          const domain = container.labels?.['docklite.domain'] || container.name || 'Site';
          return { id: Number(siteId), label: domain };
        })
        .filter(Boolean);
      setSites(found);
      if (found.length > 0) {
        setTargetId((current) => (current === '' ? found[0].id : current));
      }
    } catch (err) {
      console.error('Error loading sites:', err);
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    setLoading(true);

    try {
      const isEdit = mode === 'edit' && initialJob?.id;
      const selectedTargetId = targetId === '' ? null : targetId;
      const payload: any = {
        destination_id: destinationId,
        target_type: targetType,
        target_id: targetType === 'database' || targetType === 'site' ? selectedTargetId : null,
        frequency,
        retention_days: retentionDays,
        enabled: enabled ? 1 : 0,
      };

      if (isEdit) {
        payload.id = initialJob.id;
      }

      const res = await fetch('/api/backups/jobs', {
        method: isEdit ? 'PUT' : 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload)
      });

      const data = await res.json();

      if (!res.ok) {
        throw new Error(data.error || 'Failed to create backup job');
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
            <ClockCounterClockwise size={32} weight="duotone" color="var(--status-success)" />
            {mode === 'edit' ? 'Edit Schedule' : 'Create Schedule'}
          </h2>
          <p className="text-sm mt-2" style={{ color: 'var(--text-secondary)' }}>
            Schedule automated backups for your sites and databases
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
              Backup Destination
            </label>
            <select
              value={destinationId}
              onChange={(e) => setDestinationId(Number(e.target.value))}
              className="input-vapor w-full"
              required
              disabled={loading || destinations.length === 0}
            >
              {destinations.map((dest) => (
                <option key={dest.id} value={dest.id}>
                  {dest.name} ({dest.type.toUpperCase()})
                </option>
              ))}
            </select>
            {destinations.length === 0 && (
              <p className="text-xs mt-1 flex items-center gap-2" style={{ color: 'var(--status-warning)' }}>
                <WarningCircle size={14} weight="duotone" />
                No destinations configured. A local destination will be used automatically.
              </p>
            )}
          </div>

          <div>
            <label className="block text-sm font-bold mb-2" style={{ color: 'var(--neon-pink)' }}>
              What to Backup
            </label>
            <select
              value={targetType}
              onChange={(e) => {
                setTargetType(e.target.value as TargetType);
                setTargetId('');
              }}
              className="input-vapor w-full"
              required
              disabled={loading}
            >
              {TARGET_TYPES.map((t) => (
                <option key={t.value} value={t.value}>
                  {t.label} - {t.description}
                </option>
              ))}
            </select>
          </div>

          {targetType === 'site' && (
            <div>
              <label className="block text-sm font-bold mb-2" style={{ color: 'var(--neon-pink)' }}>
                Select Site
              </label>
              <select
                value={targetId}
                onChange={(e) => setTargetId(Number(e.target.value))}
                className="input-vapor w-full"
                required
                disabled={loading || sites.length === 0}
              >
                {sites.map((site) => (
                  <option key={site.id} value={site.id}>
                    {site.label}
                  </option>
                ))}
              </select>
              {sites.length === 0 && (
                <p className="text-xs mt-1" style={{ color: 'var(--text-secondary)' }}>
                  No sites found
                </p>
              )}
            </div>
          )}

          {targetType === 'database' && (
            <div>
              <label className="block text-sm font-bold mb-2" style={{ color: 'var(--neon-pink)' }}>
                Select Database
              </label>
              <select
                value={targetId}
                onChange={(e) => setTargetId(Number(e.target.value))}
                className="input-vapor w-full"
                required
                disabled={loading || databases.length === 0}
              >
                {databases.map((db) => (
                  <option key={db.id} value={db.id}>
                    {db.name}
                  </option>
                ))}
              </select>
              {databases.length === 0 && (
                <p className="text-xs mt-1" style={{ color: 'var(--text-secondary)' }}>
                  No databases found
                </p>
              )}
            </div>
          )}

          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-sm font-bold mb-2" style={{ color: 'var(--neon-pink)' }}>
                Frequency
              </label>
              <select
                value={frequency}
                onChange={(e) => setFrequency(e.target.value)}
                className="input-vapor w-full"
                required
                disabled={loading}
              >
                {FREQUENCIES.map((f) => (
                  <option key={f.value} value={f.value}>
                    {f.label}
                  </option>
                ))}
              </select>
            </div>

            <div>
              <label className="block text-sm font-bold mb-2" style={{ color: 'var(--neon-pink)' }}>
                Keep Backups For
              </label>
              <select
                value={retentionDays}
                onChange={(e) => setRetentionDays(Number(e.target.value))}
                className="input-vapor w-full"
                required
                disabled={loading}
              >
                <option value={7}>7 days</option>
                <option value={14}>14 days</option>
                <option value={30}>30 days</option>
                <option value={60}>60 days</option>
                <option value={90}>90 days</option>
                <option value={180}>180 days</option>
                <option value={365}>1 year</option>
              </select>
            </div>
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
              Enable this backup job
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
              disabled={loading || destinations.length === 0}
              className="flex-1 px-4 py-3 rounded-lg font-bold transition-all hover:scale-105 disabled:opacity-50 disabled:cursor-not-allowed"
              style={{
                background: 'linear-gradient(135deg, var(--neon-cyan) 0%, var(--neon-purple) 100%)',
                color: 'var(--button-text)',
                boxShadow: '0 0 20px rgba(var(--status-success-rgb), 0.4)',
              }}
            >
              {loading ? (mode === 'edit' ? 'Saving...' : 'Creating...') : (mode === 'edit' ? 'Save Schedule' : 'Create Schedule')}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
