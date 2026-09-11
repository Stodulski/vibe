import { z } from 'zod';
import api, { withSignal } from '@/shared/lib/ky';
import { parseWith } from '@/shared/lib/apiParse';
import type { PlaceDetails, Prediction } from './types';

// `ky`'s default retry (limit 2, GET included) retries 429 and 502 — the two
// statuses `internal/places/upstream.go` now maps rate-limit and upstream
// failures onto instead of a blanket 500. Retrying a 429 is the opposite of
// what it asks for, and it triples load on an address lookup that is already
// rate-limited or degraded. Scoped to these two calls rather than the shared
// `ky.ts` client: other GET traffic (dashboard stats, bookings, etc.) may
// still benefit from retrying a transient 502/503, and nothing in this
// finding shows otherwise — narrowing the whole client's retry policy would
// be a bigger change than this finding justifies.
const NO_RETRY = { limit: 0 };

// Google Places autocomplete/details, not the vibe backend — these shapes
// have no home in `src/shared/types/api.types`, so the schema lives next to
// the local `Prediction`/`PlaceDetails` types it validates.
const predictionSchema = z
  .object({
    place_id: z.string(),
    description: z.string(),
    structured_formatting: z
      .object({
        main_text: z.string(),
        secondary_text: z.string(),
      })
      .loose(),
  })
  .loose() satisfies z.ZodType<Prediction>;

const predictionsResponseSchema = z
  .object({
    predictions: z.array(predictionSchema),
  })
  .loose() satisfies z.ZodType<{ predictions: Prediction[] }>;

const placeDetailsSchema = z
  .object({
    address: z.string(),
    city: z.string(),
    province: z.string(),
    formatted_address: z.string(),
    latitude: z.string(),
    longitude: z.string(),
  })
  .loose() satisfies z.ZodType<PlaceDetails>;

export async function fetchAutocompletePredictions(
  query: string,
  sessionToken: string,
  signal?: AbortSignal,
): Promise<Prediction[]> {
  const data = await api
    .get('places/autocomplete', {
      searchParams: { input: query, session_token: sessionToken },
      retry: NO_RETRY,
      ...withSignal(signal),
    })
    .json()
    .then(parseWith(predictionsResponseSchema, 'addressApi.fetchAutocompletePredictions'));
  return data.predictions;
}

export function fetchPlaceDetails(placeId: string, sessionToken: string, signal?: AbortSignal): Promise<PlaceDetails> {
  return api
    .get('places/details', {
      searchParams: { place_id: placeId, session_token: sessionToken },
      retry: NO_RETRY,
      ...withSignal(signal),
    })
    .json()
    .then(parseWith(placeDetailsSchema, 'addressApi.fetchPlaceDetails'));
}
