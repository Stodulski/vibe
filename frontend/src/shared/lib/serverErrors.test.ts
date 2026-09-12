import { describe, expect, it } from 'vitest';
import { ES_AR } from '@/shared/i18n/es_AR';
import { getFieldErrors, translateServerError } from '@/shared/lib/serverErrors';
import { getApiError } from '@/shared/lib/utils';

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
    const fields = getFieldErrors({ error: { slug: 'slug_taken' } });

    expect(fields).toEqual({ slug: ES_AR.validation.server.slugTaken });
  });

  it('translates every field it finds', () => {
    const fields = getFieldErrors({
      error: { deposit_amount: 'deposit_exceeds_price', slug: 'slug_taken' },
    });

    expect(fields.deposit_amount).toBe(ES_AR.validation.server.depositExceedsPrice);
    expect(fields.slug).toBe(ES_AR.validation.server.slugTaken);
  });

  it.each([
    ['a plain string error', { error: 'something went wrong' }],
    ['no error at all', { message: 'ok' }],
    ['a non-object body', 'not json'],
    ['null', null],
  ])('returns nothing for %s', (_label, body) => {
    expect(getFieldErrors(body)).toEqual({});
  });
});

describe('getApiError', () => {
  const fallback = 'Algo salió mal';

  // This is the case that was silently broken: the backend answered
  // {"error":{"slug":"slug_taken"}}, getApiError found no `.message`, and the
  // user saw the generic fallback instead of the one thing only the server
  // could tell them.
  it('surfaces a server-side field error', () => {
    expect(getApiError({ error: { slug: 'slug_taken' } }, fallback)).toBe(ES_AR.validation.server.slugTaken);
  });

  it('joins several field errors', () => {
    const message = getApiError({ error: { slug: 'slug_taken', deposit_amount: 'deposit_exceeds_price' } }, fallback);

    expect(message).toContain(ES_AR.validation.server.slugTaken);
    expect(message).toContain(ES_AR.validation.server.depositExceedsPrice);
  });

  it('translates a code returned as a plain string', () => {
    expect(getApiError({ error: 'month_out_of_range' }, fallback)).toBe(ES_AR.validation.server.monthOutOfRange);
  });

  it('still returns a plain prose error unchanged', () => {
    expect(getApiError({ error: 'the requested resource could not be found' }, fallback)).toBe(
      'the requested resource could not be found',
    );
  });

  it('falls back when the body carries nothing usable', () => {
    expect(getApiError({}, fallback)).toBe(fallback);
    expect(getApiError(null, fallback)).toBe(fallback);
    expect(getApiError({ error: {} }, fallback)).toBe(fallback);
  });
});
