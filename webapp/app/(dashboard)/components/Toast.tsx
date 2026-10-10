'use client';

import { useEffect, useState } from 'react';
import { CheckCircle, XCircle, WarningCircle, Info, X } from '@phosphor-icons/react';

export interface ToastProps {
  message: string;
  type: 'success' | 'error' | 'info' | 'warning';
  onClose: () => void;
  duration?: number;
}

export default function Toast({ message, type, onClose, duration = 5000 }: ToastProps) {
  const [isExiting, setIsExiting] = useState(false);

  const handleClose = () => {
    setIsExiting(true);
    setTimeout(onClose, 300); // Wait for exit animation
  };

  useEffect(() => {
    if (duration > 0) {
      const timer = setTimeout(handleClose, duration);
      return () => clearTimeout(timer);
    }
  }, [duration]);

  const typeStyles = {
    success: {
      bg: 'rgba(var(--status-success-rgb), 0.15)',
      border: 'var(--neon-green)',
      color: 'var(--text-green)',
      icon: <CheckCircle size={22} weight="duotone" />,
    },
    error: {
      bg: 'rgba(var(--status-error-rgb), 0.15)',
      border: 'var(--status-error)',
      color: 'var(--status-error)',
      icon: <XCircle size={22} weight="duotone" />,
    },
    warning: {
      bg: 'rgba(var(--status-warning-rgb), 0.15)',
      border: 'var(--neon-yellow)',
      color: 'var(--text-yellow)',
      icon: <WarningCircle size={22} weight="duotone" />,
    },
    info: {
      bg: 'rgba(var(--neon-cyan-rgb), 0.15)',
      border: 'var(--neon-cyan)',
      color: 'var(--text-cyan)',
      icon: <Info size={22} weight="duotone" />,
    },
  };

  const style = typeStyles[type];

  return (
    <div
      className={`fixed top-24 right-6 z-[100000] min-w-[300px] max-w-md card-vapor p-4 rounded-xl border-2 shadow-2xl transition-all duration-300 ${
        isExiting ? 'animate-slide-out-right' : 'animate-slide-in-right'
      }`}
      style={{
        background: style.bg,
        borderColor: style.border,
        backdropFilter: 'blur(12px)',
        boxShadow: `0 0 30px ${style.border}40, 0 10px 40px rgba(0, 0, 0, 0.3)`,
      }}
    >
      <div className="flex items-start gap-3">
        <div className="flex-shrink-0 animate-pulse-subtle" style={{ color: style.color }}>
          {style.icon}
        </div>
        <div className="flex-1">
          <p className="font-bold text-sm leading-relaxed" style={{ color: style.color }}>
            {message}
          </p>
        </div>
        <button
          onClick={handleClose}
          className="flex-shrink-0 opacity-70 hover:opacity-100 hover:scale-110 transition-all duration-200"
          style={{ color: style.color }}
        >
          <X size={18} weight="bold" />
        </button>
      </div>
      {/* Progress bar */}
      {duration > 0 && (
        <div
          className="absolute bottom-0 left-0 h-1 rounded-b-xl"
          style={{
            background: style.border,
            animation: `toast-progress ${duration}ms linear forwards`,
          }}
        />
      )}
    </div>
  );
}
