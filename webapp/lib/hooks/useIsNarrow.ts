'use client';

import { useEffect, useState } from 'react';

/** True on phone-sized screens (under 768px wide), where side panels take the whole screen. */
export function useIsNarrow(query = '(max-width: 767px)') {
  const [narrow, setNarrow] = useState(false);
  useEffect(() => {
    const mq = window.matchMedia(query);
    const update = () => setNarrow(mq.matches);
    update();
    mq.addEventListener('change', update);
    return () => mq.removeEventListener('change', update);
  }, [query]);
  return narrow;
}
