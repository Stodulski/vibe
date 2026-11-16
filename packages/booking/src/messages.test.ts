import { describe, expect, it } from 'vitest';
import { BOOKING_MESSAGES } from './messages';

describe('BOOKING_MESSAGES copy', () => {
  it('writes the payment provider as two words, the way its own brand does', () => {
    expect(JSON.stringify(BOOKING_MESSAGES)).not.toContain('MercadoPago');
  });
});
