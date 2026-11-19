import { tournamentTitle } from '@/shared/lib/traitLabels';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderWithProviders, screen, userEvent, waitFor, within } from '@/test/test-utils';
import { ES_AR } from '@/shared/i18n/es_AR';
import { makeDrawnDetail, makeMatch } from '../testing/fixtures';
import { pickSection } from '../testing/sectionSelect';
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
  await screen.findByRole('heading', { name: tournamentTitle(detail.tournament.sport) });
  const user = userEvent.setup();
  if (section) await pickSection(user, section);
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

  it('draws every round under its heading in one region', async () => {
    await renderDetail(makeDrawnDetail(), t.detail.sections.bracket);

    const region = screen.getByRole('region', { name: t.bracket.label });
    expect(within(region).getByRole('heading', { name: t.bracket.semifinal })).toBeInTheDocument();
    expect(within(region).getByRole('heading', { name: t.bracket.final })).toBeInTheDocument();
    expect(region).toHaveTextContent(t.bracket.sourceOf(1, 'A'));
    expect(region).toHaveTextContent(t.bracket.sourceOf(2, 'B'));
  });
});

describe('TournamentDetail: phone layout', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });
  it('marks a qualifier with a check and a label that reaches assistive technology', async () => {
    await renderDetail(makeDrawnDetail(), t.detail.sections.groups);

    const table = screen.getByRole('table', { name: t.groups.table('A') });
    const qualifying = within(table).getAllByRole('row').slice(1)[0];
    if (!qualifying) throw new Error('the zone has no rows');
    const label = within(qualifying).getByText(t.groups.qualifies);
    expect(label).toBeInTheDocument();
    expect(qualifying.querySelector('svg')).toHaveAttribute('aria-hidden', 'true');
  });
});

/** Opens the actions menu of the pair whose row holds `text`. */
async function openPairMenu(user: ReturnType<typeof userEvent.setup>, text: string) {
  const list = screen.getByRole('list', { name: t.detail.sections.pairs });
  const row = within(list)
    .getAllByRole('listitem')
    .find((item) => item.textContent.includes(text));
  if (!row) throw new Error(`no pair row holds ${text}`);
  await user.click(within(row).getByRole('button', { name: t.pairs.actions }));
}

describe('TournamentDetail: swapping pairs', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('swaps two pairs of different groups', async () => {
    const { tournamentsApi, user } = await renderDetail(makeDrawnDetail(), t.detail.sections.pairs);
    vi.mocked(tournamentsApi.moveDraw).mockResolvedValue(makeDrawnDetail());

    await openPairMenu(user, 'Ana Pérez');
    await user.click(await screen.findByRole('menuitem', { name: t.pairs.swap }));
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
    const { user } = await renderDetail(detail, t.detail.sections.pairs);

    await openPairMenu(user, 'Ana Pérez');
    expect(await screen.findByRole('menuitem', { name: t.pairs.call('Ana Pérez') })).toBeInTheDocument();
    expect(screen.queryByRole('menuitem', { name: t.pairs.swap })).not.toBeInTheDocument();
    expect(screen.queryByText(t.draw.regenerateHint)).not.toBeInTheDocument();
  });

  it('tells the draw can be repeated while no result is loaded', async () => {
    await renderDetail(makeDrawnDetail(), t.detail.sections.pairs);

    expect(screen.getByText(t.draw.regenerateHint)).toBeInTheDocument();
  });

  it('shows no draw button and no note once a result is loaded', async () => {
    const detail = makeDrawnDetail();
    detail.matches = [
      makeMatch({ id: 'gm1', stage: 'group', groupId: 'g1', pair1Id: 'p1', pair2Id: 'p2', status: 'completed' }),
    ];
    await renderDetail(detail, t.detail.sections.pairs);

    expect(screen.queryByRole('button', { name: t.draw.regenerate })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: t.draw.generate })).not.toBeInTheDocument();
    expect(screen.queryByText(t.draw.regenerateHint)).not.toBeInTheDocument();
  });
});
