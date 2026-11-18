import { describe, expect, it } from 'vitest';
import { HTTPError } from 'ky';
import { BOOKING_MESSAGES } from '../../messages';
import { waitlistRefusal } from './waitlistRefusal';

const t = BOOKING_MESSAGES.waitlist;

function refusal(status: number, errors: { field: string; message: string }[] = []): HTTPError {
  const response = new Response(null, { status });
  const error = new HTTPError(response, new Request('https://api.test/x'), {} as never);
  error.data = { title: 'Refused', status, errors };
  return error;
}

describe('waitlistRefusal', () => {
  it.each([
    ['is free now: book it instead of waiting', t.placeFree],
    ['is already waiting for 20 places at this complex', t.capReached],
    ['is not a slot the court sells on that day for that duration', t.slotGone],
    ['must be in the future', t.slotGone],
  ])('says "%s" in Spanish', (message, copy) => {
    expect(waitlistRefusal(refusal(422, [{ field: 'start_time', message }]))).toBe(copy);
  });

  it('asks to try later when rate limited, and falls back to the generic error', () => {
    expect(waitlistRefusal(refusal(429))).toBe(t.tooMany);
    expect(waitlistRefusal(new Error('network'))).toBe(t.error);
  });
});
