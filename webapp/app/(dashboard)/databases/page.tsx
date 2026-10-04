'use client';

import { useEffect, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import { Database as DatabaseType } from '@/types';
import DbViewer from './DbViewer';
import SkeletonLoader from '../components/SkeletonLoader';
import {
  Database,
  DotsThree,
  PencilSimpleLine,
  SignIn,
  Trash,
  DownloadSimple,
  Package,
  SpinnerGap,
  Sparkle,
  X,
  XCircle,
  Globe,
  Plug,
  UserCircle,
  Key,
  WarningCircle,
  ThumbsUp,
  Info,
  LinkSimple,
  Brain,
  ArrowDown,
  CalendarBlank,
  Lightbulb,
} from '@phosphor-icons/react';
import { useRouter } from 'next/navigation';

interface DatabaseWithSize extends DatabaseType {
  size: number;
  sizeCategory: 'empty' | 'tiny' | 'small' | 'medium' | 'large' | 'huge';
  username?: string;
}

interface DockliteDbInfo {
  size: number;
  tables: number;
  path: string;
}

export default function DatabasesPage() {
  const router = useRouter();
  const [databases, setDatabases] = useState<DatabaseWithSize[]>([]);
  const [dockliteDb, setDockliteDb] = useState<DockliteDbInfo | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [showCreateForm, setShowCreateForm] = useState(false);
  const [newDbName, setNewDbName] = useState('');
  const [newDbUsername, setNewDbUsername] = useState('docklite');
  const [newDbPassword, setNewDbPassword] = useState('');
  const [dbType, setDbType] = useState<'postgres' | 'sqlite'>('postgres');
  const [availableContainers, setAvailableContainers] = useState<any[]>([]);
  const [selectedContainerId, setSelectedContainerId] = useState('');
  const [sqlitePath, setSqlitePath] = useState('/app/data/db.sqlite');
  const [fetchingContainers, setFetchingContainers] = useState(false);
  const [creating, setCreating] = useState(false);
  const [connectionInfo, setConnectionInfo] = useState<any>(null);

  useEffect(() => {
    if (dbType === 'sqlite' && showCreateForm) {
      setFetchingContainers(true);
      fetch('/api/containers/all')
        .then(res => res.json())
        .then(data => {
            setAvailableContainers(data.containers || []);
            setFetchingContainers(false);
        })
        .catch(err => {
            console.error('Failed to fetch containers:', err);
            setFetchingContainers(false);
        });
    }
  }, [dbType, showCreateForm]);
  const [editingDb, setEditingDb] = useState<DatabaseWithSize | null>(null);
  const [editUsername, setEditUsername] = useState('');
  const [editPassword, setEditPassword] = useState('');
  const [menuDbId, setMenuDbId] = useState<number | null>(null);
  const [deleteDb, setDeleteDb] = useState<DatabaseWithSize | null>(null);
  const [deleteConfirmText, setDeleteConfirmText] = useState('');
  const [deletingDb, setDeletingDb] = useState(false);
  const [editModeDb, setEditModeDb] = useState<DatabaseWithSize | null>(null);
  const [editModeUsername, setEditModeUsername] = useState('');
  const [editModePassword, setEditModePassword] = useState('');
  const [downloadDb, setDownloadDb] = useState<DatabaseWithSize | null>(null);
  const [downloadUsername, setDownloadUsername] = useState('');
  const [downloadPassword, setDownloadPassword] = useState('');
  const [downloadGzip, setDownloadGzip] = useState(true);
  const [downloadingDb, setDownloadingDb] = useState(false);
  const [menuPosition, setMenuPosition] = useState<{ top: number; left: number } | null>(null);
  const menuPopupRef = useRef<HTMLDivElement>(null);
  const stopMenu = (event: React.MouseEvent | React.PointerEvent) => event.stopPropagation();

  const fetchDatabases = async () => {
    try {
      const res = await fetch('/api/databases/stats');
      if (!res.ok) throw new Error('Failed to fetch databases');

      const data = await res.json();
      setDatabases(data.databases);
      setDockliteDb(data.dockliteDb);
      setLoading(false);
    } catch (err) {
      setError('Failed to load databases');
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchDatabases();
    const interval = setInterval(fetchDatabases, 30000); // Refresh every 30s
    return () => clearInterval(interval);
  }, []);

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    setCreating(true);
    setError('');

    try {
      const payload: any = {
        name: newDbName,
        type: dbType,
      };

      if (dbType === 'postgres') {
        payload.username = newDbUsername;
        payload.password = newDbPassword || undefined;
      } else {
        payload.container_id = selectedContainerId;
        payload.db_path = sqlitePath;
      }

      const res = await fetch('/api/databases', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      });

      if (!res.ok) {
        const data = await res.json();
        throw new Error(data.error || 'Failed to create database');
      }

      const data = await res.json();
      setConnectionInfo(data.connection);
      setNewDbName('');
      setNewDbUsername('docklite');
      setNewDbPassword('');
      setSqlitePath('/app/data/db.sqlite');
      setSelectedContainerId('');
      setDbType('postgres');
      setShowCreateForm(false);
      fetchDatabases();
    } catch (err: any) {
      setError(err.message);
    } finally {
      setCreating(false);
    }
  };

  const handleEdit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!editingDb) return;

    setCreating(true);
    setError('');

    try {
      const res = await fetch(`/api/databases/${editingDb.id}`, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          username: editUsername,
          password: editPassword,
        }),
      });

      if (!res.ok) {
        const data = await res.json();
        throw new Error(data.error || 'Failed to update database');
      }

      setEditingDb(null);
      setEditUsername('');
      setEditPassword('');
      fetchDatabases();
      alert('✓ Database credentials updated successfully!');
    } catch (err: any) {
      setError(err.message);
    } finally {
      setCreating(false);
    }
  };

  const openEditModal = (db: DatabaseWithSize) => {
    setEditingDb(db);
    setEditUsername(db.username || 'docklite');
    setEditPassword('');
    setError('');
  };

  const openDeleteModal = (db: DatabaseWithSize) => {
    setDeleteDb(db);
    setDeleteConfirmText('');
    setError('');
  };

  const handleDelete = async () => {
    if (!deleteDb) return;
    setDeletingDb(true);
    setError('');

    try {
      const res = await fetch(`/api/databases/${deleteDb.id}`, { method: 'DELETE' });
      if (!res.ok) {
        const data = await res.json();
        throw new Error(data.error || 'Failed to delete database');
      }
      setDeleteDb(null);
      setDeleteConfirmText('');
      await fetchDatabases();
    } catch (err: any) {
      setError(err.message || 'Failed to delete database');
    } finally {
      setDeletingDb(false);
    }
  };

  const openEditModeModal = (db: DatabaseWithSize) => {
    setEditModeDb(db);
    setEditModeUsername(db.username || 'docklite');
    setEditModePassword('');
    setError('');
  };

  const handleEnterEditMode = () => {
    if (!editModeDb) return;
    if (!editModeUsername || !editModePassword) {
      setError('Database username and password are required for edit mode.');
      return;
    }
    const payload = {
      username: editModeUsername,
      password: editModePassword,
    };
    sessionStorage.setItem(`docklite-db-edit-${editModeDb.id}`, JSON.stringify(payload));
    setEditModeDb(null);
    router.push(`/databases/${editModeDb.id}/edit`);
  };

  const openDownloadModal = (db: DatabaseWithSize) => {
    setDownloadDb(db);
    setDownloadUsername('docklite');
    setDownloadPassword('');
    setDownloadGzip(true);
    setError('');
  };

  const handleDownload = async () => {
    if (!downloadDb) return;
    if (!downloadUsername || !downloadPassword) {
      setError('Database username and password are required to download.');
      return;
    }

    setDownloadingDb(true);
    setError('');

    try {
      const res = await fetch(`/api/databases/${downloadDb.id}/download`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          username: downloadUsername,
          password: downloadPassword,
          gzip: downloadGzip,
        }),
      });

      if (!res.ok) {
        const data = await res.json().catch(() => ({}));
        throw new Error(data.error || 'Failed to download database');
      }

      const blob = await res.blob();
      const disposition = res.headers.get('Content-Disposition') || '';
      const match = disposition.match(/filename="([^"]+)"/i);
      const fallbackName = downloadGzip
        ? `docklite-${downloadDb.name}.dump.gz`
        : `docklite-${downloadDb.name}.dump`;
      const filename = match?.[1] || fallbackName;

      const url = URL.createObjectURL(blob);
      const anchor = document.createElement('a');
      anchor.href = url;
      anchor.download = filename;
      document.body.appendChild(anchor);
      anchor.click();
      document.body.removeChild(anchor);
      URL.revokeObjectURL(url);

      setDownloadDb(null);
      setDownloadUsername('');
      setDownloadPassword('');
      setDownloadGzip(true);
    } catch (err: any) {
      setError(err.message || 'Failed to download database');
    } finally {
      setDownloadingDb(false);
    }
  };

  const formatBytes = (bytes: number) => {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return Math.round(bytes / Math.pow(k, i) * 100) / 100 + ' ' + sizes[i];
  };

  const getSizeLabel = (category: string) => {
    const labelMap = {
      empty: 'Empty',
      tiny: 'Tiny',
      small: 'Small',
      medium: 'Medium',
      large: 'Large',
      huge: 'Huge',
    };
    return labelMap[category as keyof typeof labelMap] || '';
  };

  if (loading) {
    return (
      <div className="max-w-[1400px] mx-auto">
        <div className="mb-6">
          <h1 className="text-3xl lg:text-4xl font-bold neon-text mb-2 flex items-center gap-2" style={{ color: 'var(--neon-purple)' }}>
            <Database size={24} weight="duotone" />
            Databases
          </h1>
          <p className="text-xs font-mono flex items-center gap-2" style={{ color: 'var(--text-secondary)' }}>
            <SpinnerGap size={14} weight="duotone" className="animate-spin" />
            Loading...
          </p>
        </div>
        <SkeletonLoader type="database" count={4} />
      </div>
    );
  }

  return (
    <div className="max-w-[1400px] mx-auto">
      <DbViewer />

      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-3xl lg:text-4xl font-bold neon-text mb-2 flex items-center gap-2" style={{ color: 'var(--neon-purple)' }}>
            <Database size={24} weight="duotone" />
            Databases
          </h1>
          <p className="text-xs font-mono" style={{ color: 'var(--text-secondary)' }}>
            Database management system
          </p>
        </div>
        <button
          onClick={() => setShowCreateForm(!showCreateForm)}
          className="btn-neon inline-flex items-center gap-2"
        >
          {showCreateForm ? (
            <span className="inline-flex items-center gap-2">
              <X size={14} weight="bold" />
              Cancel
            </span>
          ) : (
            <span className="inline-flex items-center gap-2">
              <Sparkle size={14} weight="duotone" />
              Create Database
            </span>
          )}
        </button>
      </div>

      {error && (
        <div className="mt-4 card-vapor p-4 rounded-lg border-2" style={{ borderColor: 'rgba(var(--status-error-rgb), 0.5)' }}>
          <p className="font-bold flex items-center gap-2" style={{ color: 'var(--status-error)' }}>
            <XCircle size={16} weight="duotone" />
            {error}
          </p>
        </div>
      )}

      {connectionInfo && (
        <div className="mt-4 card-vapor p-6 rounded-lg border-2" style={{ borderColor: 'rgba(var(--status-success-rgb), 0.5)' }}>
          <h3 className="text-lg font-bold mb-3 neon-text flex items-center gap-2" style={{ color: 'var(--neon-green)' }}>
            <Sparkle size={16} weight="duotone" />
            Database Created Successfully!
          </h3>
          <div className="space-y-2 font-mono text-sm" style={{ color: 'var(--text-primary)' }}>
            <div className="flex justify-between">
              <span className="inline-flex items-center gap-2" style={{ color: 'var(--neon-cyan)' }}>
                <Globe size={14} weight="duotone" />
                HOST:
              </span>
              <span>{connectionInfo.host}</span>
            </div>
            <div className="flex justify-between">
              <span className="inline-flex items-center gap-2" style={{ color: 'var(--neon-cyan)' }}>
                <Plug size={14} weight="duotone" />
                PORT:
              </span>
              <span>{connectionInfo.port}</span>
            </div>
            <div className="flex justify-between">
              <span className="inline-flex items-center gap-2" style={{ color: 'var(--neon-cyan)' }}>
                <Database size={14} weight="duotone" />
                DATABASE:
              </span>
              <span>{connectionInfo.database}</span>
            </div>
            <div className="flex justify-between">
              <span className="inline-flex items-center gap-2" style={{ color: 'var(--neon-cyan)' }}>
                <UserCircle size={14} weight="duotone" />
                USERNAME:
              </span>
              <span>{connectionInfo.username}</span>
            </div>
            <div className="flex justify-between">
              <span className="inline-flex items-center gap-2" style={{ color: 'var(--neon-cyan)' }}>
                <Key size={14} weight="duotone" />
                PASSWORD:
              </span>
              <span className="font-bold" style={{ color: 'var(--neon-pink)' }}>{connectionInfo.password}</span>
            </div>
          </div>
          <p className="mt-4 text-xs font-mono flex items-center gap-2" style={{ color: 'var(--neon-yellow)' }}>
            <WarningCircle size={14} weight="duotone" />
            Save these credentials - the password will not be shown again!
          </p>
          <button
            onClick={() => setConnectionInfo(null)}
            className="mt-4 px-3 py-1.5 text-sm rounded-lg font-bold transition-all hover:scale-105"
            style={{
              background: 'linear-gradient(135deg, var(--neon-green) 0%, var(--neon-cyan) 100%)',
              color: 'var(--button-text)',
            }}
          >
            <span className="inline-flex items-center gap-2">
              <ThumbsUp size={14} weight="duotone" />
              Got it!
            </span>
          </button>
        </div>
      )}

      {showCreateForm && (
        <div className="mt-6 card-vapor p-6 rounded-xl">
          <form onSubmit={handleCreate} className="space-y-4">
            
            {/* DB Type Selection */}
            <div className="flex gap-4 mb-4">
              <button
                type="button"
                onClick={() => setDbType('postgres')}
                className={`flex-1 p-3 rounded-lg border-2 font-bold transition-all flex items-center justify-center gap-2 ${
                  dbType === 'postgres'
                    ? 'border-[var(--neon-cyan)] bg-[rgba(var(--neon-cyan-rgb),0.1)] text-[var(--neon-cyan)]'
                    : 'border-transparent bg-[var(--surface-dim)] text-[var(--text-secondary)] hover:bg-[var(--surface-highlight)]'
                }`}
              >
                <Database size={20} weight={dbType === 'postgres' ? 'duotone' : 'regular'} />
                PostgreSQL
              </button>
              <button
                type="button"
                onClick={() => setDbType('sqlite')}
                className={`flex-1 p-3 rounded-lg border-2 font-bold transition-all flex items-center justify-center gap-2 ${
                  dbType === 'sqlite'
                    ? 'border-[var(--neon-purple)] bg-[rgba(var(--neon-purple-rgb),0.1)] text-[var(--neon-purple)]'
                    : 'border-transparent bg-[var(--surface-dim)] text-[var(--text-secondary)] hover:bg-[var(--surface-highlight)]'
                }`}
              >
                <Package size={20} weight={dbType === 'sqlite' ? 'duotone' : 'regular'} />
                SQLite
              </button>
            </div>

            <div>
              <label htmlFor="name" className="block text-sm font-bold mb-2 flex items-center gap-2" style={{ color: 'var(--neon-cyan)' }}>
                <Database size={16} weight="duotone" />
                DATABASE NAME
              </label>
              <input
                type="text"
                id="name"
                required
                value={newDbName}
                onChange={(e) => setNewDbName(e.target.value)}
                className="input-vapor w-full"
                placeholder="my_awesome_database"
              />
              <p className="mt-2 text-xs font-mono" style={{ color: 'var(--text-secondary)' }}>
                Only alphanumeric characters and underscores
              </p>
            </div>

            {dbType === 'postgres' ? (
              <>
                <div>
                  <label htmlFor="username" className="block text-sm font-bold mb-2 flex items-center gap-2" style={{ color: 'var(--neon-purple)' }}>
                    <UserCircle size={16} weight="duotone" />
                    USERNAME
                  </label>
                  <input
                    type="text"
                    id="username"
                    required
                    value={newDbUsername}
                    onChange={(e) => setNewDbUsername(e.target.value)}
                    className="input-vapor w-full"
                    placeholder="docklite"
                  />
                  <p className="mt-2 text-xs font-mono" style={{ color: 'var(--text-secondary)' }}>
                    Default: docklite
                  </p>
                </div>

                <div>
                  <label htmlFor="password" className="block text-sm font-bold mb-2 flex items-center gap-2" style={{ color: 'var(--neon-pink)' }}>
                    <Key size={16} weight="duotone" />
                    PASSWORD
                  </label>
                  <input
                    type="password"
                    id="password"
                    value={newDbPassword}
                    onChange={(e) => setNewDbPassword(e.target.value)}
                    className="input-vapor w-full"
                    placeholder="Leave empty for auto-generated password"
                  />
                  <p className="mt-2 text-xs font-mono" style={{ color: 'var(--text-secondary)' }}>
                    Leave empty to auto-generate a secure password
                  </p>
                </div>
              </>
            ) : (
              <>
                <div>
                  <label htmlFor="container" className="block text-sm font-bold mb-2 flex items-center gap-2" style={{ color: 'var(--neon-green)' }}>
                    <Package size={16} weight="duotone" />
                    HOST CONTAINER
                  </label>
                  {fetchingContainers ? (
                     <div className="flex items-center gap-2 text-sm text-[var(--text-secondary)]">
                       <SpinnerGap className="animate-spin" /> Loading containers...
                     </div>
                  ) : (
                    <select
                      id="container"
                      required
                      value={selectedContainerId}
                      onChange={(e) => setSelectedContainerId(e.target.value)}
                      className="input-vapor w-full"
                    >
                      <option value="">Select a container...</option>
                      {availableContainers.map((c) => (
                        <option key={c.id} value={c.id}>
                          {c.name} ({c.image})
                        </option>
                      ))}
                    </select>
                  )}
                  <p className="mt-2 text-xs font-mono" style={{ color: 'var(--text-secondary)' }}>
                    Select the container where the SQLite file will be stored
                  </p>
                </div>

                <div>
                  <label htmlFor="path" className="block text-sm font-bold mb-2 flex items-center gap-2" style={{ color: 'var(--neon-yellow)' }}>
                    <LinkSimple size={16} weight="duotone" />
                    FILE PATH
                  </label>
                  <input
                    type="text"
                    id="path"
                    required
                    value={sqlitePath}
                    onChange={(e) => setSqlitePath(e.target.value)}
                    className="input-vapor w-full"
                    placeholder="/app/data/db.sqlite"
                  />
                  <p className="mt-2 text-xs font-mono" style={{ color: 'var(--text-secondary)' }}>
                    Absolute path inside the container
                  </p>
                </div>
              </>
            )}

            <button
              type="submit"
              disabled={creating}
              className="btn-neon w-full disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {creating ? (
                <span className="inline-flex items-center gap-2">
                  <SpinnerGap size={16} weight="duotone" className="animate-spin" />
                  Creating...
                </span>
              ) : (
                <span className="inline-flex items-center gap-2">
                  <Sparkle size={16} weight="duotone" />
                  Create {dbType === 'postgres' ? 'PostgreSQL' : 'SQLite'} Database
                </span>
              )}
            </button>
          </form>
        </div>
      )}

      {/* PostgreSQL Databases */}
      <div className="mt-8">
        <h2 className="text-2xl font-bold neon-text mb-4 flex items-center gap-2" style={{ color: 'var(--neon-pink)' }}>
          <Database size={20} weight="duotone" />
          PostgreSQL Databases
        </h2>

        {databases.length === 0 ? (
          <div className="text-center py-12 card-vapor">
            <p className="text-lg font-bold mb-2 flex items-center justify-center gap-2" style={{ color: 'var(--neon-pink)' }}>
              <Package size={20} weight="duotone" />
              No PostgreSQL databases yet!
            </p>
            <p className="text-sm font-mono flex items-center justify-center gap-2" style={{ color: 'var(--text-secondary)' }}>
              <Sparkle size={14} weight="duotone" />
              Create your first database to get started
            </p>
          </div>
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
            {databases.map((db) => (
              <div
                key={db.id}
                className="p-6 rounded-xl transition-all hover:scale-[1.02] relative"
                style={{
                  background: 'var(--surface-dim)',
                  backdropFilter: 'blur(12px)',
                  border: '2px solid var(--neon-green)',
                  boxShadow: `
                    0 0 3px rgba(var(--neon-green-rgb), 1),
                    0 0 6px rgba(var(--neon-green-rgb), 0.7),
                    0 0 12px rgba(var(--neon-green-rgb), 0.5),
                    0 0 18px rgba(var(--neon-green-rgb), 0.35),
                    inset 0 0 2px rgba(var(--neon-green-rgb), 0.9),
                    inset 0 0 4px rgba(var(--neon-green-rgb), 0.6),
                    inset 0 0 8px rgba(var(--neon-green-rgb), 0.4)
                  `,
                }}
              >
                {/* 3-dot menu */}
                <div className="absolute top-4 right-4 z-10" data-db-menu="true">
                  <button
                    type="button"
                    onClick={(e) => {
                      e.stopPropagation();
                      const nextOpen = menuDbId === db.id ? null : db.id;
                      setMenuDbId(nextOpen);
                      if (nextOpen !== null && typeof window !== 'undefined') {
                        const rect = (e.currentTarget as HTMLButtonElement).getBoundingClientRect();
                        const menuWidth = 200;
                        const left = Math.max(12, Math.min(rect.right - menuWidth, window.innerWidth - menuWidth - 12));
                        setMenuPosition({ top: rect.bottom + 8, left });
                      } else {
                        setMenuPosition(null);
                      }
                    }}
                    onMouseDown={stopMenu}
                    className="p-2 rounded-lg text-sm font-bold transition-all hover:scale-105"
                    style={{
                      background: 'transparent',
                      border: '2px solid var(--neon-purple)',
                      color: 'var(--neon-purple)',
                      boxShadow: `
                        0 0 5px rgba(var(--neon-purple-rgb), 0.6),
                        0 0 10px rgba(var(--neon-purple-rgb), 0.3)
                      `,
                    }}
                    title="Database actions"
                  >
                    <DotsThree size={16} weight="bold" />
                  </button>

                  {menuDbId === db.id && menuPosition && typeof document !== 'undefined' &&
                    createPortal(
                      <div
                        className="fixed inset-0 z-[10000]"
                        onMouseDown={(e) => {
                          if (e.target !== e.currentTarget) return;
                          setMenuDbId(null);
                          setMenuPosition(null);
                        }}
                      >
                        <div
                          ref={menuPopupRef}
                          data-db-menu="true"
                          className="absolute rounded-lg overflow-hidden animate-slide-down"
                          style={{
                            top: menuPosition.top,
                            left: menuPosition.left,
                            background: 'linear-gradient(135deg, var(--modal-bg-1) 0%, var(--modal-bg-2) 100%)',
                            border: '1px solid var(--neon-purple)',
                            boxShadow: '0 0 20px rgba(var(--neon-purple-rgb), 0.4)',
                            width: '200px',
                            maxWidth: 'calc(100vw - 24px)',
                          }}
                          onClick={(e) => e.stopPropagation()}
                          onMouseDown={stopMenu}
                        >
                          <button
                            type="button"
                            onClick={(e) => {
                              console.log('Edit Credentials button clicked');
                              e.stopPropagation();
                              setMenuDbId(null);
                              setMenuPosition(null);
                              setTimeout(() => openEditModal(db), 0);
                            }}
                            onMouseDown={stopMenu}
                            className="w-full px-4 py-3 text-left text-sm font-bold transition-all hover:bg-white/5 flex items-center gap-3"
                            style={{ color: 'var(--neon-cyan)' }}
                          >
                            <PencilSimpleLine size={16} weight="duotone" />
                            Edit Credentials
                          </button>
                          <button
                            type="button"
                            onClick={(e) => {
                              console.log('Edit Database Mode button clicked');
                              e.stopPropagation();
                              setMenuDbId(null);
                              setMenuPosition(null);
                              setTimeout(() => openEditModeModal(db), 0);
                            }}
                            onMouseDown={stopMenu}
                            className="w-full px-4 py-3 text-left text-sm font-bold transition-all hover:bg-white/5 flex items-center gap-3"
                            style={{ color: 'var(--neon-purple)' }}
                          >
                            <SignIn size={16} weight="duotone" />
                            Edit Database Mode
                          </button>
                          <button
                            type="button"
                            onClick={(e) => {
                              console.log('Delete Database button clicked');
                              e.stopPropagation();
                              setMenuDbId(null);
                              setMenuPosition(null);
                              setTimeout(() => openDeleteModal(db), 0);
                            }}
                            onMouseDown={stopMenu}
                            className="w-full px-4 py-3 text-left text-sm font-bold transition-all flex items-center gap-3 hover:bg-white/5"
                            style={{ color: 'var(--status-error)' }}
                          >
                            <Trash size={16} weight="duotone" />
                            Delete Database
                          </button>
                          <button
                            type="button"
                            onClick={(e) => {
                              console.log('Download Database button clicked');
                              e.stopPropagation();
                              setMenuDbId(null);
                              setMenuPosition(null);
                              setTimeout(() => openDownloadModal(db), 0);
                            }}
                            onMouseDown={stopMenu}
                            className="w-full px-4 py-3 text-left text-sm font-bold transition-all hover:bg-white/5 flex items-center gap-3"
                            style={{ color: 'var(--neon-green)' }}
                          >
                            <DownloadSimple size={16} weight="duotone" />
                            Download Database
                          </button>
                        </div>
                      </div>,
                      document.body
                    )}
                </div>

                {/* Header with centered database icon */}
                <div className="mb-4">
                  <h3 className="docklite-db-title text-xl font-bold neon-text truncate" style={{ color: 'var(--neon-cyan)' }}>
                    {db.name}
                  </h3>
                  <div className="mt-3 flex justify-center pointer-events-none">
                    <Database
                      size={46}
                      weight="duotone"
                      className="animate-wobble"
                      style={{
                        color: 'var(--neon-cyan)',
                        filter: 'drop-shadow(0 0 8px rgba(var(--neon-cyan-rgb), 0.7)) drop-shadow(0 0 14px rgba(var(--neon-cyan-rgb), 0.45))',
                      }}
                    />
                  </div>
                </div>

                {/* Size badge */}
                <div className="mb-4">
                  <span
                    className="inline-block px-3 py-1 rounded-full text-xs font-bold"
                    style={{
                      background: db.size === 0
                        ? 'rgba(var(--text-muted-rgb), 0.2)'
                        : 'rgba(var(--status-success-rgb), 0.2)',
                      color: db.size === 0 ? 'var(--text-muted)' : 'var(--neon-green)',
                      border: `1px solid ${db.size === 0 ? 'var(--text-muted)' : 'var(--neon-green)'}`,
                    }}
                  >
                            {db.size === 0 ? '○ EMPTY' : `● ${formatBytes(db.size)}`} {getSizeLabel(db.sizeCategory)}
                  </span>
                </div>

                {/* Info */}
                <div className="space-y-2 text-sm font-mono">
                  {db.type === 'sqlite' ? (
                    <div className="flex items-center gap-2">
                      <span className="opacity-60 flex items-center gap-2" style={{ color: 'var(--text-secondary)' }}>
                        <LinkSimple size={12} weight="duotone" />
                        Path:
                      </span>
                      <span className="font-bold truncate" title={db.db_path} style={{ color: 'var(--neon-yellow)' }}>
                        {db.db_path}
                      </span>
                    </div>
                  ) : (
                    <div className="flex items-center gap-2">
                      <span className="opacity-60 flex items-center gap-2" style={{ color: 'var(--text-secondary)' }}>
                        <Plug size={12} weight="duotone" />
                        Port:
                      </span>
                      <span className="font-bold" style={{ color: 'var(--neon-purple)' }}>
                        {db.postgres_port}
                      </span>
                    </div>
                  )}
                  <div className="flex items-center gap-2">
                    <span className="opacity-60 flex items-center gap-2" style={{ color: 'var(--text-secondary)' }}>
                      <Info size={12} weight="duotone" />
                      ID:
                    </span>
                    <span className="opacity-70 text-xs">
                      {db.container_id ? db.container_id.substring(0, 12) : `sqlite-${db.id}`}
                    </span>
                  </div>
                  <div className="flex items-center gap-2">
                    <span className="opacity-60 flex items-center gap-2" style={{ color: 'var(--text-secondary)' }}>
                      <CalendarBlank size={12} weight="duotone" />
                      Created:
                    </span>
                    <span className="opacity-70 text-xs">
                      {new Date(db.created_at).toLocaleDateString()}
                    </span>
                  </div>
                </div>

                {/* Connection hint */}
                <div className="mt-4 pt-4 border-t" style={{ borderColor: 'rgba(var(--neon-purple-rgb), 0.2)' }}>
                  {db.type === 'sqlite' ? (
                    <p className="text-xs font-mono opacity-60 mb-3" style={{ color: 'var(--text-secondary)' }}>
                      File-based database in container
                    </p>
                  ) : (
                    <p className="text-xs font-mono opacity-60 mb-3" style={{ color: 'var(--text-secondary)' }}>
                      Connect: localhost:{db.postgres_port}
                    </p>
                  )}
                  <p className="text-xs font-mono opacity-60" style={{ color: 'var(--text-secondary)' }}>
                    Use the menu for credentials or deletion.
                  </p>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      <div className="mt-8 card-vapor p-6 rounded-xl border-2" style={{ borderColor: 'rgba(var(--neon-cyan-rgb), 0.3)' }}>
        <div className="flex items-start gap-3">
          <Lightbulb size={22} weight="duotone" />
          <div>
            <p className="text-sm font-bold mb-2 flex items-center gap-2" style={{ color: 'var(--neon-cyan)' }}>
              <LinkSimple size={14} weight="duotone" />
              CONNECTION INFO
            </p>
            <p className="text-sm font-mono" style={{ color: 'var(--text-secondary)' }}>
              Connect to your PostgreSQL databases using <span className="font-bold" style={{ color: 'var(--neon-pink)' }}>localhost</span> as the host and the port shown on each card.
              Default username is <span className="font-bold" style={{ color: 'var(--neon-pink)' }}>docklite</span>. Password is generated randomly during creation.
            </p>
          </div>
        </div>
      </div>

      {/* Edit Database Modal */}
      {editingDb && (
        <div className="fixed inset-0 bg-black/80 backdrop-blur-sm flex items-center justify-center z-[10000] p-4">
          <div className="card-vapor max-w-lg w-full p-8">
            <div className="mb-6">
              <h2 className="text-2xl font-bold neon-text mb-2 flex items-center gap-2" style={{ color: 'var(--neon-cyan)' }}>
                <PencilSimpleLine size={18} weight="duotone" />
                Edit Database Credentials
              </h2>
              <p className="text-sm font-mono opacity-70" style={{ color: 'var(--text-secondary)' }}>
                {editingDb.name}
              </p>
            </div>

            <form onSubmit={handleEdit} className="space-y-4">
              <div>
                <label htmlFor="edit-username" className="block text-sm font-bold mb-2 flex items-center gap-2" style={{ color: 'var(--neon-purple)' }}>
                  <UserCircle size={16} weight="duotone" />
                  NEW USERNAME
                </label>
                <input
                  type="text"
                  id="edit-username"
                  required
                  value={editUsername}
                  onChange={(e) => setEditUsername(e.target.value)}
                  className="input-vapor w-full"
                  placeholder="docklite"
                />
              </div>

              <div>
                <label htmlFor="edit-password" className="block text-sm font-bold mb-2 flex items-center gap-2" style={{ color: 'var(--neon-pink)' }}>
                  <Key size={16} weight="duotone" />
                  NEW PASSWORD
                </label>
                <input
                  type="password"
                  id="edit-password"
                  required
                  value={editPassword}
                  onChange={(e) => setEditPassword(e.target.value)}
                  className="input-vapor w-full"
                  placeholder="Enter new password"
                />
              </div>

              <div className="p-4 rounded-lg" style={{ background: 'rgba(var(--status-warning-rgb), 0.1)', border: '1px solid rgba(var(--status-warning-rgb), 0.3)' }}>
                <p className="text-xs font-mono flex items-center gap-2" style={{ color: 'var(--neon-yellow)' }}>
                  <WarningCircle size={14} weight="duotone" />
                  This will update the PostgreSQL user credentials in the database container.
                  Make sure to update your application connection strings!
                </p>
              </div>

              <div className="flex gap-4">
                <button
                  type="button"
                  onClick={() => {
                    setEditingDb(null);
                    setEditUsername('');
                    setEditPassword('');
                  }}
                  className="flex-1 px-4 py-3 rounded-lg font-bold transition-all hover:scale-105"
                  style={{
                    background: 'rgba(var(--text-muted-rgb), 0.3)',
                    border: '2px solid var(--text-secondary)',
                    color: 'var(--text-secondary)',
                  }}
                >
                  <span className="inline-flex items-center gap-2">
                    <X size={14} weight="bold" />
                    Cancel
                  </span>
                </button>
                <button
                  type="submit"
                  disabled={creating}
                  className="flex-1 px-4 py-3 rounded-lg font-bold transition-all hover:scale-105 disabled:opacity-50 disabled:cursor-not-allowed"
                  style={{
                    background: 'linear-gradient(135deg, var(--neon-cyan) 0%, var(--neon-purple) 100%)',
                    color: 'var(--button-text)',
                  }}
                >
                  {creating ? (
                    <span className="inline-flex items-center gap-2">
                      <SpinnerGap size={14} weight="duotone" className="animate-spin" />
                      Updating...
                    </span>
                  ) : (
                    <span className="inline-flex items-center gap-2">
                      <Sparkle size={14} weight="duotone" />
                      Update Credentials
                    </span>
                  )}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Delete Database Modal */}
      {deleteDb && (
        <div className="fixed inset-0 bg-black/80 backdrop-blur-sm flex items-center justify-center z-[10000] p-4">
          <div className="card-vapor max-w-lg w-full p-8 border-2" style={{ borderColor: 'var(--status-error)' }}>
            <div className="mb-6 text-center">
              <div className="flex justify-center mb-4">
                <WarningCircle size={36} weight="duotone" color="var(--status-error)" />
              </div>
              <h2 className="text-2xl font-bold neon-text mb-2" style={{ color: 'var(--status-error)' }}>
                Delete Database
              </h2>
              <p className="text-sm font-mono opacity-80" style={{ color: 'var(--text-secondary)' }}>
                This deletes the PostgreSQL container and all data. This cannot be undone.
              </p>
            </div>

            <div className="space-y-4">
              <div className="p-4 rounded-lg border" style={{ background: 'rgba(var(--status-error-rgb), 0.08)', borderColor: 'rgba(var(--status-error-rgb), 0.4)' }}>
                <p className="text-xs font-mono" style={{ color: 'var(--status-error)' }}>
                  Type <span className="font-bold">{deleteDb.name}</span> to confirm deletion.
                </p>
              </div>

              <div>
                <label htmlFor="delete-confirm" className="block text-sm font-bold mb-2" style={{ color: 'var(--neon-pink)' }}>
                  Confirm Database Name
                </label>
                <input
                  id="delete-confirm"
                  type="text"
                  value={deleteConfirmText}
                  onChange={(e) => setDeleteConfirmText(e.target.value)}
                  className="input-vapor w-full"
                  placeholder={deleteDb.name}
                />
              </div>

              <div className="flex gap-4">
                <button
                  type="button"
                  onClick={() => {
                    setDeleteDb(null);
                    setDeleteConfirmText('');
                  }}
                  className="flex-1 px-4 py-3 rounded-lg font-bold transition-all hover:scale-105"
                  style={{
                    background: 'rgba(var(--text-muted-rgb), 0.3)',
                    border: '2px solid var(--text-secondary)',
                    color: 'var(--text-secondary)',
                  }}
                >
                  <span className="inline-flex items-center gap-2">
                    <X size={14} weight="bold" />
                    Cancel
                  </span>
                </button>
                <button
                  type="button"
                  disabled={deletingDb || deleteConfirmText !== deleteDb.name}
                  onClick={handleDelete}
                  className="flex-1 px-4 py-3 rounded-lg font-bold transition-all hover:scale-105 disabled:opacity-50 disabled:cursor-not-allowed"
                  style={{
                    background: 'linear-gradient(135deg, var(--status-error) 0%, var(--neon-pink) 100%)',
                    color: 'var(--button-text)',
                  }}
                >
                  {deletingDb ? (
                    <span className="inline-flex items-center gap-2">
                      <SpinnerGap size={14} weight="duotone" className="animate-spin" />
                      Deleting...
                    </span>
                  ) : (
                    'Delete Database'
                  )}
                </button>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Enter Edit Mode Modal */}
      {editModeDb && (
        <div className="fixed inset-0 bg-black/80 backdrop-blur-sm flex items-center justify-center z-[10000] p-4">
          <div className="card-vapor max-w-lg w-full p-8 border-2" style={{ borderColor: 'var(--neon-purple)' }}>
            <div className="mb-6 text-center">
              <div className="flex justify-center mb-4">
                <Brain size={36} weight="duotone" color="var(--neon-cyan)" />
              </div>
              <h2 className="text-2xl font-bold neon-text mb-2" style={{ color: 'var(--neon-cyan)' }}>
                DO YOU WANT TO EDIT MODE?
              </h2>
              <p className="text-sm font-mono opacity-80" style={{ color: 'var(--text-secondary)' }}>
                Enter database credentials to access live schema tools and SQL runner.
              </p>
            </div>

            <div className="space-y-4">
              <div>
                <label htmlFor="edit-mode-username" className="block text-sm font-bold mb-2 flex items-center gap-2" style={{ color: 'var(--neon-purple)' }}>
                  <UserCircle size={16} weight="duotone" />
                  DATABASE USERNAME
                </label>
                <input
                  id="edit-mode-username"
                  type="text"
                  value={editModeUsername}
                  onChange={(e) => setEditModeUsername(e.target.value)}
                  className="input-vapor w-full"
                  placeholder="docklite"
                />
              </div>

              <div>
                <label htmlFor="edit-mode-password" className="block text-sm font-bold mb-2 flex items-center gap-2" style={{ color: 'var(--neon-pink)' }}>
                  <Key size={16} weight="duotone" />
                  DATABASE PASSWORD
                </label>
                <input
                  id="edit-mode-password"
                  type="password"
                  value={editModePassword}
                  onChange={(e) => setEditModePassword(e.target.value)}
                  className="input-vapor w-full"
                  placeholder="Enter database password"
                />
              </div>

              <div className="flex gap-4">
                <button
                  type="button"
                  onClick={() => {
                    setEditModeDb(null);
                    setEditModeUsername('');
                    setEditModePassword('');
                  }}
                  className="flex-1 px-4 py-3 rounded-lg font-bold transition-all hover:scale-105"
                  style={{
                    background: 'rgba(var(--text-muted-rgb), 0.3)',
                    border: '2px solid var(--text-secondary)',
                    color: 'var(--text-secondary)',
                  }}
                >
                  <span className="inline-flex items-center gap-2">
                    <X size={14} weight="bold" />
                    Cancel
                  </span>
                </button>
                <button
                  type="button"
                  onClick={handleEnterEditMode}
                  className="flex-1 px-4 py-3 rounded-lg font-bold transition-all hover:scale-105"
                  style={{
                    background: 'linear-gradient(135deg, var(--neon-cyan) 0%, var(--neon-purple) 100%)',
                    color: 'var(--button-text)',
                  }}
                >
                  <span className="inline-flex items-center gap-2">
                    <Sparkle size={14} weight="duotone" />
                    Enter Edit Mode
                  </span>
                </button>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Download Database Modal */}
      {downloadDb && (
        <div className="fixed inset-0 bg-black/80 backdrop-blur-sm flex items-center justify-center z-[10000] p-4">
          <div className="card-vapor max-w-lg w-full p-8 border-2" style={{ borderColor: 'var(--neon-green)' }}>
            <div className="mb-6 text-center">
              <div className="flex justify-center mb-4">
                <ArrowDown size={36} weight="duotone" color="var(--neon-green)" />
              </div>
              <h2 className="text-2xl font-bold neon-text mb-2" style={{ color: 'var(--neon-green)' }}>
                Download Database
              </h2>
              <p className="text-sm font-mono opacity-80" style={{ color: 'var(--text-secondary)' }}>
                {downloadDb.name}
              </p>
            </div>

            <div className="space-y-4">
              <div>
                <label htmlFor="download-username" className="block text-sm font-bold mb-2 flex items-center gap-2" style={{ color: 'var(--neon-purple)' }}>
                  <UserCircle size={16} weight="duotone" />
                  DATABASE USERNAME
                </label>
                <input
                  id="download-username"
                  type="text"
                  value={downloadUsername}
                  onChange={(e) => setDownloadUsername(e.target.value)}
                  className="input-vapor w-full"
                  placeholder="docklite"
                />
              </div>

              <div>
                <label htmlFor="download-password" className="block text-sm font-bold mb-2 flex items-center gap-2" style={{ color: 'var(--neon-pink)' }}>
                  <Key size={16} weight="duotone" />
                  DATABASE PASSWORD
                </label>
                <input
                  id="download-password"
                  type="password"
                  value={downloadPassword}
                  onChange={(e) => setDownloadPassword(e.target.value)}
                  className="input-vapor w-full"
                  placeholder="Enter database password"
                />
              </div>

              <label className="flex items-center gap-3 text-sm font-bold" style={{ color: 'var(--neon-green)' }}>
                <input
                  type="checkbox"
                  checked={downloadGzip}
                  onChange={(e) => setDownloadGzip(e.target.checked)}
                  className="h-4 w-4"
                  style={{ accentColor: 'var(--neon-green)' }}
                />
                Gzip compress download
              </label>

              <div className="p-4 rounded-lg border" style={{ background: 'rgba(var(--status-warning-rgb), 0.1)', borderColor: 'rgba(var(--status-warning-rgb), 0.35)' }}>
                <p className="text-xs font-mono flex items-center gap-2" style={{ color: 'var(--neon-yellow)' }}>
                  <WarningCircle size={14} weight="duotone" />
                  This backs up the database only. Files/uploads are separate.
                </p>
              </div>

              <div className="flex gap-4">
                <button
                  type="button"
                  onClick={() => {
                    setDownloadDb(null);
                    setDownloadUsername('');
                    setDownloadPassword('');
                    setDownloadGzip(true);
                  }}
                  className="flex-1 px-4 py-3 rounded-lg font-bold transition-all hover:scale-105"
                  style={{
                    background: 'rgba(var(--text-muted-rgb), 0.3)',
                    border: '2px solid var(--text-secondary)',
                    color: 'var(--text-secondary)',
                  }}
                >
                  <span className="inline-flex items-center gap-2">
                    <X size={14} weight="bold" />
                    Cancel
                  </span>
                </button>
                <button
                  type="button"
                  onClick={handleDownload}
                  disabled={downloadingDb}
                  className="flex-1 px-4 py-3 rounded-lg font-bold transition-all hover:scale-105 disabled:opacity-50 disabled:cursor-not-allowed"
                  style={{
                    background: 'linear-gradient(135deg, var(--neon-green) 0%, var(--neon-cyan) 100%)',
                    color: 'var(--button-text)',
                  }}
                >
                  {downloadingDb ? (
                    <span className="inline-flex items-center gap-2">
                      <SpinnerGap size={14} weight="duotone" className="animate-spin" />
                      Preparing...
                    </span>
                  ) : (
                    <span className="inline-flex items-center gap-2">
                      <ArrowDown size={14} weight="duotone" />
                      Download Dump
                    </span>
                  )}
                </button>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
