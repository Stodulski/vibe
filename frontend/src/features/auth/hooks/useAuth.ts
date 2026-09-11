import { useQuery } from '@tanstack/react-query';
import { useStore } from '@/shared/stores';
import { queryKeys } from '@/shared/lib/queryKeys';
import { bootstrapSession } from '@/shared/lib/ky';
import type { User } from '@/shared/types/api.types';

interface AuthState {
  user: User | null;
  isLoading: boolean;
  isAuthenticated: boolean;
}

// The zustand store is the single source of truth for `user` — it's what
// Sidebar, RootRedirect and other non-auth consumers read directly. This
// query's job is only to run the session check (`bootstrapSession`) and feed
// the store; it used to also hand back its own `data.user` as a fallback,
// which meant two places could disagree about who's logged in (e.g. a stale
// cache entry surviving a missed `queryClient.clear()`). See
// 06-auth-shared-tooling.md M5.
export function useAuth(): AuthState {
  const { user, setUser } = useStore();

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
        return null;
      }
      if (!session) {
        // No valid session — user needs to log in.
        return null;
      }

      useStore.getState().setCsrfToken(session.csrf_token);
      setUser(session.user);
      return null;
    },
    enabled: !user,
    staleTime: 5 * 60 * 1000,
    retry: false,
  });

  return {
    user,
    isLoading: query.isLoading && !user,
    isAuthenticated: !!user,
  };
}
