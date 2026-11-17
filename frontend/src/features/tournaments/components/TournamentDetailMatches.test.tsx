import { tournamentTitle } from '@/shared/lib/traitLabels';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderWithProviders, screen, userEvent, waitFor } from '@/test/test-utils';
import { ES_AR } from '@/shared/i18n/es_AR';
import { makeCategory, makeDrawnDetail, makeMatch } from '../testing/fixtures';
import { menuLabels, openMatchMenu } from '../testing/matchMenu';
import { pickSection } from '../testing/sectionSelect';
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
  await screen.findByRole('heading', { name: tournamentTitle(detail.tournament.sport) });
  const user = userEvent.setup();
  await pickSection(user, t.detail.sections.matches);
  return { tournamentsApi, user };
}

describe('TournamentDetail: matches', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('lists the matches with their court and time, and one actions menu for each', async () => {
    const detail = makeDrawnDetail();
    detail.matches = [
      makeMatch({ id: 'gm1', stage: 'group', groupId: 'g1', pair1Id: 'p1', pair2Id: 'p2', schedule: SCHEDULE }),
      makeMatch({ id: 'gm2', stage: 'group', groupId: 'g1', round: 2, pair1Id: 'p1', pair2Id: 'p3' }),
    ];
    const { user } = await renderMatches(detail);

    expect(screen.getByText(/Cancha 1/)).toBeInTheDocument();
    expect(screen.getByText(t.matches.unscheduled)).toBeInTheDocument();
    expect(screen.getAllByRole('button', { name: t.matches.actionsLabel })).toHaveLength(2);
    expect(screen.queryByRole('button', { name: t.matches.loadResult })).not.toBeInTheDocument();

    await openMatchMenu(user, 0);
    expect(menuLabels()).toEqual([t.matches.reschedule, t.matches.unschedule, t.matches.loadResult]);
  });

  it('shows the score of a played match and offers to correct it in its menu', async () => {
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
    const { user } = await renderMatches(detail);

    expect(screen.getByText('6-4 6-3')).toBeInTheDocument();
    await openMatchMenu(user, 0);
    expect(menuLabels()).toEqual([t.matches.correctResult]);
  });
});

describe('TournamentDetail: correcting a result', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('offers no correction for a match whose next match already has a result, and says nothing about it', async () => {
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
    const { user } = await renderMatches(detail);

    // The locked semifinal has no menu at all; the final still open to correction has one.
    expect(screen.getAllByRole('button', { name: t.matches.actionsLabel })).toHaveLength(1);
    await openMatchMenu(user, 0);
    expect(menuLabels()).toEqual([t.matches.correctResult]);
    expect(screen.queryByText(t.matches.correctLocked)).not.toBeInTheDocument();
  });
});

describe('TournamentDetail: match actions', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('lists only the actions each match has now, the court fee of each pair included', async () => {
    const detail = makeDrawnDetail();
    detail.matches = [
      makeMatch({
        id: 'gm1',
        stage: 'group',
        groupId: 'g1',
        pair1Id: 'p1',
        pair2Id: 'p2',
        schedule: SCHEDULE,
        courtFee: { due: 100000 },
      }),
      makeMatch({ id: 'gm2', stage: 'group', groupId: 'g1', round: 2, pair1Id: 'p1', pair2Id: 'p3' }),
    ];
    detail.categories = [
      makeCategory({ status: 'closed', draw_generated: true, confirmed_count: 4, court_fee: 100000 }),
    ];
    const { user } = await renderMatches(detail);

    await openMatchMenu(user, 0);
    expect(menuLabels()).toEqual([
      t.matches.reschedule,
      t.matches.unschedule,
      t.matches.loadResult,
      t.courtFee.chargeAria('Pérez / Gómez'),
      t.courtFee.chargeAria('Ruiz / Soto'),
    ]);
    await user.keyboard('{Escape}');

    await openMatchMenu(user, 1);
    expect(menuLabels()).toEqual([t.matches.schedule, t.matches.loadResult]);
  });
});

describe('TournamentDetail: match readiness and scheduling', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('does not list a bye, and offers no result before both pairs are known', async () => {
    const detail = makeDrawnDetail();
    detail.matches = [
      makeMatch({ id: 'bye', bye: true, status: 'completed', pair1Id: 'p1', round: 1, position: 2 }),
      makeMatch({ id: 'b1', round: 1, position: 1, source1: { group_id: 'g1', rank: 1 } }),
    ];
    const { user } = await renderMatches(detail);

    expect(screen.queryByText(t.bracket.bye)).not.toBeInTheDocument();
    expect(screen.getByText(t.matches.notReady)).toBeInTheDocument();
    await openMatchMenu(user, 0);
    expect(menuLabels()).toEqual([t.matches.schedule]);
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

    await openMatchMenu(user, 0);
    await user.click(await screen.findByRole('menuitem', { name: t.matches.unschedule }));

    await waitFor(() => {
      expect(tournamentsApi.unschedule).toHaveBeenCalledWith('c1', 't1', 'gm1');
    });
  });
});
