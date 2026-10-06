'use client';

import { useEffect, useState } from 'react';
import { useToast } from '@/lib/hooks/useToast';

type TemplateType = 'static' | 'php' | 'node';

interface AddContainerModalProps {
  onClose: () => void;
  onCreated: () => void;
}

export default function AddContainerModal({ onClose, onCreated }: AddContainerModalProps) {
  const [domain, setDomain] = useState('');
  const [templateType, setTemplateType] = useState<TemplateType>('static');
  const [codePath, setCodePath] = useState('');
  const [port, setPort] = useState(3000);
  const [portTouched, setPortTouched] = useState(false);
  const [includeWww, setIncludeWww] = useState(true);
  const [loading, setLoading] = useState(false);
  const toast = useToast();

  useEffect(() => {
    if (templateType !== 'node' || portTouched) return;
    const loadSuggestedPort = async () => {
      try {
        const res = await fetch('/api/ports/suggest?type=node');
        if (!res.ok) return;
        const data = await res.json();
        if (typeof data.port === 'number') {
          setPort(data.port);
        }
      } catch {
        // Ignore suggestion failures; keep default
      }
    };
    loadSuggestedPort();
  }, [templateType, portTouched]);

  const handleSubmit = async () => {
    if (!domain.trim()) {
      toast.error('Domain is required');
      return;
    }
    setLoading(true);
    try {
      const res = await fetch('/api/containers', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          domain: domain.trim(),
          template_type: templateType,
          code_path: codePath.trim() || undefined,
          port: templateType === 'node' ? Number(port) || 3000 : undefined,
          include_www: includeWww,
        }),
      });
      const data = await res.json().catch(() => ({}));
      if (!res.ok) {
        throw new Error(data.error || 'Failed to create container');
      }
      const warning = data.warning;
      if (warning) {
        toast.success(`Container created. Note: ${warning}`);
      } else {
        toast.success('Container created with nginx proxy configured');
      }
      onCreated();
      onClose();
    } catch (err: any) {
      toast.error(err.message || 'Failed to create container');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="fixed inset-0 bg-black/80 backdrop-blur-sm flex items-center justify-center z-[10000] p-4">
      <div className="card-vapor max-w-2xl w-full p-6 rounded-2xl border-2 border-neon-purple/40">
        <div className="flex items-start justify-between gap-3 mb-4">
          <div>
                          <div className="text-xs uppercase tracking-[0.2em] text-neon-purple/70">Create Container</div>            <div className="text-2xl font-bold text-neon-cyan">New Site</div>
          </div>
          <button onClick={onClose} className="btn-neon px-3 py-1 text-sm font-bold">✕ Close</button>
        </div>

        <div className="space-y-4">
          <div>
            <label className="text-sm font-bold text-neon-cyan/80 block mb-2">Domain</label>
            <input
              className="input-vapor w-full px-3 py-2 font-mono"
              placeholder="example.com"
              value={domain}
              onChange={(e) => setDomain(e.target.value)}
            />
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div>
              <label className="text-sm font-bold text-neon-cyan/80 block mb-2">Template</label>
              <select
                className="input-vapor w-full px-3 py-2"
                value={templateType}
                onChange={(e) => setTemplateType(e.target.value as TemplateType)}
              >
                <option value="static">Static (nginx)</option>
                <option value="php">PHP (php-nginx)</option>
                <option value="node">Node.js</option>
              </select>
            </div>
            <div>
              <label className="text-sm font-bold text-neon-cyan/80 block mb-2">Code Path (optional)</label>
              <input
                className="input-vapor w-full px-3 py-2 font-mono"
                placeholder="/var/www/sites/username/example.com"
                value={codePath}
                onChange={(e) => setCodePath(e.target.value)}
              />
            </div>
          </div>

          {templateType === 'node' && (
            <div>
              <label className="text-sm font-bold text-neon-cyan/80 block mb-2">Node Internal Port</label>
              <input
                type="number"
                className="input-vapor w-full px-3 py-2 font-mono"
                value={port}
                onChange={(e) => {
                  setPort(Number(e.target.value));
                  setPortTouched(true);
                }}
                min={1}
              />
            </div>
          )}

          <div className="flex items-center gap-2">
            <input
              id="include-www"
              type="checkbox"
              className="w-4 h-4 accent-cyan-400"
              checked={includeWww}
              onChange={(e) => setIncludeWww(e.target.checked)}
            />
                              <label htmlFor="include-www" className="text-sm text-neon-purple">              Request SSL for <code>www.{domain || 'example.com'}</code> too
            </label>
          </div>

          <div className="flex justify-end gap-3 pt-2">
            <button
              onClick={onClose}
              className="btn-neon px-4 py-2 font-bold"
              disabled={loading}
            >
              Cancel
            </button>
            <button
              onClick={handleSubmit}
              className="btn-neon px-6 py-2 font-bold"
              disabled={loading}
            >
              {loading ? 'Creating...' : 'Create'}
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
