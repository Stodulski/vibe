import { describe, it, expect, vi, afterEach } from 'vitest';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { API_CACHE_NAME, purgeApiCache } from './apiCache';

describe('purgeApiCache', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('deletes the runtime cache the service worker writes API responses to', () => {
    const del = vi.fn().mockResolvedValue(true);
    vi.stubGlobal('caches', { delete: del });

    purgeApiCache();

    expect(del).toHaveBeenCalledWith(API_CACHE_NAME);
  });

  it('does nothing when the browser exposes no Cache Storage', () => {
    vi.stubGlobal('caches', undefined);

    expect(() => {
      purgeApiCache();
    }).not.toThrow();
  });

  it('swallows a rejected delete instead of surfacing an unhandled rejection', async () => {
    const del = vi.fn().mockRejectedValue(new Error('storage disabled'));
    vi.stubGlobal('caches', { delete: del });

    expect(() => {
      purgeApiCache();
    }).not.toThrow();
    await expect(del.mock.results[0]?.value).rejects.toThrow('storage disabled');
  });

  it('swallows a throwing Cache Storage accessor', () => {
    vi.stubGlobal('caches', {
      delete: () => {
        throw new Error('blocked by policy');
      },
    });

    expect(() => {
      purgeApiCache();
    }).not.toThrow();
  });
});

describe('API_CACHE_NAME', () => {
  it('is the cacheName the Workbox rule in vite.config.ts stores API responses under', () => {
    // vite.config.ts cannot import this module (it runs without the browser
    // Cache API), so the literal there is pinned here: renaming one side
    // without the other would silently turn purgeApiCache() into a no-op.
    // Vitest runs with the package directory as cwd.
    const viteConfig = readFileSync(resolve(process.cwd(), 'vite.config.ts'), 'utf8');
    expect(viteConfig).toContain(`cacheName: '${API_CACHE_NAME}',`);
  });
});
