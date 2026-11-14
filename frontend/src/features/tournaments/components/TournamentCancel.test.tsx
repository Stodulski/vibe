import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderWithProviders, screen, userEvent, waitFor, within } from '@/test/test-utils';
import { ES_AR } from '@/shared/i18n/es_AR';
import { makeDetail, makePair } from '../testing/fixtures';
import { TournamentDetailContent } from '../pages/TournamentDetailPage';

vi.mock('../api/tournaments.api', () => ({
  tournamentsApi: { get: vi.fn(), cancel: vi.fn(), cancelCategory: vi.fn() },
}));
vi.mock('@/features/courts', () => ({ useCourts: () => ({ data: [] }) }));
vi.mock('sonner', () => ({ toast: { error: vi.fn(), success: vi.fn() } }));

const t = ES_AR.tournaments;
const paid = (id: string) => makePair({ id, registration: 'confirmed' });

async function renderCancel() {
  const { tournamentsApi } = await import('../api/tournaments.api');
  vi.mocked(tournamentsApi.get).mockResolvedValue(
    makeDetail({ pairs: [paid('p1'), paid('p2'), makePair({ id: 'p3', registration: 'held' })] }),
  );
  renderWithProviders(<TournamentDetailContent complexId="c1" tournamentId="t1" />);
  await screen.findByRole('heading', { name: 'Copa Vibe' });
  return { tournamentsApi, user: userEvent.setup() };
}

describe('TournamentDetail: cancelling', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('says how many paid pairs the tournament cancel refunds, and cancels', async () => {
    const { tournamentsApi, user } = await renderCancel();
    vi.mocked(tournamentsApi.cancel).mockResolvedValue({ tournament: makeDetail().tournament, refunds: [] });

    await user.click(screen.getByRole('button', { name: t.cancel.action }));
    const dialog = await screen.findByRole('alertdialog');
    expect(dialog).toHaveTextContent(t.cancel.confirmDescriptionPaid(2));
    await user.click(within(dialog).getByRole('button', { name: t.cancel.confirmLabel }));

    await waitFor(() => {
      expect(tournamentsApi.cancel).toHaveBeenCalledWith('c1', 't1');
    });
  });

  it('says how many paid pairs a category cancel refunds, and cancels only that category', async () => {
    const { tournamentsApi, user } = await renderCancel();
    vi.mocked(tournamentsApi.cancelCategory).mockResolvedValue({ refunds: [] });

    await user.click(screen.getByRole('button', { name: t.cancel.categoryAction }));
    const dialog = await screen.findByRole('alertdialog');
    expect(dialog).toHaveTextContent(t.cancel.categoryDescriptionPaid(2));
    await user.click(within(dialog).getByRole('button', { name: t.cancel.categoryConfirmLabel }));

    await waitFor(() => {
      expect(tournamentsApi.cancelCategory).toHaveBeenCalledWith('c1', 't1', 'cat1');
    });
  });
});
