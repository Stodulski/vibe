import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, waitFor, act } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createElement } from 'react';
import { http, HttpResponse } from 'msw';
import { server } from '@/test/msw/server';
import { makeCashSession, makeProduct, makeSale } from '@/test/factories';
import { useSelectedComplex } from '@/features/complex/hooks/useSelectedComplex';
import { useSellPage } from './useSellPage';
import { saveCart } from '../../lib/cartStorage';

vi.mock('@/features/complex/hooks/useSelectedComplex', () => ({ useSelectedComplex: vi.fn() }));
vi.mock('sonner', () => ({ toast: { error: vi.fn(), success: vi.fn() } }));

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 }, mutations: { retry: 1, retryDelay: 0 } },
  });
  return ({ children }: { children: React.ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
}

const PRODUCT = makeProduct({ id: 'p1', name: 'Agua', price: 100000, active: true });

function mockCashSessionOpen() {
  server.use(
    http.get('*/complexes/:complexId/cash-session', () =>
      HttpResponse.json({
        cash_session: makeCashSession(),
        summary: { opening_cash: 0, expected_cash: 0, movement_totals: [], booking_payments: [] },
      }),
    ),
  );
}

function mockProducts(products = [PRODUCT]) {
  server.use(http.get('*/complexes/:complexId/products', () => HttpResponse.json({ products })));
}

function mockSalesList() {
  server.use(
    http.get('*/complexes/:complexId/sales', () => HttpResponse.json({ sales: [], metadata: { has_more: false } })),
  );
}

beforeEach(() => {
  window.sessionStorage.clear();
  vi.mocked(useSelectedComplex).mockReturnValue({
    complex: { id: 'c1' },
    selectedComplexId: 'c1',
  } as unknown as ReturnType<typeof useSelectedComplex>);
  mockCashSessionOpen();
  mockProducts();
  mockSalesList();
});

afterEach(() => {
  window.sessionStorage.clear();
});

describe('useSellPage — cart building', () => {
  it('adds a product to the cart and computes the total from its current price', async () => {
    const { result } = renderHook(() => useSellPage(), { wrapper: createWrapper() });
    await waitFor(() => {
      expect(result.current.visibleProducts).toHaveLength(1);
    });

    act(() => {
      result.current.addProduct(PRODUCT);
    });

    expect(result.current.lines).toEqual([{ productId: 'p1', quantity: 1 }]);
    expect(result.current.total).toBe(100000);
  });

  it('two quick taps in the same batch both land — a functional updater, not the render-time lines closure', async () => {
    const { result } = renderHook(() => useSellPage(), { wrapper: createWrapper() });
    await waitFor(() => {
      expect(result.current.visibleProducts).toHaveLength(1);
    });

    // Both calls happen before React commits a re-render — the exact "two
    // quick taps before a re-render" shape that loses an increment when
    // `add()` computes the next lines from this render's closed-over `lines`
    // instead of the latest committed state.
    act(() => {
      result.current.addProduct(PRODUCT);
      result.current.addProduct(PRODUCT);
    });

    expect(result.current.lines).toEqual([{ productId: 'p1', quantity: 2 }]);
  });
});

describe('useSellPage — complex switch', () => {
  it('replaces the cart with the new complex’s own cart, never carrying lines across complexes', async () => {
    const { result, rerender } = renderHook(() => useSellPage(), { wrapper: createWrapper() });
    await waitFor(() => {
      expect(result.current.visibleProducts).toHaveLength(1);
    });

    act(() => {
      result.current.addProduct(PRODUCT);
    });
    expect(result.current.lines).toEqual([{ productId: 'p1', quantity: 1 }]);

    vi.mocked(useSelectedComplex).mockReturnValue({
      complex: { id: 'c2' },
      selectedComplexId: 'c2',
    } as unknown as ReturnType<typeof useSelectedComplex>);
    rerender();

    await waitFor(() => {
      expect(result.current.lines).toEqual([]);
    });
  });
});

describe('useSellPage — sessionStorage persistence', () => {
  it('loads a cart left behind by an accidental reload', async () => {
    saveCart('c1', [{ productId: 'p1', quantity: 3 }]);
    const { result } = renderHook(() => useSellPage(), { wrapper: createWrapper() });

    await waitFor(() => {
      expect(result.current.lines).toEqual([{ productId: 'p1', quantity: 3 }]);
    });
    expect(result.current.droppedStaleNotice).toBe(false);
  });

  it('drops a stored line for a product no longer active and flags it', async () => {
    saveCart('c1', [
      { productId: 'p1', quantity: 1 },
      { productId: 'deactivated', quantity: 2 },
    ]);
    const { result } = renderHook(() => useSellPage(), { wrapper: createWrapper() });

    await waitFor(() => {
      expect(result.current.droppedStaleNotice).toBe(true);
    });
    expect(result.current.lines).toEqual([{ productId: 'p1', quantity: 1 }]);
  });
});

describe('useSellPage — charge payload and idempotency', () => {
  it('sends the payload shape the API expects', async () => {
    let body: Record<string, unknown> = {};
    server.use(
      http.post('*/complexes/:complexId/sales', async ({ request }) => {
        body = (await request.json()) as Record<string, unknown>;
        return HttpResponse.json({ sale: makeSale({ total: 200000 }), stock_warnings: [] }, { status: 201 });
      }),
    );
    const { result } = renderHook(() => useSellPage(), { wrapper: createWrapper() });
    await waitFor(() => {
      expect(result.current.visibleProducts).toHaveLength(1);
    });

    act(() => {
      result.current.addProduct(PRODUCT);
      result.current.incrementCartLine('p1');
    });

    act(() => {
      result.current.charge();
    });

    await waitFor(() => {
      expect(result.current.saleResult).not.toBeNull();
    });
    expect(body).toEqual({ items: [{ product_id: 'p1', quantity: 2 }], method: 'cash' });
  });

  it('reuses the same Idempotency-Key when a failed attempt is retried', async () => {
    const keys: string[] = [];
    server.use(
      http.post('*/complexes/:complexId/sales', ({ request }) => {
        const key = request.headers.get('Idempotency-Key') ?? '';
        keys.push(key);
        if (keys.length === 1) return HttpResponse.json({ title: 'upstream unavailable' }, { status: 503 });
        return HttpResponse.json({ sale: makeSale(), stock_warnings: [] }, { status: 201 });
      }),
    );
    const { result } = renderHook(() => useSellPage(), { wrapper: createWrapper() });
    await waitFor(() => {
      expect(result.current.visibleProducts).toHaveLength(1);
    });

    act(() => {
      result.current.addProduct(PRODUCT);
    });
    act(() => {
      result.current.charge();
    });

    await waitFor(() => {
      expect(result.current.saleResult).not.toBeNull();
    });
    expect(keys).toHaveLength(2);
    expect(keys[1]).toBe(keys[0]);
  });
});

describe('useSellPage — charge success', () => {
  it('on success: clears the cart, wipes the stored cart, and surfaces the server total and stock warnings', async () => {
    server.use(
      http.post('*/complexes/:complexId/sales', () =>
        HttpResponse.json(
          {
            sale: makeSale({ total: 555500 }),
            stock_warnings: [{ product_id: 'p1', product_name: 'Agua', stock_on_hand: -1 }],
          },
          { status: 201 },
        ),
      ),
    );
    const { result } = renderHook(() => useSellPage(), { wrapper: createWrapper() });
    await waitFor(() => {
      expect(result.current.visibleProducts).toHaveLength(1);
    });

    act(() => {
      result.current.addProduct(PRODUCT);
    });
    act(() => {
      result.current.charge();
    });

    await waitFor(() => {
      expect(result.current.saleResult).not.toBeNull();
    });
    expect(result.current.saleResult?.sale.total).toBe(555500);
    expect(result.current.saleResult?.stockWarnings).toHaveLength(1);
    expect(result.current.lines).toEqual([]);
    // The persistence effect runs after the state update commits — proves
    // the clear reaches storage too, not just in-memory state.
    await waitFor(() => {
      expect(window.sessionStorage.getItem('vibe_pos_cart_c1')).toBeNull();
    });
  });
});

describe('useSellPage — charge success while the cart keeps changing', () => {
  it('keeps a line added while the charge is in flight — only what was actually sent is removed on success', async () => {
    let resolveSale!: () => void;
    const salePromise = new Promise<void>((resolve) => {
      resolveSale = resolve;
    });
    server.use(
      http.post('*/complexes/:complexId/sales', async () => {
        await salePromise;
        return HttpResponse.json({ sale: makeSale(), stock_warnings: [] }, { status: 201 });
      }),
    );
    const { result } = renderHook(() => useSellPage(), { wrapper: createWrapper() });
    await waitFor(() => {
      expect(result.current.visibleProducts).toHaveLength(1);
    });

    act(() => {
      result.current.addProduct(PRODUCT);
    });
    act(() => {
      result.current.charge();
    });
    await waitFor(() => {
      expect(result.current.isCharging).toBe(true);
    });

    // The charge button is disabled while pending, but nothing else stops
    // this — a stepper tap, or another tap on the same tile, landing before
    // the response does.
    act(() => {
      result.current.incrementCartLine('p1');
    });
    expect(result.current.lines).toEqual([{ productId: 'p1', quantity: 2 }]);

    resolveSale();
    await waitFor(() => {
      expect(result.current.saleResult).not.toBeNull();
    });

    // 2 in the cart at completion time minus the 1 that was actually charged.
    expect(result.current.lines).toEqual([{ productId: 'p1', quantity: 1 }]);
  });
});
