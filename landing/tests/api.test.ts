/**
 * Unit tests for the public API client: URL, status mapping, timeout and payload
 * validation. Run with `pnpm test:unit`.
 */
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { ApiError, createPublicApi } from '../src/lib/api.ts';

const baseUrl = 'https://api.test/api/v1/';

const payload = {
  hub: { slug: 'banfield', name: 'Banfield', complex_count: 1 },
  complexes: [{ slug: 'club-sur', name: 'Club Sur', address: 'Av. X 1', city: 'Banfield', sports: ['padel'] }],
};

type FakeFetch = typeof fetch & { calls: string[] };

function fakeFetch(respond: () => Response | Promise<Response>): FakeFetch {
  const calls: string[] = [];
  const impl = async (input: string | URL | Request) => {
    calls.push(String(input));
    return respond();
  };
  return Object.assign(impl, { calls }) as unknown as FakeFetch;
}

function expectApiError(kind: ApiError['kind'], status: ApiError['status']) {
  return (error: unknown) => {
    assert.ok(error instanceof ApiError, `expected ApiError, got ${String(error)}`);
    assert.equal(error.kind, kind);
    assert.equal(error.status, status);
    return true;
  };
}

test('requests the hub under the base URL with the city encoded', async () => {
  const fetchImpl = fakeFetch(() => Response.json(payload));
  const api = createPublicApi({ baseUrl, fetchImpl });

  const hub = await api.hubData('Vicente López');

  assert.equal(fetchImpl.calls[0], 'https://api.test/api/v1/public/hubs/Vicente%20L%C3%B3pez/data');
  assert.equal(hub.hub.slug, 'banfield');
  assert.equal(hub.complexes[0].slug, 'club-sur');
});

test('an unknown city is a not_found error', async () => {
  const api = createPublicApi({ baseUrl, fetchImpl: fakeFetch(() => new Response('{}', { status: 404 })) });

  await assert.rejects(api.hubData('no-existe'), expectApiError('not_found', 404));
});

test('a server error is unavailable with its status', async () => {
  const api = createPublicApi({ baseUrl, fetchImpl: fakeFetch(() => new Response('', { status: 503 })) });

  await assert.rejects(api.hubData('banfield'), expectApiError('unavailable', 503));
});

test('a network failure is unavailable and carries no status', async () => {
  const api = createPublicApi({
    baseUrl,
    fetchImpl: fakeFetch(() => {
      throw new TypeError('fetch failed');
    }),
  });

  await assert.rejects(api.hubData('banfield'), expectApiError('unavailable', null));
});

test('a body that is not JSON is unavailable', async () => {
  const api = createPublicApi({ baseUrl, fetchImpl: fakeFetch(() => new Response('<html>', { status: 200 })) });

  await assert.rejects(api.hubData('banfield'), expectApiError('unavailable', 200));
});

test('a payload without the hub fields is unavailable, not rendered', async () => {
  const api = createPublicApi({
    baseUrl,
    fetchImpl: fakeFetch(() => Response.json({ complexes: [] })),
  });

  await assert.rejects(api.hubData('banfield'), expectApiError('unavailable', 200));
});

test('a request that outlives the timeout is unavailable', async () => {
  const fetchImpl = ((_input: string | URL | Request, init?: RequestInit) =>
    new Promise<Response>((_resolve, reject) => {
      init?.signal?.addEventListener('abort', () => reject(init.signal?.reason));
    })) as typeof fetch;
  const api = createPublicApi({ baseUrl, fetchImpl, timeoutMs: 10 });

  await assert.rejects(api.hubData('banfield'), expectApiError('unavailable', null));
});
