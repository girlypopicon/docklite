// Replaces window.fetch (for /api calls) and window.WebSocket (for the terminal) with fakes.
// Called once, as early as possible, from DemoBoot.
import { handleDemoRequest } from './api';
import { DemoSocket } from './shell';

let installed = false;

export function installDemo(): void {
  if (installed || typeof window === 'undefined') return;
  installed = true;
  const realFetch = window.fetch.bind(window);

  window.fetch = async (input: RequestInfo | URL, init?: RequestInit): Promise<Response> => {
    const raw = typeof input === 'string' ? input : input instanceof URL ? input.href : input.url;
    let url: URL;
    try { url = new URL(raw, window.location.origin); } catch { return realFetch(input, init); }
    if (url.origin !== window.location.origin || !url.pathname.startsWith('/api/')) return realFetch(input, init);

    const method = (init?.method || (typeof input !== 'string' && !(input instanceof URL) ? input.method : 'GET') || 'GET').toUpperCase();
    let body: any = undefined;
    const rawBody = init?.body;
    if (typeof rawBody === 'string' && rawBody) { try { body = JSON.parse(rawBody); } catch { body = rawBody; } }
    else if (typeof FormData !== 'undefined' && rawBody instanceof FormData) {
      // File uploads: keep the name, size and (for small text files) the contents.
      const f = rawBody.get('file');
      let file: { name: string; size: number; text?: string } | undefined;
      if (f && typeof f !== 'string') file = { name: f.name, size: f.size, text: f.size <= 20_000 && /^(text\/|application\/(json|xml|javascript))/.test(f.type) ? await f.text() : undefined };
      body = { file, path: rawBody.get('path') };
    }
    await new Promise((r) => setTimeout(r, 60 + Math.random() * 140)); // feels like a real server
    const res = await handleDemoRequest({ method, path: url.pathname.replace(/\/+$/, '') || '/', query: url.searchParams, body });
    const isText = typeof res!.body === 'string';
    return new Response(isText ? res!.body : JSON.stringify(res!.body), { status: res!.status, headers: { 'Content-Type': isText ? 'text/plain' : 'application/json' } });
  };

  const RealWebSocket = window.WebSocket;
  function PatchedWebSocket(this: any, url: string | URL, protocols?: string | string[]) {
    const u = String(url);
    if (/\/api\/containers\/[^/]+\/terminal/.test(u)) return new DemoSocket(u) as any;
    return new RealWebSocket(url, protocols);
  }
  Object.assign(PatchedWebSocket, { CONNECTING: 0, OPEN: 1, CLOSING: 2, CLOSED: 3 });
  (window as any).WebSocket = PatchedWebSocket;
}
