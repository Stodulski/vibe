import { describe, it, expect } from 'vitest';
import { makeMatch } from '../testing/fixtures';
import { isCorrectionLocked } from './correctionLock';

describe('isCorrectionLocked', () => {
  it('locks a bracket match once the match it feeds has a result', () => {
    const semi = makeMatch({ id: 's1', round: 1, position: 0, status: 'completed' });
    const final = makeMatch({ id: 'f', round: 2, position: 0, status: 'completed' });
    expect(isCorrectionLocked(semi, [semi, final])).toBe(true);
  });

  it('leaves a bracket match open while the match it feeds has no result, and the final always', () => {
    const semi = makeMatch({ id: 's1', round: 1, position: 1, status: 'completed' });
    const otherSemi = makeMatch({ id: 's2', round: 1, position: 2, status: 'completed' });
    const final = makeMatch({ id: 'f', round: 2, position: 1 });
    expect(isCorrectionLocked(semi, [semi, otherSemi, final])).toBe(false);
    expect(isCorrectionLocked(final, [semi, final])).toBe(false);
  });

  it('feeds position 2k and 2k+1 into position k of the next round', () => {
    const a = makeMatch({ id: 'a', round: 1, position: 2, status: 'completed' });
    const b = makeMatch({ id: 'b', round: 1, position: 3, status: 'completed' });
    const next = makeMatch({ id: 'n', round: 2, position: 1, status: 'completed' });
    expect(isCorrectionLocked(a, [a, b, next])).toBe(true);
    expect(isCorrectionLocked(b, [a, b, next])).toBe(true);
  });

  it('locks a group match once a bracket match its group feeds has a result', () => {
    const group = makeMatch({ id: 'g', stage: 'group', groupId: 'g1', status: 'completed' });
    const played = makeMatch({
      id: 'b',
      round: 1,
      position: 0,
      source1: { group_id: 'g1', rank: 1 },
      status: 'completed',
    });
    const open = makeMatch({ id: 'b', round: 1, position: 0, source1: { group_id: 'g1', rank: 1 } });
    expect(isCorrectionLocked(group, [group, played])).toBe(true);
    expect(isCorrectionLocked(group, [group, open])).toBe(false);
  });

  it('looks through a bye a group feeds to the match after it', () => {
    const group = makeMatch({ id: 'g', stage: 'group', groupId: 'g1', status: 'completed' });
    const bye = makeMatch({
      id: 'bye',
      round: 1,
      position: 0,
      bye: true,
      source1: { group_id: 'g1', rank: 1 },
      status: 'completed',
    });
    const next = makeMatch({ id: 'n', round: 2, position: 0, status: 'completed' });
    const pending = makeMatch({ id: 'n', round: 2, position: 0 });
    expect(isCorrectionLocked(group, [group, bye, next])).toBe(true);
    expect(isCorrectionLocked(group, [group, bye, pending])).toBe(false);
  });

  it('ignores a group that feeds nothing played', () => {
    const group = makeMatch({ id: 'g', stage: 'group', groupId: 'g1', status: 'completed' });
    const elsewhere = makeMatch({
      id: 'b',
      round: 1,
      position: 0,
      source1: { group_id: 'g2', rank: 1 },
      status: 'completed',
    });
    expect(isCorrectionLocked(group, [group, elsewhere])).toBe(false);
  });
});
