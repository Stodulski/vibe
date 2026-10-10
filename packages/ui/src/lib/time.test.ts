import { describe, it, expect } from 'vitest';
import { parseHhMm, parseYmd, timeToMinutes, generateTimeSlots, addMinutes } from './time';

describe('parseHhMm', () => {
  it('parses a valid HH:MM string into hours and minutes', () => {
    expect(parseHhMm('09:30')).toEqual({ hours: 9, minutes: 30 });
  });

  it('parses midnight and near-midnight boundaries', () => {
    expect(parseHhMm('00:00')).toEqual({ hours: 0, minutes: 0 });
    expect(parseHhMm('23:59')).toEqual({ hours: 23, minutes: 59 });
  });

  it('returns null for a string missing the minutes segment', () => {
    expect(parseHhMm('09')).toBeNull();
  });

  it('returns null for a string with too many segments', () => {
    expect(parseHhMm('09:30:00')).toBeNull();
  });

  it('returns null when either segment is not numeric', () => {
    expect(parseHhMm('aa:30')).toBeNull();
    expect(parseHhMm('09:bb')).toBeNull();
  });

  it('returns null for an empty string', () => {
    expect(parseHhMm('')).toBeNull();
  });
});

describe('parseYmd', () => {
  it('parses a valid YYYY-MM-DD string into year, month, and day', () => {
    expect(parseYmd('2026-03-15')).toEqual({ year: 2026, month: 3, day: 15 });
  });

  it('returns null for a string missing segments', () => {
    expect(parseYmd('2026-03')).toBeNull();
  });

  it('returns null when a segment is not numeric', () => {
    expect(parseYmd('2026-xx-15')).toBeNull();
  });

  it('returns null for an empty string', () => {
    expect(parseYmd('')).toBeNull();
  });
});

describe('timeToMinutes', () => {
  it('converts a mid-day HH:MM string to minutes since midnight', () => {
    expect(timeToMinutes('09:30')).toBe(570);
  });

  it('converts midnight to 0', () => {
    expect(timeToMinutes('00:00')).toBe(0);
  });

  it('returns 0 for a malformed string (invariant: never hit with real API/UI data)', () => {
    expect(timeToMinutes('not-a-time')).toBe(0);
  });
});

describe('generateTimeSlots', () => {
  it('generates 30-minute slots for a bounded range', () => {
    expect(generateTimeSlots('08:00', '10:00')).toEqual(['08:00', '08:30', '09:00', '09:30']);
  });

  it('generates 30-minute slots for the default full-day range', () => {
    const slots = generateTimeSlots();
    expect(slots[0]).toBe('00:00');
    expect(slots[slots.length - 1]).toBe('23:30');
    expect(slots).toHaveLength(48);
  });

  it('returns an empty array for a malformed bound (invariant: never hit with real UI defaults)', () => {
    expect(generateTimeSlots('not-a-time', '10:00')).toEqual([]);
  });
});

describe('addMinutes', () => {
  it('adds minutes within the same hour', () => {
    expect(addMinutes('09:00', 15)).toBe('09:15');
  });

  it('rolls over to the next hour', () => {
    expect(addMinutes('09:45', 30)).toBe('10:15');
  });

  it('wraps past midnight (24h format)', () => {
    expect(addMinutes('23:30', 90)).toBe('01:00');
  });

  it('returns the malformed input unchanged (invariant: never hit with real API/UI data)', () => {
    expect(addMinutes('not-a-time', 15)).toBe('not-a-time');
  });
});
