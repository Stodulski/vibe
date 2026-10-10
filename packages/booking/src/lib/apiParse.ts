import { z } from 'zod';

/**
 * Zod types every `.optional()` field as `field?: X | undefined`. Under
 * `exactOptionalPropertyTypes` that is not the document's `field?: X`, so
 * {@link Loose} widens a type's optional fields the way Zod infers them. A
 * schema declared with {@link exact} is then checked against the narrow type
 * the callers read. Copied from the app's `shared/lib/apiParse.ts`.
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
 * Declares a schema as producing exactly `T`. The parameter still forces the
 * structural check against `Loose<T>`, so a wrong field type or a missing
 * required field fails to compile. The runtime is the identity.
 */
export function exact<T>(schema: z.ZodType<Loose<T>>): z.ZodType<T>;
export function exact(schema: z.ZodType): z.ZodType {
  return schema;
}

export interface FlattenedApiIssues {
  formErrors: string[];
  fieldErrors: Record<string, string[] | undefined>;
}

/**
 * Thrown when a response body fails its schema. Not an `HTTPError`: the
 * request succeeded, the shape of the body is what is wrong.
 */
export class ApiResponseError extends Error {
  readonly context: string;
  readonly issues: FlattenedApiIssues;

  constructor(context: string, error: z.ZodError) {
    super(`Invalid API response shape in ${context}: ${String(error.issues.length)} issue(s)`);
    this.name = 'ApiResponseError';
    this.context = context;
    this.issues = z.flattenError(error);
  }
}

/**
 * Where a schema mismatch goes. The app passes its own reporter; the package
 * itself reports nothing. Optional, so a host that does not report still gets
 * the thrown error.
 */
export type ErrorReporter = (error: unknown, context?: Record<string, unknown>) => void;

/**
 * Validates `data` against `schema`. Returns the parsed value, or throws
 * {@link ApiResponseError} after handing it to `report`. `context` names the
 * call site, the only thing that says which endpoint sent the unexpected shape.
 */
export function parseResponse<T>(schema: z.ZodType<T>, data: unknown, context: string, report?: ErrorReporter): T {
  const result = schema.safeParse(data);
  if (result.success) return result.data;

  const error = new ApiResponseError(context, result.error);
  report?.(error, { context });
  throw error;
}

/** Curried form of {@link parseResponse} for `.then()` chaining. */
export function parseWith<T>(schema: z.ZodType<T>, context: string, report?: ErrorReporter): (data: unknown) => T {
  return (data: unknown) => parseResponse(schema, data, context, report);
}
