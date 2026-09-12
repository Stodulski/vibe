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
  /** Short, human-readable summary. Empty when the body carried none. */
  title: string;
  status: number;
  detail: string | undefined;
  instance: string | undefined;
  errors: ProblemFieldError[];
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
    const message = asString(entry.detail) ?? asString(entry.message) ?? asString(entry.title);
    if (field && message) errors.push({ field, message: translateServerError(message) });
  }
  return errors;
}

/** `{"error": {...}}` — the shape the API answers with today. */
function legacyProblem(error: unknown, status: number): Problem {
  const base = { type: 'about:blank', status, instance: undefined };

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
    return { type: 'about:blank', title: '', status, detail: undefined, instance: undefined, errors: [] };
  }

  const isProblemJson = typeof body.type === 'string' || typeof body.title === 'string' || Array.isArray(body.errors);
  if (!isProblemJson) return legacyProblem(body.error, status);

  return {
    type: asString(body.type) ?? 'about:blank',
    title: translateServerError(asString(body.title) ?? ''),
    status: typeof body.status === 'number' ? body.status : status,
    detail: asString(body.detail),
    instance: asString(body.instance),
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
    this.requestId = source.response.headers.get('X-Request-ID') ?? undefined;
    this.problem = normalizeProblem(source.data, source.response.status);
  }
}

/** The `Problem` behind a failed request, or `undefined` if it isn't an `ApiError`. */
export function getProblem(error: unknown): Problem | undefined {
  return error instanceof ApiError ? error.problem : undefined;
}
