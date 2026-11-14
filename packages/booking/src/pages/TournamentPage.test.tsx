import { describe, it, expect, vi, beforeEach } from 'vitest';
import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { server } from '../test/msw/server';
import { renderBooking, setBookingUrl } from '../test/booking';
import { CAT_OPEN, DETAIL, TOURNAMENT, pairRegistration } from '../test/tournamentFixtures';
import { BOOKING_MESSAGES } from '../messages';
import { storeTournamentRegistrationToken } from '../lib/tournamentRegistrationStorage';
import { navigateTo } from '../lib/navigation';
import TournamentPage from './TournamentPage';

vi.mock('../lib/navigation', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../lib/navigation')>()),
  navigateTo: vi.fn(),
}));

const t = BOOKING_MESSAGES;

function serveTournament(detail = DETAIL) {
  server.use(http.get('*/public/complexes/:slug/tournaments/:id', () => HttpResponse.json(detail)));
}

function serveRegistration(body: ReturnType<typeof pairRegistration>) {
  server.use(http.get('*/public/tournament-registrations/status', () => HttpResponse.json(body)));
}

function open(search = '') {
  setBookingUrl(`/c/test-club/tournaments/t1${search}`);
  return renderBooking(<TournamentPage slug="test-club" tournamentId="t1" />);
}

beforeEach(() => {
  vi.mocked(navigateTo).mockClear();
});

async function openForm(user: ReturnType<typeof userEvent.setup>) {
  await user.click(await screen.findByRole('button', { name: t.tournaments.signUp }));
}

async function fillForm(user: ReturnType<typeof userEvent.setup>, partnerPhone = '1198765432') {
  const you = within(await screen.findByRole('group', { name: t.tournaments.payerTitle }));
  await user.type(you.getByLabelText(/^Nombre/), 'Ana');
  await user.type(you.getByLabelText(/^Apellido/), 'Pérez');
  await user.type(you.getByLabelText(/^Teléfono/), '1123456789');
  const partner = within(screen.getByRole('group', { name: t.tournaments.partnerTitle }));
  await user.type(partner.getByLabelText(/^Nombre/), 'Luis');
  await user.type(partner.getByLabelText(/^Apellido/), 'Gómez');
  await user.type(partner.getByLabelText(/^Teléfono/), partnerPhone);
}

describe('TournamentPage detail', () => {
  it('shows the tournament, each category and its format', async () => {
    serveTournament();
    open();

    expect(await screen.findByRole('heading', { level: 1, name: TOURNAMENT.name })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: '8va' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: '6ta' })).toBeInTheDocument();
    expect(screen.getAllByText('Al mejor de 3 sets \u00b7 super tie-break').length).toBeGreaterThan(0);
    expect(screen.getByRole('heading', { name: '7ma' })).toBeInTheDocument();
    expect(screen.getAllByText(t.tournaments.formatGroups)).toHaveLength(2);
    expect(screen.getByText(t.tournaments.formatElimination)).toBeInTheDocument();
  });

  it('shows the groups, the bracket and the schedule of a drawn category', async () => {
    serveTournament();
    open();

    expect(await screen.findByRole('heading', { name: 'Zona A' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Zona B' })).toBeInTheDocument();
    expect(screen.getAllByRole('region', { name: t.tournaments.bracketTitle })).toHaveLength(2);
    expect(screen.getByRole('heading', { name: t.tournaments.scheduleTitle })).toBeInTheDocument();
  });

  it('offers the sign-up only on the category that is open, with its price', async () => {
    serveTournament();
    open();

    expect(await screen.findAllByRole('button', { name: t.tournaments.signUp })).toHaveLength(1);
    expect(screen.getByText(`Quedan ${String(CAT_OPEN.spots_left)} lugares`)).toBeInTheDocument();
    expect(screen.getAllByText(t.tournaments.signUpClosedCategory)).toHaveLength(2);
  });

  it('never shows a phone, an email or a payment of a pair', async () => {
    serveTournament();
    const { container } = open();
    await screen.findByRole('heading', { name: 'Zona A' });
    expect(container.textContent).not.toMatch(/\+54|@|\d{8}/);
  });

  it('shows no sign-up on a cancelled tournament and says it was cancelled', async () => {
    serveTournament({ ...DETAIL, tournament: { ...TOURNAMENT, status: 'cancelled' } });
    open();
    expect(await screen.findByRole('heading', { name: t.tournaments.cancelledTitle })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: t.tournaments.signUp })).not.toBeInTheDocument();
  });

  it('shows not found for an unknown tournament', async () => {
    server.use(http.get('*/public/complexes/:slug/tournaments/:id', () => new HttpResponse(null, { status: 404 })));
    open();
    expect(await screen.findByText(t.tournaments.notFound)).toBeInTheDocument();
  });
});

