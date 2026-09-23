import { describe, it, expect, vi } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createElement } from 'react';
import { http, HttpResponse } from 'msw';
import { server } from '@/test/msw/server';
import { makeSale } from '@/test/factories';
import { queryKeys } from '@/shared/lib/queryKeys';
import { useCreateSale } from './useCreateSale';

vi.mock('sonner', () => ({ toast: { error: vi.fn(), success: vi.fn() } }));

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 }, mutations: { retry: 1, retryDelay: 0 } },
  });
  const wrapper = ({ children }: { children: React.ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
  return { wrapper, queryClient };
}

const CART = { items: [{ product_id: 'p1', quantity: 2 }], method: 'cash' as const };

describe('useCreateSale — payload and idempotency', () => {
  it('sends the payload shape the API expects, with an Idempotency-Key header', async () => {
    let body: Record<string, unknown> = {};
    let key: string | null = null;
    server.use(
      http.post('*/complexes/:complexId/sales', async ({ request }) => {
        body = (await request.json()) as Record<string, unknown>;
        key = request.headers.get('Idempotency-Key');
        return HttpResponse.json({ sale: makeSale(), stock_warnings: [] }, { status: 201 });
      }),
    );
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useCreateSale('c1', 's1'), { wrapper });

    result.current.mutate(CART);
    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(body).toEqual(CART);
    expect(body).not.toHaveProperty('attemptKey');
    expect(key).toMatch(/^[0-9a-f-]{36}$/);
  });

  it('reuses the same Idempotency-Key when a failed attempt is retried', async () => {
    const keys: string[] = [];
    server.use(
      http.post('*/complexes/:complexId/sales', ({ request }) => {
        const k = request.headers.get('Idempotency-Key') ?? '';
        keys.push(k);
        if (keys.length === 1) return HttpResponse.json({ title: 'upstream unavailable' }, { status: 503 });
        return HttpResponse.json({ sale: makeSale(), stock_warnings: [] }, { status: 201 });
      }),
    );
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useCreateSale('c1', 's1'), { wrapper });

    result.current.mutate(CART);
    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(keys).toHaveLength(2);
    expect(keys[1]).toBe(keys[0]);
  });
});

describe('useCreateSale — cache invalidation', () => {
  it("invalidates products, current session and this session's sales on success", async () => {
    server.use(
      http.post('*/complexes/:complexId/sales', () =>
        HttpResponse.json({ sale: makeSale(), stock_warnings: [] }, { status: 201 }),
      ),
    );
    const { wrapper, queryClient } = createWrapper();
    const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries');
    const { result } = renderHook(() => useCreateSale('c1', 's1'), { wrapper });

    result.current.mutate(CART);
    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: queryKeys.products.byComplexAll('c1') });
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: queryKeys.cash.current('c1') });
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: queryKeys.sales.bySession('c1', 's1') });
  });

  it('invalidates the current session on a 409 (closed till) too', async () => {
    server.use(
      http.post('*/complexes/:complexId/sales', () =>
        HttpResponse.json({ title: 'no cash session is open' }, { status: 409 }),
      ),
    );
    const { wrapper, queryClient } = createWrapper();
    const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries');
    const { result } = renderHook(() => useCreateSale('c1', 's1'), { wrapper });

    result.current.mutate(CART);
    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: queryKeys.cash.current('c1') });
  });
});
