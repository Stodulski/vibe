import { describe, expect, it } from 'vitest';
import { screen } from '@testing-library/react';
import { renderBooking } from '../../test/booking';
import { CAT_GROUPS, CAT_OPEN, SUMMARY } from '../../test/tournamentFixtures';
import { BOOKING_MESSAGES } from '../../messages';
import { agendaRows } from './agendaModel';
import { TournamentsSection } from './TournamentsSection';

const t = BOOKING_MESSAGES.tournaments;

describe('TournamentsSection', () => {
  it('titles the agenda and lists the open categories with their sign-up', () => {
    renderBooking(<TournamentsSection rows={agendaRows([SUMMARY])} slug="test-club" />);

    expect(screen.getByRole('heading', { level: 3, name: t.sectionTitle })).toBeInTheDocument();
    expect(screen.getByRole('heading', { level: 3, name: CAT_OPEN.name })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: /Inscribirme/ })).toHaveAttribute(
      'href',
      `/c/test-club/tournaments/t1?category=${CAT_OPEN.id}`,
    );
  });

  it('leaves out a category whose sign-up is closed', () => {
    const rows = agendaRows([{ ...SUMMARY, categories: [CAT_GROUPS] }]);
    renderBooking(<TournamentsSection rows={rows} slug="test-club" />);

    expect(screen.queryByRole('heading', { name: CAT_GROUPS.name })).not.toBeInTheDocument();
    expect(screen.queryByRole('link', { name: /Inscribirme/ })).not.toBeInTheDocument();
  });
});
