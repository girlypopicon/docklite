'use client';

import { useCallback, useEffect, useMemo, useState } from 'react';
import { Database, Globe } from '@phosphor-icons/react';
import { formatBytes } from '@/lib/database-engines';

// What goes into one .litpkg. Mirrors backup.Selection in go-app/internal/backup/selection.go.
export type BackupSelection = {
  sites?: number[]; // sites whose files are included
  site_databases?: number[]; // sites whose databases are included
  databases?: number[]; // single databases
  all_sites?: boolean; // every site's files, including sites added later
  all_databases?: boolean; // every database, including ones added later
  groups?: number[]; // every container in these groups, including ones added later
  containers?: string[]; // containers by name
};

type TargetDB = { id: number; name: string; type: string; site_id: number | null };
type TargetSite = { id: number; domain: string; databases: TargetDB[] };

export function selectionIsEmpty(sel: BackupSelection) {
  return !sel.all_sites && !sel.all_databases && !sel.sites?.length && !sel.site_databases?.length && !sel.databases?.length && !sel.groups?.length && !sel.containers?.length;
}

const toggle = (list: number[] | undefined, id: number, on: boolean) => {
  const rest = (list || []).filter((x) => x !== id);
  return on ? [...rest, id] : rest;
};

type QuickChoice = 'everything' | 'sites' | 'databases' | 'choose';

const QUICK: Array<{ id: QuickChoice; label: string }> = [
  { id: 'everything', label: 'Everything' },
  { id: 'sites', label: 'All sites' },
  { id: 'databases', label: 'All databases' },
  { id: 'choose', label: 'Choose…' },
];

// Which quick choice a selection is, if it is one of the three simple ones.
function quickChoiceOf(sel: BackupSelection): QuickChoice {
  const picked = !!(sel.sites?.length || sel.site_databases?.length || sel.databases?.length || sel.groups?.length || sel.containers?.length);
  if (picked) return 'choose';
  if (sel.all_sites && sel.all_databases) return 'everything';
  if (sel.all_sites) return 'sites';
  if (sel.all_databases) return 'databases';
  return 'choose';
}

const plural = (n: number, word: string) => `${n} ${word}${n === 1 ? '' : 's'}`;

interface BackupPickerProps {
  value: BackupSelection;
  onChange: (value: BackupSelection) => void;
  disabled?: boolean;
  // Schedules say "every site (also ones added later)"; a backup made now just means the ones there are.
  forSchedule?: boolean;
}

