/**
 * The /baja page's logic: where the request goes, which token it reads, and what each answer
 * means to the person. Run with `pnpm test:unit`.
 */
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { outcomeForStatus, tokenFromSearch, unsubscribe, unsubscribeUrl } from '../src/lib/unsubscribe.ts';

const API = 'https://api.vibe.com.ar/api/v1';
const ENDPOINT = 'https://api.vibe.com.ar/api/v1/public/email/unsubscribe';

test('the request goes to the public endpoint under the API base, with or without a trailing slash', () => {
  assert.equal(unsubscribeUrl(API), ENDPOINT);
  assert.equal(unsubscribeUrl(`${API}/`), ENDPOINT);
});

test('the token is the token query parameter, trimmed; a missing or blank one is null', () => {
  assert.equal(tokenFromSearch('?token=abc.def'), 'abc.def');
  assert.equal(tokenFromSearch('?token=%20abc%20'), 'abc');
  assert.equal(tokenFromSearch(''), null);
  assert.equal(tokenFromSearch('?token='), null);
  assert.equal(tokenFromSearch('?other=1'), null);
});

test('a 204 is done, a 400 is an invalid link, and any other answer asks to try again', () => {
  assert.equal(outcomeForStatus(204), 'done');
  assert.equal(outcomeForStatus(400), 'invalid');
  assert.equal(outcomeForStatus(500), 'retry');
  assert.equal(outcomeForStatus(429), 'retry');
});

test('the request is a POST of the token as JSON to the endpoint', async () => {
  const calls: { url: string; init: RequestInit }[] = [];
  const fetchImpl = (async (url: string | URL | Request, init?: RequestInit) => {
    calls.push({ url: String(url), init: init ?? {} });
    return new Response(null, { status: 204 });
  }) as typeof fetch;

  const outcome = await unsubscribe({ baseUrl: API, token: 'tok-1', fetchImpl });

  assert.equal(outcome, 'done');
  assert.equal(calls.length, 1);
  assert.equal(calls[0]?.url, ENDPOINT);
  assert.equal(calls[0]?.init.method, 'POST');
  assert.equal(calls[0]?.init.body, JSON.stringify({ token: 'tok-1' }));
});

test('a missing token is an invalid link, and the endpoint is never called', async () => {
  let called = false;
  const fetchImpl = (async () => {
    called = true;
    return new Response(null, { status: 204 });
  }) as typeof fetch;

  assert.equal(await unsubscribe({ baseUrl: API, token: null, fetchImpl }), 'invalid');
  assert.equal(called, false);
});

test('a network failure asks the person to try again', async () => {
  const fetchImpl = (async () => {
    throw new TypeError('fetch failed');
  }) as typeof fetch;

  assert.equal(await unsubscribe({ baseUrl: API, token: 'tok-1', fetchImpl }), 'retry');
});
