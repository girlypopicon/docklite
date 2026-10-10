'use client';

import { useEffect, useMemo, useRef, useState } from 'react';
import { usePathname } from 'next/navigation';

interface ColumnInfo {
  name: string;
  type: string;
  nullable: boolean;
  default?: string | null;
  key?: string | null;
}

interface TableInfo {
  name: string;
  columns: ColumnInfo[];
}

interface SchemaResponse {
  tables: TableInfo[];
}

const STORAGE_PREFIX = 'docklite-db-edit-';

function getDbId(pathname: string | null): string | null {
  if (!pathname) return null;
  const match = pathname.match(/^\/databases\/(\d+)\/edit\/?/);
  return match?.[1] || null;
}

export default function SchemaBrowser() {
  const [tables, setTables] = useState<TableInfo[]>([]);
  const [expandedTables, setExpandedTables] = useState<Record<string, boolean>>({});
  const [selectedTable, setSelectedTable] = useState<string | null>(null);
  const [dbName, setDbName] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const hasAutoSelected = useRef(false);

  const pathname = usePathname();
  const dbId = useMemo(() => getDbId(pathname), [pathname]);

  useEffect(() => {
    if (!dbId) return;
    const fetchDb = async () => {
      const res = await fetch(`/api/databases/${dbId}`);
      if (!res.ok) return;
      const data = await res.json();
      if (data?.database?.name) {
        setDbName(data.database.name);
      }
    };
    fetchDb();
  }, [dbId]);

  useEffect(() => {
    if (!dbId) {
      setLoading(false);
      setError('Missing database context.');
      return;
    }

    const rawAuth = sessionStorage.getItem(`${STORAGE_PREFIX}${dbId}`);
    if (!rawAuth) {
      setLoading(false);
      setError('Missing database credentials. Return to the databases page to enter edit mode.');
      return;
    }

    const auth = JSON.parse(rawAuth) as { username: string; password: string };

    const fetchSchema = async () => {
      try {
        setLoading(true);
        const res = await fetch(`/api/databases/${dbId}/schema`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(auth),
        });

        if (res.status === 401) {
          if (dbId) {
            sessionStorage.removeItem(`${STORAGE_PREFIX}${dbId}`);
          }
          setError('Invalid database credentials. Return to the databases page to enter edit mode.');
          return;
        }
        if (!res.ok) {
          const data = await res.json().catch(() => ({}));
          throw new Error(data.error || 'Failed to load schema');
        }

        const data = (await res.json()) as SchemaResponse;
        const nextTables = data.tables || [];
        setTables(nextTables);

        if (!hasAutoSelected.current && nextTables.length > 0) {
          const firstTable = nextTables[0].name;
          setSelectedTable(firstTable);
          hasAutoSelected.current = true;
          window.dispatchEvent(
            new CustomEvent('docklite-db-select-table', { detail: { table: firstTable } })
          );
        }
      } catch (err: any) {
        setError(err.message || 'Failed to load schema');
      } finally {
        setLoading(false);
      }
    };

    fetchSchema();
  }, [dbId]);

  if (loading) {
    return (
      <div className="p-4 text-xs font-mono" style={{ color: 'var(--text-cyan)' }}>
        ⟳ Loading schema...
      </div>
    );
  }

  if (error) {
    return (
      <div className="p-4 text-xs font-mono" style={{ color: 'var(--status-error)' }}>
        {error}
      </div>
    );
  }

  return (
    <div className="p-4 space-y-4">
      <div className="text-sm font-bold" style={{ color: 'var(--text-cyan)' }}>
        🧬 Schema Browser
      </div>
      <div
        className="text-[11px] font-mono px-3 py-2 rounded-lg"
        style={{
          color: 'var(--text-secondary)',
          background: 'rgba(var(--neon-cyan-rgb), 0.06)',
          border: '1px solid rgba(var(--neon-cyan-rgb), 0.2)',
        }}
      >
        Database: <span style={{ color: 'var(--text-cyan)' }}>{dbName || `#${dbId}`}</span>
      </div>
      {tables.length === 0 ? (
        <div className="text-xs font-mono opacity-70" style={{ color: 'var(--text-secondary)' }}>
          No tables found.
        </div>
      ) : (
        <div className="space-y-2">
          {tables.map((table) => {
            const isExpanded = !!expandedTables[table.name];
            const isActive = selectedTable === table.name;
            return (
              <div key={table.name} className="text-xs font-mono">
                <button
                  type="button"
                  className="w-full flex items-center gap-2 px-3 py-2 rounded-md transition-all"
                  style={{
                    background: isActive ? 'rgba(var(--neon-cyan-rgb), 0.3)' : 'rgba(var(--neon-cyan-rgb), 0.1)',
                    border: '1px solid rgba(var(--neon-cyan-rgb), 0.3)',
                    color: 'var(--text-primary)',
                    boxShadow: isActive ? '0 0 12px rgba(var(--neon-cyan-rgb), 0.4)' : 'none',
                  }}
                  onClick={() => {
                    setExpandedTables((prev) => ({ ...prev, [table.name]: !isExpanded }));
                    setSelectedTable(table.name);
                    window.dispatchEvent(
                      new CustomEvent('docklite-db-select-table', { detail: { table: table.name } })
                    );
                  }}
                >
                  <span className="text-xs" style={{ color: 'var(--text-cyan)' }}>
                    {isExpanded ? '▼' : '▶'}
                  </span>
                  <span className="font-bold truncate">{table.name}</span>
                </button>
                {isExpanded && (
                  <div className="mt-2 space-y-1 pl-6">
                    {table.columns.map((column) => (
                      <button
                        key={column.name}
                        type="button"
                        className="w-full text-left text-[11px] px-2 py-1 rounded-md transition-all hover:bg-white/5"
                        style={{ color: 'var(--text-primary)' }}
                        onClick={() => {
                          setExpandedTables((prev) => ({ ...prev, [table.name]: true }));
                          setSelectedTable(table.name);
                          window.dispatchEvent(
                            new CustomEvent('docklite-db-select-column', {
                              detail: { table: table.name, column: column.name },
                            })
                          );
                        }}
                      >
                        <span className="mr-2" style={{ color: 'var(--text-secondary)' }}>
                          ├─
                        </span>
                        <span style={{ color: 'var(--text-cyan)' }}>{column.name}</span>{' '}
                        <span style={{ color: 'var(--text-purple)' }}>({column.type})</span>
                      </button>
                    ))}
                  </div>
                )}
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
