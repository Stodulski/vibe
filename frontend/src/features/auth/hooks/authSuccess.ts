import { useNavigate, useLocation } from 'react-router-dom';
import { useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { useStore } from '@/shared/stores';
import { setSessionUser } from './session';
import { loginRedirectStateSchema } from '../schemas/auth.schema';
import type { AuthResponse } from '@/shared/types/api.types';

// Allowed redirect destinations after a successful login to prevent open redirect.
const SAFE_PREFIXES = [
  '/complexes',
  '/dashboard',
  '/bookings',
  '/courts',
  '/clients',
  '/settings',
  '/onboarding',
  '/admin',
];

function isSafeRedirect(path: string | undefined): path is string {
  if (!path) return false;
  // Only the path segment decides. A `?from=` carries the query string the
  // person was on (`/bookings?date=...`), and matching the raw value against
  // the prefixes would reject exactly those — while `/bookings?x` must still
  // not be able to slip past as something other than /bookings.
  const pathname = path.split('?')[0] ?? '';
  return SAFE_PREFIXES.some((prefix) => pathname === prefix || pathname.startsWith(prefix + '/'));
}

/**
 * Shared "just authenticated" success handling for every mutation that ends
 * a session the same way a plain login does — `useLogin` and
 * `useGoogleExchange`/`useGoogleComplete` (an existing-account Google sign-in
 * or a freshly-completed Google profile is a login in every way that
 * matters here).
 *
 * Seeds the session cache with the user the response carries (so no screen
 * waits on a second `GET /auth/me`), puts the CSRF token in the store,
 * dismisses any lingering error toast from an earlier failed attempt, and
 * redirects to the safe `location.state.from` path or the role-based default.
 */
export function useAuthSuccessHandler() {
  const navigate = useNavigate();
  const location = useLocation();
  const setCsrfToken = useStore((s) => s.setCsrfToken);
  const queryClient = useQueryClient();

  return (data: AuthResponse) => {
    // Sonner's <Toaster> lives above the router (in Providers), so it isn't
    // unmounted by the navigation below — an error toast from an earlier
    // failed attempt in this same session would otherwise still be sitting
    // on screen after a subsequent successful attempt.
    toast.dismiss();

    setCsrfToken(data.csrf_token);
    setSessionUser(queryClient, data.user);

    // `location.state` is null (not undefined) when the visit didn't come
    // through a ProtectedRoute redirect — e.g. navigating to /login directly
    // — and isn't guaranteed to match this shape even when it isn't null
    // (browser back/forward, a hand-edited URL). An `as` cast used to trust
    // it blindly: a `from.pathname` that wasn't a string hit
    // `isSafeRedirect`'s `.startsWith` and threw, which made TanStack Query
    // treat the whole mutation as failed and fire onError's
    // "invalid credentials" toast — on top of a login that had actually just
    // succeeded.
    const parsedState = loginRedirectStateSchema.safeParse(location.state);
    // `?from=` is the same intent arriving the only way it can survive the
    // hard navigation `ky.ts` does when a session cannot be refreshed:
    // `window.location.href` discards router state, so an expired token used
    // to cost the person the page they were on. Both paths are filtered by
    // `isSafeRedirect` below, so neither can be turned into an open redirect.
    const from = parsedState.success
      ? parsedState.data.from.pathname
      : (new URLSearchParams(location.search).get('from') ?? undefined);
    const defaultRoute = data.user.role === 'superadmin' ? '/admin' : '/complexes';
    void navigate(isSafeRedirect(from) ? from : defaultRoute, { replace: true });
  };
}
