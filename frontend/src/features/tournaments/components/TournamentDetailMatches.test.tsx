import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderWithProviders, screen, userEvent, waitFor } from '@/test/test-utils';
import { ES_AR } from '@/shared/i18n/es_AR';
import { makeDrawnDetail, makeMatch } from '../testing/fixtures';
import { TournamentDetailContent } from '../pages/TournamentDetailPage';
import type { TournamentDetailResponse } from '@/shared/types/api.types';

vi.mock('../api/tournaments.api', () => ({
  tournamentsApi: { get: vi.fn(), cancel: vi.fn(), cancelCategory: vi.fn(), unschedule: vi.fn(), setResult: vi.fn() },
}));
vi.mock('@/features/courts', () => ({ useCourts: () => ({ data: [] }) }));
vi.mock('sonner', () => ({ toast: { error: vi.fn(), success: vi.fn() } }));

const t = ES_AR.tournaments;
const SCHEDULE = {
  court_id: 'ct1',
  court_name: 'Cancha 1',
  date: '2099-01-03',
  start_time: '18:00',
  end_time: '19:30',
  duration_minutes: 90,
};

async function renderMatches(detail: TournamentDetailResponse) {
  const { tournamentsApi } = await import('../api/tournaments.api');
  vi.mocked(tournamentsApi.get).mockResolvedValue(detail);
  renderWithProviders(<TournamentDetailContent complexId="c1" tournamentId="t1" />);
  await screen.findByRole('heading', { name: detail.tournament.name });
  const user = userEvent.setup();
  await user.click(screen.getByRole('button', { name: t.detail.sections.matches }));
  return { tournamentsApi, user };
}

describe('TournamentDetail: matches', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('lists the matches with their court and time, or that they have none yet', async () => {
    const detail = makeDrawnDetail();
    detail.matches = [
      makeMatch({ id: 'gm1', stage: 'group', groupId: 'g1', pair1Id: 'p1', pair2Id: 'p2', schedule: SCHEDULE }),
      makeMatch({ id: 'gm2', stage: 'group', groupId: 'g1', round: 2, pair1Id: 'p1', pair2Id: 'p3' }),
    ];
    await renderMatches(detail);

    expect(screen.getByText(/Cancha 1/)).toBeInTheDocument();
    expect(screen.getByText(t.matches.unscheduled)).toBeInTheDocument();
    expect(screen.getAllByRole('button', { name: t.matches.loadResult })).toHaveLength(2);
  });

  it('shows the score of a played match and offers to correct it', async () => {
    const detail = makeDrawnDetail();
    detail.matches = [
      makeMatch({
        id: 'gm1',
        stage: 'group',
        groupId: 'g1',
        pair1Id: 'p1',
        pair2Id: 'p2',
        status: 'completed',
        winnerPairId: 'p1',
        sets: [
          { a: 6, b: 4, super: false },
          { a: 6, b: 3, super: false },
        ],
      }),
    ];
    await renderMatches(detail);

    expect(screen.getByText('6-4 6-3')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: t.matches.correctResult })).toBeInTheDocument();
  });
});

describe('TournamentDetail: correcting a result', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('disables the correction, and says why, when the next match already has a result', async () => {
    const detail = makeDrawnDetail();
    detail.matches = [
      makeMatch({
        id: 's1',
        round: 1,
        position: 0,
        pair1Id: 'p1',
        pair2Id: 'p2',
        status: 'completed',
        winnerPairId: 'p1',
      }),
      makeMatch({
        id: 'f',
        round: 2,
        position: 0,
        pair1Id: 'p1',
        pair2Id: 'p3',
        status: 'completed',
        winnerPairId: 'p1',
      }),
    ];
    await renderMatches(detail);

    const buttons = screen.getAllByRole('button', { name: t.matches.correctResult });
    expect(buttons[0]).toBeDisabled();
    expect(buttons[1]).toBeEnabled();
    expect(screen.getAllByText(t.matches.correctLocked)).toHaveLength(1);
  });
});

describe('TournamentDetail: match readiness and scheduling', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('does not list a bye, and cannot load a result before both pairs are known', async () => {
    const detail = makeDrawnDetail();
    detail.matches = [
      makeMatch({ id: 'bye', bye: true, status: 'completed', pair1Id: 'p1', round: 1, position: 2 }),
      makeMatch({ id: 'b1', round: 1, position: 1, source1: { group_id: 'g1', rank: 1 } }),
    ];
    await renderMatches(detail);

    expect(screen.queryByText(t.bracket.bye)).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: t.matches.loadResult })).not.toBeInTheDocument();
    expect(screen.getByText(t.matches.notReady)).toBeInTheDocument();
  });

  it('takes a scheduled match off its court', async () => {
    const detail = makeDrawnDetail();
    const scheduled = makeMatch({
      id: 'gm1',
      stage: 'group',
      groupId: 'g1',
      pair1Id: 'p1',
      pair2Id: 'p2',
      schedule: SCHEDULE,
    });
    detail.matches = [scheduled];
    const { tournamentsApi, user } = await renderMatches(detail);
    vi.mocked(tournamentsApi.unschedule).mockResolvedValue({ match: scheduled });

    await user.click(screen.getByRole('button', { name: t.matches.unschedule }));

    await waitFor(() => {
      expect(tournamentsApi.unschedule).toHaveBeenCalledWith('c1', 't1', 'gm1');
    });
  });
});
