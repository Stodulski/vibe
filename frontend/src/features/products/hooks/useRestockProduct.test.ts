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

async function renderRestockProduct(complexId: string, productId: string) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: 1, retryDelay: 0 } },
  });
  const wrapper = ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
  const { useRestockProduct } = await import('./useRestockProduct');
  const rendered = renderHook(() => useRestockProduct(complexId, productId), { wrapper });
  return { ...rendered, queryClient };
}

afterEach(() => {
  vi.clearAllMocks();
});

describe('useRestockProduct — invalidation', () => {
  it('invalidates the product, its stock movements, and the open cash session on success', async () => {
    server.use(
      http.post('*/complexes/:complexId/products/:productId/restock', () =>
        HttpResponse.json(
          { product: makeProduct({ id: 'p1', stock_on_hand: 30 }), stock_movement: makeStockMovement() },
          { status: 201 },
        ),
      ),
    );

    const { result, queryClient } = await renderRestockProduct('c1', 'p1');
    // Seed a cache entry per target key — `invalidateQueries` only flips
    // `isInvalidated` on a query that already exists; a key with nothing
    // cached under it has no state to observe either way.
    queryClient.setQueryData(queryKeys.products.byComplex('c1', true), { products: [] });
    queryClient.setQueryData(queryKeys.products.detail('c1', 'p1'), { product: makeProduct({ id: 'p1' }) });
    queryClient.setQueryData(queryKeys.products.stockMovements('c1', 'p1'), {
      pages: [{ stock_movements: [], metadata: { has_more: false } }],
      pageParams: [''],
    });
    queryClient.setQueryData(queryKeys.cash.current('c1'), { cash_session: null });
    // `cash.detailBase` covers a session's own detail page — the restock's
    // expense shows up in that session's movement ledger too, not only the
    // "current session" summary.
    queryClient.setQueryData(queryKeys.cash.detailBase('c1'), { cash_session: null });

    result.current.mutate({ quantity: 10, total_cost: 500000, method: 'cash' });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(toast.success).toHaveBeenCalledWith(ES_AR.products.restockSuccess);
    expect(queryClient.getQueryState(queryKeys.products.byComplex('c1', true))?.isInvalidated).toBe(true);
    expect(queryClient.getQueryState(queryKeys.products.detail('c1', 'p1'))?.isInvalidated).toBe(true);
    expect(queryClient.getQueryState(queryKeys.products.stockMovements('c1', 'p1'))?.isInvalidated).toBe(true);
    expect(queryClient.getQueryState(queryKeys.cash.current('c1'))?.isInvalidated).toBe(true);
    expect(queryClient.getQueryState(queryKeys.cash.detailBase('c1'))?.isInvalidated).toBe(true);
  });
});

describe('useRestockProduct — error handling', () => {
  // The toast now shows the neutral global copy (T5a review fix), not the
  // restock-specific "Para reponer necesitás..." wording — that one lives
  // only in `RestockDialog`'s own `ClosedTillNotice`, driven by the till
  // state it already reads, not by this mapping (see `serverErrors.ts`).
  it('shows the neutral cash-closed copy on a 409, not the restock-specific one, and invalidates the current session', async () => {
    server.use(
      http.post('*/complexes/:complexId/products/:productId/restock', () =>
        HttpResponse.json(
          {
            type: 'https://vibe.com.ar/problems/conflict',
            title: 'Conflict',
            status: 409,
            detail: 'no cash session is open',
          },
          { status: 409 },
        ),
      ),
    );

    const { result, queryClient } = await renderRestockProduct('c1', 'p1');
    queryClient.setQueryData(queryKeys.cash.current('c1'), { cash_session: null });
    result.current.mutate({ quantity: 10, total_cost: 500000, method: 'cash' });

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(toast.error).toHaveBeenCalledWith(ES_AR.validation.server.cashClosed);
    expect(toast.error).not.toHaveBeenCalledWith(ES_AR.products.restockNeedsOpenTill);
    expect(queryClient.getQueryState(queryKeys.cash.current('c1'))?.isInvalidated).toBe(true);
  });

  it('sends a UUID Idempotency-Key, reused on retry, never in the body', async () => {
    const keys: string[] = [];
    let attempt = 0;
    server.use(
      http.post('*/complexes/:complexId/products/:productId/restock', async ({ request }) => {
        keys.push(request.headers.get('Idempotency-Key') ?? '');
        attempt += 1;
        if (attempt === 1) return HttpResponse.json({ title: 'upstream unavailable' }, { status: 503 });
        const body = (await request.json()) as Record<string, unknown>;
        expect(body).not.toHaveProperty('attemptKey');
        return HttpResponse.json(
          { product: makeProduct({ id: 'p1' }), stock_movement: makeStockMovement() },
          { status: 201 },
        );
      }),
    );

    const { result } = await renderRestockProduct('c1', 'p1');
    result.current.mutate({ quantity: 10, total_cost: 500000, method: 'cash' });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(keys).toHaveLength(2);
    expect(keys[1]).toBe(keys[0]);
  });
});
