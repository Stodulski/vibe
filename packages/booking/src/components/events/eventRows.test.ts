import { describe, expect, it } from 'vitest';
import type { PublicOpenPlayEvent, PublicTournamentCategory, PublicTournamentSummary } from '../../api/types';
import { EVENT } from '../../test/eventFixtures';
import { CAT_OPEN, TOURNAMENT } from '../../test/tournamentFixtures';
import { agendaRows } from '../tournaments/agendaModel';
import { eventDayChips, eventModalityOptions, eventRows, filterEventRows, type EventRow } from './eventRows';
import { openPlayRows } from './openPlayAgendaModel';

/** A pádel open play for anyone, on the day and the hour given, or for the modality given. */
function openPlay(
  id: string,
  date: string,
  start: string,
  modality?: PublicOpenPlayEvent['modality'],
): PublicOpenPlayEvent {
  return { ...EVENT, id, date, start_time: start, end_time: '23:00', ...(modality ? { modality } : {}) };
}

/** A pádel category open for sign-up with five places left. A `null` day or time is a legacy category. */
function category(
  id: string,
  date: string | null,
  time: string | null,
  modality: 'men' | 'women',
): PublicTournamentCategory {
  return {
    ...CAT_OPEN,
    id,
    modality,
    level: 6,
    suma: null,
    prizes: null,
    status: 'open',
    spots_left: 5,
    starts_on: date,
    starts_at: time,
  };
}

function tournamentRows(...categories: PublicTournamentCategory[]) {
  const tournament: PublicTournamentSummary = { ...TOURNAMENT, categories };
  return agendaRows([tournament]);
}

/** The id of a row, whichever kind it is. */
function idOf(row: EventRow): string {
  return row.kind === 'open-play' ? row.row.event.id : row.row.category.id;
}

describe('eventRows', () => {
  it('lists open play and tournaments in one list, soonest first', () => {
    const rows = eventRows(
      openPlayRows([openPlay('o-sat', '2099-12-05', '19:00')]),
      tournamentRows(
        category('t-sun', '2099-12-06', '18:00', 'men'),
        category('t-sat-10', '2099-12-05', '10:00', 'women'),
      ),
    );

    expect(rows.map((row) => row.kind)).toEqual(['tournament', 'open-play', 'tournament']);
    expect(rows.map(idOf)).toEqual(['t-sat-10', 'o-sat', 't-sun']);
  });

  it('puts a row without a start time after the timed rows of its day', () => {
    const rows = eventRows(
      openPlayRows([openPlay('o-sat-late', '2099-12-05', '22:00')]),
      tournamentRows(category('t-sat-none', '2099-12-05', null, 'men')),
    );

    expect(rows.map(idOf)).toEqual(['o-sat-late', 't-sat-none']);
  });

  it('keeps a legacy tournament row on its tournament day', () => {
    const rows = eventRows([], tournamentRows(category('t-legacy', null, null, 'men')));

    expect(rows.map(idOf)).toEqual(['t-legacy']);
    expect(rows[0]?.row.date).toBe('2099-12-05');
  });
});

describe('filterEventRows', () => {
  const rows = eventRows(
    openPlayRows([openPlay('o-any', '2099-12-05', '19:00'), openPlay('o-men', '2099-12-05', '20:00', 'men')]),
    tournamentRows(category('t-women', '2099-12-06', '10:00', 'women')),
  );

  it('keeps the rows of one modality from both kinds, and a row without one only under all', () => {
    expect(filterEventRows(rows, { modality: 'men', day: null }).map(idOf)).toEqual(['o-men']);
    expect(filterEventRows(rows, { modality: 'women', day: null }).map(idOf)).toEqual(['t-women']);
    expect(filterEventRows(rows, { modality: 'all', day: null })).toHaveLength(3);
  });

  it('keeps the rows of one day from both kinds', () => {
    expect(filterEventRows(rows, { modality: 'all', day: '2099-12-06' }).map(idOf)).toEqual(['t-women']);
    expect(filterEventRows(rows, { modality: 'men', day: '2099-12-06' })).toEqual([]);
  });
});

describe('eventModalityOptions and eventDayChips', () => {
  const rows = eventRows(
    openPlayRows([openPlay('o-men', '2099-12-05', '19:00', 'men')]),
    tournamentRows(category('t-women', '2099-12-06', '10:00', 'women')),
  );

  it('names the modalities of both kinds in the words of their sport', () => {
    expect(eventModalityOptions(rows)).toEqual([
      { value: 'men', label: 'Caballeros' },
      { value: 'women', label: 'Damas' },
    ]);
  });

  it('offers each day of both kinds once, in order', () => {
    expect(eventDayChips(rows)).toEqual([
      { date: '2099-12-05', label: 'Sáb 5 dic' },
      { date: '2099-12-06', label: 'Dom 6 dic' },
    ]);
  });
});
