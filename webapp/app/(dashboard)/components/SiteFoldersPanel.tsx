'use client';

import { useCallback, useEffect, useState } from 'react';
import { ArrowClockwise, FolderOpen, Trash, WarningCircle, Plus, Broom } from '@phosphor-icons/react';
import { useToast } from '@/lib/hooks/useToast';
import { formatBytes } from '@/lib/utils/formatters';
import ConfirmModal from './ConfirmModal';

/**
 * What is actually on disk under /var/www/sites, compared with what DockLite knows about.
 * Admins can adopt folders DockLite doesn't know as sites, move orphans to the trash, and
 * empty the trash (which DockLite otherwise empties on its own after a number of days).
 */

interface FolderEntry {
  path: string;
  name: string;
  depth: number;
  status: 'user-dir' | 'registered' | 'unregistered' | 'unknown';
  owner?: string;
  detected_type?: string;
  files: number;
  size_bytes: number;
  mounted_by?: string;
  site_id?: number;
  can_remove: boolean;
  note?: string;
}

interface Ghost {
  site_id: number;
  domain: string;
  path: string;
}

interface TrashEntry {
  name: string;
  path: string;
  original: string;
  trashed_at: string;
  age_days: number;
  files: number;
  size_bytes: number;
  delete_on?: string;
}

interface Report {
  folders: FolderEntry[];
  ghosts: Ghost[];
  trash: TrashEntry[];
  trash_keep_days: number;
}

interface Pending {
  title: string;
  message: string;
  confirmText: string;
  run: () => Promise<void>;
}

const rowStyle = { border: '1px solid rgba(var(--text-muted-rgb), 0.25)' } as const;

function describe(e: FolderEntry): string {
  const parts = [`${e.files} file${e.files === 1 ? '' : 's'}`, formatBytes(e.size_bytes)];
  if (e.detected_type) parts.push(`looks ${e.detected_type}`);
  if (e.mounted_by) parts.push(`used by container ${e.mounted_by}`);
  return parts.join(' · ');
}

function dayLabel(iso?: string): string {
  if (!iso) return '';
  const d = new Date(iso);
  return isNaN(d.getTime()) ? '' : d.toLocaleDateString();
}

