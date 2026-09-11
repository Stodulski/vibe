// @vitest-environment node
import { describe, it, expect } from 'vitest';
import { queryKeys } from './queryKeys';

describe('queryKeys.bookings', () => {
  it('detail is not a prefix match of byComplex, so invalidating one never sweeps the other', () => {
    const complexId = 'b-1'; // deliberately equal to the booking id used below
    const byComplexKey = queryKeys.bookings.byComplex(complexId);
    const detailKey = queryKeys.bookings.detail('b-1');

    // A React Query prefix match means `detailKey` starts with every element
    // of `byComplexKey` in order. If it did, invalidating `byComplex(complexId)`
    // would also invalidate `detail('b-1')` whenever the two ids collide.
    const isPrefixMatch = byComplexKey.every((segment, index) => detailKey[index] === segment);

    expect(isPrefixMatch).toBe(false);
    expect(detailKey).toEqual(['bookings', 'detail', 'b-1']);
  });
});
