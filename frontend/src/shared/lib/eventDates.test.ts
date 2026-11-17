import { describe, it, expect } from 'vitest';
import { eventDayParts, formatEventDay, formatEventTimeRange, formatEventWhen } from './eventDates';

describe('event dates', () => {
  it('splits a day into the short weekday, the number without a leading zero, and the short month', () => {
    expect(eventDayParts('2026-11-18')).toEqual({ weekday: 'Mié', day: '18', month: 'nov' });
    expect(eventDayParts('2026-01-05')).toEqual({ weekday: 'Lun', day: '5', month: 'ene' });
  });

  it('writes the day as weekday, number and month', () => {
    expect(formatEventDay('2026-11-20')).toBe('viernes 20 de noviembre');
  });

  it('writes the hours with an en dash', () => {
    expect(formatEventTimeRange('19:00', '21:00')).toBe('19:00 a 21:00');
  });

  it('joins both the way the cards read', () => {
    expect(formatEventWhen('2026-11-20', '19:00', '21:00')).toBe('viernes 20 de noviembre · 19:00 a 21:00');
  });
});
