'use client';

import { useState } from 'react';
import { HardDrives } from '@phosphor-icons/react';
import BackupPlanModal, { useCanManageBackups } from './BackupPlanModal';

/** In a container's details: a way to back it up on a schedule, next to the Backups page. Admins only. */
export default function BackupPlanCard({ containerName }: { containerName: string }) {
  const can = useCanManageBackups();
  const [open, setOpen] = useState(false);
  if (!can) return null;
  const name = containerName.replace(/^\//, '');
  return (
    <div className="card-vapor p-6 rounded-xl">
      <h3 className="text-xl font-bold neon-text mb-1 flex items-center gap-2" style={{ color: 'var(--text-pink)' }}>
        <HardDrives size={22} weight="regular" /> Backups
      </h3>
      <p className="text-xs mb-4" style={{ color: 'var(--text-secondary)' }}>
        Back this container up every day or week, to this server, a bucket or another server. Every backup also shows on the Backups page.
      </p>
      <button type="button" className="btn-neon px-4 py-2 text-sm font-bold" onClick={() => setOpen(true)}>
        Back up this container…
      </button>
      {open && <BackupPlanModal kind="container" name={name} onClose={() => setOpen(false)} />}
    </div>
  );
}
