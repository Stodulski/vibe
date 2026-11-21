import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderWithProviders, screen, userEvent, waitFor, within } from '@/test/test-utils';
import { ES_AR } from '@/shared/i18n/es_AR';
import { makeDetail, makePair } from '../testing/fixtures';
import { TournamentDetailContent } from '../pages/TournamentDetailPage';
import type { TournamentDetailResponse } from '@/shared/types/api.types';

vi.mock('../api/tournaments.api', () => ({
  tournamentsApi: { get: vi.fn(), withdrawPair: vi.fn() },
}));
vi.mock('@/features/courts', () => ({ useCourts: () => ({ data: [] }) }));
vi.mock('sonner', () => ({ toast: { error: vi.fn(), success: vi.fn() } }));

const t = ES_AR.tournaments.pairs;

const PAIRS = [
  makePair({ id: 'p1', players: ['Ana Pérez', 'Bea Gómez'], registration: 'confirmed' }),
  makePair({ id: 'p2', players: ['Carla Ruiz', 'Dana Soto'], registration: 'held' }),
  makePair({ id: 'p3', players: ['Eva Luna', 'Flor Paz'], registration: 'cancelled' }),
];

async function renderPairs(detail: TournamentDetailResponse = makeDetail({ pairs: PAIRS })) {
  const { tournamentsApi } = await import('../api/tournaments.api');
  vi.mocked(tournamentsApi.get).mockResolvedValue(detail);
  renderWithProviders(<TournamentDetailContent complexId="c1" tournamentId="t1" />);
  const list = await screen.findByRole('list', { name: ES_AR.tournaments.detail.sections.pairs });
  return { tournamentsApi, list, user: userEvent.setup() };
}

/** Opens the "⋯" menu of the row naming `player`. */
async function openRowMenu(user: ReturnType<typeof userEvent.setup>, list: HTMLElement, player: string) {
  const row = within(list)
    .getAllByRole('listitem')
    .find((item) => item.textContent.includes(player));
  if (!row) throw new Error(`no row for ${player}`);
  await user.click(within(row).getByRole('button', { name: t.actions }));
}

describe('TournamentDetail: withdrawing a pair', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('says a paid pair gets everything back, and withdraws it by its registration', async () => {
    const { tournamentsApi, list, user } = await renderPairs();
    vi.mocked(tournamentsApi.withdrawPair).mockResolvedValue({
      registration_id: 'reg-p1',
      refund: { status: 'issued', message: 'ok', amount: 2100000 },
    });

    await openRowMenu(user, list, 'Ana Pérez');
    await user.click(await screen.findByRole('menuitem', { name: t.withdraw }));
    const dialog = await screen.findByRole('alertdialog');
    expect(dialog).toHaveTextContent(t.withdrawDescriptionPaid);
    await user.click(within(dialog).getByRole('button', { name: t.withdrawConfirm }));

    await waitFor(() => {
      expect(tournamentsApi.withdrawPair).toHaveBeenCalledWith('c1', 't1', 'cat1', 'reg-p1');
    });
  });

  it('says a pair still paying has nothing to get back', async () => {
    const { list, user } = await renderPairs();

    await openRowMenu(user, list, 'Carla Ruiz');
    await user.click(await screen.findByRole('menuitem', { name: t.withdraw }));
    expect(await screen.findByRole('alertdialog')).toHaveTextContent(t.withdrawDescriptionUnpaid);
  });

  it('offers no withdrawal for a pair whose spot is already over', async () => {
    const { list, user } = await renderPairs();

    await openRowMenu(user, list, 'Eva Luna');
    await screen.findAllByRole('menuitem');
    expect(screen.queryByRole('menuitem', { name: t.withdraw })).not.toBeInTheDocument();
  });

  it('offers no withdrawal in a cancelled category', async () => {
    const detail = makeDetail({ pairs: PAIRS });
    const category = detail.categories[0];
    if (!category) throw new Error('no category');
    detail.categories = [{ ...category, status: 'cancelled' }];
    const { list, user } = await renderPairs(detail);

    await openRowMenu(user, list, 'Ana Pérez');
    await screen.findAllByRole('menuitem');
    expect(screen.queryByRole('menuitem', { name: t.withdraw })).not.toBeInTheDocument();
  });
});
