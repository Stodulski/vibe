import { describe, it, expect } from 'vitest';
import { seriesSummaryRows } from './seriesSummary';

// 2099-01-05 is a Monday.
const base = { date: '2099-01-05', startTime: '18:00', durationMinutes: 90, courtName: 'Cancha 1', end: {} };

describe('seriesSummaryRows', () => {
  it('lists the weekday, the hours, the court and the first date, one fact per row', () => {
    expect(seriesSummaryRows(base)).toEqual([
      { label: 'Repite', value: 'Todos los lunes' },
      { label: 'Horario', value: '18:00 a 19:30' },
      { label: 'Cancha', value: 'Cancha 1' },
      { label: 'Desde', value: '05/01/2099' },
      { label: 'Hasta', value: 'Sin fecha de fin' },
    ]);
  });

  it('gives the last date of a series that ends on a date', () => {
    expect(seriesSummaryRows({ ...base, end: { ends_on: '2099-03-30' } }).at(-1)).toEqual({
      label: 'Hasta',
      value: '30/03/2099',
    });
  });

  it('gives the last date of a series counted in weeks', () => {
    // Four weeks: the 5th, 12th, 19th and 26th of January.
    expect(seriesSummaryRows({ ...base, end: { weeks: 4 } }).at(-1)?.value).toBe('26/01/2099');
  });

  it('writes an end past midnight as the clock reads', () => {
    expect(seriesSummaryRows({ ...base, startTime: '23:00', durationMinutes: 120 })[1]?.value).toBe('23:00 a 01:00');
  });
});
