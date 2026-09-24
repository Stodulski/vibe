import { describe, it, expect } from 'vitest';
import { formatSessionRange } from './formatSessionRange';

describe('formatSessionRange', () => {
  it('shows the day once when the session opened and closed on the same venue day', () => {
    expect(formatSessionRange('2026-09-20T09:00:00-03:00', '2026-09-20T22:00:00-03:00')).toBe('20/9 · 09:00–22:00');
  });

  it('shows both days when the session ran past venue midnight', () => {
    expect(formatSessionRange('2026-09-20T09:00:00-03:00', '2026-09-21T02:00:00-03:00')).toBe(
      '20/9 09:00 – 21/9 02:00',
    );
  });

  it('uses the venue day, not UTC, to decide whether the days match', () => {
    // 23:30 venue time is already the next day in UTC.
    expect(formatSessionRange('2026-09-20T18:00:00-03:00', '2026-09-20T23:30:00-03:00')).toBe('20/9 · 18:00–23:30');
  });

  it('shows only the opening when the session has no close yet', () => {
    expect(formatSessionRange('2026-09-20T09:00:00-03:00', null)).toBe('20/9 · 09:00');
  });

  it('returns an empty string for a missing or invalid opening', () => {
    expect(formatSessionRange(null, null)).toBe('');
    expect(formatSessionRange('nope', null)).toBe('');
  });
});
