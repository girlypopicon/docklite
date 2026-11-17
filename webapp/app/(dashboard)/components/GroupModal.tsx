'use client';

import { useState } from 'react';
import { ContainerInfo, BoxGroup } from '@/types';
import { LinkSimple, Plus } from '@phosphor-icons/react';
import { containerDisplayName } from '@/lib/container-names';
import { groupIconFor } from '@/lib/box-styles';
import CloseButton from './CloseButton';
import { useEscape } from '@/lib/hooks/useEscape';

interface GroupModalProps {
  group: BoxGroup;
  boxes: ContainerInfo[]; // the group's boxes
  addable: ContainerInfo[]; // boxes not in any group, which can be put in this one
  renderBox: (box: ContainerInfo) => React.ReactNode;
  onAdd: (box: ContainerInfo) => void;
  onClose: () => void;
}

/** The window a group opens: its boxes as cards, plus a way to add another. */
export default function GroupModal({ group, boxes, addable, renderBox, onAdd, onClose }: GroupModalProps) {
  useEscape(onClose);
  const [pick, setPick] = useState('');
  const Icon = groupIconFor(group.icon);

  return (
    <div className="fixed inset-0 z-[9000] flex items-center justify-center p-3 sm:p-6 bg-black/75 backdrop-blur-sm" onClick={onClose}>
      <div
        className="card-vapor w-full max-w-5xl max-h-[92vh] overflow-y-auto p-4 sm:p-6 relative"
        style={{
          background: 'linear-gradient(135deg, var(--modal-bg-1) 0%, var(--modal-bg-2) 100%)',
          border: '2px solid var(--modal-border)',
          boxShadow: '0 0 30px var(--modal-shadow)',
        }}
        onClick={(e) => e.stopPropagation()}
        role="dialog"
        aria-label={`Group ${group.name}`}
      >
        <CloseButton onClick={onClose} className="absolute top-3 right-3" />

        <div className="pr-12 mb-5">
          <h2 className="text-2xl font-bold flex items-center gap-3 min-w-0" style={{ color: 'var(--text-primary)' }}>
            <Icon size={28} weight="regular" className="shrink-0" style={{ color: 'var(--text-cyan)' }} />
            <span className="truncate">{group.name}</span>
            {group.linked && (
              <span className="inline-flex items-center gap-1 text-xs font-mono px-2 py-0.5 rounded-full shrink-0" style={{ color: 'var(--text-cyan)', border: '1px solid var(--text-cyan)' }}>
                <LinkSimple size={12} weight="bold" /> Linked
              </span>
            )}
          </h2>
          <p className="text-sm mt-1" style={{ color: 'var(--text-secondary)' }}>
            {group.linked
              ? 'Linked groups are for boxes that belong together, like a website and its database.'
              : 'A group keeps related boxes together on your desktop.'}
          </p>
        </div>

        {boxes.length === 0 ? (
          <p className="py-10 text-center" style={{ color: 'var(--text-secondary)' }}>
            Nothing in here yet. Add a box below, or drag one onto this group on the desktop.
          </p>
        ) : (
          <div className="grid gap-3 sm:gap-4 grid-cols-[repeat(auto-fill,minmax(min(100%,148px),1fr))] sm:grid-cols-[repeat(auto-fill,minmax(172px,1fr))]">
            {boxes.map((b) => (
              <div key={b.id}>{renderBox(b)}</div>
            ))}
          </div>
        )}

        {addable.length > 0 && (
          <form
            className="mt-6 flex flex-wrap items-end gap-3"
            onSubmit={(e) => {
              e.preventDefault();
              const box = addable.find((b) => b.id === pick);
              if (box) {
                onAdd(box);
                setPick('');
              }
            }}
          >
            <label className="block flex-1 min-w-[14rem]">
              <span className="block text-xs mb-1" style={{ color: 'var(--text-secondary)' }}>Add a box to this group</span>
              <select value={pick} onChange={(e) => setPick(e.target.value)} className="input-vapor w-full">
                <option value="">Choose a box…</option>
                {addable.map((b) => (
                  <option key={b.id} value={b.id}>
                    {containerDisplayName(b)}
                  </option>
                ))}
              </select>
            </label>
            <button type="submit" className="cyber-button inline-flex items-center gap-2" disabled={!pick}>
              <Plus size={16} weight="bold" /> Add
            </button>
          </form>
        )}
      </div>
    </div>
  );
}
