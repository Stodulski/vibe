import { describe, it, expect } from 'vitest';
import { ES_AR } from '@/shared/i18n/es_AR';
import {
  DEFAULT_CATEGORY,
  tournamentFormSchema,
  toCreateRequest,
  type CategoryFormValues,
  type TournamentFormValues,
} from './tournamentForm.schema';

const t = ES_AR.tournaments.validation;

function category(overrides: Partial<CategoryFormValues> = {}): CategoryFormValues {
  return { ...DEFAULT_CATEGORY, name: '6ta', price: 20000, ...overrides };
}

function form(overrides: Partial<TournamentFormValues> = {}): TournamentFormValues {
  return {
    name: 'Torneo de primavera',
    sport: 'padel',
    start_date: '2026-12-05',
    end_date: '2026-12-07',
    categories: [category()],
    ...overrides,
  };
}

function messages(values: TournamentFormValues): string[] {
  const result = tournamentFormSchema.safeParse(values);
  return result.success ? [] : result.error.issues.map((issue) => issue.message);
}

describe('tournamentFormSchema', () => {
  it('accepts a valid tournament with the defaults: groups of 3, 2 advance, best of three to 6, 90 minutes', () => {
    expect(DEFAULT_CATEGORY).toMatchObject({
      group_size: 3,
      qualifiers_per_group: 2,
      sets_to_win: 2,
      games_per_set: 6,
      super_tiebreak_decider: true,
      match_duration_minutes: 90,
    });
    expect(messages(form())).toEqual([]);
  });

  it('asks for the name and at least one category', () => {
    expect(messages(form({ name: '  ', categories: [] }))).toEqual([t.nameRequired, t.categoriesRequired]);
  });

  it('refuses a last day before the first and a tournament over 60 days', () => {
    expect(messages(form({ end_date: '2026-12-04' }))).toEqual([t.endBeforeStart]);
    expect(messages(form({ end_date: '2027-02-04' }))).toEqual([t.tooLong]);
  });

  it('refuses qualifiers that are not below the group size', () => {
    const groups = (size: number, qualifiers: number) =>
      messages(
        form({ categories: [category({ format: 'groups', group_size: size, qualifiers_per_group: qualifiers })] }),
      );
    expect(groups(3, 3)).toEqual([t.qualifiersRange]);
    expect(groups(4, 4)).toEqual([t.qualifiersRange]);
    expect(groups(3, 0)).toEqual([t.qualifiersRange]);
    expect(groups(4, 3)).toEqual([]);
    expect(groups(13, 2)).toEqual([t.groupSizeRange]);
    expect(groups(1, 1)).toContain(t.groupSizeRange);
  });

  it('does not check the group settings of an elimination category', () => {
    expect(
      messages(form({ categories: [category({ format: 'elimination', group_size: 1, qualifiers_per_group: 9 })] })),
    ).toEqual([]);
  });

  it('keeps the category name, capacity, price, games and duration in range', () => {
    const bad = category({ name: '', capacity: 1, price: Number.NaN, games_per_set: 3, match_duration_minutes: 20 });
    expect(messages(form({ categories: [bad] }))).toEqual([
      t.categoryNameRequired,
      t.capacityRange,
      t.priceRequired,
      t.gamesRange,
      t.durationRange,
    ]);
  });
});

describe('toCreateRequest', () => {
  it('sends the price in centavos and the group settings only for a groups category', () => {
    const request = toCreateRequest(
      form({
        name: ' Copa ',
        categories: [
          category({ format: 'groups', group_size: 4, qualifiers_per_group: 2, price: 20000.5 }),
          category({ name: 'Mixto', format: 'elimination' }),
        ],
      }),
    );
    expect(request.name).toBe('Copa');
    expect(request.categories[0]).toMatchObject({
      format: 'groups',
      price: 2000050,
      group_size: 4,
      qualifiers_per_group: 2,
    });
    expect(request.categories[1]).not.toHaveProperty('group_size');
    expect(request.categories[1]).not.toHaveProperty('qualifiers_per_group');
  });

  it('sends the match format, and no match tiebreak for a single set', () => {
    const request = toCreateRequest(
      form({
        categories: [
          category({ sets_to_win: 1, games_per_set: 4, match_duration_minutes: 60 }),
          category({ name: '7ma', super_tiebreak_decider: false }),
        ],
      }),
    );
    expect(request.categories[0]).toMatchObject({ sets_to_win: 1, games_per_set: 4, match_duration_minutes: 60 });
    expect(request.categories[0]).not.toHaveProperty('super_tiebreak_decider');
    expect(request.categories[1]).toMatchObject({ sets_to_win: 2, super_tiebreak_decider: false });
  });
});
