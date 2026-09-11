import { describe, expect, it } from 'vitest';
import { catmullRom } from './catmullRom';

// Approval tests for `catmullRom` (Catmull-Rom cubic interpolation).
// These lock the pre-existing numeric behavior in place after adding a
// `noUncheckedIndexedAccess` guard for the tuple-index `restrict-plus-operands`
// finding (slice 9.2). Values were captured from the unmodified implementation
// before the guard was introduced.
describe('catmullRom', () => {
  const p0: [number, number] = [0, 10];
  const p1: [number, number] = [10, 20];
  const p2: [number, number] = [20, 15];
  const p3: [number, number] = [30, 25];

  it('returns p1 at t=0', () => {
    expect(catmullRom(p0, p1, p2, p3, 0)).toEqual([10, 20]);
  });

  it('returns p2 at t=1', () => {
    expect(catmullRom(p0, p1, p2, p3, 1)).toEqual([20, 15]);
  });

  it('interpolates at t=0.5', () => {
    expect(catmullRom(p0, p1, p2, p3, 0.5)).toEqual([15, 17.5]);
  });

  it('interpolates at t=0.25', () => {
    expect(catmullRom(p0, p1, p2, p3, 0.25)).toEqual([12.5, 19.453125]);
  });
});
