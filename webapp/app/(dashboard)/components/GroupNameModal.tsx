'use client';

import { useEffect, useRef, useState } from 'react';
import CloseButton from './CloseButton';
import { useEscape } from '@/lib/hooks/useEscape';

interface GroupNameModalProps {
  title: string; // "New group", "Rename group", "Make a group"
  intro?: string;
  initial?: string;
  confirmLabel: string;
  busy?: boolean;
  error?: string;
  onSubmit: (name: string) => void;
  onClose: () => void;
}

/** Asks for a group's name: when making one, and when renaming. */
export default function GroupNameModal({ title, intro, initial = '', confirmLabel, busy, error, onSubmit, onClose }: GroupNameModalProps) {
  useEscape(onClose);
  const [name, setName] = useState(initial);
  const ref = useRef<HTMLInputElement>(null);
  useEffect(() => {
    ref.current?.focus();
    ref.current?.select();
  }, []);
  const ok = name.trim().length > 0 && name.trim().length <= 50 && !/[\\/]/.test(name);

  return (
    <div className="fixed inset-0 z-[10000] flex items-center justify-center p-4 bg-black/75 backdrop-blur-sm" onClick={onClose}>
      <form
        className="card-vapor w-full max-w-md p-5 sm:p-6 relative"
        style={{ background: 'linear-gradient(135deg, var(--modal-bg-1) 0%, var(--modal-bg-2) 100%)', border: '2px solid var(--modal-border)', boxShadow: '0 0 30px var(--modal-shadow)' }}
        onClick={(e) => e.stopPropagation()}
        onSubmit={(e) => {
          e.preventDefault();
          if (ok && !busy) onSubmit(name.trim());
        }}
      >
        <CloseButton onClick={onClose} className="absolute top-3 right-3" />
        <h2 className="text-xl font-bold pr-10" style={{ color: 'var(--text-primary)' }}>{title}</h2>
        {intro && <p className="text-sm mt-1" style={{ color: 'var(--text-secondary)' }}>{intro}</p>}
        <label className="block mt-4">
          <span className="block text-xs mb-1" style={{ color: 'var(--text-secondary)' }}>Name</span>
          <input ref={ref} value={name} onChange={(e) => setName(e.target.value)} maxLength={50} className="input-vapor w-full" placeholder="Shop, Client work, Blog…" />
        </label>
        {error && <p className="text-sm mt-2" style={{ color: 'var(--status-error)' }}>{error}</p>}
        <div className="flex gap-3 mt-5">
          <button type="button" onClick={onClose} disabled={busy} className="flex-1 px-4 py-2 rounded-lg font-bold border-2" style={{ borderColor: 'var(--surface-dim)', color: 'var(--text-primary)' }}>
            Cancel
          </button>
          <button type="submit" disabled={!ok || busy} className="flex-1 cyber-button">
            {busy ? 'Working…' : confirmLabel}
          </button>
        </div>
      </form>
    </div>
  );
}
