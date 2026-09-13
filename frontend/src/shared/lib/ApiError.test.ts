import { describe, it, expect } from 'vitest';
// @vitest-environment node
import { HTTPError } from 'ky';
import type { NormalizedOptions } from 'ky';
import { ApiError, getProblem, normalizeProblem } from './ApiError';

/**
 * The backend answers every 4xx/5xx as RFC 9457 problem+json
 * (`type`/`title`/`status`/`detail`/`instance`/`errors[]`).
 */

function httpError(status: number, body: unknown, headers: Record<string, string> = {}): HTTPError {
  const response = new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json', ...headers },
  });
  const request = new Request('https://api.vibe.com.ar/api/v1/book', { method: 'POST' });
  const error = new HTTPError(response, request, {} as NormalizedOptions);
  // ky populates `data` on the error after constructing it and before the
  // `beforeError` hooks run; this reproduces that ordering.
  error.data = body;
  return error;
}

/**
 * The exact 422 the backend will send once its problem+json PR lands, copied
 * verbatim from the contract rather than paraphrased: the field names in
 * `errors[]` (`field`/`message`, not `detail`) and the body-level
 * `request_id` are the two places where a plausible-looking guess would parse
 * to an empty problem and lose the message the person needs to read.
 */
const VALIDATION_PROBLEM = {
  type: 'https://vibe.com.ar/problems/validation',
  title: 'Validation Failed',
  status: 422,
  detail: 'the request failed validation',
  instance: '/api/v1/auth/register',
  request_id: '8f14e45f-ceea-467a-9c1d-3f3b4b5a6c7d',
  errors: [
    { field: 'email', message: 'must be provided' },
    { field: 'password', message: 'must be provided' },
  ],
};

describe("normalizeProblem — the API's problem+json contract", () => {
  it('reads the validation 422 exactly as the backend sends it', () => {
    expect(normalizeProblem(VALIDATION_PROBLEM, 422)).toEqual({
      type: 'https://vibe.com.ar/problems/validation',
      kind: 'validation',
      title: 'Validation Failed',
      status: 422,
      detail: 'the request failed validation',
      instance: '/api/v1/auth/register',
      requestId: '8f14e45f-ceea-467a-9c1d-3f3b4b5a6c7d',
      errors: [
        { field: 'email', message: 'must be provided' },
        { field: 'password', message: 'must be provided' },
      ],
    });
  });

  // Every 4xx/5xx carries type/title/status/detail/instance/request_id; only
  // validation carries `errors[]`.
  it.each([
    'invalid-json',
    'not-found',
    'route-not-found',
    'conflict',
    'unauthorized',
    'forbidden',
    'rate-limited',
    'too-large',
    'unavailable',
    'gone',
    'internal',
    'method-not-allowed',
  ])('exposes %s as a bare kind, so callers never compare URIs', (kind) => {
    const problem = normalizeProblem(
      {
        type: `https://vibe.com.ar/problems/${kind}`,
        title: 'Something',
        status: 409,
        detail: 'a detail',
        instance: '/api/v1/book',
        request_id: 'req-1',
      },
      409,
    );
    expect(problem.kind).toBe(kind);
    expect(problem.errors).toEqual([]);
  });

  it('leaves kind undefined for a type from anywhere else', () => {
    expect(
      normalizeProblem({ type: 'https://example.test/problems/validation', title: 'x' }, 400).kind,
    ).toBeUndefined();
    expect(normalizeProblem({ error: 'slot_taken' }, 409).kind).toBeUndefined();
  });
});

describe('normalizeProblem — a body that is not problem+json', () => {
  it('answers an empty problem for a body carrying none of type/title/errors', () => {
    expect(normalizeProblem({ error: 'booking_not_found' }, 404)).toEqual({
      type: 'about:blank',
      kind: undefined,
      title: '',
      status: 404,
      detail: undefined,
      instance: undefined,
      requestId: undefined,
      errors: [],
    });
  });

  it('answers an empty problem for a body it cannot read at all', () => {
    expect(normalizeProblem('<html>502 Bad Gateway</html>', 502)).toEqual({
      type: 'about:blank',
      kind: undefined,
      title: '',
      status: 502,
      detail: undefined,
      instance: undefined,
      requestId: undefined,
      errors: [],
    });
  });
});