// A list of sites, each with a "Site" (its files) and a "Databases" (the databases that belong to it) tick box, then the
// databases that don't belong to any site. Whatever is ticked goes into one .litpkg.
export default function BackupPicker({ value, onChange, disabled, forSchedule }: BackupPickerProps) {
  const [sites, setSites] = useState<TargetSite[]>([]);
  const [databases, setDatabases] = useState<TargetDB[]>([]);
  const [isAdmin, setIsAdmin] = useState(false);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  // Stays on the list once someone asks for it, even while what they've ticked happens to equal a quick choice.
  const [choosing, setChoosing] = useState(() => quickChoiceOf(value) === 'choose' && !selectionIsEmpty(value));

  const load = useCallback(async () => {
    try {
      const res = await fetch('/api/backups/targets', { cache: 'no-store' });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || 'Could not load sites and databases');
      setSites(data.sites || []);
      setDatabases(data.databases || []);
      setIsAdmin(Boolean(data.is_admin));
      setError('');
    } catch (err: any) {
      setError(err.message || 'Could not load sites and databases');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  // How big the package would be, asked of the server a moment after the ticks stop changing.
  const [estimate, setEstimate] = useState<Estimate | null>(null);
  const [estimating, setEstimating] = useState(false);
  const selectionKey = JSON.stringify(value);
  useEffect(() => {
    if (selectionIsEmpty(value)) {
      setEstimate(null);
      return;
    }
    let cancelled = false;
    setEstimating(true);
    const t = setTimeout(async () => {
      try {
        const res = await fetch('/api/backups/estimate', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ selection: value }) });
        const data = await res.json();
        if (!cancelled) setEstimate(res.ok ? data : null);
      } catch {
        if (!cancelled) setEstimate(null);
      } finally {
        if (!cancelled) setEstimating(false);
      }
    }, 400);
    return () => {
      cancelled = true;
      clearTimeout(t);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [selectionKey]);

  // Non-admins pick from the databases they can reach, whatever site those belong to.
  const loose = useMemo(() => (isAdmin ? databases.filter((d) => d.site_id === null) : databases), [databases, isAdmin]);

  const linkDatabase = async (databaseId: number, siteId: number) => {
    const res = await fetch('/api/backups/links', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ database_id: databaseId, site_id: siteId }),
    });
    if (res.ok) {
      // A database that moves under a site is now picked through that site's box.
      onChange({ ...value, databases: (value.databases || []).filter((id) => id !== databaseId) });
      load();
    }
  };

  const allSites = !!value.all_sites;
  const allDatabases = !!value.all_databases;
  const siteFiles = (id: number) => allSites || (value.sites || []).includes(id);
  const siteDbs = (id: number) => allDatabases || (value.site_databases || []).includes(id);
  const looseDb = (id: number) => allDatabases || (value.databases || []).includes(id);

  const summary = useMemo(() => {
    if (selectionIsEmpty(value)) return 'Nothing selected yet';
    const parts: string[] = [];
    if (allSites) parts.push(forSchedule ? 'every site (including new ones)' : `all ${plural(sites.length, 'site')}`);
    else if (value.sites?.length) {
      const names = sites.filter((s) => value.sites!.includes(s.id)).map((s) => s.domain);
      parts.push(names.length <= 2 ? names.join(', ') : plural(names.length, 'site'));
    }
    if (allDatabases) parts.push(forSchedule ? 'every database (including new ones)' : `all ${plural(databases.length, 'database')}`);
    else {
      const ids = new Set<number>(value.databases || []);
      sites.filter((s) => value.site_databases?.includes(s.id)).forEach((s) => s.databases.forEach((d) => ids.add(d.id)));
      if (ids.size) parts.push(plural(ids.size, 'database'));
    }
    return `Backs up ${parts.join(' + ')}, all in one .litpkg file`;
  }, [value, sites, databases, allSites, allDatabases, forSchedule]);

  if (loading) return <div className="text-sm text-ink-muted py-4">Loading sites and databases…</div>;
  if (error) return <div className="text-sm py-4" style={{ color: 'var(--status-error)' }}>{error}</div>;

  const box = 'w-4 h-4 cursor-pointer disabled:cursor-not-allowed';
  // Narrow checkbox columns on phones so the domain keeps most of the row
  const row = 'grid grid-cols-[1fr_2.5rem_3rem] sm:grid-cols-[1fr_4.5rem_5.5rem] items-center gap-2 px-2 sm:px-3 py-2';

  const quick = choosing ? 'choose' : quickChoiceOf(value);
  const pickQuick = (id: QuickChoice) => {
    setChoosing(id === 'choose');
    if (id === 'everything') onChange({ all_sites: true, all_databases: true });
    if (id === 'sites') onChange({ all_sites: true });
    if (id === 'databases') onChange({ all_databases: true });
    // Choosing starts from nothing, so every container can be ticked or left alone.
    if (id === 'choose' && quickChoiceOf(value) !== 'choose') onChange({});
  };

  return (
    <div className="space-y-3">
      {isAdmin && (
        <div role="radiogroup" aria-label="What to back up" className="grid grid-cols-2 sm:grid-cols-4 gap-2">
          {QUICK.map((q) => (
            <button
              key={q.id}
              type="button"
              role="radio"
              aria-checked={quick === q.id}
              disabled={disabled}
              onClick={() => pickQuick(q.id)}
              className={`px-3 py-2 rounded-lg text-sm font-bold border transition-colors ${
                quick === q.id ? 'border-neon-cyan bg-neon-cyan/15 text-accent-cyan' : 'border-neon-purple/30 text-ink-muted hover:text-accent-cyan'
              }`}
            >
              {q.label}
            </button>
          ))}
        </div>
      )}

      {(!isAdmin || quick === 'choose') && (
      <div className="rounded-lg border border-neon-purple/30 overflow-hidden">
        <div className={`${row} text-xs uppercase text-ink-muted border-b border-neon-purple/20 bg-surface-muted`}>
          <span>{isAdmin ? 'Site' : 'Database'}</span>
          <span className="text-center">{isAdmin ? 'Site' : ''}</span>
          <span className="text-center">{isAdmin ? <><span className="sm:hidden">DBs</span><span className="hidden sm:inline">Databases</span></> : 'Back up'}</span>
        </div>


        <div className="max-h-[28rem] overflow-y-auto">
          {isAdmin && sites.length === 0 && <div className="px-3 py-3 text-sm text-ink-muted">No sites yet.</div>}
          {sites.map((site) => (
            <div key={site.id} className={`${row} border-b border-neon-purple/10`}>
              <div className="min-w-0">
                <div className="flex items-center gap-2 text-gray-100 truncate">
                  <Globe size={14} weight="regular" className="shrink-0" />
                  <span className="truncate">{site.domain}</span>
                </div>
                <div className="text-xs text-ink-muted truncate pl-6">
                  {site.databases.length ? site.databases.map((d) => d.name).join(', ') : 'no databases linked'}
                </div>
              </div>
              <span className="text-center">
                <input
                  type="checkbox"
                  className={box}
                  checked={siteFiles(site.id)}
                  disabled={disabled || allSites}
                  onChange={(e) => onChange({ ...value, sites: toggle(value.sites, site.id, e.target.checked) })}
                  aria-label={`${site.domain} files`}
                />
              </span>
              <span className="text-center">
                <input
                  type="checkbox"
                  className={box}
                  checked={site.databases.length > 0 && siteDbs(site.id)}
                  disabled={disabled || allDatabases || site.databases.length === 0}
                  onChange={(e) => onChange({ ...value, site_databases: toggle(value.site_databases, site.id, e.target.checked) })}
                  aria-label={`${site.domain} databases`}
                  title={site.databases.length === 0 ? 'No databases belong to this site. Link one below.' : site.databases.map((d) => d.name).join(', ')}
                />
              </span>
            </div>
          ))}

          {loose.length > 0 && (
            <>
              {isAdmin && (
                <div className="px-3 pt-3 pb-1 text-xs uppercase text-ink-muted">Databases not linked to a site</div>
              )}
              {loose.map((db) => (
                <div key={db.id} className={`${row} border-b border-neon-purple/10`}>
                  <div className="min-w-0">
                    <div className="flex items-center gap-2 text-gray-100 truncate">
                      <Database size={14} weight="regular" className="shrink-0" />
                      <span className="truncate">{db.name}</span>
                      <span className="text-xs text-ink-muted">{db.type}</span>
                    </div>
                    {isAdmin && sites.length > 0 && (
                      <select
                        className="mt-1 ml-6 text-xs bg-surface-muted border border-neon-purple/30 rounded px-1 py-0.5 text-gray-100"
                        value=""
                        disabled={disabled}
                        onChange={(e) => e.target.value && linkDatabase(db.id, Number(e.target.value))}
                        aria-label={`Link ${db.name} to a site`}
                      >
                        <option value="">Link to a site…</option>
                        {sites.map((s) => (
                          <option key={s.id} value={s.id}>
                            {s.domain}
                          </option>
                        ))}
                      </select>
                    )}
                  </div>
                  <span />
                  <span className="text-center">
                    <input
                      type="checkbox"
                      className={box}
                      checked={looseDb(db.id)}
                      disabled={disabled || allDatabases}
                      onChange={(e) => onChange({ ...value, databases: toggle(value.databases, db.id, e.target.checked) })}
                      aria-label={db.name}
                    />
                  </span>
                </div>
              ))}
            </>
          )}
          {!isAdmin && databases.length === 0 && (
            <div className="px-3 py-3 text-sm text-ink-muted">You don&apos;t have access to any databases yet.</div>
          )}
        </div>
      </div>
      )}

      {isAdmin && quick === 'choose' && sites.some((s) => s.databases.length > 0) && (
        <UnlinkHint sites={sites} onUnlink={(id) => linkDatabase(id, 0)} disabled={disabled} />
      )}

      <div className="text-sm text-accent-cyan">{summary}</div>
      {!selectionIsEmpty(value) && <EstimateLine estimate={estimate} loading={estimating} />}
    </div>
  );
}

