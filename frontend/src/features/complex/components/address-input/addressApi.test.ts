import { describe, it, expect } from 'vitest';
// @vitest-environment node
import { http, HttpResponse } from 'msw';
import { server } from '@/test/msw/server';
import { fetchAutocompletePredictions, fetchPlaceDetails } from './addressApi';
import { ApiResponseError } from '@/shared/lib/apiParse';

// ky's default retry (limit 2, GET included, statusCodes incl. 429/502)
// retries a rate-limited or degraded upstream instead of failing once —
// the opposite of what a 429 asks for. internal/places/upstream.go now maps
// upstream failures onto exactly those two statuses instead of a blanket
// 500, so these calls must opt out explicitly. Proven here by observing the
// actual network behavior (one attempt, not three) rather than by asserting
// on the `retry` option ky was called with — this is what changed migrating
// off the `vi.mock('@/shared/lib/ky')` stub to MSW: there is no mocked call
// signature to inspect any more, only the requests that actually reach the
// handler.
describe('address autocomplete API — opts out of ky default retry', () => {
  it('fetchAutocompletePredictions makes exactly one attempt against a retryable 429', async () => {
    let calls = 0;
    server.use(
      http.get('*/places/autocomplete', () => {
        calls += 1;
        return HttpResponse.json({ title: 'rate limited' }, { status: 429 });
      }),
    );

    await expect(fetchAutocompletePredictions('Av Corrientes', 'tok1')).rejects.toThrow();
    expect(calls).toBe(1);
  });

  it('fetchPlaceDetails makes exactly one attempt against a retryable 502', async () => {
    let calls = 0;
    server.use(
      http.get('*/places/details', () => {
        calls += 1;
        return HttpResponse.json({ title: 'bad gateway' }, { status: 502 });
      }),
    );

    await expect(fetchPlaceDetails('place1', 'tok1')).rejects.toThrow();
    expect(calls).toBe(1);
  });
});

describe('address autocomplete API — validates response shape', () => {
  it('fetchPlaceDetails rejects with ApiResponseError when latitude is missing', async () => {
    server.use(
      http.get('*/places/details', () =>
        HttpResponse.json({
          address: 'Av Corrientes 1234',
          city: 'CABA',
          province: 'Buenos Aires',
          formatted_address: 'Av Corrientes 1234, CABA',
        }),
      ),
    );
    await expect(fetchPlaceDetails('place1', 'tok1')).rejects.toBeInstanceOf(ApiResponseError);
  });
});
