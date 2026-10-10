// @vitest-environment node
import { describe, it, expect } from 'vitest';
import { http, HttpResponse } from 'msw';
import { server } from '@/test/msw/server';
import { ApiError } from '@/shared/lib/ApiError';
import { createBookingClient, BOOKING_REQUEST_TIMEOUT_MS } from './client';

const BASE = 'http://localhost/api/v1';

async function settle<T>(promise: Promise<T>): Promise<unknown> {
  try {
    await promise;
    return undefined;
  } catch (error: unknown) {
    return error;
  }
}

describe('createBookingClient', () => {
  it('resolves paths against the configured API base', async () => {
    let url = '';
    server.use(
      http.get(`${BASE}/ping`, ({ request }) => {
        url = request.url;
        return HttpResponse.json({});
      }),
    );

    await createBookingClient(BASE).get('ping').json();

    expect(url).toBe(`${BASE}/ping`);
  });

  it('sends its requests with credentials omitted', async () => {
    let credentials: RequestCredentials | undefined;
    server.use(
      http.get(`${BASE}/ping`, ({ request }) => {
        credentials = request.credentials;
        return HttpResponse.json({});
      }),
    );

    await createBookingClient(BASE).get('ping').json();

    expect(credentials).toBe('omit');
  });

  it('never sends a CSRF header, because the public endpoints do not expect one', async () => {
    let csrfHeader: string | null | undefined;
    server.use(
      http.get(`${BASE}/ping`, ({ request }) => {
        csrfHeader = request.headers.get('X-CSRF-Token');
        return HttpResponse.json({});
      }),
    );

    await createBookingClient(BASE).get('ping').json();

    expect(csrfHeader).toBeNull();
  });
});

describe('createBookingClient errors', () => {
  it('answers a 401 as an ApiError without trying to refresh the session', async () => {
    const refreshCalls: string[] = [];
    server.use(
      http.get(`${BASE}/private`, () => HttpResponse.json({ title: 'Unauthorized' }, { status: 401 })),
      http.post(`${BASE}/auth/refresh`, () => {
        refreshCalls.push('refresh');
        return HttpResponse.json({ csrf_token: 'fresh' });
      }),
    );

    const error = await settle(createBookingClient(BASE).get('private').json());

    expect(error).toBeInstanceOf(ApiError);
    expect((error as ApiError).response.status).toBe(401);
    expect(refreshCalls).toHaveLength(0);
  });

  it('maps a problem+json body onto ApiError.problem', async () => {
    server.use(
      http.post(`${BASE}/book`, () =>
        HttpResponse.json(
          {
            type: 'https://vibe.com.ar/problems/validation',
            title: 'Datos inválidos',
            status: 422,
            detail: 'phone',
          },
          { status: 422, headers: { 'Content-Type': 'application/problem+json' } },
        ),
      ),
    );

    const error = await settle(createBookingClient(BASE).post('book', { json: {} }).json());

    expect(error).toBeInstanceOf(ApiError);
    expect((error as ApiError).problem).toMatchObject({
      kind: 'validation',
      title: 'Datos inválidos',
      status: 422,
      detail: 'phone',
    });
  });
});

describe('createBookingClient retries', () => {
  it('retries an idempotent GET on 503 and gives up after two retries', async () => {
    let calls = 0;
    server.use(
      http.get(`${BASE}/flaky`, () => {
        calls += 1;
        return HttpResponse.json({}, { status: 503 });
      }),
    );

    const error = await settle(createBookingClient(BASE).get('flaky').json());

    expect(error).toBeInstanceOf(ApiError);
    expect(calls).toBe(3);
  });

  it('does not retry a POST, which is what the Idempotency-Key header is for', async () => {
    let calls = 0;
    server.use(
      http.post(`${BASE}/book`, () => {
        calls += 1;
        return HttpResponse.json({}, { status: 503 });
      }),
    );

    const error = await settle(createBookingClient(BASE).post('book', { json: {} }).json());

    expect(error).toBeInstanceOf(ApiError);
    expect(calls).toBe(1);
  });

  it('gives a request ten seconds before timing out', () => {
    expect(BOOKING_REQUEST_TIMEOUT_MS).toBe(10_000);
  });
});
