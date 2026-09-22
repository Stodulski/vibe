import { describe, it, expect, vi } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createElement } from 'react';
import { http, HttpResponse } from 'msw';
import { server } from '@/test/msw/server';
import { makeCashSession } from '@/test/factories';

vi.mock('sonner', () => ({ toast: { error: vi.fn(), success: vi.fn() } }));

/**
 * Same shape as `useCreateBooking.idempotency.test.ts` — goes through the
 * real `cashApi` and ky client, MSW standing in for the backend, proving the
 * `Idempotency-Key` header reaches the wire and is reused on a retried submit.
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
    http.post('*/complexes/:complexId/cash-sessions', ({ request }) => {
      keys.push(request.headers.get('Idempotency-Key') ?? '');
      if (keys.length <= failures) {
        return HttpResponse.json({ title: 'upstream unavailable' }, { status: 503 });
      }
      return HttpResponse.json({ cash_session: makeCashSession() }, { status: 201 });
    }),
  );
  return keys;
}

describe('useOpenCashSession — Idempotency-Key', () => {
  it('sends a UUID key with the request', async () => {
    const keys = captureKeys();
    const { useOpenCashSession } = await import('./useOpenCashSession');
    const { result } = renderHook(() => useOpenCashSession('c1'), { wrapper: createWrapper() });

    result.current.mutate({ opening_cash: 50000 });
    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(keys).toHaveLength(1);
    expect(keys[0]).toMatch(/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/);
  });

  it('reuses the same key when the attempt is retried, so the server replays instead of opening two sessions', async () => {
    const keys = captureKeys(1);
    const { useOpenCashSession } = await import('./useOpenCashSession');
    const { result } = renderHook(() => useOpenCashSession('c1'), { wrapper: createWrapper() });

    result.current.mutate({ opening_cash: 50000 });
    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(keys).toHaveLength(2);
    expect(keys[1]).toBe(keys[0]);
  });

  it('never puts attemptKey in the request body', async () => {
    let body: Record<string, unknown> = {};
    server.use(
      http.post('*/complexes/:complexId/cash-sessions', async ({ request }) => {
        body = (await request.json()) as Record<string, unknown>;
        return HttpResponse.json({ cash_session: makeCashSession() }, { status: 201 });
      }),
    );

    const { useOpenCashSession } = await import('./useOpenCashSession');
    const { result } = renderHook(() => useOpenCashSession('c1'), { wrapper: createWrapper() });

    result.current.mutate({ opening_cash: 50000 });
    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(body).toEqual({ opening_cash: 50000 });
    expect(body).not.toHaveProperty('attemptKey');
  });
});
