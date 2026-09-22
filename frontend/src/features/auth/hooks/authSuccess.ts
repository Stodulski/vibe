import { useNavigate, useLocation } from 'react-router-dom';
import { useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { useStore } from '@/shared/stores';
import { setSessionUser } from './session';
import { loginRedirectStateSchema } from '../schemas/auth.schema';
import type { AuthResponse } from '@/shared/types/api.types';

// Allowed redirect destinations after a successful login to prevent open redirect.
const SAFE_PREFIXES = [
  '/dashboard',
  '/bookings',
  '/courts',
  '/clients',
  '/settings',
  '/onboarding',
  '/admin',
];

/**
 * Whether `path` is somewhere inside this app that a just-authenticated
 * person may be sent to. Every redirect target passes through here, wherever
 * it came from, so no caller can turn one into an open redirect.
 *
 * Exported for `rememberGoogleReturnPath`, which has to apply the same test
 * before a destination is parked in `sessionStorage` across the Google
 * redirect hop — a value that cannot survive that trip is not worth storing.
 */
export function isSafeRedirect(path: string | undefined): path is string {
  if (!path) return false;
  // Only the path segment decides. A `?from=` carries the query string the
  // person was on (`/bookings?date=...`), and matching the raw value against
  // the prefixes would reject exactly those — while `/bookings?x` must still
  // not be able to slip past as something other than /bookings.
  const pathname = path.split('?')[0] ?? '';
  return SAFE_PREFIXES.some((prefix) => pathname === prefix || pathname.startsWith(prefix + '/'));
}

/**
 * The destination a visitor was heading for before they were sent to sign in,
 * as the two mechanisms that can carry it state it — unfiltered, because
 * every caller runs the result through {@link isSafeRedirect} at the point of
 * use.
 *
 * `ProtectedRoute` puts it in router state. The 401 handler in
 * `src/shared/lib/ky.ts` cannot: it ends the session with
 * `window.location.href`, and router state does not survive a document load,
 * so it appends `?from=` instead. Router state wins when both are present.
 *
 * `state` is `unknown` on purpose — `history.state` is not guaranteed to be
 * what this app put there (back/forward, a hand-edited URL, an extension), so
 * it is parsed rather than cast.
 */
export function readIntendedFrom(state: unknown, search: string): string | undefined {
  const parsedState = loginRedirectStateSchema.safeParse(state);
  if (parsedState.success) return parsedState.data.from.pathname;
  return new URLSearchParams(search).get('from') ?? undefined;
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
 *
 * A caller that knows the destination better than the current location does
 * passes it as `options.from`, and it wins. Exactly one does: the Google
 * redirect flow signs in on `/auth/google/return`, a page the visitor never
 * chose and which carries neither the router state nor the `?from=` the
 * original `/login` had — so `useGoogleExchange` hands back the destination
 * it parked before leaving for Google. The override is filtered by
 * {@link isSafeRedirect} exactly like the other two, so an override is not a
 * way around the allowlist.
 */
export function useAuthSuccessHandler() {
  const navigate = useNavigate();
  const location = useLocation();
  const setCsrfToken = useStore((s) => s.setCsrfToken);
  const queryClient = useQueryClient();

  return (data: AuthResponse, options?: { from?: string | undefined }) => {
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
    // succeeded. `readIntendedFrom` parses both carriers; see its own note on
    // why `?from=` exists alongside router state. All three candidates are
    // filtered by `isSafeRedirect` below, so none can become an open redirect.
    const from = options?.from ?? readIntendedFrom(location.state, location.search);
    const defaultRoute = data.user.role === 'superadmin' ? '/admin' : '/dashboard';
    void navigate(isSafeRedirect(from) ? from : defaultRoute, { replace: true });
  };
}
