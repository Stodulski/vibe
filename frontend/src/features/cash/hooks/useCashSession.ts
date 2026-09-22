import { useQuery } from '@tanstack/react-query';
import { HTTPError } from 'ky';
import { cashApi } from '../api/cash.api';
import { queryKeys } from '@/shared/lib/queryKeys';

/**
 * The complex's currently open cash session, if any.
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
    queryFn: ({ signal }) => cashApi.current(id, signal),
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

  return { ...query, isClosed, isRealError };
}
