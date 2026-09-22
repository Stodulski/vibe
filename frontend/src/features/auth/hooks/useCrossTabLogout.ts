import { useEffect } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';
import { useStore } from '@/shared/stores';
import { STORAGE_KEYS } from '@/shared/lib/storageKeys';
import { loginUrlPreserving } from '@/shared/lib/ky';
import { queryKeys } from '@/shared/lib/queryKeys';
import type { SessionData } from './session';

/**
 * Every path this app actually guards with `<ProtectedRoute>` — see
 * `ownerRoutes.tsx`, `adminRoutes.tsx`. Kept as prefixes (not the router's
 * own route table) so this stays a plain string check with no dependency on
 * anything route-config-shaped: a feature hook importing `app/router` would
 * invert this app's dependency direction (`app` composes `features`, never
 * the other way — see CLAUDE.md's Dependency Inversion rule).
 *
 * That means this list is hand-kept, not derived, so it can go stale on its
 * own. `src/app/router/protectedPaths.test.tsx` is the guard: it walks the
 * actual route config and fails if a path wrapped in `<ProtectedRoute>` there
 * is missing here, or if a prefix here no longer matches any guarded route.
 * Exported for exactly that test.
 */
export const PROTECTED_PATH_PREFIXES = [
  '/onboarding',
  '/settings',
  '/dashboard',
  '/bookings',
  '/courts',
  '/clients',
  '/reports',
  '/profile',
  '/admin',
];

export function isProtectedPath(pathname: string): boolean {
  return PROTECTED_PATH_PREFIXES.some((prefix) => pathname === prefix || pathname.startsWith(`${prefix}/`));
}

/**
 * Ends this tab's session when another tab logs out.
 *
 * `logout()` (auth.slice.ts) and the 401 handler in `ky.ts` only ever tear
 * down the tab they run in: `queryClient` and the zustand store are both
 * per-tab, in-memory state, and `refetchOnWindowFocus` is off
 * (`queryClient.ts`), so a second tab left open keeps rendering a cached
 * owner/admin session after the first tab signs out. `logout()` writes a
 * changing value to `STORAGE_KEYS.SESSION_LOGOUT_BROADCAST`, which fires the
 * browser's `storage` event in every *other* tab on this origin (never the
 * tab that wrote it) — this hook is that reaction, run through
 * `logoutLocal()` so it does not write the key again and bounce back and
 * forth between tabs (see `AuthSlice.logoutLocal`).
 *
 * Mounted once, above every route, by `CrossTabLogout` — the sibling
 * component at the router root (`router.tsx`) that calls this hook and
 * renders nothing, the same reach `ScrollToTop` has for other cross-cutting
 * navigation concerns.
 */
export function useCrossTabLogout(): void {
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const logoutLocal = useStore((s) => s.logoutLocal);

  useEffect(() => {
    function handleStorage(event: StorageEvent) {
      if (event.key !== STORAGE_KEYS.SESSION_LOGOUT_BROADCAST) return;

      const session = queryClient.getQueryData<SessionData>(queryKeys.auth.me);
      const wasAuthenticated = !!session?.user;
      // An anonymous visitor on a public booking page has no session to end
      // — bouncing them to /login for someone else's logout would be its
      // own bug.
      if (!wasAuthenticated && !isProtectedPath(window.location.pathname)) return;

      logoutLocal();
      queryClient.clear();
      const target = loginUrlPreserving(window.location);
      if (target) {
        void navigate(target, { replace: true });
      }
    }

    window.addEventListener('storage', handleStorage);
    return () => {
      window.removeEventListener('storage', handleStorage);
    };
  }, [queryClient, navigate, logoutLocal]);
}