describe('TournamentPage sign-up validation', () => {
  it('validates the payer and the partner before sending', async () => {
    serveTournament();
    const post = vi.fn();
    server.use(
      http.post('*/public/complexes/:slug/tournaments/:id/categories/:cid/pairs', () => {
        post();
        return HttpResponse.json({});
      }),
    );
    open();
    const user = userEvent.setup();
    await openForm(user);
    await user.click(screen.getByRole('button', { name: t.publicBooking.paySecurely }));

    expect(await screen.findAllByText(t.validation.nameRequired)).toHaveLength(2);
    expect(post).not.toHaveBeenCalled();
  });

  it('refuses the same phone for both players', async () => {
    serveTournament();
    open();
    const user = userEvent.setup();
    await openForm(user);
    await fillForm(user, '1123456789');
    await user.click(screen.getByRole('button', { name: t.publicBooking.paySecurely }));

    expect(await screen.findByText(t.tournaments.partnerPhoneSame)).toBeInTheDocument();
    expect(navigateTo).not.toHaveBeenCalled();
  });

});

describe('TournamentPage sign-up', () => {
  it('sends the pair to the chosen category and goes to the Mercado Pago checkout', async () => {
    serveTournament();
    let body: unknown;
    let url = '';
    server.use(
      http.post('*/public/complexes/:slug/tournaments/:id/categories/:cid/pairs', async ({ request }) => {
        url = new URL(request.url).pathname;
        body = await request.json();
        return HttpResponse.json(
          {
            registration: { status: 'held', held_until: '2099-12-01T10:15:00-03:00' },
            token: 'tok-1',
            tournament: TOURNAMENT,
            category: CAT_OPEN,
            mp_init_point: 'https://mp.test/checkout',
            mp_preference_id: 'pref-1',
            service_fee: 200_000,
            total_price: 2_200_000,
          },
          { status: 201 },
        );
      }),
    );
    open();
    const user = userEvent.setup();
    await openForm(user);
    await fillForm(user);
    await user.click(screen.getByRole('button', { name: t.publicBooking.paySecurely }));

    await waitFor(() => {
      expect(navigateTo).toHaveBeenCalledWith('https://mp.test/checkout');
    });
    expect(url).toMatch(/\/tournaments\/t1\/categories\/c-open\/pairs$/);
    expect(body).toEqual({
      first_name: 'Ana',
      last_name: 'Pérez',
      phone: '+541123456789',
      partner: { first_name: 'Luis', last_name: 'Gómez', phone: '+541198765432' },
    });
    expect(sessionStorage.getItem('vibe_tournament_registration:t1')).toBe('"tok-1"');
  });
});

describe('TournamentPage refused sign-up', () => {
  const REFUSALS = [
    { kind: 'event-full', status: 409, copy: t.tournaments.categoryFullRefusal },
    { kind: 'event-closed', status: 409, copy: t.tournaments.signUpClosed },
    { kind: 'already-registered', status: 409, copy: t.tournaments.alreadyRegistered },
    { kind: 'conflict', status: 409, copy: t.tournaments.signUpConflict },
    { kind: 'rate-limited', status: 429, copy: t.tournaments.tooManyUnpaid },
  ];

  it.each(REFUSALS)('explains a $kind refusal in its own words without leaving the page', async ({ kind, status, copy }) => {
    serveTournament();
    server.use(
      http.post('*/public/complexes/:slug/tournaments/:id/categories/:cid/pairs', () =>
        HttpResponse.json(
          { type: `https://vibe.com.ar/problems/${kind}`, title: 'Refused', status, detail: 'English prose' },
          { status, headers: { 'Content-Type': 'application/problem+json' } },
        ),
      ),
    );
    open();
    const user = userEvent.setup();
    await openForm(user);
    await fillForm(user);
    await user.click(screen.getByRole('button', { name: t.publicBooking.paySecurely }));

    expect(await screen.findByRole('alert')).toHaveTextContent(copy);
    expect(navigateTo).not.toHaveBeenCalled();
  });
});

