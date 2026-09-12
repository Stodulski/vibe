import { describe, it, expect, vi } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createElement } from 'react';
import { http, HttpResponse } from 'msw';
import { server } from '@/test/msw/server';
import { makeBooking } from '@/test/factories';
import type { CreateBookingRequest } from '@/shared/types/api.types';

vi.mock('sonner', () => ({ toast: { error: vi.fn(), success: vi.fn() } }));

/**
 * Goes through the real `bookingsApi` and the real ky client, with MSW
 * standing in for the backend — the assertion is that the header reaches the
 * wire, which a mocked api module could only ever confirm as an argument.
 *
 * One retry, not the app's zero: the key's whole job is to be the same across
 * the attempts of one submit, and a client that never retries cannot show
 * that. ky's own `retry` never fires here — it is restricted to idempotent
 * methods, so a failed POST is React Query's to repeat.
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

const BOOKING: CreateBookingRequest = {
  court_id: 'ct1',
  date: '2026-03-18',
  start_time: '10:00',
  duration_minutes: 90,
  client_phone: '1155550000',
  client_first_name: 'Juan',
  client_last_name: 'Perez',
};

/**
 * Records the `Idempotency-Key` of every `POST /complexes/:id/bookings` that
 * reaches the server, and fails the first `failures` of them so a retry can
 * be observed.
 */
function captureKeys(failures = 0): string[] {
  const keys: string[] = [];
  server.use(
    http.post('*/complexes/:complexId/bookings', ({ request }) => {
      keys.push(request.headers.get('Idempotency-Key') ?? '');
      if (keys.length <= failures) {
        return HttpResponse.json({ error: 'upstream unavailable' }, { status: 503 });
      }
      return HttpResponse.json({ booking: makeBooking() });
    }),
  );
  return keys;
}

describe('useCreateBooking — Idempotency-Key', () => {
  it('sends a UUID key with the request', async () => {
    const keys = captureKeys();
    const { useCreateBooking } = await import('./useCreateBooking');
    const { result } = renderHook(() => useCreateBooking('c1'), { wrapper: createWrapper() });

    result.current.mutate(BOOKING);
    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(keys).toHaveLength(1);
    expect(keys[0]).toMatch(/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/);
  });

  it('reuses the same key when the attempt is retried, so the server replays instead of double-booking', async () => {
    const keys = captureKeys(1);
    const { useCreateBooking } = await import('./useCreateBooking');
    const { result } = renderHook(() => useCreateBooking('c1'), { wrapper: createWrapper() });

    result.current.mutate(BOOKING);
    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(keys).toHaveLength(2);
    expect(keys[1]).toBe(keys[0]);
  });
});

// A sibling describe, not nested in the one above: max-lines-per-function
// counts a describe callback's whole body, so this stays a genuinely
// separate top-level call.
describe('useCreateBooking — Idempotency-Key under concurrency', () => {
  // The regression this guards: the key used to live in a `useRef` on the
  // hook, so a second `mutate()` starting while the first was still in flight
  // overwrote it — both bodies went out under the second call's key and the
  // backend answered the second one 409 (same key, different body). In the
  // public booking flow that 409 reads as "ese turno ya fue reservado", a
  // conflict that never happened. `onMutate` awaits `cancelQueries`, so the
  // window is wide open in exactly the hooks that carry a key.
  it('gives two overlapping submits different keys, each paired with its own body', async () => {
    const seen: { key: string; startTime: string }[] = [];
    let release: (() => void) | undefined;
    const gate = new Promise<void>((resolve) => {
      release = resolve;
    });
    server.use(
      http.post('*/complexes/:complexId/bookings', async ({ request }) => {
        const body = (await request.json()) as { start_time: string };
        seen.push({ key: request.headers.get('Idempotency-Key') ?? '', startTime: body.start_time });
        await gate;
        return HttpResponse.json({ booking: makeBooking() });
      }),
    );

    const { useCreateBooking } = await import('./useCreateBooking');
    const { result } = renderHook(() => useCreateBooking('c1'), { wrapper: createWrapper() });

    result.current.mutate(BOOKING);
    result.current.mutate({ ...BOOKING, start_time: '11:00' });
    await waitFor(() => {
      expect(seen).toHaveLength(2);
    });
    release?.();

    expect(seen[0]?.startTime).toBe('10:00');
    expect(seen[1]?.startTime).toBe('11:00');
    expect(seen[1]?.key).not.toBe(seen[0]?.key);
  });

  it('never puts attemptKey in the request body, which the API validates against its spec', async () => {
    let body: Record<string, unknown> = {};
    server.use(
      http.post('*/complexes/:complexId/bookings', async ({ request }) => {
        body = (await request.json()) as Record<string, unknown>;
        return HttpResponse.json({ booking: makeBooking() });
      }),
    );

    const { useCreateBooking } = await import('./useCreateBooking');
    const { result } = renderHook(() => useCreateBooking('c1'), { wrapper: createWrapper() });

    result.current.mutate(BOOKING);
    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(body).toEqual(BOOKING);
    expect(body).not.toHaveProperty('attemptKey');
  });

  it('mints a new key for a genuinely new submit, so a second booking is not refused as a replay', async () => {
    const keys = captureKeys();
    const { useCreateBooking } = await import('./useCreateBooking');
    const { result } = renderHook(() => useCreateBooking('c1'), { wrapper: createWrapper() });

    result.current.mutate(BOOKING);
    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });
    result.current.mutate({ ...BOOKING, start_time: '11:00' });
    await waitFor(() => {
      expect(keys).toHaveLength(2);
    });

    expect(keys[1]).not.toBe(keys[0]);
  });
});
