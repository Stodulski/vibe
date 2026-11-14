import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderWithProviders, screen, userEvent, waitFor, within } from '@/test/test-utils';
import { ES_AR } from '@/shared/i18n/es_AR';
import { CreateTournamentModal } from './CreateTournamentModal';

vi.mock('../api/tournaments.api', () => ({ tournamentsApi: { create: vi.fn() } }));
vi.mock('sonner', () => ({ toast: { error: vi.fn(), success: vi.fn() } }));

const t = ES_AR.tournaments;
type User = ReturnType<typeof userEvent.setup>;

/** A field by its label text, whatever "(requerido)" or "(opcional)" follows it. */
function field(label: string) {
  const escaped = label.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  return screen.getByLabelText(new RegExp(`^${escaped}( \\(|$)`));
}

async function pickDay(user: User, label: string, last: boolean) {
  await user.click(screen.getByRole('button', { name: new RegExp(`^${label}`) }));
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
  await user.type(field(t.form.name), '  Copa Vibe ');
  await pickDay(user, t.form.startDate, false);
  await pickDay(user, t.form.endDate, true);
  await user.type(field(t.form.categoryName), ' 6ta ');
  await user.type(field(t.form.price), '20000');
}

describe('CreateTournamentModal', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('starts a groups category with the suggested defaults: groups of 3, 2 advance, best of three to 6, 90 minutes', async () => {
    await renderModal();

    expect(field(t.form.groupSize)).toHaveValue(3);
    expect(field(t.form.qualifiers)).toHaveValue(2);
    expect(field(t.form.gamesPerSet)).toHaveValue(6);
    expect(field(t.form.matchDuration)).toHaveValue(90);
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

    await user.clear(field(t.form.qualifiers));
    await user.type(field(t.form.qualifiers), '3');
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

  it('sends the court fee per pair in centavos when the owner sets one, and leaves it out otherwise', async () => {
    const { tournamentsApi, user } = await renderModal();
    await fillBasics(user);
    await user.type(field(t.form.courtFee), '1500');

    await user.click(screen.getByRole('button', { name: t.form.submit }));

    await waitFor(() => {
      expect(tournamentsApi.create).toHaveBeenCalledTimes(1);
    });
    expect(vi.mocked(tournamentsApi.create).mock.calls[0]?.[1].categories[0]).toMatchObject({ court_fee: 150000 });
  });
});

describe('CreateTournamentModal: layout', () => {
  it('opens the first category and a new one, folding the one before', async () => {
    const { user } = await renderModal();
    const trigger = (n: number) => screen.getByRole('button', { name: t.form.category(n) });

    expect(trigger(1)).toHaveAttribute('aria-expanded', 'true');
    await user.click(screen.getByRole('button', { name: t.form.addCategory }));

    expect(trigger(2)).toHaveAttribute('aria-expanded', 'true');
    expect(trigger(1)).toHaveAttribute('aria-expanded', 'false');
  });

  it('names the category in its header once it has a name', async () => {
    const { user } = await renderModal();
    await user.type(field(t.form.categoryName), '6ta');

    expect(screen.getByRole('button', { name: t.form.categoryTitle(1, '6ta') })).toBeInTheDocument();
  });

  it('opens a folded category that has an error when sending', async () => {
    const { user } = await renderModal();
    await user.click(screen.getByRole('button', { name: t.form.addCategory }));
    expect(screen.getByRole('button', { name: t.form.category(1) })).toHaveAttribute('aria-expanded', 'false');

    await user.click(screen.getByRole('button', { name: t.form.submit }));

    expect(await screen.findByRole('button', { name: t.form.category(1) })).toHaveAttribute('aria-expanded', 'true');
  });
});

describe('CreateTournamentModal: required and optional', () => {
  it('marks every field required or optional in its label, with the hint above the field', async () => {
    await renderModal();

    expect(screen.getByLabelText(`${t.form.name} (${ES_AR.common.requiredMarker})`)).toBeInTheDocument();
    expect(screen.getByLabelText(`${t.form.courtFee} (${ES_AR.common.optional})`)).toBeInTheDocument();
    const hint = screen.getByText(t.form.courtFeeHint);
    const input = screen.getByLabelText(`${t.form.courtFee} (${ES_AR.common.optional})`);
    expect(hint.compareDocumentPosition(input) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    expect(hint).not.toHaveTextContent(/^Opcional/);
  });
});
