import { describe, it, expect } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import type { ReactNode } from 'react';
import { useIdempotentMutation, type WithAttemptKey } from './idempotency';

/** A query client with retries off, so a failing mutation settles at once. */
function createQueryWrapper() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
  return function QueryWrapper({ children }: { children: ReactNode }) {
    return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
  };
}

const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;

/** An error shaped like ky's `HTTPError`: the server answered with `status`. */
function httpError(status: number): Error {
  return Object.assign(new Error(`HTTP ${String(status)}`), { response: new Response(null, { status }) });
}

/** An error shaped like ky's `HTTPError` for a problem response: `data` holds the parsed body, as ky populates it. */
function problemError(status: number, problem: { type: string; detail: string }): Error {
  return Object.assign(new Error(`HTTP ${String(status)}`), {
    response: new Response(null, { status }),
    data: { type: problem.type, status, detail: problem.detail },
  });
}

/** The backend's refusal for a repeated key whose first request is still running (backend/internal/middleware/idempotency.go). */
function inProgressError(): Error {
  return problemError(409, {
    type: 'https://vibe.com.ar/problems/conflict',
    detail: 'a request with this Idempotency-Key is still in progress, retry shortly',
  });
}

/** An error shaped like ky's `TimeoutError`: no HTTP answer came back. */
function timeoutError(): Error {
  return Object.assign(new Error('Request timed out'), { name: 'TimeoutError' });
}

/**
 * Renders the hook with a `mutationFn` that records every attempt's variables.
 * `failures` lists what the first attempts reject with, in order; attempts past
 * the list succeed.
 */
function renderRetryable(failures: Error[]) {
  const queue = [...failures];
  const seen: WithAttemptKey<{ id: string }>[] = [];
  const hook = renderHook(
    () =>
      useIdempotentMutation({
        mutationFn: (variables: WithAttemptKey<{ id: string }>) => {
          seen.push(variables);
          const failure = queue.shift();
          return failure ? Promise.reject(failure) : Promise.resolve('ok');
        },
      }),
    { wrapper: createQueryWrapper() },
  );
  return { ...hook, seen };
}

/**
 * The key has to stay the same while one submission is retried and change for
 * a new one — the backend replays the stored answer for a repeated key and
 * refuses a repeated key carrying a different body. A retry is a new `mutate()`
 * call, so the hook keeps the key of the pending submission and reuses it while
 * the payload is unchanged and the server gave no HTTP answer. A success, a
 * changed payload, or an HTTP error starts a new one.
 */
describe('useIdempotentMutation', () => {
  it('adds a UUID attemptKey to the variables the mutationFn receives', async () => {
    const seen: WithAttemptKey<{ id: string }>[] = [];
    const { result } = renderHook(
      () =>
        useIdempotentMutation({
          mutationFn: (variables: WithAttemptKey<{ id: string }>) => {
            seen.push(variables);
            return Promise.resolve('ok');
          },
        }),
      { wrapper: createQueryWrapper() },
    );

    // Called with the bare variables: `attemptKey` is the hook's business,
    // not the call site's, so no consumer had to change.
    result.current.mutate({ id: 'a' });
    await waitFor(() => {
      expect(seen).toHaveLength(1);
    });

    expect(seen[0]?.id).toBe('a');
    expect(seen[0]?.attemptKey).toMatch(UUID);
  });

  it('gives two overlapping calls different keys', async () => {
    const seen: WithAttemptKey<{ id: string }>[] = [];
    let release: (() => void) | undefined;
    const gate = new Promise<void>((resolve) => {
      release = resolve;
    });

    const { result } = renderHook(
      () =>
        useIdempotentMutation({
          mutationFn: async (variables: WithAttemptKey<{ id: string }>) => {
            seen.push(variables);
            await gate;
            return 'ok';
          },
        }),
      { wrapper: createQueryWrapper() },
    );

    result.current.mutate({ id: 'a' });
    result.current.mutate({ id: 'b' });
    await waitFor(() => {
      expect(seen).toHaveLength(2);
    });
    release?.();

    expect(seen[0]?.id).toBe('a');
    expect(seen[1]?.id).toBe('b');
    expect(seen[1]?.attemptKey).not.toBe(seen[0]?.attemptKey);
  });
});

// Sibling describes, not nested in the one above: max-lines-per-function.
describe('useIdempotentMutation key reuse without an HTTP answer', () => {
  it('reuses the key after a timeout/network error', async () => {
    const { result, seen } = renderRetryable([timeoutError()]);

    result.current.mutate({ id: 'a' });
    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    result.current.mutate({ id: 'a' });
    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(seen).toHaveLength(2);
    expect(seen[1]?.attemptKey).toBe(seen[0]?.attemptKey);
  });

  it('reuses the key after a network error', async () => {
    const { result, seen } = renderRetryable([new TypeError('Failed to fetch')]);

    result.current.mutate({ id: 'a' });
    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    result.current.mutate({ id: 'a' });
    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(seen).toHaveLength(2);
    expect(seen[1]?.attemptKey).toBe(seen[0]?.attemptKey);
  });

  it('keeps the key while the first request under it is still in progress', async () => {
    const { result, seen } = renderRetryable([inProgressError()]);

    result.current.mutate({ id: 'a' });
    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    result.current.mutate({ id: 'a' });
    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(seen).toHaveLength(2);
    expect(seen[1]?.attemptKey).toBe(seen[0]?.attemptKey);
  });
});

describe('useIdempotentMutation key renewal after an HTTP error', () => {
  it('mints a new key after the server answered with an error', async () => {
    const { result, seen } = renderRetryable([httpError(409)]);

    result.current.mutate({ id: 'a' });
    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    result.current.mutate({ id: 'a' });
    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(seen).toHaveLength(2);
    expect(seen[1]?.attemptKey).not.toBe(seen[0]?.attemptKey);
  });

  it('mints a new key after the server answered with a 5xx', async () => {
    const { result, seen } = renderRetryable([httpError(500)]);

    result.current.mutate({ id: 'a' });
    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    result.current.mutate({ id: 'a' });
    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(seen).toHaveLength(2);
    expect(seen[1]?.attemptKey).not.toBe(seen[0]?.attemptKey);
  });

  it('mints a new key after a generic conflict that is not the in-progress refusal', async () => {
    const { result, seen } = renderRetryable([
      problemError(409, { type: 'https://vibe.com.ar/problems/conflict', detail: 'stale version' }),
    ]);

    result.current.mutate({ id: 'a' });
    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    result.current.mutate({ id: 'a' });
    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(seen).toHaveLength(2);
    expect(seen[1]?.attemptKey).not.toBe(seen[0]?.attemptKey);
  });
});

describe('useIdempotentMutation key renewal after success or a changed payload', () => {
  it('mints a new key for the same payload once the previous submission succeeded', async () => {
    const { result, seen } = renderRetryable([]);

    result.current.mutate({ id: 'a' });
    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    result.current.mutate({ id: 'a' });
    await waitFor(() => {
      expect(seen).toHaveLength(2);
    });

    expect(seen[1]?.attemptKey).not.toBe(seen[0]?.attemptKey);
  });

  it('mints a new key when the payload changes after a failure', async () => {
    const { result, seen } = renderRetryable([timeoutError()]);

    result.current.mutate({ id: 'a' });
    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    result.current.mutate({ id: 'b' });
    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(seen).toHaveLength(2);
    expect(seen[1]?.attemptKey).not.toBe(seen[0]?.attemptKey);
  });
});
