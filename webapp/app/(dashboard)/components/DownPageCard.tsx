'use client';

import { useCallback, useEffect, useState } from 'react';
import { Warning } from '@phosphor-icons/react';
import { useToast } from '@/lib/hooks/useToast';

/**
 * Settings → System: the contact line on the page visitors see when a site's box is stopped. The page itself is one
 * file every site shares, so one line here changes it everywhere. Shows the page as it will look.
 */
export default function DownPageCard() {
  const toast = useToast();
  const [contact, setContact] = useState('');
  const [saved, setSaved] = useState('');
  const [html, setHtml] = useState('');
  const [max, setMax] = useState(200);
  const [loaded, setLoaded] = useState(false);
  const [unavailable, setUnavailable] = useState(false);
  const [saving, setSaving] = useState(false);

  const load = useCallback(async () => {
    try {
      const res = await fetch('/api/settings/down-page', { cache: 'no-store' });
      if (!res.ok) throw new Error('unavailable');
      const data = await res.json();
      setContact(data.contact || '');
      setSaved(data.contact || '');
      setHtml(data.html || '');
      setMax(data.max_length || 200);
    } catch {
      setUnavailable(true);
    } finally {
      setLoaded(true);
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  const save = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true);
    try {
      const res = await fetch('/api/settings/down-page', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ contact }),
      });
      const data = await res.json().catch(() => ({}));
      if (!res.ok) throw new Error(data.error || 'Could not save');
      setContact(data.contact || '');
      setSaved(data.contact || '');
      setHtml(data.html || '');
      toast.success('Saved. Every site now shows this contact when its box is stopped');
    } catch (err: any) {
      toast.error(err.message || 'Could not save');
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="card-vapor p-4 sm:p-6 rounded-xl space-y-4">
      <div>
        <h2 className="text-2xl font-bold neon-text flex items-center gap-2" style={{ color: 'var(--text-pink)' }}>
          <Warning size={22} weight="regular" />
          Page for stopped boxes
        </h2>
        <p className="text-sm mt-1" style={{ color: 'var(--text-secondary)' }}>
          When a site&apos;s box is stopped, visitors see this page instead of a plain &quot;502 Bad Gateway&quot;. Every site
          shares it. Add a name, an email address or a link so people know who to ask.
        </p>
      </div>

      {unavailable && loaded ? (
        <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>Not available here.</p>
      ) : (
        <form onSubmit={save} className="flex flex-wrap items-end gap-3">
          <label className="block flex-1 min-w-[16rem]">
            <span className="block text-xs mb-1" style={{ color: 'var(--text-secondary)' }}>Contact line (optional)</span>
            <input
              value={contact}
              onChange={(e) => setContact(e.target.value)}
              maxLength={max}
              placeholder="admin@example.com"
              className="input-vapor w-full"
              disabled={!loaded}
            />
          </label>
          <button type="submit" className="cyber-button" disabled={saving || !loaded || contact === saved}>
            {saving ? 'Saving…' : 'Save'}
          </button>
        </form>
      )}

      {html && (
        <div>
          <div className="text-xs mb-1" style={{ color: 'var(--text-secondary)' }}>How it looks (shown as soon as you save)</div>
          {/* The real page at twice the size, shown at half: what a visitor sees, in a frame that fits a phone too */}
          <div className="relative w-full max-w-xl overflow-hidden rounded-lg border border-neon-purple/30 aspect-[4/5] sm:aspect-[16/10]" style={{ background: '#0b0b16' }}>
            <iframe
              title="Preview of the page shown for a stopped box"
              srcDoc={html}
              sandbox=""
              tabIndex={-1}
              className="absolute left-0 top-0 border-0"
              style={{ width: '200%', height: '200%', transform: 'scale(0.5)', transformOrigin: '0 0' }}
            />
          </div>
        </div>
      )}
    </div>
  );
}
