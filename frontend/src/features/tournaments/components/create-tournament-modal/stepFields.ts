import { getProblem } from '@/shared/lib/ApiError';
import { categoryField, type CategoryFormValues, type TournamentFormValues } from '../../schemas/tournamentForm.schema';

export type CreateStep = 1 | 2 | 3;

/** What step 1 validates before advancing. */
export const BASICS_FIELDS = ['sport', 'start_date', 'end_date'] as const satisfies (keyof TournamentFormValues)[];

type CategoryKey = keyof CategoryFormValues;

function paths(index: number, keys: CategoryKey[]) {
  return keys.map((key) => categoryField(index, key));
}

/** The fields of a category's first view ("Categoría"): its traits, its start, and what it costs. */
export function categoryViewFields(index: number) {
  return paths(index, [
    'modality',
    'kind',
    'level',
    'suma',
    'starts_on',
    'starts_at',
    'format',
    'capacity',
    'price',
    'court_fee',
    'prizes',
  ]);
}

/** The fields of a category's second view ("Cómo se juega"); the zone settings only count for zones. */
export function playViewFields(index: number, format: CategoryFormValues['format']) {
  const zones: CategoryKey[] = format === 'groups' ? ['group_size', 'qualifiers_per_group'] : [];
  return paths(index, [...zones, 'sets_to_win', 'games_per_set', 'super_tiebreak_decider', 'match_duration_minutes']);
}

/**
 * The step a server refusal belongs to, read from the field it names: the tournament's own
 * fields are step 1, anything under `categories` is step 2. An error that names no field stays
 * on the review.
 */
export function stepForServerError(error: unknown): CreateStep {
  const fields = getProblem(error)?.errors.map((e) => e.field) ?? [];
  if (fields.some((field) => field.startsWith('categories'))) return 2;
  if (fields.some((field) => (BASICS_FIELDS as readonly string[]).includes(field))) return 1;
  return 3;
}

/** The step holding the first invalid field after a submit that skipped the steps' own checks. */
export function stepForErrors(keys: string[]): CreateStep {
  return keys.includes('categories') ? 2 : 1;
}
