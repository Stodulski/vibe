import { z } from 'zod';
import type { PaginatedResponse, ErrorResponse } from '@/shared/types/api.types';
import { exact } from '@/shared/lib/apiParse';

// ─── Envelope pattern (matches backend) ───

/**
 * The `metadata` object every cursor-paginated list response repeats
 * (bookings, clients, admin users, admin complexes). Kept as one schema so
 * the four list endpoints validate the same shape instead of four
 * hand-copied near-duplicates drifting apart.
 */
export const paginationMetadataSchema = exact<PaginatedResponse<unknown>['metadata']>(
  z
    .object({
      next_cursor: z.string().optional(),
      has_more: z.boolean(),
      total_count: z.number().optional(),
    })
    .loose(),
);

/** `PaginatedResponse<T>`'s generic shell. Callers supply `itemSchema` for `T`. */
export function paginatedResponseSchema<T extends z.ZodType>(itemSchema: T) {
  return z
    .object({
      data: z.array(itemSchema),
      metadata: paginationMetadataSchema,
    })
    .loose() satisfies z.ZodType<PaginatedResponse<z.infer<T>>>;
}

/**
 * The `{ message: string }` shape returned by most mutation endpoints
 * (register, verify-email, toggle-active, delete, …). Endpoints that also
 * return extra fields alongside `message` (e.g. `complexApi.delete`'s
 * `courts_deactivated`) declare their own schema instead of reusing this one.
 */
export const messageResponseSchema = z
  .object({
    message: z.string(),
  })
  .loose();

/**
 * `error` is either a bare human-readable string (the document's `Error`) or
 * a field-name-to-message map (its `ValidationError`, what a 422 answers
 * with). It used to be typed as a bare string or `{ message, details? }` —
 * an object shape the API has never sent; deriving `ErrorResponse` from
 * `openapi.yaml` is what surfaced it.
 *
 * Not consumed through any `.json<T>()` call today — `getHttpErrorMessage`
 * (`src/shared/lib/utils.ts`) reads `HTTPError.data` as `unknown` and
 * narrows by hand — but kept here so a future typed read of an error body
 * has a validated schema to reach for instead of another handwritten guard.
 */
export const errorResponseSchema = exact<ErrorResponse>(
  z.union([
    z.object({ error: z.string() }).loose(),
    z.object({ error: z.record(z.string(), z.string()) }).loose(),
  ]),
);
