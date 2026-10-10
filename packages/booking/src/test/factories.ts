import { HTTPError } from 'ky';
import type { NormalizedOptions } from 'ky';
import type { Court } from '../api/types';

const defaultCourt: Court = {
  id: 'ct1',
  complex_id: 'c1',
  name: 'Cancha 1',
  sport: 'padel',
  court_type: 'outdoor',
  is_active: true,
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
};

/** Builds a typed `Court` fixture with sensible defaults, overridable per-field. */
export function makeCourt(overrides: Partial<Court> = {}): Court {
  return { ...defaultCourt, ...overrides };
}

/**
 * Builds a ky `HTTPError` whose response body is already consumed, the way ky
 * hands it to `beforeError`: `HTTPError.data` is populated before the error is
 * thrown, so `error.response.json()` can no longer be read.
 */
export async function makeConsumedHttpError(status: number, data: unknown): Promise<HTTPError> {
  const bodyText = data === undefined ? '' : JSON.stringify(data);
  const response = new Response(bodyText, { status });
  await response.text();
  const request = new Request('https://example.com/test');
  const error = new HTTPError(response, request, {} as NormalizedOptions);
  error.data = data;
  return error;
}
