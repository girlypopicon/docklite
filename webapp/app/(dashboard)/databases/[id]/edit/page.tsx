'use client';

import { useCallback, useEffect, useMemo, useState } from 'react';
import { useParams, useRouter } from 'next/navigation';
import { Sparkle, CheckCircle, ArrowLeft, SpinnerGap, Play } from '@phosphor-icons/react';

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

interface TableData {
  columns: ColumnInfo[];
  rows: Record<string, any>[];
}

interface DatabaseInfo {
  id: number;
  name: string;
  postgres_port: number;
}

interface QueryResult {
  type: 'select' | 'command';
  columns?: string[];
  rows?: Record<string, any>[];
  output?: string;
}

const STORAGE_PREFIX = 'docklite-db-edit-';
const TABLE_ROW_LIMIT = 100;

const numericTokens = ['int', 'numeric', 'decimal', 'real', 'double', 'float'];

function isNumericColumn(type: string) {
  const normalized = type.toLowerCase();
  return numericTokens.some((token) => normalized.includes(token));
}

function areValuesEqual(original: any, updated: any) {
  if (original === null || original === undefined) {
    return updated === null || updated === undefined || updated === '';
  }
  if (updated === null || updated === undefined) {
    return original === null || original === undefined || original === '';
  }
  return String(original) === String(updated);
}

function formatCellValue(value: any) {
  if (value === null || value === undefined) return '-';
  if (value === '') return '""';
  return String(value);
}

