import { describe, expect, it } from 'vitest';
import { HTTPError } from 'ky';
import type { NormalizedOptions } from 'ky';
import { ES_AR } from '@/shared/i18n/es_AR';
import { getFieldErrors, translateServerError } from '@/shared/lib/serverErrors';

/** ky consumes the body before throwing, so the payload lives on `.data`. */
function httpError(status: number, body: unknown): HTTPError {
  const response = new Response(JSON.stringify(body), { status });
  const request = new Request('https://api.vibe.com.ar/api/v1/test', { method: 'POST' });
  const error = new HTTPError(response, request, {} as NormalizedOptions);
  error.data = body;
  return error;
}

describe('translateServerError', () => {
  it('turns a known code into its Spanish text', () => {
    expect(translateServerError('slug_taken')).toBe(ES_AR.validation.server.slugTaken);
  });

  // The backend still returns prose for messages that have not been converted
  // to codes. Showing an imperfect one beats replacing it with a generic that
  // says less.
  it('passes an unknown value through unchanged', () => {
    expect(translateServerError('cannot delete court while it has active bookings')).toBe(
      'cannot delete court while it has active bookings',
    );
  });
});

describe('getFieldErrors', () => {
  it('reads the field map a failed validation returns', () => {
    const fields = getFieldErrors(
      httpError(422, { title: 'Validation failed', errors: [{ field: 'slug', message: 'slug_taken' }] }),
    );

    expect(fields).toEqual({ slug: ES_AR.validation.server.slugTaken });
  });

  it('translates every field it finds', () => {
    const fields = getFieldErrors(
      httpError(422, {
        title: 'Validation failed',
        errors: [
          { field: 'deposit_amount', message: 'deposit_exceeds_price' },
          { field: 'slug', message: 'slug_taken' },
        ],
      }),
    );

    expect(fields.deposit_amount).toBe(ES_AR.validation.server.depositExceedsPrice);
    expect(fields.slug).toBe(ES_AR.validation.server.slugTaken);
  });

  it.each([
    ['a failure with no fields at all', httpError(500, { title: 'Server error' })],
    ['a non-HTTP failure', new Error('boom')],
    ['no error at all', undefined],
  ])('returns nothing for %s', (_label, error) => {
    expect(getFieldErrors(error)).toEqual({});
  });
});