type Estimate = {
  items: { name: string; kind: string; bytes: number; known: boolean }[];
  raw_bytes: number;
  estimate_bytes: number;
  unknown?: number;
  free_bytes?: number;
};

// "About 120 MB" with what it is made of, and a warning when it wouldn't fit where backups are kept.
function EstimateLine({ estimate, loading }: { estimate: Estimate | null; loading: boolean }) {
  if (!estimate) return loading ? <div className="text-xs text-ink-muted">Working out the size…</div> : null;
  const tooBig = estimate.free_bytes ? estimate.estimate_bytes > estimate.free_bytes * 0.9 : false;
  const sites = estimate.items.filter((i) => i.kind === 'site');
  const dbs = estimate.items.filter((i) => i.kind !== 'site');
  const sum = (xs: { bytes: number }[]) => xs.reduce((n, x) => n + x.bytes, 0);
  return (
    <div className="text-xs space-y-1" style={{ color: 'var(--text-secondary)' }}>
      <div>
        <span style={{ color: 'var(--text-primary)' }}>Projected size: about {formatBytes(estimate.estimate_bytes)}</span>
        {' '}
        ({formatBytes(estimate.raw_bytes)} before compressing
        {sites.length > 0 ? `; sites ${formatBytes(sum(sites))}` : ''}
        {dbs.length > 0 ? `; databases ${formatBytes(sum(dbs))}` : ''})
        {loading ? ' …' : ''}
      </div>
      {estimate.unknown ? (
        <div>{estimate.unknown} {estimate.unknown === 1 ? 'database' : 'databases'} couldn&apos;t be measured (stopped, or a file database), so the real size may be a bit more.</div>
      ) : null}
      {estimate.free_bytes ? (
        <div style={{ color: tooBig ? 'var(--status-error)' : undefined }}>
          {formatBytes(estimate.free_bytes)} free on this server{tooBig ? ': this may not fit' : ''}.
        </div>
      ) : null}
    </div>
  );
}

// A quiet way to undo a link made by mistake.
function UnlinkHint({ sites, onUnlink, disabled }: { sites: TargetSite[]; onUnlink: (databaseId: number) => void; disabled?: boolean }) {
  const linked = sites.flatMap((s) => s.databases.map((d) => ({ ...d, domain: s.domain })));
  return (
    <details className="text-xs text-ink-muted">
      <summary className="cursor-pointer">Linked databases ({linked.length})</summary>
      <div className="mt-2 space-y-1">
        {linked.map((d) => (
          <div key={d.id} className="flex items-center gap-2">
            <span>
              {d.name} → {d.domain}
            </span>
            <button type="button" className="underline hover:text-accent-pink" disabled={disabled} onClick={() => onUnlink(d.id)}>
              unlink
            </button>
          </div>
        ))}
      </div>
    </details>
  );
}
