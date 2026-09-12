import { QueryCache, QueryClient, MutationCache } from '@tanstack/react-query';
import * as Sentry from '@sentry/react';
import { HTTPError, NetworkError, TimeoutError } from 'ky';

/**
 * Statuses a query/mutation fails with often enough in normal use that
 * reporting every one to Sentry would be noise, not signal: an anonymous
 * visit (401), a role boundary (403), a stale link (404), or a form the
 * backend rejected (422). Anything else — 5xx, or the request never getting
 * a response at all — is reported below.
 */
const EXPECTED_STATUSES = new Set([401, 403, 404, 422]);

/**
 * Shared `onError` for both caches (see `queryClient` below).
 *
 * `src/shared/lib/ky.ts` is the one client every query and mutation goes
 * through, but it is reserved for a separate change — reporting from here
 * instead reads the same `HTTPError` ky already throws, without adding a
 * `beforeError` hook there.
 */
function reportQueryError(error: unknown): void {
  if (error instanceof HTTPError) {
    if (EXPECTED_STATUSES.has(error.response.status)) return;

    const requestId = error.response.headers.get('X-Request-Id');
    Sentry.captureException(error, {
      tags: {
        status: error.response.status,
        pathname: new URL(error.request.url).pathname,
        ...(requestId ? { request_id: requestId } : {}),
      },
    });
    return;
  }

  // A timeout or a dropped connection never reached `HTTPError` above — no
  // status, no response — but it is exactly the kind of failure a person
  // never sees a toast explain, so it still needs reporting.
  if (error instanceof TimeoutError || error instanceof NetworkError) {
    Sentry.captureException(error);
  }
}

export const queryClient = new QueryClient({
  queryCache: new QueryCache({ onError: reportQueryError }),
  mutationCache: new MutationCache({ onError: reportQueryError }),
  defaultOptions: {
    queries: {
      staleTime: 5 * 60 * 1000,
      gcTime: 10 * 60 * 1000,
      retry: 1,
      refetchOnWindowFocus: false,
    },
    mutations: {
      retry: 0,
    },
  },
});
