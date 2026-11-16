import { describe, it, expect } from 'vitest';
import { screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { server } from '../test/msw/server';
import { renderBooking, setBookingUrl } from '../test/booking';
import { CAT_OPEN, DETAIL } from '../test/tournamentFixtures';
import { BOOKING_MESSAGES } from '../messages';
import TournamentPage from './TournamentPage';

const t = BOOKING_MESSAGES;

function serveTournament() {
  server.use(http.get('*/public/complexes/:slug/tournaments/:id', () => HttpResponse.json(DETAIL)));
}

function open() {
  setBookingUrl('/c/test-club/tournaments/t1');
  return renderBooking(<TournamentPage slug="test-club" tournamentId="t1" />);
}

async function openForm(user: ReturnType<typeof userEvent.setup>) {
  await user.click(await screen.findByRole('button', { name: t.tournaments.signUp }));
}

describe('TournamentPage checkout parity with a booking', () => {
  it('shows the same green ticket as a booking above the pair form, with the same price rows and pay footer', async () => {
    serveTournament();
    open();
    const user = userEvent.setup();
    await openForm(user);

    const ticket = await screen.findByRole('region', { name: t.tournaments.summaryLabel });
    expect(within(ticket).getByText('Torneo de pádel')).toBeInTheDocument();
    expect(within(ticket).getByText(CAT_OPEN.name)).toBeInTheDocument();
    expect(within(ticket).getByText(t.serviceFee.label)).toBeInTheDocument();
    expect(within(ticket).getByText(t.publicBooking.totalOnline)).toBeInTheDocument();
    expect(screen.getAllByText(t.publicBooking.totalOnline)).toHaveLength(2);
    expect(screen.queryByText(/Una persona inscribe a los dos/)).not.toBeInTheDocument();
  });

  it('asks both phones with the booking helper text on the payer', async () => {
    serveTournament();
    open();
    const user = userEvent.setup();
    await openForm(user);
    expect(await screen.findByText(t.publicBooking.phoneHelper)).toBeInTheDocument();
  });
});
