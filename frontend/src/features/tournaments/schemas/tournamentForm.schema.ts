import { differenceInCalendarDays } from 'date-fns/differenceInCalendarDays';
import { parseISO } from 'date-fns/parseISO';
import { z } from 'zod';
import { ES_AR } from '@/shared/i18n/es_AR';
import { pesosToCentavos } from '@/shared/lib/money';
import type { CreateTournamentRequest, Sport } from '@/shared/types/api.types';

const t = ES_AR.tournaments.validation;

const MAX_DAYS = 60;
const MODALITIES = ['men', 'women', 'mixed'] as const;
const MAX_PRIZES = 200;

/** The most a pair can be asked per match, in pesos; the API's own ceiling. */
export const MAX_COURT_FEE = 1_000_000;

const categoryFormSchema = z
  .object({
    modality: z.enum(MODALITIES).optional(),
    kind: z.enum(['level', 'suma']),
    level: z.number().int().min(1, t.levelRequired).max(9, t.levelRequired).optional(),
    suma: z.number({ error: t.sumaRange }).int(t.sumaRange).min(2, t.sumaRange).max(18, t.sumaRange).optional(),
    starts_on: z.string(),
    starts_at: z.string(),
    prizes: z.string().trim().max(MAX_PRIZES, t.prizesMax),
    format: z.enum(['elimination', 'groups']),
    capacity: z
      .number({ error: t.capacityRange })
      .int(t.capacityRange)
      .min(2, t.capacityRange)
      .max(128, t.capacityRange),
    price: z.number({ error: t.priceRequired }).positive(t.pricePositive),
    court_fee: z
      .number({ error: t.courtFeeRange })
      .min(0, t.courtFeeRange)
      .max(MAX_COURT_FEE, t.courtFeeRange)
      .optional(),
    group_size: z.number({ error: t.groupSizeRange }),
    qualifiers_per_group: z.number({ error: t.qualifiersRange }),
    sets_to_win: z.union([z.literal(1), z.literal(2)]),
    games_per_set: z.number({ error: t.gamesRange }).int(t.gamesRange).min(4, t.gamesRange).max(9, t.gamesRange),
    super_tiebreak_decider: z.boolean(),
    match_duration_minutes: z
      .number({ error: t.durationRange })
      .int(t.durationRange)
      .min(30, t.durationRange)
      .max(480, t.durationRange),
  })
  // `when` runs these even when a number is still blank: otherwise zod skips the object's checks
  // and the owner sees the modality error only after typing the price.
  .refine((data) => !!data.modality, { message: t.modalityRequired, path: ['modality'], when: () => true })
  .refine((data) => data.kind !== 'level' || data.level !== undefined, {
    message: t.levelRequired,
    path: ['level'],
    when: () => true,
  })
  .refine((data) => data.kind !== 'suma' || data.suma !== undefined, {
    message: t.sumaRange,
    path: ['suma'],
    when: () => true,
  })
  .refine((data) => !!data.starts_on, { message: t.startsOnRequired, path: ['starts_on'], when: () => true })
  .refine((data) => !!data.starts_at, { message: t.startsAtRequired, path: ['starts_at'], when: () => true })
  .superRefine((data, ctx) => {
    if (data.format !== 'groups') return;
    const sizeOk = Number.isInteger(data.group_size) && data.group_size >= 2 && data.group_size <= 12;
    if (!sizeOk) {
      ctx.addIssue({ code: 'custom', path: ['group_size'], message: t.groupSizeRange });
      return;
    }
    const q = data.qualifiers_per_group;
    if (!Number.isInteger(q) || q < 1 || q >= data.group_size) {
      ctx.addIssue({ code: 'custom', path: ['qualifiers_per_group'], message: t.qualifiersRange });
    }
  });

/**
 * The create-tournament form's own shape, not the wire shape
 * (`CreateTournamentRequest`, built from it by `toCreateRequest`). Prices are
 * pesos here, centavos on the wire. The court fee is optional: blank or 0 sends none. Group settings only count for a `groups`
 * category, and the match tiebreak only for best of three. A tournament has no name (S10): it is composed from its sport.
 */
