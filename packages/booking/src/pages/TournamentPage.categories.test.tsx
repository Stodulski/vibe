import { describe, it, expect } from 'vitest';
import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { server } from '../test/msw/server';
import { renderBooking, setBookingUrl } from '../test/booking';
import { CAT_OPEN, DETAIL } from '../test/tournamentFixtures';
import { BOOKING_MESSAGES } from '../messages';
import TournamentPage from './TournamentPage';

const t = BOOKING_MESSAGES;

function serveTournament(detail = DETAIL) {
  server.use(http.get('*/public/complexes/:slug/tournaments/:id', () => HttpResponse.json(detail)));
}

function open() {
  setBookingUrl('/c/test-club/tournaments/t1');
  return renderBooking(<TournamentPage slug="test-club" tournamentId="t1" />);
}

describe('TournamentPage categories', () => {
  it('lists the scheduled knockout matches in the schedule', async () => {
    const schedule = { court_id: 'k1', court_name: 'Cancha 1', date: '2099-12-06', start_time: '16:00', end_time: '17:30', duration_minutes: 90 };
    serveTournament({ ...DETAIL, matches: DETAIL.matches.map((m) => (m.id === 'b-f' ? { ...m, schedule } : m)) });
    open();
    expect(await screen.findByRole('heading', { name: t.tournaments.scheduleTitle })).toBeInTheDocument();
  });

  it('lists each group match once: in its zone, not again in the schedule', async () => {
    serveTournament();
    open();
    await screen.findByRole('heading', { name: 'Zona A' });

    expect(screen.getAllByText('6-4 6-3')).toHaveLength(1);
  });

  it('shows only the cancelled state of a cancelled category, with no groups or bracket', async () => {
    serveTournament({ ...DETAIL, categories: DETAIL.categories.map((c) => (c.id === 'c-groups' ? { ...c, status: 'cancelled' } : c)) });
    open();

    expect(await screen.findByText(t.tournaments.categoryCancelled)).toBeInTheDocument();
    expect(screen.queryByRole('heading', { name: 'Zona A' })).not.toBeInTheDocument();
    expect(screen.queryByRole('heading', { name: t.tournaments.groupsTitle })).not.toBeInTheDocument();
    // Only the elimination category keeps its bracket.
    expect(screen.getAllByRole('region', { name: t.tournaments.bracketTitle })).toHaveLength(1);
  });

  it('demotes the other categories sign-up while one form is open', async () => {
    const second = { ...CAT_OPEN, id: 'c-open-2', name: '9na' };
    serveTournament({ ...DETAIL, categories: [CAT_OPEN, second] });
    open();
    const user = userEvent.setup();
    const [first] = await screen.findAllByRole('button', { name: t.tournaments.signUp });
    if (!first) throw new Error('no sign-up button');
    expect(first.className).toMatch(/(^|\s)bg-primary/);
    await user.click(first);

    const others = screen.getAllByRole('button', { name: t.tournaments.signUp });
    expect(others).toHaveLength(1);
    expect(others[0]?.className).not.toMatch(/(^|\s)bg-primary/);
  });

});
