import { describe, expect, it } from 'vitest';
import { makeConsumedHttpError } from '../test/factories';
import { BOOKING_MESSAGES } from '../messages';
import { holdRefusalCopy } from './bookingRefusals';

const t = BOOKING_MESSAGES.publicBooking;
const HOLD_CAP = 'https://vibe.com.ar/problems/hold-cap';

describe('holdRefusalCopy on a 429', () => {
  it("shows the server's sentence when the visitor already holds too many slots", async () => {
    const detail = 'Ya tenés 3 horarios reservándose desde esta conexión.';
    const error = await makeConsumedHttpError(429, { type: HOLD_CAP, title: 'Hold cap', status: 429, detail });
    expect(holdRefusalCopy(error)).toBe(detail);
  });

  it('falls back to the local hold-cap sentence when the server sent no detail', async () => {
    const error = await makeConsumedHttpError(429, { type: HOLD_CAP, title: 'Hold cap', status: 429 });
    expect(holdRefusalCopy(error)).toBe(t.holdCap);
    expect(holdRefusalCopy(error)).not.toBe(t.holdRateLimited);
  });

  it('keeps the rate-limit sentence for a plain 429', async () => {
    const error = await makeConsumedHttpError(429, {
      type: 'https://vibe.com.ar/problems/rate-limited',
      title: 'Too many',
      status: 429,
    });
    expect(holdRefusalCopy(error)).toBe(t.holdRateLimited);
  });
});
