import { describe, it, expect } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createElement } from 'react';
import { http, HttpResponse } from 'msw';
import { server } from '@/test/msw/server';
import { makeCashSession } from '@/test/factories';
import { useCashSession } from './useCashSession';

function createWrapper() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { gcTime: 0 } } });
  return ({ children }: { children: React.ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
}

const SUMMARY = { opening_cash: 500000, expected_cash: 650000, movement_totals: [], booking_payments: [] };
const NOT_FOUND_PROBLEM = { type: 'https://vibe.com.ar/problems/not-found', title: 'Not Found', status: 404 };

async function reportsClosedWithNoDataAndNoRetry() {
  let requests = 0;
  server.use(
    http.get('*/complexes/:complexId/cash-session', () => {
      requests += 1;
      return HttpResponse.json(NOT_FOUND_PROBLEM, { status: 404 });
    }),
  );

  const { result } = renderHook(() => useCashSession('c1'), { wrapper: createWrapper() });

  await waitFor(() => {
    expect(result.current.isClosed).toBe(true);
  });
  expect(result.current.isRealError).toBe(false);
  expect(result.current.data).toBeUndefined();
  // The query's own `retry` callback returns `false` for a 404 synchronously
  // — nothing is ever scheduled, so settling on `isClosed` already proves no
  // retry will follow; no real-time wait needed to "give a retry a beat"
  // (the lesson from the cash review: a timed wait for a non-event proves
  // nothing and only slows the suite down).
  expect(result.current.isFetching).toBe(false);
  expect(requests).toBe(1);
}

async function reportsRealErrorAfterRetryExhausted() {
  let requests = 0;
  server.use(
    http.get('*/complexes/:complexId/cash-session', () => {
      requests += 1;
      return HttpResponse.json({ title: 'Service Unavailable' }, { status: 503 });
    }),
  );

  const { result } = renderHook(() => useCashSession('c1'), { wrapper: createWrapper() });

  await waitFor(
    () => {
      expect(result.current.isRealError).toBe(true);
    },
    { timeout: 5000 },
  );
  expect(result.current.isClosed).toBe(false);
  // More than one request reached the server (the hook's own retry, each of
  // which the shared ky client also retries a couple of times on a 5xx) —
  // not pinned to an exact count, which would couple this to ky's own retry
  // limit; the behavior under test is that it eventually settles as
  // isRealError rather than retrying forever.
  expect(requests).toBeGreaterThan(1);
}

async function neverReportsStaleDataOnceClosed() {
  let call = 0;
  server.use(
    http.get('*/complexes/:complexId/cash-session', () => {
      call += 1;
      if (call === 1) return HttpResponse.json({ cash_session: makeCashSession(), summary: SUMMARY });
      return HttpResponse.json(NOT_FOUND_PROBLEM, { status: 404 });
    }),
  );

  const { result } = renderHook(() => useCashSession('c1'), { wrapper: createWrapper() });
  await waitFor(() => {
    expect(result.current.data).toBeDefined();
  });

  void result.current.refetch();

  await waitFor(() => {
    expect(result.current.isClosed).toBe(true);
  });
  expect(result.current.data).toBeUndefined();
}

describe('useCashSession', () => {
  it(
    'reports isClosed with no data on a 404 that IS the "no session open" answer, and never retries',
    reportsClosedWithNoDataAndNoRetry,
  );

  it(
    'reports isRealError (not isClosed) once a 5xx exhausts its one configured retry',
    reportsRealErrorAfterRetryExhausted,
  );

  it(
    "never reports isClosed alongside the previous open session's stale data once a refetch finds it closed",
    neverReportsStaleDataOnceClosed,
  );
});
