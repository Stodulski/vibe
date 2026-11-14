import { describe, it, expect } from 'vitest';
import { screen } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import { server } from '../../test/msw/server';
import { renderBooking } from '../../test/booking';
import { SUMMARY } from '../../test/tournamentFixtures';
import { BOOKING_MESSAGES } from '../../messages';
import { TournamentsSection } from './TournamentsSection';

const t = BOOKING_MESSAGES.tournaments;

describe('TournamentsSection', () => {
  it('renders nothing when the complex has no upcoming tournaments', async () => {
    const { container } = renderBooking(<TournamentsSection slug="test-club" />);
    await new Promise((resolve) => setTimeout(resolve, 50));
    expect(container).toBeEmptyDOMElement();
  });

  it('renders nothing when the list fails to load', async () => {
    server.use(http.get('*/public/complexes/:slug/tournaments', () => new HttpResponse(null, { status: 500 })));
    const { container } = renderBooking(<TournamentsSection slug="test-club" />);
    await new Promise((resolve) => setTimeout(resolve, 50));
    expect(container).toBeEmptyDOMElement();
  });

  it('lists each tournament with its categories and a link to its page', async () => {
    server.use(http.get('*/public/complexes/:slug/tournaments', () => HttpResponse.json({ tournaments: [SUMMARY] })));
    renderBooking(<TournamentsSection slug="test-club" />);

    expect(await screen.findByRole('heading', { name: t.sectionTitle })).toBeInTheDocument();
    expect(screen.getByText(SUMMARY.name)).toBeInTheDocument();
    expect(screen.getByText('8va')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: new RegExp(t.viewTournament) })).toHaveAttribute('href', '/c/test-club/tournaments/t1');
  });
});
