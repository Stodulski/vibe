import { http, HttpResponse } from 'msw';

/**
 * Default, schema-valid handlers for the public endpoints the booking flow
 * calls. Every route is answered under `http://localhost/api/v1`, the base the
 * suite configures (`TEST_BOOKING_CONFIG` in `../booking.tsx`).
 */
export const handlers = [
  http.get('*/public/complexes/:slug', () =>
    HttpResponse.json({
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
    }),
  ),
  http.get('*/public/complexes/:slug/availability', () =>
    HttpResponse.json({ availability: { date: '2026-03-18', day: 'wednesday', is_open: true, courts: [] } }),
  ),
  http.post('*/book', () =>
    HttpResponse.json({
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
    }),
  ),
  http.get('*/book/status', () =>
    HttpResponse.json({ booking: { status: 'pending', collection_status: 'unpaid', refund_status: 'none' } }),
  ),
  http.get('*/book/cancel-info', () =>
    HttpResponse.json({
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
    }),
  ),
  http.post('*/book/cancel', () =>
    HttpResponse.json({
      booking: { status: 'cancelled', collection_status: 'unpaid', refund_status: 'pending' },
      refunded: true,
      refund: { status: 'issued', message: 'Reembolso en curso' },
    }),
  ),
];
