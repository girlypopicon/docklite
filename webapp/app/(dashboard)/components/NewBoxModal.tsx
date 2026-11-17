'use client';

import { useState } from 'react';
import { ContainerInfo } from '@/types';
import { Globe, Database as DatabaseIcon } from '@phosphor-icons/react';
import CloseButton from './CloseButton';
import { useEscape } from '@/lib/hooks/useEscape';
import AddContainerModal from './AddContainerModal';
import CreateDatabaseModal from './CreateDatabaseModal';
import { DB_ENGINES, type DbType } from '@/lib/database-engines';

interface NewBoxModalProps {
  sites: ContainerInfo[]; // the sites a SQLite database can live in
  onClose: () => void;
  onCreated: () => void;
}

type Step = { kind: 'choose' } | { kind: 'website' } | { kind: 'database'; type: DbType };

/**
 * "New box": asks what kind first. A website (static files, PHP or Node) or a database (PostgreSQL, MySQL, MariaDB, MongoDB,
 * Redis, SQLite). More kinds of server will join this list later.
 */
export default function NewBoxModal({ sites, onClose, onCreated }: NewBoxModalProps) {
  useEscape(onClose);
  const [step, setStep] = useState<Step>({ kind: 'choose' });

  if (step.kind === 'website') return <AddContainerModal onClose={onClose} onCreated={onCreated} />;
  if (step.kind === 'database') {
    return <CreateDatabaseModal type={step.type} sites={sites} onBack={() => setStep({ kind: 'choose' })} onClose={onClose} onCreated={onCreated} />;
  }

  const tile = (key: string, title: string, blurb: string, icon: React.ReactNode, onClick: () => void) => (
    <button
      key={key}
      type="button"
      onClick={onClick}
      className="text-left p-4 rounded-xl transition-all hover:-translate-y-0.5 flex gap-3 items-start"
      style={{ border: '1px solid rgba(var(--neon-purple-rgb), 0.4)', background: 'var(--surface-dim)' }}
    >
      <span className="shrink-0 mt-0.5" style={{ color: 'var(--text-cyan)' }}>{icon}</span>
      <span className="min-w-0">
        <span className="block font-bold" style={{ color: 'var(--text-primary)' }}>{title}</span>
        <span className="block text-xs mt-1" style={{ color: 'var(--text-secondary)' }}>{blurb}</span>
      </span>
    </button>
  );

  return (
    <div className="fixed inset-0 z-[10000] flex items-center justify-center p-3 sm:p-4 bg-black/75 backdrop-blur-sm" onClick={onClose}>
      <div
        className="card-vapor w-full max-w-3xl max-h-[92vh] overflow-y-auto p-4 sm:p-6 relative"
        style={{ background: 'linear-gradient(135deg, var(--modal-bg-1) 0%, var(--modal-bg-2) 100%)', border: '2px solid var(--modal-border)', boxShadow: '0 0 30px var(--modal-shadow)' }}
        onClick={(e) => e.stopPropagation()}
        role="dialog"
        aria-label="New box"
      >
        <CloseButton onClick={onClose} className="absolute top-3 right-3" />
        <h2 className="text-2xl font-bold pr-10" style={{ color: 'var(--text-primary)' }}>New box</h2>
        <p className="text-sm mt-1" style={{ color: 'var(--text-secondary)' }}>What kind of box do you want to make?</p>

        <h3 className="mt-5 mb-2 text-xs uppercase tracking-[0.2em]" style={{ color: 'var(--text-purple)' }}>Website</h3>
        <div className="grid gap-3 sm:grid-cols-2">
          {tile('website', 'A website', 'Static files, PHP or Node.js, on its own domain. Gets its own address and a certificate.', <Globe size={26} weight="regular" />, () => setStep({ kind: 'website' }))}
        </div>

        <h3 className="mt-5 mb-2 text-xs uppercase tracking-[0.2em]" style={{ color: 'var(--text-purple)' }}>Database</h3>
        <div className="grid gap-3 sm:grid-cols-2">
          {DB_ENGINES.map((e) =>
            tile(e.id, e.label, e.blurb, <e.Icon size={26} weight="regular" />, () => setStep({ kind: 'database', type: e.id })),
          )}
        </div>

        <p className="mt-5 text-xs flex items-center gap-2" style={{ color: 'var(--text-secondary)' }}>
          <DatabaseIcon size={14} weight="regular" /> More kinds of box (game and media servers, and others) are coming.
        </p>
      </div>
    </div>
  );
}
