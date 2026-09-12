// @vitest-environment node
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import middleware from './middleware';

const GOOGLEBOT = 'Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)';
const CHROME =
  'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0 Safari/537.36';

function request(path: string, userAgent: string): Request {
  return new Request(`https://app.vibe.com.ar${path}`, { headers: { 'user-agent': userAgent } });
}

/** `next()` marks the response for the SPA with this header rather than a body. */
function servedBySpa(response: Response): boolean {
  return response.headers.get('x-middleware-next') === '1';
}

function prerenderResponse(body = '<html lang="es"><head><title>Los Alamos | Vibe</title></head></html>') {
  return new Response(body, {
    status: 200,
    headers: { 'content-type': 'text/html; charset=utf-8', 'cache-control': 'public, max-age=300' },
  });
}

const fetchMock = vi.fn<typeof fetch>();

beforeEach(() => {
  vi.stubGlobal('fetch', fetchMock);
  fetchMock.mockResolvedValue(prerenderResponse());
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('middleware, serving the prerender', () => {
  it('serves a crawler the prerendered page for a slug', async () => {
    const response = await middleware(request('/los-alamos', GOOGLEBOT));

    expect(fetchMock).toHaveBeenCalledOnce();
    expect(fetchMock.mock.calls[0]?.[0]).toBe('https://api.vibe.com.ar/api/v1/public/prerender/los-alamos');
    expect(servedBySpa(response)).toBe(false);
    expect(response.status).toBe(200);
    await expect(response.text()).resolves.toContain('Los Alamos');
  });

  it('passes the prerender cache header through', async () => {
    const response = await middleware(request('/los-alamos', GOOGLEBOT));

    expect(response.headers.get('cache-control')).toBe('public, max-age=300');
    expect(response.headers.get('content-type')).toBe('text/html; charset=utf-8');
  });

  it('uses BACKEND_URL when the edge sets one', async () => {
    vi.stubEnv('BACKEND_URL', 'https://api-staging.vibe.com.ar');

    await middleware(request('/los-alamos', GOOGLEBOT));

    expect(fetchMock.mock.calls[0]?.[0]).toBe('https://api-staging.vibe.com.ar/api/v1/public/prerender/los-alamos');

    vi.unstubAllEnvs();
  });
});

describe('middleware, choosing what to prerender', () => {
  it('leaves a browser on the SPA', async () => {
    const response = await middleware(request('/los-alamos', CHROME));

    expect(servedBySpa(response)).toBe(true);
    expect(fetchMock).not.toHaveBeenCalled();
  });

  // The denylist this replaced was missing /profile, /reports and /admin, so a
  // crawler on any of them was handed the backend's 404.
  it.each(['/login', '/register', '/dashboard', '/profile', '/reports', '/admin', '/settings', '/complexes'])(
    'leaves a crawler on the SPA for the app route %s',
    async (path) => {
      const response = await middleware(request(path, GOOGLEBOT));

      expect(servedBySpa(response)).toBe(true);
      expect(fetchMock).not.toHaveBeenCalled();
    },
  );

  it.each(['/', '/los-alamos/book', '/los-alamos/book/confirm', '/admin/users'])(
    'only prerenders single-segment paths, not %s',
    async (path) => {
      const response = await middleware(request(path, GOOGLEBOT));

      expect(servedBySpa(response)).toBe(true);
      expect(fetchMock).not.toHaveBeenCalled();
    },
  );

  it.each(['/robots.txt', '/sw.js', '/manifest.json', '/favicon.ico'])(
    'never prerenders the static file %s',
    async (path) => {
      const response = await middleware(request(path, GOOGLEBOT));

      expect(servedBySpa(response)).toBe(true);
      expect(fetchMock).not.toHaveBeenCalled();
    },
  );

  it.each(['/Los-Alamos', '/los_alamos', '/-los-alamos', '/los--alamos'])(
    'never prerenders %s, which slugify could not have produced',
    async (path) => {
      const response = await middleware(request(path, GOOGLEBOT));

      expect(servedBySpa(response)).toBe(true);
      expect(fetchMock).not.toHaveBeenCalled();
    },
  );
});

describe('middleware, when the backend does not answer', () => {
  it('falls back to the SPA on a timeout', async () => {
    fetchMock.mockRejectedValue(new DOMException('The operation was aborted', 'TimeoutError'));

    const response = await middleware(request('/los-alamos', GOOGLEBOT));

    expect(servedBySpa(response)).toBe(true);
  });

  it('falls back to the SPA on a 500 instead of passing it to the crawler', async () => {
    fetchMock.mockResolvedValue(new Response('boom', { status: 500 }));

    const response = await middleware(request('/los-alamos', GOOGLEBOT));

    expect(servedBySpa(response)).toBe(true);
    expect(response.status).not.toBe(500);
  });

  it('falls back to the SPA on an unknown slug rather than the backend 404', async () => {
    fetchMock.mockResolvedValue(new Response('not found', { status: 404 }));

    const response = await middleware(request('/nope', GOOGLEBOT));

    expect(servedBySpa(response)).toBe(true);
  });
});
