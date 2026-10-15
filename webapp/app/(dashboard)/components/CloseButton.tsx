'use client';

import { X } from '@phosphor-icons/react';

interface Props {
  onClick: () => void;
  label?: string;
  disabled?: boolean;
  className?: string;
}

/**
 * The one close button for dialogs: a round, clearly visible button in the theme's own button colours
 * (the rainbow in Unicorn, the neon gradient elsewhere), big enough to hit. Pass `absolute top-4 right-4`
 * to pin it to the corner of a dialog that is `relative`.
 */
export default function CloseButton({ onClick, label = 'Close', disabled, className = '' }: Props) {
  return (
    <button
      type="button"
      onClick={(e) => {
        e.stopPropagation();
        onClick();
      }}
      disabled={disabled}
      aria-label={label}
      title={label}
      className={`btn-neon inline-flex items-center justify-center w-12 h-12 rounded-full shrink-0 transition-transform hover:scale-110 disabled:opacity-50 ${className}`}
    >
      <X size={28} weight="bold" />
    </button>
  );
}
