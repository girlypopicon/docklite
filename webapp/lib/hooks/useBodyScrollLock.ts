'use client';

import { useEffect } from 'react';

// Shared across every caller so stacked modals don't fight: the page stays
// locked until the last lock is released. Setting body overflow directly in
// each component let one closing (or merely re-rendering) component unlock
// the page while another modal was still open.
let activeLocks = 0;
let savedOverflow = '';
let savedPaddingRight = '';

/** Stop the page behind a modal or drawer from scrolling while `active`. */
export function useBodyScrollLock(active = true) {
  useEffect(() => {
    if (!active) return;
    const body = document.body;
    if (activeLocks === 0) {
      savedOverflow = body.style.overflow;
      savedPaddingRight = body.style.paddingRight;
      // Hiding the scrollbar widens the page; pad by its width so the layout
      // behind the modal doesn't jump sideways.
      const scrollbarWidth = window.innerWidth - document.documentElement.clientWidth;
      if (scrollbarWidth > 0) body.style.paddingRight = `${scrollbarWidth}px`;
      body.style.overflow = 'hidden';
    }
    activeLocks += 1;
    return () => {
      activeLocks -= 1;
      if (activeLocks === 0) {
        body.style.overflow = savedOverflow;
        body.style.paddingRight = savedPaddingRight;
      }
    };
  }, [active]);
}
