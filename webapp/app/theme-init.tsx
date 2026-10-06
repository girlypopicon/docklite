'use client';

import { useEffect } from 'react';
import { applyAppearance, loadAppearance } from '@/lib/appearance';

// Re-applies the saved look after hydration. The inline boot script in
// layout.tsx already did this before first paint; this keeps it correct
// if the attributes get reset (e.g. by a hot reload).
export default function ThemeInit() {
  useEffect(() => {
    applyAppearance(loadAppearance());
  }, []);

  return null;
}
