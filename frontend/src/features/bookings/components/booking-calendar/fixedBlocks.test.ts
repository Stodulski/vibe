import { describe, it, expect } from 'vitest';
import { heldBlockRange } from './fixedBlocks';

const night = { court_id: 'c', date: '2027-03-19', start_time: '23:00', end_time: '01:00' };

describe('heldBlockRange', () => {
  it('draws a same-day block between its own hours', () => {
    expect(heldBlockRange({ ...night, start_time: '18:00', end_time: '19:30' }, '2027-03-19')).toEqual({
      startMin: 1080,
      endMin: 1170,
      isTail: false,
      runsOn: false,
    });
  });

  it('draws an overnight block to the end of its own day and its tail on the next', () => {
    expect(heldBlockRange(night, '2027-03-19')).toEqual({ startMin: 1380, endMin: 1440, isTail: false, runsOn: true });
    expect(heldBlockRange(night, '2027-03-20')).toEqual({ startMin: 0, endMin: 60, isTail: true, runsOn: false });
  });

  it('is not on any other day, nor does a same-day block leave a tail', () => {
    expect(heldBlockRange(night, '2027-03-21')).toBeNull();
    expect(heldBlockRange(night, '2027-03-18')).toBeNull();
    expect(heldBlockRange({ ...night, start_time: '18:00', end_time: '19:30' }, '2027-03-20')).toBeNull();
  });
});
