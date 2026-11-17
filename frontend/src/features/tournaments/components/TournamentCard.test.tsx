import { describe, it, expect } from 'vitest';
import { renderWithProviders as render, screen, within } from '@/test/test-utils';
import { ES_AR } from '@/shared/i18n/es_AR';
import { makeCategory, makeDetail } from '../testing/fixtures';
import { TournamentCard } from './TournamentCard';
import type { TournamentOwnerSummary } from '@/shared/types/api.types';

const t = ES_AR.tournaments;

function summary(overrides: Partial<TournamentOwnerSummary> = {}): TournamentOwnerSummary {
  const { tournament } = makeDetail();
  return {
    ...tournament,
    id: 't1',
    status: 'active',
    start_date: '2099-01-02',
    end_date: '2099-01-02',
    unscheduled_count: 0,
    categories: [
      makeCategory({ confirmed_count: 6 }),
      makeCategory({ id: 'cat2', name: 'Caballeros 7ma', level: 7, confirmed_count: 4 }),
    ],
    ...overrides,
  };
}

describe('TournamentCard', () => {
  it('links the whole row to the tournament detail', () => {
    render(<TournamentCard tournament={summary()} />);

    expect(screen.getByRole('link')).toHaveAttribute('href', '/events/tournaments/t1');
  });

  it('puts the day the tournament starts before the kicker and the categories', () => {
    render(<TournamentCard tournament={summary()} />);

    const link = screen.getByRole('link');
    const time = link.querySelector('time');
    if (!time) throw new Error('expected a date block');
    expect(time).toHaveAttribute('datetime', '2099-01-02');
    const kicker = screen.getByText('Torneo de pádel');
    const title = screen.getByText('Caballeros 6ta, Caballeros 7ma');
    expect(time.compareDocumentPosition(kicker) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    expect(kicker.compareDocumentPosition(title) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  });

  it('shows the days, the categories and the pairs as the meta line, the days capitalised', () => {
    render(<TournamentCard tournament={summary()} />);

    const link = screen.getByRole('link');
    expect(within(link).getByText('viernes 2 de enero')).toHaveClass('first-letter:uppercase');
    expect(within(link).getByText(t.categoriesCount(2))).toBeInTheDocument();
    expect(within(link).getByText(t.pairsCount(10))).toBeInTheDocument();
  });

  it('marks a cancelled tournament beside its title', () => {
    render(<TournamentCard tournament={summary({ status: 'cancelled' })} />);

    expect(within(screen.getByRole('link')).getByText(t.cancelledBadge)).toHaveClass('text-error-text');
  });
});
