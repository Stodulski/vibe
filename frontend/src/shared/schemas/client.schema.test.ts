import { describe, it, expect } from 'vitest';
// @vitest-environment node
import { makeBooking } from '@/test/factories';
import {
  clientSchema,
  clientsListResponseSchema,
  clientDetailResponseSchema,
  clientEnvelopeSchema,
} from './client.schema';

const validClient = {
  id: 'cl1',
  complex_id: 'c1',
  first_name: 'Juan',
  last_name: 'Garcia',
  phone: '1155550000',
  is_blocked: false,
  total_bookings: 3,
  no_shows: 0,
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
};

describe('clientSchema', () => {
  it('validates a realistic Client fixture', () => {
    expect(clientSchema.safeParse(validClient).success).toBe(true);
  });
});

describe('clientsListResponseSchema', () => {
  it('validates clientsApi.list response shape', () => {
    const result = clientsListResponseSchema.safeParse({
      clients: [validClient],
      metadata: { has_more: false },
    });
    expect(result.success).toBe(true);
  });
});

describe('clientDetailResponseSchema', () => {
  it('validates clientsApi.getById response shape, including nested Booking[]', () => {
    const result = clientDetailResponseSchema.safeParse({
      client: validClient,
      recent_bookings: [makeBooking()],
    });
    expect(result.success).toBe(true);
  });
});

describe('clientEnvelopeSchema', () => {
  it('validates clientsApi.update response shape', () => {
    expect(clientEnvelopeSchema.safeParse({ client: validClient }).success).toBe(true);
  });
});
