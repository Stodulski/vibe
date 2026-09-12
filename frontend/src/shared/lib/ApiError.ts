import { HTTPError } from 'ky';
import { translateServerError } from '@/shared/lib/serverErrors';

/**
 * One field-level validation failure, whichever envelope carried it.
 *
 * `field` is the form field's own name: problem+json addresses it with either
 * `field` or a JSON `pointer` (`/body/first_name`), and the legacy envelope
 * uses the key of its `error` object. All three end up here as `first_name`.
 */
export interface ProblemFieldError {
  field: string;
  message: string;
}

/**
 * The backend's error body, normalized to one shape.
 *
 * Today the API answers `{"error": "code"}` or, for a 422,
 * `{"error": {"slug": "slug_taken"}}`. It is moving to RFC 9457 problem+json
 * (`type`/`title`/`status`/`detail`/`instance`/`errors[]`). Both are parsed
 * into this, so the frontend can ship before the backend switches and keep
 * working after — no coordinated deploy, and no reader of an error has to
 * know which envelope arrived.
 */
export interface Problem {
  /** RFC 9457 `type`, or `about:blank` for the legacy envelope. */
  type: string;
  /**
   * The last segment of a `type` under {@link PROBLEM_TYPE_PREFIX} —
   * `validation`, `rate-limited`, `conflict`, and so on.
   *
   * The URI is the stable identifier on the wire, but comparing against it at
   * a call site means pasting a domain into a `switch`, and one rename of the
   * documentation host would then silently stop matching everywhere. Reading
   * the suffix once, here, gives callers a short token and keeps that risk in
   * one file. `undefined` for the legacy envelope and for any `type` from
   * somewhere else, so an unrecognized URI can never be mistaken for a known
   * kind.
   */
  kind: string | undefined;
  /** Short, human-readable summary. Empty when the body carried none. */
  title: string;
  status: number;
  detail: string | undefined;
  instance: string | undefined;
  /** The backend's own `request_id` for this failure, when the body carried one. */
  requestId: string | undefined;
  errors: ProblemFieldError[];
}

/** Every problem type the API mints lives under this prefix. */
export const PROBLEM_TYPE_PREFIX = 'https://vibe.com.ar/problems/';

/** The short name behind a problem `type`, or `undefined` if it isn't one of ours. */
function problemKind(type: string): string | undefined {
  return type.startsWith(PROBLEM_TYPE_PREFIX) ? type.slice(PROBLEM_TYPE_PREFIX.length) : undefined;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null;
}

function asString(value: unknown): string | undefined {
  return typeof value === 'string' ? value : undefined;
}

/**
 * The field name a problem+json error entry is about.
 *
 * `field` wins when present; otherwise the last segment of the JSON `pointer`
 * (`#/body/first_name`, `/first_name`) — the form knows its inputs by name,
 * not by the path the server reached them through.
 */
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
    // `message` is what the API sends (see the validation problem body);
    // `detail` and `title` are read too because RFC 9457 leaves the shape of
    // an `errors` entry to the API, and a later endpoint may pick either.
    const message = asString(entry.message) ?? asString(entry.detail) ?? asString(entry.title);
    if (field && message) errors.push({ field, message: translateServerError(message) });
  }
  return errors;
}

/** `{"error": {...}}` — the shape the API answers with today. */
function legacyProblem(error: unknown, status: number): Problem {
  const base = { type: 'about:blank', kind: undefined, status, instance: undefined, requestId: undefined };

  const message = asString(error);
  if (message !== undefined) {
    return { ...base, title: translateServerError(message), detail: undefined, errors: [] };
  }

  if (isRecord(error)) {
    // `{"error": {"message": "..."}}` is a sentence, not a field map — the
    // one key that must never reach a form as a field named "message".
    const single = asString(error.message);
    if (single !== undefined && Object.keys(error).length === 1) {
      return { ...base, title: translateServerError(single), detail: undefined, errors: [] };
    }

    const errors: ProblemFieldError[] = [];
    for (const [field, value] of Object.entries(error)) {
      const text = asString(value);
      if (text !== undefined) errors.push({ field, message: translateServerError(text) });
    }
    return { ...base, title: '', detail: undefined, errors };
  }

  return { ...base, title: '', detail: undefined, errors: [] };
}

/**
 * Read either envelope into a {@link Problem}.
 *
 * A body that carries `type`, `title` or `errors` is read as problem+json;
 * anything else falls back to the legacy `{"error": ...}` reading, so an
 * unrecognizable body still yields a usable (empty) problem rather than
 * throwing inside an error path.
 */
export function normalizeProblem(body: unknown, status: number): Problem {
  if (!isRecord(body)) {
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

  const isProblemJson = typeof body.type === 'string' || typeof body.title === 'string' || Array.isArray(body.errors);
  if (!isProblemJson) return legacyProblem(body.error, status);

  const type = asString(body.type) ?? 'about:blank';
  return {
    type,
    kind: problemKind(type),
    title: translateServerError(asString(body.title) ?? ''),
    status: typeof body.status === 'number' ? body.status : status,
    detail: asString(body.detail),
    instance: asString(body.instance),
    requestId: asString(body.request_id),
    errors: problemErrors(body.errors),
  };
}

/**
 * ky's `HTTPError` with the backend's error body already read.
 *
 * It extends `HTTPError` rather than replacing it on purpose: about twenty
 * call sites branch on `instanceof HTTPError` and read `error.data` /
 * `error.response.status`, and every one of them keeps working unchanged
 * while new code can reach for `problem` and `requestId` instead of
 * re-parsing the body at each site.
 *
 * Built in `ky.ts`'s `beforeError` hook, which is where ky hands over an
 * error whose `data` it has already populated — the body's stream is spent
 * by then, so this is the only place the payload can still be read.
 */
export class ApiError extends HTTPError {
  /** The error body, normalized across both envelopes. */
  readonly problem: Problem;
  /** The backend's `X-Request-ID` for this response, when CORS exposed it. */
  readonly requestId: string | undefined;

  constructor(source: HTTPError) {
    super(source.response, source.request, source.options);
    // ky populates `data` on the error it built, after construction and
    // before `beforeError` runs; the body is consumed, so it can only be
    // carried over, never re-read.
    this.data = source.data;
    // The stack that matters is where the request was made, not this
    // wrapper's constructor; `stack` is optional on Error, so it is only
    // carried over when the source had one.
    if (source.stack !== undefined) this.stack = source.stack;
    this.problem = normalizeProblem(source.data, source.response.status);
    // The header is the primary source — it is present on every response,
    // including the ones with no body to read. The body's `request_id` is the
    // fallback for when CORS has not exposed the header (a preview
    // deployment, a misconfigured origin): losing the id there would mean
    // losing the only link between a Sentry event and the backend's own log
    // line for the same request.
    this.requestId = source.response.headers.get('X-Request-ID') ?? this.problem.requestId;
  }
}

/**
 * The `Problem` behind a failed request, or `undefined` if there is none.
 *
 * An `ApiError` already carries one. A bare `HTTPError` is normalized on the
 * spot: the calls that bypass the shared client (`auth/me`, `auth/refresh`)
 * still throw one, and so does any caller that built its own — reading them
 * too is what keeps every existing `HTTPError` path working.
 */
export function getProblem(error: unknown): Problem | undefined {
  if (error instanceof ApiError) return error.problem;
  if (error instanceof HTTPError) return normalizeProblem(error.data, error.response.status);
  return undefined;
}
