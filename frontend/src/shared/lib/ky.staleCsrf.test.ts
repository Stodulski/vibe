import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import api from './ky';
import { ApiError } from '@/shared/lib/ApiError';
import { useStore } from '@/shared/stores';
import { makeUser } from '@/test/factories';

// Goes through the real client and its hooks, with only `fetch` stubbed: the
// unit tests in `ky.test.ts` call the hook directly and would not notice it
// being dropped from the client.

const json = (status: number, body: unknown) =>
  new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });

const csrfRefused = () =>
  json(403, { type: 'about:blank', title: 'Forbidden', status: 403, detail: 'invalid or missing CSRF token' });

describe('api — a write sent with the CSRF token of a session another tab renewed', () => {
  const fetchMock = vi.fn<(request: Request) => Promise<Response>>();
  const sent: { url: string; method: string; csrf: string | null; body: string }[] = [];

  beforeEach(() => {
    sent.length = 0;
    fetchMock.mockReset();
    vi.stubGlobal('fetch', async (input: Request) => {
      const request = input.clone();
      sent.push({
        url: request.url,
        method: request.method,
        csrf: request.headers.get('X-CSRF-Token'),
        body: await request.text(),
      });
      return fetchMock(input);
    });
    useStore.getState().setCsrfToken('stale');
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    useStore.getState().setCsrfToken(null);
  });

  it('repeats the write once, body intact, with the token of the current session', async () => {
    fetchMock
      .mockResolvedValueOnce(csrfRefused())
      .mockResolvedValueOnce(json(200, { user: makeUser(), csrf_token: 'current', pending_email: null }))
      .mockResolvedValueOnce(json(201, { booking: { id: 'b1' } }));

    await expect(api.post('complexes/c1/bookings', { json: { court_id: 'k1' } }).json()).resolves.toEqual({
      booking: { id: 'b1' },
    });

    expect(sent.map((r) => [r.method, r.csrf])).toEqual([
      ['POST', 'stale'],
      ['GET', null],
      ['POST', 'current'],
    ]);
    expect(sent[1]?.url).toContain('/auth/me');
    expect(sent[2]?.body).toBe('{"court_id":"k1"}');
    expect(useStore.getState().csrfToken).toBe('current');
  });

  it('answers a refusal the repeated write meets as an ApiError, like any other', async () => {
    fetchMock
      .mockResolvedValueOnce(csrfRefused())
      .mockResolvedValueOnce(json(200, { user: makeUser(), csrf_token: 'current', pending_email: null }))
      .mockResolvedValueOnce(json(409, { type: 'about:blank', title: 'Conflict', status: 409, detail: 'taken' }));

    const error: unknown = await api
      .post('complexes/c1/bookings', { json: { court_id: 'k1' } })
      .json()
      .catch((e: unknown) => e);

    expect(error).toBeInstanceOf(ApiError);
    expect((error as ApiError).problem.status).toBe(409);
    expect(sent).toHaveLength(3);
  });
});
