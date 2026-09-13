import { HTTPError } from 'ky';
import { translateServerError } from '@/shared/lib/serverErrors';

/**
 * One field-level validation failure.
 *
 * `field` is the form field's own name: problem+json addresses it with either
 * `field` or a JSON `pointer` (`#/body/first_name`), and both end up here as
 * `first_name`.
 */
interface ProblemFieldError {
  field: string;
  message: string;
}

/**
 * The backend's error body, normalized to one shape.
 *
 * The API answers RFC 9457 problem+json
 * (`type`/`title`/`status`/`detail`/`instance`/`errors[]`) for every 4xx/5xx.
 * A body that isn't shaped like problem+json (a non-JSON body, or one from
 * somewhere else entirely) still normalizes into this — with an empty title
 * and no errors — so a reader never has to check which shape arrived.
 */
export interface Problem {
  /** RFC 9457 `type`, or `about:blank` for a body that isn't problem+json. */
  type: string;
  /**
   * The last segment of a `type` under {@link PROBLEM_TYPE_PREFIX} —
   * `validation`, `rate-limited`, `conflict`, and so on.
   *
   * The URI is the stable identifier on the wire, but comparing against it at
   * a call site means pasting a domain into a `switch`, and one rename of the
   * documentation host would then silently stop matching everywhere. Reading
   * the suffix once, here, gives callers a short token and keeps that risk in
   * one file. `undefined` for a body that isn't problem+json and for any
   * `type` from somewhere else, so an unrecognized URI can never be mistaken
   * for a known kind.
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
const PROBLEM_TYPE_PREFIX = 'https://vibe.com.ar/problems/';

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

/** The empty {@link Problem} for a body that carries nothing readable. */
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

/**
 * Read a problem+json body into a {@link Problem}.
 *
 * A body that carries none of `type`, `title` or `errors` isn't problem+json
 * at all (a non-JSON response, a body from somewhere else entirely) and
 * normalizes to the empty problem, rather than throwing inside an error path.
 *
 * `title` and every `errors[]` message run through {@link translateServerError}
 * — the backend sends a stable code there (`slug_taken`, `required`) rather
 * than prose, same as `detail`, which carries a code just as often (a
 * refusal's `Detail` — see `internal/httpx/refusals.go`'s `detailOf`, which
 * every non-validation refusal writes its message through).
 */
export function normalizeProblem(body: unknown, status: number): Problem {
  if (!isRecord(body)) return emptyProblem(status);

  const isProblemJson = typeof body.type === 'string' || typeof body.title === 'string' || Array.isArray(body.errors);
  if (!isProblemJson) return emptyProblem(status);

  const type = asString(body.type) ?? 'about:blank';
  const detail = asString(body.detail);
  return {
    type,
    kind: problemKind(type),
    title: translateServerError(asString(body.title) ?? ''),
    status: typeof body.status === 'number' ? body.status : status,
    detail: detail === undefined ? undefined : translateServerError(detail),
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
  /** The error body, normalized from problem+json. */
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

/**
 * The kind a stale optimistic-concurrency write is refused with — a PUT whose
 * `version` (or `If-Match`) named a row that has since moved. Distinct from
 * the generic `conflict` kind (e.g. a duplicate slug): only this one means
 * "reload and try again," so callers must not fold it into a generic 409
 * handler.
 */
const STALE_VERSION_KIND = 'stale-version';

/** Whether `error` is a 409 refused specifically for a stale `version`. */
export function isVersionConflict(error: unknown): boolean {
  return getProblem(error)?.kind === STALE_VERSION_KIND;
}
