import { todayInArgentina } from './today';

describe('todayInArgentina', () => {
  afterEach(() => {
    vi.useRealTimers();
  });

  it('stays on the Argentina calendar day after UTC has already rolled over', () => {
    // 2026-09-04T23:30 in Argentina (UTC-3) is 2026-09-05T02:30 in UTC — a
    // plain `new Date().toISOString().slice(0, 10)` reads this as "2026-09-05".
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-09-05T02:30:00Z'));

    expect(todayInArgentina()).toBe('2026-09-04');
    expect(new Date().toISOString().slice(0, 10)).toBe('2026-09-05');
  });

  it('agrees with UTC well inside the Argentina day', () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-09-05T15:00:00Z'));

    expect(todayInArgentina()).toBe('2026-09-05');
  });
});
