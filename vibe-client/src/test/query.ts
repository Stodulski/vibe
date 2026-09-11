import type { UseQueryResult } from '@tanstack/react-query';

/**
 * Builds a fully-populated, success-shaped `UseQueryResult<T>` fixture for
 * `vi.mocked(useSomeQuery).mockReturnValue(queryResult(data))`.
 *
 * `UseQueryResult` is a large discriminated union (success/pending/error/...),
 * so the base object is intentionally cast to the public type at the single
 * return boundary below — this is a bounded, typed assertion to the real
 * TanStack Query type, never `any` or `unknown`. Override individual fields
 * (e.g. `isLoading`, `status`) to model loading/error fixtures.
 */
export function queryResult<T>(data: T, overrides: Partial<UseQueryResult<T>> = {}): UseQueryResult<T> {
  const refetch = (): Promise<UseQueryResult<T>> => Promise.resolve(result);
  const base = {
    data,
    dataUpdatedAt: Date.now(),
    error: null,
    errorUpdatedAt: 0,
    failureCount: 0,
    failureReason: null,
    errorUpdateCount: 0,
    isError: false,
    isFetched: true,
    isFetchedAfterMount: true,
    isFetching: false,
    isLoading: false,
    isPending: false,
    isLoadingError: false,
    isInitialLoading: false,
    isPaused: false,
    isPlaceholderData: false,
    isRefetchError: false,
    isRefetching: false,
    isStale: false,
    isSuccess: true,
    status: 'success',
    fetchStatus: 'idle',
    refetch,
    promise: Promise.resolve(data),
    ...overrides,
  };
  const result = base as UseQueryResult<T>;
  return result;
}
