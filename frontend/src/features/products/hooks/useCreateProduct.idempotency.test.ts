import { describe, it, expect, vi } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createElement } from 'react';
import { http, HttpResponse } from 'msw';
import { server } from '@/test/msw/server';
import { makeProduct } from '@/test/factories';

vi.mock('sonner', () => ({ toast: { error: vi.fn(), success: vi.fn() } }));

/**
 * Same shape as `useOpenCashSession.idempotency.test.ts` — goes through the
 * real `productsApi` and ky client, MSW standing in for the backend, proving
 * the `Idempotency-Key` header reaches the wire and is reused on a retried
 * submit. `retryDelay: 0` (not fake timers) is what keeps this test from
 * waiting on a real backoff — the lesson from the cash review ("a 50ms
 * 'no retry' wait proves nothing").
 */
function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false, gcTime: 0 },
      mutations: { retry: 1, retryDelay: 0 },
    },
  });
  return ({ children }: { children: React.ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
}

function captureKeys(failures = 0): string[] {
  const keys: string[] = [];
  server.use(
    http.post('*/complexes/:complexId/products', ({ request }) => {
      keys.push(request.headers.get('Idempotency-Key') ?? '');
      if (keys.length <= failures) {
        return HttpResponse.json({ title: 'upstream unavailable' }, { status: 503 });
      }
      return HttpResponse.json({ product: makeProduct() }, { status: 201 });
    }),
  );
  return keys;
}

describe('useCreateProduct — Idempotency-Key', () => {
  it('sends a UUID key with the request', async () => {
    const keys = captureKeys();
    const { useCreateProduct } = await import('./useCreateProduct');
    const { result } = renderHook(() => useCreateProduct('c1'), { wrapper: createWrapper() });

    result.current.mutate({ name: 'Agua', price: 150000 });
    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(keys).toHaveLength(1);
    expect(keys[0]).toMatch(/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/);
  });

  it('reuses the same key when the attempt is retried', async () => {
    const keys = captureKeys(1);
    const { useCreateProduct } = await import('./useCreateProduct');
    const { result } = renderHook(() => useCreateProduct('c1'), { wrapper: createWrapper() });

    result.current.mutate({ name: 'Agua', price: 150000 });
    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(keys).toHaveLength(2);
    expect(keys[1]).toBe(keys[0]);
  });

  it('never puts attemptKey in the request body', async () => {
    let body: Record<string, unknown> = {};
    server.use(
      http.post('*/complexes/:complexId/products', async ({ request }) => {
        body = (await request.json()) as Record<string, unknown>;
        return HttpResponse.json({ product: makeProduct() }, { status: 201 });
      }),
    );

    const { useCreateProduct } = await import('./useCreateProduct');
    const { result } = renderHook(() => useCreateProduct('c1'), { wrapper: createWrapper() });

    result.current.mutate({ name: 'Agua', price: 150000 });
    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(body).toEqual({ name: 'Agua', price: 150000 });
    expect(body).not.toHaveProperty('attemptKey');
  });
});
