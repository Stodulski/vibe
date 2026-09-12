import { useQuery } from '@tanstack/react-query';
import { useStore } from '@/shared/stores';
import { queryKeys } from '@/shared/lib/queryKeys';
import { bootstrapSession } from '@/shared/lib/ky';
import { identifySession } from './session';
import type { User } from '@/shared/types/api.types';

interface AuthState {
  user: User | null;
  isLoading: boolean;
  isAuthenticated: boolean;
}

/**
 * Who is signed in, as one query.
 *
 * The user is server state and is cached as server state: this query's `data`
 * *is* the session, and every consumer — the sidebars, `RootRedirect`, the
 * route guards, the profile form — reads it through this hook. It used to be
 * copied into zustand and read from there, with the query `enabled: !user` and
 * returning `null` on purpose; that meant the session was fetched once per tab
 * and then never revalidated, however long the tab stayed open. Now `staleTime`
 * decides when to re-check, and a login, a profile update or a logout write
 * through `session.ts` instead of into a second source of truth.
 *
 * The store keeps only what is not server state: the in-memory CSRF token and
 * the selected complex.
 */
export function useAuth(): AuthState {
  const query = useQuery({
    queryKey: queryKeys.auth.me,
    queryFn: async ({ signal }) => {
      // On reload the cookies are still there but the in-memory CSRF token is
      // gone. GET /auth/me hands it back for the access token the cookie
      // carries, so the page load rotates nothing; the refresh token is only
      // spent when the access token has expired. See `bootstrapSession`.
      let session: Awaited<ReturnType<typeof bootstrapSession>>;
      try {
        session = await bootstrapSession(signal);
      } catch {
        // The session could not be read at all — treated as signed out, as a
        // failed refresh always was; the next guarded request retries.
        return identifySession(null);
      }
      if (!session) {
        // No valid session — user needs to log in.
        return identifySession(null);
      }

      useStore.getState().setCsrfToken(session.csrf_token);
      return identifySession(session.user);
    },
    staleTime: 5 * 60 * 1000,
    retry: false,
    // A session that cannot be read is "signed out", not a broken screen: the
    // queryFn above already answers `null` for it, and the route guards turn
    // that into a redirect to /login.
    throwOnError: false,
  });

  const user = query.data ?? null;

  return {
    user,
    isLoading: query.isLoading,
    isAuthenticated: !!user,
  };
}
