import { describe, it, expect } from 'vitest';
import { shortDayLabel } from './shortDayLabel';

describe('shortDayLabel', () => {
  it('abbreviates weekday and month so it fits a 320px header', () => {
    expect(shortDayLabel('2026-12-07')).toBe('lun 7 dic');
  });

  it('keeps a two-digit day', () => {
    expect(shortDayLabel('2026-11-15')).toBe('dom 15 nov');
  });
});
