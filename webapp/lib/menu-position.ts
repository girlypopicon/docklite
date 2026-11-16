export interface MenuPlacement {
  left: number;
  top?: number;
  bottom?: number;
  /** The room the menu has on the side it opens to; the menu scrolls inside it instead of running off the screen. */
  maxHeight: number;
}

/**
 * Where a card's `⋯` menu goes: right-aligned under (or above) its button, on whichever side has more room, never
 * wider or taller than the window, so no row is ever off-screen.
 */
export function placeMenu(rect: DOMRect, width = 232, margin = 12, gap = 6): MenuPlacement {
  const w = Math.min(width, window.innerWidth - margin * 2);
  const left = Math.min(Math.max(margin, rect.right - w), window.innerWidth - w - margin);
  const below = window.innerHeight - rect.bottom - gap - margin;
  const above = rect.top - gap - margin;
  // Open downwards unless there is clearly more room above (a card near the bottom of the screen).
  if (below >= 320 || below >= above) return { left, top: rect.bottom + gap, maxHeight: Math.max(120, below) };
  return { left, bottom: window.innerHeight - rect.top + gap, maxHeight: Math.max(120, above) };
}
