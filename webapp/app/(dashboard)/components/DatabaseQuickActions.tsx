'use client';

import { useState } from 'react';
import { Plus, Table, Rows, Columns, PencilSimple, Trash, Eraser } from '@phosphor-icons/react';
import CloseButton from './CloseButton';

export interface QAColumn {
  name: string;
  type: string;
  nullable: boolean;
  default?: string | null;
  key?: string | null;
}

type Action = 'new-table' | 'add-row' | 'add-column' | 'delete-rows' | 'rename-table' | 'empty-table' | 'drop-table';

const quote = (identifier: string) => `"${identifier.replace(/"/g, '""')}"`;
const isNumeric = (type: string) => ['int', 'numeric', 'decimal', 'real', 'double', 'float', 'serial'].some((t) => type.toLowerCase().includes(t));

/** A value typed in a box, as SQL text: numbers stay numbers, everything else is quoted with its quotes doubled. */
function literal(value: string, type: string): string {
  const v = value.trim();
  if (isNumeric(type) && v !== '' && !Number.isNaN(Number(v))) return String(Number(v));
  if (/bool/i.test(type)) return /^(1|true|yes|on)$/i.test(v) ? 'TRUE' : 'FALSE';
  return `'${value.replace(/'/g, "''")}'`;
}

const COLUMN_KINDS = [
  { id: 'text', label: 'Text (words, names, anything)' },
  { id: 'number', label: 'Whole number' },
  { id: 'decimal', label: 'Number with decimals' },
  { id: 'yesno', label: 'Yes / No' },
  { id: 'date', label: 'Date' },
];

function kindToSql(kind: string, engine: string): string {
  const sqlite = engine === 'sqlite';
  switch (kind) {
    case 'number': return sqlite ? 'INTEGER' : 'integer';
    case 'decimal': return sqlite ? 'REAL' : 'numeric';
    case 'yesno': return sqlite ? 'INTEGER' : 'boolean';
    case 'date': return sqlite ? 'TEXT' : 'date';
    default: return sqlite ? 'TEXT' : 'text';
  }
}

interface Props {
  dbId: string;
  engine: string; // 'sqlite', 'postgres', ...
  table: { name: string; columns: QAColumn[] } | null;
  onDone: () => void;
}

/**
 * Buttons for the things most people want to do with a database, each asking for what it needs in plain boxes and
 * writing the SQL itself. The SQL it ran is shown afterwards, so it doubles as a way to learn.
 */
