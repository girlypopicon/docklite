'use client';

import { useEffect, useState } from 'react';

export type CertState = 'valid' | 'expiring' | 'expired' | 'none' | 'unknown';
export interface SiteCert {
  state: CertState;
  days: number | null;
}

// One check shared by every card on the page, repeated at most every five minutes.
let cache: { at: number; byDomain: Record<string, SiteCert> } | null = null;
let inflight: Promise<Record<string, SiteCert>> | null = null;
const listeners = new Set<(m: Record<string, SiteCert>) => void>();
const MAX_AGE_MS = 5 * 60 * 1000;

async function load(): Promise<Record<string, SiteCert>> {
  if (inflight) return inflight;
  inflight = (async () => {
    try {
      const res = await fetch('/api/ssl/status');
      if (!res.ok) throw new Error('ssl status');
      const data = await res.json();
      const byDomain: Record<string, SiteCert> = {};
      for (const c of [...(data.allCerts || []), ...(data.sites || [])]) {
        const entry: SiteCert = { state: (c.status || 'unknown') as CertState, days: typeof c.daysUntilExpiry === 'number' ? c.daysUntilExpiry : null };
        for (const d of c.domains?.length ? c.domains : [c.domain]) if (d) byDomain[String(d).toLowerCase()] = entry;
      }
      cache = { at: Date.now(), byDomain };
      listeners.forEach((fn) => fn(byDomain));
      return byDomain;
    } catch {
      return cache?.byDomain || {};
    } finally {
      inflight = null;
    }
  })();
  return inflight;
}

/** Forget the shared answer after a certificate was issued, renewed or removed, so the cards update. */
export function refreshSslStatus() {
  cache = null;
  void load();
}

/** The certificate state of every site, by domain; null until the first answer arrives. */
export function useSslStatuses(): Record<string, SiteCert> | null {
  const [map, setMap] = useState<Record<string, SiteCert> | null>(cache?.byDomain || null);
  useEffect(() => {
    listeners.add(setMap);
    if (!cache || Date.now() - cache.at > MAX_AGE_MS) void load();
    else setMap(cache.byDomain);
    return () => {
      listeners.delete(setMap);
    };
  }, []);
  return map;
}
