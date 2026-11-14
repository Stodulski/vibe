import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderWithProviders, screen, userEvent } from '@/test/test-utils';
import { ES_AR } from '@/shared/i18n/es_AR';
import { todayInArgentina } from '@/shared/lib/today';
import { makeCategory, makeDetail } from '../testing/fixtures';
import { pastRange, upcomingRange } from '../lib/tournamentRanges';
import TournamentsPage from './TournamentsPage';

vi.mock('../api/tournaments.api', () => ({ tournamentsApi: { list: vi.fn(), create: vi.fn() } }));
vi.mock('@/features/complex', () => ({ useSelectedComplex: () => ({ selectedComplexId: 'c1' }) }));

const t = ES_AR.tournaments;

function summary(id: string, name: string, status: 'active' | 'cancelled' = 'active') {
  const { tournament } = makeDetail();
  return {
    ...tournament,
    id,
    name,
    status,
    categories: [makeCategory({ confirmed_count: 6 }), makeCategory({ id: 'cat2', name: '7ma', confirmed_count: 4 })],
  };
}

describe('TournamentsPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('lists the upcoming tournaments with their categories and pairs, and links to the detail', async () => {
    const { tournamentsApi } = await import('../api/tournaments.api');
    vi.mocked(tournamentsApi.list).mockResolvedValue({ tournaments: [summary('t1', 'Copa Vibe')] });
    renderWithProviders(<TournamentsPage />);

    const card = await screen.findByRole('link', { name: /Copa Vibe/ });
    expect(card).toHaveAttribute('href', '/tournaments/t1');
    expect(card).toHaveTextContent('6ta, 7ma');
    expect(card).toHaveTextContent(t.pairsCount(10));
    const { from, to } = upcomingRange(todayInArgentina());
    expect(tournamentsApi.list).toHaveBeenCalledWith('c1', from, to, expect.anything());
  });

  it('marks a cancelled tournament', async () => {
    const { tournamentsApi } = await import('../api/tournaments.api');
    vi.mocked(tournamentsApi.list).mockResolvedValue({ tournaments: [summary('t1', 'Copa Vibe', 'cancelled')] });
    renderWithProviders(<TournamentsPage />);

    expect(await screen.findByText(t.cancelledBadge)).toBeInTheDocument();
  });

  it('invites creating a tournament when there is none, and loads the past ones only on their tab', async () => {
    const { tournamentsApi } = await import('../api/tournaments.api');
    vi.mocked(tournamentsApi.list).mockResolvedValue({ tournaments: [] });
    const user = userEvent.setup();
    renderWithProviders(<TournamentsPage />);

    expect(await screen.findByText(t.noUpcoming)).toBeInTheDocument();
    expect(tournamentsApi.list).toHaveBeenCalledTimes(1);

    await user.click(screen.getByRole('button', { name: t.past }));

    expect(await screen.findByText(t.noPast)).toBeInTheDocument();
    const { from, to } = pastRange(todayInArgentina());
    expect(tournamentsApi.list).toHaveBeenLastCalledWith('c1', from, to, expect.anything());
  });

  it('opens the create dialog from the header', async () => {
    const { tournamentsApi } = await import('../api/tournaments.api');
    vi.mocked(tournamentsApi.list).mockResolvedValue({ tournaments: [] });
    const user = userEvent.setup();
    renderWithProviders(<TournamentsPage />);

    await user.click(await screen.findByRole('button', { name: t.create, hidden: false }));

    expect(await screen.findByRole('dialog')).toHaveTextContent(t.form.title);
  });
});
