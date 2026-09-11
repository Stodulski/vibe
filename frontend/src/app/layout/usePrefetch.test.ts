import { renderHook, act } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createElement } from 'react';
import { usePrefetch } from './usePrefetch';

vi.mock('@/shared/lib/queryKeys', () => ({
  queryKeys: {
    dashboard: { stats: (id: string) => ['dashboard', 'stats', id] },
    bookings: { byDate: (id: string, date: string) => ['bookings', id, date] },
    courts: { byComplex: (id: string) => ['courts', id] },
  },
}));

vi.mock('@/features/dashboard/api/dashboard.api', () => ({
  dashboardApi: { getStats: vi.fn().mockResolvedValue({}) },
}));

vi.mock('@/features/courts/api/courts.api', () => ({
  courtsApi: { list: vi.fn().mockResolvedValue({}) },
}));

vi.mock('@/features/bookings/api/bookings.api', () => ({
  bookingsApi: { list: vi.fn().mockResolvedValue({}) },
}));

vi.mock('date-fns', () => ({
  format: () => '2026-03-18',
}));

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  // Spy on query — capture the spy itself so assertions never
  // reference `queryClient.query` unbound.
  const query = vi.spyOn(queryClient, 'query');
  return {
    queryClient,
    query,
    wrapper: ({ children }: { children: React.ReactNode }) =>
      createElement(QueryClientProvider, { client: queryClient }, children),
  };
}

// Shared by every route-key assertion below — extracted (alongside the
// nested `describe` further down) to keep this file's function bodies under
// the repo's max-lines-per-function cap.
function renderPrefetch(complexId: string | null) {
  const { wrapper, query } = createWrapper();
  const { result } = renderHook(() => usePrefetch(complexId), { wrapper });
  return { result, query };
}

describe('usePrefetch', () => {
  it('returns a function', () => {
    const { result } = renderPrefetch('complex-1');
    expect(typeof result.current).toBe('function');
  });

  it('does nothing when complexId is null', () => {
    const { result, query } = renderPrefetch(null);
    act(() => {
      result.current('/dashboard');
    });
    expect(query).not.toHaveBeenCalled();
  });

  // query() rejects where the old prefetchQuery absorbed, so every prefetch has
  // to attach its own rejection handler: a hover that cannot warm the cache
  // must stay invisible.
  //
  // The invariant is asserted directly rather than through a rejected promise.
  // Whether an unheld rejection gets reported depends on the runtime and on
  // when the queue drains, and a test that waits for that verdict passes just
  // as happily when nobody is listening. What the code either does or does not
  // do is attach the handler, so that is what is observed.
  it('attaches a rejection handler to every prefetch', () => {
    const { result, query } = renderPrefetch('complex-1');
    const attachCatch = vi.fn(() => Promise.resolve());
    query.mockReturnValue({ catch: attachCatch } as never);

    for (const route of ['/dashboard', '/bookings', '/courts']) {
      attachCatch.mockClear();
      act(() => {
        result.current(route);
      });
      expect(attachCatch, `${route} left its rejection unhandled`).toHaveBeenCalledTimes(1);
    }
  });
});

// A sibling `describe`, not nested inside the one above: `max-lines-per-function`
// counts a describe callback's full body including any nested describe's, so
// nesting these here wouldn't have shrunk the outer one — only a genuinely
// separate top-level call does.
describe('usePrefetch routing', () => {
  it('calls query for /dashboard route', () => {
    const { result, query } = renderPrefetch('complex-1');
    act(() => {
      result.current('/dashboard');
    });
    expect(query).toHaveBeenCalledWith(
      expect.objectContaining({
        queryKey: ['dashboard', 'stats', 'complex-1'],
      }),
    );
  });

  it('calls query for /bookings route', () => {
    const { result, query } = renderPrefetch('complex-1');
    act(() => {
      result.current('/bookings');
    });
    expect(query).toHaveBeenCalledWith(
      expect.objectContaining({
        queryKey: ['bookings', 'complex-1', '2026-03-18'],
      }),
    );
  });

  it('calls query for /courts route', () => {
    const { result, query } = renderPrefetch('complex-1');
    act(() => {
      result.current('/courts');
    });
    expect(query).toHaveBeenCalledWith(
      expect.objectContaining({
        queryKey: ['courts', 'complex-1'],
      }),
    );
  });

  it('does nothing for unknown routes', () => {
    const { result, query } = renderPrefetch('complex-1');
    act(() => {
      result.current('/unknown');
    });
    expect(query).not.toHaveBeenCalled();
  });
});
