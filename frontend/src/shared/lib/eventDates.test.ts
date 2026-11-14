import { describe, it, expect } from 'vitest';
import { formatEventDay, formatEventTimeRange, formatEventWhen } from './eventDates';

describe('event dates', () => {
  it('writes the day as weekday, number and month', () => {
    expect(formatEventDay('2026-11-20')).toBe('viernes 20 de noviembre');
  });

  it('writes the hours with an en dash', () => {
    expect(formatEventTimeRange('19:00', '21:00')).toBe('19:00 – 21:00');
  });

  it('joins both the way the cards read', () => {
    expect(formatEventWhen('2026-11-20', '19:00', '21:00')).toBe('viernes 20 de noviembre · 19:00 – 21:00');
  });
});
