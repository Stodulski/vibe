// @vitest-environment node
import { http, HttpResponse } from 'msw';
import { server } from '@/test/msw/server';
import { publicBookingApi } from './public-booking.api';
import type { PublicBookingRequest, PublicCancelBookingRequest } from '@/shared/types/api.types';

// Complete response fixtures — one per method — so the schema wired into
// each method (`H-schemas.md`'s wiring table) actually parses instead of
// rejecting a partial `{}` mock. Only the fields each schema requires are
// filled in; optional fields are left out on purpose to prove the schema
// still accepts the older-server shape described in
// `publicBooking.ts` (e.g. `BookingStatusDetails`, lines ~136-141).
const COMPLEX_RESPONSE = {
  complex: {
    id: 'complex-1',
    name: 'Club Norte',
    slug: 'test-club',
    address: 'Av. Siempre Viva 123',
    city: 'CABA',
    province: 'Buenos Aires',
    country_code: 'AR',
    currency: 'ARS',
    phone: '1155550000',
    deposit_percentage: 30,
    cancellation_hours: 24,
    amenities: [],
    payments_enabled: true,
  },
  courts: [],
  schedules: [],
};

const AVAILABILITY_RESPONSE = {
  availability: {
    date: '2026-03-18',
    day: 'wednesday',
    is_open: true,
    courts: [],
  },
};

const CREATE_BOOKING_RESPONSE = {
  booking: {
    status: 'pending',
    collection_status: 'unpaid',
    refund_status: 'none',
    date: '2026-03-18',
    start_time: '10:00',
    starts_at: '2026-03-18T10:00:00-03:00',
    ends_at: '2026-03-18T11:30:00-03:00',
    court_name: 'Cancha 1',
    complex_name: 'Club Norte',
    price: 1_000_000,
    deposit_amount: 300_000,
  },
  token: 'tok1',
};

// Only `status`/`collection_status`/`refund_status` are guaranteed by an
// older server (see `BookingStatusDetails`'s doc comment) — every other
// field is intentionally absent here.
const BOOKING_STATUS_RESPONSE = {
  booking: {
    status: 'pending',
    collection_status: 'unpaid',
    refund_status: 'none',
  },
};

const CANCEL_INFO_RESPONSE = {
  booking: {
    status: 'confirmed',
    date: '2026-03-18',
    start_time: '10:00',
    court_name: 'Cancha 1',
    complex_name: 'Club Norte',
  },
  can_cancel: true,
  can_refund: true,
  refund_method: 'mercadopago',
  cancellation_hours: 24,
};

const CANCEL_BOOKING_RESPONSE = {
  booking: {
    status: 'cancelled',
    collection_status: 'unpaid',
    refund_status: 'pending',
  },
  refunded: true,
  refund: {
    status: 'issued',
    message: 'Reembolso en curso',
  },
};

describe('publicBookingApi', () => {
  it('getComplex calls GET public/complexes/:slug', async () => {
    server.use(http.get('*/public/complexes/:slug', () => HttpResponse.json(COMPLEX_RESPONSE)));
    await expect(publicBookingApi.getComplex('test-club')).resolves.toEqual(COMPLEX_RESPONSE);
  });

  it('getAvailability calls GET public/complexes/:slug/availability with the date and duration params', async () => {
    let receivedDate: string | null = null;
    let receivedDuration: string | null = null;
    server.use(
      http.get('*/public/complexes/:slug/availability', ({ request }) => {
        const params = new URL(request.url).searchParams;
        receivedDate = params.get('date');
        receivedDuration = params.get('duration');
        return HttpResponse.json(AVAILABILITY_RESPONSE);
      }),
    );

    await publicBookingApi.getAvailability('test-club', '2026-03-18', 90);

    expect(receivedDate).toBe('2026-03-18');
    expect(receivedDuration).toBe('90');
  });

  it('createBooking calls POST book with the request body and the attempt Idempotency-Key', async () => {
    let receivedBody: unknown;
    let receivedKey: string | null = null;
    server.use(
      http.post('*/book', async ({ request }) => {
        receivedBody = await request.json();
        receivedKey = request.headers.get('Idempotency-Key');
        return HttpResponse.json(CREATE_BOOKING_RESPONSE);
      }),
    );

    const data: PublicBookingRequest = {
      complex_id: 'c1',
      court_id: 'ct1',
      date: '2026-03-18',
      start_time: '10:00',
      duration_minutes: 90,
      client_first_name: 'Juan',
      client_last_name: 'Garcia',
      client_phone: '1155550000',
      client_email: 'juan@test.com',
    };
    await publicBookingApi.createBooking(data, 'attempt-key-1');

    expect(receivedBody).toEqual(data);
    // The server deduplicates a retried booking on this header, so it has to
    // be on the wire, not merely accepted as an argument.
    expect(receivedKey).toBe('attempt-key-1');
  });
});

// A sibling describe, not nested in the one above: max-lines-per-function
// counts a describe callback's whole body, so this stays a genuinely
// separate top-level call.
describe('publicBookingApi — booking status and cancellation', () => {
  it('getBookingStatus calls GET book/status with the token query param', async () => {
    let receivedToken: string | null = null;
    server.use(
      http.get('*/book/status', ({ request }) => {
        receivedToken = new URL(request.url).searchParams.get('token');
        return HttpResponse.json(BOOKING_STATUS_RESPONSE);
      }),
    );

    await publicBookingApi.getBookingStatus('tok1');

    expect(receivedToken).toBe('tok1');
  });

  it('getCancelInfo calls GET book/cancel-info with the token query param', async () => {
    let receivedToken: string | null = null;
    server.use(
      http.get('*/book/cancel-info', ({ request }) => {
        receivedToken = new URL(request.url).searchParams.get('token');
        return HttpResponse.json(CANCEL_INFO_RESPONSE);
      }),
    );

    await publicBookingApi.getCancelInfo('tok1');

    expect(receivedToken).toBe('tok1');
  });

  it('cancelBooking calls POST book/cancel with the token in the body', async () => {
    let receivedBody: unknown;
    server.use(
      http.post('*/book/cancel', async ({ request }) => {
        receivedBody = await request.json();
        return HttpResponse.json(CANCEL_BOOKING_RESPONSE);
      }),
    );

    const data: PublicCancelBookingRequest = { token: 'tok1' };
    await publicBookingApi.cancelBooking(data);
    expect(receivedBody).toEqual(data);
  });

  it('rejects with ApiResponseError when the response body does not match the schema', async () => {
    const { ApiResponseError } = await import('@/shared/lib/apiParse');
    server.use(http.get('*/book/status', () => HttpResponse.json({ booking: { status: 'not-a-real-status' } })));

    await expect(publicBookingApi.getBookingStatus('tok1')).rejects.toThrow(ApiResponseError);
  });
});
