import { describe, it, expect } from 'vitest';
import { makeMatch } from '../testing/fixtures';
import { courtFeeSummary } from './courtFee';

describe('courtFeeSummary', () => {
  it('collects charges at the amount they were made and leaves pending what the chargeable pairs owe now', () => {
    const matches = [
      makeMatch({
        id: 'a',
        pair1Id: 'p1',
        pair2Id: 'p2',
        courtFee: { due: 2000, charged: { 1: { amount: 1500, method: 'cash' } } },
      }),
      makeMatch({ id: 'b', pair1Id: 'p1', pair2Id: 'p3', courtFee: { due: 2000 } }),
    ];
    expect(courtFeeSummary(matches)).toEqual({ collected: 1500, pending: 6000 });
  });

  it('counts nothing pending for a walkover, a bye, an unknown pair or no fee', () => {
    const matches = [
      makeMatch({
        id: 'w',
        pair1Id: 'p1',
        pair2Id: 'p2',
        status: 'completed',
        walkoverLoser: 1,
        courtFee: { due: 2000 },
      }),
      makeMatch({ id: 'b', pair1Id: 'p1', bye: true, courtFee: { due: 2000 } }),
      makeMatch({ id: 'u', courtFee: { due: 2000 } }),
      makeMatch({ id: 'n', pair1Id: 'p1', pair2Id: 'p2' }),
    ];
    expect(courtFeeSummary(matches)).toEqual({ collected: 0, pending: 0 });
  });
});
