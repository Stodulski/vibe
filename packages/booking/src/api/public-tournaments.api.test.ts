// @vitest-environment node
import { describe, it, expect } from 'vitest';
import { http, HttpResponse } from 'msw';
import { server } from '../test/msw/server';
import { createPublicBookingApi } from './public-booking.api';
import { createBookingClient } from './client';
import { ApiResponseError } from '../lib/apiParse';
import { CAT_OPEN, DETAIL, SUMMARY, TOURNAMENT, pairRegistration } from '../test/tournamentFixtures';

const api = createPublicBookingApi(createBookingClient('http://localhost/api/v1'));

describe('public tournaments api reads', () => {
  it('lists the upcoming tournaments and reads a null list as empty', async () => {
    server.use(http.get('*/public/complexes/club/tournaments', () => HttpResponse.json({ tournaments: [SUMMARY] })));
    expect((await api.listTournaments('club')).tournaments).toEqual([SUMMARY]);

    server.use(http.get('*/public/complexes/club/tournaments', () => HttpResponse.json({ tournaments: null })));
    expect((await api.listTournaments('club')).tournaments).toEqual([]);
  });

  it('reads a tournament and turns the nil slices of an undrawn one into empty lists', async () => {
    server.use(
      http.get('*/public/complexes/club/tournaments/t1', () =>
        HttpResponse.json({ ...DETAIL, pairs: null, groups: null, matches: null }),
      ),
    );
    const detail = await api.getTournament('club', 't1');
    expect(detail.pairs).toEqual([]);
    expect(detail.groups).toEqual([]);
    expect(detail.matches).toEqual([]);
  });

  it('keeps a format and a status this build does not know', async () => {
    server.use(
      http.get('*/public/complexes/club/tournaments/t1', () =>
        HttpResponse.json({
          ...DETAIL,
          tournament: { ...TOURNAMENT, status: 'paused' },
          categories: [{ ...CAT_OPEN, format: 'americano' }],
        }),
      ),
    );
    const detail = await api.getTournament('club', 't1');
    expect(detail.tournament.status).toBe('paused');
    expect(detail.categories[0]?.format).toBe('americano');
  });

  it('rejects a match without its slots', async () => {
    const [first, ...rest] = DETAIL.matches;
    const { pair1_id: _drop, ...broken } = first ?? {};
    server.use(
      http.get('*/public/complexes/club/tournaments/t1', () => HttpResponse.json({ ...DETAIL, matches: [broken, ...rest] })),
    );
    await expect(api.getTournament('club', 't1')).rejects.toBeInstanceOf(ApiResponseError);
  });

  it('reads a pair registration through its token', async () => {
    let token: string | null = null;
    server.use(
      http.get('*/public/tournament-registrations/status', ({ request }) => {
        token = new URL(request.url).searchParams.get('token');
        return HttpResponse.json(pairRegistration());
      }),
    );
    expect((await api.getTournamentRegistration('tok')).payer.first_name).toBe('Ana');
    expect(token).toBe('tok');
  });
});

describe('public tournaments api writes', () => {
  it('posts the pair to its category and returns the token and checkout link', async () => {
    let body: unknown;
    server.use(
      http.post('*/public/complexes/club/tournaments/t1/categories/c1/pairs', async ({ request }) => {
        body = await request.json();
        return HttpResponse.json(
          {
            registration: { status: 'held', held_until: '2099-12-01T10:15:00-03:00' },
            token: 'tok',
            tournament: TOURNAMENT,
            category: CAT_OPEN,
            mp_init_point: 'https://mp.test/x',
            mp_preference_id: 'pref-1',
            service_fee: 200_000,
            total_price: 2_200_000,
          },
          { status: 201 },
        );
      }),
    );
    const request = {
      first_name: 'Ana',
      last_name: 'Pérez',
      phone: '+541123456789',
      partner: { first_name: 'Luis', last_name: 'Gómez', phone: '+541198765432' },
    };
    const result = await api.signUpPairForTournament('club', 't1', 'c1', request);
    expect(body).toEqual(request);
    expect(result.token).toBe('tok');
    expect(result.mp_init_point).toBe('https://mp.test/x');
  });

  it('cancels a pair by token', async () => {
    let body: unknown;
    server.use(
      http.post('*/public/tournament-registrations/cancel', async ({ request }) => {
        body = await request.json();
        return HttpResponse.json({
          registration: pairRegistration({ status: 'cancelled' }),
          refunded: true,
          refund: { status: 'issued', message: 'ok', amount: 1 },
        });
      }),
    );
    const result = await api.cancelTournamentRegistration({ token: 'tok' });
    expect(body).toEqual({ token: 'tok' });
    expect(result.refunded).toBe(true);
  });
});
