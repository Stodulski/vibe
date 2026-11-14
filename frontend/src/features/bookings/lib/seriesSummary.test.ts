import { describe, it, expect } from 'vitest';
import { seriesSummary } from './seriesSummary';

// 2099-01-05 is a Monday.
const base = { date: '2099-01-05', startTime: '18:00', durationMinutes: 90, courtName: 'Cancha 1', end: {} };

describe('seriesSummary', () => {
  it('says the weekday, the hours, the court and when it starts, for a series with no end', () => {
    expect(seriesSummary(base)).toBe('Todos los lunes de 18:00 a 19:30 en Cancha 1, desde el 05/01');
  });

  it('adds "hasta el" for a series that ends on a date', () => {
    expect(seriesSummary({ ...base, end: { ends_on: '2099-03-30' } })).toBe(
      'Todos los lunes de 18:00 a 19:30 en Cancha 1, desde el 05/01 hasta el 30/03',
    );
  });

  it('adds the last date of a series counted in weeks', () => {
    // Four weeks: the 5th, 12th, 19th and 26th of January.
    expect(seriesSummary({ ...base, end: { weeks: 4 } })).toBe(
      'Todos los lunes de 18:00 a 19:30 en Cancha 1, desde el 05/01 hasta el 26/01',
    );
  });

  it('writes an end past midnight as the clock reads', () => {
    expect(seriesSummary({ ...base, startTime: '23:00', durationMinutes: 120 })).toContain('de 23:00 a 01:00');
  });
});
