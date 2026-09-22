import { describe, it, expect, vi } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createElement } from 'react';
import { http, HttpResponse } from 'msw';
import { server } from '@/test/msw/server';
import { makeCashSession } from '@/test/factories';
import { queryKeys } from '@/shared/lib/queryKeys';
import { useCloseCashSession } from './useCloseCashSession';

vi.mock('sonner', () => ({ toast: { error: vi.fn(), success: vi.fn() } }));

function createWrapper() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } });
  const wrapper = ({ children }: { children: React.ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
  return { wrapper, queryClient };
}

describe('useCloseCashSession', () => {
  it('invalidates current, sessions and detail on success', async () => {
    server.use(
      http.post('*/complexes/:complexId/cash-sessions/:sessionId/close', () =>
        HttpResponse.json({ cash_session: makeCashSession({ closed_at: '2026-01-01T00:00:00Z' }) }),
      ),
    );
    const { wrapper, queryClient } = createWrapper();
    const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries');
    const { result } = renderHook(() => useCloseCashSession('c1', 's1'), { wrapper });

    result.current.mutate({ counted_cash: 50000 });
    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: queryKeys.cash.current('c1') });
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: queryKeys.cash.sessions('c1') });
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: queryKeys.cash.detail('c1', 's1') });
  });

  it('invalidates current on a 409 (already closed or closed concurrently) too', async () => {
    server.use(
      http.post('*/complexes/:complexId/cash-sessions/:sessionId/close', () =>
        HttpResponse.json({ title: 'Conflict' }, { status: 409 }),
      ),
    );
    const { wrapper, queryClient } = createWrapper();
    const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries');
    const { result } = renderHook(() => useCloseCashSession('c1', 's1'), { wrapper });

    result.current.mutate({ counted_cash: 50000 });
    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: queryKeys.cash.current('c1') });
  });

  it('never puts attemptKey in the request body', async () => {
    let body: Record<string, unknown> = {};
    server.use(
      http.post('*/complexes/:complexId/cash-sessions/:sessionId/close', async ({ request }) => {
        body = (await request.json()) as Record<string, unknown>;
        return HttpResponse.json({ cash_session: makeCashSession({ closed_at: '2026-01-01T00:00:00Z' }) });
      }),
    );
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useCloseCashSession('c1', 's1'), { wrapper });

    result.current.mutate({ counted_cash: 50000 });
    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(body).toEqual({ counted_cash: 50000 });
    expect(body).not.toHaveProperty('attemptKey');
  });
});
