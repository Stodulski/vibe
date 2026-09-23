import { useQuery } from '@tanstack/react-query';
import { HTTPError } from 'ky';
import { getCurrentCashSession } from '@/shared/api/cashSession.api';
import { queryKeys } from '@/shared/lib/queryKeys';

/**
 * The complex's currently open cash session, if any.
 *
 * Moved here from `features/cash/hooks/` (pos-cashbox T5a): `features/products`'
 * restock dialog needs to know whether the till is open before it lets
 * someone submit, and features never import from one another — same move as
 * `shared/lib/paymentMethods.ts`. `features/cash`'s own `cashApi.current`
 * delegates to `shared/api/cashSession.api`, so this stays the one query
 * both features observe against the same cache entry (`queryKeys.cash.current`).
 *
 * `GET /complexes/:id/cash-session` answers 404 when the till is closed — that
 * is a legitimate, expected outcome (see `useComplexBySlug` for the same
 * "don't retry, don't treat as a query failure" shape for a 404 that is
 * itself the answer), never retried and never surfaced as `isError`.
 */
export function useCashSession(complexId: string | null) {
  const id = complexId ?? '';
  const query = useQuery({
    queryKey: queryKeys.cash.current(id),
    queryFn: ({ signal }) => getCurrentCashSession(id, signal),
    enabled: !!complexId,
    throwOnError: false,
    retry: (failureCount, error) => {
      if (error instanceof HTTPError && error.response.status === 404) return false;
      return failureCount < 1;
    },
    // Expected cash is a live projection through `now()` — short staleTime so
    // reopening/refocusing the page shows a fresh number instead of one
    // computed minutes ago.
    staleTime: 30 * 1000,
  });

  const isClosed = query.error instanceof HTTPError && query.error.response.status === 404;
  const isRealError = query.isError && !isClosed;

  // One unambiguous state: React Query keeps serving the last successful
  // `data` through a failed background refetch, so a session that was open
  // and then closed (404 on refetch) used to report `isClosed: true`
  // alongside the stale open session's own fields — every consumer happened
  // to check `isClosed` before reading `data`, but the contract itself was
  // ambiguous. Closed means no data, full stop.
  return { ...query, data: isClosed ? undefined : query.data, isClosed, isRealError };
}
