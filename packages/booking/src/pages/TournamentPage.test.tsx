import { describe, it, expect, vi, beforeEach } from 'vitest';
import { screen } from '@testing-library/react';
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

describe('TournamentPage detail', () => {
  it('shows the tournament, each category and its format', async () => {
    serveTournament();
    open();

    expect(await screen.findByRole('heading', { level: 1, name: 'Torneo de pádel' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: '8va' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: '6ta' })).toBeInTheDocument();
    expect(screen.getAllByText('Al mejor de 3 sets \u00b7 súper tie-break').length).toBeGreaterThan(0);
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
    // Nothing in the knockout is scheduled yet: group matches are listed in their zone only.
    expect(screen.queryByRole('heading', { name: t.tournaments.scheduleTitle })).not.toBeInTheDocument();
  });

  it('offers the sign-up only on the category that is open, with its price', async () => {
    serveTournament();
    open();

    expect(await screen.findAllByRole('link', { name: t.tournaments.signUp })).toHaveLength(1);
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
    expect(screen.queryByRole('link', { name: t.tournaments.signUp })).not.toBeInTheDocument();
  });

  it('shows not found for an unknown tournament', async () => {
    server.use(http.get('*/public/complexes/:slug/tournaments/:id', () => new HttpResponse(null, { status: 404 })));
    open();
    expect(await screen.findByText(t.tournaments.notFound)).toBeInTheDocument();
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

describe('TournamentPage checkout return, summary', () => {
  it('shows what the pair paid instead of the availability, and a cancel button', async () => {
    serveRegistration(pairRegistration());
    open('?token=tok-1&registered=1');

    expect(await screen.findByText(`${t.tournaments.paidPrefix} $22.000`)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: t.tournaments.cancelPair })).toBeInTheDocument();
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

    expect(await screen.findByRole('heading', { level: 1, name: 'Torneo de pádel' })).toBeInTheDocument();
    expect(cancelBody).toEqual({ token: 'tok-held' });
    expect(window.location.search).toBe('');
  });
});
