import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderWithProviders, screen, userEvent, waitFor, within } from '@/test/test-utils';
import { ES_AR } from '@/shared/i18n/es_AR';
import { CreateTournamentModal } from './CreateTournamentModal';

vi.mock('../api/tournaments.api', () => ({ tournamentsApi: { create: vi.fn() } }));
vi.mock('sonner', () => ({ toast: { error: vi.fn(), success: vi.fn() } }));

const t = ES_AR.tournaments;
type User = ReturnType<typeof userEvent.setup>;

async function pickDay(user: User, label: string, last: boolean) {
  await user.click(screen.getByRole('button', { name: label }));
  const grid = await screen.findByRole('grid');
  const days = within(grid)
    .getAllByRole('button')
    .filter((day) => !(day as HTMLButtonElement).disabled);
  const day = last ? days[days.length - 1] : days[days.length - 2];
  if (!day) throw new Error('no selectable day');
  await user.click(day);
}

async function renderModal() {
  const { tournamentsApi } = await import('../api/tournaments.api');
  vi.mocked(tournamentsApi.create).mockResolvedValue({} as never);
  const onClose = vi.fn();
  renderWithProviders(<CreateTournamentModal open onClose={onClose} complexId="c1" />);
  return { tournamentsApi, onClose, user: userEvent.setup() };
}

async function fillBasics(user: User) {
  await user.type(screen.getByLabelText(t.form.name), '  Copa Vibe ');
  await pickDay(user, t.form.startDate, false);
  await pickDay(user, t.form.endDate, true);
  await user.type(screen.getByLabelText(t.form.categoryName), ' 6ta ');
  await user.type(screen.getByLabelText(t.form.price), '20000');
}

describe('CreateTournamentModal', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('starts a groups category with the suggested defaults: groups of 3, 2 advance, best of three to 6, 90 minutes', async () => {
    await renderModal();

    expect(screen.getByLabelText(t.form.groupSize)).toHaveValue(3);
    expect(screen.getByLabelText(t.form.qualifiers)).toHaveValue(2);
    expect(screen.getByLabelText(t.form.gamesPerSet)).toHaveValue(6);
    expect(screen.getByLabelText(t.form.matchDuration)).toHaveValue(90);
    expect(screen.getByRole('checkbox', { name: t.form.superTiebreak })).toBeChecked();
  });

  it('asks for the name, days, category name and price when sent empty, and sends nothing', async () => {
    const { tournamentsApi, user } = await renderModal();

    await user.click(screen.getByRole('button', { name: t.form.submit }));

    expect(await screen.findByText(t.validation.nameRequired)).toBeInTheDocument();
    expect(screen.getAllByText(ES_AR.validation.dateRequired)).toHaveLength(2);
    expect(screen.getByText(t.validation.categoryNameRequired)).toBeInTheDocument();
    expect(screen.getByText(t.validation.priceRequired)).toBeInTheDocument();
    expect(tournamentsApi.create).not.toHaveBeenCalled();
  });

  it('refuses qualifiers that are not below the group size', async () => {
    const { tournamentsApi, user } = await renderModal();
    await fillBasics(user);

    await user.clear(screen.getByLabelText(t.form.qualifiers));
    await user.type(screen.getByLabelText(t.form.qualifiers), '3');
    await user.click(screen.getByRole('button', { name: t.form.submit }));

    expect(await screen.findByText(t.validation.qualifiersRange)).toBeInTheDocument();
    expect(tournamentsApi.create).not.toHaveBeenCalled();
  });
});

describe('CreateTournamentModal: match format and submit', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('hides the match tiebreak option for a single set', async () => {
    const { user } = await renderModal();

    await user.click(screen.getByRole('combobox', { name: t.form.matchFormat }));
    await user.click(await screen.findByRole('option', { name: t.form.setsToWin[1] }));

    expect(screen.queryByRole('checkbox', { name: t.form.superTiebreak })).not.toBeInTheDocument();
  });

  it('sends the category with its group and match format and the price in centavos', async () => {
    const { tournamentsApi, onClose, user } = await renderModal();
    await fillBasics(user);

    await user.click(screen.getByRole('button', { name: t.form.submit }));

    await waitFor(() => {
      expect(tournamentsApi.create).toHaveBeenCalledTimes(1);
    });
    const body = vi.mocked(tournamentsApi.create).mock.calls[0]?.[1];
    expect(body).toMatchObject({ name: 'Copa Vibe', sport: 'padel' });
    expect(body?.categories).toEqual([
      {
        name: '6ta',
        format: 'groups',
        capacity: 16,
        price: 2000000,
        group_size: 3,
        qualifiers_per_group: 2,
        sets_to_win: 2,
        games_per_set: 6,
        super_tiebreak_decider: true,
        match_duration_minutes: 90,
      },
    ]);
    await waitFor(() => {
      expect(onClose).toHaveBeenCalled();
    });
  });
});
