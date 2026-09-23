import { describe, it, expect, vi, afterEach } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createElement, type ReactNode } from 'react';
import { http, HttpResponse } from 'msw';
import { toast } from 'sonner';
import { server } from '@/test/msw/server';
import { makeProduct } from '@/test/factories';
import { ES_AR } from '@/shared/i18n/es_AR';

vi.mock('sonner', () => ({ toast: { error: vi.fn(), success: vi.fn() } }));

/** Same shape as `useUpdateCourt.version.test.ts` — real `productsApi`/ky client, MSW standing in for the backend. */
async function renderUpdateProduct(complexId: string) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  const wrapper = ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
  const { useUpdateProduct } = await import('./useUpdateProduct');
  return renderHook(() => useUpdateProduct(complexId), { wrapper });
}

afterEach(() => {
  vi.clearAllMocks();
});

describe('useUpdateProduct — conflict mapping', () => {
  it('sends the body (including version) on the PATCH', async () => {
    let receivedBody: unknown;
    server.use(
      http.patch('*/complexes/:complexId/products/:productId', async ({ request }) => {
        receivedBody = await request.json();
        return HttpResponse.json({ product: makeProduct({ id: 'p1', version: 2 }) });
      }),
    );

    const { result } = await renderUpdateProduct('c1');
    result.current.mutate({ productId: 'p1', data: { version: 1, name: 'Agua con gas' } });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });
    expect(receivedBody).toMatchObject({ version: 1, name: 'Agua con gas' });
  });

  it('toasts the product-specific reload message on a stale-version 409, distinct from the generic one', async () => {
    server.use(
      http.patch('*/complexes/:complexId/products/:productId', () =>
        HttpResponse.json(
          {
            type: 'https://vibe.com.ar/problems/stale-version',
            title: 'Stale Version',
            status: 409,
            detail: 'unable to update the record due to an edit conflict, please try again',
          },
          { status: 409 },
        ),
      ),
    );

    const { result } = await renderUpdateProduct('c1');
    result.current.mutate({ productId: 'p1', data: { version: 1, name: 'Agua con gas' } });

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(toast.error).toHaveBeenCalledWith(ES_AR.products.updateConflict);
    expect(toast.error).not.toHaveBeenCalledWith(ES_AR.common.versionConflict);
  });

  it('shows the owner-specified Spanish copy for the "turn off stock tracking" 409', async () => {
    server.use(
      http.patch('*/complexes/:complexId/products/:productId', () =>
        HttpResponse.json(
          {
            type: 'https://vibe.com.ar/problems/conflict',
            title: 'Conflict',
            status: 409,
            detail: 'cannot stop tracking stock while stock_on_hand is not zero',
          },
          { status: 409 },
        ),
      ),
    );

    const { result } = await renderUpdateProduct('c1');
    result.current.mutate({ productId: 'p1', data: { tracks_stock: false } });

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(toast.error).toHaveBeenCalledWith(ES_AR.products.stockNotZero);
  });
});
