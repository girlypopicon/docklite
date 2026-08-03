'use client';

import { useEffect, useState } from 'react';
import ConfirmModal from './ConfirmModal';
import { useToast } from '@/lib/hooks/useToast';
import {
  Cube,
  X,
  Play,
  ArrowClockwise,
  Stop,
  Trash,
  Eye,
  EyeSlash,
  FilePlus,
  CheckCircle,
  Warning,
  Question,
  MagnifyingGlass,
  SpinnerGap,
  Tag,
  FolderSimple,
} from '@phosphor-icons/react';

interface DockerContainer {
  id: string;
  name: string;
  image: string;
  state: string;
  status: string;
  created: number;
  labels: Record<string, string>;
  tracked: boolean;
  siteId?: number;
  domain?: string;
  codePath?: string;
  hasManifest: boolean;
}

interface ScannedSite {
  path: string;
  manifest: {
    domain: string;
    templateType: string;
    image: string;
    username: string;
    createdAt: string;
  };
  registered: boolean;
}

interface ClaimForm {
  domain: string;
  templateType: string;
  codePath: string;
}

interface AllContainersModalProps {
  onClose: () => void;
}

function trackingTier(c: DockerContainer): 'managed' | 'no-manifest' | 'tracked' | 'untracked' {
  if (c.siteId) return c.hasManifest ? 'managed' : 'no-manifest';
  if (c.tracked) return 'tracked';
  return 'untracked';
}

const TIER_LABELS: Record<string, { label: string; color: string; desc: string }> = {
  managed:     { label: 'DockLite Site', color: 'var(--neon-cyan)',    desc: 'Site record + .dkl manifest — survives reinstalls' },
  'no-manifest': { label: 'Missing .dkl', color: 'var(--status-warning)', desc: 'Site record exists but no .dkl — won\'t survive a reinstall' },
  tracked:     { label: 'Tracked',       color: 'var(--neon-purple)',  desc: 'DockLite watches this container but has no site record' },
  untracked:   { label: 'Untracked',     color: 'var(--text-muted)',   desc: 'DockLite ignores this container' },
};

