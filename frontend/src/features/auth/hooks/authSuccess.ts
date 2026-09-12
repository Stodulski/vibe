import { useNavigate, useLocation } from 'react-router-dom';
import { toast } from 'sonner';
import { useStore } from '@/shared/stores';
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
  return SAFE_PREFIXES.some((prefix) => path === prefix || path.startsWith(prefix + '/'));
}

/**
 * Shared "just authenticated" success handling for every mutation that ends
 * a session the same way a plain login does — `useLogin` and
 * `useGoogleSignIn`/`useGoogleComplete` (an existing-account Google sign-in
 * or a freshly-completed Google profile is a login in every way that
 * matters here).
 *
 * Sets the store (`user`, `csrf_token`), dismisses any lingering error toast
 * from an earlier failed attempt, and redirects to the safe
 * `location.state.from` path or the role-based default.
 */
export function useAuthSuccessHandler() {
  const navigate = useNavigate();
  const location = useLocation();
  const { setUser, setCsrfToken } = useStore();

  return (data: AuthResponse) => {
    // Sonner's <Toaster> lives above the router (in Providers), so it isn't
    // unmounted by the navigation below — an error toast from an earlier
    // failed attempt in this same session would otherwise still be sitting
    // on screen after a subsequent successful attempt.
    toast.dismiss();

    setCsrfToken(data.csrf_token);
    setUser(data.user);

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
    const from = parsedState.success ? parsedState.data.from.pathname : undefined;
    const defaultRoute = data.user.role === 'superadmin' ? '/admin' : '/complexes';
    void navigate(isSafeRedirect(from) ? from : defaultRoute, { replace: true });
  };
}
