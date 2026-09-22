import { describe, it, expect, vi } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createElement } from 'react';
import { http, HttpResponse } from 'msw';
import { server } from '@/test/msw/server';
import { makeCashMovement } from '@/test/factories';
import { queryKeys } from '@/shared/lib/queryKeys';
import { useVoidCashMovement } from './useVoidCashMovement';

vi.mock('sonner', () => ({ toast: { error: vi.fn(), success: vi.fn() } }));

function createWrapper() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } });
  const wrapper = ({ children }: { children: React.ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
  return { wrapper, queryClient };
}

describe('useVoidCashMovement', () => {
  it('invalidates current and detail on success', async () => {
    server.use(
      http.post('*/complexes/:complexId/cash-sessions/:sessionId/movements/:movementId/void', () =>
        HttpResponse.json({ cash_movement: makeCashMovement({ id: 'void1' }) }, { status: 201 }),
      ),
    );
    const { wrapper, queryClient } = createWrapper();
    const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries');
    const { result } = renderHook(() => useVoidCashMovement('c1', 's1'), { wrapper });

    result.current.mutate({ movementId: 'm1', note: undefined });
    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: queryKeys.cash.current('c1') });
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: queryKeys.cash.detail('c1', 's1') });
  });

  // Same fix as create-movement/close: a 409 here can mean "no session open"
  // (among other causes), so `current` must be invalidated alongside
  // `detail`, not just the latter (T3 review).
  it('invalidates current alongside detail on a 409', async () => {
    server.use(
      http.post('*/complexes/:complexId/cash-sessions/:sessionId/movements/:movementId/void', () =>
        HttpResponse.json({ title: 'Conflict' }, { status: 409 }),
      ),
    );
    const { wrapper, queryClient } = createWrapper();
    const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries');
    const { result } = renderHook(() => useVoidCashMovement('c1', 's1'), { wrapper });

    result.current.mutate({ movementId: 'm1', note: undefined });
    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: queryKeys.cash.current('c1') });
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: queryKeys.cash.detail('c1', 's1') });
  });

  it('never puts movementId or attemptKey in the request body — movementId is the URL segment', async () => {
    let body: Record<string, unknown> = {};
    let urlMovementId = '';
    server.use(
      http.post(
        '*/complexes/:complexId/cash-sessions/:sessionId/movements/:movementId/void',
        async ({ request, params }) => {
          body = (await request.json()) as Record<string, unknown>;
          urlMovementId = String(params.movementId);
          return HttpResponse.json({ cash_movement: makeCashMovement({ id: 'void1' }) }, { status: 201 });
        },
      ),
    );
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useVoidCashMovement('c1', 's1'), { wrapper });

    result.current.mutate({ movementId: 'm1', note: 'error de tipeo' });
    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(urlMovementId).toBe('m1');
    expect(body).toEqual({ note: 'error de tipeo' });
    expect(body).not.toHaveProperty('movementId');
    expect(body).not.toHaveProperty('attemptKey');
  });
});
