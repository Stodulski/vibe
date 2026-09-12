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

// The three keys that used to be written as literals at their call site
// (`useVerifyEmail`, `useCancelBookingFlow`, `useSlugAvailability`). Asserting
// the exact arrays here is what makes moving them into the factory a no-op
// for anything already sitting in a warm cache.
describe('queryKeys for the formerly inline keys', () => {
  it('keeps the shapes the call sites used before the factory owned them', () => {
    expect(queryKeys.auth.verifyEmail('tok')).toEqual(['auth', 'verify-email', 'tok']);
    expect(queryKeys.complexes.slugAvailable('club-norte')).toEqual(['complexes', 'slug-available', 'club-norte']);
    expect(queryKeys.cancelInfo.byToken('tok')).toEqual(['cancel-info', 'tok']);
  });
});
