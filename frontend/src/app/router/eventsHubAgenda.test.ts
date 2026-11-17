import { describe, it, expect } from 'vitest';
import { makeCategory, makeDetail } from '@/features/tournaments/testing/fixtures';
import type { OpenPlayEvent, TournamentOwnerSummary } from '@/shared/types/api.types';
import {
  ALL_AGENDA_FILTER,
  agendaDayOptions,
  agendaModalityOptions,
  buildAgenda,
  filterAgenda,
} from './eventsHubAgenda';

function openPlay(
  id: string,
  date: string,
  startTime = '19:00',
  modality: OpenPlayEvent['modality'] = null,
): OpenPlayEvent {
  return {
    id,
    complex_id: 'c1',
    name: id,
    sport: 'padel',
    date,
    start_time: startTime,
    end_time: '21:00',
    duration_minutes: 120,
    capacity: 8,
    price: 500000,
    status: 'active',
    modality,
    courts: [{ id: 'ct1', name: 'Cancha 1' }],
    confirmed_count: 0,
    held_count: 0,
    spots_left: 8,
    created_at: '2026-11-14T12:00:00Z',
  };
}

function tournament(
  id: string,
  startDate: string,
  overrides: Partial<TournamentOwnerSummary> = {},
): TournamentOwnerSummary {
  return {
    ...makeDetail().tournament,
    id,
    start_date: startDate,
    end_date: startDate,
    unscheduled_count: 0,
    categories: [],
    ...overrides,
  };
}

describe('buildAgenda', () => {
  it('lists upcoming items soonest first, across both kinds', () => {
    const items = buildAgenda([openPlay('ev-late', '2099-01-10')], [tournament('t-early', '2099-01-02')], 'upcoming');
    expect(items.map((item) => item.key)).toEqual(['tournament-t-early', 'openPlay-ev-late']);
  });

  it('lists past items most recent first, across both kinds', () => {
    const items = buildAgenda([openPlay('ev-old', '2020-01-01')], [tournament('t-new', '2020-06-01')], 'past');
    expect(items.map((item) => item.key)).toEqual(['tournament-t-new', 'openPlay-ev-old']);
  });

  it('breaks a same-day tie by start time', () => {
    const items = buildAgenda(
      [openPlay('late', '2099-01-02', '21:00'), openPlay('early', '2099-01-02', '19:00')],
      [],
      'upcoming',
    );
    expect(items.map((item) => item.key)).toEqual(['openPlay-early', 'openPlay-late']);
  });

  it('keeps each item tagged with its kind', () => {
    const [item] = buildAgenda([], [tournament('t1', '2099-01-02')], 'upcoming');
    expect(item?.kind).toBe('tournament');
  });
});

describe('filterAgenda', () => {
  it('keeps every row when no filter narrows', () => {
    const items = buildAgenda([openPlay('ev1', '2099-01-03')], [tournament('t1', '2099-01-02')], 'upcoming');
    expect(filterAgenda(items, ALL_AGENDA_FILTER)).toHaveLength(2);
  });

  it('keeps only the rows of the chosen kind', () => {
    const items = buildAgenda([openPlay('ev1', '2099-01-03')], [tournament('t1', '2099-01-02')], 'upcoming');
    const kept = filterAgenda(items, { ...ALL_AGENDA_FILTER, kind: 'tournament' });
    expect(kept.map((item) => item.key)).toEqual(['tournament-t1']);
  });

  it('matches an open play by its modality, and a tournament by any live category having it', () => {
    const items = buildAgenda(
      [openPlay('men', '2099-01-02', '19:00', 'men'), openPlay('women', '2099-01-03', '19:00', 'women')],
      [
        tournament('t1', '2099-01-04', {
          categories: [
            makeCategory({ id: 'cat-a', modality: 'mixed', status: 'open' }),
            makeCategory({ id: 'cat-b', modality: 'women', status: 'open' }),
          ],
        }),
      ],
      'upcoming',
    );
    const kept = filterAgenda(items, { ...ALL_AGENDA_FILTER, modality: 'women' });
    expect(kept.map((item) => item.key)).toEqual(['openPlay-women', 'tournament-t1']);
  });

  it('does not count a cancelled category toward the modalities of its tournament', () => {
    const items = buildAgenda(
      [],
      [
        tournament('t1', '2099-01-02', {
          categories: [makeCategory({ id: 'cat-a', modality: 'women', status: 'cancelled' })],
        }),
      ],
      'upcoming',
    );
    expect(filterAgenda(items, { ...ALL_AGENDA_FILTER, modality: 'women' })).toEqual([]);
  });

  it('matches an open play on its own day, and a tournament on any day of its range', () => {
    const items = buildAgenda(
      [openPlay('ev1', '2099-01-03')],
      [tournament('t1', '2099-01-02', { end_date: '2099-01-04' })],
      'upcoming',
    );
    const onDay = (day: string) => filterAgenda(items, { ...ALL_AGENDA_FILTER, day }).map((item) => item.key);

    expect(onDay('2099-01-02')).toEqual(['tournament-t1']);
    expect(onDay('2099-01-03')).toEqual(['tournament-t1', 'openPlay-ev1']);
    expect(onDay('2099-01-05')).toEqual([]);
  });
});

describe('agendaModalityOptions', () => {
  it('offers each modality once, in the order of the modalities', () => {
    const items = buildAgenda(
      [openPlay('women', '2099-01-02', '19:00', 'women')],
      [tournament('t1', '2099-01-03', { categories: [makeCategory({ modality: 'men', status: 'open' })] })],
      'upcoming',
    );
    expect(agendaModalityOptions(items)).toEqual([
      { value: 'men', label: 'Caballeros' },
      { value: 'women', label: 'Damas' },
    ]);
  });

  it('names a modality in the words of each sport that has it', () => {
    const items = buildAgenda(
      [openPlay('padel', '2099-01-02', '19:00', 'men')],
      [
        tournament('soccer', '2099-01-03', {
          sport: 'soccer',
          categories: [makeCategory({ modality: 'men', status: 'open' })],
        }),
      ],
      'upcoming',
    );
    expect(agendaModalityOptions(items)).toEqual([{ value: 'men', label: 'Caballeros / Masculino' }]);
  });
});

describe('agendaDayOptions', () => {
  it('offers each day a row starts on once, soonest first, labelled as a date block reads', () => {
    const items = buildAgenda(
      [openPlay('a', '2026-11-18'), openPlay('b', '2026-11-18', '21:00')],
      [tournament('t', '2026-01-05')],
      'upcoming',
    );
    expect(agendaDayOptions(items)).toEqual([
      { date: '2026-01-05', label: 'Lun 5 ene' },
      { date: '2026-11-18', label: 'Mié 18 nov' },
    ]);
  });
});
