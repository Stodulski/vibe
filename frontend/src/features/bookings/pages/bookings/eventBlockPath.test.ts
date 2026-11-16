import { describe, it, expect } from 'vitest';
import type { CourtEventBlock } from '@/shared/types/api.types';
import { eventBlockPath } from './eventBlockPath';

describe('eventBlockPath', () => {
  it('opens an open-play event on the events page with ?event=', () => {
    const block = { event_id: 'ev1' } as unknown as CourtEventBlock;
    expect(eventBlockPath(block)).toBe('/events?event=ev1');
  });

  it('opens a tournament match on the tournament detail', () => {
    const block = { match_id: 'm1', tournament_id: 't9' } as unknown as CourtEventBlock;
    expect(eventBlockPath(block)).toBe('/events/tournaments/t9');
  });
});
