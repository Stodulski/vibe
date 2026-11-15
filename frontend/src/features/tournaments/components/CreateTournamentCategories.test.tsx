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
  renderModal,
  type User,
} from '../testing/createModalHelpers';

vi.mock('../api/tournaments.api', () => ({ tournamentsApi: { create: vi.fn() } }));
vi.mock('sonner', () => ({ toast: { error: vi.fn(), success: vi.fn() } }));

const t = ES_AR.tournaments;
const c = t.createSteps;

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

    expect(screen.getByText(c.categoryView)).toBeInTheDocument();
    for (const label of [t.form.categoryName, t.form.categoryFormat, t.form.capacity, t.form.price, t.form.courtFee]) {
      expect(field(label)).toBeInTheDocument();
    }
    expect(screen.queryByLabelText(new RegExp(`^${t.form.groupSize}`))).not.toBeInTheDocument();
  });

  it('asks for the name and price before the second view', async () => {
    const { user } = await renderModal();
    await toCategories(user);
    await user.click(button(t.form.addCategory));

    await next(user);

    expect(await screen.findByText(t.validation.categoryNameRequired)).toBeInTheDocument();
    expect(screen.getByText(t.validation.priceRequired)).toBeInTheDocument();
    expect(screen.queryByText(c.playView)).not.toBeInTheDocument();
  });

  it('starts the second view with the suggested defaults: groups of 3, 2 advance, best of three to 6, 90 minutes', async () => {
    const { user } = await renderModal();
    await toCategories(user);

    await addCategory(user, '6ta', '20000');

    expect(screen.getByText(c.playView)).toBeInTheDocument();
    expect(field(t.form.groupSize)).toHaveValue(3);
    expect(field(t.form.qualifiers)).toHaveValue(2);
    expect(field(t.form.gamesPerSet)).toHaveValue(6);
    expect(field(t.form.matchDuration)).toHaveValue(90);
    expect(screen.getByRole('checkbox', { name: t.form.superTiebreak })).toBeChecked();
  });

  it('hides the zone fields for elimination and the match tiebreak for a single set', async () => {
    const { user } = await renderModal();
    await toCategories(user);
    await user.click(button(t.form.addCategory));
    await user.type(field(t.form.categoryName), '6ta');
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
    await addCategory(user, '6ta', '20000');

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

  it('saves the category as a summary row with name, format, capacity and price', async () => {
    const { user } = await renderModal();
    await toCategories(user);
    await addCategory(user, '6ta', '20000');

    await user.click(button(c.saveCategory));

    const row = screen.getByRole('listitem', { name: '6ta' });
    expect(row).toHaveTextContent('6ta');
    expect(row).toHaveTextContent(t.format.groups);
    expect(row).toHaveTextContent(c.capacityShort(16));
    expect(row).toHaveTextContent(formatPrice(2000000));
    expect(button(c.editCategoryLabel('6ta'))).toBeInTheDocument();
    // Touch targets: the standard 40 px button, not the 32 px small one.
    expect(button(c.editCategoryLabel('6ta'))).toHaveAttribute('data-size', 'lg');
    expect(button(c.removeCategoryLabel('6ta'))).toHaveAttribute('data-size', 'lg');
  });

  it('edits a saved category in the same step and keeps the change', async () => {
    const { user } = await renderModal();
    await toCategories(user);
    await addCategory(user, '6ta', '20000');
    await user.click(button(c.saveCategory));

    await user.click(button(c.editCategoryLabel('6ta')));
    expect(field(t.form.categoryName)).toHaveValue('6ta');
    await user.clear(field(t.form.categoryName));
    await user.type(field(t.form.categoryName), '7ma');
    await next(user);
    await user.click(button(c.saveCategory));

    expect(screen.getByRole('listitem', { name: '7ma' })).toBeInTheDocument();
    expect(screen.queryByRole('listitem', { name: '6ta' })).not.toBeInTheDocument();
  });

  it('drops a new category when leaving its first view with Volver, and restores an edited one', async () => {
    const { user } = await renderModal();
    await toCategories(user);
    await user.click(button(t.form.addCategory));
    await user.type(field(t.form.categoryName), 'Fantasma');
    await user.click(button(ES_AR.common.back));
    expect(screen.queryByText('Fantasma')).not.toBeInTheDocument();

    await addCategory(user, '6ta', '20000');
    await user.click(button(c.saveCategory));
    await user.click(button(c.editCategoryLabel('6ta')));
    await user.clear(field(t.form.categoryName));
    await user.type(field(t.form.categoryName), 'Cambiada');
    await user.click(button(ES_AR.common.back));

    expect(screen.getByRole('listitem', { name: '6ta' })).toBeInTheDocument();
  });
});

describe('CreateTournamentModal: category list', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('removes a category from the list', async () => {
    const { user } = await renderModal();
    await toCategories(user);
    await addCategory(user, '6ta', '20000');
    await user.click(button(c.saveCategory));

    await user.click(button(c.removeCategoryLabel('6ta')));

    expect(screen.queryByRole('listitem', { name: '6ta' })).not.toBeInTheDocument();
    expect(screen.getByText(c.noCategories)).toBeInTheDocument();
  });

  it('keeps several categories in the list', async () => {
    const { user } = await renderModal();
    await toCategories(user);
    await addCategory(user, '6ta', '20000');
    await user.click(button(c.saveCategory));
    await addCategory(user, '7ma', '15000');
    await user.click(button(c.saveCategory));

    const rows = screen.getAllByRole('listitem').filter((item) => item.getAttribute('aria-label'));
    expect(rows.map((row) => within(row).getByText(/^(6ta|7ma)$/).textContent)).toEqual(['6ta', '7ma']);
  });
});
