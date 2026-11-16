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
  return {
    ...DEFAULT_CATEGORY,
    modality: 'men',
    kind: 'level',
    level: 6,
    starts_on: '2026-12-05',
    starts_at: '18:00',
    price: 20000,
    ...overrides,
  };
}

function form(overrides: Partial<TournamentFormValues> = {}): TournamentFormValues {
  return {
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

  it('asks for at least one category and no tournament name', () => {
    expect(messages(form({ categories: [] }))).toEqual([t.categoriesRequired]);
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

  it('keeps the capacity, price, games and duration in range', () => {
    const bad = category({ capacity: 1, price: Number.NaN, games_per_set: 3, match_duration_minutes: 20 });
    expect(messages(form({ categories: [bad] }))).toEqual([
      t.capacityRange,
      t.priceRequired,
      t.gamesRange,
      t.durationRange,
    ]);
  });
});

describe('tournamentFormSchema: category traits and dates', () => {
  it('needs a modality: a category is never created without one', () => {
    expect(messages(form({ categories: [category({ modality: undefined })] }))).toEqual([t.modalityRequired]);
  });

  it('needs a level for a level category, and a suma between 2 and 18 for a suma', () => {
    expect(messages(form({ categories: [category({ level: undefined })] }))).toEqual([t.levelRequired]);
    expect(messages(form({ categories: [category({ kind: 'suma', level: undefined, suma: undefined })] }))).toEqual([
      t.sumaRange,
    ]);
    expect(messages(form({ categories: [category({ kind: 'suma', level: undefined, suma: 1 })] }))).toEqual([
      t.sumaRange,
    ]);
    expect(messages(form({ categories: [category({ kind: 'suma', level: undefined, suma: 19 })] }))).toEqual([
      t.sumaRange,
    ]);
    expect(messages(form({ categories: [category({ kind: 'suma', level: undefined, suma: 18 })] }))).toEqual([]);
  });

  it('needs the day and the time the category starts', () => {
    expect(messages(form({ categories: [category({ starts_on: '' })] }))).toEqual([t.startsOnRequired]);
    expect(messages(form({ categories: [category({ starts_at: '' })] }))).toEqual([t.startsAtRequired]);
  });

  it('keeps the category day inside the tournament days, both ends included', () => {
    expect(messages(form({ categories: [category({ starts_on: '2026-12-05' })] }))).toEqual([]);
    expect(messages(form({ categories: [category({ starts_on: '2026-12-07' })] }))).toEqual([]);
    expect(messages(form({ categories: [category({ starts_on: '2026-12-08' })] }))).toEqual([t.startsOnRange]);
    expect(messages(form({ categories: [category({ starts_on: '2026-12-04' })] }))).toEqual([t.startsOnRange]);
  });

  it('caps the prizes at 200 characters and accepts exactly 200', () => {
    expect(messages(form({ categories: [category({ prizes: 'a'.repeat(200) })] }))).toEqual([]);
    expect(messages(form({ categories: [category({ prizes: 'a'.repeat(201) })] }))).toEqual([t.prizesMax]);
  });
});

describe('toCreateRequest', () => {
  it('sends no tournament name and composes nothing: the sport, the days and the categories', () => {
    const request = toCreateRequest(form({ sport: 'padel' }));
    expect(request).not.toHaveProperty('name');
    expect(request).toMatchObject({ sport: 'padel', start_date: '2026-12-05', end_date: '2026-12-07' });
  });

  it('sends the price in centavos and the group settings only for a groups category', () => {
    const request = toCreateRequest(
      form({
        categories: [
          category({ format: 'groups', group_size: 4, qualifiers_per_group: 2, price: 20000.5 }),
          category({ modality: 'mixed', level: 7, format: 'elimination' }),
        ],
      }),
    );
    expect(request.categories[0]).toMatchObject({
      format: 'groups',
      price: 2000050,
      group_size: 4,
      qualifiers_per_group: 2,
    });
    expect(request.categories[1]).not.toHaveProperty('group_size');
    expect(request.categories[1]).not.toHaveProperty('qualifiers_per_group');
  });
});

describe('toCreateRequest: traits', () => {
  it('sends the modality, the level, the start and the prizes, and never a name', () => {
    const request = toCreateRequest(
      form({
        categories: [category({ modality: 'mixed', level: 6, prizes: '  Trofeo  ', starts_at: '19:30' })],
      }),
    );
    expect(request.categories[0]).not.toHaveProperty('name');
    expect(request.categories[0]).toMatchObject({
      modality: 'mixed',
      level: 6,
      starts_on: '2026-12-05',
      starts_at: '19:30',
      prizes: 'Trofeo',
    });
    expect(request.categories[0]).not.toHaveProperty('suma');
  });

  it('sends a suma instead of a level, and omits prizes when there are none', () => {
    const request = toCreateRequest(
      form({
        categories: [category({ modality: 'women', kind: 'suma', level: undefined, suma: 15, prizes: '   ' })],
      }),
    );
    expect(request.categories[0]).toMatchObject({ modality: 'women', suma: 15 });
    expect(request.categories[0]).not.toHaveProperty('level');
    expect(request.categories[0]).not.toHaveProperty('prizes');
  });

  it('sends the match format, and no match tiebreak for a single set', () => {
    const request = toCreateRequest(
      form({
        categories: [
          category({ sets_to_win: 1, games_per_set: 4, match_duration_minutes: 60 }),
          category({ modality: 'women', level: 7, super_tiebreak_decider: false }),
        ],
      }),
    );
    expect(request.categories[0]).toMatchObject({ sets_to_win: 1, games_per_set: 4, match_duration_minutes: 60 });
    expect(request.categories[0]).not.toHaveProperty('super_tiebreak_decider');
    expect(request.categories[1]).toMatchObject({ sets_to_win: 2, super_tiebreak_decider: false });
  });
});
