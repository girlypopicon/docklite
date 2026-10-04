'use client';

import { useState, useEffect, useMemo, useCallback, useRef } from 'react';
import {
  HardDrives,
  ClockCounterClockwise,
  Play,
  ArrowsClockwise,
  PencilSimple,
  Copy,
  Trash,
  DownloadSimple,
  Info,
  ShieldCheck,
  Sparkle,
} from '@phosphor-icons/react';
import AddBackupJobModal from '../components/AddBackupJobModal';
import ConfirmDeleteModal from '../components/ConfirmDeleteModal';
import { useToast } from '@/lib/hooks/useToast';

type Tab = 'backups' | 'schedules' | 'restore';

type BackupRecord = {
  id: number;
  target_type: string;
  target_id: number;
  backup_path?: string;
  size_bytes?: number;
  status: string;
  created_at: string;
  error_message?: string;
};

type BackupJob = {
  id: number;
  target_type: string;
  target_id?: number;
  frequency: string;
  retention_days: number;
  enabled: number;
  destination: number;
  last_run_at?: string;
  next_run_at?: string;
};

type BackupDestination = {
  id: number;
  name: string;
  type: string;
  enabled: number;
};

export default function BackupsPage() {
  const [activeTab, setActiveTab] = useState<Tab>('backups');
  const [backups, setBackups] = useState<BackupRecord[]>([]);
  const [jobs, setJobs] = useState<BackupJob[]>([]);
  const [destinations, setDestinations] = useState<BackupDestination[]>([]);
  const [localBackupPath, setLocalBackupPath] = useState('');
  const [isAdmin, setIsAdmin] = useState(false);

  const [exportSites, setExportSites] = useState<Array<{ id: number; label: string }>>([]);
  const [exportDatabases, setExportDatabases] = useState<Array<{ id: number; label: string }>>([]);
  const [exportTargetType, setExportTargetType] = useState<'site' | 'database'>('site');
  const [exportTargetId, setExportTargetId] = useState<number | null>(null);
  const [exportDelivery, setExportDelivery] = useState<'download' | 'local'>('download');
  const [exportRetentionDays, setExportRetentionDays] = useState<string>('7');
  const [exportNotes, setExportNotes] = useState('');
  const [exportLoading, setExportLoading] = useState(false);

  const [filterApp, setFilterApp] = useState('all');
  const [filterType, setFilterType] = useState('all');
  const [filterStatus, setFilterStatus] = useState('all');
  const [filterStart, setFilterStart] = useState('');
  const [filterEnd, setFilterEnd] = useState('');

  const [loading, setLoading] = useState(true);
  const [showScheduleModal, setShowScheduleModal] = useState(false);
  const [editingSchedule, setEditingSchedule] = useState<BackupJob | null>(null);
  const [duplicateSchedule, setDuplicateSchedule] = useState<BackupJob | null>(null);
  const [confirmAction, setConfirmAction] = useState<{
    type: 'backup' | 'schedule';
    id: number;
    name: string;
  } | null>(null);
  const [deleteLoading, setDeleteLoading] = useState(false);
  const toast = useToast();

  // The loaders below must not depend on state they themselves change (the
  // admin flag, the auto-selected target): that made them a new function every
  // time, which re-ran the load effect and re-showed the loading screen two or
  // three times on first visit. They read the current values from refs instead.
  const exportTypeRef = useRef(exportTargetType);
  exportTypeRef.current = exportTargetType;
  const exportIdRef = useRef(exportTargetId);
  exportIdRef.current = exportTargetId;
  // Tabs whose data has been shown once. Only the first load shows the
  // loading screen; later refreshes update in place.
  const loadedTabs = useRef<Record<string, boolean>>({});

  const loadExportData = useCallback(async () => {
    try {
      let admin = false;
      const authRes = await fetch('/api/auth/me');
      if (authRes.ok) {
        const authData = await authRes.json();
        admin = Boolean(authData?.user?.isAdmin);
        setIsAdmin(admin);
      }
      const [containersRes, databasesRes] = await Promise.all([
        fetch('/api/containers/all'),
        fetch('/api/databases/stats'),
      ]);
      const containersData = await containersRes.json();
      const databasesData = await databasesRes.json();

      const sites = (containersData.containers || [])
        .map((container: any) => {
          const siteId = container.labels?.['docklite.site.id'];
          if (!siteId) return null;
          const domain = container.labels?.['docklite.domain'] || container.name || 'Site';
          return { id: Number(siteId), label: `${domain} (ID ${siteId})` };
        })
        .filter(Boolean) as Array<{ id: number; label: string }>;

      const databases = (databasesData.databases || []).map((db: any) => ({
        id: db.id,
        label: `${db.name} (ID ${db.id})`,
      }));

      setExportSites(sites);
      setExportDatabases(databases);

      if (!admin && exportTypeRef.current === 'site') {
        setExportTargetType('database');
        setExportTargetId(databases.length > 0 ? databases[0].id : null);
        return;
      }

      if (!exportIdRef.current) {
        const first = (exportTypeRef.current === 'site' ? sites : databases)[0];
        if (first) setExportTargetId(first.id);
      }
    } catch (error) {
      console.error('Error loading export options:', error);
    }
  }, []);

  const loadBackups = useCallback(async () => {
    if (!loadedTabs.current.backups) setLoading(true);
    try {
      const [backupsRes, localRes] = await Promise.all([
        fetch('/api/backups'),
        fetch('/api/backups/local'),
      ]);
      if (backupsRes.ok) {
        const backupsData = await backupsRes.json();
        setBackups(backupsData.backups || []);
      } else {
        setBackups([]);
      }

      if (localRes.ok) {
        const localData = await localRes.json();
        setLocalBackupPath(localData.path || '');
      }

      await loadExportData();
    } catch (error) {
      console.error('Error loading backups:', error);
    } finally {
      loadedTabs.current.backups = true;
      setLoading(false);
    }
  }, [loadExportData]);

  const loadSchedules = useCallback(async () => {
    if (!loadedTabs.current.schedules) setLoading(true);
    try {
      const [jobsRes, destinationsRes] = await Promise.all([
        fetch('/api/backups/jobs'),
        fetch('/api/backups/destinations'),
      ]);
      if (jobsRes.ok) {
        const jobsData = await jobsRes.json();
        setJobs(jobsData.jobs || []);
      } else {
        setJobs([]);
      }
      if (destinationsRes.ok) {
        const destinationsData = await destinationsRes.json();
        setDestinations(destinationsData.destinations || []);
      } else {
        setDestinations([]);
      }
    } catch (error) {
      console.error('Error loading schedules:', error);
    } finally {
      loadedTabs.current.schedules = true;
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (activeTab === 'backups') {
      loadBackups();
    } else if (activeTab === 'schedules') {
      loadSchedules();
    } else if (activeTab === 'restore') {
      setLoading(false);
    }
  }, [activeTab, loadBackups, loadSchedules]);

  const appLabelMap = useMemo(() => {
    const map = new Map<string, string>();
    exportSites.forEach((site) => map.set(`site:${site.id}`, site.label));
    exportDatabases.forEach((db) => map.set(`database:${db.id}`, db.label));
    return map;
  }, [exportDatabases, exportSites]);

  const getBackupAppLabel = (backup: BackupRecord) => {
    return appLabelMap.get(`${backup.target_type}:${backup.target_id}`) ||
      `${backup.target_type} #${backup.target_id}`;
  };

  const getBackupTypeLabel = (backup: BackupRecord) => {
    if (backup.target_type === 'database') return 'db';
    if (backup.target_type === 'site') return 'files';
    return backup.target_type;
  };

  const getDownloadUrl = (backup: BackupRecord) => {
    if (!backup.backup_path) return null;
    const normalized = backup.backup_path.replace(/\\/g, '/');
    if (normalized.startsWith('sites/') || normalized.startsWith('databases/') || normalized.startsWith('downloads/')) {
      return `/api/backups/local/download?file=${encodeURIComponent(normalized)}`;
    }
    if (localBackupPath) {
      const base = localBackupPath.replace(/\\/g, '/').replace(/\/+$/, '');
      if (normalized.startsWith(base)) {
        const rel = normalized.slice(base.length).replace(/^\/+/, '');
        if (rel) {
          return `/api/backups/local/download?file=${encodeURIComponent(rel)}`;
        }
      }
    }
    return null;
  };

  const filteredBackups = useMemo(() => {
    return backups.filter((backup) => {
      if (filterApp !== 'all') {
        const key = `${backup.target_type}:${backup.target_id}`;
        if (key !== filterApp) return false;
      }
      if (filterType !== 'all' && getBackupTypeLabel(backup) !== filterType) return false;
      if (filterStatus !== 'all' && backup.status !== filterStatus) return false;
      if (filterStart) {
        const start = new Date(filterStart).getTime();
        if (new Date(backup.created_at).getTime() < start) return false;
      }
      if (filterEnd) {
        const end = new Date(filterEnd).getTime();
        if (new Date(backup.created_at).getTime() > end) return false;
      }
      return true;
    });
  }, [backups, filterApp, filterEnd, filterStart, filterStatus, filterType]);

  const handleExport = async () => {
    if (!exportTargetId) {
      toast.error('Select a target to back up');
      return;
    }
    setExportLoading(true);
    try {
      const payload: any = {
        target_type: exportTargetType,
        target_id: exportTargetId,
        delivery: exportDelivery,
      };
      if (exportDelivery === 'local') {
        const days = Number(exportRetentionDays);
        if (!Number.isNaN(days) && days > 0) {
          payload.retention_days = days;
        }
      }
      if (exportNotes.trim()) {
        payload.notes = exportNotes.trim();
      }
      const res = await fetch('/api/backups/export', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      });
      const raw = await res.text();
      let data: any = {};
      if (raw) {
        try {
          data = JSON.parse(raw);
        } catch {
          throw new Error(`Unexpected response: ${raw.slice(0, 200)}`);
        }
      }
      if (!res.ok) {
        throw new Error(data.error || 'Backup failed');
      }
      if (data.download_url) {
        window.location.href = data.download_url;
        toast.success('Backup complete. Download starting');
      } else {
        toast.success('Backup complete');
      }
      loadBackups();
    } catch (error: any) {
      toast.error(error.message || 'Backup failed');
    } finally {
      setExportLoading(false);
    }
  };

  const handleDelete = async () => {
    if (!confirmAction) return;
    setDeleteLoading(true);
    try {
      let endpoint = '';
      if (confirmAction.type === 'backup') {
        endpoint = `/api/backups?id=${confirmAction.id}`;
      } else {
        endpoint = `/api/backups/jobs?id=${confirmAction.id}`;
      }
      const res = await fetch(endpoint, { method: 'DELETE' });
      const data = await res.json();
      if (!res.ok) {
        throw new Error(data.error || 'Failed to delete');
      }
      if (confirmAction.type === 'backup') {
        toast.success('Backup deleted');
        loadBackups();
      } else {
        toast.success('Schedule deleted');
        loadSchedules();
      }
      setConfirmAction(null);
    } catch (error: any) {
      toast.error(error.message || 'Delete failed');
    } finally {
      setDeleteLoading(false);
    }
  };

  const handleToggleSchedule = async (job: BackupJob, enabled: number) => {
    try {
      const res = await fetch('/api/backups/jobs', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ id: job.id, enabled }),
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || 'Failed to update schedule');
      toast.success(enabled ? 'Schedule enabled' : 'Schedule disabled');
      loadSchedules();
    } catch (error: any) {
      toast.error(error.message || 'Update failed');
    }
  };

  const handleRunSchedule = async (jobId: number) => {
    try {
      const res = await fetch('/api/backups/trigger', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ job_id: jobId }),
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || 'Failed to trigger schedule');
      toast.success('Schedule triggered');
    } catch (error: any) {
      toast.error(error.message || 'Trigger failed');
    }
  };

  const handleDuplicateSchedule = (job: BackupJob) => {
    setDuplicateSchedule(job);
    setEditingSchedule(null);
    setShowScheduleModal(true);
  };

  const handleEditSchedule = (job: BackupJob) => {
    setEditingSchedule(job);
    setDuplicateSchedule(null);
    setShowScheduleModal(true);
  };

  const scheduleDestinationMap = useMemo(() => {
    return new Map(destinations.map((dest) => [dest.id, dest]));
  }, [destinations]);

  return (
    <div className="docklite-backups max-w-[1400px] mx-auto space-y-6">
      <div className="flex flex-col md:flex-row md:items-center md:justify-between gap-4">
        <div>
          <h1 className="text-3xl font-bold neon-text-pink flex items-center gap-3">
            <HardDrives size={32} weight="duotone" color="var(--neon-pink)" />
            Backups
          </h1>
          <p className="text-sm text-gray-400 mt-1">
            Run instant backups, schedule regular protection, and manage restores.
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-3">
          {activeTab === 'backups' && (
            <button onClick={loadBackups} className="cyber-button-sm flex items-center gap-2">
              <ArrowsClockwise size={16} weight="duotone" />
              Refresh
            </button>
          )}
          {activeTab === 'schedules' && (
            <button
              onClick={() => {
                setEditingSchedule(null);
                setDuplicateSchedule(null);
                setShowScheduleModal(true);
              }}
              className="cyber-button flex items-center gap-2"
            >
              <ClockCounterClockwise size={18} weight="duotone" />
              Create schedule
            </button>
          )}
          {activeTab === 'restore' && (
            <button
              onClick={() => toast.info('Restore wizard coming soon')}
              className="cyber-button flex items-center gap-2"
            >
              <ShieldCheck size={18} weight="duotone" />
              Start restore
            </button>
          )}
        </div>
      </div>

      <div className="flex gap-2 border-b border-neon-purple/30">
        <button
          onClick={() => setActiveTab('backups')}
          className={`px-4 py-2 font-bold transition-colors ${
            activeTab === 'backups'
              ? 'border-b-2 border-neon-pink text-neon-pink'
              : 'text-gray-400 hover:text-neon-cyan'
          }`}
        >
          <HardDrives size={20} weight="duotone" className="inline mr-2" />
          Backups
        </button>
        <button
          onClick={() => setActiveTab('schedules')}
          className={`px-4 py-2 font-bold transition-colors ${
            activeTab === 'schedules'
              ? 'border-b-2 border-neon-pink text-neon-pink'
              : 'text-gray-400 hover:text-neon-cyan'
          }`}
        >
          <ClockCounterClockwise size={20} weight="duotone" className="inline mr-2" />
          Schedules
        </button>
        <button
          onClick={() => setActiveTab('restore')}
          className={`px-4 py-2 font-bold transition-colors ${
            activeTab === 'restore'
              ? 'border-b-2 border-neon-pink text-neon-pink'
              : 'text-gray-400 hover:text-neon-cyan'
          }`}
        >
          <ShieldCheck size={20} weight="duotone" className="inline mr-2" />
          Restore
        </button>
      </div>

      <div className="cyber-card p-6">
        {loading ? (
          <div className="text-center py-12 text-gray-400">Loading...</div>
        ) : (
          <>
            {activeTab === 'backups' && (
              <BackupsTab
                backups={filteredBackups}
                allBackups={backups}
                exportSites={exportSites}
                exportDatabases={exportDatabases}
                exportTargetType={exportTargetType}
                exportTargetId={exportTargetId}
                exportDelivery={exportDelivery}
                exportRetentionDays={exportRetentionDays}
                exportNotes={exportNotes}
                exportLoading={exportLoading}
                isAdmin={isAdmin}
                onTargetTypeChange={(value: 'site' | 'database') => {
                  if (!isAdmin && value === 'site') {
                    toast.error('Site backups require an admin account');
                    return;
                  }
                  setExportTargetType(value);
                  setExportTargetId(null);
                }}
                onTargetIdChange={(value: number) => setExportTargetId(value)}
                onDeliveryChange={(value: 'download' | 'local') => setExportDelivery(value)}
                onRetentionDaysChange={setExportRetentionDays}
                onNotesChange={setExportNotes}
                onExport={handleExport}
                getBackupAppLabel={getBackupAppLabel}
                getBackupTypeLabel={getBackupTypeLabel}
                getDownloadUrl={getDownloadUrl}
                onDelete={(backup: BackupRecord) =>
                  setConfirmAction({ type: 'backup', id: backup.id, name: getBackupAppLabel(backup) })
                }
                onRestore={() => toast.info('Restore workflow coming soon')}
                onDetails={() => toast.info('Details view coming soon')}
                filterApp={filterApp}
                filterType={filterType}
                filterStatus={filterStatus}
                filterStart={filterStart}
                filterEnd={filterEnd}
                onFilterApp={setFilterApp}
                onFilterType={setFilterType}
                onFilterStatus={setFilterStatus}
                onFilterStart={setFilterStart}
                onFilterEnd={setFilterEnd}
              />
            )}

            {activeTab === 'schedules' && (
              <SchedulesTab
                jobs={jobs}
                destinations={destinations}
                destinationMap={scheduleDestinationMap}
                onEdit={handleEditSchedule}
                onDuplicate={handleDuplicateSchedule}
                onDelete={(job: BackupJob) =>
                  setConfirmAction({ type: 'schedule', id: job.id, name: `Schedule #${job.id}` })
                }
                onRun={handleRunSchedule}
                onToggle={handleToggleSchedule}
              />
            )}

            {activeTab === 'restore' && <RestoreTab />}
          </>
        )}
      </div>

      {showScheduleModal && (
        <AddBackupJobModal
          destinations={destinations}
          onClose={() => setShowScheduleModal(false)}
          onSuccess={() => {
            loadSchedules();
            setShowScheduleModal(false);
          }}
          initialJob={duplicateSchedule || editingSchedule || undefined}
          mode={editingSchedule ? 'edit' : 'create'}
        />
      )}

      {confirmAction && (
        <ConfirmDeleteModal
          title={confirmAction.type === 'backup' ? 'Delete Backup' : 'Delete Schedule'}
          message={
            confirmAction.type === 'backup'
              ? 'Delete this restore point and its files? This cannot be undone.'
              : 'Delete this schedule? Existing backups are not affected.'
          }
          itemName={confirmAction.name}
          onConfirm={handleDelete}
          onCancel={() => setConfirmAction(null)}
          loading={deleteLoading}
        />
      )}
      <toast.ToastContainer />
    </div>
  );
}