describe('normalizeProblem — RFC 9457 problem+json', () => {
  it('reads type, title, status, detail and instance', () => {
    const problem = normalizeProblem(
      {
        type: 'https://vibe.com.ar/problems/slot-taken',
        title: 'Slot taken',
        status: 409,
        detail: 'Ese horario ya fue reservado',
        instance: '/api/v1/book',
      },
      409,
    );
    expect(problem).toEqual({
      type: 'https://vibe.com.ar/problems/slot-taken',
      kind: 'slot-taken',
      title: 'Slot taken',
      status: 409,
      detail: 'Ese horario ya fue reservado',
      instance: '/api/v1/book',
      requestId: undefined,
      errors: [],
    });
  });

  it("prefers the body's own `status` over the response status", () => {
    expect(normalizeProblem({ title: 'Unprocessable', status: 422 }, 400).status).toBe(422);
  });

  it('reads `errors[]` entries addressed by `field`, whether the text is in `message` or `detail`', () => {
    expect(
      normalizeProblem({ title: 'Validation failed', errors: [{ field: 'phone', message: 'invalid' }] }, 422).errors,
    ).toEqual([{ field: 'phone', message: 'invalid' }]);
    expect(
      normalizeProblem({ title: 'Validation failed', errors: [{ field: 'phone', detail: 'invalid' }] }, 422).errors,
    ).toEqual([{ field: 'phone', message: 'invalid' }]);
  });

  it('reads `errors[]` entries addressed by JSON `pointer`', () => {
    expect(
      normalizeProblem(
        { title: 'Validation failed', errors: [{ pointer: '#/body/first_name', detail: 'required' }] },
        422,
      ).errors,
    ).toEqual([{ field: 'first_name', message: 'required' }]);
  });

  it('translates a known server code inside `errors[]`', () => {
    expect(
      normalizeProblem({ title: 'Validation failed', errors: [{ field: 'slug', detail: 'slug_taken' }] }, 422).errors,
    ).toEqual([{ field: 'slug', message: 'Ya existe un complejo con esa URL, elegí otra' }]);
  });

  it('skips entries that name no field or carry no message', () => {
    const problem = normalizeProblem(
      { title: 'Validation failed', errors: [{ detail: 'orphan' }, { field: 'phone' }, 'nonsense'] },
      422,
    );
    expect(problem.errors).toEqual([]);
  });
});

// Most refusals carry their code as `detail` (see `internal/httpx/refusals.go`'s
// `detailOf`), not inside `errors[]` — a validation problem is the one
// exception. Both must localize the same known codes.
describe('normalizeProblem — translates a known server code carried as `detail`', () => {
  it('translates it', () => {
    expect(normalizeProblem({ title: 'Conflict', detail: 'slug_taken' }, 409).detail).toBe(
      'Ya existe un complejo con esa URL, elegí otra',
    );
  });

  it('leaves an unrecognized one unchanged', () => {
    expect(normalizeProblem({ title: 'Conflict', detail: 'Ese horario ya fue reservado' }, 409).detail).toBe(
      'Ese horario ya fue reservado',
    );
  });
});

describe('ApiError', () => {
  it('keeps working for every existing `instanceof HTTPError` call site', () => {
    const error = new ApiError(httpError(409, { error: 'slot_taken' }));
    expect(error).toBeInstanceOf(HTTPError);
    expect(error.response.status).toBe(409);
    expect(error.data).toEqual({ error: 'slot_taken' });
  });

  it('carries the backend request id when CORS exposed the header', () => {
    const error = new ApiError(httpError(500, { error: 'internal' }, { 'X-Request-ID': 'req-abc123' }));
    expect(error.requestId).toBe('req-abc123');
  });

  it('falls back to the body request_id when CORS did not expose the header', () => {
    const error = new ApiError(httpError(422, VALIDATION_PROBLEM));
    expect(error.requestId).toBe('8f14e45f-ceea-467a-9c1d-3f3b4b5a6c7d');
  });

  it('prefers the header over the body when both are there', () => {
    const error = new ApiError(httpError(422, VALIDATION_PROBLEM, { 'X-Request-ID': 'from-header' }));
    expect(error.requestId).toBe('from-header');
  });

  it('leaves requestId undefined when neither carries one', () => {
    expect(new ApiError(httpError(500, { error: 'internal' })).requestId).toBeUndefined();
  });

  it('normalizes the problem+json body the response carried', () => {
    expect(
      new ApiError(httpError(422, { title: 'Validation failed', errors: [{ field: 'slug', detail: 'slug_taken' }] }))
        .problem.errors,
    ).toEqual([{ field: 'slug', message: 'Ya existe un complejo con esa URL, elegí otra' }]);
  });
});

describe('getProblem', () => {
  it('answers the problem for an ApiError', () => {
    expect(getProblem(new ApiError(httpError(404, { title: 'not_found' })))?.title).toBe('not_found');
  });

  // The calls that bypass the shared client (`auth/me`, `auth/refresh`) still
  // throw a bare HTTPError, and so does any caller that built its own —
  // reading those too is what keeps every pre-existing `HTTPError` path
  // working after the `beforeError` hook was added.
  it('normalizes a bare HTTPError on the spot', () => {
    expect(getProblem(httpError(404, { title: 'not_found' }))?.title).toBe('not_found');
  });

  it('answers undefined for a failure that never carried a response', () => {
    expect(getProblem(new Error('boom'))).toBeUndefined();
    expect(getProblem('boom')).toBeUndefined();
  });
});