export const tournamentFormSchema = z
  .object({
    sport: z.string().min(1),
    start_date: z.string().min(1, ES_AR.validation.dateRequired),
    end_date: z.string().min(1, ES_AR.validation.dateRequired),
    categories: z.array(categoryFormSchema).min(1, t.categoriesRequired).max(20, t.categoriesMax),
  })
  .superRefine((data, ctx) => {
    if (!data.start_date || !data.end_date) return;
    const days = differenceInCalendarDays(parseISO(data.end_date), parseISO(data.start_date));
    if (days < 0) ctx.addIssue({ code: 'custom', path: ['end_date'], message: t.endBeforeStart });
    else if (days > MAX_DAYS) ctx.addIssue({ code: 'custom', path: ['end_date'], message: t.tooLong });
    // A category starts on a day of the tournament itself, judged only against a valid range (ISO dates compare as text).
    if (days < 0 || days > MAX_DAYS) return;
    data.categories.forEach((category, index) => {
      if (!category.starts_on) return;
      if (category.starts_on < data.start_date || category.starts_on > data.end_date) {
        ctx.addIssue({ code: 'custom', path: ['categories', index, 'starts_on'], message: t.startsOnRange });
      }
    });
  });

export type TournamentFormValues = z.infer<typeof tournamentFormSchema>;
export type CategoryFormValues = TournamentFormValues['categories'][number];

/**
 * A new category row: a level category still to be chosen, groups of 3 with 2 advancing, best of
 * three to 6 games with a match tiebreak, 90 minutes a match.
 */
export const DEFAULT_CATEGORY: CategoryFormValues = {
  kind: 'level',
  starts_on: '',
  starts_at: '',
  prizes: '',
  format: 'groups',
  capacity: 16,
  price: Number.NaN,
  group_size: 3,
  qualifiers_per_group: 2,
  sets_to_win: 2,
  games_per_set: 6,
  super_tiebreak_decider: true,
  match_duration_minutes: 90,
};

type CategoryRequest = CreateTournamentRequest['categories'][number];

/**
 * The modality and the level or suma of a category. The schema has required them before a request is
 * built; these guards narrow the optional form fields for the type.
 */
function traitsOf(
  category: CategoryFormValues,
): Pick<CategoryRequest, 'modality'> & ({ level: number } | { suma: number }) {
  if (!category.modality) throw new Error('a category reaches the request with its modality');
  if (category.kind === 'suma') {
    if (category.suma === undefined) throw new Error('a suma category reaches the request with its suma');
    return { modality: category.modality, suma: category.suma };
  }
  if (category.level === undefined) throw new Error('a level category reaches the request with its level');
  return { modality: category.modality, level: category.level };
}

function categoryRequest(category: CategoryFormValues): CategoryRequest {
  const prizes = category.prizes.trim();
  return {
    ...traitsOf(category),
    starts_on: category.starts_on,
    starts_at: category.starts_at,
    ...(prizes ? { prizes } : {}),
    format: category.format,
    capacity: category.capacity,
    price: pesosToCentavos(category.price),
    ...(category.format === 'groups'
      ? { group_size: category.group_size, qualifiers_per_group: category.qualifiers_per_group }
      : {}),
    sets_to_win: category.sets_to_win,
    games_per_set: category.games_per_set,
    ...(category.sets_to_win === 2 ? { super_tiebreak_decider: category.super_tiebreak_decider } : {}),
    match_duration_minutes: category.match_duration_minutes,
    ...(category.court_fee ? { court_fee: pesosToCentavos(category.court_fee) } : {}),
  };
}

export function toCreateRequest(values: TournamentFormValues): CreateTournamentRequest {
  return {
    sport: values.sport as Sport,
    start_date: values.start_date,
    end_date: values.end_date,
    categories: values.categories.map(categoryRequest),
  };
}

/** The react-hook-form path of one field of the category at `index`. */
export function categoryField<F extends keyof CategoryFormValues>(index: number, field: F) {
  return `categories.${String(index)}.${field}` as `categories.${number}.${F}`;
}