export default function DatabaseQuickActions({ dbId, engine, table, onDone }: Props) {
  const [action, setAction] = useState<Action | null>(null);
  const [values, setValues] = useState<Record<string, string>>({});
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [ran, setRan] = useState('');

  const open = (a: Action) => {
    setAction(a);
    setValues({});
    setError('');
    setRan('');
  };
  const set = (k: string, v: string) => setValues((s) => ({ ...s, [k]: v }));
  const close = () => setAction(null);

  const build = (): string | null => {
    const t = table ? quote(table.name) : '';
    switch (action) {
      case 'new-table': {
        const name = (values.name || '').trim();
        const cols = (values.cols || '').split(',').map((c) => c.trim()).filter(Boolean);
        if (!name) { setError('Give the table a name.'); return null; }
        if (!cols.length) { setError('Add at least one column, like "name, email".'); return null; }
        const id = engine === 'sqlite' ? '"id" INTEGER PRIMARY KEY AUTOINCREMENT' : '"id" SERIAL PRIMARY KEY';
        return `CREATE TABLE ${quote(name)} (${[id, ...cols.map((c) => `${quote(c)} ${kindToSql('text', engine)}`)].join(', ')});`;
      }
      case 'add-row': {
        if (!table) return null;
        const cols = table.columns.filter((c) => (values[c.name] ?? '') !== '');
        if (!cols.length) { setError('Fill in at least one box.'); return null; }
        return `INSERT INTO ${t} (${cols.map((c) => quote(c.name)).join(', ')}) VALUES (${cols.map((c) => literal(values[c.name], c.type)).join(', ')});`;
      }
      case 'add-column': {
        const name = (values.name || '').trim();
        if (!name) { setError('Give the column a name.'); return null; }
        return `ALTER TABLE ${t} ADD COLUMN ${quote(name)} ${kindToSql(values.kind || 'text', engine)};`;
      }
      case 'delete-rows': {
        const col = table?.columns.find((c) => c.name === values.col) || table?.columns[0];
        if (!col) return null;
        if ((values.val ?? '') === '') { setError('Say which value to look for, so only those rows are deleted.'); return null; }
        return `DELETE FROM ${t} WHERE ${quote(col.name)} = ${literal(values.val, col.type)};`;
      }
      case 'rename-table': {
        const name = (values.name || '').trim();
        if (!name) { setError('Type the new name.'); return null; }
        return `ALTER TABLE ${t} RENAME TO ${quote(name)};`;
      }
      case 'empty-table':
        return `DELETE FROM ${t};`;
      case 'drop-table':
        if ((values.confirm || '') !== table?.name) { setError(`Type ${table?.name} to confirm.`); return null; }
        return `DROP TABLE ${t};`;
      default:
        return null;
    }
  };

  const run = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    const sql = build();
    if (!sql) return;
    setBusy(true);
    try {
      let creds = '{}';
      try { creds = sessionStorage.getItem(`docklite-db-edit-${dbId}`) || '{}'; } catch { /* private window */ }
      const res = await fetch(`/api/databases/${dbId}/query`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ ...JSON.parse(creds), sql }),
      });
      const data = await res.json().catch(() => ({}));
      if (!res.ok) throw new Error(data.error || 'That did not work');
      setRan(sql);
      onDone();
    } catch (err: any) {
      setError(err.message || 'That did not work');
    } finally {
      setBusy(false);
    }
  };

  const btn = 'btn-neon w-full px-3 py-2 text-xs font-bold inline-flex items-center justify-start gap-2';
  const label = 'block text-xs mb-1';
  const needsTable = !!table;

  const titles: Record<Action, string> = {
    'new-table': 'Make a new table',
    'add-row': `Add a row to ${table?.name}`,
    'add-column': `Add a column to ${table?.name}`,
    'delete-rows': `Delete rows from ${table?.name}`,
    'rename-table': `Rename ${table?.name}`,
    'empty-table': `Empty ${table?.name}?`,
    'drop-table': `Delete ${table?.name}?`,
  };

  return (
    <div className="space-y-2">
      <div className="text-xs font-bold" style={{ color: 'var(--text-purple)' }}>Do something</div>
      <button type="button" className={btn} onClick={() => open('new-table')}><Plus size={14} /> New table</button>
      {needsTable && (
        <>
          <button type="button" className={btn} onClick={() => open('add-row')}><Rows size={14} /> Add a row</button>
          <button type="button" className={btn} onClick={() => open('add-column')}><Columns size={14} /> Add a column</button>
          <button type="button" className={btn} onClick={() => open('delete-rows')}><Eraser size={14} /> Delete some rows</button>
          <button type="button" className={btn} onClick={() => open('rename-table')}><PencilSimple size={14} /> Rename this table</button>
          <button type="button" className={btn} onClick={() => open('empty-table')}><Table size={14} /> Empty this table</button>
          <button type="button" className={btn} onClick={() => open('drop-table')}><Trash size={14} /> Delete this table</button>
        </>
      )}

      {action && (
        <div className="fixed inset-0 z-[10000] flex items-center justify-center p-3 sm:p-4 bg-black/75 backdrop-blur-sm" onClick={close}>
          <form
            onSubmit={run}
            onClick={(e) => e.stopPropagation()}
            className="card-vapor w-full max-w-md max-h-[90vh] overflow-y-auto p-5 relative space-y-3"
            style={{ background: 'linear-gradient(135deg, var(--modal-bg-1) 0%, var(--modal-bg-2) 100%)', border: '2px solid var(--modal-border)', boxShadow: '0 0 30px var(--modal-shadow)' }}
          >
            <CloseButton onClick={close} className="absolute top-3 right-3" />
            <h3 className="text-lg font-bold pr-12 break-all" style={{ color: 'var(--text-primary)' }}>{titles[action]}</h3>

            {action === 'new-table' && (
              <>
                <label className="block">
                  <span className={label} style={{ color: 'var(--text-secondary)' }}>Table name</span>
                  <input className="input-vapor w-full" value={values.name || ''} onChange={(e) => set('name', e.target.value)} placeholder="customers" autoFocus />
                </label>
                <label className="block">
                  <span className={label} style={{ color: 'var(--text-secondary)' }}>Column names, separated by commas</span>
                  <input className="input-vapor w-full" value={values.cols || ''} onChange={(e) => set('cols', e.target.value)} placeholder="name, email, phone" />
                </label>
                <p className="text-xs" style={{ color: 'var(--text-secondary)' }}>An <b>id</b> column that numbers each row for you is added automatically. You can change column kinds later with SQL.</p>
              </>
            )}

            {action === 'add-row' && table && (
              <>
                <p className="text-xs" style={{ color: 'var(--text-secondary)' }}>Fill in what you know; empty boxes are left for the database to fill in (an id numbers itself).</p>
                {table.columns.map((c) => (
                  <label key={c.name} className="block">
                    <span className={label} style={{ color: 'var(--text-secondary)' }}>{c.name} <span className="opacity-60">· {c.type}{!c.nullable && c.key !== 'PRIMARY KEY' ? ' · required' : ''}</span></span>
                    <input className="input-vapor w-full" value={values[c.name] || ''} onChange={(e) => set(c.name, e.target.value)} />
                  </label>
                ))}
              </>
            )}

            {action === 'add-column' && (
              <>
                <label className="block">
                  <span className={label} style={{ color: 'var(--text-secondary)' }}>Column name</span>
                  <input className="input-vapor w-full" value={values.name || ''} onChange={(e) => set('name', e.target.value)} placeholder="phone" autoFocus />
                </label>
                <label className="block">
                  <span className={label} style={{ color: 'var(--text-secondary)' }}>What goes in it</span>
                  <select className="input-vapor w-full" value={values.kind || 'text'} onChange={(e) => set('kind', e.target.value)}>
                    {COLUMN_KINDS.map((k) => <option key={k.id} value={k.id}>{k.label}</option>)}
                  </select>
                </label>
              </>
            )}

            {action === 'delete-rows' && table && (
              <>
                <p className="text-xs" style={{ color: 'var(--text-secondary)' }}>Deletes every row where the chosen column has exactly this value. This cannot be undone.</p>
                <label className="block">
                  <span className={label} style={{ color: 'var(--text-secondary)' }}>Column</span>
                  <select className="input-vapor w-full" value={values.col || table.columns[0]?.name} onChange={(e) => set('col', e.target.value)}>
                    {table.columns.map((c) => <option key={c.name} value={c.name}>{c.name}</option>)}
                  </select>
                </label>
                <label className="block">
                  <span className={label} style={{ color: 'var(--text-secondary)' }}>Equals</span>
                  <input className="input-vapor w-full" value={values.val || ''} onChange={(e) => set('val', e.target.value)} autoFocus />
                </label>
              </>
            )}

            {action === 'rename-table' && (
              <label className="block">
                <span className={label} style={{ color: 'var(--text-secondary)' }}>New name</span>
                <input className="input-vapor w-full" value={values.name || ''} onChange={(e) => set('name', e.target.value)} autoFocus />
              </label>
            )}

            {action === 'empty-table' && (
              <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>This deletes every row in <b>{table?.name}</b> and keeps the table and its columns. It cannot be undone.</p>
            )}

            {action === 'drop-table' && (
              <label className="block">
                <span className={label} style={{ color: 'var(--text-secondary)' }}>This deletes the table and everything in it, for good. Type <b>{table?.name}</b> to confirm.</span>
                <input className="input-vapor w-full" value={values.confirm || ''} onChange={(e) => set('confirm', e.target.value)} autoFocus />
              </label>
            )}

            {error && <p className="text-sm" style={{ color: 'var(--status-error)' }}>{error}</p>}
            {ran && (
              <div className="text-xs">
                <div className="font-bold mb-1" style={{ color: 'var(--text-green)' }}>Done. The SQL that ran:</div>
                <code className="block p-2 rounded-lg font-mono break-all select-all" style={{ background: 'var(--bg-darker)', color: 'var(--text-primary)' }}>{ran}</code>
              </div>
            )}

            <div className="flex gap-3 pt-1">
              <button type="button" onClick={close} className="flex-1 px-4 py-2 rounded-lg font-bold border-2" style={{ borderColor: 'var(--surface-dim)', color: 'var(--text-primary)' }}>
                {ran ? 'Close' : 'Cancel'}
              </button>
              {!ran && (
                <button type="submit" disabled={busy} className="flex-1 cyber-button">
                  {busy ? 'Working…' : action === 'drop-table' || action === 'delete-rows' || action === 'empty-table' ? 'Yes, do it' : 'Do it'}
                </button>
              )}
            </div>
          </form>
        </div>
      )}
    </div>
  );
}
