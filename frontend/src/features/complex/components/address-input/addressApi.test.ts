// @vitest-environment node
const { mockGet } = vi.hoisted(() => ({
  // Two endpoints share one mocked `get`, so the fixture must branch on the
  // path to answer each with a shape its own schema actually accepts —
  // `places/details` returns a `PlaceDetails` object, never `{ predictions }`.
  mockGet: vi.fn((path: string) => {
    if (path === 'places/details') {
      return {
        json: vi.fn().mockResolvedValue({
          address: 'Av Corrientes 1234',
          city: 'CABA',
          province: 'Buenos Aires',
          formatted_address: 'Av Corrientes 1234, CABA, Argentina',
          latitude: '-34.6037',
          longitude: '-58.3816',
        }),
      };
    }
    return { json: vi.fn().mockResolvedValue({ predictions: [] }) };
  }),
}));

vi.mock('@/shared/lib/ky', () => ({
  default: { get: mockGet },
  withSignal: (signal?: AbortSignal) => (signal ? { signal } : {}),
}));

import { fetchAutocompletePredictions, fetchPlaceDetails } from './addressApi';
import { ApiResponseError } from '@/shared/lib/apiParse';

// ky's default retry (limit 2, GET included, statusCodes incl. 429/502)
// retries a rate-limited or degraded upstream instead of failing once —
// the opposite of what a 429 asks for. internal/places/upstream.go now maps
// upstream failures onto exactly those two statuses instead of a blanket
// 500, so these calls must opt out explicitly.
describe('address autocomplete API — opts out of ky default retry', () => {
  beforeEach(() => vi.clearAllMocks());

  it('fetchAutocompletePredictions requests no retries', async () => {
    await fetchAutocompletePredictions('Av Corrientes', 'tok1');
    expect(mockGet).toHaveBeenCalledWith(
      'places/autocomplete',
      expect.objectContaining({
        retry: { limit: 0 },
      }),
    );
  });

  it('fetchPlaceDetails requests no retries', async () => {
    await fetchPlaceDetails('place1', 'tok1');
    expect(mockGet).toHaveBeenCalledWith(
      'places/details',
      expect.objectContaining({
        retry: { limit: 0 },
      }),
    );
  });
});

describe('address autocomplete API — validates response shape', () => {
  beforeEach(() => vi.clearAllMocks());

  it('fetchPlaceDetails rejects with ApiResponseError when latitude is missing', async () => {
    mockGet.mockReturnValueOnce({
      json: vi.fn().mockResolvedValue({
        address: 'Av Corrientes 1234',
        city: 'CABA',
        province: 'Buenos Aires',
        formatted_address: 'Av Corrientes 1234, CABA',
      }),
    });
    await expect(fetchPlaceDetails('place1', 'tok1')).rejects.toBeInstanceOf(ApiResponseError);
  });
});
