import { z } from 'zod';
import * as Sentry from '@sentry/react';

/**
 * Zod types every `.optional()` field as `field?: X | undefined`, never as
 * `field?: X`. Under `exactOptionalPropertyTypes` those are different types:
 * a target declared `field?: X` (as `src/shared/types/api.types` does
 * throughout) refuses a value that is present-but-`undefined`, which is
 * exactly what Zod's inferred type allows for every optional field — even
 * though Zod's object parser never actually writes `undefined` back onto a
 * key that was absent from the input (see `ZodObject`'s parser); the parsed
 * value's *runtime* shape already matches the handwritten type exactly. This
 * widens a type's optional fields (recursively) to mirror what Zod infers,
 * so a schema's declared output can be checked against it structurally.
 */
type IsOptionalKey<T, K extends keyof T> = { [P in K]?: T[P] } extends Pick<T, K> ? true : false;

export type Loose<T> = T extends (infer U)[]
  ? Loose<U>[]
  : T extends object
    ? { [K in keyof T as IsOptionalKey<T, K> extends true ? never : K]: Loose<T[K]> } & {
        [K in keyof T as IsOptionalKey<T, K> extends true ? K : never]?: Loose<T[K]> | undefined;
      }
    : T;

/**
 * Declares a schema as producing exactly `T` (not `Loose<T>`) for every
 * caller downstream of {@link parseWith} — `src/shared/types/api.types` is
 * this codebase's source of truth for optionality, so nothing outside this
 * file should have to know about the `Loose<T>` friction above. The
 * parameter type still forces the structural check against `Loose<T>`, so a
 * genuine mismatch (a wrong field type, a missing required field) still
 * fails to compile; only the optional-field friction above is bridged.
 *
 * Expressed as an overload over an identity implementation rather than as a
 * `schema as unknown as z.ZodType<T>` body: the runtime *is* the identity —
 * nothing is converted — and stating that as the implementation signature
 * keeps the only visible contract the narrow one callers are checked
 * against, instead of a double cast TypeScript can no longer police.
 */
export function exact<T>(schema: z.ZodType<Loose<T>>): z.ZodType<T>;
export function exact(schema: z.ZodType): z.ZodType {
  return schema;
}

/**
 * Thrown by {@link parseResponse} when a response body fails its schema.
 *
 * Deliberately not an `HTTPError` (ky's own error class): the request
 * succeeded — the server answered 2xx — the *shape* of the body is what's
 * wrong. `getHttpErrorMessage` (`src/shared/lib/utils.ts`) only recognizes
 * `HTTPError` and falls back to its caller-supplied `fallback` message for
 * anything else, so a mutation's `onError` still shows a sensible toast
 * instead of crashing; it just can't surface the specific schema mismatch
 * the way it surfaces a server-sent field error. Widening
 * `getHttpErrorMessage` to special-case `ApiResponseError` would need a
 * change to `src/shared/lib/utils.ts`, which is outside this change's
 * allowed paths — flagged for whichever wave wires responses through
 * `parseResponse` to decide if that's worth doing.
 */
export interface FlattenedApiIssues {
  formErrors: string[];
  fieldErrors: Record<string, string[] | undefined>;
}

export class ApiResponseError extends Error {
  readonly context: string;
  readonly issues: FlattenedApiIssues;

  constructor(context: string, error: z.ZodError) {
    super(`Invalid API response shape in ${context}: ${String(error.issues.length)} issue(s)`);
    this.name = 'ApiResponseError';
    this.context = context;
    // `z.flattenError`'s return type keys `fieldErrors` off the schema's
    // inferred `T` (here left generic-less, `unknown`), which collapses it
    // to `{}` — the object it returns at runtime is unaffected, and `{}` is
    // already structurally assignable to `FlattenedApiIssues`, so no cast
    // is needed for the declared field type above.
    this.issues = z.flattenError(error);
  }
}

function reportApiResponseError(error: ApiResponseError): void {
  // Sentry is initialized only when `VITE_SENTRY_DSN` is set
  // (`src/shared/lib/sentry.ts`); calling `captureException` without `init`
  // having run is a documented no-op, so this needs no separate guard.
  Sentry.captureException(error);
  if (import.meta.env.DEV) {
    console.error(error.message, error.issues);
  }
}

/**
 * Validates `data` against `schema`. Returns the parsed, typed value on
 * success; throws {@link ApiResponseError} (and reports it) on failure.
 *
 * `context` should identify the call site (e.g. `'bookingsApi.list'`) — it's
 * the only thing that tells you which endpoint sent a shape the client
 * didn't expect.
 */
export function parseResponse<T>(schema: z.ZodType<T>, data: unknown, context: string): T {
  const result = schema.safeParse(data);
  if (result.success) return result.data;

  const error = new ApiResponseError(context, result.error);
  reportApiResponseError(error);
  throw error;
}

/**
 * Curried form of {@link parseResponse} for `.then()` chaining, e.g.
 * `api.get(...).json().then(parseWith(bookingSchema, 'bookingsApi.getById'))`.
 */
export function parseWith<T>(schema: z.ZodType<T>, context: string): (data: unknown) => T {
  return (data: unknown) => parseResponse(schema, data, context);
}
