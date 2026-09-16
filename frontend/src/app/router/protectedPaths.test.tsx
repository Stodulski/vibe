import { describe, it, expect } from 'vitest';
import type { RouteObject } from 'react-router-dom';
import { ownerStandaloneRoutes, ownerDashboardRoutes } from './ownerRoutes';
import { adminRoutes } from './adminRoutes';
import { isProtectedPath, PROTECTED_PATH_PREFIXES } from '@/features/auth/hooks/useCrossTabLogout';

function leafPaths(routes: RouteObject[]): string[] {
  return routes.flatMap((route) => [
    ...(route.path ? [route.path] : []),
    ...(route.children ? leafPaths(route.children) : []),
  ]);
}

/**
 * `useCrossTabLogout.ts`'s `PROTECTED_PATH_PREFIXES` is a hand-kept list, not
 * derived from this router config (see that file's comment on why: a feature
 * hook cannot import `app/router` without inverting the app's dependency
 * direction). This is its regression test instead — it walks the actual
 * route arrays every `<ProtectedRoute>`-guarded page is registered in, so a
 * new protected route that forgets to update the hook fails here rather than
 * leaking a stale session across tabs after someone signs out elsewhere.
 */
describe('PROTECTED_PATH_PREFIXES stays in sync with the router config', () => {
  const guardedPaths = [
    ...leafPaths(ownerStandaloneRoutes),
    ...leafPaths(ownerDashboardRoutes),
    ...leafPaths(adminRoutes),
  ];

  it('found at least one guarded route to check against (a broken test fixture would pass vacuously otherwise)', () => {
    expect(guardedPaths.length).toBeGreaterThan(0);
  });

  it('covers every path guarded by <ProtectedRoute> in ownerRoutes.tsx and adminRoutes.tsx', () => {
    const uncovered = guardedPaths.filter((path) => !isProtectedPath(path));
    expect(uncovered).toEqual([]);
  });

  it('has no stale prefix left over from a route that no longer exists', () => {
    const stale = PROTECTED_PATH_PREFIXES.filter(
      (prefix) => !guardedPaths.some((path) => path === prefix || path.startsWith(`${prefix}/`)),
    );
    expect(stale).toEqual([]);
  });
});
