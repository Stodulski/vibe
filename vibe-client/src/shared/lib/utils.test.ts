// @vitest-environment node
import { z } from 'zod';
import {
  cn,
  formatPrice,
  formatDate,
  formatDateShort,
  formatTime,
  formatDeadline,
  formatHourRange,
  formatInstantTime,
  endsOnALaterDay,
  getApiError,
  getHttpErrorMessage,
  getHttpStatus,
} from './utils';
import { makeConsumedHttpError } from '@/test/factories';
import { ApiResponseError } from '@/shared/lib/apiParse';
import { ES_AR } from '@/shared/i18n/es_AR';

vi.mock('@sentry/react', () => ({
  captureException: vi.fn(),
}));

/** A real `ApiResponseError`, built from an actual failed zod parse. */
function makeApiResponseError(context = 'test.context'): ApiResponseError {
  const result = z.object({ name: z.string() }).safeParse({});
  if (result.success) throw new Error('expected the parse to fail');
  return new ApiResponseError(context, result.error);
}

describe('cn', () => {
  it('merges class names', () => {
    expect(cn('foo', 'bar')).toBe('foo bar');
  });

  it('handles conditional classes', () => {
    const isHidden = () => false;
    expect(cn('base', isHidden() && 'hidden', 'visible')).toBe('base visible');
  });

  it('merges tailwind conflicts correctly', () => {
    expect(cn('p-4', 'p-8')).toBe('p-8');
  });
});

describe('formatPrice', () => {
  it('formats centavos to ARS currency', () => {
    const result = formatPrice(500000);
    // Should contain $ and 5.000 or 5000 depending on locale
    expect(result).toContain('$');
    expect(result).toContain('5');
  });

  it('formats zero', () => {
    const result = formatPrice(0);
    expect(result).toContain('$');
    expect(result).toContain('0');
  });

  it('formats large amounts', () => {
    const result = formatPrice(1500000);
    expect(result).toContain('$');
    expect(result).toContain('15');
  });
});

describe('formatDate', () => {
  it('formats Date object to long format in Spanish', () => {
    const result = formatDate(new Date(2024, 2, 15));
    expect(result).toContain('15');
    expect(result).toContain('de');
    expect(result).toMatch(/marzo/i);
  });

  it('formats another Date object', () => {
    const result = formatDate(new Date(2024, 0, 1));
    expect(result).toContain('01');
    expect(result).toMatch(/enero/i);
  });
});

describe('formatDateShort', () => {
  it('formats to dd/MM/yyyy', () => {
    const result = formatDateShort(new Date(2024, 2, 15));
    expect(result).toBe('15/03/2024');
  });
});

describe('formatTime', () => {
  it('returns first 5 characters', () => {
    expect(formatTime('08:30:00')).toBe('08:30');
  });

  it('handles already short time', () => {
    expect(formatTime('08:30')).toBe('08:30');
  });
});

describe('formatDeadline', () => {
  it('formats an RFC3339 instant as "weekday day de month, HH:MM", Argentina time', () => {
    expect(formatDeadline('2026-09-05T19:00:00-03:00')).toBe('sábado 5 de septiembre, 19:00');
  });

  it('reads the instant in Argentina time regardless of the runtime timezone', () => {
    // 23:30 UTC on the 5th is 20:30 in Argentina (UTC-3), same calendar day.
    expect(formatDeadline('2026-09-05T23:30:00Z')).toBe('sábado 5 de septiembre, 20:30');
  });
});

describe('formatInstantTime', () => {
  it('reads an instant on the venue clock, not the runtime one', () => {
    // 02:00 UTC is 23:00 the previous evening in Argentina. A slice of the
    // string, or a Date read in the runner's own zone, gives another answer.
    expect(formatInstantTime('2026-03-19T02:00:00Z')).toBe('23:00');
  });

  it('keeps an instant already carrying the venue offset', () => {
    expect(formatInstantTime('2026-03-18T23:00:00-03:00')).toBe('23:00');
  });

  it('answers with nothing rather than "Invalid Date"', () => {
    expect(formatInstantTime('')).toBe('');
    expect(formatInstantTime('nope')).toBe('');
  });

  it('is the function an instant needs, which formatTime is not', () => {
    // formatTime is `slice(0, 5)`. Handed an instant it answers "2026-", and
    // both are strings, so nothing on either side of the change would have
    // said so — which is why this is a separate function and this assertion
    // is here rather than in a comment.
    expect(formatTime('2026-03-18T23:00:00-03:00')).toBe('2026-');
    expect(formatInstantTime('2026-03-18T23:00:00-03:00')).toBe('23:00');
  });
});

