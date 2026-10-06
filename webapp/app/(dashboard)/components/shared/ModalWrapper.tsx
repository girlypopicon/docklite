'use client';

import React from 'react';
import { X } from '@phosphor-icons/react';

export type ModalVariant = 'default' | 'danger' | 'success' | 'warning';

interface ModalWrapperProps {
  isOpen: boolean;
  onClose: () => void;
  title: string;
  subtitle?: string;
  variant?: ModalVariant;
  children: React.ReactNode;
  maxWidth?: 'sm' | 'md' | 'lg' | 'xl' | '2xl';
  className?: string;
}

export default function ModalWrapper({
  isOpen,
  onClose,
  title,
  subtitle,
  variant = 'default',
  children,
  maxWidth = 'lg',
  className = '',
}: ModalWrapperProps) {
  if (!isOpen) return null;

  const getVariantStyles = () => {
    switch (variant) {
      case 'danger':
        return {
          borderColor: 'var(--status-error)',
          titleColor: 'var(--status-error)',
        };
      case 'success':
        return {
          borderColor: 'var(--neon-green)',
          titleColor: 'var(--neon-green)',
        };
      case 'warning':
        return {
          borderColor: 'var(--neon-yellow)',
          titleColor: 'var(--neon-yellow)',
        };
      default:
        return {
          borderColor: 'var(--neon-purple)',
          titleColor: 'var(--neon-cyan)',
        };
    }
  };

  const variantStyles = getVariantStyles();

  const maxWidthClass = {
    sm: 'max-w-sm',
    md: 'max-w-md',
    lg: 'max-w-lg',
    xl: 'max-w-xl',
    '2xl': 'max-w-2xl',
  }[maxWidth];

  return (
    <div
      className="fixed inset-0 flex items-center justify-center z-[10000] p-4"
      style={{ background: 'rgba(0, 0, 0, 0.8)' }}
      onClick={onClose}
    >
      <div
        className={`cyber-card ${maxWidthClass} w-full p-6 border-2 ${className}`}
        style={{ borderColor: variantStyles.borderColor }}
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div className="flex items-center justify-between mb-4">
          <div>
            <h2 className="text-2xl font-bold neon-text" style={{ color: variantStyles.titleColor }}>
              {title}
            </h2>
            {subtitle && (
              <p className="text-sm mt-1 font-mono" style={{ color: 'var(--text-secondary)' }}>
                {subtitle}
              </p>
            )}
          </div>
          <button
            onClick={onClose}
            className="transition-colors"
            style={{ color: 'var(--text-secondary)' }}
            onMouseEnter={(e) => (e.currentTarget.style.color = 'var(--neon-cyan)')}
            onMouseLeave={(e) => (e.currentTarget.style.color = 'var(--text-secondary)')}
          >
            <X size={24} weight="bold" />
          </button>
        </div>

        {/* Content */}
        {children}
      </div>
    </div>
  );
}
