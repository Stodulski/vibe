// @vitest-environment node
import { makeBooking } from '@/test/factories';
import {
  bookingSchema,
  bookingsListResponseSchema,
  bookingDetailResponseSchema,
  bookingEnvelopeSchema,
  cancelBookingResponseSchema,
  confirmPaymentResponseSchema,
  manualRefundResponseSchema,
} from './booking.schema';

const validPayment = {
  id: 'pay1',
  booking_id: 'b1',
  complex_id: 'c1',
  amount: 5000,
  service_fee: 250,
  method: 'cash',
  status: 'approved',
  refund_amount: 0,
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
};

describe('bookingSchema', () => {
  it('validates a realistic Booking fixture', () => {
    expect(bookingSchema.safeParse(makeBooking()).success).toBe(true);
  });

  it('rejects a Booking with an unknown status', () => {
    const result = bookingSchema.safeParse(makeBooking({ status: 'ghost' as never }));
    expect(result.success).toBe(false);
  });
});

describe('bookingsListResponseSchema', () => {
  it('validates bookingsApi.list response shape', () => {
    const result = bookingsListResponseSchema.safeParse({
      bookings: [makeBooking()],
      metadata: { has_more: false },
    });
    expect(result.success).toBe(true);
  });
});

describe('bookingDetailResponseSchema', () => {
  it('validates bookingsApi.getById response shape, including nested Client (via z.lazy)', () => {
    const result = bookingDetailResponseSchema.safeParse({
      booking: makeBooking(),
      client: {
        id: 'cl1',
        complex_id: 'c1',
        first_name: 'Juan',
        last_name: 'Garcia',
        phone: '1155550000',
        is_blocked: false,
        total_bookings: 1,
        no_shows: 0,
        created_at: '2026-01-01T00:00:00Z',
        updated_at: '2026-01-01T00:00:00Z',
      },
      payment: validPayment,
      payments: [validPayment],
    });
    expect(result.success).toBe(true);
  });

  it('validates without the optional client and payment fields', () => {
    const result = bookingDetailResponseSchema.safeParse({ booking: makeBooking(), payments: [] });
    expect(result.success).toBe(true);
  });
});

describe('bookingEnvelopeSchema', () => {
  it('validates bookingsApi.create / update response shape', () => {
    expect(bookingEnvelopeSchema.safeParse({ booking: makeBooking() }).success).toBe(true);
  });
});

describe('cancelBookingResponseSchema', () => {
  it('validates bookingsApi.cancel response shape', () => {
    const result = cancelBookingResponseSchema.safeParse({
      booking: makeBooking({ status: 'cancelled' }),
      refund: { status: 'issued', message: 'Devuelto', amount: 2500 },
    });
    expect(result.success).toBe(true);
  });
});

describe('confirmPaymentResponseSchema', () => {
  it('validates bookingsApi.confirmPayment response shape', () => {
    const result = confirmPaymentResponseSchema.safeParse({ booking: makeBooking(), payment: validPayment });
    expect(result.success).toBe(true);
  });
});

describe('manualRefundResponseSchema', () => {
  it('validates bookingsApi.markManualRefund response shape', () => {
    const result = manualRefundResponseSchema.safeParse({
      booking: makeBooking(),
      payments: [validPayment],
      returned_amount: 2500,
    });
    expect(result.success).toBe(true);
  });
});
