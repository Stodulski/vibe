// @vitest-environment node
const { mockGet, mockPost } = vi.hoisted(() => ({
  mockGet: vi.fn().mockReturnValue({ json: vi.fn().mockResolvedValue({}) }),
  mockPost: vi.fn().mockReturnValue({ json: vi.fn().mockResolvedValue({}) }),
}));

vi.mock('@/shared/lib/ky', () => ({
  default: { get: mockGet, post: mockPost },
  withSignal: (signal?: AbortSignal) => (signal ? { signal } : {}),
}));

import { publicBookingApi } from './public-booking.api';
import type { PublicBookingRequest, PublicCancelBookingRequest } from '@/shared/types/api.types';

// Complete response fixtures — one per method — so the schema wired into
// each method (`H-schemas.md`'s wiring table) actually parses instead of
// rejecting a partial `{}` mock. Only the fields each schema requires are
// filled in; optional fields are left out on purpose to prove the schema
// still accepts the older-server shape described in
// `publicBooking.ts` (e.g. `BookingStatusDetails`, lines ~136-141).
function mockJsonOnce(mock: typeof mockGet, data: unknown) {
  mock.mockReturnValueOnce({ json: vi.fn().mockResolvedValue(data) });
}

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

beforeEach(() => vi.clearAllMocks());

describe('publicBookingApi', () => {
  it('getComplex calls GET public/complexes/:slug', async () => {
    mockJsonOnce(mockGet, COMPLEX_RESPONSE);
    await publicBookingApi.getComplex('test-club');
    expect(mockGet).toHaveBeenCalledWith('public/complexes/test-club', expect.any(Object));
  });

  it('getAvailability calls GET public/complexes/:slug/availability', async () => {
    mockJsonOnce(mockGet, AVAILABILITY_RESPONSE);
    await publicBookingApi.getAvailability('test-club', '2026-03-18', 90);
    expect(mockGet).toHaveBeenCalledWith(
      'public/complexes/test-club/availability',
      expect.objectContaining({
        searchParams: { date: '2026-03-18', duration: 90 },
      }),
    );
  });

  it('createBooking calls POST book', async () => {
    mockJsonOnce(mockPost, CREATE_BOOKING_RESPONSE);
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
    await publicBookingApi.createBooking(data);
    expect(mockPost).toHaveBeenCalledWith('book', { json: data });
  });
});

// A sibling describe, not nested in the one above: max-lines-per-function
// counts a describe callback's whole body, so this stays a genuinely
// separate top-level call.
describe('publicBookingApi — booking status and cancellation', () => {
  it('getBookingStatus calls GET book/status with the token query param', async () => {
    mockJsonOnce(mockGet, BOOKING_STATUS_RESPONSE);
    await publicBookingApi.getBookingStatus('tok1');
    expect(mockGet).toHaveBeenCalledWith(
      'book/status',
      expect.objectContaining({
        searchParams: { token: 'tok1' },
      }),
    );
  });

  it('getCancelInfo calls GET book/cancel-info with the token query param', async () => {
    mockJsonOnce(mockGet, CANCEL_INFO_RESPONSE);
    await publicBookingApi.getCancelInfo('tok1');
    expect(mockGet).toHaveBeenCalledWith(
      'book/cancel-info',
      expect.objectContaining({
        searchParams: { token: 'tok1' },
      }),
    );
  });

  it('cancelBooking calls POST book/cancel with the token in the body', async () => {
    mockJsonOnce(mockPost, CANCEL_BOOKING_RESPONSE);
    const data: PublicCancelBookingRequest = { token: 'tok1' };
    await publicBookingApi.cancelBooking(data);
    expect(mockPost).toHaveBeenCalledWith('book/cancel', { json: data });
  });

  it('rejects with ApiResponseError when the response body does not match the schema', async () => {
    const { ApiResponseError } = await import('@/shared/lib/apiParse');
    mockJsonOnce(mockGet, { booking: { status: 'not-a-real-status' } });

    await expect(publicBookingApi.getBookingStatus('tok1')).rejects.toThrow(ApiResponseError);
  });
});
