import { describe, it, expect, vi, beforeEach } from 'vitest';
import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { server } from '../test/msw/server';
import { renderBooking, setBookingUrl } from '../test/booking';
import { CAT_OPEN, DETAIL, TOURNAMENT } from '../test/tournamentFixtures';
import { BOOKING_MESSAGES } from '../messages';
import { cancellationWindowText } from '../components/booking-confirmed/cancellationCopy';
import { navigateTo } from '../lib/navigation';
import TournamentSignUpPage from './TournamentSignUpPage';

vi.mock('../lib/navigation', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../lib/navigation')>()),
  navigateTo: vi.fn(),
}));

const t = BOOKING_MESSAGES;

function serveTournament(detail = DETAIL) {
  server.use(http.get('*/public/complexes/:slug/tournaments/:id', () => HttpResponse.json(detail)));
}

function open(categoryId = 'c-open') {
  setBookingUrl(`/c/test-club/tournaments/t1/inscripcion?category=${categoryId}`);
  return renderBooking(<TournamentSignUpPage slug="test-club" tournamentId="t1" />);
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

beforeEach(() => {
  vi.mocked(navigateTo).mockClear();
});

describe('TournamentSignUpPage layout', () => {
  it('shows the checkout steps with the category first, on the second step', async () => {
    serveTournament();
    open();

    const steps = await screen.findByRole('navigation', { name: t.publicBooking.stepProgress });
    expect(steps).toHaveTextContent(t.tournaments.stepCategory);
    expect(steps).toHaveTextContent(t.publicBooking.stepData);
    expect(steps).toHaveTextContent(t.publicBooking.stepPay);
    expect(steps.querySelector('[aria-current="step"]')).toHaveTextContent(t.publicBooking.stepData);
  });

  it('links back to the tournament page', async () => {
    serveTournament();
    open();

    expect(await screen.findByRole('link', { name: t.tournaments.backToTournamentLink })).toHaveAttribute(
      'href',
      '/c/test-club/tournaments/t1',
    );
  });

  it('shows the green ticket of the category above the pair form', async () => {
    serveTournament();
    open();

    const ticket = await screen.findByRole('region', { name: t.tournaments.summaryLabel });
    expect(within(ticket).getByText('Torneo de pádel')).toBeInTheDocument();
    expect(within(ticket).getByRole('heading', { level: 1, name: CAT_OPEN.name })).toBeInTheDocument();
    expect(within(ticket).getByText(t.courts.sportTypes.padel)).toBeInTheDocument();
    expect(within(ticket).getByText('Club Norte')).toBeInTheDocument();
    expect(within(ticket).getByText(t.serviceFee.label)).toBeInTheDocument();
    expect(within(ticket).getByText(cancellationWindowText(DETAIL.cancellation_hours) ?? '')).toBeInTheDocument();
    expect(screen.getAllByText(t.publicBooking.totalOnline)).toHaveLength(2);
  });

  it('asks both phones with the booking helper text on the payer', async () => {
    serveTournament();
    open();
    expect(await screen.findByText(t.publicBooking.phoneHelper)).toBeInTheDocument();
  });

  it('keeps the pay button out of reach for a category that is not open, and says why not', async () => {
    serveTournament();
    open('c-groups');

    expect(await screen.findByRole('link', { name: t.tournaments.backToTournamentLink })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: t.publicBooking.paySecurely })).not.toBeInTheDocument();
    expect(screen.queryByRole('group', { name: t.tournaments.payerTitle })).not.toBeInTheDocument();
  });

  it('keeps the pay button out of reach on a cancelled tournament', async () => {
    serveTournament({ ...DETAIL, tournament: { ...TOURNAMENT, status: 'cancelled' } });
    open();

    expect(await screen.findByText(t.tournaments.cancelledTitle)).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: t.publicBooking.paySecurely })).not.toBeInTheDocument();
  });
});

describe('TournamentSignUpPage sign-up validation', () => {
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
    await screen.findByRole('group', { name: t.tournaments.payerTitle });
    const user = userEvent.setup();
    await user.click(screen.getByRole('button', { name: t.publicBooking.paySecurely }));

    expect(await screen.findAllByText(t.validation.nameRequired)).toHaveLength(2);
    expect(post).not.toHaveBeenCalled();
  });

  it('refuses the same phone for both players', async () => {
    serveTournament();
    open();
    const user = userEvent.setup();
    await fillForm(user, '1123456789');
    await user.click(screen.getByRole('button', { name: t.publicBooking.paySecurely }));

    expect(await screen.findByText(t.tournaments.partnerPhoneSame)).toBeInTheDocument();
    expect(navigateTo).not.toHaveBeenCalled();
  });
});

describe('TournamentSignUpPage sign-up', () => {
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

describe('TournamentSignUpPage refused sign-up', () => {
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
    await fillForm(user);
    await user.click(screen.getByRole('button', { name: t.publicBooking.paySecurely }));

    expect(await screen.findByRole('alert')).toHaveTextContent(copy);
    expect(navigateTo).not.toHaveBeenCalled();
  });
});
