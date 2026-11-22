'use client';

import { useState } from 'react';
import { BoxGroup } from '@/types';
import CloseButton from './CloseButton';
import { useEscape } from '@/lib/hooks/useEscape';

interface MoveToGroupModalProps {
  boxName: string;
  groups: BoxGroup[];
  currentGroupId: number | null;
  busy?: boolean;
  error?: string;
  // groupId: an existing group; null: back to the desktop; "new": make a group with that name
  onMove: (target: { groupId: number | null } | { newName: string }) => void;
  onClose: () => void;
}

/** The phone-friendly way to put a container in a group (dragging is for people with a mouse). */
export default function MoveToGroupModal({ boxName, groups, currentGroupId, busy, error, onMove, onClose }: MoveToGroupModalProps) {
  useEscape(onClose);
  const [choice, setChoice] = useState<string>(currentGroupId === null ? 'none' : String(currentGroupId));
  const [newName, setNewName] = useState('');
  const valid = choice !== 'new' || (newName.trim().length > 0 && newName.trim().length <= 50 && !/[\\/]/.test(newName));

  const submit = () => {
    if (choice === 'new') onMove({ newName: newName.trim() });
    else if (choice === 'none') onMove({ groupId: null });
    else onMove({ groupId: Number(choice) });
  };

  const option = (value: string, label: string, hint?: string) => (
    <label key={value} className="flex items-center gap-3 px-3 py-2 rounded-lg cursor-pointer hover:bg-white/5" style={{ border: choice === value ? '1px solid var(--text-cyan)' : '1px solid rgba(var(--neon-purple-rgb), 0.25)' }}>
      <input type="radio" name="group-choice" value={value} checked={choice === value} onChange={() => setChoice(value)} />
      <span className="flex-1 min-w-0">
        <span className="block truncate font-bold" style={{ color: 'var(--text-primary)' }}>{label}</span>
        {hint && <span className="block text-xs" style={{ color: 'var(--text-secondary)' }}>{hint}</span>}
      </span>
    </label>
  );

  return (
    <div className="fixed inset-0 z-[10000] flex items-center justify-center p-4 bg-black/75 backdrop-blur-sm" onClick={onClose}>
      <div
        className="card-vapor w-full max-w-md max-h-[90vh] overflow-y-auto p-5 sm:p-6 relative"
        style={{ background: 'linear-gradient(135deg, var(--modal-bg-1) 0%, var(--modal-bg-2) 100%)', border: '2px solid var(--modal-border)', boxShadow: '0 0 30px var(--modal-shadow)' }}
        onClick={(e) => e.stopPropagation()}
      >
        <CloseButton onClick={onClose} className="absolute top-3 right-3" />
        <h2 className="text-xl font-bold pr-10" style={{ color: 'var(--text-primary)' }}>Move to group</h2>
        <p className="text-sm mt-1" style={{ color: 'var(--text-secondary)' }}>Where should <b style={{ color: 'var(--text-cyan)' }}>{boxName}</b> go?</p>
        <div className="mt-4 space-y-2">
          {option('none', 'On the desktop', 'Not in any group')}
          {groups.map((g) => option(String(g.id), g.name, `${g.members.length} ${g.members.length === 1 ? 'container' : 'containers'}${g.linked ? ' · linked' : ''}`))}
          {option('new', 'A new group…')}
          {choice === 'new' && (
            <input value={newName} onChange={(e) => setNewName(e.target.value)} maxLength={50} autoFocus placeholder="Name the group" className="input-vapor w-full" />
          )}
        </div>
        {error && <p className="text-sm mt-3" style={{ color: 'var(--status-error)' }}>{error}</p>}
        <div className="flex gap-3 mt-5">
          <button type="button" onClick={onClose} disabled={busy} className="flex-1 px-4 py-2 rounded-lg font-bold border-2" style={{ borderColor: 'var(--surface-dim)', color: 'var(--text-primary)' }}>
            Cancel
          </button>
          <button type="button" onClick={submit} disabled={!valid || busy} className="flex-1 cyber-button">
            {busy ? 'Moving…' : 'Move'}
          </button>
        </div>
      </div>
    </div>
  );
}
