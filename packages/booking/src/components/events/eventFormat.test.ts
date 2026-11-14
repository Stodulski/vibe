import { describe, it, expect } from 'vitest';
import { eventHasStarted } from './eventFormat';

const EVENT = { date: '2026-12-05', start_time: '19:00' };

describe('eventHasStarted', () => {
  it('reads the start in the venue zone (UTC-3), not the process zone', () => {
    expect(eventHasStarted(EVENT, new Date('2026-12-05T21:59:00Z'))).toBe(false);
    expect(eventHasStarted(EVENT, new Date('2026-12-05T22:00:00Z'))).toBe(true);
  });

  it('is false for an event days away and true for one long past', () => {
    expect(eventHasStarted(EVENT, new Date('2026-11-14T12:00:00Z'))).toBe(false);
    expect(eventHasStarted(EVENT, new Date('2027-01-01T00:00:00Z'))).toBe(true);
  });
});
