import { HTTPError } from 'ky';

/** One field-level validation failure, keyed by the form field's own name. */
export interface ProblemFieldError {
  field: string;
  message: string;
}

/**
 * The backend's error body, normalized from RFC 9457 problem+json. A body that
 * is not problem+json still normalizes into this, with an empty title and no
 * errors, so a reader never has to check which shape arrived.
 */
export interface Problem {
  /** RFC 9457 `type`, or `about:blank` for a body that is not problem+json. */
  type: string;
  /** The short name under the API's problem prefix (`validation`, `slot-unavailable`), or `undefined`. */
  kind: string | undefined;
  /** Short summary. Empty when the body carried none. */
  title: string;
  status: number;
  detail: string | undefined;
  instance: string | undefined;
  /** The backend's `request_id` from the body, when it carried one. */
  requestId: string | undefined;
  errors: ProblemFieldError[];
}

/**
 * The problem prefix of the API. Kept as a constant so the package can read
 * the short kind without importing the app's error module.
 */
const PROBLEM_TYPE_PREFIX = 'https://vibe.com.ar/problems/';

function problemKind(type: string): string | undefined {
  return type.startsWith(PROBLEM_TYPE_PREFIX) ? type.slice(PROBLEM_TYPE_PREFIX.length) : undefined;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null;
}

function asString(value: unknown): string | undefined {
  return typeof value === 'string' ? value : undefined;
}

/** The field a problem+json error entry is about: `field`, else the last segment of its JSON `pointer`. */
function entryField(entry: Record<string, unknown>): string | undefined {
  const field = asString(entry.field);
  if (field) return field;

  const pointer = asString(entry.pointer);
  if (!pointer) return undefined;
  return pointer.split('/').filter(Boolean).at(-1);
}

function problemErrors(raw: unknown): ProblemFieldError[] {
  if (!Array.isArray(raw)) return [];

  const errors: ProblemFieldError[] = [];
  for (const entry of raw as unknown[]) {
    if (!isRecord(entry)) continue;
    const field = entryField(entry);
    const message = asString(entry.message) ?? asString(entry.detail) ?? asString(entry.title);
    if (field && message) errors.push({ field, message });
  }
  return errors;
}

function emptyProblem(status: number): Problem {
  return {
    type: 'about:blank',
    kind: undefined,
    title: '',
    status,
    detail: undefined,
    instance: undefined,
    requestId: undefined,
    errors: [],
  };
}

/** Reads a problem+json body into a {@link Problem}. Anything else yields the empty problem. */
export function normalizeProblem(body: unknown, status: number): Problem {
  if (!isRecord(body)) return emptyProblem(status);

  const isProblemJson = typeof body.type === 'string' || typeof body.title === 'string' || Array.isArray(body.errors);
  if (!isProblemJson) return emptyProblem(status);

  const type = asString(body.type) ?? 'about:blank';
  return {
    type,
    kind: problemKind(type),
    title: asString(body.title) ?? '',
    status: typeof body.status === 'number' ? body.status : status,
    detail: asString(body.detail),
    instance: asString(body.instance),
    requestId: asString(body.request_id),
    errors: problemErrors(body.errors),
  };
}

/**
 * ky's `HTTPError` with the backend's body already read. It extends
 * `HTTPError`, so `instanceof HTTPError` keeps working for every caller.
 */
export class ApiError extends HTTPError {
  readonly problem: Problem;
  /** The backend's `X-Request-ID`, falling back to the body's `request_id`. */
  readonly requestId: string | undefined;

  constructor(source: HTTPError) {
    super(source.response, source.request, source.options);
    // ky populates `data` before `beforeError` runs and the body stream is
    // spent by then, so the payload can only be carried over.
    this.data = source.data;
    if (source.stack !== undefined) this.stack = source.stack;
    this.problem = normalizeProblem(source.data, source.response.status);
    this.requestId = source.response.headers.get('X-Request-ID') ?? this.problem.requestId;
  }
}

/** The {@link Problem} behind a failed request, or `undefined` when there is none. */
export function getProblem(error: unknown): Problem | undefined {
  if (error instanceof ApiError) return error.problem;
  if (error instanceof HTTPError) return normalizeProblem(error.data, error.response.status);
  return undefined;
}
