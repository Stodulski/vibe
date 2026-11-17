import { describe, it, expect, vi, beforeEach } from 'vitest';
import { screen, within } from '@/test/test-utils';
import { ES_AR } from '@/shared/i18n/es_AR';
import { formatPrice } from '@/shared/lib/utils';
import {
  addCategory,
  button,
  field,
  fillTournament,
  next,
  pickDay,
  pickOption,
  renderModal,
  type User,
} from '../testing/createModalHelpers';

vi.mock('../api/tournaments.api', () => ({ tournamentsApi: { create: vi.fn() } }));
vi.mock('sonner', () => ({ toast: { error: vi.fn(), success: vi.fn() } }));

const t = ES_AR.tournaments;
const c = t.createSteps;
const SIX = { modality: t.traits.modality.men, level: '6ta', price: '20000' };

async function toCategories(user: User) {
  await fillTournament(user);
  await next(user);
}

describe('CreateTournamentModal: categories step', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('opens a new category on its first view with the category fields only', async () => {
    const { user } = await renderModal();
    await toCategories(user);

    await user.click(button(t.form.addCategory));

    expect(screen.getByRole('heading', { name: c.categoryView })).toBeInTheDocument();
    expect(screen.getByRole('group', { name: t.form.modality })).toBeInTheDocument();
    for (const label of [t.form.kind, t.form.level, t.form.startsAt, t.form.categoryFormat, t.form.capacity]) {
      expect(field(label)).toBeInTheDocument();
    }
    for (const label of [t.form.price, t.form.courtFee, t.form.prizes]) {
      expect(field(label)).toBeInTheDocument();
    }
    expect(screen.queryByLabelText(/Nombre/)).not.toBeInTheDocument();
    expect(screen.queryByLabelText(new RegExp(`^${t.form.groupSize}`))).not.toBeInTheDocument();
  });

  it('words the modality by the sport and shows no level or suma off padel (S15)', async () => {
    const { user } = await renderModal();
    await pickOption(user, t.form.sport, ES_AR.courts.sportTypes.soccer);
    await fillTournament(user);
    await next(user);
    await user.click(button(t.form.addCategory));

    expect(screen.getByRole('button', { name: t.traits.modalityMasculine.women })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: t.traits.modality.women })).not.toBeInTheDocument();
    expect(screen.queryByText(new RegExp(t.form.kind))).not.toBeInTheDocument();
    expect(screen.queryByText(new RegExp(t.form.level))).not.toBeInTheDocument();
  });

  it('asks for the modality, the level, the day, the time and the price before the second view', async () => {
    const { user } = await renderModal();
    await toCategories(user);
    await user.click(button(t.form.addCategory));

    await next(user);

    expect(await screen.findByText(t.validation.modalityRequired)).toBeInTheDocument();
    expect(screen.getByText(t.validation.levelRequired)).toBeInTheDocument();
    expect(screen.getByText(t.validation.startsOnRequired)).toBeInTheDocument();
    expect(screen.getByText(t.validation.startsAtRequired)).toBeInTheDocument();
    expect(screen.getByText(t.validation.priceRequired)).toBeInTheDocument();
    expect(screen.queryByText(c.playView)).not.toBeInTheDocument();
  });

  it('starts the second view with the suggested defaults: groups of 3, 2 advance, best of three to 6, 90 minutes', async () => {
    const { user } = await renderModal();
    await toCategories(user);

    await addCategory(user, SIX);

    expect(screen.getByText(c.playView)).toBeInTheDocument();
    expect(field(t.form.groupSize)).toHaveValue(3);
    expect(field(t.form.qualifiers)).toHaveValue(2);
    expect(field(t.form.gamesPerSet)).toHaveValue(6);
    expect(field(t.form.matchDuration)).toHaveValue(90);
    expect(screen.getByRole('checkbox', { name: t.form.superTiebreak })).toBeChecked();
  });
});

