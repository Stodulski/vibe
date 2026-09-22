import { describe, it, expect } from 'vitest';
import { formatSessionInstant } from './formatCashInstant';

describe('formatSessionInstant', () => {
  it('answers with nothing for null, undefined or an empty string', () => {
    expect(formatSessionInstant(null)).toBe('');
    expect(formatSessionInstant(undefined)).toBe('');
    expect(formatSessionInstant('')).toBe('');
  });

  it('answers with nothing rather than "Invalid Date" for an unparseable string', () => {
    expect(formatSessionInstant('nope')).toBe('');
  });

  it('reads an instant on the venue clock (Buenos Aires, UTC-3), not UTC', () => {
    // 02:00 UTC on the 19th is 23:00 the previous evening in Buenos Aires —
    // both the day and the hour would read differently off the raw UTC value.
    expect(formatSessionInstant('2026-03-19T02:00:00Z')).toBe('18/3, 23:00');
  });

  it('formats as day/month, 24-hour time (es-AR, no AM/PM)', () => {
    expect(formatSessionInstant('2026-01-05T13:30:00-03:00')).toBe('5/1, 13:30');
  });
});
