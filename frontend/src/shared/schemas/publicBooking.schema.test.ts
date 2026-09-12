import { describe, it, expect } from 'vitest';
// @vitest-environment node
import { makeCourt, makePrice } from '@/test/factories';
import {
  publicComplexResponseSchema,
  publicBookingResponseSchema,
  mpConnectResponseSchema,
  mpStatusResponseSchema,
  bookingStatusResponseSchema,
  cancelInfoResponseSchema,
  publicCancelBookingResponseSchema,
} from './publicBooking.schema';

const validPublicComplex = {
  id: 'c1',
  name: 'Club Padel',
  slug: 'club-padel',
  address: 'Calle Falsa 123',
  city: 'CABA',
  province: 'Buenos Aires',
  country_code: 'AR',
  currency: 'ARS',
  phone: '1155550000',
  deposit_percentage: 50,
  cancellation_hours: 24,
  amenities: ['wifi'],
  payments_enabled: true,
};

describe('publicComplexResponseSchema', () => {
  it('validates publicBookingApi.getComplex response shape', () => {
    const result = publicComplexResponseSchema.safeParse({
      complex: validPublicComplex,
      courts: [{ ...makeCourt(), prices: [makePrice()] }],
      schedules: [],
    });
    expect(result.success).toBe(true);
  });

  it('accepts a cleared email, logo, cover or coordinate sent as null, and reads it as absent', () => {
    // The owner projection of the same row declares these nullable, so a
    // cleared column can reach the storefront as null. The storefront has no
    // "cleared" state to render — only "has one" or "does not".
    const result = publicComplexResponseSchema.safeParse({
      complex: {
        ...validPublicComplex,
        email: null,
        logo_url: null,
        cover_url: null,
        latitude: null,
        longitude: null,
      },
      courts: [{ ...makeCourt(), prices: [makePrice()] }],
      schedules: [],
    });
    expect(result.success).toBe(true);
    expect(result.success && result.data.complex.email).toBeUndefined();
    expect(result.success && result.data.complex.logo_url).toBeUndefined();
    expect(result.success && result.data.complex.latitude).toBeUndefined();
  });
});

describe('publicBookingResponseSchema', () => {
  it('validates publicBookingApi.createBooking response shape', () => {
    const result = publicBookingResponseSchema.safeParse({
      booking: {
        status: 'confirmed',
        collection_status: 'deposit_paid',
        refund_status: 'none',
        date: '2026-03-18',
        start_time: '10:00',
        starts_at: '2026-03-18T13:00:00Z',
        ends_at: '2026-03-18T14:00:00Z',
        court_name: 'Cancha 1',
        complex_name: 'Club Padel',
        price: 5000,
        deposit_amount: 2500,
      },
      token: 'tok123',
    });
    expect(result.success).toBe(true);
  });
});

describe('mp status/connect response schemas', () => {
  it('validates useMPCallback response shape', () => {
    expect(mpConnectResponseSchema.safeParse({ connected: true, mp_user_id: 'mp1' }).success).toBe(true);
  });

  it('validates useOnboardingMPConnect response shape', () => {
    expect(mpStatusResponseSchema.safeParse({ connected: false }).success).toBe(true);
  });
});

describe('bookingStatusResponseSchema', () => {
  it('validates a full BookingStatusDetails fixture', () => {
    const result = bookingStatusResponseSchema.safeParse({
      booking: { status: 'confirmed', collection_status: 'fully_paid', refund_status: 'none' },
    });
    expect(result.success).toBe(true);
  });

  it('accepts a venue with no phone on file, sent as null, and reads it as absent', () => {
    const result = bookingStatusResponseSchema.safeParse({
      booking: {
        status: 'confirmed',
        collection_status: 'fully_paid',
        refund_status: 'none',
        complex_phone: null,
      },
    });
    expect(result.success).toBe(true);
    expect(result.success && result.data.booking.complex_phone).toBeUndefined();
  });
});

describe('cancelInfoResponseSchema', () => {
  it('validates publicBookingApi.getCancelInfo response shape', () => {
    const result = cancelInfoResponseSchema.safeParse({
      booking: {
        status: 'confirmed',
        date: '2026-03-18',
        start_time: '10:00',
        court_name: 'Cancha 1',
        complex_name: 'Club Padel',
      },
      can_cancel: true,
      can_refund: true,
      refund_method: 'mercadopago',
      cancellation_hours: 24,
    });
    expect(result.success).toBe(true);
  });
});

describe('publicCancelBookingResponseSchema', () => {
  it('validates publicBookingApi.cancelBooking response shape', () => {
    const result = publicCancelBookingResponseSchema.safeParse({
      booking: { status: 'cancelled', collection_status: 'unpaid', refund_status: 'none' },
      refunded: false,
      refund: { status: 'none', message: 'Nada para devolver' },
    });
    expect(result.success).toBe(true);
  });
});
