'use client';

import { useCallback, useEffect, useState } from 'react';
import {
  ArrowClockwise,
  CheckCircle,
  CaretDown,
  CaretUp,
  SpinnerGap,
  Warning,
  XCircle,
  ArrowCounterClockwise,
  FloppyDisk,
  Stack,
} from '@phosphor-icons/react';

interface NginxSite {
  domain: string;
  templateType: string;
  enabled: boolean;
  hasConfig: boolean;
  config?: string;
}

interface SiteEditorState {
  loading: boolean;
  config: string;
  originalConfig: string;
  saving: boolean;
  error: string;
  success: boolean;
}

export default function NginxPanel() {
  const [sites, setSites] = useState<NginxSite[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [expanded, setExpanded] = useState<string | null>(null);
  const [editors, setEditors] = useState<Record<string, SiteEditorState>>({});
  const [testResult, setTestResult] = useState<{ ok: boolean; error?: string } | null>(null);
  const [reloading, setReloading] = useState(false);
  const [testing, setTesting] = useState(false);

  const fetchSites = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const res = await fetch('/api/nginx/sites');
      if (!res.ok) throw new Error('Failed to load nginx sites');
      const data = await res.json();
      setSites(data.sites || []);
    } catch (e: any) {
      setError(e.message || 'Failed to load nginx sites');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { fetchSites(); }, [fetchSites]);

  const expandSite = async (domain: string) => {
    if (expanded === domain) {
      setExpanded(null);
      return;
    }
    setExpanded(domain);
    if (editors[domain]) return; // already loaded

    setEditors(prev => ({
      ...prev,
      [domain]: { loading: true, config: '', originalConfig: '', saving: false, error: '', success: false },
    }));
    try {
      const res = await fetch(`/api/nginx/sites/${encodeURIComponent(domain)}`);
      if (!res.ok) throw new Error('Failed to load config');
      const data = await res.json();
      setEditors(prev => ({
        ...prev,
        [domain]: { loading: false, config: data.config || '', originalConfig: data.config || '', saving: false, error: '', success: false },
      }));
    } catch (e: any) {
      setEditors(prev => ({
        ...prev,
        [domain]: { loading: false, config: '', originalConfig: '', saving: false, error: e.message, success: false },
      }));
    }
  };

  const updateConfig = (domain: string, value: string) => {
    setEditors(prev => ({
      ...prev,
      [domain]: { ...prev[domain], config: value, success: false, error: '' },
    }));
  };

  const resetConfig = (domain: string) => {
    setEditors(prev => ({
      ...prev,
      [domain]: { ...prev[domain], config: prev[domain].originalConfig, success: false, error: '' },
    }));
  };

  const saveConfig = async (domain: string) => {
    const state = editors[domain];
    if (!state) return;
    setEditors(prev => ({ ...prev, [domain]: { ...prev[domain], saving: true, error: '', success: false } }));
    try {
      const res = await fetch(`/api/nginx/sites/${encodeURIComponent(domain)}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ config: state.config }),
      });
      const data = await res.json();
      if (!data.ok) {
        setEditors(prev => ({ ...prev, [domain]: { ...prev[domain], saving: false, error: data.error || 'Save failed' } }));
        return;
      }
      setEditors(prev => ({
        ...prev,
        [domain]: { ...prev[domain], saving: false, success: true, originalConfig: state.config },
      }));
      fetchSites();
    } catch (e: any) {
      setEditors(prev => ({ ...prev, [domain]: { ...prev[domain], saving: false, error: e.message } }));
    }
  };

  const testConfig = async () => {
    setTesting(true);
    setTestResult(null);
    try {
      const res = await fetch('/api/nginx/test', { method: 'POST' });
      const data = await res.json();
      setTestResult(data);
    } catch (e: any) {
      setTestResult({ ok: false, error: e.message });
    } finally {
      setTesting(false);
    }
  };

  const reloadNginx = async () => {
    setReloading(true);
    setTestResult(null);
    try {
      const res = await fetch('/api/nginx/reload', { method: 'POST' });
      const data = await res.json();
      setTestResult(data);
    } catch (e: any) {
      setTestResult({ ok: false, error: e.message });
    } finally {
      setReloading(false);
    }
  };

  const isDirty = (domain: string) => {
    const s = editors[domain];
    return s && s.config !== s.originalConfig;
  };

  return (
    <div className="space-y-4">
      {/* Top actions bar */}
      <div className="card-vapor p-4 rounded-xl flex flex-wrap items-center gap-3">
        <div className="flex items-center gap-2" style={{ color: 'var(--neon-cyan)' }}>
          <Stack size={18} weight="duotone" />
          <span className="font-bold text-sm">Nginx Config Manager</span>
        </div>

        <div className="ml-auto flex items-center gap-2">
          <button
            onClick={testConfig}
            disabled={testing}
            className="inline-flex items-center gap-2 px-3 py-1.5 rounded-lg text-xs font-bold transition-all disabled:opacity-50"
            style={{ background: 'rgba(var(--neon-cyan-rgb), 0.12)', color: 'var(--neon-cyan)', border: '1px solid rgba(var(--neon-cyan-rgb), 0.3)' }}
          >
            {testing ? <SpinnerGap size={13} className="animate-spin" /> : <CheckCircle size={13} weight="duotone" />}
            Test Config
          </button>
          <button
            onClick={reloadNginx}
            disabled={reloading}
            className="inline-flex items-center gap-2 px-3 py-1.5 rounded-lg text-xs font-bold transition-all disabled:opacity-50"
            style={{ background: 'rgba(var(--neon-purple-rgb), 0.12)', color: 'var(--neon-purple)', border: '1px solid rgba(var(--neon-purple-rgb), 0.3)' }}
          >
            {reloading ? <SpinnerGap size={13} className="animate-spin" /> : <ArrowClockwise size={13} weight="duotone" />}
            Reload Nginx
          </button>
        </div>

        {testResult && (
          <div
            className="w-full text-xs font-mono px-3 py-2 rounded-lg mt-1"
            style={{
              background: testResult.ok ? 'rgba(var(--neon-green-rgb), 0.1)' : 'rgba(var(--status-error-rgb), 0.1)',
              color: testResult.ok ? 'var(--neon-green)' : 'var(--status-error)',
              border: `1px solid ${testResult.ok ? 'rgba(var(--neon-green-rgb), 0.3)' : 'rgba(var(--status-error-rgb), 0.3)'}`,
            }}
          >
            {testResult.ok ? '✓ nginx -t passed' : testResult.error}
          </div>
        )}
      </div>

      {/* Site list */}
      {loading ? (
        <div className="flex items-center gap-2 text-sm font-mono" style={{ color: 'var(--text-secondary)' }}>
          <SpinnerGap size={14} className="animate-spin" />
          Loading nginx sites…
        </div>
      ) : error ? (
        <div className="text-sm font-mono" style={{ color: 'var(--status-error)' }}>{error}</div>
      ) : sites.length === 0 ? (
        <div className="text-sm font-mono" style={{ color: 'var(--text-secondary)' }}>No DockLite-managed sites found.</div>
      ) : (
        <div className="space-y-2">
          {sites.map(site => {
            const editor = editors[site.domain];
            const open = expanded === site.domain;
            return (
              <div
                key={site.domain}
                className="card-vapor rounded-xl overflow-hidden"
                style={{ border: '1px solid rgba(var(--neon-purple-rgb), 0.2)' }}
              >
                {/* Site row header */}
                <button
                  onClick={() => expandSite(site.domain)}
                  className="w-full flex items-center gap-3 px-4 py-3 text-left transition-all hover:bg-white/5"
                >
                  <div className="flex items-center gap-2 flex-1 min-w-0">
                    <span className="font-bold text-sm truncate" style={{ color: 'var(--text-primary)' }}>
                      {site.domain}
                    </span>
                    <span
                      className="text-[10px] px-2 py-0.5 rounded-full font-mono shrink-0"
                      style={{ background: 'rgba(var(--neon-purple-rgb), 0.12)', color: 'var(--neon-purple)' }}
                    >
                      {site.templateType}
                    </span>
                  </div>

                  <div className="flex items-center gap-2 shrink-0">
                    {site.enabled ? (
                      <span className="flex items-center gap-1 text-[11px] font-bold" style={{ color: 'var(--neon-green)' }}>
                        <CheckCircle size={12} weight="duotone" /> enabled
                      </span>
                    ) : site.hasConfig ? (
                      <span className="flex items-center gap-1 text-[11px] font-bold" style={{ color: 'var(--status-warning)' }}>
                        <Warning size={12} weight="duotone" /> disabled
                      </span>
                    ) : (
                      <span className="flex items-center gap-1 text-[11px] font-bold" style={{ color: 'var(--text-muted)' }}>
                        <XCircle size={12} weight="duotone" /> no config
                      </span>
                    )}
                    {isDirty(site.domain) && (
                      <span className="text-[10px] px-2 py-0.5 rounded-full font-bold"
                        style={{ background: 'rgba(var(--status-warning-rgb), 0.15)', color: 'var(--status-warning)' }}>
                        unsaved
                      </span>
                    )}
                    {open ? <CaretUp size={14} /> : <CaretDown size={14} />}
                  </div>
                </button>

                {/* Config editor */}
                {open && (
                  <div className="border-t px-4 pb-4 space-y-3" style={{ borderColor: 'rgba(var(--neon-purple-rgb), 0.2)' }}>
                    {editor?.loading ? (
                      <div className="flex items-center gap-2 text-xs font-mono py-3" style={{ color: 'var(--text-secondary)' }}>
                        <SpinnerGap size={13} className="animate-spin" /> Loading config…
                      </div>
                    ) : editor?.error && !editor.config ? (
                      <div className="text-xs font-mono py-3" style={{ color: 'var(--status-error)' }}>{editor.error}</div>
                    ) : (
                      <>
                        <textarea
                          value={editor?.config || ''}
                          onChange={e => updateConfig(site.domain, e.target.value)}
                          spellCheck={false}
                          rows={18}
                          className="w-full text-xs font-mono rounded-lg p-3 resize-y outline-none focus:ring-1"
                          style={{
                            background: 'var(--bg-darker)',
                            color: 'var(--text-primary)',
                            border: '1px solid rgba(var(--neon-purple-rgb), 0.3)',
                            lineHeight: '1.6',
                          }}
                        />

                        {editor?.error && (
                          <div className="text-xs font-mono px-3 py-2 rounded-lg"
                            style={{ background: 'rgba(var(--status-error-rgb), 0.1)', color: 'var(--status-error)', border: '1px solid rgba(var(--status-error-rgb), 0.3)' }}>
                            {editor.error}
                          </div>
                        )}
                        {editor?.success && (
                          <div className="text-xs font-mono px-3 py-2 rounded-lg"
                            style={{ background: 'rgba(var(--neon-green-rgb), 0.1)', color: 'var(--neon-green)', border: '1px solid rgba(var(--neon-green-rgb), 0.3)' }}>
                            ✓ Config saved and nginx reloaded
                          </div>
                        )}

                        <div className="flex items-center gap-2 pt-1">
                          <button
                            onClick={() => saveConfig(site.domain)}
                            disabled={editor?.saving || !isDirty(site.domain)}
                            className="inline-flex items-center gap-2 px-3 py-1.5 rounded-lg text-xs font-bold transition-all disabled:opacity-40"
                            style={{ background: 'rgba(var(--neon-cyan-rgb), 0.15)', color: 'var(--neon-cyan)', border: '1px solid rgba(var(--neon-cyan-rgb), 0.3)' }}
                          >
                            {editor?.saving ? <SpinnerGap size={12} className="animate-spin" /> : <FloppyDisk size={12} weight="duotone" />}
                            Save & Reload
                          </button>
                          <button
                            onClick={() => resetConfig(site.domain)}
                            disabled={!isDirty(site.domain)}
                            className="inline-flex items-center gap-2 px-3 py-1.5 rounded-lg text-xs font-bold transition-all disabled:opacity-40"
                            style={{ background: 'rgba(var(--neon-purple-rgb), 0.1)', color: 'var(--neon-purple)', border: '1px solid rgba(var(--neon-purple-rgb), 0.2)' }}
                          >
                            <ArrowCounterClockwise size={12} weight="duotone" />
                            Revert
                          </button>
                        </div>
                      </>
                    )}
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
