import { describe, it, expect, vi, afterEach } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createElement, type ReactNode } from 'react';
import { http, HttpResponse } from 'msw';
import { toast } from 'sonner';
import { server } from '@/test/msw/server';
import { makeProduct } from '@/test/factories';
import { queryKeys } from '@/shared/lib/queryKeys';
import { ES_AR } from '@/shared/i18n/es_AR';

vi.mock('sonner', () => ({ toast: { error: vi.fn(), success: vi.fn() } }));

async function renderUpdateProduct(complexId: string) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  const wrapper = ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
  const { useUpdateProduct } = await import('./useUpdateProduct');
  const rendered = renderHook(() => useUpdateProduct(complexId), { wrapper });
  return { ...rendered, queryClient };
}

afterEach(() => {
  vi.clearAllMocks();
});

describe('useUpdateProduct — deactivate/reactivate toasts', () => {
  it('toasts "Producto desactivado" (not the generic update copy) when active goes to false', async () => {
    server.use(
      http.patch('*/complexes/:complexId/products/:productId', () =>
        HttpResponse.json({ product: makeProduct({ id: 'p1', active: false }) }),
      ),
    );

    const { result } = await renderUpdateProduct('c1');
    result.current.mutate({ productId: 'p1', data: { version: 1, active: false }, toggledActive: false });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(toast.success).toHaveBeenCalledWith(ES_AR.products.deactivateSuccess);
    expect(toast.success).not.toHaveBeenCalledWith(ES_AR.products.updateSuccess);
  });

  it('toasts "Producto reactivado" when active goes to true', async () => {
    server.use(
      http.patch('*/complexes/:complexId/products/:productId', () =>
        HttpResponse.json({ product: makeProduct({ id: 'p1', active: true }) }),
      ),
    );

    const { result } = await renderUpdateProduct('c1');
    result.current.mutate({ productId: 'p1', data: { version: 1, active: true }, toggledActive: true });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(toast.success).toHaveBeenCalledWith(ES_AR.products.reactivateSuccess);
  });

  it('toasts the deactivate-specific error fallback (not the generic one) for a non-conflict failure', async () => {
    server.use(http.patch('*/complexes/:complexId/products/:productId', () => HttpResponse.json({}, { status: 500 })));

    const { result } = await renderUpdateProduct('c1');
    result.current.mutate({ productId: 'p1', data: { version: 1, active: false }, toggledActive: false });

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(toast.error).toHaveBeenCalledWith(ES_AR.products.deactivateError);
  });
});

describe('useUpdateProduct — toast intent, not payload shape', () => {
  it('keeps the generic update toast for an ordinary content edit (no active field)', async () => {
    server.use(
      http.patch('*/complexes/:complexId/products/:productId', () =>
        HttpResponse.json({ product: makeProduct({ id: 'p1' }) }),
      ),
    );

    const { result } = await renderUpdateProduct('c1');
    result.current.mutate({ productId: 'p1', data: { version: 1, name: 'Agua con gas' } });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(toast.success).toHaveBeenCalledWith(ES_AR.products.updateSuccess);
  });

  // Reproduces the bug the intent field fixes: the toast is picked by the
  // explicit `toggledActive` intent, never by `data.active`'s mere presence —
  // an ordinary edit that happens to include `active` in its payload (with no
  // `toggledActive`) must still get the generic toast, not the
  // deactivate/reactivate one.
  it('keeps the generic update toast for an edit whose payload includes active but no toggledActive intent', async () => {
    server.use(
      http.patch('*/complexes/:complexId/products/:productId', () =>
        HttpResponse.json({ product: makeProduct({ id: 'p1', active: false }) }),
      ),
    );

    const { result } = await renderUpdateProduct('c1');
    result.current.mutate({ productId: 'p1', data: { version: 1, name: 'Agua con gas', active: false } });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(toast.success).toHaveBeenCalledWith(ES_AR.products.updateSuccess);
    expect(toast.success).not.toHaveBeenCalledWith(ES_AR.products.deactivateSuccess);
  });
});

describe('useUpdateProduct — settled invalidation', () => {
  it('invalidates the product list and the product detail once the mutation settles', async () => {
    server.use(
      http.patch('*/complexes/:complexId/products/:productId', () =>
        HttpResponse.json({ product: makeProduct({ id: 'p1' }) }),
      ),
    );

    const { result, queryClient } = await renderUpdateProduct('c1');

    queryClient.setQueryData(queryKeys.products.byComplex('c1', true), { products: [] });
    queryClient.setQueryData(queryKeys.products.detail('c1', 'p1'), { product: makeProduct({ id: 'p1' }) });

    result.current.mutate({ productId: 'p1', data: { version: 1, name: 'Agua con gas' } });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(queryClient.getQueryState(queryKeys.products.byComplex('c1', true))?.isInvalidated).toBe(true);
    expect(queryClient.getQueryState(queryKeys.products.detail('c1', 'p1'))?.isInvalidated).toBe(true);
  });
});
