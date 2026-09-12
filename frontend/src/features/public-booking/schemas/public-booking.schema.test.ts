// @vitest-environment node
import { publicBookingSchema } from './public-booking.schema';

const validForm = {
  client_first_name: 'Juan',
  client_last_name: 'Garcia',
  client_phone: '+541155550000',
  client_email: 'juan@test.com',
};

describe('publicBookingSchema', () => {
  it('accepts a fully valid booking form', () => {
    expect(publicBookingSchema.safeParse(validForm).success).toBe(true);
  });

  it('accepts an empty client_email, which is optional', () => {
    expect(publicBookingSchema.safeParse({ ...validForm, client_email: '' }).success).toBe(true);
  });

  it('rejects a malformed, non-empty client_email', () => {
    expect(publicBookingSchema.safeParse({ ...validForm, client_email: 'not-an-email' }).success).toBe(false);
  });

  it('rejects an empty client_first_name', () => {
    expect(publicBookingSchema.safeParse({ ...validForm, client_first_name: '' }).success).toBe(false);
  });

  it('rejects an empty client_last_name', () => {
    expect(publicBookingSchema.safeParse({ ...validForm, client_last_name: '' }).success).toBe(false);
  });

  it('rejects a phone missing the leading country code plus sign', () => {
    expect(publicBookingSchema.safeParse({ ...validForm, client_phone: '1155550000' }).success).toBe(false);
  });

  it('rejects client_notes longer than 2000 characters', () => {
    const result = publicBookingSchema.safeParse({ ...validForm, client_notes: 'a'.repeat(2001) });
    expect(result.success).toBe(false);
  });

  it('accepts client_notes at the 2000 character boundary', () => {
    const result = publicBookingSchema.safeParse({ ...validForm, client_notes: 'a'.repeat(2000) });
    expect(result.success).toBe(true);
  });
});
