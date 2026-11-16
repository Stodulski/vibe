import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderWithProviders, screen, userEvent, waitFor, within } from '@/test/test-utils';
import { ES_AR } from '@/shared/i18n/es_AR';
import { makeDrawnDetail, makeMatch } from '../testing/fixtures';
import { TournamentDetailContent } from '../pages/TournamentDetailPage';
import type { TournamentDetailResponse } from '@/shared/types/api.types';

vi.mock('../api/tournaments.api', () => ({
  tournamentsApi: { get: vi.fn(), moveDraw: vi.fn(), cancel: vi.fn(), cancelCategory: vi.fn(), unschedule: vi.fn() },
}));
vi.mock('@/features/courts', () => ({ useCourts: () => ({ data: [] }) }));
vi.mock('sonner', () => ({ toast: { error: vi.fn(), success: vi.fn() } }));

const t = ES_AR.tournaments;

async function renderDetail(detail: TournamentDetailResponse, section?: string) {
  const { tournamentsApi } = await import('../api/tournaments.api');
  vi.mocked(tournamentsApi.get).mockResolvedValue(detail);
  renderWithProviders(<TournamentDetailContent complexId="c1" tournamentId="t1" />);
  await screen.findByRole('heading', { name: detail.tournament.name });
  const user = userEvent.setup();
  if (section) await user.click(screen.getByRole('button', { name: section }));
  return { tournamentsApi, user };
}

describe('TournamentDetail: groups', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('draws a standings table per group, best first, marking who qualifies', async () => {
    await renderDetail(makeDrawnDetail(), t.detail.sections.groups);

    const table = screen.getByRole('table', { name: t.groups.table('A') });
    const rows = within(table).getAllByRole('row').slice(1);
    expect(rows).toHaveLength(3);
    expect(rows[0]).toHaveTextContent('Pérez / Gómez');
    expect(rows[0]).toHaveTextContent(t.groups.qualifies);
    expect(rows[2]).not.toHaveTextContent(t.groups.qualifies);
    expect(screen.getByRole('table', { name: t.groups.table('B') })).toBeInTheDocument();
  });
});

describe('TournamentDetail: bracket', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('draws one column per round inside a region that scrolls on its own', async () => {
    await renderDetail(makeDrawnDetail(), t.detail.sections.bracket);

    const region = screen.getByRole('region', { name: t.bracket.label });
    expect(region.className).toContain('overflow-x-auto');
    expect(within(region).getByRole('heading', { name: t.bracket.semifinal })).toBeInTheDocument();
    expect(within(region).getByRole('heading', { name: t.bracket.final })).toBeInTheDocument();
    expect(region).toHaveTextContent(t.bracket.sourceOf(1, 'A'));
    expect(region).toHaveTextContent(t.bracket.sourceOf(2, 'B'));
  });
});

describe('TournamentDetail: swapping pairs', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('swaps two pairs of different groups', async () => {
    const { tournamentsApi, user } = await renderDetail(makeDrawnDetail());
    vi.mocked(tournamentsApi.moveDraw).mockResolvedValue(makeDrawnDetail());

    await user.click(screen.getByRole('button', { name: `${t.pairs.swap} Pérez / Gómez` }));
    await user.click(await screen.findByRole('combobox', { name: t.pairs.swapWith }));
    await user.click(await screen.findByRole('option', { name: /Luna \/ Paz/ }));
    await user.click(screen.getByRole('button', { name: t.pairs.swapConfirm }));

    await waitFor(() => {
      expect(tournamentsApi.moveDraw).toHaveBeenCalledWith('c1', 't1', 'cat1', { pair_id: 'p1', with_pair_id: 'p4' });
    });
  });

  it('no longer offers to swap once a result is loaded', async () => {
    const detail = makeDrawnDetail();
    detail.matches = [
      makeMatch({ id: 'gm1', stage: 'group', groupId: 'g1', pair1Id: 'p1', pair2Id: 'p2', status: 'completed' }),
    ];
    await renderDetail(detail);

    expect(screen.queryByRole('button', { name: /^Intercambiar/ })).not.toBeInTheDocument();
    expect(screen.getByText(t.pairs.swapLocked)).toBeInTheDocument();
  });

  it('tells the draw can be repeated, and only that, while no result is loaded', async () => {
    await renderDetail(makeDrawnDetail());

    expect(screen.getByText(t.draw.regenerateHint)).toBeInTheDocument();
    expect(screen.queryByText(t.pairs.swapLocked)).not.toBeInTheDocument();
  });

  it('shows one muted line and no draw button once a result is loaded', async () => {
    const detail = makeDrawnDetail();
    detail.matches = [
      makeMatch({ id: 'gm1', stage: 'group', groupId: 'g1', pair1Id: 'p1', pair2Id: 'p2', status: 'completed' }),
    ];
    await renderDetail(detail);

    expect(screen.getByText(t.pairs.swapLocked)).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: t.draw.regenerate })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: t.draw.generate })).not.toBeInTheDocument();
    expect(screen.queryByText(t.draw.regenerateHint)).not.toBeInTheDocument();
  });
});
