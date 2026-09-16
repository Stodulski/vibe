import { setUser } from '@/shared/lib/observability';
import type { QueryClient } from '@tanstack/react-query';
import { queryKeys } from '@/shared/lib/queryKeys';
import type { User } from '@/shared/types/api.types';

/**
 * The session's user lives in the React Query cache under
 * `queryKeys.auth.me` — `useAuth` is the only reader, and these two are the
 * only writers.
 *
 * It used to be copied into the zustand store instead, which put it outside
 * the staleTime/invalidation cycle the rest of the app's server state runs on:
 * `useAuth`'s query was `enabled: !user`, so once the store held someone the
 * session was never revalidated again for as long as the tab stayed open.
 */

/**
 * What `queryKeys.auth.me` actually holds.
 *
 * `pendingEmail` travels alongside `user` rather than as a cache entry of its
 * own: both come from the same `GET`/`PUT /auth/me` answer, and a second key
 * populated as a side effect of the first query's `queryFn` would be one more
 * place for the two to drift out of sync.
 */
export interface SessionData {
  user: User | null;
  pendingEmail: string | null;
}

/**
 * Tells Sentry who is using the app, by id only — never an email or a name.
 *
 * Kept next to the cache write (and called by the store's `logout`) so every
 * event reported while a session is active carries the same `id`, exactly as
 * the store's old `setUser` guaranteed.
 */
export function identifySession(user: User | null): User | null {
  setUser(user ? { id: user.id } : null);
  return user;
}

/**
 * Seeds the session cache from a response that already carries the user —
 * a login, a Google sign-in, a profile update — so the screen updates without
 * a second round-trip to `GET /auth/me`.
 *
 * `pendingEmail` defaults to `null`: only `GET`/`PUT /auth/me` ever answer
 * with one, so a login or a logout (the other two callers) leaves the cache
 * at `null` until the next `/auth/me` refetch reports whatever is actually
 * pending for the signed-in account.
 */
export function setSessionUser(queryClient: QueryClient, user: User | null, pendingEmail: string | null = null): void {
  queryClient.setQueryData<SessionData>(queryKeys.auth.me, { user: identifySession(user), pendingEmail });
}
