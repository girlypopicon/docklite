import React from 'react';
import { SpinnerGap } from '@phosphor-icons/react';

interface LoadingStateProps {
  message?: string;
  size?: number;
  className?: string;
}

export default function LoadingState({ message = 'Loading...', size = 16, className = '' }: LoadingStateProps) {
  return (
    <div className={`flex items-center gap-2 text-sm font-mono ${className}`} style={{ color: 'var(--text-secondary)' }}>
      <SpinnerGap size={size} weight="duotone" className="animate-spin" style={{ color: 'var(--text-cyan)' }} />
      {message}
    </div>
  );
}
