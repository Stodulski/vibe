// @vitest-environment node
import { HTTPError, NetworkError, TimeoutError } from 'ky';
import type { NormalizedOptions } from 'ky';
import { ApiError } from './ApiError';

const mockCaptureException = vi.fn<(...args: unknown[]) => void>();

vi.mock('@sentry/react', () => ({
  captureException: (...args: unknown[]) => {
    mockCaptureException(...args);
  },
}));

const { queryClient } = await import('./queryClient');

function makeHttpError(status: number, url = 'https://api.vibe.com.ar/v1/bookings', headers?: HeadersInit) {
  const response = new Response(null, { status, ...(headers ? { headers } : {}) });
  const request = new Request(url);
  return new HTTPError(response, request, {} as NormalizedOptions);
}

describe('queryClient', () => {
  it('is an instance of QueryClient', () => {
    expect(queryClient).toBeDefined();
    expect(typeof queryClient.getDefaultOptions).toBe('function');
  });

  it('has correct default staleTime for queries', () => {
    const defaults = queryClient.getDefaultOptions();
    expect(defaults.queries?.staleTime).toBe(5 * 60 * 1000);
  });

  it('has correct default gcTime for queries', () => {
    const defaults = queryClient.getDefaultOptions();
    expect(defaults.queries?.gcTime).toBe(10 * 60 * 1000);
  });

  it('retries a 5xx once and never retries a 4xx', () => {
    const retry = queryClient.getDefaultOptions().queries?.retry;
    if (typeof retry !== 'function') throw new Error('expected a retry predicate');

    expect(retry(0, makeHttpError(500))).toBe(true);
    expect(retry(1, makeHttpError(500))).toBe(false);
    for (const status of [401, 403, 404, 422, 400]) {
      expect(retry(0, makeHttpError(status))).toBe(false);
    }
  });

  it('retries a timeout or a dropped connection, which carry no status at all', () => {
    const retry = queryClient.getDefaultOptions().queries?.retry;
    if (typeof retry !== 'function') throw new Error('expected a retry predicate');

    expect(retry(0, new TimeoutError(new Request('https://api.vibe.com.ar/v1/bookings')))).toBe(true);
    expect(retry(0, new NetworkError(new Request('https://api.vibe.com.ar/v1/bookings')))).toBe(true);
  });

  it('has refetchOnWindowFocus disabled', () => {
    const defaults = queryClient.getDefaultOptions();
    expect(defaults.queries?.refetchOnWindowFocus).toBe(false);
  });

  it('has retry set to 0 for mutations', () => {
    const defaults = queryClient.getDefaultOptions();
    expect(defaults.mutations?.retry).toBe(0);
  });
});

describe('queryClient error reporting', () => {
  beforeEach(() => {
    mockCaptureException.mockClear();
  });

  it('does not report an expected 401/403/404/422 HTTPError', () => {
    for (const status of [401, 403, 404, 422]) {
      queryClient.getQueryCache().config.onError?.(makeHttpError(status), {} as never);
    }
    expect(mockCaptureException).not.toHaveBeenCalled();
  });

  it('reports a 500 HTTPError with status, pathname and request_id tags', () => {
    const error = makeHttpError(500, 'https://api.vibe.com.ar/v1/bookings?date=2026-03-18', {
      'X-Request-Id': 'req-123',
    });
    queryClient.getQueryCache().config.onError?.(error, {} as never);
    expect(mockCaptureException).toHaveBeenCalledWith(error, {
      tags: { status: 500, pathname: '/v1/bookings', request_id: 'req-123' },
    });
  });

  it('reports a 500 HTTPError without a request_id tag when the header is absent', () => {
    const error = makeHttpError(500);
    queryClient.getQueryCache().config.onError?.(error, {} as never);
    expect(mockCaptureException).toHaveBeenCalledWith(error, {
      tags: { status: 500, pathname: '/v1/bookings' },
    });
  });

  // Every failure from the shared ky client is an `ApiError`, which read
  // `X-Request-ID` once when it was built. The tag has to come from there,
  // not from a second read of the headers, or the two could disagree.
  it('reports an ApiError with the request_id it already carries', () => {
    const error = new ApiError(
      makeHttpError(500, 'https://api.vibe.com.ar/v1/bookings', { 'X-Request-ID': 'req-from-api-error' }),
    );
    queryClient.getQueryCache().config.onError?.(error, {} as never);
    expect(mockCaptureException).toHaveBeenCalledWith(error, {
      tags: { status: 500, pathname: '/v1/bookings', request_id: 'req-from-api-error' },
    });
  });

  it('still skips an expected status when it arrives as an ApiError', () => {
    queryClient.getQueryCache().config.onError?.(new ApiError(makeHttpError(422)), {} as never);
    expect(mockCaptureException).not.toHaveBeenCalled();
  });

  it('reports a TimeoutError', () => {
    const error = new TimeoutError(new Request('https://api.vibe.com.ar/v1/bookings'));
    queryClient.getMutationCache().config.onError?.(error, {}, {}, {} as never, {} as never);
    expect(mockCaptureException).toHaveBeenCalledWith(error);
  });

  it('reports a NetworkError', () => {
    const error = new NetworkError(new Request('https://api.vibe.com.ar/v1/bookings'));
    queryClient.getMutationCache().config.onError?.(error, {}, {}, {} as never, {} as never);
    expect(mockCaptureException).toHaveBeenCalledWith(error);
  });

  it('does not report a schema mismatch or other plain error', () => {
    queryClient.getQueryCache().config.onError?.(new Error('Invalid API response shape'), {} as never);
    expect(mockCaptureException).not.toHaveBeenCalled();
  });
});