function BackupsTab({
  backups,
  allBackups,
  exportSites,
  exportDatabases,
  exportTargetType,
  exportTargetId,
  exportDelivery,
  exportRetentionDays,
  exportNotes,
  exportLoading,
  isAdmin,
  onTargetTypeChange,
  onTargetIdChange,
  onDeliveryChange,
  onRetentionDaysChange,
  onNotesChange,
  onExport,
  getBackupAppLabel,
  getBackupTypeLabel,
  getDownloadUrl,
  onDelete,
  onRestore,
  onDetails,
  filterApp,
  filterType,
  filterStatus,
  filterStart,
  filterEnd,
  onFilterApp,
  onFilterType,
  onFilterStatus,
  onFilterStart,
  onFilterEnd,
}: any) {
  const appOptions = useMemo(() => {
    const options = new Map<string, string>();
    allBackups.forEach((backup: any) => {
      options.set(`${backup.target_type}:${backup.target_id}`, getBackupAppLabel(backup));
    });
    return Array.from(options.entries()).map(([value, label]) => ({ value, label }));
  }, [allBackups, getBackupAppLabel]);

  return (
    <div className="space-y-8">
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        <div className="lg:col-span-2 card-vapor p-6 border border-neon-purple/40">
          <div className="flex items-center justify-between">
            <div>
              <h2 className="text-xl font-bold text-neon-cyan">Run backup now</h2>
              <p className="text-sm text-gray-400 mt-1">
                Create an instant restore point and optionally download it.
              </p>
            </div>
            <Sparkle size={28} weight="duotone" color="var(--status-success)" />
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mt-6">
            <div>
              <label className="block text-sm text-gray-400 mb-2">Target type</label>
              <select
                value={exportTargetType}
                onChange={(e) => onTargetTypeChange(e.target.value)}
                className="w-full px-3 py-2 bg-dark-bg/60 border border-neon-purple/30 rounded-md text-gray-100"
              >
                <option value="site" disabled={!isAdmin}>Site</option>
                <option value="database">Database</option>
              </select>
              {!isAdmin && (
                <p className="text-xs text-gray-500 mt-1">Site backups require admin access.</p>
              )}
            </div>
            <div>
              <label className="block text-sm text-gray-400 mb-2">Target</label>
              <select
                value={exportTargetId ?? ''}
                onChange={(e) => onTargetIdChange(Number(e.target.value))}
                className="w-full px-3 py-2 bg-dark-bg/60 border border-neon-purple/30 rounded-md text-gray-100"
              >
                <option value="" disabled>
                  Select a target
                </option>
                {(exportTargetType === 'site' ? exportSites : exportDatabases).map((target: any) => (
                  <option key={target.id} value={target.id}>
                    {target.label}
                  </option>
                ))}
              </select>
            </div>
            <div>
              <label className="block text-sm text-gray-400 mb-2">Delivery</label>
              <select
                value={exportDelivery}
                onChange={(e) => onDeliveryChange(e.target.value)}
                className="w-full px-3 py-2 bg-dark-bg/60 border border-neon-purple/30 rounded-md text-gray-100"
              >
                <option value="download">Download</option>
                <option value="local">Local</option>
              </select>
            </div>
            {exportDelivery === 'local' && (
              <div>
                <label className="block text-sm text-gray-400 mb-2">Retention days</label>
                <input
                  type="number"
                  min="1"
                  value={exportRetentionDays}
                  onChange={(e) => onRetentionDaysChange(e.target.value)}
                  className="w-full px-3 py-2 bg-dark-bg/60 border border-neon-purple/30 rounded-md text-gray-100"
                />
              </div>
            )}
            <div className="md:col-span-2">
              <label className="block text-sm text-gray-400 mb-2">Notes (optional)</label>
              <input
                type="text"
                value={exportNotes}
                onChange={(e) => onNotesChange(e.target.value)}
                placeholder="Why this backup was created"
                className="w-full px-3 py-2 bg-dark-bg/60 border border-neon-purple/30 rounded-md text-gray-100"
              />
            </div>
          </div>

          <button
            onClick={onExport}
            disabled={exportLoading}
            className="cyber-button mt-6 flex items-center gap-2"
          >
            <Play size={18} weight="duotone" />
            {exportLoading ? 'Creating backup...' : 'Run backup now'}
          </button>
        </div>

        <div className="card-vapor p-6 border border-neon-purple/30">
          <h3 className="text-lg font-bold text-neon-pink mb-4">Filters</h3>
          <div className="space-y-4">
            <div>
              <label className="block text-xs text-gray-400 mb-2">App</label>
              <select
                value={filterApp}
                onChange={(e) => onFilterApp(e.target.value)}
                className="w-full px-3 py-2 bg-dark-bg/60 border border-neon-purple/30 rounded-md text-gray-100"
              >
                <option value="all">All apps</option>
                {appOptions.map((option) => (
                  <option key={option.value} value={option.value}>
                    {option.label}
                  </option>
                ))}
              </select>
            </div>
            <div>
              <label className="block text-xs text-gray-400 mb-2">Type</label>
              <select
                value={filterType}
                onChange={(e) => onFilterType(e.target.value)}
                className="w-full px-3 py-2 bg-dark-bg/60 border border-neon-purple/30 rounded-md text-gray-100"
              >
                <option value="all">All types</option>
                <option value="files">Files</option>
                <option value="db">Database</option>
              </select>
            </div>
            <div>
              <label className="block text-xs text-gray-400 mb-2">Status</label>
              <select
                value={filterStatus}
                onChange={(e) => onFilterStatus(e.target.value)}
                className="w-full px-3 py-2 bg-dark-bg/60 border border-neon-purple/30 rounded-md text-gray-100"
              >
                <option value="all">All statuses</option>
                <option value="success">Success</option>
                <option value="failed">Failed</option>
                <option value="in_progress">In progress</option>
              </select>
            </div>
            <div>
              <label className="block text-xs text-gray-400 mb-2">Date range</label>
              <input
                type="date"
                value={filterStart}
                onChange={(e) => onFilterStart(e.target.value)}
                className="w-full px-3 py-2 bg-dark-bg/60 border border-neon-purple/30 rounded-md text-gray-100 mb-2"
              />
              <input
                type="date"
                value={filterEnd}
                onChange={(e) => onFilterEnd(e.target.value)}
                className="w-full px-3 py-2 bg-dark-bg/60 border border-neon-purple/30 rounded-md text-gray-100"
              />
            </div>
          </div>
        </div>
      </div>

      <div>
        <div className="flex items-center justify-between mb-4">
          <h3 className="text-lg font-bold text-neon-cyan">Restore points</h3>
          <span className="text-xs text-gray-500">{backups.length} total</span>
        </div>
        {backups.length === 0 ? (
          <div className="text-center py-12 text-gray-400">No backups yet</div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-left text-sm">
              <thead className="text-xs uppercase text-gray-500 border-b border-neon-purple/20">
                <tr>
                  <th className="py-3 pr-4">Date</th>
                  <th className="py-3 pr-4">App</th>
                  <th className="py-3 pr-4">Type</th>
                  <th className="py-3 pr-4">Size</th>
                  <th className="py-3 pr-4">Status</th>
                  <th className="py-3 pr-4">Actions</th>
                </tr>
              </thead>
              <tbody>
                {backups.map((backup: any) => {
                  const downloadUrl = getDownloadUrl(backup);
                  return (
                    <tr key={backup.id} className="border-b border-neon-purple/10">
                      <td className="py-4 pr-4 text-gray-200">
                        {new Date(backup.created_at).toLocaleString()}
                      </td>
                      <td className="py-4 pr-4 text-gray-200">{getBackupAppLabel(backup)}</td>
                      <td className="py-4 pr-4 text-gray-200 uppercase">{getBackupTypeLabel(backup)}</td>
                      <td className="py-4 pr-4 text-gray-200">{formatBytes(backup.size_bytes)}</td>
                      <td className="py-4 pr-4">
                        <span className={`px-2 py-1 rounded-full text-xs ${statusBadge(backup.status)}`}>
                          {backup.status.replace('_', ' ')}
                        </span>
                      </td>
                      <td className="py-4 pr-4">
                        <div className="flex flex-wrap gap-2">
                          <button
                            onClick={() => onRestore(backup)}
                            className="cyber-button-sm flex items-center gap-1"
                          >
                            <ShieldCheck size={14} weight="duotone" />
                            Restore
                          </button>
                          <button
                            onClick={() => onDetails(backup)}
                            className="cyber-button-sm flex items-center gap-1"
                          >
                            <Info size={14} weight="duotone" />
                            Details
                          </button>
                          {downloadUrl && (
                            <a
                              href={downloadUrl}
                              className="cyber-button-sm flex items-center gap-1"
                            >
                              <DownloadSimple size={14} weight="duotone" />
                              Download
                            </a>
                          )}
                          <button
                            onClick={() => onDelete(backup)}
                            className="cyber-button-sm bg-red-500/20 hover:bg-red-500/30 border border-red-500/30"
                            style={{ color: 'var(--status-error)' }}
                          >
                            <Trash size={14} weight="duotone" />
                            Delete
                          </button>
                        </div>
                        {backup.error_message && (
                          <div className="text-xs text-status-error mt-2">{backup.error_message}</div>
                        )}
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  );
}

function SchedulesTab({ jobs, destinations, destinationMap, onEdit, onDuplicate, onDelete, onRun, onToggle }: any) {
  return (
    <div className="space-y-4">
      {jobs.length === 0 ? (
        <div className="text-center py-12 text-gray-400">No schedules configured yet</div>
      ) : (
        jobs.map((job: any) => {
          const destination = destinationMap.get(job.destination);
          const enabled = job.enabled === 1;
          return (
            <div
              key={job.id}
              className="p-5 bg-dark-bg/40 rounded-xl border border-neon-purple/20 hover:border-neon-cyan/40 transition-colors"
            >
              <div className="flex flex-col md:flex-row md:items-center md:justify-between gap-4">
                <div className="space-y-1">
                  <div className="text-sm text-gray-400">Schedule #{job.id}</div>
                  <h3 className="text-lg font-bold text-neon-pink">
                    {job.target_type.replace('-', ' ')}{job.target_id ? ` #${job.target_id}` : ''}
                  </h3>
                  <div className="text-sm text-gray-400">
                    Frequency: {job.frequency} • Retention: {job.retention_days} days
                  </div>
                  <div className="text-xs text-gray-500">
                    Destination: {destination ? destination.name : 'Local'} • {enabled ? 'Enabled' : 'Disabled'}
                  </div>
                  <div className="text-xs text-gray-500">
                    Last run: {job.last_run_at ? new Date(job.last_run_at).toLocaleString() : '—'}
                  </div>
                  <div className="text-xs text-gray-500">
                    Next run: {job.next_run_at ? new Date(job.next_run_at).toLocaleString() : '—'}
                  </div>
                </div>
                <div className="flex flex-wrap gap-2">
                  <button onClick={() => onRun(job.id)} className="cyber-button-sm flex items-center gap-1">
                    <Play size={14} weight="duotone" />
                    Run now
                  </button>
                  <button onClick={() => onEdit(job)} className="cyber-button-sm flex items-center gap-1">
                    <PencilSimple size={14} weight="duotone" />
                    Edit
                  </button>
                  <button onClick={() => onDuplicate(job)} className="cyber-button-sm flex items-center gap-1">
                    <Copy size={14} weight="duotone" />
                    Duplicate
                  </button>
                  <button
                    onClick={() => onToggle(job, enabled ? 0 : 1)}
                    className="cyber-button-sm flex items-center gap-1"
                  >
                    <ArrowsClockwise size={14} weight="duotone" />
                    {enabled ? 'Disable' : 'Enable'}
                  </button>
                  <button
                    onClick={() => onDelete(job)}
                    className="cyber-button-sm bg-red-500/20 hover:bg-red-500/30 border border-red-500/30"
                    style={{ color: 'var(--status-error)' }}
                  >
                    <Trash size={14} weight="duotone" />
                    Delete
                  </button>
                </div>
              </div>
            </div>
          );
        })
      )}
      {destinations.length === 0 && (
        <div className="text-xs text-gray-500">
          No destinations available. Backups will use the default local destination.
        </div>
      )}
    </div>
  );
}

function RestoreTab() {
  return (
    <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
      <div className="lg:col-span-2 card-vapor p-6 border border-neon-purple/30">
        <h2 className="text-xl font-bold text-neon-cyan">Restore wizard</h2>
        <p className="text-sm text-gray-400 mt-2">
          Choose a restore point and walk through a guided recovery flow.
        </p>
        <button className="cyber-button mt-6 flex items-center gap-2" disabled>
          <ShieldCheck size={18} weight="duotone" />
          Start restore (coming soon)
        </button>
      </div>
      <div className="card-vapor p-6 border border-neon-purple/20">
        <h3 className="text-lg font-bold text-neon-pink">Recent restores</h3>
        <div className="text-sm text-gray-400 mt-4">No restore history yet.</div>
      </div>
    </div>
  );
}

function formatBytes(bytes?: number) {
  if (bytes === undefined || bytes === null || Number.isNaN(bytes)) return 'N/A';
  if (bytes === 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  const index = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1);
  const value = bytes / Math.pow(1024, index);
  const decimals = value >= 10 || index === 0 ? 0 : 2;
  return `${value.toFixed(decimals)} ${units[index]}`;
}

function statusBadge(status: string) {
  switch (status) {
    case 'success':
      return 'bg-status-success/20 text-status-success';
    case 'failed':
      return 'bg-status-error/20 text-status-error';
    case 'in_progress':
      return 'bg-status-warning/20 text-status-warning';
    default:
      return 'bg-gray-500/20 text-gray-300';
  }
}
