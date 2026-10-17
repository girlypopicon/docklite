'use client';

import { X, Warning } from '@phosphor-icons/react';
import CloseButton from './CloseButton';

interface ConfirmDeleteModalProps {
  title: string;
  message: string;
  itemName?: string;
  onConfirm: () => void;
  onCancel: () => void;
  loading?: boolean;
}

export default function ConfirmDeleteModal({
  title,
  message,
  itemName,
  onConfirm,
  onCancel,
  loading = false
}: ConfirmDeleteModalProps) {
  return (
    <div className="fixed inset-0 z-[10000] flex items-center justify-center p-4 bg-black/70 backdrop-blur-sm">
      <div
        className="card-vapor max-w-md w-full p-6 relative"
        style={{
          background: 'linear-gradient(135deg, rgba(var(--status-error-rgb), 0.25) 0%, rgba(var(--status-error-rgb), 0.45) 100%)',
          border: '2px solid var(--status-error)',
          boxShadow: '0 0 30px rgba(var(--status-error-rgb), 0.5)',
        }}
      >
        {/* Close button */}
        <CloseButton onClick={onCancel} disabled={loading} className="absolute top-4 right-4" />

        {/* Header */}
        <div className="mb-6">
          <div className="flex items-center gap-3 mb-3">
            <div
              className="p-3 rounded-lg"
              style={{
                background: 'rgba(var(--status-error-rgb), 0.2)',
                border: '1px solid var(--status-error)',
              }}
            >
              <Warning size={32} color="var(--status-error)" weight="duotone" />
            </div>
            <h2 className="text-2xl font-bold" style={{ color: 'var(--status-error)' }}>
              {title}
            </h2>
          </div>
          <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>
            {message}
          </p>
        </div>

        {itemName && (
          <div
            className="mb-6 p-4 rounded-lg font-mono text-sm"
            style={{
              background: 'rgba(var(--status-error-rgb), 0.1)',
              border: '1px solid rgba(var(--status-error-rgb), 0.3)',
              color: 'var(--status-error)',
            }}
          >
            {itemName}
          </div>
        )}

        <div
          className="mb-6 p-3 rounded-lg text-sm flex items-center gap-2"
          style={{
            background: 'rgba(var(--status-warning-rgb), 0.1)',
            border: '1px solid rgba(var(--status-warning-rgb), 0.3)',
            color: 'var(--status-warning)',
          }}
        >
          <Warning size={16} weight="duotone" />
          This action cannot be undone.
        </div>

        <div className="flex gap-3">
          <button
            type="button"
            onClick={onCancel}
            disabled={loading}
            className="flex-1 px-4 py-3 rounded-lg font-bold transition-all hover:scale-105 disabled:opacity-50"
            style={{
              background: 'rgba(var(--text-muted-rgb), 0.1)',
              border: '1px solid rgba(var(--text-muted-rgb), 0.2)',
              color: 'var(--text-secondary)',
            }}
          >
            Cancel
          </button>
          <button
            type="button"
            onClick={onConfirm}
            disabled={loading}
            className="flex-1 px-4 py-3 rounded-lg font-bold transition-all hover:scale-105 disabled:opacity-50 disabled:cursor-not-allowed"
            style={{
              background: 'linear-gradient(135deg, var(--status-error) 0%, var(--status-error-strong) 100%)',
              color: 'white',
              boxShadow: '0 0 20px rgba(var(--status-error-rgb), 0.4)',
            }}
          >
            {loading ? 'Deleting...' : 'Delete'}
          </button>
        </div>
      </div>
    </div>
  );
}
