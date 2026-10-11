'use client';

import { WarningCircle, Lightning, Info } from '@phosphor-icons/react';

interface ConfirmModalProps {
  title: string;
  message: string;
  confirmText?: string;
  cancelText?: string;
  onConfirm: () => void;
  onCancel: () => void;
  type?: 'danger' | 'warning' | 'info';
}

export default function ConfirmModal({
  title,
  message,
  confirmText = 'Confirm',
  cancelText = 'Cancel',
  onConfirm,
  onCancel,
  type = 'danger',
}: ConfirmModalProps) {
  const typeStyles = {
    danger: {
      icon: <WarningCircle size={48} weight="duotone" />,
      color: 'var(--status-error)',
      gradient: 'linear-gradient(135deg, var(--status-error) 0%, var(--neon-pink) 100%)',
    },
    warning: {
      icon: <Lightning size={48} weight="duotone" />,
      color: 'var(--text-yellow)',
      gradient: 'linear-gradient(135deg, var(--neon-yellow) 0%, var(--neon-pink) 100%)',
    },
    info: {
      icon: <Info size={48} weight="duotone" />,
      color: 'var(--text-cyan)',
      gradient: 'linear-gradient(135deg, var(--neon-cyan) 0%, var(--neon-purple) 100%)',
    },
  };

  const style = typeStyles[type];

  return (
    <div
      className="fixed inset-0 flex items-center justify-center z-[100000]"
      onClick={onCancel}
    >
      {/* Backdrop */}
      <div className="absolute inset-0 bg-black/70 backdrop-blur-sm"></div>

      {/* Modal */}
      <div
        className="relative card-vapor p-8 rounded-2xl border-2 max-w-md w-full mx-4 animate-slide-down"
        style={{ borderColor: style.color }}
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div className="text-center mb-6">
          <div className="flex justify-center mb-4 animate-pulse" style={{ color: style.color }}>
            {style.icon}
          </div>
          <h2
            className="text-2xl font-bold neon-text mb-3"
            style={{ color: style.color }}
          >
            {title}
          </h2>
          <p
            className="text-sm font-mono leading-relaxed"
            style={{ color: 'var(--text-secondary)' }}
          >
            {message}
          </p>
        </div>

        {/* Actions */}
        <div className="flex gap-3">
          <button
            onClick={onCancel}
            className="flex-1 px-4 py-3 rounded-lg font-bold transition-all hover:scale-105 border-2"
            style={{
              borderColor: 'rgba(var(--text-muted-rgb), 0.2)',
              color: 'var(--text-secondary)',
              background: 'rgba(var(--text-muted-rgb), 0.05)',
            }}
          >
            {cancelText}
          </button>
          <button
            onClick={onConfirm}
            className="flex-1 px-4 py-3 rounded-lg font-bold transition-all hover:scale-105"
            style={{
              background: style.gradient,
              color: 'white',
              boxShadow: `0 0 20px ${style.color}40`,
            }}
          >
            {confirmText}
          </button>
        </div>
      </div>
    </div>
  );
}
