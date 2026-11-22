'use client';

interface AssignBoxModalProps {
  boxName: string;
  users: Array<{ id: number; username: string }>;
  userId: string;
  onUserId: (id: string) => void;
  error: string;
  busy: boolean;
  onSubmit: () => void;
  onClose: () => void;
}

/** Admin only: give a container to another user. */
export default function AssignBoxModal({ boxName, users, userId, onUserId, error, busy, onSubmit, onClose }: AssignBoxModalProps) {
  return (
    <div className="fixed inset-0 bg-black/80 flex items-center justify-center z-[10000] p-4" onClick={onClose}>
      <div className="cyber-card max-w-md w-full p-6" onClick={(e) => e.stopPropagation()}>
        <div className="flex items-center justify-between mb-6">
          <h2 className="text-2xl font-bold neon-text-pink">Assign container</h2>
          <button onClick={onClose} className="transition-colors" style={{ color: 'var(--text-secondary)' }} aria-label="Close">✕</button>
        </div>
        <p className="text-sm mb-4" style={{ color: 'var(--text-secondary)' }}>
          Assign <span className="font-bold" style={{ color: 'var(--text-cyan)' }}>{boxName}</span> to a user.
        </p>
        <label className="block mb-4">
          <span className="block text-sm font-bold mb-2 text-accent-cyan">Select user</span>
          <select value={userId} onChange={(e) => onUserId(e.target.value)} className="input-vapor w-full">
            <option value="">Choose a user...</option>
            {users.map((u) => (
              <option key={u.id} value={String(u.id)}>{u.username}</option>
            ))}
          </select>
        </label>
        {error && (
          <div className="mb-4 p-3 rounded-lg" style={{ background: 'rgba(var(--status-error-rgb), 0.2)', border: '1px solid rgba(var(--status-error-rgb), 0.5)' }}>
            <p className="text-sm" style={{ color: 'var(--status-error)' }}>{error}</p>
          </div>
        )}
        <div className="flex gap-3">
          <button type="button" onClick={onClose} disabled={busy} className="flex-1 px-4 py-2 rounded-lg font-bold border-2" style={{ borderColor: 'var(--surface-dim)', color: 'var(--text-primary)' }}>
            Cancel
          </button>
          <button type="button" onClick={onSubmit} disabled={busy || !userId} className="flex-1 cyber-button">
            {busy ? 'Assigning…' : 'Assign'}
          </button>
        </div>
      </div>
    </div>
  );
}
