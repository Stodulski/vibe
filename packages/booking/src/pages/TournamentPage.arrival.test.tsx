import { beforeAll, describe, it, expect, vi } from 'vitest';
import { screen, within } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import { server } from '../test/msw/server';
import { renderBooking, setBookingUrl } from '../test/booking';
import { DETAIL } from '../test/tournamentFixtures';
import { BOOKING_MESSAGES } from '../messages';
import TournamentPage from './TournamentPage';

const t = BOOKING_MESSAGES.tournaments;

beforeAll(() => {
  // jsdom has no layout, so it has no scrolling: the arrival scrolls to the category, which is only asked for.
  Element.prototype.scrollIntoView = vi.fn();
});

function open(search: string) {
  server.use(http.get('*/public/complexes/:slug/tournaments/:id', () => HttpResponse.json(DETAIL)));
  setBookingUrl(`/c/test-club/tournaments/t1${search}`);
  return renderBooking(<TournamentPage slug="test-club" tournamentId="t1" />);
}

describe('TournamentPage arrival from an agenda row', () => {
  it('opens the sign-up of the category it was asked for, without another press', async () => {
    open('?category=c-open');

    expect(within(await screen.findByRole('group', { name: t.payerTitle })).getByLabelText(/^Nombre/)).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: t.signUp })).not.toBeInTheDocument();
  });

  it('keeps every sign-up closed when the category it was asked for is not open', async () => {
    open('?category=c-groups');

    expect(await screen.findByRole('heading', { name: '6ta' })).toBeInTheDocument();
    expect(screen.queryByRole('group', { name: t.payerTitle })).not.toBeInTheDocument();
  });
});