export default function DatabaseEditPage() {
  const params = useParams();
  const router = useRouter();
  const id = Array.isArray(params?.id) ? params.id[0] : params?.id;
  const dbId = useMemo(() => (id ? String(id) : null), [id]);

  const [auth, setAuth] = useState<{ username: string; password: string } | null>(null);
  const [dbInfo, setDbInfo] = useState<DatabaseInfo | null>(null);
  const [tables, setTables] = useState<TableInfo[]>([]);
  const [loadingSchema, setLoadingSchema] = useState(false);
  const [schemaError, setSchemaError] = useState<string | null>(null);
  const [selectedTable, setSelectedTable] = useState<string | null>(null);
  const [selectedColumn, setSelectedColumn] = useState<string | null>(null);
  const [tableData, setTableData] = useState<TableData | null>(null);
  const [loadingTable, setLoadingTable] = useState(false);
  const [tableError, setTableError] = useState<string | null>(null);
  const [isEditMode, setIsEditMode] = useState(false);
  const [editedRows, setEditedRows] = useState<Record<string, any>[]>([]);
  const [savingRows, setSavingRows] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saveSuccess, setSaveSuccess] = useState<string | null>(null);
  const [sql, setSql] = useState('SELECT * FROM users LIMIT 10;');
  const [queryResult, setQueryResult] = useState<QueryResult | null>(null);
  const [queryError, setQueryError] = useState<string | null>(null);
  const [runningQuery, setRunningQuery] = useState(false);
  const [activeTab, setActiveTab] = useState<'rows' | 'query' | 'structure'>('rows');

  const handleInvalidCredentials = useCallback(() => {
    if (dbId) {
      sessionStorage.removeItem(`${STORAGE_PREFIX}${dbId}`);
    }
    router.push('/databases');
  }, [dbId, router]);

  useEffect(() => {
    if (!dbId) return;
    const rawAuth = sessionStorage.getItem(`${STORAGE_PREFIX}${dbId}`);
    if (!rawAuth) {
      return;
    }
    setAuth(JSON.parse(rawAuth));
  }, [dbId]);

  useEffect(() => {
    if (!dbId) return;
    const fetchDb = async () => {
      const res = await fetch(`/api/databases/${dbId}`);
      if (!res.ok) return;
      const data = await res.json();
      setDbInfo(data.database);
    };
    fetchDb();
  }, [dbId]);

  useEffect(() => {
    if (!dbId || !auth) return;
    const fetchSchema = async () => {
      try {
        setLoadingSchema(true);
        setSchemaError(null);
        const res = await fetch(`/api/databases/${dbId}/schema`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(auth),
        });
        if (res.status === 401) {
          handleInvalidCredentials();
          return;
        }
        if (!res.ok) {
          const data = await res.json().catch(() => ({}));
          throw new Error(data.error || 'Failed to load schema');
        }
        const data = await res.json();
        setTables(data.tables || []);
      } catch (err: any) {
        setSchemaError(err.message || 'Failed to load schema');
      } finally {
        setLoadingSchema(false);
      }
    };
    fetchSchema();
  }, [dbId, auth]);

  const loadTableData = useCallback(
    async (tableName: string) => {
      if (!dbId || !auth || !tableName) return;
      try {
        setLoadingTable(true);
        setTableError(null);
        setSaveError(null);
        const res = await fetch(`/api/databases/${dbId}/table`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ ...auth, table: tableName }),
        });
        if (res.status === 401) {
          handleInvalidCredentials();
          return;
        }
        if (!res.ok) {
          const data = await res.json().catch(() => ({}));
          throw new Error(data.error || 'Failed to load table');
        }
        const data = await res.json();
        setTableData(data);
        window.dispatchEvent(
          new CustomEvent('docklite-db-table-loaded', { detail: { table: tableName, rows: (data.rows || []).length } })
        );
        setIsEditMode(false);
        setEditedRows([]);
      } catch (err: any) {
        setTableError(err.message || 'Failed to load table');
      } finally {
        setLoadingTable(false);
      }
    },
    [dbId, auth]
  );

  useEffect(() => {
    const handleTableSelect = (event: Event) => {
      const detail = (event as CustomEvent<{ table: string }>).detail;
      if (detail?.table) {
        setSelectedTable(detail.table);
        setSelectedColumn(null);
        setSaveError(null);
        setSaveSuccess(null);
      }
    };
    window.addEventListener('docklite-db-select-table', handleTableSelect);
    return () => window.removeEventListener('docklite-db-select-table', handleTableSelect);
  }, []);

  useEffect(() => {
    const handleColumnSelect = (event: Event) => {
      const detail = (event as CustomEvent<{ table: string; column: string }>).detail;
      if (detail?.table) {
        setSelectedTable(detail.table);
        setSelectedColumn(detail.column || null);
        setSaveError(null);
        setSaveSuccess(null);
      }
    };
    window.addEventListener('docklite-db-select-column', handleColumnSelect);
    return () => window.removeEventListener('docklite-db-select-column', handleColumnSelect);
  }, []);

  useEffect(() => {
    if (!selectedTable) return;
    loadTableData(selectedTable);
  }, [selectedTable, loadTableData]);

  const handleRunQuery = async () => {
    if (!dbId || !auth || !sql.trim()) return;
    try {
      setRunningQuery(true);
      setQueryError(null);
      const res = await fetch(`/api/databases/${dbId}/query`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ ...auth, sql }),
      });
      const data = await res.json().catch(() => ({}));
      if (res.status === 401) {
        handleInvalidCredentials();
        return;
      }
      if (!res.ok) {
        throw new Error(data.error || 'Query failed');
      }
      setQueryResult(data);
    } catch (err: any) {
      setQueryError(err.message || 'Query failed');
    } finally {
      setRunningQuery(false);
    }
  };

  const handleEnableEdit = () => {
    if (!tableData) return;
    setEditedRows(tableData.rows.map((row) => ({ ...row })));
    setIsEditMode(true);
    setSaveError(null);
    setSaveSuccess(null);
  };

  const handleCellEdit = (rowIndex: number, columnName: string, columnType: string, rawValue: string) => {
    const updatedRows = [...editedRows];
    const normalizedValue = isNumericColumn(columnType)
      ? rawValue === ''
        ? null
        : Number.isNaN(Number(rawValue))
          ? rawValue
          : Number(rawValue)
      : rawValue;
    updatedRows[rowIndex] = {
      ...updatedRows[rowIndex],
      [columnName]: normalizedValue,
    };
    setEditedRows(updatedRows);
  };

  const handleCancel = async () => {
    setIsEditMode(false);
    setEditedRows([]);
    setSaveError(null);
    setSaveSuccess(null);
    if (selectedTable) {
      await loadTableData(selectedTable);
    }
  };

  const handleSave = async () => {
    if (!dbId || !auth || !selectedTable || !tableData) return;
    if (!tableData.columns.some((column) => column.name === 'id')) {
      setSaveError('Edit mode requires an id column to save changes.');
      return;
    }

    const changedRows: Record<string, any>[] = [];
    tableData.rows.forEach((originalRow, rowIndex) => {
      const editedRow = editedRows[rowIndex];
      if (!editedRow) return;
      const changes: Record<string, any> = {};
      tableData.columns.forEach((column) => {
        const originalValue = originalRow[column.name];
        const updatedValue = editedRow[column.name];
        if (!areValuesEqual(originalValue, updatedValue)) {
          changes[column.name] = updatedValue;
        }
      });
      if (Object.keys(changes).length > 0) {
        changes.id = editedRow.id ?? originalRow.id;
        changedRows.push(changes);
      }
    });

    if (changedRows.length === 0) {
      setSaveError('No changes to save.');
      return;
    }

    if (changedRows.some((row) => row.id === undefined || row.id === null || row.id === '')) {
      setSaveError('Missing id values for one or more edited rows.');
      return;
    }

    try {
      setSavingRows(true);
      setSaveError(null);
      const res = await fetch(`/api/databases/${dbId}/update-rows`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          ...auth,
          table: selectedTable,
          rows: changedRows,
        }),
      });
      if (res.status === 401) {
        handleInvalidCredentials();
        return;
      }
      if (!res.ok) {
        const data = await res.json().catch(() => ({}));
        throw new Error(data.error || 'Failed to save');
      }
      setSaveSuccess('Changes saved.');
      setIsEditMode(false);
      setEditedRows([]);
      await loadTableData(selectedTable);
    } catch (err: any) {
      setSaveError(err.message || 'Save failed');
    } finally {
      setSavingRows(false);
    }
  };

  const structureColumns = tables.find((table) => table.name === selectedTable)?.columns || [];

  if (!dbId) {
    return (
      <div className="card-vapor p-8 max-w-xl">
        <h1 className="text-xl font-bold" style={{ color: 'var(--status-error)' }}>
          Missing database ID
        </h1>
      </div>
    );
  }

  if (!auth) {
    return (
      <div className="max-w-xl card-vapor p-8">
        <h1 className="text-xl font-bold mb-3" style={{ color: 'var(--status-error)' }}>
          Missing database credentials
        </h1>
        <p className="text-sm font-mono mb-6" style={{ color: 'var(--text-secondary)' }}>
          Return to the databases page and enter edit mode again.
        </p>
        <button
          type="button"
          onClick={() => router.push('/databases')}
          className="btn-neon px-5 py-2 text-sm font-bold inline-flex items-center gap-2"
        >
          <ArrowLeft size={14} weight="bold" />
          Back to Databases
        </button>
      </div>
    );
  }

  return (
    <div className="max-w-[1400px] mx-auto space-y-6">
      <div className="card-vapor p-6 rounded-xl border border-purple-500/30">
        <div className="flex flex-wrap items-center justify-between gap-4">
          <div>
            <h1 className="text-3xl font-bold neon-text flex items-center gap-2" style={{ color: 'var(--neon-cyan)' }}>
              <Sparkle size={20} weight="duotone" />
              Database Edit Mode
            </h1>
            <p className="text-xs font-mono" style={{ color: 'var(--text-secondary)' }}>
              {dbInfo ? `${dbInfo.name} • localhost:${dbInfo.postgres_port}` : 'Loading database info...'}
            </p>
            {auth && (
              <div
                className="mt-2 inline-flex items-center gap-2 px-3 py-1 rounded-full text-xs font-mono"
                style={{
                  background: 'rgba(var(--neon-cyan-rgb), 0.1)',
                  border: '1px solid rgba(var(--neon-cyan-rgb), 0.4)',
                  color: 'var(--neon-cyan)',
                }}
              >
                <CheckCircle size={14} weight="duotone" />
                Connected as <span className="font-bold">{auth.username}</span>
              </div>
            )}
          </div>
          <button
            type="button"
            onClick={() => router.push('/databases')}
            className="px-4 py-2 rounded-lg font-bold transition-all hover:scale-105"
            style={{
              background: 'linear-gradient(135deg, var(--neon-purple) 0%, var(--neon-pink) 100%)',
              color: 'var(--button-text)',
            }}
          >
            <span className="inline-flex items-center gap-2">
              <ArrowLeft size={14} weight="bold" />
              Exit Edit Mode
            </span>
          </button>
        </div>
      </div>

      <div className="card-vapor p-4 rounded-xl border border-purple-500/30">
        <div className="flex flex-wrap gap-3">
          <button
            type="button"
            onClick={() => setActiveTab('rows')}
            className="px-4 py-2 rounded-lg text-sm font-bold transition-all"
            style={{
              background: activeTab === 'rows'
                ? 'linear-gradient(135deg, var(--neon-pink) 0%, var(--neon-purple) 100%)'
                : 'rgba(var(--text-muted-rgb), 0.05)',
              color: activeTab === 'rows' ? 'white' : 'var(--text-secondary)',
              border: activeTab === 'rows' ? '2px solid var(--neon-pink)' : '2px solid transparent',
              boxShadow: activeTab === 'rows' ? '0 0 12px rgba(var(--neon-pink-rgb), 0.4)' : 'none',
            }}
          >
            📊 Rows
          </button>
          <button
            type="button"
            onClick={() => setActiveTab('query')}
            className="px-4 py-2 rounded-lg text-sm font-bold transition-all"
            style={{
              background: activeTab === 'query'
                ? 'linear-gradient(135deg, var(--neon-cyan) 0%, var(--neon-green) 100%)'
                : 'rgba(var(--text-muted-rgb), 0.05)',
              color: activeTab === 'query' ? 'white' : 'var(--text-secondary)',
              border: activeTab === 'query' ? '2px solid var(--neon-cyan)' : '2px solid transparent',
              boxShadow: activeTab === 'query' ? '0 0 12px rgba(var(--neon-cyan-rgb), 0.3)' : 'none',
            }}
          >
            ⚡ Query
          </button>
          <button
            type="button"
            onClick={() => setActiveTab('structure')}
            className="px-4 py-2 rounded-lg text-sm font-bold transition-all"
            style={{
              background: activeTab === 'structure'
                ? 'linear-gradient(135deg, var(--neon-cyan) 0%, var(--neon-purple) 100%)'
                : 'rgba(var(--text-muted-rgb), 0.05)',
              color: activeTab === 'structure' ? 'white' : 'var(--text-secondary)',
              border: activeTab === 'structure' ? '2px solid var(--neon-cyan)' : '2px solid transparent',
              boxShadow: activeTab === 'structure' ? '0 0 12px rgba(var(--neon-cyan-rgb), 0.35)' : 'none',
            }}
          >
            🏗️ Structure
          </button>
        </div>
      </div>

      {activeTab === 'rows' && (
        <div className="card-vapor p-6 rounded-xl border border-purple-500/20 space-y-4">
          <div className="flex flex-wrap items-center justify-between gap-4">
            <div>
              <h2 className="text-xl font-bold neon-text flex items-center gap-2" style={{ color: 'var(--neon-pink)' }}>
                📋 {selectedTable || 'Rows'}
              </h2>
              {tableData && (
                <p className="text-xs font-mono" style={{ color: 'var(--text-secondary)' }}>
                  {tableData.rows.length} rows
                  {tableData.rows.length >= TABLE_ROW_LIMIT ? ` (showing first ${TABLE_ROW_LIMIT})` : ''}
                </p>
              )}
            </div>
            {selectedTable && !isEditMode && (
              <button
                type="button"
                onClick={handleEnableEdit}
                className="px-4 py-2 text-sm font-bold rounded-lg transition-all animate-pulse"
                style={{
                  background: 'linear-gradient(135deg, var(--status-error) 0%, var(--status-error-strong) 100%)',
                  border: '3px solid var(--status-error-strong)',
                  color: 'white',
                  boxShadow: '0 0 16px rgba(var(--status-error-strong-rgb), 0.55)',
                }}
              >
                🔓 Enable Edit Mode
              </button>
            )}
            {selectedTable && isEditMode && (
              <div className="flex flex-wrap items-center gap-3">
                <button
                  type="button"
                  onClick={handleSave}
                  disabled={savingRows}
                  className="px-4 py-2 text-sm font-bold rounded-lg transition-all disabled:opacity-50"
                  style={{
                    background: 'linear-gradient(135deg, var(--status-error) 0%, var(--status-error-strong) 100%)',
                    border: '2px solid var(--status-error-strong)',
                    color: 'white',
                  }}
                >
                  💾 {savingRows ? 'Saving...' : 'Save'}
                </button>
                <button
                  type="button"
                  onClick={handleCancel}
                  className="px-4 py-2 text-sm font-bold rounded-lg transition-all"
                  style={{
                    background: 'rgba(var(--text-muted-rgb), 0.08)',
                    border: '2px solid rgba(var(--text-muted-rgb), 0.15)',
                    color: 'var(--text-primary)',
                  }}
                >
                  ❌ Cancel
                </button>
                <button
                  type="button"
                  onClick={handleCancel}
                  className="px-4 py-2 text-sm font-bold rounded-lg transition-all"
                  style={{
                    background: 'linear-gradient(135deg, var(--status-success) 0%, var(--status-success) 100%)',
                    border: '2px solid var(--status-success)',
                    color: 'white',
                  }}
                >
                  ✓ Back to View
                </button>
              </div>
            )}
          </div>

          {loadingTable && (
            <div className="text-sm font-mono flex items-center gap-2" style={{ color: 'var(--neon-cyan)' }}>
              <SpinnerGap size={14} weight="duotone" className="animate-spin" />
              Loading table...
            </div>
          )}
          {tableError && (
            <div className="text-sm font-mono" style={{ color: 'var(--status-error)' }}>
              {tableError}
            </div>
          )}
          {saveError && (
            <div className="text-sm font-mono" style={{ color: 'var(--status-error)' }}>
              {saveError}
            </div>
          )}
          {saveSuccess && (
            <div className="text-sm font-mono" style={{ color: 'var(--neon-green)' }}>
              {saveSuccess}
            </div>
          )}

          {!loadingTable && !tableError && tableData && (
            <div className="space-y-4">
              <div className="flex flex-wrap gap-3">
                {tableData.columns.map((col) => (
                  <div
                    key={col.name}
                    className="px-3 py-1 rounded-full text-xs font-mono border"
                    style={{
                      borderColor: 'rgba(var(--neon-cyan-rgb), 0.4)',
                      color: 'var(--neon-cyan)',
                      background: 'rgba(var(--neon-cyan-rgb), 0.08)',
                    }}
                  >
                    {col.name} • {col.type}
                  </div>
                ))}
              </div>

              <div className="overflow-x-auto">
                <table className="w-full text-xs font-mono">
                  <thead>
                    <tr>
                      {tableData.columns.map((col) => {
                        const isHighlighted = selectedColumn === col.name;
                        return (
                          <th
                            key={col.name}
                            className="px-3 py-2 text-left"
                            style={{
                              color: 'var(--neon-purple)',
                              background: isHighlighted ? 'rgba(var(--neon-cyan-rgb), 0.15)' : 'transparent',
                            }}
                          >
                            {col.name}
                          </th>
                        );
                      })}
                    </tr>
                  </thead>
                  <tbody>
                    {tableData.rows.length === 0 ? (
                      <tr>
                        <td
                          colSpan={tableData.columns.length}
                          className="px-3 py-4 text-center opacity-60"
                          style={{ color: 'var(--text-secondary)' }}
                        >
                          No rows found in this table.
                        </td>
                      </tr>
                    ) : (
                      (isEditMode ? editedRows : tableData.rows).map((row, rowIndex) => (
                        <tr key={rowIndex} className="border-t border-purple-500/10">
                          {tableData.columns.map((col) => {
                            const isHighlighted = selectedColumn === col.name;
                            const isModified = isEditMode && !areValuesEqual(tableData.rows[rowIndex]?.[col.name], row[col.name]);
                            const cellStyle = isModified
                              ? {
                                  background: 'rgba(var(--status-warning-rgb), 0.18)',
                                  boxShadow: 'inset 0 0 0 1px rgba(var(--status-warning-rgb), 0.35)',
                                }
                              : isHighlighted
                                ? { background: 'rgba(var(--neon-cyan-rgb), 0.12)' }
                                : {};

                            return (
                              <td key={col.name} className="px-3 py-2" style={cellStyle}>
                                {isEditMode ? (
                                  <input
                                    type={isNumericColumn(col.type) ? 'number' : 'text'}
                                    value={row[col.name] === null || row[col.name] === undefined ? '' : String(row[col.name])}
                                    onChange={(e) => handleCellEdit(rowIndex, col.name, col.type, e.target.value)}
                                    className="w-full px-2 py-1 text-xs rounded-md"
                                    style={{
                                      background: 'var(--surface-muted)',
                                      border: '1px solid rgba(var(--neon-purple-rgb), 0.4)',
                                      color: 'var(--text-primary)',
                                    }}
                                  />
                                ) : (
                                  formatCellValue(row[col.name])
                                )}
                              </td>
                            );
                          })}
                        </tr>
                      ))
                    )}
                  </tbody>
                </table>
              </div>
            </div>
          )}
          {!loadingTable && !tableError && !tableData && (
            <div className="text-sm font-mono opacity-70" style={{ color: 'var(--text-secondary)' }}>
              Select a table from the schema browser.
            </div>
          )}
        </div>
      )}

      {activeTab === 'query' && (
        <div className="card-vapor p-6 rounded-xl border border-purple-500/20 space-y-4">
          <div className="text-xl font-bold" style={{ color: 'var(--neon-green)' }}>
            ⚡ Query Editor
          </div>
          <textarea
            className="w-full min-h-[200px] p-3 rounded-lg text-xs font-mono"
            style={{
              background: 'var(--surface-muted)',
              border: '2px solid var(--neon-purple)',
              color: 'var(--text-primary)',
            }}
            value={sql}
            onChange={(e) => setSql(e.target.value)}
          />
          <button
            type="button"
            disabled={runningQuery}
            onClick={handleRunQuery}
            className="btn-neon w-full py-2 text-sm font-bold disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {runningQuery ? (
              <span className="inline-flex items-center gap-2">
                <SpinnerGap size={14} weight="duotone" className="animate-spin" />
                Running...
              </span>
            ) : (
              <span className="inline-flex items-center gap-2">
                <Play size={14} weight="duotone" />
                Execute SQL
              </span>
            )}
          </button>

          {queryError && (
            <div className="text-xs font-mono" style={{ color: 'var(--status-error)' }}>
              {queryError}
            </div>
          )}

          {queryResult && (
            <div className="space-y-2 text-xs font-mono">
              {queryResult.type === 'select' && queryResult.rows ? (
                <div>
                  <div className="text-xs mb-2" style={{ color: 'var(--text-secondary)' }}>
                    ⚡ Query Results ({queryResult.rows.length} rows)
                  </div>
                  <div className="overflow-x-auto">
                    <table className="w-full text-xs">
                      <thead>
                        <tr>
                          {queryResult.columns?.map((col) => (
                            <th key={col} className="px-2 py-1 text-left" style={{ color: 'var(--neon-cyan)' }}>
                              {col}
                            </th>
                          ))}
                        </tr>
                      </thead>
                      <tbody>
                        {queryResult.rows.length === 0 ? (
                          <tr>
                            <td
                              colSpan={queryResult.columns?.length || 1}
                              className="px-2 py-2 opacity-60"
                              style={{ color: 'var(--text-secondary)' }}
                            >
                              Query returned no rows.
                            </td>
                          </tr>
                        ) : (
                          queryResult.rows.map((row, idx) => (
                            <tr key={idx} className="border-t border-purple-500/10">
                              {queryResult.columns?.map((col) => (
                                <td key={col} className="px-2 py-1">
                                  {formatCellValue(row[col])}
                                </td>
                              ))}
                            </tr>
                          ))
                        )}
                      </tbody>
                    </table>
                  </div>
                </div>
              ) : (
                <div className="p-3 rounded-lg border border-purple-500/30 bg-black/30">
                  <div style={{ color: 'var(--neon-cyan)' }}>
                    {queryResult.output || 'Query executed.'}
                  </div>
                </div>
              )}
            </div>
          )}
        </div>
      )}

      {activeTab === 'structure' && (
        <div className="card-vapor p-6 rounded-xl border border-purple-500/20 space-y-4">
          <div className="text-xl font-bold" style={{ color: 'var(--neon-cyan)' }}>
            🏗️ Table Structure {selectedTable ? `: ${selectedTable}` : ''}
          </div>

          {loadingSchema && (
            <div className="text-xs font-mono" style={{ color: 'var(--neon-cyan)' }}>
              Loading schema...
            </div>
          )}

          {schemaError && (
            <div className="text-xs font-mono" style={{ color: 'var(--status-error)' }}>
              {schemaError}
            </div>
          )}

          {!loadingSchema && !schemaError && !selectedTable && (
            <div className="text-sm font-mono opacity-70" style={{ color: 'var(--text-secondary)' }}>
              Select a table from the schema browser.
            </div>
          )}

          {!loadingSchema && !schemaError && selectedTable && (
            <div className="overflow-x-auto">
              <table className="w-full text-xs font-mono">
                <thead>
                  <tr>
                    <th className="px-3 py-2 text-left" style={{ color: 'var(--neon-purple)' }}>
                      Column
                    </th>
                    <th className="px-3 py-2 text-left" style={{ color: 'var(--neon-purple)' }}>
                      Type
                    </th>
                    <th className="px-3 py-2 text-left" style={{ color: 'var(--neon-purple)' }}>
                      Nullable
                    </th>
                    <th className="px-3 py-2 text-left" style={{ color: 'var(--neon-purple)' }}>
                      Default
                    </th>
                    <th className="px-3 py-2 text-left" style={{ color: 'var(--neon-purple)' }}>
                      Key
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {structureColumns.length === 0 ? (
                    <tr>
                      <td
                        colSpan={5}
                        className="px-3 py-4 text-center opacity-60"
                        style={{ color: 'var(--text-secondary)' }}
                      >
                        No column data available.
                      </td>
                    </tr>
                  ) : (
                    structureColumns.map((column) => (
                      <tr key={column.name} className="border-t border-purple-500/10">
                        <td className="px-3 py-2" style={{ color: 'var(--neon-cyan)' }}>
                          {column.name}
                        </td>
                        <td className="px-3 py-2">{column.type}</td>
                        <td className="px-3 py-2">{column.nullable ? 'YES' : 'NO'}</td>
                        <td className="px-3 py-2">
                          {column.default === null || column.default === undefined || column.default === ''
                            ? '-'
                            : String(column.default)}
                        </td>
                        <td className="px-3 py-2" style={{ color: 'var(--neon-purple)' }}>
                          {column.key || '-'}
                        </td>
                      </tr>
                    ))
                  )}
                </tbody>
              </table>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
