import { describe, it, expect, vi, afterEach } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createElement, type ReactNode } from 'react';
import { http, HttpResponse } from 'msw';
import { toast } from 'sonner';
import { server } from '@/test/msw/server';
import { makeProduct, makeStockMovement } from '@/test/factories';
import { queryKeys } from '@/shared/lib/queryKeys';
import { ES_AR } from '@/shared/i18n/es_AR';

vi.mock('sonner', () => ({ toast: { error: vi.fn(), success: vi.fn() } }));

async function renderAdjustProduct(complexId: string, productId: string) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: 1, retryDelay: 0 } },
  });
  const wrapper = ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
  const { useAdjustProduct } = await import('./useAdjustProduct');
  const rendered = renderHook(() => useAdjustProduct(complexId, productId), { wrapper });
  return { ...rendered, queryClient };
}

afterEach(() => {
  vi.clearAllMocks();
});

describe('useAdjustProduct', () => {
  it('invalidates the product and its stock movements on success, sends a UUID key never in the body', async () => {
    let body: Record<string, unknown> = {};
    server.use(
      http.post('*/complexes/:complexId/products/:productId/adjustments', async ({ request }) => {
        body = (await request.json()) as Record<string, unknown>;
        return HttpResponse.json(
          {
            product: makeProduct({ id: 'p1', stock_on_hand: 17 }),
            stock_movement: makeStockMovement({ kind: 'adjustment', quantity: -3, reason: 'breakage' }),
          },
          { status: 201 },
        );
      }),
    );

    const { result, queryClient } = await renderAdjustProduct('c1', 'p1');
    // `invalidateQueries` only flips `isInvalidated` on a query that already
    // has cache state — seed one so the assertions below observe something.
    queryClient.setQueryData(queryKeys.products.detail('c1', 'p1'), { product: makeProduct({ id: 'p1' }) });
    queryClient.setQueryData(queryKeys.products.stockMovements('c1', 'p1'), {
      pages: [{ stock_movements: [], metadata: { has_more: false } }],
      pageParams: [''],
    });
    result.current.mutate({ quantity: -3, reason: 'breakage' });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(toast.success).toHaveBeenCalledWith(ES_AR.products.adjustSuccess);
    expect(body).toMatchObject({ quantity: -3, reason: 'breakage' });
    expect(body).not.toHaveProperty('attemptKey');
    expect(queryClient.getQueryState(queryKeys.products.detail('c1', 'p1'))?.isInvalidated).toBe(true);
    expect(queryClient.getQueryState(queryKeys.products.stockMovements('c1', 'p1'))?.isInvalidated).toBe(true);
  });

  it('shows the Spanish copy for a not-tracking-stock 409', async () => {
    server.use(
      http.post('*/complexes/:complexId/products/:productId/adjustments', () =>
        HttpResponse.json(
          {
            type: 'https://vibe.com.ar/problems/conflict',
            title: 'Conflict',
            status: 409,
            detail: 'this product does not track stock',
          },
          { status: 409 },
        ),
      ),
    );

    const { result } = await renderAdjustProduct('c1', 'p1');
    result.current.mutate({ quantity: 5, reason: 'count_correction' });

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(toast.error).toHaveBeenCalledWith(ES_AR.products.productNotTrackingStock);
  });
});