export default function AllContainersModal({ onClose }: AllContainersModalProps) {
  const [containers, setContainers] = useState<DockerContainer[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [containerToDelete, setContainerToDelete] = useState<{ id: string; name: string } | null>(null);
  const [claimTarget, setClaimTarget] = useState<DockerContainer | null>(null);
  const [claimForm, setClaimForm] = useState<ClaimForm>({ domain: '', templateType: 'static', codePath: '' });
  const [claimSaving, setClaimSaving] = useState(false);
  const [scannedSites, setScannedSites] = useState<ScannedSite[] | null>(null);
  const [scanning, setScanning] = useState(false);
  const [onboarding, setOnboarding] = useState<string | null>(null);
  const toast = useToast();

  const fetchContainers = async () => {
    setLoading(true);
    setError('');
    try {
      const res = await fetch('/api/containers/all');
      if (!res.ok) throw new Error('Failed to fetch containers');
      const data = await res.json();
      setContainers(data.containers || []);
    } catch (err: any) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { fetchContainers(); }, []);

  const handleAction = async (containerId: string, action: 'start' | 'stop' | 'restart' | 'delete', name?: string) => {
    if (action === 'delete') {
      setContainerToDelete({ id: containerId, name: name || containerId });
      return;
    }
    try {
      const res = await fetch(`/api/containers/${containerId}/${action}`, { method: 'POST' });
      if (!res.ok) throw new Error((await res.json().catch(() => ({}))).error || `Failed to ${action}`);
      toast.success(`Container ${action}ed`);
      fetchContainers();
    } catch (err: any) {
      toast.error(err.message);
    }
  };

  const handleTracking = async (containerId: string, tracked: boolean) => {
    try {
      const res = await fetch(`/api/containers/${containerId}/${tracked ? 'untrack' : 'track'}`, { method: 'POST' });
      if (!res.ok) throw new Error((await res.json().catch(() => ({}))).error || 'Failed');
      toast.success(tracked ? 'Container untracked' : 'Container tracked');
      fetchContainers();
    } catch (err: any) {
      toast.error(err.message);
    }
  };

  const writeManifest = async (containerId: string) => {
    try {
      const res = await fetch(`/api/containers/${containerId}/write-manifest`, { method: 'POST' });
      const data = await res.json();
      if (!res.ok || !data.ok) throw new Error(data.error || 'Failed to write manifest');
      toast.success(`.dkl manifest written to ${data.path}`);
      fetchContainers();
    } catch (err: any) {
      toast.error(err.message);
    }
  };

  const openClaim = (c: DockerContainer) => {
    setClaimTarget(c);
    setClaimForm({ domain: '', templateType: 'static', codePath: '' });
  };

  const submitClaim = async () => {
    if (!claimTarget || !claimForm.domain.trim()) return;
    setClaimSaving(true);
    try {
      const res = await fetch(`/api/containers/${claimTarget.id}/claim`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(claimForm),
      });
      const data = await res.json();
      if (!res.ok || !data.ok) throw new Error(data.error || 'Failed to claim container');
      toast.success(`Container claimed as ${claimForm.domain} — .dkl written`);
      setClaimTarget(null);
      fetchContainers();
    } catch (err: any) {
      toast.error(err.message);
    } finally {
      setClaimSaving(false);
    }
  };

  const scanDisk = async () => {
    setScanning(true);
    try {
      const res = await fetch('/api/containers/scan');
      if (!res.ok) throw new Error('Scan failed');
      const data = await res.json();
      setScannedSites(data.sites || []);
    } catch (err: any) {
      toast.error(err.message);
    } finally {
      setScanning(false);
    }
  };

  const onboardSite = async (path: string) => {
    setOnboarding(path);
    try {
      const res = await fetch('/api/containers/onboard', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ path }),
      });
      const data = await res.json();
      if (!res.ok || !data.success) throw new Error(data.error || 'Onboard failed');
      toast.success('Site onboarded successfully');
      fetchContainers();
      scanDisk();
    } catch (err: any) {
      toast.error(err.message);
    } finally {
      setOnboarding(null);
    }
  };

  const handleDeleteConfirm = async () => {
    if (!containerToDelete) return;
    try {
      const res = await fetch(`/api/containers/${containerToDelete.id}/delete`, { method: 'POST' });
      if (!res.ok) throw new Error((await res.json().catch(() => ({}))).error || 'Delete failed');
      toast.success('Container deleted');
      fetchContainers();
    } catch (err: any) {
      toast.error(err.message);
    } finally {
      setContainerToDelete(null);
    }
  };

  const tiers: Array<{ key: string; containers: DockerContainer[] }> = [
    { key: 'managed',     containers: containers.filter(c => trackingTier(c) === 'managed') },
    { key: 'no-manifest', containers: containers.filter(c => trackingTier(c) === 'no-manifest') },
    { key: 'tracked',     containers: containers.filter(c => trackingTier(c) === 'tracked') },
    { key: 'untracked',   containers: containers.filter(c => trackingTier(c) === 'untracked') },
  ].filter(t => t.containers.length > 0);

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center p-4"
      style={{ background: 'var(--modal-backdrop)' }}
    >
      <div
        className="relative w-full max-w-3xl max-h-[90vh] overflow-y-auto rounded-2xl p-6 space-y-5"
        style={{ background: 'var(--bg-dark)', border: '1px solid rgba(var(--neon-purple-rgb), 0.3)' }}
      >
        {/* Header */}
        <div className="flex items-center justify-between">
          <div>
            <h2 className="text-xl font-bold flex items-center gap-2" style={{ color: 'var(--neon-cyan)' }}>
              <Cube size={20} weight="duotone" /> All Containers
            </h2>
            <p className="text-xs font-mono mt-0.5" style={{ color: 'var(--text-secondary)' }}>
              Track, untrack, and manage .dkl manifests for all Docker containers.
            </p>
          </div>
          <button onClick={onClose} style={{ color: 'var(--text-secondary)' }}><X size={20} /></button>
        </div>

        {/* Legend */}
        <div className="flex flex-wrap gap-2">
          {Object.entries(TIER_LABELS).map(([key, t]) => (
            <span key={key} className="text-[10px] font-mono px-2 py-1 rounded-full flex items-center gap-1"
              style={{ background: 'rgba(0,0,0,0.2)', border: `1px solid ${t.color}`, color: t.color }}>
              {t.label}
            </span>
          ))}
        </div>

        {loading ? (
          <div className="flex items-center gap-2 text-sm font-mono" style={{ color: 'var(--text-secondary)' }}>
            <SpinnerGap size={14} className="animate-spin" /> Loading containers…
          </div>
        ) : error ? (
          <div className="text-sm font-mono" style={{ color: 'var(--status-error)' }}>{error}</div>
        ) : (
          <div className="space-y-5">
            {tiers.map(({ key, containers: group }) => {
              const tier = TIER_LABELS[key];
              return (
                <div key={key}>
                  <div className="flex items-center gap-2 mb-2">
                    <span className="text-xs font-bold px-2 py-0.5 rounded-full"
                      style={{ background: `color-mix(in srgb, ${tier.color} 15%, transparent)`, color: tier.color }}>
                      {tier.label}
                    </span>
                    <span className="text-[11px] font-mono" style={{ color: 'var(--text-secondary)' }}>{tier.desc}</span>
                  </div>
                  <div className="space-y-2">
                    {group.map(c => (
                      <div key={c.id}
                        className="rounded-xl px-4 py-3 flex flex-wrap items-center gap-3"
                        style={{ background: 'rgba(var(--neon-purple-rgb), 0.05)', border: '1px solid rgba(var(--neon-purple-rgb), 0.15)' }}
                      >
                        {/* Info */}
                        <div className="flex-1 min-w-0">
                          <div className="flex items-center gap-2 flex-wrap">
                            <span className="font-bold text-sm truncate" style={{ color: 'var(--text-primary)' }}>
                              {c.name}
                            </span>
                            {c.domain && (
                              <span className="text-[11px] font-mono flex items-center gap-1" style={{ color: 'var(--neon-cyan)' }}>
                                <Tag size={10} /> {c.domain}
                              </span>
                            )}
                            <span className={`text-[10px] px-1.5 py-0.5 rounded font-mono`}
                              style={{
                                background: c.state === 'running' ? 'rgba(var(--neon-green-rgb),0.12)' : 'rgba(var(--status-error-rgb),0.1)',
                                color: c.state === 'running' ? 'var(--neon-green)' : 'var(--status-error)',
                              }}>
                              {c.state}
                            </span>
                          </div>
                          <div className="text-[11px] font-mono truncate mt-0.5" style={{ color: 'var(--text-secondary)' }}>
                            {c.image}
                          </div>
                          {c.codePath && (
                            <div className="text-[10px] font-mono flex items-center gap-1 mt-0.5" style={{ color: 'var(--text-muted)' }}>
                              <FolderSimple size={10} /> {c.codePath}
                              {key === 'no-manifest' && (
                                <span style={{ color: 'var(--status-warning)' }}> — no .dkl</span>
                              )}
                              {key === 'managed' && (
                                <span style={{ color: 'var(--neon-green)' }}> — .dkl ✓</span>
                              )}
                            </div>
                          )}
                        </div>

                        {/* Actions */}
                        <div className="flex items-center gap-1.5 flex-wrap">
                          {/* Lifecycle */}
                          {c.state === 'running' ? (
                            <>
                              <button onClick={() => handleAction(c.id, 'restart')} title="Restart"
                                className="p-1.5 rounded-lg transition-colors hover:bg-white/10"
                                style={{ color: 'var(--neon-cyan)' }}>
                                <ArrowClockwise size={15} weight="duotone" />
                              </button>
                              <button onClick={() => handleAction(c.id, 'stop')} title="Stop"
                                className="p-1.5 rounded-lg transition-colors hover:bg-white/10"
                                style={{ color: 'var(--status-warning)' }}>
                                <Stop size={15} weight="duotone" />
                              </button>
                            </>
                          ) : (
                            <button onClick={() => handleAction(c.id, 'start')} title="Start"
                              className="p-1.5 rounded-lg transition-colors hover:bg-white/10"
                              style={{ color: 'var(--neon-green)' }}>
                              <Play size={15} weight="duotone" />
                            </button>
                          )}

                          {/* Manifest */}
                          {key === 'no-manifest' && (
                            <button onClick={() => writeManifest(c.id)}
                              title="Write .dkl manifest"
                              className="inline-flex items-center gap-1 px-2 py-1 rounded-lg text-[11px] font-bold transition-colors hover:bg-white/10"
                              style={{ color: 'var(--status-warning)', border: '1px solid rgba(var(--status-warning-rgb),0.3)' }}>
                              <FilePlus size={12} weight="duotone" /> Write .dkl
                            </button>
                          )}

                          {/* Claim */}
                          {key === 'untracked' && (
                            <button onClick={() => openClaim(c)}
                              title="Claim as DockLite site"
                              className="inline-flex items-center gap-1 px-2 py-1 rounded-lg text-[11px] font-bold transition-colors hover:bg-white/10"
                              style={{ color: 'var(--neon-cyan)', border: '1px solid rgba(var(--neon-cyan-rgb),0.3)' }}>
                              <CheckCircle size={12} weight="duotone" /> Claim
                            </button>
                          )}

                          {/* Track / Untrack */}
                          {(key === 'untracked') && (
                            <button onClick={() => handleTracking(c.id, false)}
                              title="Track container"
                              className="p-1.5 rounded-lg transition-colors hover:bg-white/10"
                              style={{ color: 'var(--neon-purple)' }}>
                              <Eye size={15} weight="duotone" />
                            </button>
                          )}
                          {(key === 'tracked') && (
                            <button onClick={() => handleTracking(c.id, true)}
                              title="Untrack container"
                              className="p-1.5 rounded-lg transition-colors hover:bg-white/10"
                              style={{ color: 'var(--text-muted)' }}>
                              <EyeSlash size={15} weight="duotone" />
                            </button>
                          )}

                          {/* Delete */}
                          <button onClick={() => handleAction(c.id, 'delete', c.name)} title="Delete"
                            className="p-1.5 rounded-lg transition-colors hover:bg-white/10"
                            style={{ color: 'var(--status-error)' }}>
                            <Trash size={15} weight="duotone" />
                          </button>
                        </div>
                      </div>
                    ))}
                  </div>
                </div>
              );
            })}
          </div>
        )}

        {/* Claim form */}
        {claimTarget && (
          <div className="rounded-xl p-4 space-y-3"
            style={{ background: 'rgba(var(--neon-cyan-rgb),0.06)', border: '1px solid rgba(var(--neon-cyan-rgb),0.25)' }}>
            <div className="font-bold text-sm" style={{ color: 'var(--neon-cyan)' }}>
              Claim &ldquo;{claimTarget.name}&rdquo; as a DockLite site
            </div>
            <p className="text-xs font-mono" style={{ color: 'var(--text-secondary)' }}>
              This creates a site record and writes a .dkl manifest so DockLite manages and tracks this container.
            </p>
            <div className="grid grid-cols-2 gap-3">
              <div>
                <label className="text-[11px] font-mono mb-1 block" style={{ color: 'var(--text-secondary)' }}>Domain *</label>
                <input
                  value={claimForm.domain}
                  onChange={e => setClaimForm(f => ({ ...f, domain: e.target.value }))}
                  placeholder="example.com"
                  className="w-full text-xs font-mono px-3 py-2 rounded-lg outline-none"
                  style={{ background: 'var(--bg-darker)', color: 'var(--text-primary)', border: '1px solid rgba(var(--neon-purple-rgb),0.3)' }}
                />
              </div>
              <div>
                <label className="text-[11px] font-mono mb-1 block" style={{ color: 'var(--text-secondary)' }}>Template type</label>
                <select
                  value={claimForm.templateType}
                  onChange={e => setClaimForm(f => ({ ...f, templateType: e.target.value }))}
                  className="w-full text-xs font-mono px-3 py-2 rounded-lg outline-none"
                  style={{ background: 'var(--bg-darker)', color: 'var(--text-primary)', border: '1px solid rgba(var(--neon-purple-rgb),0.3)' }}
                >
                  <option value="static">Static (nginx)</option>
                  <option value="node">Node.js</option>
                  <option value="php">PHP</option>
                </select>
              </div>
              <div className="col-span-2">
                <label className="text-[11px] font-mono mb-1 block" style={{ color: 'var(--text-secondary)' }}>
                  Code path <span style={{ color: 'var(--text-muted)' }}>(leave blank for default)</span>
                </label>
                <input
                  value={claimForm.codePath}
                  onChange={e => setClaimForm(f => ({ ...f, codePath: e.target.value }))}
                  placeholder="/var/www/sites/username/domain"
                  className="w-full text-xs font-mono px-3 py-2 rounded-lg outline-none"
                  style={{ background: 'var(--bg-darker)', color: 'var(--text-primary)', border: '1px solid rgba(var(--neon-purple-rgb),0.3)' }}
                />
              </div>
            </div>
            <div className="flex gap-2">
              <button onClick={submitClaim} disabled={claimSaving || !claimForm.domain.trim()}
                className="inline-flex items-center gap-2 px-3 py-1.5 rounded-lg text-xs font-bold disabled:opacity-50"
                style={{ background: 'rgba(var(--neon-cyan-rgb),0.15)', color: 'var(--neon-cyan)', border: '1px solid rgba(var(--neon-cyan-rgb),0.3)' }}>
                {claimSaving ? <SpinnerGap size={12} className="animate-spin" /> : <CheckCircle size={12} weight="duotone" />}
                Claim + Write .dkl
              </button>
              <button onClick={() => setClaimTarget(null)} className="px-3 py-1.5 rounded-lg text-xs font-bold"
                style={{ color: 'var(--text-secondary)' }}>Cancel</button>
            </div>
          </div>
        )}

        {/* Disk scan */}
        <div className="rounded-xl p-4 space-y-3"
          style={{ background: 'rgba(var(--neon-purple-rgb),0.04)', border: '1px solid rgba(var(--neon-purple-rgb),0.15)' }}>
          <div className="flex items-center justify-between">
            <div>
              <div className="font-bold text-sm flex items-center gap-2" style={{ color: 'var(--neon-purple)' }}>
                <MagnifyingGlass size={15} weight="duotone" /> Scan Disk for .dkl Files
              </div>
              <p className="text-[11px] font-mono mt-0.5" style={{ color: 'var(--text-secondary)' }}>
                Finds .dkl manifests under /var/www/sites that aren&apos;t registered in DockLite.
              </p>
            </div>
            <button onClick={scanDisk} disabled={scanning}
              className="inline-flex items-center gap-2 px-3 py-1.5 rounded-lg text-xs font-bold disabled:opacity-50"
              style={{ background: 'rgba(var(--neon-purple-rgb),0.15)', color: 'var(--neon-purple)', border: '1px solid rgba(var(--neon-purple-rgb),0.3)' }}>
              {scanning ? <SpinnerGap size={12} className="animate-spin" /> : <MagnifyingGlass size={12} weight="duotone" />}
              Scan
            </button>
          </div>

          {scannedSites !== null && (
            scannedSites.length === 0 ? (
              <div className="text-xs font-mono" style={{ color: 'var(--neon-green)' }}>
                ✓ All .dkl sites are registered.
              </div>
            ) : (
              <div className="space-y-2">
                {scannedSites.map(s => (
                  <div key={s.path}
                    className="flex items-center justify-between rounded-lg px-3 py-2.5"
                    style={{ background: 'rgba(var(--neon-purple-rgb),0.08)' }}>
                    <div>
                      <div className="text-sm font-bold" style={{ color: s.registered ? 'var(--text-secondary)' : 'var(--text-primary)' }}>
                        {s.manifest.domain}
                        {s.registered && <span className="ml-2 text-[10px]" style={{ color: 'var(--neon-green)' }}>registered</span>}
                      </div>
                      <div className="text-[11px] font-mono" style={{ color: 'var(--text-secondary)' }}>
                        {s.path} · {s.manifest.templateType} · {s.manifest.username}
                      </div>
                    </div>
                    {!s.registered && (
                      <button onClick={() => onboardSite(s.path)} disabled={onboarding === s.path}
                        className="inline-flex items-center gap-1 px-2 py-1 rounded-lg text-[11px] font-bold disabled:opacity-50"
                        style={{ background: 'rgba(var(--neon-cyan-rgb),0.15)', color: 'var(--neon-cyan)', border: '1px solid rgba(var(--neon-cyan-rgb),0.3)' }}>
                        {onboarding === s.path ? <SpinnerGap size={11} className="animate-spin" /> : null}
                        Onboard
                      </button>
                    )}
                  </div>
                ))}
              </div>
            )
          )}
        </div>
      </div>

      {containerToDelete && (
        <ConfirmModal
          title="Delete Container"
          message={`Are you sure you want to delete "${containerToDelete.name}"? This cannot be undone.`}
          onConfirm={handleDeleteConfirm}
          onCancel={() => setContainerToDelete(null)}
        />
      )}
    </div>
  );
}
