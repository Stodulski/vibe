import { describe, it, expect } from 'vitest';
import { timeOptionsForOpeningHours } from './timeOptions';

const day = (open: string, close: string, closed = false) => ({
  open_time: open,
  close_time: close,
  is_closed: closed,
});

describe('timeOptionsForOpeningHours', () => {
  it('offers every half hour from opening to closing, both included', () => {
    const options = timeOptionsForOpeningHours([day('08:00', '23:00')]);
    expect(options[0]).toBe('08:00');
    expect(options.at(-1)).toBe('23:00');
    expect(options).toHaveLength(31);
    expect(options).not.toContain('07:30');
    expect(options).not.toContain('23:30');
  });

  it('spans the widest hours of the open days and ignores closed ones', () => {
    const options = timeOptionsForOpeningHours([
      day('09:00', '22:00'),
      day('08:00', '23:00'),
      day('00:00', '00:00', true),
    ]);
    expect(options[0]).toBe('08:00');
    expect(options.at(-1)).toBe('23:00');
  });

  it('keeps going past midnight when the complex closes after it', () => {
    const options = timeOptionsForOpeningHours([day('18:00', '02:00')]);
    expect(options.slice(0, 2)).toEqual(['18:00', '18:30']);
    expect(options).toContain('23:30');
    expect(options).toContain('00:00');
    expect(options.at(-1)).toBe('02:00');
    expect(options).not.toContain('09:00');
    expect(new Set(options).size).toBe(options.length);
  });

  it('offers the whole day when there is no schedule or every day is closed', () => {
    expect(timeOptionsForOpeningHours([])).toHaveLength(48);
    expect(timeOptionsForOpeningHours([day('08:00', '23:00', true)])).toHaveLength(48);
    expect(timeOptionsForOpeningHours(undefined)).toHaveLength(48);
  });

  it('keeps a current value that falls outside the hours', () => {
    const options = timeOptionsForOpeningHours([day('08:00', '23:00')], ['07:00', '12:00']);
    expect(options).toContain('07:00');
    expect(options.filter((o) => o === '12:00')).toHaveLength(1);
    expect(options.indexOf('07:00')).toBeLessThan(options.indexOf('08:00'));
  });

  it('ignores an empty current value', () => {
    expect(timeOptionsForOpeningHours([day('08:00', '23:00')], [''])).toHaveLength(31);
  });
});
