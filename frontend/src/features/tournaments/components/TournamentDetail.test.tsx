import { tournamentTitle } from '@/shared/lib/traitLabels';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderWithProviders, screen, userEvent, waitFor, within } from '@/test/test-utils';
import { ES_AR } from '@/shared/i18n/es_AR';
import { makeDetail, makeDrawnDetail, makePair } from '../testing/fixtures';
import { TournamentDetailContent } from '../pages/TournamentDetailPage';
import type { TournamentDetailResponse } from '@/shared/types/api.types';

vi.mock('../api/tournaments.api', () => ({
  tournamentsApi: {
    get: vi.fn(),
    generateDraw: vi.fn(),
    moveDraw: vi.fn(),
    cancel: vi.fn(),
    cancelCategory: vi.fn(),
    unschedule: vi.fn(),
  },
}));
vi.mock('@/features/courts', () => ({ useCourts: () => ({ data: [] }) }));
vi.mock('sonner', () => ({ toast: { error: vi.fn(), success: vi.fn() } }));

const t = ES_AR.tournaments;

async function renderDetail(detail: TournamentDetailResponse) {
  const { tournamentsApi } = await import('../api/tournaments.api');
  vi.mocked(tournamentsApi.get).mockResolvedValue(detail);
  renderWithProviders(<TournamentDetailContent complexId="c1" tournamentId="t1" />);
  await screen.findByRole('heading', { name: tournamentTitle(detail.tournament.sport) });
  return { tournamentsApi, user: userEvent.setup() };
}

const openDetail = () =>
  makeDetail({
    pairs: [
      makePair({ id: 'p1', players: ['Ana Pérez', 'Bea Gómez'] }),
      makePair({ id: 'p2', players: ['Carla Ruiz', 'Dana Soto'], registration: 'held' }),
      makePair({ id: 'p3', players: ['Eva Díaz', 'Flor Vega'] }),
    ],
  });

describe('TournamentDetail: pairs', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('lists each pair with its players and state, and keeps the phones behind its actions menu', async () => {
    const { user } = await renderDetail(openDetail());

    const list = screen.getByRole('list', { name: t.detail.sections.pairs });
    expect(within(list).getAllByRole('listitem')).toHaveLength(3);
    expect(list).toHaveTextContent('Ana Pérez / Bea Gómez');
    // Paid at the category price: no badge for the two confirmed pairs; the held one names its state.
    expect(within(list).queryByText(/^Pagó/)).not.toBeInTheDocument();
    expect(within(list).getByText(t.pairs.state.held)).toBeInTheDocument();
    expect(within(list).queryByText('1155550000')).not.toBeInTheDocument();

    const actions = within(list).getAllByRole('button', { name: t.pairs.actions })[0];
    if (!actions) throw new Error('the first pair has no actions menu');
    await user.click(actions);
    expect(await screen.findByRole('menuitem', { name: t.pairs.call('Ana Pérez') })).toHaveAttribute(
      'href',
      'tel:1155550000',
    );
  });
});

describe('TournamentDetail: draw', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('draws after a confirmation, sending no seed order and offering no seed control', async () => {
    const { tournamentsApi, user } = await renderDetail(openDetail());
    vi.mocked(tournamentsApi.generateDraw).mockResolvedValue(makeDrawnDetail());

    expect(screen.queryByRole('button', { name: /cabezas de serie/i })).not.toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: t.draw.generate }));
    await user.click(within(await screen.findByRole('alertdialog')).getByRole('button', { name: t.draw.confirmLabel }));

    await waitFor(() => {
      expect(tournamentsApi.generateDraw).toHaveBeenCalledWith('c1', 't1', 'cat1', {});
    });
  });

  it.each([0, 1])('offers no draw while only %i pairs paid', async (count) => {
    const pairs = [makePair({ id: 'p1' })].slice(0, count);
    await renderDetail(makeDetail({ pairs }));

    expect(screen.queryByRole('heading', { name: t.draw.title })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: t.draw.generate })).not.toBeInTheDocument();
  });
});
