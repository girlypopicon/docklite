'use client';

import { useEffect } from 'react';

/**
 * Publishes where the top bar currently ends as the CSS variable --nav-offset.
 * The bar scrolls away with the page, so a sidebar fixed at "5rem from the top"
 * left a gap under it once you scrolled; sidebars use this instead to stay flush
 * against the bar's bottom edge while it's visible, and the screen's top once
 * it's gone.
 */
export function useNavOffset() {
  useEffect(() => {
    let frame = 0;
    const update = () => {
      frame = 0;
      const nav = document.querySelector('nav.card-vapor') as HTMLElement | null;
      const bottom = nav ? Math.max(0, Math.round(nav.getBoundingClientRect().bottom)) : 80;
      document.documentElement.style.setProperty('--nav-offset', `${bottom}px`);
    };
    const schedule = () => {
      if (!frame) frame = requestAnimationFrame(update);
    };
    update();
    window.addEventListener('scroll', schedule, { passive: true });
    window.addEventListener('resize', schedule);
    return () => {
      window.removeEventListener('scroll', schedule);
      window.removeEventListener('resize', schedule);
      if (frame) cancelAnimationFrame(frame);
      document.documentElement.style.removeProperty('--nav-offset');
    };
  }, []);
}
