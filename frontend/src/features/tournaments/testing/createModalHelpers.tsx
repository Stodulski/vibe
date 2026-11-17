import { vi } from 'vitest';
import { renderWithProviders, screen, userEvent, within } from '@/test/test-utils';
import { ES_AR } from '@/shared/i18n/es_AR';
import { CreateTournamentModal } from '../components/CreateTournamentModal';

const t = ES_AR.tournaments;
export type User = ReturnType<typeof userEvent.setup>;

/** A field by its label text, whatever "(requerido)" or "(opcional)" follows it. */
export function field(label: string) {
  const escaped = label.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  return screen.getByLabelText(new RegExp(`^${escaped}( \\(|$)`));
}

export function button(name: string) {
  return screen.getByRole('button', { name });
}

export async function pickDay(user: User, label: string, last: boolean) {
  await user.click(screen.getByRole('button', { name: new RegExp(`^${label}`) }));
  const grid = await screen.findByRole('grid');
  const days = within(grid)
    .getAllByRole('button')
    .filter((day) => !(day as HTMLButtonElement).disabled);
  const day = last ? days[days.length - 1] : days[days.length - 2];
  if (!day) throw new Error('no selectable day');
  await user.click(day);
}

export async function renderModal() {
  const { tournamentsApi } = await import('../api/tournaments.api');
  vi.mocked(tournamentsApi.create).mockResolvedValue({} as never);
  const onClose = vi.fn();
  renderWithProviders(<CreateTournamentModal open onClose={onClose} complexId="c1" />);
  return { tournamentsApi, onClose, user: userEvent.setup() };
}

export async function next(user: User) {
  await user.click(button(ES_AR.common.next));
}

/** Step 1 filled in (the sport is already padel); leaves the dialog on step 1. */
export async function fillTournament(user: User) {
  await pickDay(user, t.form.startDate, false);
  await pickDay(user, t.form.endDate, true);
}

/** Picks an option of a Radix select by its label, after opening its trigger. */
export async function pickOption(user: User, triggerLabel: string, option: string) {
  await user.click(field(triggerLabel));
  await user.click(await screen.findByRole('option', { name: option }));
}

/**
 * Opens a new category in step 2 and fills its first view: a level category (6ta) or a suma, its
 * modality, the day and the time it starts, and the price. Leaves the dialog on "Cómo se juega".
 */
export async function addCategory(
  user: User,
  { modality, level, suma, price }: { modality: string; level?: string; suma?: string; price: string },
) {
  await user.click(button(t.form.addCategory));
  await user.click(button(modality));
  if (suma) {
    await pickOption(user, t.form.kind, t.form.kindSuma);
    await user.type(field(t.form.suma), suma);
  } else if (level) {
    await pickOption(user, t.form.level, level);
  }
  await pickDay(user, t.form.startsOn, false);
  await pickOption(user, t.form.startsAt, '18:00');
  await user.type(field(t.form.price), price);
  await next(user);
}

/** The whole way to the review step with one category, "Caballeros 6ta" at $20.000. */
export async function reachReview(user: User) {
  await fillTournament(user);
  await next(user);
  await addCategory(user, { modality: modalityButton('men'), level: '6ta', price: '20000' });
  await user.click(button(t.createSteps.saveCategory));
  await next(user);
}

function modalityButton(value: 'men' | 'women' | 'mixed'): string {
  return ES_AR.tournaments.traits.modality[value];
}