export default function SiteFoldersPanel() {
  const toast = useToast();
  const [report, setReport] = useState<Report | null>(null);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState<string | null>(null);
  const [pending, setPending] = useState<Pending | null>(null);
  const [users, setUsers] = useState<Array<{ id: number; username: string }>>([]);
  const [adopting, setAdopting] = useState<FolderEntry | null>(null);
  const [adoptUserId, setAdoptUserId] = useState('');
  const [adoptType, setAdoptType] = useState('');
  const [checkResult, setCheckResult] = useState<string | null>(null);

  const load = useCallback(async () => {
    try {
      const res = await fetch('/api/sites/folders');
      const data = await res.json().catch(() => ({}));
      if (!res.ok) throw new Error(data.error || 'Could not read the site folders');
      setReport(data);
    } catch (err: any) {
      toast.error(err.message || 'Could not read the site folders');
    } finally {
      setLoading(false);
    }
  }, [toast]);

  useEffect(() => {
    load();
  }, [load]);

  const post = async (url: string, body: unknown) => {
    const res = await fetch(url, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) });
    const data = await res.json().catch(() => ({}));
    if (!res.ok) throw new Error(data.error || 'The request failed');
    return data;
  };

  const act = async (key: string, fn: () => Promise<string>) => {
    setBusy(key);
    try {
      toast.success(await fn());
      await load();
    } catch (err: any) {
      toast.error(err.message || 'The request failed');
    } finally {
      setBusy(null);
      setPending(null);
    }
  };

  const trashFolder = (e: FolderEntry) =>
    setPending({
      title: 'Move folder to the trash',
      message: `${e.path} (${describe(e)}) will go to the trash. Nothing is deleted yet: the trash keeps it${
        report?.trash_keep_days ? ` for ${report.trash_keep_days} days` : ''
      }, and you can delete it sooner from the trash list below.`,
      confirmText: 'Move to trash',
      run: () => act(e.path, async () => {
        await post('/api/sites/folders/trash', { path: e.path });
        return `Moved ${e.name} to the trash`;
      }),
    });

  const purge = (t: TrashEntry) =>
    setPending({
      title: 'Delete for good',
      message: `${t.original} (${t.files} files, ${formatBytes(t.size_bytes)}) will be deleted from the trash. This cannot be undone.`,
      confirmText: 'Delete',
      run: () => act(t.name, async () => {
        await post('/api/sites/folders/trash/purge', { name: t.name });
        return `Deleted ${t.original}`;
      }),
    });

  const emptyTrash = () =>
    setPending({
      title: 'Empty the trash',
      message: `Every folder in the trash (${report?.trash.length ?? 0}) will be deleted. This cannot be undone.`,
      confirmText: 'Empty trash',
      run: () => act('empty', async () => {
        const data = await post('/api/sites/folders/trash/purge', { all: true });
        return `Deleted ${data.deleted?.length ?? 0} folder${data.deleted?.length === 1 ? '' : 's'} from the trash`;
      }),
    });

  const startAdopt = async (e: FolderEntry) => {
    setAdopting(e);
    setAdoptType(e.detected_type || 'static');
    setAdoptUserId('');
    if (users.length === 0) {
      try {
        const res = await fetch('/api/users');
        const data = await res.json().catch(() => ({}));
        setUsers(data.users || []);
      } catch {
        setUsers([]);
      }
    }
  };

  const adopt = () => {
    if (!adopting) return;
    const e = adopting;
    const body: Record<string, unknown> = { path: e.path, template_type: adoptType };
    if (e.depth === 1) {
      if (!adoptUserId) {
        toast.error('Choose which user this site belongs to');
        return;
      }
      body.user_id = Number(adoptUserId);
    }
    act(e.path, async () => {
      const data = await post('/api/sites/folders/adopt', body);
      setAdopting(null);
      return `${e.name} is now a ${data.template_type} site owned by ${data.owner}`;
    });
  };

  const checkUserFolders = () =>
    act('check', async () => {
      const data = await post('/api/system/check-folders', {});
      setCheckResult(`Checked ${data.userCount ?? data.checked ?? 0} users`);
      return 'Every user has a folder';
    });

  const attention = (report?.folders ?? []).filter((f) => f.status === 'unregistered' || f.status === 'unknown');
  const strayUserDirs = attention.filter((f) => f.status === 'unknown' && f.depth === 1 && !f.note?.includes('domain'));
  const trash = report?.trash ?? [];

  return (
    <div className="card-vapor p-6 rounded-xl">
      <div className="flex items-start justify-between gap-4 mb-4">
        <div>
          <h2 className="text-2xl font-bold neon-text flex items-center gap-2" style={{ color: 'var(--text-pink)' }}>
            <FolderOpen size={20} weight="duotone" />
            Site folders
          </h2>
          <p className="text-sm mt-1" style={{ color: 'var(--text-secondary)' }}>
            Every site lives in <span className="font-mono">/var/www/sites/&lt;user&gt;/&lt;domain&gt;</span>. Folders left behind by
            deleted sites, transfers or hand copies show up here: adopt them as sites or move them to the trash.
            {report?.trash_keep_days
              ? ` The trash empties itself after ${report.trash_keep_days} days.`
              : ' The trash is only emptied by hand.'}
          </p>
        </div>
        <button
          onClick={() => load()}
          disabled={loading}
          className="px-3 py-2 rounded-lg text-sm font-bold inline-flex items-center gap-2 transition-all hover:scale-105 disabled:opacity-50"
          style={{ background: 'rgba(var(--text-muted-rgb), 0.15)', color: 'var(--text-primary)' }}
          title="Scan again"
        >
          <ArrowClockwise size={16} weight="duotone" className={loading ? 'animate-spin' : ''} />
          Rescan
        </button>
      </div>

      {loading && !report ? (
        <p className="text-sm font-mono" style={{ color: 'var(--text-secondary)' }}>Scanning /var/www/sites…</p>
      ) : (
        <div className="space-y-6">
          {/* Folders DockLite doesn't know about */}
          <section>
            <h3 className="font-bold mb-2 flex items-center gap-2" style={{ color: 'var(--text-yellow)' }}>
              <WarningCircle size={18} weight="duotone" />
              Needs a decision ({attention.length})
            </h3>
            {attention.length === 0 ? (
              <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>Every folder belongs to a user and a registered site.</p>
            ) : (
              <ul className="space-y-2">
                {attention.map((e) => (
                  <li key={e.path} className="p-3 rounded-lg flex flex-wrap items-center justify-between gap-3" style={rowStyle}>
                    <div className="min-w-0">
                      <div className="font-mono text-sm break-all" style={{ color: 'var(--text-primary)' }}>{e.path}</div>
                      <div className="text-xs" style={{ color: 'var(--text-secondary)' }}>
                        {e.note ? `${e.note} · ` : ''}{describe(e)}
                      </div>
                    </div>
                    <div className="flex items-center gap-2">
                      {e.status === 'unregistered' && (
                        <button
                          onClick={() => startAdopt(e)}
                          disabled={busy !== null}
                          className="px-3 py-1.5 rounded-lg text-sm font-bold inline-flex items-center gap-1 transition-all hover:scale-105 disabled:opacity-50"
                          style={{ background: 'rgba(var(--status-success-rgb), 0.15)', color: 'var(--text-green)', border: '1px solid var(--neon-green)' }}
                        >
                          <Plus size={14} weight="bold" /> Adopt as site
                        </button>
                      )}
                      <button
                        onClick={() => trashFolder(e)}
                        disabled={busy !== null || !e.can_remove}
                        title={e.can_remove ? 'Move to the trash' : e.mounted_by ? `A container (${e.mounted_by}) still uses it` : 'Not empty; deal with the sites inside first'}
                        className="px-3 py-1.5 rounded-lg text-sm font-bold inline-flex items-center gap-1 transition-all hover:scale-105 disabled:opacity-40 disabled:cursor-not-allowed"
                        style={{ background: 'rgba(var(--status-error-rgb), 0.12)', color: 'var(--status-error)', border: '1px solid var(--status-error)' }}
                      >
                        <Trash size={14} weight="bold" /> Move to trash
                      </button>
                    </div>
                  </li>
                ))}
              </ul>
            )}
            {strayUserDirs.length > 0 && (
              <p className="text-xs mt-2" style={{ color: 'var(--text-secondary)' }}>
                A folder named after a user who no longer exists can only go to the trash once the site folders inside it have been adopted or trashed.
              </p>
            )}
          </section>

          {/* Sites whose folder is missing */}
          {(report?.ghosts.length ?? 0) > 0 && (
            <section>
              <h3 className="font-bold mb-2" style={{ color: 'var(--text-purple)' }}>Sites with no folder ({report!.ghosts.length})</h3>
              <ul className="space-y-1">
                {report!.ghosts.map((g) => (
                  <li key={g.site_id} className="text-sm font-mono" style={{ color: 'var(--text-secondary)' }}>
                    {g.domain} expects {g.path}. Restore it from a backup, or delete the site from the Containers page.
                  </li>
                ))}
              </ul>
            </section>
          )}

          {/* Trash */}
          <section>
            <div className="flex items-center justify-between mb-2">
              <h3 className="font-bold flex items-center gap-2" style={{ color: 'var(--text-cyan)' }}>
                <Trash size={18} weight="duotone" />
                Trash ({trash.length})
              </h3>
              {trash.length > 0 && (
                <button
                  onClick={emptyTrash}
                  disabled={busy !== null}
                  className="px-3 py-1.5 rounded-lg text-sm font-bold inline-flex items-center gap-1 transition-all hover:scale-105 disabled:opacity-50"
                  style={{ background: 'rgba(var(--status-error-rgb), 0.12)', color: 'var(--status-error)', border: '1px solid var(--status-error)' }}
                >
                  <Broom size={14} weight="bold" /> Empty trash
                </button>
              )}
            </div>
            {trash.length === 0 ? (
              <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>The trash is empty.</p>
            ) : (
              <ul className="space-y-2">
                {trash.map((t) => (
                  <li key={t.name} className="p-3 rounded-lg flex flex-wrap items-center justify-between gap-3" style={rowStyle}>
                    <div className="min-w-0">
                      <div className="font-mono text-sm break-all" style={{ color: 'var(--text-primary)' }}>{t.original}</div>
                      <div className="text-xs" style={{ color: 'var(--text-secondary)' }}>
                        trashed {dayLabel(t.trashed_at)} ({t.age_days === 0 ? 'today' : `${t.age_days} day${t.age_days === 1 ? '' : 's'} ago`}) ·{' '}
                        {t.files} file{t.files === 1 ? '' : 's'} · {formatBytes(t.size_bytes)}
                        {t.delete_on ? ` · deleted on ${dayLabel(t.delete_on)}` : ''}
                      </div>
                    </div>
                    <button
                      onClick={() => purge(t)}
                      disabled={busy !== null}
                      className="px-3 py-1.5 rounded-lg text-sm font-bold inline-flex items-center gap-1 transition-all hover:scale-105 disabled:opacity-50"
                      style={{ background: 'rgba(var(--status-error-rgb), 0.12)', color: 'var(--status-error)', border: '1px solid var(--status-error)' }}
                    >
                      <Trash size={14} weight="bold" /> Delete now
                    </button>
                  </li>
                ))}
              </ul>
            )}
          </section>

          {/* User home folders */}
          <section className="flex flex-wrap items-center gap-3 pt-2" style={{ borderTop: '1px solid rgba(var(--text-muted-rgb), 0.25)' }}>
            <button
              onClick={checkUserFolders}
              disabled={busy !== null}
              className="px-3 py-1.5 rounded-lg text-sm font-bold inline-flex items-center gap-2 transition-all hover:scale-105 disabled:opacity-50"
              style={{ background: 'rgba(var(--text-muted-rgb), 0.15)', color: 'var(--text-primary)' }}
            >
              <ArrowClockwise size={14} weight="duotone" className={busy === 'check' ? 'animate-spin' : ''} />
              Create missing user folders
            </button>
            <span className="text-xs" style={{ color: 'var(--text-secondary)' }}>
              {checkResult ?? 'Makes sure every user has /var/www/sites/<username>. Runs by itself at startup.'}
            </span>
          </section>
        </div>
      )}

      {pending && (
        <ConfirmModal
          title={pending.title}
          message={pending.message}
          confirmText={busy ? 'Working…' : pending.confirmText}
          onConfirm={() => { if (!busy) pending.run(); }}
          onCancel={() => { if (!busy) setPending(null); }}
          type="danger"
        />
      )}

      {adopting && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4" style={{ background: 'rgba(0,0,0,0.6)' }} onClick={() => !busy && setAdopting(null)}>
          <div className="card-vapor rounded-xl p-6 w-full max-w-md space-y-4" onClick={(ev) => ev.stopPropagation()}>
            <h3 className="text-xl font-bold" style={{ color: 'var(--text-green)' }}>Adopt {adopting.name}</h3>
            <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>
              Registers the folder as a site without touching its files. Start it from the Containers page afterwards.
            </p>
            {adopting.depth === 1 ? (
              <label className="block text-sm">
                <span style={{ color: 'var(--text-secondary)' }}>Owner</span>
                <select
                  value={adoptUserId}
                  onChange={(ev) => setAdoptUserId(ev.target.value)}
                  className="mt-1 w-full p-2 rounded-lg"
                  style={{ background: 'var(--bg-darker)', color: 'var(--text-primary)', border: '1px solid rgba(var(--text-muted-rgb), 0.4)' }}
                >
                  <option value="">Choose a user…</option>
                  {users.map((u) => (
                    <option key={u.id} value={u.id}>{u.username}</option>
                  ))}
                </select>
              </label>
            ) : (
              <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>Owner: <span className="font-mono">{adopting.owner}</span> (from the folder it is in)</p>
            )}
            <label className="block text-sm">
              <span style={{ color: 'var(--text-secondary)' }}>Site type</span>
              <select
                value={adoptType}
                onChange={(ev) => setAdoptType(ev.target.value)}
                className="mt-1 w-full p-2 rounded-lg"
                style={{ background: 'var(--bg-darker)', color: 'var(--text-primary)', border: '1px solid rgba(var(--text-muted-rgb), 0.4)' }}
              >
                <option value="static">Static (HTML)</option>
                <option value="php">PHP</option>
                <option value="node">Node</option>
              </select>
            </label>
            <div className="flex justify-end gap-2">
              <button
                onClick={() => setAdopting(null)}
                disabled={busy !== null}
                className="px-4 py-2 rounded-lg font-bold"
                style={{ background: 'rgba(var(--text-muted-rgb), 0.15)', color: 'var(--text-primary)' }}
              >
                Cancel
              </button>
              <button
                onClick={adopt}
                disabled={busy !== null}
                className="px-4 py-2 rounded-lg font-bold disabled:opacity-50"
                style={{ background: 'linear-gradient(135deg, var(--neon-cyan) 0%, var(--neon-purple) 100%)', color: 'var(--button-text)' }}
              >
                {busy ? 'Working…' : 'Adopt'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
