import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderWithProviders, screen, userEvent, waitFor } from '@/test/test-utils';
import { makeConsumedHttpError } from '@/test/factories';
import { ES_AR } from '@/shared/i18n/es_AR';
import { makeCategory, makeDetail, makeMatch, makePair } from '../testing/fixtures';
import { ResultDialog } from './ResultDialog';
import type { TournamentCategory, TournamentMatch } from '@/shared/types/api.types';

vi.mock('../api/tournaments.api', () => ({ tournamentsApi: { setResult: vi.fn() } }));
vi.mock('sonner', () => ({ toast: { error: vi.fn(), success: vi.fn() } }));

const t = ES_AR.tournaments.result;
const pairs = new Map([
  ['p1', makePair({ id: 'p1', players: ['Ana Pérez', 'Bea Gómez'] })],
  ['p2', makePair({ id: 'p2', players: ['Carla Ruiz', 'Dana Soto'] })],
]);
const MATCH = makeMatch({ id: 'm1', pair1Id: 'p1', pair2Id: 'p2' });

async function renderDialog(category: TournamentCategory = makeCategory(), match: TournamentMatch = MATCH) {
  const { tournamentsApi } = await import('../api/tournaments.api');
  vi.mocked(tournamentsApi.setResult).mockResolvedValue(makeDetail());
  const onClose = vi.fn();
  renderWithProviders(
    <ResultDialog
      open
      onClose={onClose}
      complexId="c1"
      tournamentId="t1"
      category={category}
      match={match}
      pairs={pairs}
    />,
  );
  return { tournamentsApi, onClose, user: userEvent.setup() };
}

async function type(user: ReturnType<typeof userEvent.setup>, label: string, value: string) {
  const input = screen.getByLabelText(label);
  await user.clear(input);
  await user.type(input, value);
}

describe('ResultDialog', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('shows three set rows for best of three, the third being the match tiebreak', async () => {
    await renderDialog();

    expect(screen.getByRole('group', { name: t.set(1) })).toBeInTheDocument();
    expect(screen.getByRole('group', { name: t.set(2) })).toBeInTheDocument();
    expect(screen.getByRole('group', { name: t.superTiebreakSet(3) })).toBeInTheDocument();
  });

  it('shows one row for a single set', async () => {
    await renderDialog(makeCategory({ sets_to_win: 1 }));
    expect(screen.getAllByRole('group', { name: /^Set \d/ })).toHaveLength(1);
  });

  it('shows a plain third set when the category has no match tiebreak', async () => {
    await renderDialog(makeCategory({ super_tiebreak_decider: false }));
    expect(screen.getByRole('group', { name: t.set(3) })).toBeInTheDocument();
    expect(screen.queryByRole('group', { name: t.superTiebreakSet(3) })).not.toBeInTheDocument();
  });

  it('sends only the sets played and closes', async () => {
    const { tournamentsApi, onClose, user } = await renderDialog();

    await type(user, `${t.games('Pérez / Gómez')} (${t.set(1)})`, '6');
    await type(user, `${t.games('Ruiz / Soto')} (${t.set(1)})`, '4');
    await type(user, `${t.games('Pérez / Gómez')} (${t.set(2)})`, '6');
    await type(user, `${t.games('Ruiz / Soto')} (${t.set(2)})`, '3');
    await user.click(screen.getByRole('button', { name: t.submit }));

    await waitFor(() => {
      expect(tournamentsApi.setResult).toHaveBeenCalledWith('c1', 't1', 'm1', {
        sets: [
          { a: 6, b: 4, super: false },
          { a: 6, b: 3, super: false },
        ],
      });
    });
    await waitFor(() => {
      expect(onClose).toHaveBeenCalled();
    });
  });

  it('says what is wrong with the score and sends nothing', async () => {
    const { tournamentsApi, user } = await renderDialog();

    await type(user, `${t.games('Pérez / Gómez')} (${t.set(1)})`, '6');
    await type(user, `${t.games('Ruiz / Soto')} (${t.set(1)})`, '5');
    await user.click(screen.getByRole('button', { name: t.submit }));

    expect(await screen.findByText(t.errors.invalidSet(1, 6, 5, 6))).toBeInTheDocument();
    expect(tournamentsApi.setResult).not.toHaveBeenCalled();
  });
});

describe('ResultDialog: walkover and corrections', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('loads a walkover for the pair that did not show', async () => {
    const { tournamentsApi, user } = await renderDialog();

    await user.click(screen.getByRole('button', { name: t.modeWalkover }));
    await user.click(screen.getByRole('button', { name: 'Ruiz / Soto' }));
    await user.click(screen.getByRole('button', { name: t.submit }));

    await waitFor(() => {
      expect(tournamentsApi.setResult).toHaveBeenCalledWith('c1', 't1', 'm1', { walkover_loser: 2 });
    });
  });

  it('opens a loaded result with its sets to correct', async () => {
    await renderDialog(
      makeCategory(),
      makeMatch({
        id: 'm1',
        pair1Id: 'p1',
        pair2Id: 'p2',
        status: 'completed',
        winnerPairId: 'p1',
        sets: [
          { a: 6, b: 4, super: false },
          { a: 3, b: 6, super: false },
          { a: 10, b: 7, super: true },
        ],
      }),
    );

    expect(screen.getByLabelText(`${t.points('Pérez / Gómez')} (${t.superTiebreakSet(3)})`)).toHaveValue(10);
    expect(screen.getByLabelText(`${t.games('Ruiz / Soto')} (${t.set(2)})`)).toHaveValue(6);
  });

  it('tells in Spanish that a result cannot be corrected once the next match has one', async () => {
    const { tournamentsApi, user } = await renderDialog();
    vi.mocked(tournamentsApi.setResult).mockRejectedValue(
      await makeConsumedHttpError(409, { title: 'x', detail: 'x' }),
    );

    await user.click(screen.getByRole('button', { name: t.modeWalkover }));
    await user.click(screen.getByRole('button', { name: 'Pérez / Gómez' }));
    await user.click(screen.getByRole('button', { name: t.submit }));

    expect(await screen.findByText(t.apiErrors.locked)).toBeInTheDocument();
  });
});
