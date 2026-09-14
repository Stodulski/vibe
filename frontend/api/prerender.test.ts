// @vitest-environment node
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { GET } from './prerender';

function request(slug: string | null): Request {
  const url = new URL('https://app.vibe.com.ar/api/prerender');
  if (slug !== null) url.searchParams.set('slug', slug);
  return new Request(url);
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

describe('GET /api/prerender, fetching the backend prerender', () => {
  it('fetches the exact backend prerender URL for the slug, accepting text/html', async () => {
    await GET(request('los-alamos'));

    expect(fetchMock).toHaveBeenCalledOnce();
    expect(fetchMock.mock.calls[0]?.[0]).toBe('https://api.vibe.com.ar/api/v1/public/prerender/los-alamos');
    const init = fetchMock.mock.calls[0]?.[1];
    expect(init?.headers).toMatchObject({ accept: 'text/html' });
  });

  it('serves a crawler the prerendered body on success', async () => {
    const response = await GET(request('los-alamos'));

    expect(response.status).toBe(200);
    await expect(response.text()).resolves.toContain('Los Alamos');
  });

  it('passes the upstream content-type and cache-control through', async () => {
    const response = await GET(request('los-alamos'));

    expect(response.headers.get('content-type')).toBe('text/html; charset=utf-8');
    expect(response.headers.get('cache-control')).toBe('public, max-age=300');
  });

  it('defaults content-type and cache-control when the backend omits them', async () => {
    // A string body makes the Response constructor infer its own
    // text/plain content-type, so it is stripped here to simulate the
    // backend genuinely sending none.
    const upstream = new Response('<html></html>', { status: 200 });
    upstream.headers.delete('content-type');
    fetchMock.mockResolvedValue(upstream);

    const response = await GET(request('los-alamos'));

    expect(response.headers.get('content-type')).toBe('text/html; charset=utf-8');
    expect(response.headers.get('cache-control')).toBe('public, max-age=300');
  });

  it('uses BACKEND_URL when the function has one set', async () => {
    vi.stubEnv('BACKEND_URL', 'https://api-staging.vibe.com.ar');

    await GET(request('los-alamos'));

    expect(fetchMock.mock.calls[0]?.[0]).toBe('https://api-staging.vibe.com.ar/api/v1/public/prerender/los-alamos');

    vi.unstubAllEnvs();
  });
});

describe('GET /api/prerender, re-validating the slug before fetching', () => {
  it('answers 404 without fetching when the slug is missing', async () => {
    const response = await GET(request(null));

    expect(response.status).toBe(404);
    expect(response.headers.get('cache-control')).toBe('no-store');
    expect(fetchMock).not.toHaveBeenCalled();
  });

  // A direct hit on /api/prerender?slug=login must not proxy: the rewrite's
  // own `source` exclusion is bypassed by calling the function directly.
  it.each(['login', 'register', 'dashboard', 'profile', 'reports', 'admin', 'settings', 'complexes'])(
    'answers 404 without fetching for the app route %s',
    async (slug) => {
      const response = await GET(request(slug));

      expect(response.status).toBe(404);
      expect(response.headers.get('cache-control')).toBe('no-store');
      expect(fetchMock).not.toHaveBeenCalled();
    },
  );

  it.each(['Foo_Bar', 'a.b', 'two/segments', 'Los-Alamos', 'los_alamos', '-los-alamos', 'los--alamos'])(
    'answers 404 without fetching for %s, which slugify could not have produced',
    async (slug) => {
      const response = await GET(request(slug));

      expect(response.status).toBe(404);
      expect(fetchMock).not.toHaveBeenCalled();
    },
  );

  it('accepts login-norte: the exclusion is anchored to the whole slug', async () => {
    const response = await GET(request('login-norte'));

    expect(fetchMock).toHaveBeenCalledOnce();
    expect(response.status).toBe(200);
  });
});

describe('GET /api/prerender, when the backend does not answer', () => {
  // A crawler indexes a 200 and drops a URL on a 500, but retries a 503, so
  // a transient failure must never become the generic shell (which would
  // replace the venue's metadata in the index) nor a 500.
  it('answers 503 with Retry-After on a timeout', async () => {
    fetchMock.mockRejectedValue(new DOMException('The operation was aborted', 'TimeoutError'));

    const response = await GET(request('los-alamos'));

    expect(response.status).toBe(503);
    expect(response.headers.get('retry-after')).toBe('60');
    expect(response.headers.get('cache-control')).toBe('no-store');
  });

  it('answers 503 with Retry-After on a network error', async () => {
    fetchMock.mockRejectedValue(new TypeError('fetch failed'));

    const response = await GET(request('los-alamos'));

    expect(response.status).toBe(503);
    expect(response.headers.get('retry-after')).toBe('60');
    expect(response.headers.get('cache-control')).toBe('no-store');
  });

  it('turns a backend 5xx into a 503 instead of passing a 500 to the crawler', async () => {
    fetchMock.mockResolvedValue(new Response('boom', { status: 500 }));

    const response = await GET(request('los-alamos'));

    expect(response.status).toBe(503);
    expect(response.headers.get('retry-after')).toBe('60');
  });

  it('keeps the backend 503 as a 503 for the crawler', async () => {
    fetchMock.mockResolvedValue(new Response(null, { status: 503, headers: { 'retry-after': '60' } }));

    const response = await GET(request('los-alamos'));

    expect(response.status).toBe(503);
  });

  it('passes a venue-not-found 404 through as a 404 rather than a 200 shell', async () => {
    fetchMock.mockResolvedValue(
      new Response('not found', { status: 404, headers: { 'x-prerender-result': 'venue-not-found' } }),
    );

    const response = await GET(request('nope'));

    expect(response.status).toBe(404);
    expect(response.headers.get('cache-control')).toBe('no-store');
  });

  it('treats a bare 404 (stale BACKEND_URL, gateway) as transient, not as a missing venue', async () => {
    fetchMock.mockResolvedValue(new Response('not found', { status: 404 }));

    const response = await GET(request('los-alamos'));

    expect(response.status).toBe(503);
    expect(response.headers.get('retry-after')).toBe('60');
  });
});
