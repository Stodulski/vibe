import { useQuery } from '@tanstack/react-query';
import { authApi } from '../api/auth.api';
import { useStore } from '@/shared/stores';
import { queryKeys } from '@/shared/lib/queryKeys';
import { refreshAccessToken } from '@/shared/lib/ky';
import type { User } from '@/shared/types/api.types';

interface AuthState {
  user: User | null;
  isLoading: boolean;
  isAuthenticated: boolean;
}

// The zustand store is the single source of truth for `user` — it's what
// Sidebar, RootRedirect and other non-auth consumers read directly. This
// query's job is only to run the session check (refresh + getMe) and feed
// the store; it used to also hand back its own `data.user` as a fallback,
// which meant two places could disagree about who's logged in (e.g. a stale
// cache entry surviving a missed `queryClient.clear()`). See
// 06-auth-shared-tooling.md M5.
export function useAuth(): AuthState {
  const { user, setUser } = useStore();

  const query = useQuery({
    queryKey: queryKeys.auth.me,
    queryFn: async ({ signal }) => {
      // On reload, cookies are still present but CSRF token (in-memory) is lost.
      // Refresh to get a new CSRF token and rotate tokens.
      if (!useStore.getState().csrfToken) {
        try {
          await refreshAccessToken();
        } catch {
          // No valid refresh token — user needs to log in.
          return null;
        }
      }

      const data = await authApi.getMe(signal);
      setUser(data.user);
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
