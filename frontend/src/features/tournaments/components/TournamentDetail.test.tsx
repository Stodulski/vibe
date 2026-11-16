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
  await screen.findByRole('heading', { name: detail.tournament.name });
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

  it('lists each pair with its players, contact and payment state', async () => {
    await renderDetail(openDetail());

    const list = screen.getByRole('list', { name: t.detail.sections.pairs });
    expect(within(list).getAllByRole('listitem')).toHaveLength(3);
    expect(list).toHaveTextContent('Ana Pérez / Bea Gómez');
    expect(within(list).getAllByText(/^Pagó/)).toHaveLength(2);
    expect(within(list).getByText(t.pairs.state.held)).toBeInTheDocument();
    expect(within(list).getAllByRole('link', { name: /Llamar Ana Pérez/ })[0]).toHaveAttribute(
      'href',
      'tel:1155550000',
    );
  });

  it('says how the category plays: format, groups and match format', async () => {
    await renderDetail(openDetail());

    expect(screen.getByText(t.detail.groupsSetup(3, 2))).toBeInTheDocument();
    expect(screen.getByText(t.detail.matchSetup(2, 6, true))).toBeInTheDocument();
  });
});

describe('TournamentDetail: draw', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('draws after a confirmation, with no seed order when left at random', async () => {
    const { tournamentsApi, user } = await renderDetail(openDetail());
    vi.mocked(tournamentsApi.generateDraw).mockResolvedValue(makeDrawnDetail());

    await user.click(screen.getByRole('button', { name: t.draw.generate }));
    await user.click(within(await screen.findByRole('alertdialog')).getByRole('button', { name: t.draw.confirmLabel }));

    await waitFor(() => {
      expect(tournamentsApi.generateDraw).toHaveBeenCalledWith('c1', 't1', 'cat1', {});
    });
  });

  it('sends the seed order of the paid pairs once the owner orders them', async () => {
    const { tournamentsApi, user } = await renderDetail(openDetail());
    vi.mocked(tournamentsApi.generateDraw).mockResolvedValue(makeDrawnDetail());

    await user.click(screen.getByRole('button', { name: t.draw.seedOrderOn }));
    await user.click(screen.getByRole('button', { name: t.draw.moveUp('Díaz / Vega') }));
    await user.click(screen.getByRole('button', { name: t.draw.generate }));
    await user.click(within(await screen.findByRole('alertdialog')).getByRole('button', { name: t.draw.confirmLabel }));

    await waitFor(() => {
      expect(tournamentsApi.generateDraw).toHaveBeenCalledWith('c1', 't1', 'cat1', { seed_order: ['p3', 'p1'] });
    });
  });

  it('says in Spanish that fewer than two paid pairs cannot be drawn, without calling the API', async () => {
    const { tournamentsApi, user } = await renderDetail(makeDetail({ pairs: [makePair({ id: 'p1' })] }));

    expect(screen.getByRole('button', { name: t.draw.generate })).toBeEnabled();
    await user.click(screen.getByRole('button', { name: t.draw.generate }));

    expect(await screen.findByText(t.draw.errors.notEnough)).toBeInTheDocument();
    expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument();
    expect(tournamentsApi.generateDraw).not.toHaveBeenCalled();
  });
});
