import { differenceInCalendarDays } from 'date-fns/differenceInCalendarDays';
import { parseISO } from 'date-fns/parseISO';
import { z } from 'zod';
import { ES_AR } from '@/shared/i18n/es_AR';
import { pesosToCentavos } from '@/shared/lib/money';
import type { CreateTournamentRequest, Sport } from '@/shared/types/api.types';

const t = ES_AR.tournaments.validation;

const MAX_DAYS = 60;

const categoryFormSchema = z
  .object({
    name: z.string().trim().min(1, t.categoryNameRequired).max(30, t.categoryNameMax),
    format: z.enum(['elimination', 'groups']),
    capacity: z
      .number({ error: t.capacityRange })
      .int(t.capacityRange)
      .min(2, t.capacityRange)
      .max(128, t.capacityRange),
    price: z.number({ error: t.priceRequired }).positive(t.pricePositive),
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
 * pesos here, centavos on the wire. Group settings only count for a `groups`
 * category, and the match tiebreak only for best of three.
 */
export const tournamentFormSchema = z
  .object({
    name: z.string().trim().min(1, t.nameRequired).max(60, t.nameMax),
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
  });

export type TournamentFormValues = z.infer<typeof tournamentFormSchema>;
export type CategoryFormValues = TournamentFormValues['categories'][number];

/** A new category row: groups of 3 with 2 advancing, best of three to 6 games with a match tiebreak, 90 minutes a match. */
export const DEFAULT_CATEGORY: CategoryFormValues = {
  name: '',
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

function categoryRequest(category: CategoryFormValues): CategoryRequest {
  return {
    name: category.name.trim(),
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
  };
}

export function toCreateRequest(values: TournamentFormValues): CreateTournamentRequest {
  return {
    name: values.name.trim(),
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
