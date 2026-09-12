import { describe, it, expect } from 'vitest';
import { buildWeekSchedule } from './weekSchedule';
import type { Schedule } from '@/shared/types/api.types';

function day(name: string, open: string, close: string, closed = false): Schedule {
  return {
    id: name,
    complex_id: 'c1',
    day: name,
    open_time: open,
    close_time: close,
    is_closed: closed,
  } as Schedule;
}

const EVERY_DAY = ['monday', 'tuesday', 'wednesday', 'thursday', 'friday', 'saturday', 'sunday'];

describe('buildWeekSchedule', () => {
  it('returns every day of the week, Monday first', () => {
    const rows = buildWeekSchedule(EVERY_DAY.map((d) => day(d, '08:00', '23:00')));

    // Seven rows, never merged into ranges: someone checking Wednesday finds
    // Wednesday rather than working out that they fall inside "Lunes a
    // Miércoles". Monday first because a week that opens on Sunday splits the
    // weekend across both ends of the list.
    expect(rows.map((r) => r.label)).toEqual([
      'Lunes',
      'Martes',
      'Miércoles',
      'Jueves',
      'Viernes',
      'Sábado',
      'Domingo',
    ]);
  });

  it('gives each day its own hours', () => {
    const rows = buildWeekSchedule(
      EVERY_DAY.map((d) => (d === 'thursday' ? day(d, '08:00', '01:30') : day(d, '08:00', '23:00'))),
    );

    expect(rows.find((r) => r.day === 'thursday')?.hours).toBe('08:00 - 01:30');
    expect(rows.find((r) => r.day === 'friday')?.hours).toBe('08:00 - 23:00');
  });

  it('says a closed day is closed rather than leaving it out', () => {
    const rows = buildWeekSchedule(
      EVERY_DAY.map((d) => (d === 'sunday' ? day(d, '08:00', '23:00', true) : day(d, '08:00', '23:00'))),
    );

    // A missing Sunday reads as an oversight; "Cerrado" is an answer.
    expect(rows.find((r) => r.day === 'sunday')?.hours).toBe('Cerrado');
  });

  it('treats a day the venue never configured as closed', () => {
    const rows = buildWeekSchedule([day('monday', '08:00', '23:00')]);

    expect(rows.find((r) => r.day === 'monday')?.hours).toBe('08:00 - 23:00');
    expect(rows.find((r) => r.day === 'tuesday')?.hours).toBe('Cerrado');
  });

  it('marks exactly one row as today', () => {
    const rows = buildWeekSchedule(EVERY_DAY.map((d) => day(d, '08:00', '23:00')));

    expect(rows.filter((r) => r.isToday)).toHaveLength(1);
  });

  // `highlightDay` lets the caller bold whatever day the slot picker is
  // actually showing (`?date=` in the URL) instead of the real today.
  it('highlights the given day instead of today when highlightDay is passed', () => {
    const rows = buildWeekSchedule(
      EVERY_DAY.map((d) => day(d, '08:00', '23:00')),
      'saturday',
    );

    expect(rows.filter((r) => r.isToday)).toHaveLength(1);
    expect(rows.find((r) => r.isToday)?.day).toBe('saturday');
  });
});
