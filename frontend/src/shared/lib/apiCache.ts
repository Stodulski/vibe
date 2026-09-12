/**
 * Name of the Workbox runtime cache declared in `vite.config.ts`. It holds
 * responses from the public booking endpoints only — every authenticated
 * route is left out of `runtimeCaching` on purpose (PWA-03).
 */
export const API_CACHE_NAME = 'api-cache';

/**
 * Drops the API runtime cache, best effort.
 *
 * Called when a session ends and when a new service worker takes over. Two
 * reasons, both about what the *next* person to use the browser sees:
 *
 * - After logout nothing that belonged to a session may stay readable in the
 *   origin's Cache Storage, even though only public endpoints are cached
 *   today — the guarantee should not depend on which routes are cached this
 *   month (PWA-03).
 * - After an update the new worker would otherwise keep serving the previous
 *   build's API responses for up to the entry's `maxAgeSeconds` (PWA-08).
 *
 * Never throws and never awaits: `caches` is undefined on an insecure origin
 * and in the test environment, and `delete` can reject when storage is
 * disabled by policy. A failed purge must not break a logout.
 */
export function purgeApiCache(): void {
  if (typeof caches === 'undefined') return;
  try {
    void caches.delete(API_CACHE_NAME).catch(() => {
      // Storage disabled or unavailable — nothing a caller can do here.
    });
  } catch {
    // Same: accessing `caches` itself can throw under a restrictive policy.
  }
}
