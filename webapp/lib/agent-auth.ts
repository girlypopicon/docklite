import 'server-only';

import { cookies, headers } from 'next/headers';
import type { UserSession } from '@/types';
import { DEMO } from '@/lib/demo/flag';

const AGENT_URL =
  process.env.AGENT_URL ||
  process.env.DOCKLITE_AGENT_URL ||
  '';

function resolveAgentURL(): string | null {
  if (AGENT_URL) {
    return AGENT_URL.replace(/\/$/, '');
  }
  const headerStore = headers();
  const host =
    headerStore.get('x-forwarded-host') ||
    headerStore.get('host');
  if (!host) return null;
  const protoHeader = headerStore.get('x-forwarded-proto');
  const proto = protoHeader
    ? protoHeader.split(',')[0]?.trim()
    : 'http';
  return `${proto}://${host}`;
}

function buildCookieHeader(): string {
  const cookieStore = cookies();
  const all = cookieStore.getAll();
  if (!all.length) return '';
  return all.map((cookie) => `${cookie.name}=${cookie.value}`).join('; ');
}

export async function getAgentUser(): Promise<UserSession | null> {
  // The public demo has no server and no login: everyone is the demo admin.
  if (DEMO) return { userId: 1, username: 'demo', isAdmin: true, role: 'super_admin' };
  try {
    const baseURL = resolveAgentURL();
    if (!baseURL) return null;

    const cookieHeader = buildCookieHeader();
    const res = await fetch(`${baseURL}/api/auth/me`, {
      method: 'GET',
      headers: cookieHeader ? { Cookie: cookieHeader } : undefined,
      cache: 'no-store',
    });
    if (!res.ok) return null;
    const payload = await res.json();
    const user = payload?.user;
    if (!user) return null;

    return {
      userId: user.userId ?? user.id ?? 0,
      username: user.username ?? '',
      isAdmin: Boolean(user.isAdmin),
      role: user.role ?? (user.isAdmin ? 'admin' : 'user'),
    };
  } catch {
    return null;
  }
}
