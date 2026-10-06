'use client';

import { useEffect, useMemo, useState } from 'react';
import { usePathname } from 'next/navigation';
import { Copy, Key, Table } from '@phosphor-icons/react';
import { useToast } from '@/lib/hooks/useToast';

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

const STORAGE_PREFIX = 'docklite-db-edit-';

const quote = (identifier: string) => `"${identifier.replace(/"/g, '""')}"`;

/** CREATE TABLE text from a column list. It can't know indexes or foreign keys, so it says so. */
function buildCreateTable(table: TableInfo): string {
  const lines = table.columns.map((col) => {
    const parts = [`  ${quote(col.name)} ${col.type}`];
    if (!col.nullable) parts.push('NOT NULL');
    if (col.default) parts.push(`DEFAULT ${col.default}`);
    return parts.join(' ');
  });
  const primaryKeys = table.columns.filter((c) => c.key && /pri/i.test(c.key)).map((c) => quote(c.name));
  if (primaryKeys.length) lines.push(`  PRIMARY KEY (${primaryKeys.join(', ')})`);
  return `CREATE TABLE ${quote(table.name)} (\n${lines.join(',\n')}\n);`;
}

/**
 * Right sidebar for the Edit Database page: details of the table you're
 * looking at — its columns, size, and copy-ready SQL — in place of the
 * generic panels, which have nothing to do with editing a database.
 */
export default function DatabaseContextPanel() {
  const pathname = usePathname();
  const dbId = useMemo(() => pathname?.match(/^\/databases\/(\d+)\/edit/)?.[1] || null, [pathname]);
  const toast = useToast();
  const [tables, setTables] = useState<TableInfo[]>([]);
  const [selected, setSelected] = useState<string | null>(null);
  const [rowCount, setRowCount] = useState<number | null>(null);
  const [dbName, setDbName] = useState<string | null>(null);
  const [port, setPort] = useState<number | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!dbId) return;
    let cancelled = false;
    (async () => {
      try {
        const info = await fetch(`/api/databases/${dbId}`);
        if (info.ok) {
          const data = await info.json();
          if (!cancelled) {
            setDbName(data?.database?.name ?? null);
            setPort(data?.database?.postgres_port ?? null);
          }
        }
        const raw = sessionStorage.getItem(`${STORAGE_PREFIX}${dbId}`);
        if (!raw) return;
        const res = await fetch(`/api/databases/${dbId}/schema`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: raw,
        });
        if (!res.ok) throw new Error('Could not load the table details');
        const data = await res.json();
        if (!cancelled) setTables(data.tables || []);
      } catch (err: any) {
        if (!cancelled) setError(err.message || 'Could not load the table details');
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [dbId]);

  // The page and the left schema browser announce the table being viewed.
  useEffect(() => {
    const onSelect = (event: Event) => {
      const table = (event as CustomEvent<{ table: string }>).detail?.table;
      if (table) {
        setSelected(table);
        setRowCount(null);
      }
    };
    const onLoaded = (event: Event) => {
      const detail = (event as CustomEvent<{ table: string; rows: number }>).detail;
      if (detail) setRowCount(detail.rows);
    };
    window.addEventListener('docklite-db-select-table', onSelect);
    window.addEventListener('docklite-db-table-loaded', onLoaded);
    return () => {
      window.removeEventListener('docklite-db-select-table', onSelect);
      window.removeEventListener('docklite-db-table-loaded', onLoaded);
    };
  }, []);

  const table = tables.find((t) => t.name === selected) || null;

  const copy = async (text: string, what: string) => {
    try {
      await navigator.clipboard.writeText(text);
      toast.success(`Copied ${what}`);
    } catch {
      toast.error('Copy failed — your browser blocked clipboard access');
    }
  };

  if (error) return <div className="text-xs" style={{ color: 'var(--status-error)' }}>{error}</div>;

  if (!table) {
    return (
      <div className="text-sm opacity-80" style={{ color: 'var(--text-secondary)' }}>
        <div className="font-bold mb-2 flex items-center gap-2" style={{ color: 'var(--neon-cyan)' }}>
          <Table size={16} weight="duotone" /> Table details
        </div>
        Pick a table in the schema on the left to see its columns and copy ready-made SQL.
      </div>
    );
  }

  return (
    <div className="space-y-4 text-sm">
      <div>
        <div className="font-bold flex items-center gap-2 break-all" style={{ color: 'var(--neon-cyan)' }}>
          <Table size={16} weight="duotone" /> {table.name}
        </div>
        <div className="text-xs opacity-70 mt-1">
          {table.columns.length} column{table.columns.length === 1 ? '' : 's'}
          {rowCount !== null && ` · ${rowCount.toLocaleString()} row${rowCount === 1 ? '' : 's'} shown`}
        </div>
      </div>

      <ul className="space-y-1">
        {table.columns.map((col) => {
          const isKey = Boolean(col.key && /pri/i.test(col.key));
          return (
            <li key={col.name} className="p-2 rounded-lg" style={{ background: 'var(--surface-muted)' }}>
              <div className="flex items-center gap-2 font-mono text-xs font-bold" style={{ color: 'var(--text-primary)' }}>
                {isKey && <Key size={12} weight="fill" style={{ color: 'var(--neon-yellow)' }} />}
                <span className="break-all">{col.name}</span>
              </div>
              <div className="text-[11px] mt-0.5" style={{ color: 'var(--text-secondary)' }}>
                {col.type}
                {!col.nullable && ' · required'}
                {col.default ? ` · default ${col.default}` : ''}
              </div>
            </li>
          );
        })}
      </ul>

      <div className="space-y-2">
        <button type="button" className="btn-neon w-full px-3 py-2 text-xs font-bold inline-flex items-center justify-center gap-2" onClick={() => copy(`SELECT * FROM ${quote(table.name)} LIMIT 100;`, 'the SELECT query')}>
          <Copy size={14} weight="duotone" /> Copy SELECT query
        </button>
        <button type="button" className="btn-neon w-full px-3 py-2 text-xs font-bold inline-flex items-center justify-center gap-2" onClick={() => copy(buildCreateTable(table), 'CREATE TABLE')}>
          <Copy size={14} weight="duotone" /> Copy CREATE TABLE
        </button>
        <p className="text-[11px] opacity-60">
          CREATE TABLE is rebuilt from the column list, so indexes and foreign keys aren&apos;t included.
        </p>
      </div>

      {dbName && port && (
        <div className="pt-3 border-t" style={{ borderColor: 'rgba(var(--neon-purple-rgb), 0.3)' }}>
          <div className="text-xs font-bold mb-2" style={{ color: 'var(--neon-purple)' }}>Connect from a terminal</div>
          <button type="button" className="btn-neon w-full px-3 py-2 text-xs font-bold inline-flex items-center justify-center gap-2" onClick={() => copy(`psql -h ${window.location.hostname} -p ${port} -U <user> ${dbName}`, 'the psql command')}>
            <Copy size={14} weight="duotone" /> Copy psql command
          </button>
          <p className="text-[11px] opacity-60 mt-1">Fill in your database user; the password isn&apos;t included.</p>
        </div>
      )}
    </div>
  );
}
