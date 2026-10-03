
'use client';

import { useEffect, useState } from 'react';
import { Database, Eye, X, SpinnerGap, XCircle, ChartBar, Key } from '@phosphor-icons/react';
import ConfirmModal from '../components/ConfirmModal';
import { useToast } from '@/lib/hooks/useToast';

interface TableInfo {
  name: string;
  schema: any[];
  data?: any[];
  count?: number;
}

export default function DbViewer() {
  const [dbInfo, setDbInfo] = useState<TableInfo[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [showCleanupConfirm, setShowCleanupConfirm] = useState(false);
  const [cleanupLoading, setCleanupLoading] = useState(false);
  const toast = useToast();

  useEffect(() => {
    fetchDbInfo();
  }, []);

  const fetchDbInfo = async () => {
    try {
      setLoading(true);
      const res = await fetch('/api/db');
      if (!res.ok) {
        const data = await res.json().catch(() => ({}));
        throw new Error(data.error || 'Failed to fetch database info');
      }
      const data = await res.json();
      setDbInfo(data.dbInfo);
    } catch (err: any) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  const handleCleanup = async () => {
    setCleanupLoading(true);
    try {
      const res = await fetch('/api/db/cleanup', { method: 'POST' });
      const data = await res.json();
      if (!res.ok) {
        throw new Error(data.error || 'Failed to clean up database');
      }
      toast.success(`Cleanup complete: ${data.removed?.sites || 0} sites, ${data.removed?.databases || 0} databases.`);
      await fetchDbInfo();
    } catch (err: any) {
      toast.error(err.message || 'Failed to clean up database');
    } finally {
      setCleanupLoading(false);
      setShowCleanupConfirm(false);
    }
  };

  const getSafeValue = (val: any) => {
    if (val === null || val === undefined || val === '') return '-';
    if (typeof val === 'string' && val.length > 50) return val.substring(0, 50) + '...';
    return String(val);
  };

  return (
    <>
      {/* Clickable Card */}
      <button
        onClick={() => setIsModalOpen(true)}
        className="w-full mb-6 card-vapor p-6 rounded-xl border border-cyan-500/30 hover:border-cyan-500/60 transition-all hover:scale-[1.02] text-left"
      >
        <div className="flex items-center justify-between">
          <div>
            <h2 className="text-xl font-bold neon-text mb-2 flex items-center gap-2" style={{ color: 'var(--neon-cyan)' }}>
              <Database size={20} weight="duotone" />
              DockLite System Database
            </h2>
            <p className="text-xs font-mono opacity-70" style={{ color: 'var(--text-secondary)' }}>
              Click to inspect internal database structure
            </p>
          </div>
          <div className="text-3xl">
            <Eye size={26} weight="duotone" />
          </div>
        </div>
      </button>

      {/* Modal */}
      {isModalOpen && (
        <div
          className="fixed inset-0 z-[10000] flex items-center justify-center p-4"
          style={{ background: 'rgba(0, 0, 0, 0.8)' }}
          onClick={() => setIsModalOpen(false)}
        >
          <div
            className="card-vapor rounded-xl border-2 border-cyan-500 max-w-6xl w-full max-h-[90vh] overflow-hidden flex flex-col"
            onClick={(e) => e.stopPropagation()}
          >
            {/* Modal Header */}
            <div className="p-6 border-b border-purple-500/30 flex items-center justify-between">
              <div>
                <h2 className="text-2xl font-bold neon-text flex items-center gap-2" style={{ color: 'var(--neon-cyan)' }}>
                  <Database size={22} weight="duotone" />
                  DockLite Database Inspector
                </h2>
                <p className="text-xs font-mono mt-1" style={{ color: 'var(--text-secondary)' }}>
                  Internal SQLite database structure
                </p>
              </div>
              <div className="flex items-center gap-3">
                <button
                  onClick={() => setShowCleanupConfirm(true)}
                  disabled={cleanupLoading}
                  className="cyber-button-sm disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  {cleanupLoading ? 'Cleaning...' : 'Clean Up Database'}
                </button>
                <button
                  onClick={() => setIsModalOpen(false)}
                  className="px-4 py-2 rounded-lg font-bold transition-all hover:scale-105"
                  style={{
                    background: 'linear-gradient(135deg, var(--status-error) 0%, var(--neon-pink) 100%)',
                    color: 'white',
                  }}
                >
                  <span className="inline-flex items-center gap-2">
                    <X size={14} weight="bold" />
                    Close
                  </span>
                </button>
              </div>
            </div>

            {/* Modal Content */}
            <div className="flex-1 overflow-y-auto p-6">
              {loading && (
                <div className="text-center py-12">
                  <div className="text-2xl font-bold neon-text animate-pulse flex items-center justify-center gap-2" style={{ color: 'var(--neon-cyan)' }}>
                    <SpinnerGap size={20} weight="duotone" className="animate-spin" />
                    Loading...
                  </div>
                </div>
              )}

              {error && (
                <div className="card-vapor p-4 rounded-lg border-2" style={{ borderColor: 'rgba(var(--status-error-rgb), 0.5)' }}>
                  <p className="font-bold flex items-center gap-2" style={{ color: 'var(--status-error)' }}>
                    <XCircle size={16} weight="duotone" />
                    Error: {error}
                  </p>
                </div>
              )}

              {!loading && !error && (
                <div className="space-y-6">
                  {dbInfo.map(table => {
                    const visibleColumns = table.schema.filter(col => !['password_hash'].includes(col.name));
                    const rows = Array.isArray(table.data) ? table.data : [];
                    const rowCount = typeof table.count === 'number' ? table.count : rows.length;

                    return (
                      <div key={table.name} className="card-vapor p-6 rounded-xl border border-purple-500/30">
                        <h3 className="text-xl font-bold neon-text mb-4 flex items-center gap-2" style={{ color: 'var(--neon-pink)' }}>
                          <ChartBar size={16} weight="duotone" />
                          <span>{table.name}</span>
                          <span className="text-xs font-mono opacity-70" style={{ color: 'var(--text-secondary)' }}>
                            ({rowCount} rows)
                          </span>
                          {rows.length > 0 && (
                            <span className="text-[10px] font-mono opacity-60" style={{ color: 'var(--text-secondary)' }}>
                              showing first {rows.length}
                            </span>
                          )}
                        </h3>

                        {/* Data Table */}
                        <div className="overflow-x-auto">
                          <table className="w-full text-sm">
                            <thead>
                              <tr className="border-b-2" style={{ borderColor: 'rgba(var(--neon-purple-rgb), 0.3)' }}>
                                {visibleColumns.map((col: any) => (
                                  <th
                                    key={col.name}
                                    className="px-3 py-2 text-left font-bold"
                                    style={{ color: 'var(--neon-purple)' }}
                                  >
                                    {col.name}
                                    {col.pk && <Key size={12} weight="duotone" className="ml-1 inline" />}
                                  </th>
                                ))}
                              </tr>
                            </thead>
                            <tbody>
                              {rows.length === 0 ? (
                                <tr>
                                  <td
                                    colSpan={visibleColumns.length}
                                    className="px-3 py-4 text-center opacity-50"
                                    style={{ color: 'var(--text-secondary)' }}
                                  >
                                    No sample rows to display.
                                  </td>
                                </tr>
                              ) : (
                                rows.map((row: any, i: number) => (
                                  <tr
                                    key={i}
                                    className="border-b transition-colors hover:bg-purple-900/20"
                                    style={{ borderColor: 'rgba(var(--neon-purple-rgb), 0.1)' }}
                                  >
                                    {visibleColumns.map((col: any) => (
                                      <td
                                        key={col.name}
                                        className="px-3 py-2 font-mono text-xs"
                                        style={{ color: 'var(--text-primary)' }}
                                      >
                                        {getSafeValue(row[col.name])}
                                      </td>
                                    ))}
                                  </tr>
                                ))
                              )}
                            </tbody>
                          </table>
                        </div>
                      </div>
                    );
                  })}
                </div>
              )}
            </div>
          </div>
        </div>
      )}
      {showCleanupConfirm && (
        <ConfirmModal
          title="Clean Up Database"
          message="This will remove orphaned site and database records that no longer have containers."
          confirmText="Clean Up"
          cancelText="Cancel"
          onConfirm={handleCleanup}
          onCancel={() => setShowCleanupConfirm(false)}
          type="warning"
        />
      )}
      <toast.ToastContainer />
    </>
  );
}
