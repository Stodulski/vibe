import { describe, it, expect, vi } from 'vitest';
import { QueryClient, QueryClientProvider, QueryErrorResetBoundary, useQuery } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { server } from '@/test/msw/server';
import { ErrorBoundary } from '@/shared/components/common/ErrorBoundary';

vi.mock('@sentry/react', () => ({ captureException: vi.fn(), setUser: vi.fn() }));

function Stats() {
  const { data } = useQuery({
    queryKey: ['reset-scope', 'stats'],
    queryFn: async () => {
      const res = await fetch('https://api.test/stats');
      if (!res.ok) throw new Error(`HTTP ${String(res.status)}`);
      return (await res.json()) as { label: string };
    },
    retry: false,
    // What the app's own default does for a 5xx (see queryClient.ts).
    throwOnError: true,
  });
  return <div>{data?.label ?? 'sin datos'}</div>;
}

// The composition `routePage` builds around every route: the query-error reset
// scope on the outside, the boundary reading its `reset` on the inside.
function renderSection() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 5 * 60 * 1000 } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <QueryErrorResetBoundary>
        {({ reset }) => (
          <ErrorBoundary onReset={reset}>
            <Stats />
          </ErrorBoundary>
        )}
      </QueryErrorResetBoundary>
    </QueryClientProvider>,
  );
}

describe('a route section wrapped in QueryErrorResetBoundary (DATA-10)', () => {
  // Before this wiring the retry button only cleared `hasError`: the failed
  // query was still in the cache and re-threw on the very next render, so the
  // boundary came straight back and a full page reload was the only way out.
  it('re-runs the failed query when the boundary is retried, without reloading the page', async () => {
    const user = userEvent.setup();
    server.use(http.get('https://api.test/stats', () => new HttpResponse(null, { status: 500 })));

    renderSection();
    await screen.findByText(/algo sali.? mal/i);

    server.use(http.get('https://api.test/stats', () => HttpResponse.json({ label: '12 reservas' })));
    await user.click(screen.getByRole('button', { name: /reintentar/i }));

    await waitFor(() => {
      expect(screen.getByText('12 reservas')).toBeInTheDocument();
    });
  });
});
