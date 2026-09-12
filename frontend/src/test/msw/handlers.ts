import { http, HttpResponse } from 'msw';
import { makeBooking, makeClient, makeComplex, makeCourt, makePrice, makeUser } from '@/test/factories';

/**
 * Default, schema-valid handlers for every backend endpoint the unit suite
 * touches through the real `ky` client (see `src/shared/lib/ky.ts`).
 *
 * These are deliberately generic happy-path responses: a test that needs a
 * specific shape, an error status, or a call-count assertion overrides the
 * relevant handler with `server.use(...)` in its own `it`/`beforeEach` and
 * `afterEach`'s `server.resetHandlers()` (see `src/test/setup.ts`) restores
 * this baseline afterwards.
 *
 * Paths are written without a leading slash and matched with a leading `*`
 * origin wildcard so they work regardless of what `VITE_API_URL` resolves to
 * in the test environment (see the `env` block in `vitest.config.ts`).
 */
export const handlers = [
  // ─── auth ───
  http.post('*/auth/login', () => HttpResponse.json({ user: makeUser(), csrf_token: 'test-csrf-token' })),
  http.post('*/auth/register', () => HttpResponse.json({ message: 'ok' })),
  http.post('*/auth/google', () => HttpResponse.json({ status: 'complete', user: makeUser(), csrf_token: 'tok' })),
  http.post('*/auth/google/complete', () => HttpResponse.json({ user: makeUser(), csrf_token: 'tok' })),
  http.post('*/auth/logout', () => HttpResponse.json({})),
  http.post('*/auth/verify-email', () => HttpResponse.json({ message: 'ok' })),
  http.post('*/auth/resend-verification', () => HttpResponse.json({ message: 'ok' })),
  http.post('*/auth/forgot-password', () => HttpResponse.json({ message: 'ok' })),
  http.post('*/auth/reset-password', () => HttpResponse.json({ message: 'ok' })),
  http.post('*/auth/refresh', () => HttpResponse.json({ csrf_token: 'test-csrf-token' })),
  http.get('*/auth/me', () => HttpResponse.json({ user: makeUser(), csrf_token: 'test-csrf-token' })),
  http.put('*/auth/me', () => HttpResponse.json({ user: makeUser() })),
  http.delete('*/auth/me', () => HttpResponse.json({ message: 'ok' })),

  // ─── public leads ───
  http.post('*/public/leads/abandoned-registration', () => HttpResponse.json({})),

  // ─── bookings ───
  http.get('*/complexes/:complexId/bookings', () =>
    HttpResponse.json({ bookings: [makeBooking()], metadata: { has_more: false } }),
  ),
  http.get('*/complexes/:complexId/bookings/:bookingId', () => HttpResponse.json({ booking: makeBooking() })),

  // ─── dashboard ───
  http.get('*/complexes/:complexId/stats', () =>
    HttpResponse.json({
      stats: {
        today_bookings: 0,
        yesterday_bookings: 0,
        today_revenue: 0,
        yesterday_revenue: 0,
        weekly_revenue: 0,
        monthly_revenue: 0,
        occupancy_rate: 0,
        pending_bookings: 0,
        total_clients: 0,
        upcoming_bookings: [],
        payment_summary: { by_status: {}, by_method: {} },
      },
    }),
  ),
  http.get('*/complexes/:complexId/stats/revenue', () => HttpResponse.json({ revenue: [] })),
  http.get('*/complexes/:complexId/stats/occupancy', () => HttpResponse.json({ occupancy: [] })),
  http.get('*/complexes/:complexId/stats/clients', () =>
    HttpResponse.json({
      clients: {
        top: [],
        no_show_rate: 0,
        no_show_count: 0,
        resolved_count: 0,
        new_clients_30d: 0,
        recurring_30d: 0,
        total_active_30d: 0,
      },
    }),
  ),
  http.get('*/complexes/:complexId/reports/monthly', () =>
    HttpResponse.json({
      report: {
        month: 1,
        year: 2026,
        by_method: {},
        by_court: [],
        totals: { count: 0, total: 0, service_fees: 0, refunded: 0, net: 0 },
        previous_totals: { count: 0, total: 0, service_fees: 0, refunded: 0, net: 0 },
      },
    }),
  ),
  http.get('*/complexes/:complexId/reports/export', () => HttpResponse.arrayBuffer(new ArrayBuffer(0))),

  // ─── clients ───
  http.get('*/complexes/:complexId/clients', () =>
    HttpResponse.json({ clients: [makeClient()], metadata: { has_more: false } }),
  ),
  http.get('*/complexes/:complexId/clients/:clientId', () => HttpResponse.json({ client: makeClient() })),
  http.put('*/complexes/:complexId/clients/:clientId', () => HttpResponse.json({ client: makeClient() })),

  // ─── complexes ───
  http.get('*/complexes', () => HttpResponse.json({ complexes: [makeComplex()] })),
  http.get('*/complexes/:complexId', () => HttpResponse.json({ complex: makeComplex() })),
  http.delete('*/complexes/:complexId', () => HttpResponse.json({ message: 'ok', courts_deactivated: 0 })),

  // ─── admin ───
  http.get('*/admin/stats', () =>
    HttpResponse.json({
      stats: {
        total_users: 0,
        active_users: 0,
        new_users_month: 0,
        total_complexes: 0,
        new_complexes_month: 0,
        total_courts: 0,
        total_bookings: 0,
        total_revenue: 0,
      },
    }),
  ),
  http.patch('*/admin/users/:userId/toggle-active', () => HttpResponse.json({ message: 'ok' })),

  // ─── MercadoPago connect ───
  http.get('*/complexes/:complexId/mp/status', () => HttpResponse.json({ connected: false, app_id: 'app-1' })),
  http.delete('*/complexes/:complexId/mp/connect', () => HttpResponse.json({})),

  // ─── uploads ───
  http.post('*/complexes/:complexId/uploads/presign', () =>
    HttpResponse.json({ upload_url: 'https://r2.test/put', public_url: 'https://cdn.test/logo.webp', key: 'k1' }),
  ),

  // ─── Google Places proxy ───
  http.get('*/places/autocomplete', () => HttpResponse.json({ predictions: [] })),
  http.get('*/places/details', () =>
    HttpResponse.json({
      address: 'Av Corrientes 1234',
      city: 'CABA',
      province: 'Buenos Aires',
      formatted_address: 'Av Corrientes 1234, CABA, Argentina',
      latitude: '-34.6037',
      longitude: '-58.3816',
    }),
  ),

  // ─── public booking ───
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

  // ─── courts ───
  http.get('*/complexes/:complexId/courts', () =>
    HttpResponse.json({ courts: [{ ...makeCourt(), prices: [makePrice()] }] }),
  ),
  http.post('*/complexes/:complexId/courts/:courtId/block', () =>
    HttpResponse.json({
      blocked_slot: { id: 'bs1', court_id: 'ct1', date: '2026-03-18', start_time: '10:00', end_time: '11:00' },
    }),
  ),
];
