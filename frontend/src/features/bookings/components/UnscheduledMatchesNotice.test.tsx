import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderWithProviders, screen, waitFor } from '@/test/test-utils';
import { ES_AR } from '@/shared/i18n/es_AR';
import { UnscheduledMatchesNotice } from './UnscheduledMatchesNotice';
import { makeDetail } from '@/features/tournaments/testing/fixtures';
import type { TournamentOwnerSummary } from '@/shared/types/api.types';

vi.mock('@/features/tournaments/api/tournaments.api', () => ({ tournamentsApi: { list: vi.fn() } }));

const t = ES_AR.bookings.unscheduledMatches;

function tournament(
  id: string,
  name: string,
  unscheduled: number,
  status: 'active' | 'cancelled' = 'active',
): TournamentOwnerSummary {
  return { ...makeDetail().tournament, id, name, status, categories: [], unscheduled_count: unscheduled };
}

async function mockList(tournaments: TournamentOwnerSummary[]) {
  const { tournamentsApi } = await import('@/features/tournaments/api/tournaments.api');
  vi.mocked(tournamentsApi.list).mockResolvedValue({ tournaments });
  return tournamentsApi;
}

describe('UnscheduledMatchesNotice', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('says how many matches of each tournament on the day have no court and time, with a link to schedule them', async () => {
    const api = await mockList([tournament('t1', 'Copa Vibe', 3), tournament('t2', 'Liga', 1)]);
    renderWithProviders(<UnscheduledMatchesNotice complexId="c1" date="2099-01-05" />);

    expect(await screen.findByText(t.message('Copa Vibe', 3))).toBeInTheDocument();
    expect(screen.getByText('Liga: 1 partido sin cancha ni horario')).toBeInTheDocument();
    expect(screen.getByText('Copa Vibe: 3 partidos sin cancha ni horario')).toBeInTheDocument();
    const links = screen.getAllByRole('link', { name: new RegExp(t.schedule) });
    expect(links.map((l) => l.getAttribute('href'))).toEqual(['/events/tournaments/t1', '/events/tournaments/t2']);
    expect(api.list).toHaveBeenCalledWith('c1', '2099-01-05', '2099-01-05', expect.anything());
  });

  it('shows nothing for a tournament with every match scheduled, or a cancelled one', async () => {
    await mockList([tournament('t1', 'Copa Vibe', 0), tournament('t2', 'Liga', 0, 'cancelled')]);
    const { container } = renderWithProviders(<UnscheduledMatchesNotice complexId="c1" date="2099-01-05" />);

    await waitFor(() => {
      expect(container).toBeEmptyDOMElement();
    });
  });

  it('renders nothing while it loads', async () => {
    const api = await mockList([]);
    vi.mocked(api.list).mockReturnValue(new Promise(() => undefined));
    const { container } = renderWithProviders(<UnscheduledMatchesNotice complexId="c1" date="2099-01-05" />);

    expect(container).toBeEmptyDOMElement();
  });
});
