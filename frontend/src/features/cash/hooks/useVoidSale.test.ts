import { describe, it, expect, vi } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createElement } from 'react';
import { http, HttpResponse } from 'msw';
import { server } from '@/test/msw/server';
import { makeSale } from '@/test/factories';
import { queryKeys } from '@/shared/lib/queryKeys';
import { useVoidSale } from './useVoidSale';

vi.mock('sonner', () => ({ toast: { error: vi.fn(), success: vi.fn() } }));

function createWrapper() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } });
  const wrapper = ({ children }: { children: React.ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
  return { wrapper, queryClient };
}

describe('useVoidSale', () => {
  it('sends the note with an Idempotency-Key header and invalidates the session sales', async () => {
    let body: Record<string, unknown> = {};
    let key: string | null = null;
    server.use(
      http.post('*/complexes/:complexId/sales/:saleId/void', async ({ request }) => {
        body = (await request.json()) as Record<string, unknown>;
        key = request.headers.get('Idempotency-Key');
        return HttpResponse.json({ sale: makeSale({ voided_at: '2026-01-01T16:00:00Z' }) }, { status: 201 });
      }),
    );
    const { wrapper, queryClient } = createWrapper();
    const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries');
    const { result } = renderHook(() => useVoidSale('c1', 's1'), { wrapper });

    result.current.mutate({ saleId: 'sale1', note: 'roto' });
    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(body).toEqual({ note: 'roto' });
    expect(key).toMatch(/^[0-9a-f-]{36}$/);
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: queryKeys.sales.bySession('c1', 's1') });
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: queryKeys.products.byComplexAll('c1') });
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: queryKeys.cash.current('c1') });
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: queryKeys.cash.detailBase('c1') });
  });

  it('refetches on a 409 (already voided, or no session open)', async () => {
    server.use(
      http.post('*/complexes/:complexId/sales/:saleId/void', () =>
        HttpResponse.json({ title: 'this sale has already been voided' }, { status: 409 }),
      ),
    );
    const { wrapper, queryClient } = createWrapper();
    const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries');
    const { result } = renderHook(() => useVoidSale('c1', 's1'), { wrapper });

    result.current.mutate({ saleId: 'sale1' });
    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: queryKeys.sales.bySession('c1', 's1') });
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: queryKeys.cash.current('c1') });
  });
});
