import { describe, expect, it } from 'vitest';
import type { PublicTournamentCategory, PublicTournamentSummary } from '../../api/types';
import { CAT_OPEN, TOURNAMENT } from '../../test/tournamentFixtures';
import { dayChips, dayParts } from '../agenda/agendaModel';
import { agendaRows, categoryWhen, formatDetailLines, rowWhenTag, spotsNotice } from './agendaModel';

const men8: PublicTournamentCategory = {
  ...CAT_OPEN,
  id: 'c-men8',
  name: 'stored',
  modality: 'men',
  level: 8,
  suma: null,
  prizes: 'Trofeo y 2 pelotas por pareja',
  starts_on: '2099-12-06',
  starts_at: '18:00',
  spots_left: 5,
  status: 'open',
};

const women5: PublicTournamentCategory = {
  ...men8,
  id: 'c-women5',
  modality: 'women',
  level: 5,
  starts_on: '2099-12-05',
  starts_at: '10:00',
  spots_left: 2,
};

const mixedSuma: PublicTournamentCategory = {
  ...men8,
  id: 'c-suma',
  modality: 'mixed',
  level: null,
  suma: 15,
  starts_on: '2099-12-05',
  starts_at: '18:00',
};

const summary = (categories: PublicTournamentCategory[], tournament = TOURNAMENT): PublicTournamentSummary => ({
  ...tournament,
  categories,
});

describe('agendaRows', () => {
  it('lists only the categories open for sign-up with spots left', () => {
    const closed = { ...men8, id: 'c-closed', status: 'closed' as const };
    const full = { ...men8, id: 'c-full', spots_left: 0 };
    const cancelled = { ...men8, id: 'c-cancelled', status: 'cancelled' as const };

    const rows = agendaRows([summary([men8, closed, full, cancelled])]);

    expect(rows.map((row) => row.category.id)).toEqual(['c-men8']);
  });

  it('lists nothing from a cancelled tournament', () => {
    const cancelled = { ...TOURNAMENT, status: 'cancelled' as const };

    expect(agendaRows([summary([men8], cancelled)])).toEqual([]);
  });

  it('sorts by day, then time, then title', () => {
    const late = { ...men8, id: 'c-late', starts_on: '2099-12-05', starts_at: '20:00', name: 'Z' };
    const early = { ...men8, id: 'c-early', starts_on: '2099-12-05', starts_at: '09:00' };

    const rows = agendaRows([summary([men8, late, early])]);

    expect(rows.map((row) => row.category.id)).toEqual(['c-early', 'c-late', 'c-men8']);
  });

  it('gives a category without a start the first day of its tournament and no time', () => {
    const legacy = agendaRows([summary([CAT_OPEN])])[0];

    expect(legacy).toMatchObject({ date: '2099-12-05', time: null, title: '8va' });
  });

  it('composes the title from the traits and labels the row with its sport', () => {
    const row = agendaRows([summary([men8])])[0];

    expect(row).toMatchObject({ title: 'Caballeros 8va', label: 'Torneo de pádel', time: '18:00', date: '2099-12-06' });
  });
});

describe('dayParts', () => {
  it('reads the weekday, day and month of a date without moving it by time zone', () => {
    expect(dayParts('2099-12-05')).toEqual({ weekday: 'Sáb', weekdayLong: 'sábado', day: '5', month: 'dic' });
    expect(dayParts('2099-12-06')).toEqual({ weekday: 'Dom', weekdayLong: 'domingo', day: '6', month: 'dic' });
  });
});

describe('dayChips', () => {
  it('offers each listed date once, in order, as weekday and day', () => {
    const rows = agendaRows([summary([men8, women5, mixedSuma])]);

    expect(dayChips(rows)).toEqual([
      { date: '2099-12-05', label: 'Sáb 5 dic' },
      { date: '2099-12-06', label: 'Dom 6 dic' },
    ]);
  });
});


describe('spotsNotice', () => {
  it('says how many places are left while there are more than two', () => {
    expect(spotsNotice(5)).toEqual({ text: 'Quedan 5 lugares', last: false });
  });

  it('says the last places, highlighted, when two or fewer remain', () => {
    expect(spotsNotice(2)).toEqual({ text: 'Últimos lugares', last: true });
    expect(spotsNotice(1)).toEqual({ text: 'Últimos lugares', last: true });
  });
});

describe('formatDetailLines', () => {
  it('lists the groups format, their size and how many pass, one a line', () => {
    expect(formatDetailLines({ ...CAT_OPEN, format: 'groups', group_size: 4, qualifiers_per_group: 2 })).toEqual([
      'Zonas y cuadro',
      'Zonas de 4 parejas',
      'Pasan 2 por zona',
    ]);
  });

  it('lists an elimination as its format alone', () => {
    expect(formatDetailLines({ ...CAT_OPEN, format: 'elimination' })).toEqual(['Eliminación directa']);
  });
});

describe('rowWhenTag', () => {
  const oneDay = { ...TOURNAMENT, start_date: '2099-12-05', end_date: '2099-12-05' };
  const month = { ...TOURNAMENT, start_date: '2099-11-18', end_date: '2099-12-12' };

  it('shows the hours of a tournament played in one day', () => {
    const [row] = agendaRows([{ ...oneDay, categories: [{ ...CAT_OPEN, starts_on: '2099-12-05', starts_at: '18:00' }] }]);
    expect(row && rowWhenTag(row)).toBe('18:00 hs');
  });

  it('shows the last day of a tournament played over several days, instead of the hours', () => {
    const [row] = agendaRows([{ ...month, categories: [{ ...CAT_OPEN, starts_on: '2099-11-18', starts_at: '18:00' }] }]);
    expect(row && rowWhenTag(row)).toBe('Hasta 12/12');
  });

  it('shows the hours of a category that starts on its tournament last day', () => {
    const [row] = agendaRows([{ ...month, categories: [{ ...CAT_OPEN, starts_on: '2099-12-12', starts_at: '10:00' }] }]);
    expect(row && rowWhenTag(row)).toBe('10:00 hs');
  });

  it('shows nothing for a one-day category without a time', () => {
    const [row] = agendaRows([{ ...oneDay, categories: [{ ...CAT_OPEN, starts_on: null, starts_at: null }] }]);
    expect(row && rowWhenTag(row)).toBeUndefined();
  });
});

describe('categoryWhen', () => {
  it('writes the day and the time as the detail reads them', () => {
    const row = agendaRows([summary([men8])])[0];

    expect(row && categoryWhen(row)).toBe('Domingo 06/12 · 18:00 hs');
  });

  it('writes a day without a time alone', () => {
    const row = agendaRows([summary([CAT_OPEN])])[0];

    expect(row && categoryWhen(row)).toBe('Sábado 05/12');
  });
});