describe('TournamentPage checkout return', () => {
  it('shows the processing state while the spot is still held', async () => {
    serveRegistration(pairRegistration({ status: 'held' }));
    open('?token=tok-1&registered=1');
    expect(await screen.findByText(t.events.processingTitle)).toBeInTheDocument();
  });

  it('shows the under review state for ?status=pending', async () => {
    serveRegistration(pairRegistration({ status: 'held' }));
    open('?token=tok-1&registered=1&status=pending');
    expect(await screen.findByText(t.events.underReviewTitle)).toBeInTheDocument();
  });

  it('confirms the pair, names both players and shows the refund window', async () => {
    serveRegistration(pairRegistration());
    open('?token=tok-1&registered=1');

    expect(await screen.findByText(t.tournaments.confirmedTitle)).toBeInTheDocument();
    expect(screen.getByText('Ana Pérez')).toBeInTheDocument();
    expect(screen.getByText('Luis Gómez')).toBeInTheDocument();
    expect(screen.getByText(new RegExp(t.events.refundUntilPrefix))).toBeInTheDocument();
  });

});

describe('TournamentPage checkout return, cancelling', () => {
  it('warns that leaving after the draw loses the matches and keeps the payment', async () => {
    serveRegistration(
      pairRegistration({
        category: { ...CAT_OPEN, draw_generated: true, status: 'closed' },
        cancellation: { can_cancel: true, within_window: false, deadline: '2099-12-04T19:00:00-03:00', walkover: true },
      }),
    );
    open('?token=tok-1&registered=1');
    const user = userEvent.setup();
    expect(await screen.findByText(t.tournaments.walkoverNote)).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: t.tournaments.cancelPair }));
    expect(await screen.findByText(t.tournaments.cancelConfirmWalkover)).toBeInTheDocument();
  });
});

describe('TournamentPage checkout return, after the pair acts', () => {
  it('cancels the pair and renders the server refund message as it comes', async () => {
    serveRegistration(pairRegistration());
    let cancelBody: unknown;
    server.use(
      http.post('*/public/tournament-registrations/cancel', async ({ request }) => {
        cancelBody = await request.json();
        return HttpResponse.json({
          registration: pairRegistration({ status: 'cancelled' }),
          refunded: true,
          refund: { status: 'issued', message: 'Te devolvimos lo que pagaste.', amount: 2_200_000 },
        });
      }),
    );
    open('?token=tok-1&registered=1');
    const user = userEvent.setup();
    await user.click(await screen.findByRole('button', { name: t.tournaments.cancelPair }));
    await user.click(await screen.findByRole('button', { name: t.events.cancelConfirmYes }));

    expect(await screen.findByText('Te devolvimos lo que pagaste.')).toBeInTheDocument();
    expect(cancelBody).toEqual({ token: 'tok-1' });
  });

  it('shows an expired spot with a way to sign up again', async () => {
    serveRegistration(
      pairRegistration({
        status: 'expired',
        cancellation: { can_cancel: false, within_window: false, deadline: '2099-12-04T19:00:00-03:00', walkover: false },
      }),
    );
    open('?token=tok-1&registered=1');
    expect(await screen.findByText(t.events.expiredTitle)).toBeInTheDocument();
    expect(screen.getByRole('link', { name: t.events.expiredAction })).toHaveAttribute('href', '/c/test-club/tournaments/t1');
  });

  it('says a pair that already left is out of the tournament', async () => {
    serveRegistration(pairRegistration({ pair_status: 'withdrawn' }));
    open('?token=tok-1&registered=1');
    expect(await screen.findByText(t.tournaments.withdrawnTitle)).toBeInTheDocument();
  });

});

describe('TournamentPage checkout return, failures', () => {
  it('shows a link that does not resolve', async () => {
    server.use(http.get('*/public/tournament-registrations/status', () => new HttpResponse(null, { status: 404 })));
    open('?token=nope&registered=1');
    expect(await screen.findByText(t.events.registrationNotFoundDescription)).toBeInTheDocument();
  });

  it('on a failed payment releases the held spot with the stored token and brings the page back', async () => {
    serveTournament();
    storeTournamentRegistrationToken('t1', 'tok-held');
    let cancelBody: unknown;
    server.use(
      http.post('*/public/tournament-registrations/cancel', async ({ request }) => {
        cancelBody = await request.json();
        return HttpResponse.json({
          registration: pairRegistration({ status: 'cancelled' }),
          refunded: false,
          refund: { status: 'none', message: 'Sin pagos para devolver.' },
        });
      }),
    );
    open('?error=payment_failed');
    const user = userEvent.setup();
    expect(await screen.findByText(t.publicBooking.paymentFailed)).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: t.events.paymentFailedAction }));

    expect(await screen.findByRole('heading', { level: 1, name: TOURNAMENT.name })).toBeInTheDocument();
    expect(cancelBody).toEqual({ token: 'tok-held' });
    expect(window.location.search).toBe('');
  });
});