describe('CreateTournamentModal: categories step, how it plays', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('hides the zone fields for elimination and the match tiebreak for a single set', async () => {
    const { user } = await renderModal();
    await toCategories(user);
    await user.click(button(t.form.addCategory));
    await user.click(button(t.traits.modality.men));
    await pickOption(user, t.form.level, '6ta');
    await pickDay(user, t.form.startsOn, false);
    await pickOption(user, t.form.startsAt, '18:00');
    await user.type(field(t.form.price), '20000');
    await user.click(screen.getByRole('combobox', { name: new RegExp(`^${t.form.categoryFormat}`) }));
    await user.click(await screen.findByRole('option', { name: t.format.elimination }));
    await next(user);

    expect(screen.queryByLabelText(new RegExp(`^${t.form.groupSize}`))).not.toBeInTheDocument();
    await user.click(screen.getByRole('combobox', { name: t.form.matchFormat }));
    await user.click(await screen.findByRole('option', { name: t.form.setsToWin[1] }));
    expect(screen.queryByRole('checkbox', { name: t.form.superTiebreak })).not.toBeInTheDocument();
  });

  it('refuses qualifiers that are not below the group size and keeps the view', async () => {
    const { user } = await renderModal();
    await toCategories(user);
    await addCategory(user, SIX);

    await user.clear(field(t.form.qualifiers));
    await user.type(field(t.form.qualifiers), '3');
    await user.click(button(c.saveCategory));

    expect(await screen.findByText(t.validation.qualifiersRange)).toBeInTheDocument();
    expect(screen.getByText(c.playView)).toBeInTheDocument();
  });
});

describe('CreateTournamentModal: category rows', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('saves the category as a summary row with its composed title, start, format, capacity and price', async () => {
    const { user } = await renderModal();
    await toCategories(user);
    await addCategory(user, SIX);

    await user.click(button(c.saveCategory));

    const row = screen.getByRole('listitem', { name: 'Caballeros 6ta' });
    expect(row).toHaveTextContent('Caballeros 6ta');
    expect(row).toHaveTextContent('18:00');
    expect(row).toHaveTextContent(t.format.groups);
    expect(row).toHaveTextContent(c.capacityShort(16));
    expect(row).toHaveTextContent(formatPrice(2000000));
    expect(button(c.editCategoryLabel('Caballeros 6ta'))).toBeInTheDocument();
    // Touch targets: the standard 40 px button, not the 32 px small one.
    expect(button(c.editCategoryLabel('Caballeros 6ta'))).toHaveAttribute('data-size', 'lg');
    expect(button(c.removeCategoryLabel('Caballeros 6ta'))).toHaveAttribute('data-size', 'lg');
  });

  it('edits a saved category in the same step and keeps the change', async () => {
    const { user } = await renderModal();
    await toCategories(user);
    await addCategory(user, SIX);
    await user.click(button(c.saveCategory));

    await user.click(button(c.editCategoryLabel('Caballeros 6ta')));
    expect(field(t.form.level)).toHaveTextContent('6ta');
    await pickOption(user, t.form.level, '7ma');
    await next(user);
    await user.click(button(c.saveCategory));

    expect(screen.getByRole('listitem', { name: 'Caballeros 7ma' })).toBeInTheDocument();
    expect(screen.queryByRole('listitem', { name: 'Caballeros 6ta' })).not.toBeInTheDocument();
  });

  it('drops a new category when leaving its first view with Volver, and restores an edited one', async () => {
    const { user } = await renderModal();
    await toCategories(user);
    await user.click(button(t.form.addCategory));
    await user.click(button(t.traits.modality.women));
    await user.click(button(ES_AR.common.back));
    expect(screen.getByText(c.noCategories)).toBeInTheDocument();

    await addCategory(user, SIX);
    await user.click(button(c.saveCategory));
    await user.click(button(c.editCategoryLabel('Caballeros 6ta')));
    await user.click(button(t.traits.modality.women));
    await user.click(button(ES_AR.common.back));

    expect(screen.getByRole('listitem', { name: 'Caballeros 6ta' })).toBeInTheDocument();
    expect(screen.queryByRole('listitem', { name: 'Damas 6ta' })).not.toBeInTheDocument();
  });
});

describe('CreateTournamentModal: category list', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('removes a category from the list', async () => {
    const { user } = await renderModal();
    await toCategories(user);
    await addCategory(user, SIX);
    await user.click(button(c.saveCategory));

    await user.click(button(c.removeCategoryLabel('Caballeros 6ta')));

    expect(screen.queryByRole('listitem', { name: 'Caballeros 6ta' })).not.toBeInTheDocument();
    expect(screen.getByText(c.noCategories)).toBeInTheDocument();
  });

  it('keeps several categories in the list', async () => {
    const { user } = await renderModal();
    await toCategories(user);
    await addCategory(user, SIX);
    await user.click(button(c.saveCategory));
    await addCategory(user, { modality: t.traits.modality.women, level: '7ma', price: '15000' });
    await user.click(button(c.saveCategory));

    const rows = screen.getAllByRole('listitem').filter((item) => item.getAttribute('aria-label'));
    expect(rows.map((row) => within(row).getByText(/^(Caballeros 6ta|Damas 7ma)$/).textContent)).toEqual([
      'Caballeros 6ta',
      'Damas 7ma',
    ]);
  });
});
