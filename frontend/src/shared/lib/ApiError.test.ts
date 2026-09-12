// @vitest-environment node
import { HTTPError } from 'ky';
import type { NormalizedOptions } from 'ky';
import { ApiError, getProblem, normalizeProblem } from './ApiError';

/**
 * The backend answers errors two ways and is mid-migration between them:
 * `{"error": ...}` today, RFC 9457 problem+json next. Both have to be read by
 * the same client build, because the frontend and the backend deploy
 * independently — a client that only understood one of them would show a
 * generic "algo salió mal" for a whole deploy window in one direction and
 * break the other way round.
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

describe('normalizeProblem — the legacy `{"error": ...}` envelope', () => {
  it('reads a bare string error as the title', () => {
    expect(normalizeProblem({ error: 'booking_not_found' }, 404)).toMatchObject({
      type: 'about:blank',
      title: 'booking_not_found',
      status: 404,
      errors: [],
    });
  });

  it('translates a known server code into its Spanish copy', () => {
    expect(normalizeProblem({ error: { slug: 'slug_taken' } }, 422).errors).toEqual([
      { field: 'slug', message: 'Ya existe un complejo con esa URL, elegí otra' },
    ]);
  });

  it('reads a 422 field map as one error per field', () => {
    expect(normalizeProblem({ error: { first_name: 'required', phone: 'invalid' } }, 422).errors).toEqual([
      { field: 'first_name', message: 'required' },
      { field: 'phone', message: 'invalid' },
    ]);
  });

  it('reads a lone `message` key as a sentence, never as a field called "message"', () => {
    const problem = normalizeProblem({ error: { message: 'algo salió mal' } }, 500);
    expect(problem.title).toBe('algo salió mal');
    expect(problem.errors).toEqual([]);
  });

  it('answers an empty problem for a body it cannot read at all', () => {
    expect(normalizeProblem('<html>502 Bad Gateway</html>', 502)).toEqual({
      type: 'about:blank',
      title: '',
      status: 502,
      detail: undefined,
      instance: undefined,
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
      title: 'Slot taken',
      status: 409,
      detail: 'Ese horario ya fue reservado',
      instance: '/api/v1/book',
      errors: [],
    });
  });

  it("prefers the body's own `status` over the response status", () => {
    expect(normalizeProblem({ title: 'Unprocessable', status: 422 }, 400).status).toBe(422);
  });

  it('reads `errors[]` entries addressed by `field`', () => {
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

  it('translates server codes inside `errors[]` the same way as the legacy shape', () => {
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

  it('leaves requestId undefined when the header is absent', () => {
    expect(new ApiError(httpError(500, { error: 'internal' })).requestId).toBeUndefined();
  });

  it('normalizes whichever envelope the response carried', () => {
    expect(new ApiError(httpError(422, { error: { slug: 'slug_taken' } })).problem.errors).toEqual([
      { field: 'slug', message: 'Ya existe un complejo con esa URL, elegí otra' },
    ]);
    expect(
      new ApiError(httpError(422, { title: 'Validation failed', errors: [{ field: 'slug', detail: 'slug_taken' }] }))
        .problem.errors,
    ).toEqual([{ field: 'slug', message: 'Ya existe un complejo con esa URL, elegí otra' }]);
  });
});

describe('getProblem', () => {
  it('answers the problem for an ApiError', () => {
    expect(getProblem(new ApiError(httpError(404, { error: 'not_found' })))?.title).toBe('not_found');
  });

  it('answers undefined for anything else — a bare HTTPError, a timeout, a string', () => {
    expect(getProblem(httpError(404, { error: 'not_found' }))).toBeUndefined();
    expect(getProblem(new Error('boom'))).toBeUndefined();
    expect(getProblem('boom')).toBeUndefined();
  });
});
