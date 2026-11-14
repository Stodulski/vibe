// @vitest-environment node
import { describe, it, expect } from 'vitest';
import { http, HttpResponse } from 'msw';
import { server } from '../test/msw/server';
import { createPublicBookingApi } from './public-booking.api';
import { createBookingClient } from './client';
import { ApiResponseError } from '../lib/apiParse';
import { EVENT, registration } from '../test/eventFixtures';

const api = createPublicBookingApi(createBookingClient('http://localhost/api/v1'));

describe('public events api reads', () => {
  it('lists the upcoming events and reads a null list as empty', async () => {
    server.use(http.get('*/public/complexes/club/events', () => HttpResponse.json({ events: [EVENT] })));
    expect((await api.listEvents('club')).events).toEqual([EVENT]);

    server.use(http.get('*/public/complexes/club/events', () => HttpResponse.json({ events: null })));
    expect((await api.listEvents('club')).events).toEqual([]);
  });

  it('keeps a sport and status this build does not know', async () => {
    server.use(
      http.get('*/public/complexes/club/events/ev1', () =>
        HttpResponse.json({ event: { ...EVENT, sport: 'squash', status: 'postponed' } }),
      ),
    );
    const { event } = await api.getEvent('club', 'ev1');
    expect(event.sport).toBe('squash');
    expect(event.status).toBe('postponed');
  });

  it('rejects an event without its price', async () => {
    const { total_price: _drop, ...broken } = EVENT;
    server.use(http.get('*/public/complexes/club/events/ev1', () => HttpResponse.json({ event: broken })));
    await expect(api.getEvent('club', 'ev1')).rejects.toBeInstanceOf(ApiResponseError);
  });

});

describe('public events api writes', () => {
  it('posts the sign-up and returns the token and checkout link', async () => {
    let body: unknown;
    server.use(
      http.post('*/public/complexes/club/events/ev1/registrations', async ({ request }) => {
        body = await request.json();
        return HttpResponse.json(
          {
            registration: { status: 'held', held_until: '2026-12-05T19:15:00-03:00' },
            token: 'tok',
            event: EVENT,
            mp_init_point: 'https://mp.test/x',
            service_fee: 100_000,
            total_price: 1_100_000,
          },
          { status: 201 },
        );
      }),
    );
    const result = await api.signUpForEvent('club', 'ev1', { first_name: 'Ana', last_name: 'Pérez', phone: '+5411' });
    expect(body).toEqual({ first_name: 'Ana', last_name: 'Pérez', phone: '+5411' });
    expect(result.token).toBe('tok');
    expect(result.mp_init_point).toBe('https://mp.test/x');
  });

  it('reads the registration status by token and posts the cancel', async () => {
    let seenToken: string | null = null;
    server.use(
      http.get('*/public/event-registrations/status', ({ request }) => {
        seenToken = new URL(request.url).searchParams.get('token');
        return HttpResponse.json(registration());
      }),
      http.post('*/public/event-registrations/cancel', () =>
        HttpResponse.json({
          registration: registration({ status: 'cancelled' }),
          refunded: false,
          refund: { status: 'not_eligible', message: 'Fuera de plazo.' },
        }),
      ),
    );
    expect((await api.getEventRegistration('tok')).status).toBe('confirmed');
    expect(seenToken).toBe('tok');
    const cancelled = await api.cancelEventRegistration({ token: 'tok' });
    expect(cancelled.refund.message).toBe('Fuera de plazo.');
  });
});
