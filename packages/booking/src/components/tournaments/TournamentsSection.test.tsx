import { describe, it, expect } from 'vitest';
import { screen } from '@testing-library/react';
import { renderBooking } from '../../test/booking';
import { CAT_GROUPS, SUMMARY } from '../../test/tournamentFixtures';
import { BOOKING_MESSAGES } from '../../messages';
import { TournamentsSection } from './TournamentsSection';

const t = BOOKING_MESSAGES.tournaments;

describe('TournamentsSection', () => {
  it('lists each tournament with its categories and a link to its page', () => {
    renderBooking(<TournamentsSection tournaments={[SUMMARY]} slug="test-club" />);

    expect(screen.getByRole('heading', { level: 3, name: t.sectionTitle })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: SUMMARY.name })).toBeInTheDocument();
    expect(screen.getByText('8va')).toBeInTheDocument();
    expect(screen.getByText(/\$20\.000/)).toHaveTextContent('$20.000 por pareja');
    expect(screen.queryByText(/cargo de servicio/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/\$22\.000/)).not.toBeInTheDocument();
    expect(screen.getByRole('link', { name: new RegExp(t.viewTournament) })).toHaveAttribute(
      'href',
      '/c/test-club/tournaments/t1',
    );
  });

  it('writes a range inside one month as its days and the short month', () => {
    renderBooking(<TournamentsSection tournaments={[SUMMARY]} slug="test-club" />);

    expect(screen.getByText('5 al 6 dic')).toBeInTheDocument();
    expect(screen.queryByText(/Sábado/)).not.toBeInTheDocument();
  });

  it('shows a category as its name and price on one line, and its format under it', () => {
    renderBooking(<TournamentsSection tournaments={[SUMMARY]} slug="test-club" />);

    const price = screen.getByText(/\$20\.000/);
    expect(price).toHaveClass('whitespace-nowrap');
    expect(screen.getAllByText(t.formatGroups)[0]).toBeInTheDocument();
  });

  it('marks a closed category as closed when it is given one', () => {
    renderBooking(<TournamentsSection tournaments={[{ ...SUMMARY, categories: [CAT_GROUPS] }]} slug="test-club" />);

    expect(screen.getByText(t.categoryStatusClosed)).toBeInTheDocument();
  });
});