describe('endsOnALaterDay', () => {
  it('is true when the end rolls past midnight at the venue', () => {
    expect(endsOnALaterDay('2026-03-18T23:00:00-03:00', '2026-03-19T01:00:00-03:00')).toBe(true);
  });

  it('is true for a booking that ends exactly at midnight', () => {
    expect(endsOnALaterDay('2026-03-18T23:00:00-03:00', '2026-03-19T00:00:00-03:00')).toBe(true);
  });

  it('is false inside one day', () => {
    expect(endsOnALaterDay('2026-03-18T18:00:00-03:00', '2026-03-18T19:30:00-03:00')).toBe(false);
  });

  it('compares calendar days at the venue, not in the runtime zone', () => {
    // 22:00 to 23:30 Argentina is 01:00 to 02:30 UTC the next day: read in UTC
    // this booking would be marked as ending tomorrow, on a screen in Buenos
    // Aires where it plainly does not.
    expect(endsOnALaterDay('2026-03-18T22:00:00-03:00', '2026-03-18T23:30:00-03:00')).toBe(false);
  });
});

describe('formatHourRange', () => {
  it('renders an ordinary booking with no marker', () => {
    expect(formatHourRange('2026-03-18T18:00:00-03:00', '2026-03-18T19:30:00-03:00')).toBe('18:00\u00A0–\u00A019:30');
  });

  it('marks a booking whose hours end on the following day', () => {
    // This is the string the product could not produce at all while the end
    // was a clock reading: "23:00 – 01:00" with nothing saying which 01:00.
    expect(formatHourRange('2026-03-18T23:00:00-03:00', '2026-03-19T01:00:00-03:00')).toBe(
      '23:00\u00A0–\u00A001:00\u00A0Día sig.',
    );
  });

  it('takes the separator the caller renders with', () => {
    expect(formatHourRange('2026-03-18T18:00:00-03:00', '2026-03-18T19:30:00-03:00', ' a ')).toBe('18:00 a 19:30');
  });

  it('renders nothing rather than "Invalid Date" when an instant is missing', () => {
    expect(formatHourRange('', '')).toBe('');
  });
});

describe('getHttpErrorMessage', () => {
  it('returns the real backend error message from error.data, not the fallback', async () => {
    const error = await makeConsumedHttpError(400, { error: 'La cancha ya esta reservada' });
    expect(getHttpErrorMessage(error, 'Error generico')).toBe('La cancha ya esta reservada');
  });

  it('does not throw even though error.response.json() would reject (body already read)', async () => {
    const error = await makeConsumedHttpError(400, { error: 'Fondos insuficientes' });
    await expect(error.response.json()).rejects.toThrow();
    expect(getHttpErrorMessage(error, 'Error generico')).toBe('Fondos insuficientes');
  });

  it('returns the fallback when error.data has no parseable error message', async () => {
    const error = await makeConsumedHttpError(500, {});
    expect(getHttpErrorMessage(error, 'Error generico')).toBe('Error generico');
  });

  it('returns the fallback when error.data is undefined (empty body)', async () => {
    const error = await makeConsumedHttpError(500, undefined);
    expect(getHttpErrorMessage(error, 'Error generico')).toBe('Error generico');
  });

  it('returns the fallback instead of throwing when the error is not an HTTPError (network failure, timeout)', () => {
    const networkError = new TypeError('Failed to fetch');
    expect(getHttpErrorMessage(networkError, 'Error generico')).toBe('Error generico');
  });

  // A schema mismatch is not an HTTPError — the request succeeded, the shape
  // is wrong — so before this branch existed it fell into the `!(error
  // instanceof HTTPError)` case and returned whatever mutation-specific
  // `fallback` the caller passed (e.g. "no pudimos cancelar la reserva"),
  // which misattributes a shape mismatch to the action itself. This proves
  // the generic "invalid response" copy is used instead, regardless of the
  // fallback.
  it('returns the generic invalid-response message for an ApiResponseError, ignoring the caller fallback', () => {
    const error = makeApiResponseError();
    expect(getHttpErrorMessage(error, 'No pudimos cancelar la reserva')).toBe(ES_AR.common.invalidResponse);
  });
});

describe('getHttpStatus', () => {
  it('returns the response status for an HTTPError', async () => {
    const error = await makeConsumedHttpError(409, {});
    expect(getHttpStatus(error)).toBe(409);
  });

  it('returns undefined instead of throwing for a non-HTTPError (network failure, timeout)', () => {
    const networkError = new TypeError('Failed to fetch');
    expect(getHttpStatus(networkError)).toBeUndefined();
  });
});

describe('getApiError', () => {
  it('returns fallback when body has no error', () => {
    expect(getApiError({}, 'Error desconocido')).toBe('Error desconocido');
  });

  it('returns fallback when body is undefined', () => {
    expect(getApiError(undefined, 'Fallback')).toBe('Fallback');
  });

  it('returns string error directly', () => {
    expect(getApiError({ error: 'Email ya registrado' }, 'Fallback')).toBe('Email ya registrado');
  });

  it('returns message from error object', () => {
    expect(getApiError({ error: { message: 'Token expirado' } }, 'Fallback')).toBe('Token expirado');
  });

  it('returns fallback for non-standard error object', () => {
    expect(getApiError({ error: { code: 401 } }, 'Fallback')).toBe('Fallback');
  });
});
