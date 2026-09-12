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
